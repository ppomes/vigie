package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SubscribeRequest represents the request to subscribe for status updates
type SubscribeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// SubscribeResponse represents the successful subscription response
type SubscribeResponse struct {
	Message string `json:"message"`
	Email   string `json:"email"`
}

// Subscribe handles email subscription requests
// POST /api/v1/subscribe
func Subscribe(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SubscribeRequest

		// Bind and validate request JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Normalize email to lowercase
		email := strings.ToLower(strings.TrimSpace(req.Email))

		// Check if subscriber already exists (including soft-deleted)
		var existingSubscriber models.Subscriber
		err := db.Unscoped().Where("email = ?", email).First(&existingSubscriber).Error

		if err == nil {
			// Subscriber exists
			if existingSubscriber.DeletedAt.Valid {
				// Subscriber was soft-deleted, restore and re-send confirmation
				existingSubscriber.DeletedAt = gorm.DeletedAt{}
				existingSubscriber.Confirmed = false

				// Generate new tokens
				confirmToken, _ := generateToken(32)
				unsubToken, _ := generateToken(32)
				existingSubscriber.ConfirmationToken = confirmToken
				existingSubscriber.UnsubscribeToken = unsubToken

				if err := db.Save(&existingSubscriber).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reactivate subscription"})
					return
				}

				// TODO: Send confirmation email when email service is implemented (subtask-7-2)
				// email.SendConfirmationEmail(existingSubscriber.Email, existingSubscriber.ConfirmationToken)

				c.JSON(http.StatusCreated, SubscribeResponse{
					Message: "Subscription reactivated. Please check your email to confirm.",
					Email:   email,
				})
				return
			}

			if existingSubscriber.Confirmed {
				// Already subscribed and confirmed
				c.JSON(http.StatusOK, gin.H{
					"message": "Email is already subscribed to status updates",
					"email":   email,
				})
				return
			}

			// Exists but not confirmed - resend confirmation email
			// TODO: Send confirmation email when email service is implemented (subtask-7-2)
			// email.SendConfirmationEmail(existingSubscriber.Email, existingSubscriber.ConfirmationToken)

			c.JSON(http.StatusOK, SubscribeResponse{
				Message: "Confirmation email has been resent. Please check your email.",
				Email:   email,
			})
			return
		} else if err != gorm.ErrRecordNotFound {
			// Database error
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check subscription status"})
			return
		}

		// Create new subscriber
		confirmationToken, err := generateToken(32)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate confirmation token"})
			return
		}

		unsubscribeToken, err := generateToken(32)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate unsubscribe token"})
			return
		}

		subscriber := models.Subscriber{
			Email:             email,
			Confirmed:         false,
			ConfirmationToken: confirmationToken,
			UnsubscribeToken:  unsubscribeToken,
		}

		if err := db.Create(&subscriber).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subscription"})
			return
		}

		// TODO: Send confirmation email when email service is implemented (subtask-7-2)
		// email.SendConfirmationEmail(subscriber.Email, subscriber.ConfirmationToken)

		c.JSON(http.StatusCreated, SubscribeResponse{
			Message: "Subscription created. Please check your email to confirm.",
			Email:   email,
		})
	}
}

// Confirm handles subscription confirmation via confirmation token
// GET /api/v1/confirm?token=<confirmation_token>
func Confirm(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")

		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Confirmation token is required"})
			return
		}

		// Find subscriber by confirmation token
		var subscriber models.Subscriber
		if err := db.Where("confirmation_token = ?", token).First(&subscriber).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired confirmation token"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm subscription"})
			}
			return
		}

		// Check if already confirmed
		if subscriber.Confirmed {
			c.JSON(http.StatusOK, gin.H{
				"message": "Email is already confirmed",
				"email":   subscriber.Email,
			})
			return
		}

		// Confirm subscription
		subscriber.Confirmed = true
		// Clear confirmation token after successful confirmation (security best practice)
		subscriber.ConfirmationToken = ""

		if err := db.Save(&subscriber).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subscription status"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Email successfully confirmed. You will now receive status updates.",
			"email":   subscriber.Email,
		})
	}
}

// Unsubscribe handles unsubscription via unsubscribe token
// GET /api/v1/unsubscribe?token=<unsubscribe_token>
func Unsubscribe(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")

		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsubscribe token is required"})
			return
		}

		// Find subscriber by unsubscribe token
		var subscriber models.Subscriber
		if err := db.Where("unsubscribe_token = ?", token).First(&subscriber).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Invalid unsubscribe token"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unsubscribe"})
			}
			return
		}

		// Soft delete the subscriber
		if err := db.Delete(&subscriber).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unsubscribe"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Successfully unsubscribed from status updates",
			"email":   subscriber.Email,
		})
	}
}

// generateToken generates a cryptographically secure random token
// Used for confirmation and unsubscribe tokens
func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
