package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"api/internal/models"
	"api/internal/testutil"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var sourceCounter atomic.Int64

// execSQLFile runs the statements of a SQL file, one per ";" at line end.
func execSQLFile(t *testing.T, db *gorm.DB, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "--") {
			lines = append(lines, l)
		}
	}
	for _, stmt := range strings.Split(strings.Join(lines, "\n"), ";\n") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v\n%s", path, err, stmt)
		}
	}
}

// cachetSource is a Cachet 2.4 database built by Cachet's own migrations,
// with the seed data.
func cachetSource(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:cachet%d?mode=memory&cache=shared", sourceCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	execSQLFile(t, db, "testdata/cachet24_schema_sqlite.sql")
	execSQLFile(t, db, "testdata/cachet24_seed.sql")
	return db
}

// freshVigie is a Vigie database as a fresh installation leaves it: the
// default admin account and the three example components.
func freshVigie(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.NewDB(t)
	if err := db.Create(&models.User{Username: "admin", Email: "admin@example.com", Password: "x", Role: models.RoleAdmin, Active: true}).Error; err != nil {
		t.Fatal(err)
	}
	for id, name := range seedComponents {
		if err := db.Create(&models.Component{ID: id, Name: name, Status: "operational"}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.Translation{EntityID: id, EntityType: "component", Locale: "fr", Field: "name", Value: name}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func ids[T any](t *testing.T, db *gorm.DB, model T) []uint {
	t.Helper()
	var out []uint
	db.Model(model).Order("id").Pluck("id", &out)
	return out
}

func TestImportCachet24(t *testing.T) {
	src, dst := cachetSource(t), freshVigie(t)
	report, err := Import(Options{Source: src, Target: dst})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	report.Print(&out)
	t.Log("\n" + out.String())

	// Groups
	var groups []models.ComponentGroup
	dst.Order("id").Find(&groups)
	if len(groups) != 2 || groups[0].Collapse != "operational" || groups[0].Visibility != "public" ||
		groups[1].Collapse != "never" || groups[1].Visibility != "authenticated" {
		t.Fatalf("groups: %+v", groups)
	}

	// Components keep their ids; the deleted one is left out, and the
	// example components of the fresh installation are gone.
	if got := fmt.Sprint(ids(t, dst, &models.Component{})); got != "[1 2 3 5]" {
		t.Fatalf("component ids %s", got)
	}
	var api models.Component
	dst.Preload("Tags").Preload("Translations").First(&api, 1)
	if api.Status != "degraded_performance" || api.Link != "https://api.acme.test" || api.Meta["zabbix_host"] != "api-01" ||
		models.IDValue(api.GroupID) != 1 || len(api.Tags) != 2 {
		t.Fatalf("API component: %+v", api)
	}
	if len(api.Translations) != 1 || api.Translations[0].Locale != "fr" || api.Translations[0].Value != "Public API" {
		t.Fatalf("API description, stored under Cachet's locale: %+v", api.Translations)
	}
	var billing models.Component
	dst.First(&billing, 3)
	if billing.Enabled || len(billing.Meta) != 0 || billing.Status != "major_outage" {
		t.Fatalf("disabled component: %+v", billing)
	}
	var mystery models.Component
	dst.First(&mystery, 5)
	if mystery.Status != "operational" {
		t.Fatalf("unknown status: %+v", mystery)
	}
	var seedLeft int64
	dst.Model(&models.Translation{}).Where("entity_type = ? AND field = ?", "component", "name").Count(&seedLeft)
	if seedLeft != 0 {
		t.Fatal("translations of the example components were left behind")
	}

	// Users: matched by username, roles mapped, passwords and keys kept.
	var pierre, bob, admin models.User
	dst.Where("username = ?", "pierre").First(&pierre)
	dst.Where("username = ?", "bob").First(&bob)
	dst.Where("username = ?", "admin").First(&admin)
	if pierre.Role != models.RoleAdmin || bob.Role != models.RoleViewer || pierre.Email != "pierre@acme.test" {
		t.Fatalf("users: pierre %+v, bob %+v", pierre, bob)
	}
	if bcrypt.CompareHashAndPassword([]byte(pierre.Password), []byte("secret")) != nil {
		t.Fatal("the Cachet password does not work in Vigie")
	}
	if admin.Email != "admin@example.com" {
		t.Fatalf("existing admin changed: %+v", admin)
	}
	var key models.APIKey
	if err := dst.Where("key_hash = ?", models.HashAPIKey("Ab3dEf6hIj9kLm2nOp5q")).First(&key).Error; err != nil || key.UserID != pierre.ID {
		t.Fatalf("pierre's Cachet API key: %+v, %v", key, err)
	}
	var keys int64
	dst.Model(&models.APIKey{}).Count(&keys)
	if keys != 2 {
		t.Fatalf("%d API keys, want 2 (empty keys are skipped)", keys)
	}

	// Incidents
	if got := fmt.Sprint(ids(t, dst, &models.Incident{})); got != "[1 2 3 5 6]" {
		t.Fatalf("incident ids %s", got)
	}
	var i1 models.Incident
	dst.Preload("Translations").Preload("Components").First(&i1, 1)
	if i1.Status != "resolved" || !i1.Stickied || models.IDValue(i1.UserID) != pierre.ID ||
		i1.OccurredAt == nil || !i1.OccurredAt.Equal(time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("incident 1: %+v", i1)
	}
	if len(i1.Components) != 1 || i1.Components[0].ComponentStatus != "degraded_performance" {
		t.Fatalf("incident 1 components: %+v", i1.Components)
	}
	var i2 models.Incident
	dst.First(&i2, 2)
	if i2.ComponentID != nil || i2.Notifications || models.IDValue(i2.UserID) != bob.ID {
		t.Fatalf("general incident: %+v", i2)
	}
	var i3 models.Incident
	dst.First(&i3, 3)
	if i3.Visible {
		t.Fatal("hidden incident became visible")
	}
	var i5 models.Incident
	dst.First(&i5, 5)
	if i5.Status != "resolved" {
		t.Fatalf("legacy status 0: %q", i5.Status)
	}
	var i6 models.Incident
	dst.Preload("Components").First(&i6, 6)
	statuses := map[uint]string{}
	for _, ic := range i6.Components {
		statuses[ic.ComponentID] = ic.ComponentStatus
	}
	if models.IDValue(i6.ComponentID) != 1 || len(statuses) != 2 || statuses[1] != "major_outage" || statuses[2] != "partial_outage" {
		t.Fatalf("incident 6: main %v, components %v", i6.ComponentID, statuses)
	}

	// Updates of the deleted incident are left out.
	if got := fmt.Sprint(ids(t, dst, &models.IncidentUpdate{})); got != "[1 2]" {
		t.Fatalf("update ids %s", got)
	}

	// Schedules become maintenances.
	if got := fmt.Sprint(ids(t, dst, &models.Maintenance{})); got != "[1 2]" {
		t.Fatalf("maintenance ids %s", got)
	}
	var m1, m2 models.Maintenance
	dst.Preload("Components").First(&m1, 1)
	dst.Preload("Components").First(&m2, 2)
	if m1.Status != models.MaintenanceCompleted || len(m1.Components) != 2 || models.IDValue(m1.ComponentID) != 1 {
		t.Fatalf("maintenance 1: %+v", m1)
	}
	if m2.ComponentID != nil || !m2.EndAt.Equal(m2.StartAt.Add(time.Hour)) {
		t.Fatalf("maintenance without an end: %+v", m2)
	}

	// Subscribers
	if got := fmt.Sprint(ids(t, dst, &models.Subscriber{})); got != "[1 2 3 5]" {
		t.Fatalf("subscriber ids %s", got)
	}
	var subs []models.Subscriber
	dst.Order("id").Find(&subs)
	byID := map[uint]models.Subscriber{}
	tokens := map[string]bool{}
	for _, s := range subs {
		byID[s.ID] = s
		tokens[s.UnsubscribeToken] = true
	}
	if byID[1].Email != "all@example.test" || !byID[1].Confirmed || !byID[1].Global {
		t.Fatalf("subscriber 1: %+v", byID[1])
	}
	if byID[2].Global || byID[3].Confirmed || !byID[5].Global {
		t.Fatalf("subscribers: %+v", byID)
	}
	if len(tokens) != 4 {
		t.Fatal("subscribers need distinct unsubscribe tokens")
	}
	var follows []models.Subscription
	dst.Find(&follows)
	if len(follows) != 1 || follows[0].SubscriberID != 2 || follows[0].ComponentID != 1 {
		t.Fatalf("subscriptions: %+v", follows)
	}

	// Settings and report
	var branding models.Settings
	dst.Where(&models.Settings{Key: "branding"}).First(&branding)
	var b map[string]string
	json.Unmarshal([]byte(branding.Value), &b)
	if b["title_en"] != "Acme Status" {
		t.Fatalf("branding: %q", branding.Value)
	}
	if report.LeftOut["metrics (Vigie has no metrics)"] != 1 ||
		report.LeftOut["metric points (Vigie has no metrics)"] != 2 {
		t.Fatalf("left out: %v", report.LeftOut)
	}
	for _, want := range []string{"PUBLIC_URL", "status.acme.test", "two-factor", "phone-only", "level 2", "deleted in Cachet"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report does not mention %q", want)
		}
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	src, dst := cachetSource(t), freshVigie(t)
	report, err := Import(Options{Source: src, Target: dst, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Imported["incidents"] != 5 {
		t.Fatalf("dry run report: %v", report.Imported)
	}
	if got := fmt.Sprint(ids(t, dst, &models.Component{})); got != "[1 2 3]" {
		t.Fatalf("after a dry run, components %s: want the untouched example ones", got)
	}
	var n int64
	dst.Model(&models.Incident{}).Count(&n)
	if n != 0 {
		t.Fatalf("dry run wrote %d incidents", n)
	}
}

func TestRefusesANonEmptyTarget(t *testing.T) {
	src := cachetSource(t)

	used := freshVigie(t)
	used.Create(&models.Incident{Name: "real", Status: "investigating"})
	if _, err := Import(Options{Source: src, Target: used}); err == nil || !strings.Contains(err.Error(), "incidents") {
		t.Fatalf("import into a database with incidents: %v", err)
	}

	renamed := freshVigie(t)
	renamed.Model(&models.Component{}).Where("id = ?", 1).Update("name", "Payments")
	if _, err := Import(Options{Source: src, Target: renamed}); err == nil {
		t.Fatal("import over a real component should be refused")
	}

	// A second import into the same database is refused too.
	dst := freshVigie(t)
	if _, err := Import(Options{Source: src, Target: dst}); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(Options{Source: src, Target: dst}); err == nil {
		t.Fatal("a second import should be refused")
	}
}

func TestLocaleOverride(t *testing.T) {
	src, dst := cachetSource(t), freshVigie(t)
	if _, err := Import(Options{Source: src, Target: dst, Locale: "en"}); err != nil {
		t.Fatal(err)
	}
	var locales []string
	dst.Model(&models.Translation{}).Distinct("locale").Pluck("locale", &locales)
	sort.Strings(locales)
	if fmt.Sprint(locales) != "[en]" {
		t.Fatalf("locales %v, want [en]", locales)
	}
}

func TestSourceTimezone(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	src, dst := cachetSource(t), freshVigie(t)
	if err := src.Exec("INSERT INTO settings (name, value) VALUES ('app_timezone', 'Europe/Paris')").Error; err != nil {
		t.Fatal(err)
	}
	report, err := Import(Options{Source: src, Target: dst, SourceTimezone: paris})
	if err != nil {
		t.Fatal(err)
	}
	// Cachet wrote 09:00 Paris time, which is 08:00 UTC in March.
	var i models.Incident
	if err := dst.First(&i, 1).Error; err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2024, 3, 1, 8, 0, 0, 0, time.UTC); i.OccurredAt == nil || !i.OccurredAt.Equal(want) {
		t.Fatalf("occurred_at %v, want %v", i.OccurredAt, want)
	}
	var m models.Maintenance
	if err := dst.First(&m, 1).Error; err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC); !m.StartAt.Equal(want) {
		t.Fatalf("start_at %v, want %v (02:00 in Paris, summer time)", m.StartAt, want)
	}
	if !strings.Contains(strings.Join(report.Notes, "\n"), "read as Europe/Paris times") {
		t.Fatalf("notes %q do not mention the zone", report.Notes)
	}
}

func TestWarnsWhenCachetShowedAnotherZone(t *testing.T) {
	src, dst := cachetSource(t), freshVigie(t)
	if err := src.Exec("INSERT INTO settings (name, value) VALUES ('app_timezone', 'Europe/Paris')").Error; err != nil {
		t.Fatal(err)
	}
	report, err := Import(Options{Source: src, Target: dst})
	if err != nil {
		t.Fatal(err)
	}
	var i models.Incident
	if err := dst.First(&i, 1).Error; err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC); !i.OccurredAt.Equal(want) {
		t.Fatalf("occurred_at %v, want %v: UTC by default", i.OccurredAt, want)
	}
	if !strings.Contains(strings.Join(report.Notes, "\n"), "--source-timezone") {
		t.Fatalf("notes %q do not suggest --source-timezone", report.Notes)
	}
}
