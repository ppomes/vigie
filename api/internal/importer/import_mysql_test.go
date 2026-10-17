package importer

import (
	"os"
	"strings"
	"testing"

	"api/internal/database"
	"api/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestImportFromMySQL imports a Cachet 2.4 MySQL database into a Vigie
// MySQL database. It runs when CACHET_MYSQL_DSN and VIGIE_MYSQL_DSN point at
// two empty databases, as in CI.
func TestImportFromMySQL(t *testing.T) {
	cachetDSN, vigieDSN := os.Getenv("CACHET_MYSQL_DSN"), os.Getenv("VIGIE_MYSQL_DSN")
	if cachetDSN == "" || vigieDSN == "" {
		t.Skip("set CACHET_MYSQL_DSN and VIGIE_MYSQL_DSN to run against MySQL")
	}
	open := func(dsn string) *gorm.DB {
		if !strings.Contains(dsn, "parseTime") {
			dsn += "?parseTime=true&multiStatements=false"
		}
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	src, dst := open(cachetDSN), open(vigieDSN)

	execSQLFile(t, src, "testdata/cachet24_schema_mysql.sql")
	execSQLFile(t, src, "testdata/cachet24_seed.sql")
	if err := database.Migrate(dst); err != nil {
		t.Fatal(err)
	}
	if err := dst.Create(&models.User{Username: "admin", Email: "admin@example.com", Password: "x", Role: models.RoleAdmin, Active: true}).Error; err != nil {
		t.Fatal(err)
	}

	report, err := Import(Options{Source: src, Target: dst})
	if err != nil {
		t.Fatal(err)
	}
	if report.Imported["incidents"] != 5 || report.Imported["components"] != 4 || report.Imported["subscribers"] != 4 {
		t.Fatalf("imported %v", report.Imported)
	}
	var billing models.Component
	dst.First(&billing, 3)
	if billing.Enabled {
		t.Fatal("a disabled component came in enabled")
	}
	var hidden models.Incident
	dst.First(&hidden, 3)
	if hidden.Visible {
		t.Fatal("a hidden incident came in visible")
	}
	var maint models.Maintenance
	if err := dst.First(&maint, 2).Error; err != nil || maint.ComponentID != nil {
		t.Fatalf("general maintenance: %+v, %v", maint, err)
	}
}
