package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateIncidentUpdateRequest is the body of POST /admin/incidents/:id/updates.
type CreateIncidentUpdateRequest struct {
	Status string `json:"status" binding:"required,oneof=investigating identified monitoring resolved"`
	// Translations holds the update message, per locale. Only the "message"
	// field is accepted, and at least one message is required.
	Translations []TranslationRequest `json:"translations"`
	// ComponentStatus, when set, changes the status of the incident's
	// component, as for incidents.
	ComponentStatus string `json:"component_status" binding:"omitempty,oneof=operational degraded_performance partial_outage major_outage"`
}

// UpdateIncidentUpdateRequest is the body of PUT /admin/incidents/:id/updates/:update_id.
type UpdateIncidentUpdateRequest struct {
	Status       string               `json:"status" binding:"omitempty,oneof=investigating identified monitoring resolved"`
	Translations []TranslationRequest `json:"translations"`
}

// updatesNewestFirst preloads an incident's updates, newest first.
func updatesNewestFirst(db *gorm.DB) *gorm.DB {
	return db.Order("created_at DESC, id DESC")
}

// validateUpdateMessages checks that translations only carry messages, and
// that at least one is not empty when required.
func validateUpdateMessages(translations []TranslationRequest, required bool) string {
	hasMessage := false
	for _, t := range translations {
		if t.Field != "message" {
			return "Incident updates only accept the \"message\" field"
		}
		if strings.TrimSpace(t.Value) != "" {
			hasMessage = true
		}
	}
	if required && !hasMessage {
		return "A message is required"
	}
	return ""
}

// replaceUpdateTranslations stores the messages of an update.
func replaceUpdateTranslations(tx *gorm.DB, updateID uint, translations []TranslationRequest) error {
	if err := tx.Unscoped().Where("entity_id = ? AND entity_type = ?", updateID, "incident_update").
		Delete(&models.Translation{}).Error; err != nil {
		return err
	}
	for _, t := range translations {
		if strings.TrimSpace(t.Value) == "" {
			continue
		}
		tr := models.Translation{EntityID: updateID, EntityType: "incident_update", Locale: t.Locale, Field: t.Field, Value: t.Value}
		if err := tx.Create(&tr).Error; err != nil {
			return err
		}
	}
	return nil
}

// syncIncidentStatus sets the incident status to that of its latest update.
// An incident without updates keeps its own status.
func syncIncidentStatus(tx *gorm.DB, incidentID uint) error {
	var latest models.IncidentUpdate
	err := updatesNewestFirst(tx).Where("incident_id = ?", incidentID).First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return tx.Model(&models.Incident{}).Where("id = ?", incidentID).Update("status", latest.Status).Error
}

func parseID(c *gin.Context, param, label string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid " + label + " ID"})
		return 0, false
	}
	return uint(id), true
}

// loadIncidentWithUpdates reloads an incident for an admin response.
func loadIncidentWithUpdates(db *gorm.DB, id uint) (models.Incident, error) {
	var incident models.Incident
	err := db.Preload("Translations").Preload("Component").
		Preload("Updates", updatesNewestFirst).Preload("Updates.Translations").Preload("Updates.User").
		First(&incident, id).Error
	return incident, err
}

