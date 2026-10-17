package handlers

import (
	"net/http"

	"api/internal/customization"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetCustomization returns the custom CSS, header and footer of the public
// page (public endpoint). The HTML is sanitized again on the way out, in
// case the database was edited by hand.
func GetCustomization(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		custom, err := customization.Load(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load the customization"})
			return
		}
		c.JSON(http.StatusOK, custom.Sanitized())
	}
}

// UpdateCustomization saves the custom CSS, header and footer (admin only),
// and returns what was stored: the HTML without what sanitizing removed.
func UpdateCustomization(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input customization.Customization
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := input.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		saved, err := customization.Save(db, input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save the customization"})
			return
		}
		c.JSON(http.StatusOK, saved)
	}
}
