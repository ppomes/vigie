package handlers

import (
	"log"
	"net/http"
	"strconv"

	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateComponentRequest represents the request to create a new component
type CreateComponentRequest struct {
	Name   string                    `json:"name" binding:"required"`
	Status string                    `json:"status" binding:"required,oneof=operational degraded_performance partial_outage major_outage"`
	Order  int                       `json:"order"`
	Translations []TranslationRequest `json:"translations"`
}

// UpdateComponentRequest represents the request to update a component
type UpdateComponentRequest struct {
	Name   string                    `json:"name"`
	Status string                    `json:"status" binding:"omitempty,oneof=operational degraded_performance partial_outage major_outage"`
	Order  *int                      `json:"order"`
	Translations []TranslationRequest `json:"translations"`
}

// CreateIncidentRequest represents the request to create a new incident
type CreateIncidentRequest struct {
	Name         string                    `json:"name" binding:"required"`
	Status       string                    `json:"status" binding:"required,oneof=investigating identified monitoring resolved"`
	ComponentID  uint                      `json:"component_id" binding:"required"`
	// ComponentStatus, when set, changes the status of the affected
	// component. When empty, the component status is left as it is.
	ComponentStatus string                 `json:"component_status" binding:"omitempty,oneof=operational degraded_performance partial_outage major_outage"`
	Translations []TranslationRequest      `json:"translations"`
}

// UpdateIncidentRequest represents the request to update an incident
type UpdateIncidentRequest struct {
	Name         string                    `json:"name"`
	Status       string                    `json:"status" binding:"omitempty,oneof=investigating identified monitoring resolved"`
	ComponentID  *uint                     `json:"component_id"`
	// ComponentStatus, when set, changes the status of the affected
	// component. When empty, the component status is left as it is.
	ComponentStatus string                 `json:"component_status" binding:"omitempty,oneof=operational degraded_performance partial_outage major_outage"`
	Translations []TranslationRequest      `json:"translations"`
}

// TranslationRequest represents a translation entry in create/update requests
type TranslationRequest struct {
	Locale string `json:"locale" binding:"required,oneof=en fr"`
	Field  string `json:"field" binding:"required"`
	Value  string `json:"value" binding:"required"`
}

// CreateComponent handles the creation of a new component
// POST /api/v1/admin/components
func CreateComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateComponentRequest

		// Bind and validate request JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Create component
		component := models.Component{
			Name:   req.Name,
			Status: req.Status,
			Order:  req.Order,
		}

		// Start transaction
		tx := db.Begin()
		if err := tx.Create(&component).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create component"})
			return
		}

		// Create translations if provided
		if len(req.Translations) > 0 {
			for _, t := range req.Translations {
				translation := models.Translation{
					EntityID:   component.ID,
					EntityType: "component",
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

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save component"})
			return
		}

		// Reload component with translations
		db.Preload("Translations").First(&component, component.ID)

		c.JSON(http.StatusCreated, gin.H{
			"message":   "Component created successfully",
			"component": component,
		})
	}
}

// UpdateComponent handles updating an existing component
// PUT /api/v1/admin/components/:id
func UpdateComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get component ID from URL parameter
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
			return
		}

		var req UpdateComponentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Find component
		var component models.Component
		if err := db.First(&component, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch component"})
			return
		}

		// Start transaction
		tx := db.Begin()

		// Update fields if provided
		if req.Name != "" {
			component.Name = req.Name
		}
		if req.Status != "" {
			component.Status = req.Status
		}
		if req.Order != nil {
			component.Order = *req.Order
		}

		if err := tx.Save(&component).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update component"})
			return
		}

		// Update translations if provided
		if len(req.Translations) > 0 {
			// Delete existing translations for the component
			if err := tx.Unscoped().Where("entity_id = ? AND entity_type = ?", component.ID, "component").Delete(&models.Translation{}).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update translations"})
				return
			}

			// Create new translations
			for _, t := range req.Translations {
				translation := models.Translation{
					EntityID:   component.ID,
					EntityType: "component",
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

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save component"})
			return
		}

		// Reload component with translations
		db.Preload("Translations").First(&component, component.ID)

		c.JSON(http.StatusOK, gin.H{
			"message":   "Component updated successfully",
			"component": component,
		})
	}
}

// DeleteComponent handles deleting a component (soft delete)
// DELETE /api/v1/admin/components/:id
func DeleteComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get component ID from URL parameter
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
			return
		}

		// Find and delete component (soft delete)
		var component models.Component
		if err := db.First(&component, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Component not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch component"})
			return
		}

		// Soft delete (DeletedAt will be set)
		if err := db.Delete(&component).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete component"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Component deleted successfully",
		})
	}
}

// ListComponents handles listing all components (for admin)
// GET /api/v1/admin/components
func ListComponents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var components []models.Component
		if err := db.Preload("Translations").Order("\"order\" ASC, id ASC").Find(&components).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch components"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"components": components,
		})
	}
}

