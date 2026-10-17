package services

import (
	"api/internal/models"
	"context"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// NotificationService handles sending notifications to subscribers
type NotificationService struct {
	DB           *gorm.DB
	EmailService EmailService
	workerCtx    context.Context
	workerCancel context.CancelFunc
	workerWg     sync.WaitGroup
	batchSize    int
	workerCount  int
}

// NewNotificationService creates a new notification service
func NewNotificationService(db *gorm.DB, emailService EmailService) *NotificationService {
	return &NotificationService{
		DB:           db,
		EmailService: emailService,
		batchSize:    50,  // Process 50 notifications at a time
		workerCount:  3,   // 3 concurrent workers
	}
}

// StartWorker starts the background notification processing workers
func (n *NotificationService) StartWorker() {
	// Don't start workers if email service is not configured
	if n.EmailService == nil {
		log.Println("Email service not configured - notification workers not started")
		return
	}

	n.workerCtx, n.workerCancel = context.WithCancel(context.Background())

	for i := 0; i < n.workerCount; i++ {
		n.workerWg.Add(1)
		go n.processQueue(i)
	}

	log.Printf("Started %d notification queue workers", n.workerCount)
}

// StopWorker gracefully stops the background workers
func (n *NotificationService) StopWorker() {
	if n.workerCancel != nil {
		n.workerCancel()
		n.workerWg.Wait()
		log.Println("Notification queue workers stopped")
	}
}

// QueueIncidentNotifications queues notifications for all confirmed subscribers
// Returns immediately - notifications are processed asynchronously
func (n *NotificationService) QueueIncidentNotifications(incident *models.Incident, eventType string) (int, error) {
	return n.queueIncident(incident, nil, eventType)
}

// QueueIncidentUpdateNotifications queues notifications for a new update in an
// incident's timeline. The email carries the update status and message.
func (n *NotificationService) QueueIncidentUpdateNotifications(incident *models.Incident, update *models.IncidentUpdate) (int, error) {
	if err := n.DB.Preload("Translations").First(update, update.ID).Error; err != nil {
		return 0, fmt.Errorf("failed to load incident update: %w", err)
	}
	return n.queueIncident(incident, update, "update")
}

// queueIncident queues one notification per confirmed subscriber. When update
// is set, its status and message replace the incident's.
func (n *NotificationService) queueIncident(incident *models.Incident, update *models.IncidentUpdate, eventType string) (int, error) {
	// Confirmed subscribers who follow one of the affected components
	componentIDs, err := n.affectedComponentIDs(&models.IncidentComponent{}, "incident_id", incident.ID, incident.ComponentID)
	if err != nil {
		return 0, err
	}
	subscribers, err := n.subscribersFor(componentIDs)
	if err != nil {
		return 0, err
	}

	if len(subscribers) == 0 {
		log.Printf("No confirmed subscribers to notify for incident %d", incident.ID)
		return 0, nil
	}

	// Load incident translations and component for email content
	if err := n.DB.Preload("Translations").Preload("Component").First(incident, incident.ID).Error; err != nil {
		return 0, fmt.Errorf("failed to load incident details: %w", err)
	}

	// Queue notifications for each subscriber
	now := time.Now()
	queued := 0

	for _, subscriber := range subscribers {
		locale := subscriber.Locale
		if locale == "" {
			locale = "en"
		}

		// Get localized content
		incidentName := n.getTranslationWithFallback(incident.Translations, "name", locale, incident.Name)
		incidentMessage := n.getTranslationWithFallback(incident.Translations, "message", locale, "")
		status := incident.Status
		if update != nil {
			incidentMessage = n.getTranslationWithFallback(update.Translations, "message", locale, "")
			status = update.Status
		}
		if incidentMessage == "" {
			incidentMessage = "No additional details provided."
		}

		// Build email subject and body
		subject := fmt.Sprintf("[%s] %s", status, incidentName)
		body := n.buildEmailBody(incidentName, status, incidentMessage, subscriber.UnsubscribeToken)

		notification := models.NotificationQueue{
			IncidentID:   incident.ID,
			SubscriberID: subscriber.ID,
			Email:        subscriber.Email,
			Locale:       locale,
			Subject:      subject,
			Body:         body,
			Status:       models.NotificationPending,
			MaxAttempts:  3,
			NextRetryAt:  &now,
		}

		if err := n.DB.Create(&notification).Error; err != nil {
			log.Printf("Failed to queue notification for %s: %v", subscriber.Email, err)
			continue
		}
		queued++
	}

	log.Printf("Queued %d notifications for incident %d (%s)", queued, incident.ID, eventType)
	return queued, nil
}

// affectedComponentIDs reads the components of an incident or maintenance
// from its join table (model, keyed by column), falling back to its main
// component.
func (n *NotificationService) affectedComponentIDs(model any, column string, id, mainComponentID uint) ([]uint, error) {
	var ids []uint
	if err := n.DB.Model(model).Where(column+" = ?", id).Pluck("component_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch affected components: %w", err)
	}
	if len(ids) == 0 && mainComponentID != 0 {
		ids = []uint{mainComponentID}
	}
	return ids, nil
}

