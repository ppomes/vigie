package handlers

import (
	"strings"

	"api/internal/models"

	"gorm.io/gorm"
)

// replaceTranslations replaces the translations of an entity. A translation
// made by a model at import stays marked as such while its value is left
// unchanged, so that saving a form for another reason keeps the mark;
// editing it, or sending it with reviewed set, makes it a reviewed one.
// With skipEmpty, empty values are not stored.
func replaceTranslations(tx *gorm.DB, entityType string, id uint, translations []TranslationRequest, skipEmpty bool) error {
	var machine []models.Translation
	if err := tx.Where("entity_id = ? AND entity_type = ? AND machine = ?", id, entityType, true).Find(&machine).Error; err != nil {
		return err
	}
	unchanged := map[string]string{}
	for _, t := range machine {
		unchanged[t.Locale+"\x00"+t.Field] = t.Value
	}
	if err := tx.Unscoped().Where("entity_id = ? AND entity_type = ?", id, entityType).
		Delete(&models.Translation{}).Error; err != nil {
		return err
	}
	for _, t := range translations {
		if skipEmpty && strings.TrimSpace(t.Value) == "" {
			continue
		}
		v, ok := unchanged[t.Locale+"\x00"+t.Field]
		tr := models.Translation{EntityID: id, EntityType: entityType, Locale: t.Locale, Field: t.Field, Value: t.Value,
			Machine: ok && v == t.Value && !t.Reviewed}
		if err := tx.Create(&tr).Error; err != nil {
			return err
		}
	}
	return nil
}