// CreateIncident handles the creation of a new incident
// POST /api/v1/admin/incidents
func CreateIncident(db *gorm.DB, notificationService *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateIncidentRequest

		// Bind and validate request JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
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

		// Create incident
		incident := models.Incident{
			Name:        req.Name,
			Status:      req.Status,
			ComponentID: req.ComponentID,
		}

		// Start transaction
		tx := db.Begin()
		if err := tx.Create(&incident).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create incident"})
			return
		}

		// Create translations if provided
		if len(req.Translations) > 0 {
			for _, t := range req.Translations {
				translation := models.Translation{
					EntityID:   incident.ID,
					EntityType: "incident",
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

		// Change the component status only when the request asks for it
		if req.ComponentStatus != "" {
			if err := services.SetComponentStatus(tx, incident.ComponentID, req.ComponentStatus); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update component status"})
				return
			}
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save incident"})
			return
		}

		// Reload incident with translations and component
		db.Preload("Translations").Preload("Component").First(&incident, incident.ID)

		// Send notifications to subscribers (async, non-blocking)
		if notificationService != nil {
			go func() {
				if err := notificationService.NotifyIncidentCreated(&incident); err != nil {
					log.Printf("Failed to send incident creation notifications: %v", err)
				}
			}()
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":  "Incident created successfully",
			"incident": incident,
		})
	}
}

// UpdateIncident handles updating an existing incident
// PUT /api/v1/admin/incidents/:id
func UpdateIncident(db *gorm.DB, notificationService *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get incident ID from URL parameter
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid incident ID"})
			return
		}

		var req UpdateIncidentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Find incident
		var incident models.Incident
		if err := db.First(&incident, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch incident"})
			return
		}

		// Start transaction
		tx := db.Begin()

		// Update fields if provided
		if req.Name != "" {
			incident.Name = req.Name
		}
		if req.Status != "" {
			incident.Status = req.Status
		}
		if req.ComponentID != nil {
			// Verify component exists
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
			incident.ComponentID = *req.ComponentID
		}

		if err := tx.Save(&incident).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update incident"})
			return
		}

		// Update translations if provided
		if len(req.Translations) > 0 {
			// Delete existing translations for the incident
			if err := tx.Unscoped().Where("entity_id = ? AND entity_type = ?", incident.ID, "incident").Delete(&models.Translation{}).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update translations"})
				return
			}

			// Create new translations
			for _, t := range req.Translations {
				translation := models.Translation{
					EntityID:   incident.ID,
					EntityType: "incident",
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

		// Change the component status only when the request asks for it.
		// Moving an incident to another component leaves both statuses alone.
		if req.ComponentStatus != "" {
			if err := services.SetComponentStatus(tx, incident.ComponentID, req.ComponentStatus); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update component status"})
				return
			}
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save incident"})
			return
		}

		// Reload incident with translations and component
		db.Preload("Translations").Preload("Component").First(&incident, incident.ID)

		// Send notifications to subscribers (async, non-blocking)
		if notificationService != nil {
			go func() {
				if err := notificationService.NotifyIncidentUpdated(&incident); err != nil {
					log.Printf("Failed to send incident update notifications: %v", err)
				}
			}()
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Incident updated successfully",
			"incident": incident,
		})
	}
}

// DeleteIncident handles deleting an incident (soft delete)
// DELETE /api/v1/admin/incidents/:id
func DeleteIncident(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get incident ID from URL parameter
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid incident ID"})
			return
		}

		// Find incident
		var incident models.Incident
		if err := db.First(&incident, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch incident"})
			return
		}

		// Start transaction
		tx := db.Begin()

		// Remove the timeline with the incident
		if err := deleteIncidentUpdates(tx, incident.ID); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete incident updates"})
			return
		}

		// Soft delete (DeletedAt will be set)
		if err := tx.Delete(&incident).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete incident"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete incident"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Incident deleted successfully",
		})
	}
}

// ListIncidents handles listing all incidents (for admin)
// GET /api/v1/admin/incidents
func ListIncidents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var incidents []models.Incident
		if err := db.Preload("Translations").Preload("Component").
			Preload("Updates", updatesNewestFirst).Preload("Updates.Translations").Preload("Updates.User").
			Order("created_at DESC").Find(&incidents).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch incidents"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"incidents": incidents,
		})
	}
}

// ListSubscribers handles listing all subscribers (for admin)
// GET /api/v1/admin/subscribers
func ListSubscribers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var subscribers []models.Subscriber
		if err := db.Order("created_at DESC").Find(&subscribers).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscribers"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"subscribers": subscribers,
		})
	}
}

// DeleteSubscriber handles deleting a subscriber (soft delete)
// DELETE /api/v1/admin/subscribers/:id
func DeleteSubscriber(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get subscriber ID from URL parameter
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscriber ID"})
			return
		}

		// Find and delete subscriber (soft delete)
		var subscriber models.Subscriber
		if err := db.First(&subscriber, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Subscriber not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscriber"})
			return
		}

		// Soft delete (DeletedAt will be set)
		if err := db.Delete(&subscriber).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete subscriber"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Subscriber deleted successfully",
		})
	}
}
