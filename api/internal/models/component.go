package models

import (
	"time"

	"gorm.io/gorm"
)

// Component represents a system component being monitored
// Examples: API, Website, Database
type Component struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:255;not null" json:"name"`
	Status       string         `gorm:"size:50;not null;default:'operational'" json:"status"` // operational, degraded_performance, partial_outage, major_outage
	Order        int            `gorm:"default:0" json:"order"`                                // Display order on status page
	GroupID      *uint           `gorm:"index" json:"group_id"`                                 // Optional component group
	Group        *ComponentGroup `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	Translations []Translation  `gorm:"polymorphic:Entity;polymorphicValue:component" json:"translations,omitempty"`
	Incidents    []Incident     `gorm:"foreignKey:ComponentID" json:"incidents,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Component model
func (Component) TableName() string {
	return "components"
}
