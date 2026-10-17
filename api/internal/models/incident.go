package models

import (
	"time"

	"gorm.io/gorm"
)

// Incident represents a status incident or maintenance event
// Supports multi-language descriptions via Translation table
type Incident struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:255;not null" json:"name"`
	Status       string         `gorm:"size:50;not null;default:'investigating'" json:"status"` // investigating, identified, monitoring, resolved
	// ComponentID is the main affected component, or nil for a general
	// incident that affects no component.
	ComponentID *uint `gorm:"index" json:"component_id"`
	// Visible is false for internal incidents, kept off the public page.
	Visible bool `gorm:"not null;default:1" json:"visible"`
	// Stickied incidents are pinned at the top of the public page.
	Stickied bool `gorm:"not null;default:0" json:"stickied"`
	// OccurredAt is when the incident started, which can be before it was
	// posted. It defaults to the creation time.
	OccurredAt *time.Time `gorm:"index" json:"occurred_at,omitempty"`
	// Notifications records whether subscribers were notified of it.
	Notifications bool `gorm:"not null;default:1" json:"notifications"`
	// UserID is the author.
	UserID *uint `gorm:"index" json:"user_id,omitempty"`
	Component    *Component     `gorm:"foreignKey:ComponentID" json:"component,omitempty"`
	Translations []Translation  `gorm:"polymorphic:Entity;polymorphicValue:incident" json:"translations,omitempty"`
	// Components lists every affected component, including ComponentID.
	Components []IncidentComponent `gorm:"foreignKey:IncidentID" json:"components,omitempty"`
	// Updates is the incident timeline, oldest first unless a query orders it.
	Updates []IncidentUpdate `gorm:"foreignKey:IncidentID" json:"updates,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Incident model
func (Incident) TableName() string {
	return "incidents"
}
