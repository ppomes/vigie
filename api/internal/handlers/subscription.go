package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"

	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SubscribeRequest represents the request to subscribe for status updates
type SubscribeRequest struct {
	Email  string `json:"email" binding:"required,email"`
	Locale string `json:"locale"` // Preferred language: en, fr (default: en)
	// ComponentIDs limits notifications to these components. Empty means
	// every component.
	ComponentIDs []uint `json:"component_ids"`
}

// SubscribeResponse represents the successful subscription response
type SubscribeResponse struct {
	Message string `json:"message"`
	Email   string `json:"email"`
}

// Subscribe handles email subscription requests
// POST /api/v1/subscribe
func Subscribe(db *gorm.DB, emailService services.EmailService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SubscribeRequest

		// Bind and validate request JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Normalize email to lowercase
		email := strings.ToLower(strings.TrimSpace(req.Email))

		componentIDs, msg, err := publicComponentIDs(db, req.ComponentIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify components"})
			return
		}
		if msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}

		// Validate and default locale
		locale := strings.ToLower(strings.TrimSpace(req.Locale))
		if locale != "en" && locale != "fr" {
			locale = "en"
		}

		// Check if subscriber already exists (including soft-deleted)
		var existingSubscriber models.Subscriber
		err = db.Unscoped().Where("email = ?", email).First(&existingSubscriber).Error

		if err == nil {
			// Subscriber exists
			if existingSubscriber.DeletedAt.Valid {
				// Subscriber was soft-deleted, restore and re-send confirmation
				existingSubscriber.DeletedAt = gorm.DeletedAt{}
				existingSubscriber.Confirmed = false
				existingSubscriber.Locale = locale

				// Generate new tokens
				confirmToken, _ := generateRandomToken(32)
				unsubToken, _ := generateRandomToken(32)
				existingSubscriber.ConfirmationToken = confirmToken
				existingSubscriber.UnsubscribeToken = unsubToken

				err := db.Transaction(func(tx *gorm.DB) error {
					if err := tx.Save(&existingSubscriber).Error; err != nil {
						return err
					}
					return setSubscriptions(tx, &existingSubscriber, componentIDs)
				})
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reactivate subscription"})
					return
				}

				// Send confirmation email (async, non-blocking)
				if emailService != nil {
					go func() {
						if err := emailService.SendConfirmationEmail(existingSubscriber.Email, existingSubscriber.ConfirmationToken); err != nil {
							log.Printf("Failed to send confirmation email to %s: %v", existingSubscriber.Email, err)
						}
					}()
				}

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

			// Exists but not confirmed: take the new choice of components, and
			// resend the confirmation email (async, non-blocking)
			if err := db.Transaction(func(tx *gorm.DB) error {
				return setSubscriptions(tx, &existingSubscriber, componentIDs)
			}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subscription"})
				return
			}
			if emailService != nil {
				go func() {
					if err := emailService.SendConfirmationEmail(existingSubscriber.Email, existingSubscriber.ConfirmationToken); err != nil {
						log.Printf("Failed to resend confirmation email to %s: %v", existingSubscriber.Email, err)
					}
				}()
			}

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
		confirmationToken, err := generateRandomToken(32)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate confirmation token"})
			return
		}

		unsubscribeToken, err := generateRandomToken(32)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate unsubscribe token"})
			return
		}

		subscriber := models.Subscriber{
			Email:             email,
			Locale:            locale,
			Confirmed:         false,
			ConfirmationToken: confirmationToken,
			UnsubscribeToken:  unsubscribeToken,
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&subscriber).Error; err != nil {
				return err
			}
			return setSubscriptions(tx, &subscriber, componentIDs)
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subscription"})
			return
		}

		// Send confirmation email (async, non-blocking)
		if emailService != nil {
			go func() {
				if err := emailService.SendConfirmationEmail(subscriber.Email, subscriber.ConfirmationToken); err != nil {
					log.Printf("Failed to send confirmation email to %s: %v", subscriber.Email, err)
				}
			}()
		}

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

