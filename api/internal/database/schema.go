package database

import (
	"fmt"
	"strings"

	"api/internal/models"

	"gorm.io/gorm"
)

// EnsureSchema applies schema changes that AutoMigrate cannot make. It is
// idempotent and runs at startup, after AutoMigrate.
func EnsureSchema(db *gorm.DB) error {
	if db.Dialector.Name() != "mysql" {
		return nil
	}
	return ensureTranslationEntityCheck(db)
}

// ensureTranslationEntityCheck widens the chk_translation_entity constraint,
// created by api/migrations/001_schema.sql, when new entity types gain
// translations. Databases created by AutoMigrate alone have no such
// constraint and are left alone.
func ensureTranslationEntityCheck(db *gorm.DB) error {
	var clause string
	err := db.Raw(`SELECT CHECK_CLAUSE FROM information_schema.CHECK_CONSTRAINTS
		WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'chk_translation_entity'`).
		Scan(&clause).Error
	if err != nil {
		return fmt.Errorf("read translation constraint: %w", err)
	}
	if clause == "" {
		return nil
	}

	if checkAllowsAll(clause, models.TranslationEntityTypes) {
		return nil
	}

	quoted := make([]string, len(models.TranslationEntityTypes))
	for i, t := range models.TranslationEntityTypes {
		quoted[i] = "'" + t + "'"
	}
	stmt := fmt.Sprintf("ALTER TABLE translations DROP CHECK chk_translation_entity, "+
		"ADD CONSTRAINT chk_translation_entity CHECK (entity_type IN (%s))", strings.Join(quoted, ", "))
	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("update translation constraint: %w", err)
	}
	return nil
}

// checkAllowsAll reports whether a CHECK clause, as MySQL stores it, lists all
// the given values. MySQL escapes the quotes, as in
// (`entity_type` in (_utf8mb4\'component\',_utf8mb4\'incident\')).
func checkAllowsAll(clause string, values []string) bool {
	clause = strings.ReplaceAll(clause, `\'`, "'")
	for _, v := range values {
		if !strings.Contains(clause, "'"+v+"'") {
			return false
		}
	}
	return true
}
