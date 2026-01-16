package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateMaintenanceTemplateRequest represents the request to create a new maintenance template
type CreateMaintenanceTemplateRequest struct {
	Name               string                    `json:"name" binding:"required"`
	DefaultComponentID *uint                     `json:"default_component_id"`
	NameEN             string                    `json:"name_en"`
	NameFR             string                    `json:"name_fr"`
	MessageEN          string                    `json:"message_en"`
	MessageFR          string                    `json:"message_fr"`
	Variables          []models.TemplateVariable `json:"variables"`
}

// UpdateMaintenanceTemplateRequest represents the request to update a maintenance template
type UpdateMaintenanceTemplateRequest struct {
	Name               string                    `json:"name"`
	DefaultComponentID *uint                     `json:"default_component_id"`
	NameEN             string                    `json:"name_en"`
	NameFR             string                    `json:"name_fr"`
	MessageEN          string                    `json:"message_en"`
	MessageFR          string                    `json:"message_fr"`
	Variables          []models.TemplateVariable `json:"variables"`
}

// ListMaintenanceTemplates returns all maintenance templates
// GET /api/v1/admin/maintenance-templates
func ListMaintenanceTemplates(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var templates []models.MaintenanceTemplate
		if err := db.Order("name ASC").Find(&templates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch templates"})
			return
		}

		// Parse variables JSON for each template
		type TemplateResponse struct {
			models.MaintenanceTemplate
			ParsedVariables []models.TemplateVariable `json:"parsed_variables"`
		}

		var response []TemplateResponse
		for _, t := range templates {
			tr := TemplateResponse{MaintenanceTemplate: t}
			if t.Variables != "" {
				json.Unmarshal([]byte(t.Variables), &tr.ParsedVariables)
			}
			response = append(response, tr)
		}

		c.JSON(http.StatusOK, gin.H{"templates": response})
	}
}

// GetMaintenanceTemplate returns a single maintenance template by ID
// GET /api/v1/admin/maintenance-templates/:id
func GetMaintenanceTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
			return
		}

		var template models.MaintenanceTemplate
		if err := db.First(&template, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
			return
		}

		// Parse variables
		var variables []models.TemplateVariable
		if template.Variables != "" {
			json.Unmarshal([]byte(template.Variables), &variables)
		}

		c.JSON(http.StatusOK, gin.H{
			"template":         template,
			"parsed_variables": variables,
		})
	}
}

// CreateMaintenanceTemplate creates a new maintenance template
// POST /api/v1/admin/maintenance-templates
func CreateMaintenanceTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateMaintenanceTemplateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Serialize variables to JSON
		variablesJSON := "[]"
		if len(req.Variables) > 0 {
			bytes, _ := json.Marshal(req.Variables)
			variablesJSON = string(bytes)
		}

		template := models.MaintenanceTemplate{
			Name:               req.Name,
			DefaultComponentID: req.DefaultComponentID,
			NameEN:             req.NameEN,
			NameFR:             req.NameFR,
			MessageEN:          req.MessageEN,
			MessageFR:          req.MessageFR,
			Variables:          variablesJSON,
		}

		if err := db.Create(&template).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":  "Template created successfully",
			"template": template,
		})
	}
}

// UpdateMaintenanceTemplate updates an existing maintenance template
// PUT /api/v1/admin/maintenance-templates/:id
func UpdateMaintenanceTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
			return
		}

		var template models.MaintenanceTemplate
		if err := db.First(&template, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
			return
		}

		var req UpdateMaintenanceTemplateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Update fields
		if req.Name != "" {
			template.Name = req.Name
		}
		if req.DefaultComponentID != nil {
			template.DefaultComponentID = req.DefaultComponentID
		}
		if req.NameEN != "" {
			template.NameEN = req.NameEN
		}
		if req.NameFR != "" {
			template.NameFR = req.NameFR
		}
		if req.MessageEN != "" {
			template.MessageEN = req.MessageEN
		}
		if req.MessageFR != "" {
			template.MessageFR = req.MessageFR
		}
		if len(req.Variables) > 0 {
			bytes, _ := json.Marshal(req.Variables)
			template.Variables = string(bytes)
		}

		if err := db.Save(&template).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Template updated successfully",
			"template": template,
		})
	}
}

// DeleteMaintenanceTemplate deletes a maintenance template
// DELETE /api/v1/admin/maintenance-templates/:id
func DeleteMaintenanceTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
			return
		}

		var template models.MaintenanceTemplate
		if err := db.First(&template, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
			return
		}

		if err := db.Delete(&template).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Template deleted successfully"})
	}
}

// PreviewMaintenanceTemplate applies variable values to a maintenance template and returns the preview
// POST /api/v1/admin/maintenance-templates/:id/preview
func PreviewMaintenanceTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
			return
		}

		var template models.MaintenanceTemplate
		if err := db.First(&template, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
			return
		}

		var req struct {
			VariableValues map[string]string `json:"variable_values"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			req.VariableValues = make(map[string]string)
		}

		// Add system variables
		now := time.Now()
		systemVars := map[string]string{
			"date":     now.Format("2006-01-02"),
			"time":     now.Format("15:04"),
			"datetime": now.Format("2006-01-02 15:04"),
		}

		// Merge system vars with user vars (user vars take precedence)
		allVars := make(map[string]string)
		for k, v := range systemVars {
			allVars[k] = v
		}
		for k, v := range req.VariableValues {
			allVars[k] = v
		}

		c.JSON(http.StatusOK, gin.H{
			"preview": gin.H{
				"name_en":    replaceVariables(template.NameEN, allVars),
				"name_fr":    replaceVariables(template.NameFR, allVars),
				"message_en": replaceVariables(template.MessageEN, allVars),
				"message_fr": replaceVariables(template.MessageFR, allVars),
			},
		})
	}
}
