package importer

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Usage explains the import-cachet command.
const Usage = `Usage: api import-cachet --source-driver mysql|sqlite --source-dsn DSN [--dry-run] [--locale en|fr]

Copies a Cachet 2.4 installation into this Vigie database, which must be a
fresh installation. The Vigie database is the one the API is configured with
(DB_* variables). The Cachet database is only read.

  --source-driver  mysql or sqlite
  --source-dsn     MySQL: user:password@tcp(host:3306)/cachet
                   SQLite: path to the database file
  --dry-run        do everything in a transaction, print the report, roll back
  --locale         locale for Cachet's text (default: Cachet's app_locale, else en)
`

// openSource opens the Cachet database, read-only where the driver allows.
func openSource(driver, dsn string) (*gorm.DB, error) {
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	switch driver {
	case "mysql":
		if !strings.Contains(dsn, "parseTime=") {
			sep := "?"
			if strings.Contains(dsn, "?") {
				sep = "&"
			}
			dsn += sep + "parseTime=true"
		}
		return gorm.Open(mysql.Open(dsn), cfg)
	case "sqlite":
		return gorm.Open(sqlite.Open("file:"+dsn+"?mode=ro"), cfg)
	case "postgres", "pgsql":
		return nil, fmt.Errorf("PostgreSQL sources are not supported yet: export the Cachet database to MySQL or SQLite first")
	}
	return nil, fmt.Errorf("unknown source driver %q: use mysql or sqlite", driver)
}

// RunCLI runs the import-cachet command and returns the exit code. target
// opens and migrates the Vigie database.
func RunCLI(args []string, target func() (*gorm.DB, error), out io.Writer) int {
	fs := flag.NewFlagSet("import-cachet", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() { fmt.Fprint(out, Usage) }
	driver := fs.String("source-driver", "mysql", "")
	dsn := fs.String("source-dsn", "", "")
	dryRun := fs.Bool("dry-run", false, "")
	locale := fs.String("locale", "", "")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dsn == "" {
		fs.Usage()
		return 2
	}
	if *locale != "" && *locale != "en" && *locale != "fr" {
		fmt.Fprintln(out, "--locale must be en or fr")
		return 2
	}

	src, err := openSource(*driver, *dsn)
	if err != nil {
		fmt.Fprintf(out, "Cannot open the Cachet database: %v\n", err)
		return 1
	}
	if !src.Migrator().HasTable("components") || !src.Migrator().HasTable("incident_updates") {
		fmt.Fprintln(out, "The source does not look like a Cachet 2.4 database (no components or incident_updates table).")
		return 1
	}
	dst, err := target()
	if err != nil {
		fmt.Fprintf(out, "Cannot open the Vigie database: %v\n", err)
		return 1
	}

	report, err := Import(Options{Source: src, Target: dst, DryRun: *dryRun, Locale: *locale})
	if err != nil {
		fmt.Fprintf(out, "Import failed, nothing was written: %v\n", err)
		return 1
	}
	report.Print(out)
	return 0
}
