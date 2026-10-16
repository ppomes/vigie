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
	// Link points readers to the service itself, or to its documentation.
	Link string `gorm:"size:1024" json:"link,omitempty"`
	// Enabled is false to hide a component from the public page without
	// deleting it.
	Enabled bool `gorm:"not null;default:true" json:"enabled"`
	// Meta is free key/value data for integrations, as in Cachet.
	Meta JSONMap `gorm:"type:text" json:"meta,omitempty"`
	Tags []Tag   `gorm:"many2many:component_tags" json:"tags,omitempty"`
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
