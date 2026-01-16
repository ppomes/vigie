package models

import (
	"time"

	"gorm.io/gorm"
)

// IncidentTemplate represents a reusable template for creating incidents
type IncidentTemplate struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Name            string         `gorm:"size:255;not null" json:"name"` // Admin-facing name
	DefaultStatus   string         `gorm:"size:50;default:'investigating'" json:"default_status"`
	DefaultComponentID *uint       `gorm:"index" json:"default_component_id,omitempty"`

	// Template content with {{variable}} placeholders
	NameEN      string `gorm:"size:500" json:"name_en"`
	NameFR      string `gorm:"size:500" json:"name_fr"`
	MessageEN   string `gorm:"type:text" json:"message_en"`
	MessageFR   string `gorm:"type:text" json:"message_fr"`

	// Variables definition as JSON: [{"name":"service_name","label_en":"Service","label_fr":"Service","type":"text"}]
	Variables   string `gorm:"type:text" json:"variables"`

	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for IncidentTemplate model
func (IncidentTemplate) TableName() string {
	return "incident_templates"
}

// TemplateVariable represents a variable definition within a template
type TemplateVariable struct {
	Name     string   `json:"name"`      // Variable name (used in {{name}})
	LabelEN  string   `json:"label_en"`  // Display label in English
	LabelFR  string   `json:"label_fr"`  // Display label in French
	Type     string   `json:"type"`      // text, number, datetime, select
	Options  []string `json:"options,omitempty"`  // For select type
	Default  string   `json:"default,omitempty"`  // Default value
	Required bool     `json:"required"`  // Whether the variable is required
}
