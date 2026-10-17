package handlers

import (
	"net/http"

	"api/internal/language"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetLanguage returns the public page's default language and mode (public
// endpoint).
func GetLanguage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, _, err := language.Load(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load the language setting"})
			return
		}
		c.JSON(http.StatusOK, s)
	}
}

// UpdateLanguage sets the public page's default language and mode (admin
// only).
func UpdateLanguage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input language.Settings
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := input.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := language.Save(db, input); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save the language setting"})
			return
		}
		c.JSON(http.StatusOK, input)
	}
}
