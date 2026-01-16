package models

import (
	"time"

	"gorm.io/gorm"
)

// MaintenanceTemplate represents a reusable maintenance template with variable support
type MaintenanceTemplate struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	Name               string         `gorm:"size:255;not null" json:"name"`
	DefaultComponentID *uint          `json:"default_component_id,omitempty"`
	NameEN             string         `gorm:"type:text" json:"name_en"`
	NameFR             string         `gorm:"type:text" json:"name_fr"`
	MessageEN          string         `gorm:"type:text" json:"message_en"`
	MessageFR          string         `gorm:"type:text" json:"message_fr"`
	Variables          string         `gorm:"type:text" json:"variables"` // JSON array of TemplateVariable
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

// Note: TemplateVariable struct is already defined in incident_template.go
// and can be reused for maintenance templates