// subscribersFor returns the confirmed subscribers who follow at least one
// of the components: global subscribers, and those subscribed to one of them.
func (n *NotificationService) subscribersFor(componentIDs []uint) ([]models.Subscriber, error) {
	q := n.DB.Where("confirmed = ?", true)
	if len(componentIDs) > 0 {
		followers := n.DB.Model(&models.Subscription{}).Select("subscriber_id").Where("component_id IN ?", componentIDs)
		q = q.Where("global = ? OR id IN (?)", true, followers)
	} else {
		q = q.Where("global = ?", true)
	}
	var subscribers []models.Subscriber
	if err := q.Find(&subscribers).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch subscribers: %w", err)
	}
	return subscribers, nil
}

// subscriptionFooter ends every notification with links to choose the
// components to follow, on the public page, and to unsubscribe, on the API.
// PUBLIC_URL is the public status page; API_URL is where readers reach the
// API, as for confirmation emails.
func subscriptionFooter(unsubscribeToken string) string {
	publicURL := strings.TrimRight(os.Getenv("PUBLIC_URL"), "/")
	if publicURL == "" {
		publicURL = "http://localhost:3000"
	}
	apiURL := strings.TrimRight(os.Getenv("API_URL"), "/")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	token := url.QueryEscape(unsubscribeToken)
	return fmt.Sprintf(`---
To choose which components you hear about:
%s/?manage=%s

To unsubscribe from these notifications:
%s/api/v1/unsubscribe?token=%s`, publicURL, token, apiURL, token)
}

// buildEmailBody creates the email body content
func (n *NotificationService) buildEmailBody(name, status, message, unsubscribeToken string) string {
	return fmt.Sprintf(`Incident: %s
Status: %s

%s

%s`,
		name, status, message, subscriptionFooter(unsubscribeToken))
}

// processQueue is the main worker loop that processes pending notifications
func (n *NotificationService) processQueue(workerID int) {
	defer n.workerWg.Done()

	ticker := time.NewTicker(2 * time.Second) // Check queue every 2 seconds
	defer ticker.Stop()

	log.Printf("Notification worker %d started", workerID)

	for {
		select {
		case <-n.workerCtx.Done():
			log.Printf("Notification worker %d stopping", workerID)
			return
		case <-ticker.C:
			n.processBatch(workerID)
		}
	}
}

// processBatch processes a batch of pending notifications
func (n *NotificationService) processBatch(workerID int) {
	now := time.Now()

	// Claim a batch of notifications (atomic update to prevent double processing)
	var notifications []models.NotificationQueue

	// Use a transaction to atomically claim notifications
	err := n.DB.Transaction(func(tx *gorm.DB) error {
		// Find pending notifications ready for processing
		if err := tx.Where("status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)",
			models.NotificationPending, now).
			Limit(n.batchSize).
			Find(&notifications).Error; err != nil {
			return err
		}

		if len(notifications) == 0 {
			return nil
		}

		// Claim them by setting status to processing
		ids := make([]uint, len(notifications))
		for i, notif := range notifications {
			ids[i] = notif.ID
		}

		return tx.Model(&models.NotificationQueue{}).
			Where("id IN ?", ids).
			Update("status", models.NotificationProcessing).Error
	})

	if err != nil {
		log.Printf("Worker %d: Error claiming notifications: %v", workerID, err)
		return
	}

	if len(notifications) == 0 {
		return
	}

	log.Printf("Worker %d: Processing %d notifications", workerID, len(notifications))

	// Process each notification
	for _, notif := range notifications {
		n.sendQueuedNotification(&notif)
	}
}

