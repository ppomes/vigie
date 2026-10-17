package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

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
	// GroupID puts the component in a group. 0 or absent means no group.
	GroupID *uint                    `json:"group_id"`
	componentExtras
	Translations []TranslationRequest `json:"translations"`
}

// UpdateComponentRequest represents the request to update a component
type UpdateComponentRequest struct {
	Name   string                    `json:"name"`
	Status string                    `json:"status" binding:"omitempty,oneof=operational degraded_performance partial_outage major_outage"`
	Order  *int                      `json:"order"`
	// GroupID moves the component to a group; 0 removes it from its group.
	// Absent leaves it unchanged.
	GroupID *uint                    `json:"group_id"`
	componentExtras
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
	incidentExtras
	// Notify is false to create the incident without notifying
	// subscribers. It defaults to true.
	Notify       *bool                     `json:"notify"`
	Translations []TranslationRequest      `json:"translations"`
}

// incidentExtras are the optional incident fields shared by create and
// update requests. A nil field is left unchanged.
type incidentExtras struct {
	// Visible false keeps the incident off the public page, and from
	// subscribers.
	Visible *bool `json:"visible"`
	// Stickied pins the incident at the top of the public page.
	Stickied *bool `json:"stickied"`
	// OccurredAt is when the incident started. It defaults to now, and
	// null clears it, so the creation time is shown instead.
	OccurredAt optionalTime `json:"occurred_at"`
}

// optionalTime tells an omitted JSON field from an explicit null, which a
// *time.Time cannot: both decode to nil.
type optionalTime struct {
	Value *time.Time
	Set   bool
}

func (t *optionalTime) UnmarshalJSON(data []byte) error {
	t.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		t.Value = nil
		return nil
	}
	var v time.Time
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	t.Value = &v
	return nil
}

// apply writes the extras on a saved incident, through a map so false is
// written (GORM skips zero values in structs).
func (e incidentExtras) apply(tx *gorm.DB, incident *models.Incident) error {
	updates := map[string]any{}
	if e.Visible != nil {
		updates["visible"] = *e.Visible
	}
	if e.Stickied != nil {
		updates["stickied"] = *e.Stickied
	}
	if e.OccurredAt.Set {
		if e.OccurredAt.Value == nil {
			updates["occurred_at"] = nil
		} else {
			updates["occurred_at"] = *e.OccurredAt.Value
		}
	}
	if len(updates) == 0 {
		return nil
	}
	return tx.Model(incident).Updates(updates).Error
}

// UpdateIncidentRequest represents the request to update an incident
type UpdateIncidentRequest struct {
	Name         string                    `json:"name"`
	Status       string                    `json:"status" binding:"omitempty,oneof=investigating identified monitoring resolved"`
	ComponentID  *uint                     `json:"component_id"`
	// ComponentStatus, when set, changes the status of the affected
	// component. When empty, the component status is left as it is.
	ComponentStatus string                 `json:"component_status" binding:"omitempty,oneof=operational degraded_performance partial_outage major_outage"`
	incidentExtras
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

		if msg := req.componentExtras.validate(); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		groupID, ok := resolveGroupID(c, db, req.GroupID)
		if !ok {
			return
		}

		// Create component
		component := models.Component{
			Name:    req.Name,
			Status:  req.Status,
			Order:   req.Order,
			GroupID: groupID,
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

		if err := req.componentExtras.apply(tx, &component); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save component details"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save component"})
			return
		}

		// Reload component with translations
		db.Preload("Translations").Preload("Tags").First(&component, component.ID)

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
		if msg := req.componentExtras.validate(); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
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
		if req.GroupID != nil {
			groupID, ok := resolveGroupID(c, tx, req.GroupID)
			if !ok {
				tx.Rollback()
				return
			}
			component.GroupID = groupID
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

		if err := req.componentExtras.apply(tx, &component); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save component details"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save component"})
			return
		}

		// Reload component with translations
		db.Preload("Translations").Preload("Tags").First(&component, component.ID)

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
		if err := models.ByDisplayOrder(db).Preload("Translations").Preload("Tags").Find(&components).Error; err != nil {
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

		notify := req.Notify == nil || *req.Notify
		now := time.Now()

		// Create incident
		incident := models.Incident{
			Name:        req.Name,
			Status:      req.Status,
			ComponentID: req.ComponentID,
			OccurredAt:  &now,
		}
		if user, _ := services.GetCurrentUser(c); user != nil {
			incident.UserID = &user.ID
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

		extras := map[string]any{}
		if !notify {
			extras["notifications"] = false
		}
		if len(extras) > 0 {
			if err := tx.Model(&incident).Updates(extras).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save incident"})
				return
			}
		}
		if err := req.incidentExtras.apply(tx, &incident); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save incident details"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save incident"})
			return
		}

		// Reload incident with translations and component
		db.Preload("Translations").Preload("Component").First(&incident, incident.ID)

		// Send notifications to subscribers (async, non-blocking), unless
		// asked not to, or the incident is internal.
		if notificationService != nil && incident.Notifications && incident.Visible {
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

		if err := req.incidentExtras.apply(tx, &incident); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save incident details"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save incident"})
			return
		}

		// Reload incident with translations and component
		db.Preload("Translations").Preload("Component").First(&incident, incident.ID)

		// Send notifications to subscribers (async, non-blocking)
		if notificationService != nil && incident.Notifications && incident.Visible {
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
