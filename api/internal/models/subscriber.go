package models

import (
	"time"

	"gorm.io/gorm"
)

// Subscriber represents an email subscriber for status notifications
// Supports subscription confirmation and unsubscribe functionality
type Subscriber struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	Email             string         `gorm:"size:255;not null;unique" json:"email"`
	Locale            string         `gorm:"size:5;not null;default:'en'" json:"locale"` // Preferred language: en, fr
	Confirmed         bool           `gorm:"default:0" json:"confirmed"`
	// Global subscribers hear about every component. Others only about the
	// components in Subscriptions.
	Global        bool           `gorm:"not null;default:1" json:"global"`
	Subscriptions []Subscription `gorm:"foreignKey:SubscriberID" json:"subscriptions,omitempty"`
	ConfirmationToken string         `gorm:"size:255;index" json:"-"` // Hidden from JSON for security
	UnsubscribeToken  string         `gorm:"size:255;unique;index" json:"-"` // Hidden from JSON for security
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Subscriber model
func (Subscriber) TableName() string {
	return "subscribers"
}

// Subscription is a component a non-global subscriber follows.
type Subscription struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SubscriberID uint      `gorm:"not null;uniqueIndex:idx_subscription" json:"subscriber_id"`
	ComponentID  uint      `gorm:"not null;uniqueIndex:idx_subscription;index" json:"component_id"`
	CreatedAt    time.Time `json:"-"`
}
