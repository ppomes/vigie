package importer

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	// The zone database, for --source-timezone: the API image has none.
	_ "time/tzdata"

	"api/internal/translate"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Usage explains the import-cachet command.
const Usage = `Usage: api import-cachet --source-driver mysql|sqlite --source-dsn DSN [--dry-run] [--locale en|fr] [--source-timezone ZONE]
       [--translate [--translator-url URL] [--translator-model MODEL] [--translate-since DATE] [--translate-cache FILE]]

Copies a Cachet 2.4 installation into this Vigie database, which must be a
fresh installation. The Vigie database is the one the API is configured with
(DB_* variables). The Cachet database is only read.

  --source-driver  mysql or sqlite
  --source-dsn     MySQL: user:password@tcp(host:3306)/cachet
                   SQLite: path to the database file
  --dry-run        do everything in a transaction, print the report, roll back
  --locale         locale for Cachet's text (default: the language most of it
                   is written in, else Cachet's app_locale, else en)
  --source-timezone  zone Cachet wrote its dates in: APP_TIMEZONE in Cachet's
                   .env, for example Europe/Paris (default: UTC)

Translation, with a model served by Ollama, before the import:
  --translate      translate Cachet's text into the other languages of the page
  --translator-url   Ollama's address (default: $OLLAMA_URL, else http://ollama:11434)
  --translator-model model to use (default: qwen2.5:7b)
  --translate-since  YYYY-MM-DD: leave older incidents and maintenances untranslated
  --translate-cache  file that keeps translations between runs
                   (default: vigie-translation-cache.json in the temporary directory)
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
	zoneName := fs.String("source-timezone", "UTC", "")
	translateText := fs.Bool("translate", false, "")
	defaultURL := os.Getenv("OLLAMA_URL")
	if defaultURL == "" {
		defaultURL = "http://ollama:11434"
	}
	translatorURL := fs.String("translator-url", defaultURL, "")
	translatorModel := fs.String("translator-model", "qwen2.5:7b", "")
	sinceText := fs.String("translate-since", "", "")
	cachePath := fs.String("translate-cache", filepath.Join(os.TempDir(), "vigie-translation-cache.json"), "")
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

	zone, err := time.LoadLocation(*zoneName)
	if err != nil {
		fmt.Fprintf(out, "--source-timezone: unknown zone %q: use an IANA name such as Europe/Paris\n", *zoneName)
		return 2
	}
	var since time.Time
	if *sinceText != "" {
		if since, err = time.ParseInLocation("2006-01-02", *sinceText, zone); err != nil {
			fmt.Fprintf(out, "--translate-since: %q is not a date: use YYYY-MM-DD\n", *sinceText)
			return 2
		}
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

	var translations *Translations
	if *translateText {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		translations, err = translateSource(ctx, src, *locale, zone, *translatorURL, *translatorModel, *cachePath, since, out)
		if err != nil {
			fmt.Fprintf(out, "Translation failed, nothing was written: %v\n", err)
			return 1
		}
	}

	report, err := Import(Options{Source: src, Target: dst, DryRun: *dryRun, Locale: *locale, SourceTimezone: zone,
		Translations: translations})
	if err != nil {
		fmt.Fprintf(out, "Import failed, nothing was written: %v\n", err)
		return 1
	}
	report.Print(out)
	return 0
}

// translateSource checks the model, then translates Cachet's text, reading
// its dates in zone as the import does.
func translateSource(ctx context.Context, src *gorm.DB, locale string, zone *time.Location, url, model, cachePath string,
	since time.Time, out io.Writer) (*Translations, error) {
	ollama := translate.NewOllama(url, model)
	if err := ollama.Check(ctx); err != nil {
		return nil, err
	}
	cache, err := translate.OpenCache(cachePath)
	if err != nil {
		return nil, fmt.Errorf("translation cache %s: %w", cachePath, err)
	}
	zoned, err := inSourceZone(src, zone)
	if err != nil {
		return nil, err
	}
	return Translate(ctx, zoned, locale, TranslateOptions{Translator: ollama, Model: model, Cache: cache, Since: since, Progress: out})
}
