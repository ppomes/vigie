package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// getTranslationValue retrieves a translated value with fallback to English then default
func getTranslationValue(translations []models.Translation, field, locale, defaultValue string) string {
	// Try requested locale first
	for _, t := range translations {
		if t.Field == field && t.Locale == locale {
			return t.Value
		}
	}
	// Fall back to English
	if locale != "en" {
		for _, t := range translations {
			if t.Field == field && t.Locale == "en" {
				return t.Value
			}
		}
	}
	return defaultValue
}

// CreateMaintenanceRequest represents the request to create a new maintenance
type CreateMaintenanceRequest struct {
	Name         string               `json:"name" binding:"required"`
	Status       string               `json:"status" binding:"omitempty,oneof=scheduled in_progress completed"`
	// ComponentID is the main covered component. It may be omitted when
	// ComponentIDs is given; the first one is then the main component.
	ComponentID  uint                 `json:"component_id"`
	// ComponentIDs lists more covered components.
	ComponentIDs []uint               `json:"component_ids"`
	StartAt      time.Time            `json:"start_at" binding:"required"`
	EndAt        time.Time            `json:"end_at" binding:"required"`
	Translations []TranslationRequest `json:"translations"`
}

// UpdateMaintenanceRequest represents the request to update a maintenance
type UpdateMaintenanceRequest struct {
	Name         string               `json:"name"`
	Status       string               `json:"status" binding:"omitempty,oneof=scheduled in_progress completed"`
	ComponentID  *uint                `json:"component_id"`
	// ComponentIDs, when given, replaces the covered components, together
	// with ComponentID (or the current main component).
	ComponentIDs *[]uint              `json:"component_ids"`
	StartAt      *time.Time           `json:"start_at"`
	EndAt        *time.Time           `json:"end_at"`
	Translations []TranslationRequest `json:"translations"`
}

// ListMaintenances returns all maintenances
// GET /api/v1/admin/maintenances
func ListMaintenances(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var maintenances []models.Maintenance
		if err := db.Preload("Translations").Preload("Component").Preload("Components").
			Order("start_at DESC").Find(&maintenances).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch maintenances"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"maintenances": maintenances,
		})
	}
}

// CreateMaintenance handles the creation of a new maintenance
// POST /api/v1/admin/maintenances
func CreateMaintenance(db *gorm.DB, notificationService *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateMaintenanceRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Validate dates
		if req.EndAt.Before(req.StartAt) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "End date must be after start date"})
			return
		}

		componentIDs, ok := componentSet(c, db, req.ComponentID, req.ComponentIDs)
		if !ok {
			return
		}

		// Default status to scheduled if not provided
		status := req.Status
		if status == "" {
			status = models.MaintenanceScheduled
		}

		// Create maintenance
		maintenance := models.Maintenance{
			Name:        req.Name,
			Status:      status,
			ComponentID: componentIDs[0],
			StartAt:     req.StartAt,
			EndAt:       req.EndAt,
		}

		// Start transaction
		tx := db.Begin()
		if err := tx.Create(&maintenance).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create maintenance"})
			return
		}

		// Create translations if provided
		if len(req.Translations) > 0 {
			for _, t := range req.Translations {
				translation := models.Translation{
					EntityID:   maintenance.ID,
					EntityType: "maintenance",
					Locale:     t.Locale,
					Field:      t.Field,
					Value:      t.Value,
				}
				if err := tx.Create(&translation).Error; err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create translations"})
					return
				}
			}
		}

		if _, _, err := replaceMaintenanceComponents(tx, maintenance.ID, componentIDs); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save maintenance components"})
			return
		}

		// Put the components under maintenance if it starts right away
		if status == models.MaintenanceInProgress {
			if err := services.EnterMaintenance(tx, &maintenance); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update component status"})
				return
			}
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save maintenance"})
			return
		}

		// Reload maintenance with translations and components
		db.Preload("Translations").Preload("Component").Preload("Components").First(&maintenance, maintenance.ID)

		// Send notifications to subscribers (async, non-blocking)
		if notificationService != nil {
			go func() {
				if err := notificationService.NotifyMaintenanceScheduled(&maintenance); err != nil {
					log.Printf("Failed to send maintenance notifications: %v", err)
				}
			}()
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":     "Maintenance created successfully",
			"maintenance": maintenance,
		})
	}
}

