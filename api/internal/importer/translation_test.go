package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"api/internal/language"
	"api/internal/models"
	"api/internal/translate"

	"gorm.io/gorm"
)

// fakeTranslator prefixes texts with the target language, and fails on
// texts containing "DNS".
type fakeTranslator struct{ calls int }

func (f *fakeTranslator) Translate(_ context.Context, text, from, to, kind string) (string, error) {
	f.calls++
	if strings.Contains(text, "DNS") {
		return "", errors.New("model crashed")
	}
	return to + ": " + text, nil
}

// frenchCachet is the seed with its text in French, as on a French Cachet,
// apart from one English update.
func frenchCachet(t *testing.T) *gorm.DB {
	t.Helper()
	src := cachetSource(t)
	for _, stmt := range []string{
		"UPDATE settings SET value = 'Statut Acme' WHERE name = 'app_name'",
		"UPDATE components SET name = 'Base de facturation', description = 'Base de données interne' WHERE id = 3",
		"UPDATE incidents SET name = 'Lenteurs sur l''API', message = 'Nous constatons des temps de réponse élevés.' WHERE id = 1",
		"UPDATE incidents SET name = 'Ancienne maintenance', message = 'Une maintenance de Cachet 1.x.' WHERE id = 5",
		"UPDATE incident_updates SET message = 'Correctif déployé, nous surveillons la situation.' WHERE id = 1",
		"UPDATE incident_updates SET message = 'The billing database is back online and running.' WHERE id = 2",
		"UPDATE schedules SET name = 'Mise à jour de la base', message = 'Passage de MySQL en version 8.4.' WHERE id = 1",
	} {
		if err := src.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	return src
}

func translationsOf(t *testing.T, db *gorm.DB, entityType string, id uint) map[string]models.Translation {
	t.Helper()
	var rows []models.Translation
	if err := db.Where("entity_type = ? AND entity_id = ?", entityType, id).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	out := map[string]models.Translation{}
	for _, r := range rows {
		out[r.Field+"/"+r.Locale] = r
	}
	return out
}

func TestImportWithTranslation(t *testing.T) {
	src, dst := frenchCachet(t), freshVigie(t)
	cache, err := translate.OpenCache(filepath.Join(t.TempDir(), "cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeTranslator{}
	since := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var progress bytes.Buffer
	tr, err := Translate(context.Background(), src, "fr", TranslateOptions{Translator: fake, Model: "fake", Cache: cache,
		Since: since, Progress: &progress})
	if err != nil {
		t.Fatal(err)
	}
	report, err := Import(Options{Source: src, Target: dst, Translations: tr})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	report.Print(&out)
	t.Log("\n" + progress.String() + "\n" + out.String())

	// A French text keeps its original under fr, and gets a machine-made
	// English translation; names, kept in their own column, get one too.
	billing := translationsOf(t, dst, "component", 3)
	if d := billing["description/fr"]; d.Value != "Base de données interne" || d.Machine {
		t.Errorf("original description: %+v", d)
	}
	if d := billing["description/en"]; d.Value != "en: Base de données interne" || !d.Machine {
		t.Errorf("translated description: %+v", d)
	}
	if n := billing["name/en"]; n.Value != "en: Base de facturation" || !n.Machine {
		t.Errorf("translated name: %+v", n)
	}
	// Abbreviations are not translated.
	if _, ok := translationsOf(t, dst, "component", 1)["name/en"]; ok {
		t.Error("the API component name was translated")
	}
	// An English text on a French page is stored as English, and translated
	// into French.
	resolved := translationsOf(t, dst, "incident_update", 2)
	if m := resolved["message/en"]; m.Value != "The billing database is back online and running." || m.Machine {
		t.Errorf("English update: %+v", m)
	}
	if m := resolved["message/fr"]; m.Value != "fr: The billing database is back online and running." || !m.Machine {
		t.Errorf("English update in French: %+v", m)
	}
	if m := translationsOf(t, dst, "maintenance", 1)["name/en"]; m.Value != "en: Mise à jour de la base" {
		t.Errorf("maintenance name: %+v", m)
	}
	// History older than --translate-since stays in its language only.
	if old := translationsOf(t, dst, "incident", 5); len(old) != 1 || old["message/fr"].Value == "" {
		t.Errorf("old incident: %+v", old)
	}
	// A failed text stays in its language, and the report says so.
	if dns := translationsOf(t, dst, "incident", 2); len(dns) != 1 {
		t.Errorf("failed incident: %+v", dns)
	}
	if !strings.Contains(out.String(), "texts could not be translated") || !strings.Contains(out.String(), "made by fake") ||
		!strings.Contains(out.String(), "CACHET_API_LOCALE=fr") {
		t.Errorf("report: %s", out.String())
	}

	var branding models.Settings
	dst.Where("`key` = ?", "branding").First(&branding)
	var titles map[string]string
	json.Unmarshal([]byte(branding.Value), &titles)
	if titles["title_fr"] != "Statut Acme" || titles["title_en"] != "en: Statut Acme" {
		t.Errorf("branding: %v", titles)
	}
	lang, _, err := language.Load(dst)
	if err != nil || lang.Default != "fr" || lang.Mode != language.ModeBrowser {
		t.Errorf("language setting: %+v, %v", lang, err)
	}

	// A second run takes everything from the cache, apart from the
	// failures, which are tried again.
	first := fake.calls
	fake.calls = 0
	if _, err := Translate(context.Background(), src, "fr", TranslateOptions{Translator: fake, Model: "fake", Cache: cache,
		Since: since}); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 || first <= 2 {
		t.Errorf("calls: first run %d, second run %d, want only the 2 DNS texts again", first, fake.calls)
	}
}

func TestPageLocaleFollowsTheText(t *testing.T) {
	src := frenchCachet(t)
	if err := src.Exec("UPDATE settings SET value = 'en' WHERE name = 'app_locale'").Error; err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"UPDATE components SET description = 'Les commandes et le paiement en ligne' WHERE id = 1",
		"UPDATE incidents SET message = 'Le changement de serveur DNS est prévu pour ce soir.' WHERE id = 2",
		"UPDATE incidents SET name = 'La base de facturation ne répond plus', message = 'Seuls les services internes sont touchés.' WHERE id = 3",
		"UPDATE incidents SET name = 'Le site et l''API sont indisponibles', message = 'Nous cherchons la cause de la panne.' WHERE id = 6",
		"UPDATE schedules SET name = 'Déménagement des bureaux' WHERE id = 2",
	} {
		if err := src.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	locale, note, err := pageLocale(src, "")
	if err != nil || locale != "fr" || !strings.Contains(note, `app_locale is "en"`) {
		t.Fatalf("pageLocale = %q, %q, %v", locale, note, err)
	}
	if locale, note, _ := pageLocale(src, "en"); locale != "en" || note != "" {
		t.Fatalf("--locale en: %q, %q", locale, note)
	}
}
