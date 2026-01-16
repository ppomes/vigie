package handlers

import (
	"net/http"
	"strings"

	"api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// StatusResponse represents the overall system status
type StatusResponse struct {
	Status     string `json:"status"` // operational, degraded, outage
	Message    string `json:"message"`
	Components int    `json:"components_count"`
	Incidents  int    `json:"active_incidents_count"`
}

// GetStatus returns the overall system status based on components and active incidents
// Status determination: operational (all components ok), degraded (some issues), outage (major issues)
func GetStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Count total components
		var componentCount int64
		if err := db.Model(&models.Component{}).Count(&componentCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch system status"})
			return
		}

		// Count active incidents (not resolved)
		var activeIncidents int64
		if err := db.Model(&models.Incident{}).Where("status != ?", "resolved").Count(&activeIncidents).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch system status"})
			return
		}

		// Count components with issues
		var componentsWithIssues int64
		if err := db.Model(&models.Component{}).
			Where("status IN ?", []string{"degraded_performance", "partial_outage", "major_outage"}).
			Count(&componentsWithIssues).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch system status"})
			return
		}

		// Determine overall system status
		status := "operational"
		message := "All systems operational"

		if componentsWithIssues > 0 {
			var majorOutages int64
			db.Model(&models.Component{}).Where("status = ?", "major_outage").Count(&majorOutages)

			if majorOutages > 0 {
				status = "outage"
				message = "Some systems are experiencing major outages"
			} else {
				status = "degraded"
				message = "Some systems are experiencing issues"
			}
		}

		c.JSON(http.StatusOK, StatusResponse{
			Status:     status,
			Message:    message,
			Components: int(componentCount),
			Incidents:  int(activeIncidents),
		})
	}
}

// GetComponents returns all components with their translations
// Supports locale parameter via query string (?locale=fr) or Accept-Language header
func GetComponents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get locale from query param or Accept-Language header (default: en)
		locale := getLocale(c)

		// Fetch all components with translations, ordered by display order
		var components []models.Component
		if err := db.Preload("Translations", "locale = ?", locale).
			Order("\"order\" ASC, id ASC").
			Find(&components).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch components"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"components": components,
			"locale":     locale,
		})
	}
}

// GetIncidents returns all incidents with their translations and associated components
// Supports locale parameter and status filtering (?status=investigating&locale=fr)
func GetIncidents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get locale from query param or Accept-Language header (default: en)
		locale := getLocale(c)

		// Get optional status filter from query param
		statusFilter := c.Query("status")

		// Validate status filter if provided
		if statusFilter != "" {
			validStatuses := map[string]bool{
				"investigating": true,
				"identified":    true,
				"monitoring":    true,
				"resolved":      true,
			}
			if !validStatuses[statusFilter] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status value"})
				return
			}
		}

		// Build query with translations and component relationship
		query := db.Preload("Translations", "locale = ?", locale).
			Preload("Component").
			Preload("Component.Translations", "locale = ?", locale)

		// Apply status filter if provided
		if statusFilter != "" {
			query = query.Where("status = ?", statusFilter)
		}

		// Fetch incidents ordered by creation date (newest first)
		var incidents []models.Incident
		if err := query.Order("created_at DESC").Find(&incidents).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch incidents"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"incidents": incidents,
			"locale":    locale,
		})
	}
}

// getLocale extracts the locale from query parameter or Accept-Language header
// Falls back to "en" (English) if not specified
func getLocale(c *gin.Context) string {
	// Check query parameter first (?locale=fr)
	if locale := c.Query("locale"); locale != "" {
		// Validate locale (only support en and fr)
		if locale == "en" || locale == "fr" {
			return locale
		}
	}

	// Check Accept-Language header
	acceptLang := c.GetHeader("Accept-Language")
	if acceptLang != "" {
		// Parse Accept-Language header (e.g., "fr-FR,fr;q=0.9,en;q=0.8")
		// Take the first language code
		parts := strings.Split(acceptLang, ",")
		if len(parts) > 0 {
			lang := strings.TrimSpace(parts[0])
			// Extract language code (e.g., "fr" from "fr-FR")
			if strings.HasPrefix(lang, "fr") {
				return "fr"
			}
		}
	}

	// Default to English
	return "en"
}