// UpdateMaintenance handles updating an existing maintenance
// PUT /api/v1/admin/maintenances/:id
func UpdateMaintenance(db *gorm.DB, notificationService *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid maintenance ID"})
			return
		}

		var req UpdateMaintenanceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Find maintenance
		var maintenance models.Maintenance
		if err := db.First(&maintenance, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Maintenance not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch maintenance"})
			return
		}

		oldStatus := maintenance.Status

		// Start transaction
		tx := db.Begin()

		// Update fields if provided
		if req.Name != "" {
			maintenance.Name = req.Name
		}
		if req.Status != "" {
			maintenance.Status = req.Status
		}
		// The covered components after this update, when they change. Moving
		// the main component without a list replaces the set with it.
		var componentIDs []uint
		if req.ComponentID != nil || req.ComponentIDs != nil {
			main := maintenance.ComponentID
			if req.ComponentID != nil {
				main = *req.ComponentID
			}
			var extra []uint
			if req.ComponentIDs != nil {
				extra = *req.ComponentIDs
				if req.ComponentID == nil && !idSet(extra)[main] {
					main = 0
				}
			}
			var ok bool
			componentIDs, ok = componentSet(c, tx, main, extra)
			if !ok {
				tx.Rollback()
				return
			}
			maintenance.ComponentID = componentIDs[0]
		}
		if req.StartAt != nil {
			maintenance.StartAt = *req.StartAt
		}
		if req.EndAt != nil {
			maintenance.EndAt = *req.EndAt
		}

		// Validate dates
		if maintenance.EndAt.Before(maintenance.StartAt) {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "End date must be after start date"})
			return
		}

		// Save maintenance
		if err := tx.Save(&maintenance).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update maintenance"})
			return
		}

		// Update translations if provided
		if len(req.Translations) > 0 {
			// Delete existing translations
			if err := tx.Unscoped().Where("entity_id = ? AND entity_type = ?", maintenance.ID, "maintenance").
				Delete(&models.Translation{}).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update translations"})
				return
			}

			// Create new translations
			for _, t := range req.Translations {
				translation := models.Translation{
					EntityID:   maintenance.ID,
					EntityType: "maintenance",
					Locale:     t.Locale,
					Field:      t.Field,
					Value:      t.Value,
				}
				if err := tx.Create(&translation).Error; err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create translations"})
					return
				}
			}
		}

		// Move component statuses in and out of maintenance. Only the
		// transitions matter: components that stay under an in-progress
		// maintenance are left alone, so a status set meanwhile is kept.
		if err := transitionMaintenance(tx, &maintenance, oldStatus, componentIDs); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update component statuses"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save maintenance"})
			return
		}

		// Reload maintenance
		db.Preload("Translations").Preload("Component").First(&maintenance, maintenance.ID)

		// Send notifications if status changed significantly
		if notificationService != nil && oldStatus != maintenance.Status {
			go func() {
				if err := notificationService.NotifyMaintenanceUpdated(&maintenance); err != nil {
					log.Printf("Failed to send maintenance update notifications: %v", err)
				}
			}()
		}

		c.JSON(http.StatusOK, gin.H{
			"message":     "Maintenance updated successfully",
			"maintenance": maintenance,
		})
	}
}

