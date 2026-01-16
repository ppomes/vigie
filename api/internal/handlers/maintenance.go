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
	ComponentID  uint                 `json:"component_id" binding:"required"`
	StartAt      time.Time            `json:"start_at" binding:"required"`
	EndAt        time.Time            `json:"end_at" binding:"required"`
	Translations []TranslationRequest `json:"translations"`
}

// UpdateMaintenanceRequest represents the request to update a maintenance
type UpdateMaintenanceRequest struct {
	Name         string               `json:"name"`
	Status       string               `json:"status" binding:"omitempty,oneof=scheduled in_progress completed"`
	ComponentID  *uint                `json:"component_id"`
	StartAt      *time.Time           `json:"start_at"`
	EndAt        *time.Time           `json:"end_at"`
	Translations []TranslationRequest `json:"translations"`
}

// updateComponentStatusFromMaintenances checks if there's an active maintenance for a component
// and updates its status to under_maintenance, or reverts to operational/incident-based status
func updateComponentStatusFromMaintenances(tx *gorm.DB, componentID uint) error {
	now := time.Now()

	// Check if there's an active maintenance (in_progress)
	var activeMaintenance models.Maintenance
	err := tx.Where("component_id = ? AND status = ?", componentID, models.MaintenanceInProgress).
		First(&activeMaintenance).Error

	if err == nil {
		// Active maintenance found - set component to under_maintenance
		return tx.Model(&models.Component{}).Where("id = ?", componentID).Update("status", "under_maintenance").Error
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	// No active maintenance - check for upcoming scheduled maintenance that should be active
	var scheduledMaintenance models.Maintenance
	err = tx.Where("component_id = ? AND status = ? AND start_at <= ? AND end_at > ?",
		componentID, models.MaintenanceScheduled, now, now).
		First(&scheduledMaintenance).Error

	if err == nil {
		// Found a maintenance that should be in_progress
		tx.Model(&scheduledMaintenance).Update("status", models.MaintenanceInProgress)
		return tx.Model(&models.Component{}).Where("id = ?", componentID).Update("status", "under_maintenance").Error
	}

	// No active maintenance - check incidents and set appropriate status
	return updateComponentStatusFromIncidents(tx, componentID)
}

// ListMaintenances returns all maintenances
// GET /api/v1/admin/maintenances
func ListMaintenances(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var maintenances []models.Maintenance
		if err := db.Preload("Translations").Preload("Component").
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

		// Verify component exists
		var component models.Component
		if err := db.First(&component, req.ComponentID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Component not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify component"})
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
			ComponentID: req.ComponentID,
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

		// Update component status if maintenance is in_progress
		if status == models.MaintenanceInProgress {
			if err := updateComponentStatusFromMaintenances(tx, maintenance.ComponentID); err != nil {
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

		// Reload maintenance with translations and component
		db.Preload("Translations").Preload("Component").First(&maintenance, maintenance.ID)

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

		oldComponentID := maintenance.ComponentID
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
		if req.ComponentID != nil {
			var component models.Component
			if err := tx.First(&component, *req.ComponentID).Error; err != nil {
				tx.Rollback()
				if err == gorm.ErrRecordNotFound {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Component not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify component"})
				return
			}
			maintenance.ComponentID = *req.ComponentID
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

		// Update component statuses if needed
		if oldComponentID != maintenance.ComponentID {
			// Update old component's status
			if err := updateComponentStatusFromMaintenances(tx, oldComponentID); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update old component status"})
				return
			}
		}

		// Update current component's status
		if err := updateComponentStatusFromMaintenances(tx, maintenance.ComponentID); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update component status"})
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

		componentID := maintenance.ComponentID

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

		// Update component status
		if err := updateComponentStatusFromMaintenances(tx, componentID); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update component status"})
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
		if err := db.Preload("Translations").Preload("Component").
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
			StartAt     time.Time `json:"start_at"`
			EndAt       time.Time `json:"end_at"`
		}

		response := make([]MaintenanceResponse, 0, len(maintenances))
		for _, m := range maintenances {
			name := getTranslationValue(m.Translations, "name", locale, m.Name)
			message := getTranslationValue(m.Translations, "message", locale, "")
			componentName := m.Component.Name
			if len(m.Component.Translations) > 0 {
				componentName = getTranslationValue(m.Component.Translations, "name", locale, componentName)
			}

			response = append(response, MaintenanceResponse{
				ID:          m.ID,
				Name:        name,
				Message:     message,
				Status:      m.Status,
				ComponentID: m.ComponentID,
				Component:   componentName,
				StartAt:     m.StartAt,
				EndAt:       m.EndAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"maintenances": response,
		})
	}
}