// CreateIncidentUpdate adds an update to an incident's timeline. The
// incident status becomes the update status, and subscribers are notified.
// POST /api/v1/admin/incidents/:id/updates
func CreateIncidentUpdate(db *gorm.DB, notificationService *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incidentID, ok := parseID(c, "id", "incident")
		if !ok {
			return
		}
		var req CreateIncidentUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}
		if msg := validateUpdateMessages(req.Translations, true); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}

		var incident models.Incident
		if err := db.First(&incident, incidentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch incident"})
			return
		}

		update := models.IncidentUpdate{IncidentID: incident.ID, Status: req.Status}
		if user, _ := services.GetCurrentUser(c); user != nil {
			update.UserID = &user.ID
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&update).Error; err != nil {
				return err
			}
			if err := replaceUpdateTranslations(tx, update.ID, req.Translations); err != nil {
				return err
			}
			if err := syncIncidentStatus(tx, incident.ID); err != nil {
				return err
			}
			if req.ComponentStatus != "" {
				return services.SetComponentStatus(tx, incident.ComponentID, req.ComponentStatus)
			}
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create incident update"})
			return
		}

		incident, err = loadIncidentWithUpdates(db, incident.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload incident"})
			return
		}
		db.Preload("Translations").First(&update, update.ID)

		if notificationService != nil {
			go func(incident models.Incident, update models.IncidentUpdate) {
				if _, err := notificationService.QueueIncidentUpdateNotifications(&incident, &update); err != nil {
					log.Printf("Failed to queue incident update notifications: %v", err)
				}
			}(incident, update)
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":  "Incident update created successfully",
			"update":   update,
			"incident": incident,
		})
	}
}

// findIncidentUpdate loads an update and checks it belongs to the incident in
// the URL. It writes the error response and returns false when it does not.
func findIncidentUpdate(c *gin.Context, db *gorm.DB) (models.IncidentUpdate, bool) {
	var update models.IncidentUpdate
	incidentID, ok := parseID(c, "id", "incident")
	if !ok {
		return update, false
	}
	updateID, ok := parseID(c, "update_id", "update")
	if !ok {
		return update, false
	}
	err := db.Where("id = ? AND incident_id = ?", updateID, incidentID).First(&update).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incident update not found"})
		return update, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch incident update"})
		return update, false
	}
	return update, true
}

// UpdateIncidentUpdate edits an update, for example to fix a typo. It does
// not notify subscribers again.
// PUT /api/v1/admin/incidents/:id/updates/:update_id
func UpdateIncidentUpdate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UpdateIncidentUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}
		if len(req.Translations) > 0 {
			if msg := validateUpdateMessages(req.Translations, true); msg != "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": msg})
				return
			}
		}
		update, ok := findIncidentUpdate(c, db)
		if !ok {
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if req.Status != "" && req.Status != update.Status {
				if err := tx.Model(&update).Update("status", req.Status).Error; err != nil {
					return err
				}
			}
			if len(req.Translations) > 0 {
				if err := replaceUpdateTranslations(tx, update.ID, req.Translations); err != nil {
					return err
				}
			}
			return syncIncidentStatus(tx, update.IncidentID)
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save incident update"})
			return
		}

		incident, err := loadIncidentWithUpdates(db, update.IncidentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload incident"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Incident update saved", "incident": incident})
	}
}

// DeleteIncidentUpdate removes an update. The incident status falls back to
// that of the latest remaining update.
// DELETE /api/v1/admin/incidents/:id/updates/:update_id
func DeleteIncidentUpdate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		update, ok := findIncidentUpdate(c, db)
		if !ok {
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Unscoped().Where("entity_id = ? AND entity_type = ?", update.ID, "incident_update").
				Delete(&models.Translation{}).Error; err != nil {
				return err
			}
			if err := tx.Delete(&update).Error; err != nil {
				return err
			}
			return syncIncidentStatus(tx, update.IncidentID)
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete incident update"})
			return
		}

		incident, err := loadIncidentWithUpdates(db, update.IncidentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload incident"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Incident update deleted", "incident": incident})
	}
}

// deleteIncidentUpdates removes all updates of an incident, with their
// messages. It is used when the incident itself is deleted.
func deleteIncidentUpdates(tx *gorm.DB, incidentID uint) error {
	var ids []uint
	if err := tx.Model(&models.IncidentUpdate{}).Where("incident_id = ?", incidentID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Unscoped().Where("entity_type = ? AND entity_id IN ?", "incident_update", ids).
		Delete(&models.Translation{}).Error; err != nil {
		return err
	}
	return tx.Where("incident_id = ?", incidentID).Delete(&models.IncidentUpdate{}).Error
}
