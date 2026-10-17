package cachetapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"api/internal/handlers"
	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var incidentSorts = map[string]string{
	"id": "id", "name": "name", "status": "status", "visible": "visible", "stickied": "stickied",
	"component_id": "component_id", "occurred_at": "occurred_at", "created_at": "created_at", "updated_at": "updated_at",
}

func newestUpdatesFirst(tx *gorm.DB) *gorm.DB {
	return tx.Order("created_at DESC, id DESC").Preload("Translations")
}

// incidentQuery reads incidents with their translations and updates.
// Anonymous readers only see visible incidents.
func incidentQuery(c *gin.Context, db *gorm.DB) *gorm.DB {
	q := db.Model(&models.Incident{}).Preload("Translations").Preload("Updates", newestUpdatesFirst)
	if !authenticated(c) {
		q = q.Scopes(handlers.PublicIncidents)
	}
	return q
}

func listIncidents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		q := incidentQuery(c, db)
		for _, f := range []string{"id", "component_id"} {
			if v, ok, err := in.integer(f); err != nil {
				fail(c, http.StatusBadRequest, err.Error())
				return
			} else if ok {
				if f == "component_id" && v == 0 {
					q = q.Where("component_id IS NULL")
				} else {
					q = q.Where(f+" = ?", v)
				}
			}
		}
		if v, ok, err := in.integer("status"); err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		} else if ok {
			q = q.Where("status = ?", incidentStatusToVigie[v])
		}
		for _, f := range []string{"visible", "stickied"} {
			if v, ok, err := in.boolean(f); err != nil {
				fail(c, http.StatusBadRequest, err.Error())
				return
			} else if ok {
				q = q.Where(f+" = ?", v)
			}
		}
		if v, ok := in.str("name"); ok {
			q = q.Where("name = ?", v)
		}
		q = sortQuery(c, q, incidentSorts, by("id", false))

		var incidents []models.Incident
		meta, err := paginate(c, q, &incidents)
		if err != nil {
			fail(c, http.StatusInternalServerError, "Failed to fetch incidents")
			return
		}
		data := make([]gin.H, len(incidents))
		for i, inc := range incidents {
			data[i] = presentIncident(inc)
		}
		list(c, meta, data)
	}
}

func findIncident(c *gin.Context, db *gorm.DB) (models.Incident, bool) {
	var inc models.Incident
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err == nil {
		err = incidentQuery(c, db).First(&inc, id).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, strconv.ErrSyntax) {
			fail(c, http.StatusNotFound, "Incident not found")
		} else {
			fail(c, http.StatusInternalServerError, "Failed to fetch incident")
		}
		return inc, false
	}
	return inc, true
}

func showIncident(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if inc, ok := findIncident(c, db); ok {
			item(c, presentIncident(inc))
		}
	}
}

// incidentBody translates Cachet incident fields. existing is nil on
// creation.
func incidentBody(in input, existing *models.Incident) (map[string]any, error) {
	if in.has("template") {
		return nil, errors.New("incident templates are not supported through the Cachet API: send name and message")
	}
	body := map[string]any{}
	if v, ok := in.str("name"); ok {
		body["name"] = v
	}
	if v, ok, err := in.integer("status"); err != nil {
		return nil, err
	} else if ok {
		s, known := incidentStatusToVigie[v]
		if !known {
			return nil, fmt.Errorf("status %d is not supported: use 1 (investigating) to 4 (fixed)", v)
		}
		body["status"] = s
	}
	if v, ok, err := in.integer("component_id"); err != nil {
		return nil, err
	} else if ok {
		body["component_id"] = v
		if v == 0 {
			body["components"] = []any{}
		}
	}
	if v, ok, err := in.integer("component_status"); err != nil {
		return nil, err
	} else if ok && v != 0 {
		s, known := componentStatusToVigie[v]
		if !known {
			return nil, fmt.Errorf("component_status %d is not supported: use 1 (operational) to 4 (major outage)", v)
		}
		body["component_status"] = s
	}
	for _, f := range []string{"visible", "stickied"} {
		if v, ok, err := in.boolean(f); err != nil {
			return nil, err
		} else if ok {
			body[f] = v
		}
	}
	if existing == nil {
		if v, ok, err := in.boolean("notify"); err != nil {
			return nil, err
		} else if ok {
			body["notify"] = v
		}
	}
	if v, ok := in.str("occurred_at"); ok && v != "" {
		t, err := parseCachetTime(v)
		if err != nil {
			return nil, err
		}
		body["occurred_at"] = t
	}
	if v, ok := in.str("message"); ok {
		var current []models.Translation
		if existing != nil {
			current = existing.Translations
		}
		body["translations"] = withTranslation(current, "message", v)
	}
	return body, nil
}

