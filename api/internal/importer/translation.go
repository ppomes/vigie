package importer

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"api/internal/language"
	"api/internal/models"
	"api/internal/translate"

	"gorm.io/gorm"
)

// Kinds of text, as given to the model.
const (
	kindPageTitle          = "the name of the status page"
	kindGroupName          = "the name of a group of components"
	kindComponentName      = "the name of a component of the service"
	kindComponentDesc      = "the description of a component of the service"
	kindIncidentName       = "the title of an incident"
	kindIncidentMessage    = "the message of an incident"
	kindUpdateMessage      = "an update on an incident"
	kindMaintenanceName    = "the title of a planned maintenance"
	kindMaintenanceMessage = "the message of a planned maintenance"
)

// TranslateOptions configures the translation of Cachet's text.
type TranslateOptions struct {
	Translator translate.Translator
	// Model names the model, for the cache and the report.
	Model string
	// Cache keeps translations between runs. Optional.
	Cache *translate.Cache
	// Since leaves incidents and maintenances older than it untranslated.
	// Zero means everything.
	Since time.Time
	// Progress receives progress lines. Optional.
	Progress io.Writer
}

// Translations are the translations of Cachet's text, made before the
// import, so that the import transaction does not stay open while the
// model works.
type Translations struct {
	// Locale is the language of the page, as for Options.Locale.
	Locale string
	// LocaleNote explains a page language that differs from app_locale.
	LocaleNote string
	Model      string
	entries    map[string]textTranslation
	failed     []string
}

type textTranslation struct {
	lang   string            // the language the text is written in
	values map[string]string // translations by locale
}

type sourceText struct{ kind, text string }

func textKey(kind, text string) string { return kind + "\x00" + text }

// get returns the translation of text, if it was translated.
func (t *Translations) get(kind, text string) (textTranslation, bool) {
	if t == nil {
		return textTranslation{}, false
	}
	e, ok := t.entries[textKey(kind, text)]
	return e, ok
}

// collectTexts lists the text the import copies. With since set, incidents,
// their updates and maintenances older than it are left out.
func collectTexts(src *gorm.DB, since time.Time) ([]sourceText, error) {
	var texts []sourceText
	seen := map[string]bool{}
	add := func(kind string, value *string) {
		if value == nil {
			return
		}
		v := strings.TrimSpace(*value)
		if v == "" || seen[textKey(kind, v)] {
			return
		}
		seen[textKey(kind, v)] = true
		texts = append(texts, sourceText{kind, v})
	}
	old := func(t *time.Time) bool { return !since.IsZero() && t != nil && t.Before(since) }

	var name cachetSetting
	if err := src.Where("name = ?", "app_name").Limit(1).Find(&name).Error; err != nil {
		return nil, err
	}
	add(kindPageTitle, name.Value)

	var groups []cachetGroup
	if err := src.Order("id").Find(&groups).Error; err != nil {
		return nil, err
	}
	for _, g := range groups {
		add(kindGroupName, &g.Name)
	}
	var components []cachetComponent
	if err := src.Order("id").Find(&components).Error; err != nil {
		return nil, err
	}
	for _, c := range components {
		if c.DeletedAt == nil {
			add(kindComponentName, &c.Name)
			add(kindComponentDesc, c.Description)
		}
	}
	var incidents []cachetIncident
	if err := src.Order("id").Find(&incidents).Error; err != nil {
		return nil, err
	}
	kept := map[uint]bool{}
	for _, i := range incidents {
		if i.DeletedAt != nil {
			continue
		}
		kept[i.ID] = true
		date := i.OccurredAt
		if date == nil {
			date = i.CreatedAt
		}
		if old(date) {
			continue
		}
		add(kindIncidentName, &i.Name)
		add(kindIncidentMessage, i.Message)
	}
	var updates []cachetIncidentUpdate
	if err := src.Order("id").Find(&updates).Error; err != nil {
		return nil, err
	}
	for _, u := range updates {
		if kept[u.IncidentID] && !old(u.CreatedAt) {
			add(kindUpdateMessage, u.Message)
		}
	}
	var schedules []cachetSchedule
	if err := src.Order("id").Find(&schedules).Error; err != nil {
		return nil, err
	}
	for _, s := range schedules {
		if s.DeletedAt == nil && s.ScheduledAt != nil && !old(s.ScheduledAt) {
			add(kindMaintenanceName, &s.Name)
			add(kindMaintenanceMessage, s.Message)
		}
	}
	return texts, nil
}

