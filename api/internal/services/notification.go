package services

import (
	"api/internal/models"
	"fmt"
	"log"

	"gorm.io/gorm"
)

// NotificationService handles sending notifications to subscribers
type NotificationService struct {
	DB           *gorm.DB
	EmailService EmailService
}

// NewNotificationService creates a new notification service
func NewNotificationService(db *gorm.DB, emailService EmailService) *NotificationService {
	return &NotificationService{
		DB:           db,
		EmailService: emailService,
	}
}

// NotifyIncidentCreated sends notifications to all confirmed subscribers when an incident is created
func (n *NotificationService) NotifyIncidentCreated(incident *models.Incident) error {
	return n.notifySubscribers(incident, "created")
}

// NotifyIncidentUpdated sends notifications to all confirmed subscribers when an incident is updated
func (n *NotificationService) NotifyIncidentUpdated(incident *models.Incident) error {
	return n.notifySubscribers(incident, "updated")
}

// notifySubscribers sends notification emails to all confirmed subscribers
func (n *NotificationService) notifySubscribers(incident *models.Incident, eventType string) error {
	// Get all confirmed subscribers
	var subscribers []models.Subscriber
	if err := n.DB.Where("confirmed = ?", true).Find(&subscribers).Error; err != nil {
		return fmt.Errorf("failed to fetch subscribers: %w", err)
	}

	// If no subscribers, return early
	if len(subscribers) == 0 {
		log.Printf("No confirmed subscribers to notify for incident %d", incident.ID)
		return nil
	}

	// Load incident translations and component for email content
	if err := n.DB.Preload("Translations").Preload("Component").First(incident, incident.ID).Error; err != nil {
		return fmt.Errorf("failed to load incident details: %w", err)
	}

	// Extract incident message from translations (default to English)
	incidentMessage := n.getTranslation(incident.Translations, "message", "en")
	if incidentMessage == "" {
		incidentMessage = "No additional details provided."
	}

	// Send notification to each subscriber
	successCount := 0
	errorCount := 0

	for _, subscriber := range subscribers {
		err := n.EmailService.SendIncidentNotification(
			subscriber.Email,
			incident.Name,
			incident.Status,
			incidentMessage,
			subscriber.UnsubscribeToken,
		)

		if err != nil {
			log.Printf("Failed to send incident notification to %s: %v", subscriber.Email, err)
			errorCount++
		} else {
			successCount++
		}
	}

	log.Printf("Incident %d notification sent: %d succeeded, %d failed (event: %s)",
		incident.ID, successCount, errorCount, eventType)

	// Return error if all notifications failed
	if errorCount > 0 && successCount == 0 {
		return fmt.Errorf("all notification emails failed to send")
	}

	return nil
}

// getTranslation retrieves a translation value for a specific field and locale
func (n *NotificationService) getTranslation(translations []models.Translation, field, locale string) string {
	for _, t := range translations {
		if t.Field == field && t.Locale == locale {
			return t.Value
		}
	}
	return ""
}
