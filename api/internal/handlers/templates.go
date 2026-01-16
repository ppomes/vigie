package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateTemplateRequest represents the request to create a new incident template
type CreateTemplateRequest struct {
	Name               string  `json:"name" binding:"required"`
	DefaultStatus      string  `json:"default_status"`
	DefaultComponentID *uint   `json:"default_component_id"`
	NameEN             string  `json:"name_en"`
	NameFR             string  `json:"name_fr"`
	MessageEN          string  `json:"message_en"`
	MessageFR          string  `json:"message_fr"`
	Variables          []models.TemplateVariable `json:"variables"`
}

// UpdateTemplateRequest represents the request to update a template
type UpdateTemplateRequest struct {
	Name               string  `json:"name"`
	DefaultStatus      string  `json:"default_status"`
	DefaultComponentID *uint   `json:"default_component_id"`
	NameEN             string  `json:"name_en"`
	NameFR             string  `json:"name_fr"`
	MessageEN          string  `json:"message_en"`
	MessageFR          string  `json:"message_fr"`
	Variables          []models.TemplateVariable `json:"variables"`
}

// ApplyTemplateRequest represents a request to apply a template with variable values
type ApplyTemplateRequest struct {
	TemplateID    uint              `json:"template_id" binding:"required"`
	ComponentID   uint              `json:"component_id" binding:"required"`
	Status        string            `json:"status"`
	VariableValues map[string]string `json:"variable_values"`
}

// ListTemplates returns all incident templates
// GET /api/v1/admin/templates
func ListTemplates(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var templates []models.IncidentTemplate
		if err := db.Order("name ASC").Find(&templates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch templates"})
			return
		}

		// Parse variables JSON for each template
		type TemplateResponse struct {
			models.IncidentTemplate
			ParsedVariables []models.TemplateVariable `json:"parsed_variables"`
		}

		var response []TemplateResponse
		for _, t := range templates {
			tr := TemplateResponse{IncidentTemplate: t}
			if t.Variables != "" {
				json.Unmarshal([]byte(t.Variables), &tr.ParsedVariables)
			}
			response = append(response, tr)
		}

		c.JSON(http.StatusOK, gin.H{"templates": response})
	}
}

// GetTemplate returns a single template by ID
// GET /api/v1/admin/templates/:id
func GetTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
			return
		}

		var template models.IncidentTemplate
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
			"template": template,
			"parsed_variables": variables,
		})
	}
}

// CreateTemplate creates a new incident template
// POST /api/v1/admin/templates
func CreateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateTemplateRequest
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

		defaultStatus := req.DefaultStatus
		if defaultStatus == "" {
			defaultStatus = "investigating"
		}

		template := models.IncidentTemplate{
			Name:               req.Name,
			DefaultStatus:      defaultStatus,
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

// UpdateTemplate updates an existing template
// PUT /api/v1/admin/templates/:id
func UpdateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
			return
		}

		var template models.IncidentTemplate
		if err := db.First(&template, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
			return
		}

		var req UpdateTemplateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Update fields
		if req.Name != "" {
			template.Name = req.Name
		}
		if req.DefaultStatus != "" {
			template.DefaultStatus = req.DefaultStatus
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

// DeleteTemplate deletes a template
// DELETE /api/v1/admin/templates/:id
func DeleteTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
			return
		}

		var template models.IncidentTemplate
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

// PreviewTemplate applies variable values to a template and returns the preview
// POST /api/v1/admin/templates/:id/preview
func PreviewTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
			return
		}

		var template models.IncidentTemplate
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

// replaceVariables replaces {{variable}} placeholders with actual values
func replaceVariables(template string, values map[string]string) string {
	result := template
	// Match {{variable_name}} pattern
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	result = re.ReplaceAllStringFunc(result, func(match string) string {
		// Extract variable name from {{name}}
		varName := strings.Trim(match, "{}")
		if val, ok := values[varName]; ok {
			return val
		}
		return match // Keep original if no value provided
	})
	return result
}