// pageLocale picks the language Cachet's text is stored under: flag when
// set, else the language most of the text is written in, else app_locale.
// A Cachet left on its default app_locale (en) often holds French text.
func pageLocale(src *gorm.DB, flag string) (locale, note string, err error) {
	if flag != "" {
		return flag, "", nil
	}
	var setting cachetSetting
	if err := src.Where("name = ?", "app_locale").Limit(1).Find(&setting).Error; err != nil {
		return "", "", err
	}
	appLocale := "en"
	if setting.Value != nil && strings.HasPrefix(strings.ToLower(*setting.Value), "fr") {
		appLocale = "fr"
	}
	texts, err := collectTexts(src, time.Time{})
	if err != nil {
		return "", "", err
	}
	values := make([]string, len(texts))
	for i, t := range texts {
		values[i] = t.text
	}
	if l, ok := translate.Majority(values, language.Supported); ok && l != appLocale {
		return l, fmt.Sprintf("Cachet's app_locale is %q, but most of its text is in %q: the text was imported as %q. "+
			"To choose another language, run the import again with --locale.", appLocale, l, l), nil
	}
	return appLocale, "", nil
}

// Translate translates Cachet's text into every language of the page it is
// not written in. A text that cannot be translated stays in its own
// language only; the import reports it.
func Translate(ctx context.Context, src *gorm.DB, locale string, opts TranslateOptions) (*Translations, error) {
	page, note, err := pageLocale(src, locale)
	if err != nil {
		return nil, err
	}
	texts, err := collectTexts(src, opts.Since)
	if err != nil {
		return nil, err
	}
	tr := &Translations{Locale: page, LocaleNote: note, Model: opts.Model, entries: map[string]textTranslation{}}
	progress := opts.Progress
	if progress == nil {
		progress = io.Discard
	}
	fmt.Fprintf(progress, "Translating %d texts with %s (text in %q)...\n", len(texts), opts.Model, page)

	start, calls, last := time.Now(), 0, time.Now()
	for n, t := range texts {
		if translate.Skip(t.text) {
			continue
		}
		lang := page
		if l, ok := translate.Detect(t.text, language.Supported); ok {
			lang = l
		}
		entry := textTranslation{lang: lang, values: map[string]string{}}
		for _, target := range language.Supported {
			if target == lang {
				continue
			}
			key := translate.Key(opts.Model, lang, target, t.kind, t.text)
			value, cached := "", false
			if opts.Cache != nil {
				value, cached = opts.Cache.Get(key)
			}
			if !cached {
				value, err = opts.Translator.Translate(ctx, t.text, lang, target, t.kind)
				calls++
				if err != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					tr.failed = append(tr.failed, t.text)
					fmt.Fprintf(progress, "  could not translate %q: %v\n", short(t.text), err)
					continue
				}
				if opts.Cache != nil {
					if err := opts.Cache.Put(key, value); err != nil {
						return nil, fmt.Errorf("translation cache: %w", err)
					}
				}
			}
			// A text that reads the same in both languages (a product
			// name) needs no translation.
			if !strings.EqualFold(strings.TrimSpace(value), t.text) {
				entry.values[target] = value
			}
		}
		tr.entries[textKey(t.kind, t.text)] = entry
		if time.Since(last) >= 30*time.Second {
			last = time.Now()
			done := n + 1
			left := ""
			if calls > 0 {
				per := time.Since(start) / time.Duration(calls)
				left = ", under a minute left"
				if rest := per * time.Duration(len(texts)-done); rest >= time.Minute {
					left = fmt.Sprintf(", about %s left", rest.Round(time.Minute))
				}
			}
			fmt.Fprintf(progress, "  %d/%d texts%s\n", done, len(texts), left)
		}
	}
	if opts.Cache != nil {
		if err := opts.Cache.Save(); err != nil {
			return nil, fmt.Errorf("translation cache: %w", err)
		}
	}
	fmt.Fprintf(progress, "Translated in %s.\n", time.Since(start).Round(time.Second))
	return tr, nil
}

func short(s string) string {
	if r := []rune(s); len(r) > 60 {
		return string(r[:60]) + "..."
	}
	return s
}

// textLang returns the language a text is stored under.
func (im *importer) textLang(kind, text string) string {
	if e, ok := im.translations.get(kind, strings.TrimSpace(text)); ok {
		return e.lang
	}
	return im.locale
}

// machineTranslations adds the model's translations of text as field of
// the entity, marked as machine-made.
func (im *importer) machineTranslations(entityType string, id uint, field, kind, text string) error {
	e, ok := im.translations.get(kind, strings.TrimSpace(text))
	if !ok {
		return nil
	}
	for _, locale := range language.Supported {
		v, ok := e.values[locale]
		if !ok {
			continue
		}
		if err := im.dst.Create(&models.Translation{EntityID: id, EntityType: entityType, Locale: locale, Field: field,
			Value: v, Machine: true}).Error; err != nil {
			return err
		}
		im.report.imported("machine translations")
	}
	return nil
}

// reportTranslations tells what the model did.
func (im *importer) reportTranslations() {
	tr := im.translations
	if tr == nil {
		return
	}
	if n := im.report.Imported["machine translations"]; n > 0 {
		im.report.note("%d translations were made by %s. They are marked \"machine translation\" in the admin: "+
			"review them there, and edit them where needed.", n, tr.Model)
	}
	if len(tr.failed) > 0 {
		im.report.note("%d texts could not be translated and exist in their own language only, for example %q. "+
			"Translate them by hand in the admin.",
			len(tr.failed), short(tr.failed[0]))
	}
}
