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
	ComponentID  uint           `gorm:"index" json:"component_id"`
	Component    Component      `gorm:"foreignKey:ComponentID" json:"component,omitempty"`
	Translations []Translation  `gorm:"foreignKey:EntityID;constraint:OnDelete:CASCADE" json:"translations,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Incident model
func (Incident) TableName() string {
	return "incidents"
}
