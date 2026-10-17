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

var scheduleSorts = map[string]string{
	"id": "id", "name": "name", "status": "status", "scheduled_at": "start_at", "completed_at": "end_at",
	"created_at": "created_at", "updated_at": "updated_at",
}

// scheduleQuery reads maintenances with their components. Anonymous readers
// only see the components shown on the public page.
func scheduleQuery(c *gin.Context, db *gorm.DB) *gorm.DB {
	components := func(tx *gorm.DB) *gorm.DB { return tx.Order("id") }
	if !authenticated(c) {
		components = handlers.PublicAffectedComponents
	}
	return db.Model(&models.Maintenance{}).Preload("Translations").
		Preload("Components", components).
		Preload("Components.Component").Preload("Components.Component.Translations").Preload("Components.Component.Tags")
}

func listSchedules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		q := scheduleQuery(c, db)
		if v, ok, err := in.integer("status"); err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		} else if ok {
			q = q.Where("status = ?", scheduleStatusToVigie[v])
		}
		q = sortQuery(c, q, scheduleSorts, by("start_at", false), by("id", false))
		var schedules []models.Maintenance
		meta, err := paginate(c, q, &schedules)
		if err != nil {
			fail(c, http.StatusInternalServerError, "Failed to fetch schedules")
			return
		}
		data := make([]gin.H, len(schedules))
		for i, m := range schedules {
			data[i] = presentSchedule(m)
		}
		list(c, meta, data)
	}
}

func findSchedule(c *gin.Context, db *gorm.DB) (models.Maintenance, bool) {
	var m models.Maintenance
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err == nil {
		err = scheduleQuery(c, db).First(&m, id).Error
	}
	if err != nil {
		if notFound(err) {
			fail(c, http.StatusNotFound, "Schedule not found")
		} else {
			fail(c, http.StatusInternalServerError, "Failed to fetch schedule")
		}
		return m, false
	}
	return m, true
}

func showSchedule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m, ok := findSchedule(c, db); ok {
			item(c, presentSchedule(m))
		}
	}
}

// scheduleBody translates a Cachet schedule into a Vigie maintenance
// request. Vigie maintenances need an end, so completed_at is required on
// creation. Cachet's per-component statuses are not kept: Vigie puts the
// components under maintenance while it runs.
func scheduleBody(in input, existing *models.Maintenance) (map[string]any, error) {
	body := map[string]any{}
	if v, ok := in.str("name"); ok {
		body["name"] = v
	}
	if v, ok, err := in.integer("status"); err != nil {
		return nil, err
	} else if ok {
		s, known := scheduleStatusToVigie[v]
		if !known {
			return nil, fmt.Errorf("status must be 0 (upcoming), 1 (in progress) or 2 (complete)")
		}
		body["status"] = s
	}
	for cachet, vigie := range map[string]string{"scheduled_at": "start_at", "completed_at": "end_at"} {
		if v, ok := in.str(cachet); ok && v != "" {
			t, err := parseCachetTime(v)
			if err != nil {
				return nil, err
			}
			body[vigie] = t
		}
	}
	if existing == nil {
		if _, ok := body["start_at"]; !ok {
			return nil, errors.New("scheduled_at is required")
		}
		if _, ok := body["end_at"]; !ok {
			return nil, errors.New("completed_at is required: Vigie maintenances need an end")
		}
		// Cachet does not notify subscribers of schedules unless asked.
		notify, _, err := in.boolean("notify")
		if err != nil {
			return nil, err
		}
		body["notify"] = notify
	}
	if ids, ok, err := in.idMap("components"); err != nil {
		return nil, err
	} else if ok {
		// On an update, the main component stays first while it is still
		// covered.
		if existing == nil {
			body["component_id"] = 0
		}
		body["component_ids"] = ids
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

func createSchedule(db *gorm.DB, ns *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if !in.has("name") {
			fail(c, http.StatusBadRequest, "name is required")
			return
		}
		body, err := scheduleBody(in, nil)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		code, resp := forward(c, handlers.CreateMaintenance(db, ns), http.MethodPost, nil, body)
		if forwardError(c, code, resp) {
			return
		}
		c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(createdID(resp, "maintenance")), 10)}}
		showSchedule(db)(c)
	}
}

func updateSchedule(db *gorm.DB, ns *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		existing, ok := findSchedule(c, db)
		if !ok {
			return
		}
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		body, err := scheduleBody(in, &existing)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		code, resp := forward(c, handlers.UpdateMaintenance(db, ns), http.MethodPut, c.Params, body)
		if forwardError(c, code, resp) {
			return
		}
		showSchedule(db)(c)
	}
}

func deleteSchedule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := findSchedule(c, db); !ok {
			return
		}
		code, resp := forward(c, handlers.DeleteMaintenance(db), http.MethodDelete, c.Params, nil)
		if forwardError(c, code, resp) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