// sendQueuedNotification attempts to send a single queued notification
func (n *NotificationService) sendQueuedNotification(notif *models.NotificationQueue) {
	now := time.Now()
	notif.Attempts++
	notif.ProcessedAt = &now

	// Send the email using the raw send method
	err := n.EmailService.SendRawEmail(notif.Email, notif.Subject, notif.Body)

	if err != nil {
		// Handle failure
		notif.LastError = err.Error()

		if notif.Attempts >= notif.MaxAttempts {
			// Max retries reached - mark as failed
			notif.Status = models.NotificationFailed
			log.Printf("Notification %d failed permanently after %d attempts: %v",
				notif.ID, notif.Attempts, err)
		} else {
			// Schedule retry with exponential backoff
			backoff := time.Duration(math.Pow(2, float64(notif.Attempts))) * time.Minute
			nextRetry := now.Add(backoff)
			notif.NextRetryAt = &nextRetry
			notif.Status = models.NotificationPending
			log.Printf("Notification %d failed (attempt %d/%d), retry at %v: %v",
				notif.ID, notif.Attempts, notif.MaxAttempts, nextRetry.Format(time.RFC3339), err)
		}
	} else {
		// Success
		notif.Status = models.NotificationSent
		notif.SentAt = &now
		notif.LastError = ""
		log.Printf("Notification %d sent successfully to %s", notif.ID, notif.Email)
	}

	// Update the notification in the database
	if err := n.DB.Save(notif).Error; err != nil {
		log.Printf("Failed to update notification %d status: %v", notif.ID, err)
	}
}

