package models

import (
	"gorm.io/gorm"
)

// Translation represents a multi-language content translation
// Supports French and English translations for various entities
type Translation struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	EntityID   uint           `gorm:"not null;index:idx_entity_locale" json:"entity_id"`
	EntityType string         `gorm:"column:entity_type;size:50;not null;index:idx_entity_locale" json:"entity_type"` // incident, component
	Locale     string         `gorm:"size:5;not null;index:idx_entity_locale" json:"locale"`                          // en, fr
	Field      string         `gorm:"size:50;not null" json:"field"`                                                  // name, message, description
	Value      string         `gorm:"type:text" json:"value"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TranslationEntityTypes lists the entity types that may own translations.
// The MySQL schema enforces it with the chk_translation_entity constraint,
// which database.EnsureSchema keeps in sync.
var TranslationEntityTypes = []string{"component", "incident", "maintenance", "incident_update", "component_group"}