// generateRandomToken generates a cryptographically secure random token
// Used for confirmation and unsubscribe tokens
func generateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// publicComponentIDs deduplicates requested component ids and checks that
// each is shown on the public page. It returns an error message for the
// client, or "".
func publicComponentIDs(db *gorm.DB, requested []uint) ([]uint, string, error) {
	ids := []uint{}
	seen := map[uint]bool{}
	for _, id := range requested {
		if id != 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return ids, "", nil
	}
	var count int64
	if err := db.Model(&models.Component{}).Scopes(PublicComponents).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return nil, "", err
	}
	if int(count) != len(ids) {
		return nil, "Component not found", nil
	}
	return ids, "", nil
}

// setSubscriptions makes a subscriber follow exactly componentIDs. An empty
// list makes the subscriber global.
func setSubscriptions(tx *gorm.DB, subscriber *models.Subscriber, componentIDs []uint) error {
	if err := tx.Where("subscriber_id = ?", subscriber.ID).Delete(&models.Subscription{}).Error; err != nil {
		return err
	}
	for _, id := range componentIDs {
		if err := tx.Create(&models.Subscription{SubscriberID: subscriber.ID, ComponentID: id}).Error; err != nil {
			return err
		}
	}
	subscriber.Global = len(componentIDs) == 0
	return tx.Model(subscriber).Update("global", subscriber.Global).Error
}

// SubscriptionChoice is what a subscriber follows.
type SubscriptionChoice struct {
	Email        string `json:"email,omitempty"`
	Global       bool   `json:"global"`
	ComponentIDs []uint `json:"component_ids"`
}

// findSubscriberByToken loads the subscriber behind the token sent in each
// notification email. It writes the error response and returns false when
// there is none.
func findSubscriberByToken(c *gin.Context, db *gorm.DB) (models.Subscriber, bool) {
	var subscriber models.Subscriber
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return subscriber, false
	}
	if err := db.Where("unsubscribe_token = ?", token).First(&subscriber).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invalid token"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscription"})
		}
		return subscriber, false
	}
	return subscriber, true
}

func subscriptionChoice(db *gorm.DB, subscriber models.Subscriber) (SubscriptionChoice, error) {
	choice := SubscriptionChoice{Email: subscriber.Email, Global: subscriber.Global, ComponentIDs: []uint{}}
	if !subscriber.Global {
		err := db.Model(&models.Subscription{}).Where("subscriber_id = ?", subscriber.ID).
			Order("component_id").Pluck("component_id", &choice.ComponentIDs).Error
		return choice, err
	}
	return choice, nil
}

// GetSubscription returns what a subscriber follows.
// GET /api/v1/subscriptions?token=...
func GetSubscription(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subscriber, ok := findSubscriberByToken(c, db)
		if !ok {
			return
		}
		choice, err := subscriptionChoice(db, subscriber)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscription"})
			return
		}
		c.JSON(http.StatusOK, choice)
	}
}

// UpdateSubscription changes what a subscriber follows: every component, or
// a list of public components. Following nothing is unsubscribing, which has
// its own link.
// PUT /api/v1/subscriptions?token=...
func UpdateSubscription(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subscriber, ok := findSubscriberByToken(c, db)
		if !ok {
			return
		}
		var req SubscriptionChoice
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}
		ids := []uint{}
		if !req.Global {
			var msg string
			var err error
			ids, msg, err = publicComponentIDs(db, req.ComponentIDs)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify components"})
				return
			}
			if msg != "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": msg})
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Choose at least one component, or every component"})
				return
			}
		}
		if err := db.Transaction(func(tx *gorm.DB) error { return setSubscriptions(tx, &subscriber, ids) }); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save subscription"})
			return
		}
		choice, err := subscriptionChoice(db, subscriber)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscription"})
			return
		}
		c.JSON(http.StatusOK, choice)
	}
}