// DeleteMaintenance handles deleting a maintenance
// DELETE /api/v1/admin/maintenances/:id
func DeleteMaintenance(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid maintenance ID"})
			return
		}

		var maintenance models.Maintenance
		if err := db.First(&maintenance, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Maintenance not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch maintenance"})
			return
		}

		// Start transaction
		tx := db.Begin()

		// Delete translations (hard delete to avoid unique constraint issues)
		if err := tx.Unscoped().Where("entity_id = ? AND entity_type = ?", maintenance.ID, "maintenance").
			Delete(&models.Translation{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete translations"})
			return
		}

		// Delete maintenance (soft delete)
		if err := tx.Delete(&maintenance).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete maintenance"})
			return
		}

		// Deleting a maintenance in progress ends it
		if maintenance.Status == models.MaintenanceInProgress {
			if err := services.LeaveMaintenance(tx, &maintenance); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore component status"})
				return
			}
		}
		if err := tx.Where("maintenance_id = ?", maintenance.ID).Delete(&models.MaintenanceComponent{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete maintenance components"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete maintenance"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Maintenance deleted successfully",
		})
	}
}

// GetMaintenances returns active and upcoming maintenances for public display
// GET /api/v1/maintenances
func GetMaintenances(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		locale := getLocale(c)
		now := time.Now()

		// Get active (in_progress) and upcoming (scheduled) maintenances
		var maintenances []models.Maintenance
		if err := db.Preload("Translations").
			Preload("Component").Preload("Component.Translations", "locale = ?", locale).
			Preload("Components", func(db *gorm.DB) *gorm.DB { return db.Order("id") }).
			Preload("Components.Component").Preload("Components.Component.Translations", "locale = ?", locale).
			Where("status IN (?, ?) AND end_at > ?",
				models.MaintenanceScheduled, models.MaintenanceInProgress, now).
			Order("start_at ASC").
			Find(&maintenances).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch maintenances"})
			return
		}

		// Format response with localized content
		type MaintenanceResponse struct {
			ID          uint      `json:"id"`
			Name        string    `json:"name"`
			Message     string    `json:"message"`
			Status      string    `json:"status"`
			ComponentID uint      `json:"component_id"`
			Component   string    `json:"component"`
			// Components names every covered component, main one first.
			Components []string  `json:"components"`
			StartAt     time.Time `json:"start_at"`
			EndAt       time.Time `json:"end_at"`
		}

		response := make([]MaintenanceResponse, 0, len(maintenances))
		for _, m := range maintenances {
			name := getTranslationValue(m.Translations, "name", locale, m.Name)
			message := getTranslationValue(m.Translations, "message", locale, "")
			componentName := getTranslationValue(m.Component.Translations, "name", locale, m.Component.Name)
			names := []string{}
			for _, mc := range m.Components {
				if mc.Component != nil {
					names = append(names, getTranslationValue(mc.Component.Translations, "name", locale, mc.Component.Name))
				}
			}
			if len(names) == 0 {
				names = []string{componentName}
			}

			response = append(response, MaintenanceResponse{
				ID:          m.ID,
				Name:        name,
				Message:     message,
				Status:      m.Status,
				ComponentID: m.ComponentID,
				Component:   componentName,
				Components:  names,
				StartAt:     m.StartAt,
				EndAt:       m.EndAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"maintenances": response,
		})
	}
}

// transitionMaintenance applies a maintenance update to component statuses.
// componentIDs is the new set of covered components, or nil when unchanged.
//
//   - ending (in progress, then not): every component is restored;
//   - still in progress with a new set: removed components are restored,
//     added ones are put under maintenance;
//   - starting: every component is put under maintenance.
func transitionMaintenance(tx *gorm.DB, m *models.Maintenance, oldStatus string, componentIDs []uint) error {
	wasActive := oldStatus == models.MaintenanceInProgress
	isActive := m.Status == models.MaintenanceInProgress

	if wasActive && !isActive {
		if err := services.LeaveMaintenance(tx, m); err != nil {
			return err
		}
	}

	var added, removed []models.MaintenanceComponent
	if componentIDs != nil {
		var err error
		if added, removed, err = replaceMaintenanceComponents(tx, m.ID, componentIDs); err != nil {
			return err
		}
	}

	switch {
	case wasActive && isActive:
		if err := services.LeaveMaintenanceComponents(tx, m, removed); err != nil {
			return err
		}
		return services.EnterMaintenanceComponents(tx, m, added)
	case !wasActive && isActive:
		return services.EnterMaintenance(tx, m)
	}
	return nil
}
