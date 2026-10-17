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
	if db.Dialector.Name() == "mysql" {
		if err := ensureTranslationEntityCheck(db); err != nil {
			return err
		}
		// General maintenances cover no component. 001_schema.sql created
		// the column NOT NULL, and AutoMigrate does not relax it.
		if err := ensureNullable(db, "maintenances", "component_id", "BIGINT UNSIGNED"); err != nil {
			return err
		}
	}
	return backfillAffectedComponents(db)
}

// backfillAffectedComponents gives incidents and maintenances created before
// incident_components and maintenance_components existed a row for their
// main component. A maintenance in progress keeps the status it recorded.
// Rows are only added where none exist, so this is idempotent.
func backfillAffectedComponents(db *gorm.DB) error {
	if err := db.Exec(`INSERT INTO incident_components (incident_id, component_id, component_status, created_at, updated_at)
		SELECT i.id, i.component_id, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP FROM incidents i
		WHERE i.component_id IS NOT NULL AND i.component_id <> 0
		AND NOT EXISTS (SELECT 1 FROM incident_components ic WHERE ic.incident_id = i.id)`).Error; err != nil {
		return fmt.Errorf("backfill incident components: %w", err)
	}
	if err := db.Exec(`INSERT INTO maintenance_components (maintenance_id, component_id, previous_status, created_at, updated_at)
		SELECT m.id, m.component_id, COALESCE(m.previous_component_status, ''), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP FROM maintenances m
		WHERE m.component_id IS NOT NULL AND m.component_id <> 0
		AND NOT EXISTS (SELECT 1 FROM maintenance_components mc WHERE mc.maintenance_id = m.id)`).Error; err != nil {
		return fmt.Errorf("backfill maintenance components: %w", err)
	}
	return nil
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

// ensureNullable makes a MySQL column nullable when it is not, keeping its
// type. Foreign keys on the column stay in place.
func ensureNullable(db *gorm.DB, table, column, columnType string) error {
	var nullable string
	err := db.Raw(`SELECT IS_NULLABLE FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, table, column).
		Scan(&nullable).Error
	if err != nil {
		return fmt.Errorf("read %s.%s: %w", table, column, err)
	}
	if nullable != "NO" {
		return nil
	}
	stmt := fmt.Sprintf("ALTER TABLE `%s` MODIFY `%s` %s NULL", table, column, columnType)
	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("make %s.%s nullable: %w", table, column, err)
	}
	return nil
}
