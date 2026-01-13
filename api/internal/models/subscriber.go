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
	Confirmed         bool           `gorm:"default:false" json:"confirmed"`
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
