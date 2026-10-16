package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateAPIKeyRequest is the body of POST /api/v1/admin/api-keys.
type CreateAPIKeyRequest struct {
	Name string `json:"name" binding:"required,max=255"`
	// UserID is the user the key acts as. It defaults to the current user.
	UserID *uint `json:"user_id"`
}

// ListAPIKeys returns all API keys, without their secret.
// GET /api/v1/admin/api-keys
func ListAPIKeys(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var keys []models.APIKey
		if err := db.Preload("User").Order("revoked_at IS NOT NULL, created_at DESC").Find(&keys).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch API keys"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"api_keys": keys})
	}
}

// CreateAPIKey creates an API key and returns it. The raw key is only
// returned here; Vigie stores its hash.
// POST /api/v1/admin/api-keys
func CreateAPIKey(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateAPIKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		current, _ := services.GetCurrentUser(c)
		if current == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
			return
		}
		userID := current.ID
		if req.UserID != nil {
			userID = *req.UserID
		}

		var owner models.User
		if err := db.First(&owner, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify user"})
			return
		}
		if !owner.Active {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User account is deactivated"})
			return
		}

		raw, err := models.GenerateAPIKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key"})
			return
		}
		key := models.NewAPIKey(req.Name, raw, owner.ID)
		if err := db.Create(&key).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API key"})
			return
		}
		key.User = owner

		auditService.Log(c, models.ActionCreate, models.ResourceAPIKey, &key.ID, map[string]interface{}{
			"name":    key.Name,
			"user_id": owner.ID,
		}, true)

		c.JSON(http.StatusCreated, gin.H{
			"message": "API key created. Copy it now: it will not be shown again.",
			"api_key": key,
			"key":     raw,
		})
	}
}

// RevokeAPIKey revokes an API key. Revoked keys are kept for the audit trail.
// DELETE /api/v1/admin/api-keys/:id
func RevokeAPIKey(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
			return
		}

		var key models.APIKey
		if err := db.First(&key, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch API key"})
			return
		}

		if key.RevokedAt == nil {
			now := time.Now()
			if err := db.Model(&key).Update("revoked_at", now).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke API key"})
				return
			}
			auditService.Log(c, models.ActionDelete, models.ResourceAPIKey, &key.ID, map[string]interface{}{
				"name": key.Name,
			}, true)
		}

		c.JSON(http.StatusOK, gin.H{"message": "API key revoked"})
	}
}