// GetQueueStats returns statistics about the notification queue
func (n *NotificationService) GetQueueStats() (*models.NotificationQueueStats, error) {
	stats := &models.NotificationQueueStats{}

	var results []struct {
		Status string
		Count  int64
	}

	if err := n.DB.Model(&models.NotificationQueue{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Find(&results).Error; err != nil {
		return nil, err
	}

	for _, r := range results {
		switch models.NotificationStatus(r.Status) {
		case models.NotificationPending:
			stats.Pending = r.Count
		case models.NotificationProcessing:
			stats.Processing = r.Count
		case models.NotificationSent:
			stats.Sent = r.Count
		case models.NotificationFailed:
			stats.Failed = r.Count
		}
		stats.Total += r.Count
	}

	return stats, nil
}

// GetRecentNotifications returns recent notifications for monitoring
func (n *NotificationService) GetRecentNotifications(limit int) ([]models.NotificationQueue, error) {
	var notifications []models.NotificationQueue
	if err := n.DB.Order("created_at DESC").Limit(limit).Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

// RetryFailedNotifications resets failed notifications to pending for retry
func (n *NotificationService) RetryFailedNotifications() (int64, error) {
	now := time.Now()
	result := n.DB.Model(&models.NotificationQueue{}).
		Where("status = ?", models.NotificationFailed).
		Updates(map[string]interface{}{
			"status":       models.NotificationPending,
			"attempts":     0,
			"next_retry_at": now,
			"last_error":   "",
		})

	return result.RowsAffected, result.Error
}

// --- Legacy synchronous methods (kept for backward compatibility) ---

// NotifyIncidentCreated sends notifications synchronously (deprecated - use QueueIncidentNotifications)
func (n *NotificationService) NotifyIncidentCreated(incident *models.Incident) error {
	_, err := n.QueueIncidentNotifications(incident, "created")
	return err
}

// NotifyIncidentUpdated sends notifications synchronously (deprecated - use QueueIncidentNotifications)
func (n *NotificationService) NotifyIncidentUpdated(incident *models.Incident) error {
	_, err := n.QueueIncidentNotifications(incident, "updated")
	return err
}

// NotifyMaintenanceScheduled sends notifications when a maintenance is scheduled
func (n *NotificationService) NotifyMaintenanceScheduled(maintenance *models.Maintenance) error {
	_, err := n.QueueMaintenanceNotifications(maintenance, "scheduled")
	return err
}

// NotifyMaintenanceUpdated sends notifications when a maintenance is updated
func (n *NotificationService) NotifyMaintenanceUpdated(maintenance *models.Maintenance) error {
	_, err := n.QueueMaintenanceNotifications(maintenance, "updated")
	return err
}

// QueueMaintenanceNotifications queues notifications for all confirmed subscribers about a maintenance
func (n *NotificationService) QueueMaintenanceNotifications(maintenance *models.Maintenance, eventType string) (int, error) {
	// Confirmed subscribers who follow one of the covered components
	componentIDs, err := n.affectedComponentIDs(&models.MaintenanceComponent{}, "maintenance_id", maintenance.ID, maintenance.ComponentID)
	if err != nil {
		return 0, err
	}
	subscribers, err := n.subscribersFor(componentIDs)
	if err != nil {
		return 0, err
	}

	if len(subscribers) == 0 {
		log.Printf("No confirmed subscribers to notify for maintenance %d", maintenance.ID)
		return 0, nil
	}

	// Load maintenance translations and component
	if err := n.DB.Preload("Translations").Preload("Component").First(maintenance, maintenance.ID).Error; err != nil {
		return 0, fmt.Errorf("failed to load maintenance details: %w", err)
	}

	// Queue notifications for each subscriber
	now := time.Now()
	queued := 0

	for _, subscriber := range subscribers {
		locale := subscriber.Locale
		if locale == "" {
			locale = "en"
		}

		// Get localized content
		maintenanceName := n.getTranslationWithFallback(maintenance.Translations, "name", locale, maintenance.Name)
		maintenanceMessage := n.getTranslationWithFallback(maintenance.Translations, "message", locale, "")
		if maintenanceMessage == "" {
			maintenanceMessage = "No additional details provided."
		}

		// Build email subject and body
		var subject string
		if eventType == "scheduled" {
			subject = fmt.Sprintf("[Maintenance Scheduled] %s", maintenanceName)
		} else {
			subject = fmt.Sprintf("[Maintenance Update] %s", maintenanceName)
		}
		body := n.buildMaintenanceEmailBody(maintenanceName, maintenance.Status, maintenanceMessage,
			maintenance.StartAt, maintenance.EndAt, subscriber.UnsubscribeToken)

		notification := models.NotificationQueue{
			IncidentID:   0, // Not an incident
			SubscriberID: subscriber.ID,
			Email:        subscriber.Email,
			Locale:       locale,
			Subject:      subject,
			Body:         body,
			Status:       models.NotificationPending,
			MaxAttempts:  3,
			NextRetryAt:  &now,
		}

		if err := n.DB.Create(&notification).Error; err != nil {
			log.Printf("Failed to queue notification for %s: %v", subscriber.Email, err)
			continue
		}
		queued++
	}

	log.Printf("Queued %d notifications for maintenance %d (%s)", queued, maintenance.ID, eventType)
	return queued, nil
}

// buildMaintenanceEmailBody creates the email body content for maintenance notifications
func (n *NotificationService) buildMaintenanceEmailBody(name, status, message string, startAt, endAt time.Time, unsubscribeToken string) string {
	return fmt.Sprintf(`Scheduled Maintenance: %s
Status: %s

%s

Start: %s
End: %s

%s`,
		name, status, message,
		startAt.Format("2006-01-02 15:04 MST"),
		endAt.Format("2006-01-02 15:04 MST"),
		subscriptionFooter(unsubscribeToken))
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

// getTranslationWithFallback retrieves a translation with fallback chain: locale -> English -> default
func (n *NotificationService) getTranslationWithFallback(translations []models.Translation, field, locale, defaultValue string) string {
	// Try requested locale first
	if value := n.getTranslation(translations, field, locale); value != "" {
		return value
	}
	// Fall back to English if different from requested locale
	if locale != "en" {
		if value := n.getTranslation(translations, field, "en"); value != "" {
			return value
		}
	}
	// Return default value
	return defaultValue
}
