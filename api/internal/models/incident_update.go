package models

import (
	"time"

	"gorm.io/gorm"
)

// IncidentUpdate is one step in an incident's timeline, such as "identified"
// or "fix deployed". The incident status follows its latest update.
// The message is stored per locale in the translations table.
type IncidentUpdate struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	IncidentID   uint           `gorm:"not null;index" json:"incident_id"`
	Status       string         `gorm:"size:50;not null" json:"status"` // investigating, identified, monitoring, resolved
	UserID       *uint          `gorm:"index" json:"user_id,omitempty"`
	User         *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Translations []Translation  `gorm:"polymorphic:Entity;polymorphicValue:incident_update" json:"translations,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
