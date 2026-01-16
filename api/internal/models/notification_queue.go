package models

import (
	"time"
)

// NotificationStatus represents the status of a queued notification
type NotificationStatus string

const (
	NotificationPending    NotificationStatus = "pending"
	NotificationProcessing NotificationStatus = "processing"
	NotificationSent       NotificationStatus = "sent"
	NotificationFailed     NotificationStatus = "failed"
)

// NotificationQueue represents a queued email notification
type NotificationQueue struct {
	ID           uint               `gorm:"primaryKey" json:"id"`
	IncidentID   uint               `gorm:"index" json:"incident_id"`
	SubscriberID uint               `gorm:"index" json:"subscriber_id"`
	Email        string             `gorm:"size:255;not null" json:"email"`
	Locale       string             `gorm:"size:5;not null;default:'en'" json:"locale"`
	Subject      string             `gorm:"size:500" json:"subject"`
	Body         string             `gorm:"type:text" json:"body"`
	Status       NotificationStatus `gorm:"size:20;not null;default:'pending';index" json:"status"`
	Attempts     int                `gorm:"default:0" json:"attempts"`
	MaxAttempts  int                `gorm:"default:3" json:"max_attempts"`
	LastError    string             `gorm:"type:text" json:"last_error,omitempty"`
	NextRetryAt  *time.Time         `gorm:"index" json:"next_retry_at,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	ProcessedAt  *time.Time         `json:"processed_at,omitempty"`
	SentAt       *time.Time         `json:"sent_at,omitempty"`
}

// TableName specifies the table name for GORM
func (NotificationQueue) TableName() string {
	return "notification_queue"
}

// NotificationQueueStats represents queue statistics
type NotificationQueueStats struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Sent       int64 `json:"sent"`
	Failed     int64 `json:"failed"`
	Total      int64 `json:"total"`
}
