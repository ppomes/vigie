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
	Machine    bool           `gorm:"not null;default:0" json:"machine"` // made by a model at import, until edited
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TranslationEntityTypes lists the entity types that may own translations.
// The MySQL schema enforces it with the chk_translation_entity constraint,
// which database.EnsureSchema keeps in sync.
var TranslationEntityTypes = []string{"component", "incident", "maintenance", "incident_update", "component_group"}

// TranslatedValue picks the text of field for locale: its translation in
// that locale, else own, the entity's own column, which holds the text in
// the language it was written in (French for content imported from a
// French Cachet), else the English translation. own comes before English
// so that a French page shows imported French text even once an English
// translation exists.
func TranslatedValue(translations []Translation, field, locale, own string) string {
	find := func(l string) string {
		for _, t := range translations {
			if t.Field == field && t.Locale == l && t.Value != "" {
				return t.Value
			}
		}
		return ""
	}
	if v := find(locale); v != "" {
		return v
	}
	if own != "" {
		return own
	}
	return find("en")
}
