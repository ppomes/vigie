package handlers

import (
	"net/http"

	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// componentSet merges a main component and extra ones into a deduplicated
// list, main first, and checks that each exists. A zero main component means
// "the first extra one". It writes the error response and returns false on
// failure.
func componentSet(c *gin.Context, db *gorm.DB, main uint, extra []uint) ([]uint, bool) {
	ids := []uint{}
	seen := map[uint]bool{}
	for _, id := range append([]uint{main}, extra...) {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one component is required"})
		return nil, false
	}
	var count int64
	if err := db.Model(&models.Component{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify components"})
		return nil, false
	}
	if int(count) != len(ids) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Component not found"})
		return nil, false
	}
	return ids, true
}

func idSet(ids []uint) map[uint]bool {
	m := make(map[uint]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// replaceMaintenanceComponents makes the maintenance cover exactly ids. It
// returns the rows that were added and removed; removed rows keep their
// recorded previous status, so the caller can restore their components.
func replaceMaintenanceComponents(tx *gorm.DB, maintenanceID uint, ids []uint) (added, removed []models.MaintenanceComponent, err error) {
	var current []models.MaintenanceComponent
	if err := tx.Where("maintenance_id = ?", maintenanceID).Find(&current).Error; err != nil {
		return nil, nil, err
	}
	want := idSet(ids)
	have := map[uint]bool{}
	for _, row := range current {
		have[row.ComponentID] = true
		if !want[row.ComponentID] {
			removed = append(removed, row)
		}
	}
	for _, row := range removed {
		if err := tx.Delete(&models.MaintenanceComponent{}, row.ID).Error; err != nil {
			return nil, nil, err
		}
	}
	for _, id := range ids {
		if have[id] {
			continue
		}
		row := models.MaintenanceComponent{MaintenanceID: maintenanceID, ComponentID: id}
		if err := tx.Create(&row).Error; err != nil {
			return nil, nil, err
		}
		added = append(added, row)
	}
	return added, removed, nil
}

// AffectedComponentRequest is one component of an incident, with the status
// to set on it. An empty status leaves the component status unchanged.
type AffectedComponentRequest struct {
	ComponentID uint   `json:"component_id" binding:"required"`
	Status      string `json:"status" binding:"omitempty,oneof=operational degraded_performance partial_outage major_outage"`
}

// applyIncidentComponents makes the incident affect exactly the given
// components, and sets the requested statuses. Components removed from an
// incident keep their status.
func applyIncidentComponents(tx *gorm.DB, incidentID uint, components []AffectedComponentRequest) error {
	want := map[uint]bool{}
	for _, ac := range components {
		want[ac.ComponentID] = true
	}
	var current []models.IncidentComponent
	if err := tx.Where("incident_id = ?", incidentID).Find(&current).Error; err != nil {
		return err
	}
	byComponent := map[uint]models.IncidentComponent{}
	for _, row := range current {
		if !want[row.ComponentID] {
			if err := tx.Delete(&models.IncidentComponent{}, row.ID).Error; err != nil {
				return err
			}
			continue
		}
		byComponent[row.ComponentID] = row
	}
	for _, ac := range components {
		row, ok := byComponent[ac.ComponentID]
		if !ok {
			row = models.IncidentComponent{IncidentID: incidentID, ComponentID: ac.ComponentID}
		}
		if ac.Status != "" {
			row.ComponentStatus = ac.Status
		}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if ac.Status != "" {
			if err := services.SetComponentStatus(tx, ac.ComponentID, ac.Status); err != nil {
				return err
			}
		}
	}
	return nil
}

// incidentComponents builds the affected components of an incident request:
// the main component first, then the listed ones, deduplicated. The legacy
// componentStatus applies to the main component when the list gives it no
// status. It checks that every component exists.
func incidentComponents(c *gin.Context, db *gorm.DB, main uint, list []AffectedComponentRequest, componentStatus string) ([]AffectedComponentRequest, bool) {
	extra := make([]uint, 0, len(list))
	statuses := map[uint]string{}
	for _, ac := range list {
		extra = append(extra, ac.ComponentID)
		if ac.Status != "" {
			statuses[ac.ComponentID] = ac.Status
		}
	}
	ids, ok := componentSet(c, db, main, extra)
	if !ok {
		return nil, false
	}
	if componentStatus != "" && statuses[ids[0]] == "" {
		statuses[ids[0]] = componentStatus
	}
	out := make([]AffectedComponentRequest, len(ids))
	for i, id := range ids {
		out[i] = AffectedComponentRequest{ComponentID: id, Status: statuses[id]}
	}
	return out, true
}

func containsComponent(list []AffectedComponentRequest, id uint) bool {
	for _, ac := range list {
		if ac.ComponentID == id {
			return true
		}
	}
	return false
}

// currentIncidentComponents returns an incident's components as requests
// with no status, main component first.
func currentIncidentComponents(tx *gorm.DB, incidentID uint) ([]AffectedComponentRequest, error) {
	var rows []models.IncidentComponent
	if err := tx.Where("incident_id = ?", incidentID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AffectedComponentRequest, len(rows))
	for i, row := range rows {
		out[i] = AffectedComponentRequest{ComponentID: row.ComponentID}
	}
	return out, nil
}

// withMainStatus sets a status on the main component of a list, adding it
// when missing.
func withMainStatus(list []AffectedComponentRequest, main uint, status string) []AffectedComponentRequest {
	for i := range list {
		if list[i].ComponentID == main {
			list[i].Status = status
			return list
		}
	}
	return append([]AffectedComponentRequest{{ComponentID: main, Status: status}}, list...)
}