func createIncident(db *gorm.DB, ns *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if !in.has("name") || !in.has("status") {
			fail(c, http.StatusBadRequest, "name and status are required")
			return
		}
		body, err := incidentBody(in, nil)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		code, resp := forward(c, handlers.CreateIncident(db, ns), http.MethodPost, nil, body)
		if forwardError(c, code, resp) {
			return
		}
		c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(createdID(resp, "incident")), 10)}}
		showIncident(db)(c)
	}
}

func updateIncident(db *gorm.DB, ns *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		existing, ok := findIncident(c, db)
		if !ok {
			return
		}
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		body, err := incidentBody(in, &existing)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		code, resp := forward(c, handlers.UpdateIncident(db, ns), http.MethodPut, c.Params, body)
		if forwardError(c, code, resp) {
			return
		}
		showIncident(db)(c)
	}
}

func deleteIncident(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := findIncident(c, db); !ok {
			return
		}
		code, resp := forward(c, handlers.DeleteIncident(db), http.MethodDelete, c.Params, nil)
		if forwardError(c, code, resp) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// --- Incident updates ---

func listUpdates(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		inc, ok := findIncident(c, db)
		if !ok {
			return
		}
		q := db.Model(&models.IncidentUpdate{}).Where("incident_id = ?", inc.ID).Preload("Translations")
		q = sortQuery(c, q, map[string]string{"id": "id", "status": "status", "created_at": "created_at"},
			by("created_at", true), by("id", true))
		var updates []models.IncidentUpdate
		meta, err := paginate(c, q, &updates)
		if err != nil {
			fail(c, http.StatusInternalServerError, "Failed to fetch incident updates")
			return
		}
		data := make([]gin.H, len(updates))
		for i, u := range updates {
			data[i] = presentUpdate(u)
		}
		list(c, meta, data)
	}
}

func findUpdate(c *gin.Context, db *gorm.DB) (models.IncidentUpdate, bool) {
	var u models.IncidentUpdate
	inc, ok := findIncident(c, db)
	if !ok {
		return u, false
	}
	err := db.Preload("Translations").Where("incident_id = ? AND id = ?", inc.ID, c.Param("update_id")).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusNotFound, "Incident update not found")
		} else {
			fail(c, http.StatusInternalServerError, "Failed to fetch incident update")
		}
		return u, false
	}
	return u, true
}

func showUpdate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if u, ok := findUpdate(c, db); ok {
			item(c, presentUpdate(u))
		}
	}
}

func updateBody(in input, existing *models.IncidentUpdate) (map[string]any, error) {
	body := map[string]any{}
	if v, ok, err := in.integer("status"); err != nil {
		return nil, err
	} else if ok {
		s, known := incidentStatusToVigie[v]
		if !known {
			return nil, fmt.Errorf("status %d is not supported: use 1 (investigating) to 4 (fixed)", v)
		}
		body["status"] = s
	}
	if v, ok := in.str("message"); ok {
		var current []models.Translation
		if existing != nil {
			current = existing.Translations
		}
		body["translations"] = withTranslation(current, "message", v)
	}
	if existing == nil {
		compID, hasComp, err := in.integer("component_id")
		if err != nil {
			return nil, err
		}
		compStatus, hasStatus, err := in.integer("component_status")
		if err != nil {
			return nil, err
		}
		if hasStatus && compStatus != 0 {
			s, known := componentStatusToVigie[compStatus]
			if !known {
				return nil, fmt.Errorf("component_status %d is not supported: use 1 (operational) to 4 (major outage)", compStatus)
			}
			if hasComp && compID != 0 {
				body["components"] = []map[string]any{{"component_id": compID, "status": s}}
			} else {
				body["component_status"] = s
			}
		}
	}
	return body, nil
}

func createUpdate(db *gorm.DB, ns *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		inc, ok := findIncident(c, db)
		if !ok {
			return
		}
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if !in.has("status") || !in.has("message") {
			fail(c, http.StatusBadRequest, "status and message are required")
			return
		}
		body, err := updateBody(in, nil)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		params := gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(inc.ID), 10)}}
		code, resp := forward(c, handlers.CreateIncidentUpdate(db, ns), http.MethodPost, params, body)
		if forwardError(c, code, resp) {
			return
		}
		c.Params = append(params, gin.Param{Key: "update_id", Value: strconv.FormatUint(uint64(createdID(resp, "update")), 10)})
		showUpdate(db)(c)
	}
}

func updateUpdate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		existing, ok := findUpdate(c, db)
		if !ok {
			return
		}
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		body, err := updateBody(in, &existing)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		code, resp := forward(c, handlers.UpdateIncidentUpdate(db), http.MethodPut, c.Params, body)
		if forwardError(c, code, resp) {
			return
		}
		showUpdate(db)(c)
	}
}

func deleteUpdate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := findUpdate(c, db); !ok {
			return
		}
		code, resp := forward(c, handlers.DeleteIncidentUpdate(db), http.MethodDelete, c.Params, nil)
		if forwardError(c, code, resp) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
