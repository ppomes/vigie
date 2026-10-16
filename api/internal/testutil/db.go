// Package testutil provides helpers shared by tests. It is only imported from
// _test.go files, so it never ends up in the API binary.
package testutil

import (
	"fmt"
	"sync/atomic"
	"testing"

	"api/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var dbCounter atomic.Int64

// NewDB returns an empty in-memory SQLite database with the models migrated.
// Each call gets its own database. SQLite is used so tests need no MySQL
// server; queries under test must therefore stay portable.
func NewDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:test%d?mode=memory&cache=shared", dbCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	// A single connection keeps the in-memory database alive and avoids
	// SQLite locking errors between transactions.
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	if err := db.AutoMigrate(
		&models.Component{},
		&models.Incident{},
		&models.Maintenance{},
		&models.Translation{},
	); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return db
}

// CreateComponent inserts a component with the given status.
func CreateComponent(t *testing.T, db *gorm.DB, name, status string) models.Component {
	t.Helper()
	c := models.Component{Name: name, Status: status}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	return c
}

// ComponentStatus reads the current status of a component.
func ComponentStatus(t *testing.T, db *gorm.DB, id uint) string {
	t.Helper()
	var c models.Component
	if err := db.First(&c, id).Error; err != nil {
		t.Fatalf("read component %d: %v", id, err)
	}
	return c.Status
}
