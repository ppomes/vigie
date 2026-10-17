package importer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"api/internal/models"

	"gorm.io/gorm"
)

// Options configures an import.
type Options struct {
	// Source is the Cachet 2.4 database. It is only read.
	Source *gorm.DB
	// Target is the Vigie database, migrated.
	Target *gorm.DB
	// DryRun runs the whole import in a transaction and rolls it back.
	DryRun bool
	// Locale is the locale Cachet's single-language text is stored under.
	// Empty means Cachet's app_locale setting when it is en or fr, else en.
	Locale string
	// SourceTimezone is the zone Cachet wrote its dates in: its APP_TIMEZONE
	// variable. Nil means UTC, Cachet's default.
	SourceTimezone *time.Location
}

// errDryRun rolls the transaction back at the end of a dry run.
var errDryRun = errors.New("dry run")

// seedComponents are the example components 001_schema.sql creates.
var seedComponents = map[uint]string{1: "API", 2: "Website", 3: "Database"}

// Import copies a Cachet 2.4 installation into an empty Vigie database,
// keeping the ids of groups, components, incidents, schedules and
// subscribers, so that integrations and links keep pointing at the same
// objects. Users are matched by username or email, since Vigie creates an
// admin account of its own.
func Import(opts Options) (*Report, error) {
	report := newReport(opts.DryRun)
	src, err := inSourceZone(opts.Source, opts.SourceTimezone)
	if err != nil {
		return nil, err
	}
	err = opts.Target.Transaction(func(tx *gorm.DB) error {
		im := &importer{src: src, dst: tx, report: report, locale: opts.Locale, zone: opts.SourceTimezone}
		if err := im.run(); err != nil {
			return err
		}
		if opts.DryRun {
			return errDryRun
		}
		return nil
	})
	if errors.Is(err, errDryRun) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return report, nil
}

type importer struct {
	src, dst *gorm.DB
	report   *Report
	locale   string
	zone     *time.Location
	settings map[string]string

	components map[uint]bool // ids of imported components
	users      map[uint]uint // Cachet user id to Vigie user id
}

func (im *importer) run() error {
	steps := []struct {
		name string
		fn   func() error
	}{
		{"check the target", im.checkTarget},
		{"read settings", im.readSettings},
		{"component groups", im.groups},
		{"components", im.componentsAndTags},
		{"users", im.importUsers},
		{"incidents", im.incidents},
		{"schedules", im.schedules},
		{"subscribers", im.subscribers},
		{"settings", im.applySettings},
		{"leftovers", im.leftovers},
	}
	for _, s := range steps {
		if err := s.fn(); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	return nil
}

// checkTarget refuses a Vigie database that already holds data, apart from
// the example components and admin account of a fresh installation. The
// example components are removed, since imported components keep their ids.
func (im *importer) checkTarget() error {
	for _, t := range []struct {
		model any
		label string
	}{
		{&models.Incident{}, "incidents"},
		{&models.Maintenance{}, "maintenances"},
		{&models.Subscriber{}, "subscribers"},
		{&models.ComponentGroup{}, "component groups"},
	} {
		var n int64
		if err := im.dst.Unscoped().Model(t.model).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("the Vigie database already has %d %s: import into a fresh installation", n, t.label)
		}
	}
	var components []models.Component
	if err := im.dst.Unscoped().Find(&components).Error; err != nil {
		return err
	}
	for _, c := range components {
		if seedComponents[c.ID] != c.Name {
			return fmt.Errorf("the Vigie database already has components (%s): import into a fresh installation", c.Name)
		}
	}
	if len(components) > 0 {
		ids := make([]uint, 0, len(components))
		for _, c := range components {
			ids = append(ids, c.ID)
		}
		if err := im.dst.Unscoped().Where("entity_type = ? AND entity_id IN ?", "component", ids).Delete(&models.Translation{}).Error; err != nil {
			return err
		}
		if err := im.dst.Unscoped().Delete(&models.Component{}, ids).Error; err != nil {
			return err
		}
		im.report.note("The %d example components of the fresh installation were removed.", len(components))
	}
	return nil
}

func (im *importer) readSettings() error {
	var settings []cachetSetting
	if err := im.src.Find(&settings).Error; err != nil {
		return err
	}
	im.settings = map[string]string{}
	for _, s := range settings {
		if s.Value != nil {
			im.settings[s.Name] = *s.Value
		}
	}
	im.checkZone()
	if im.locale == "" {
		im.locale = "en"
		if l := strings.ToLower(im.settings["app_locale"]); strings.HasPrefix(l, "fr") {
			im.locale = "fr"
		}
	}
	return nil
}

// create inserts a row with its associations left out. defaultTrue lists
// the boolean columns that default to true, with their values: GORM writes
// the column default instead of false, even with Select("*"), so false
// values are written again afterwards, without touching updated_at.
func (im *importer) create(value any, defaultTrue map[string]bool) error {
	if err := im.dst.Omit("Translations", "Components", "Tags", "Updates", "Group", "Component", "User", "Subscriptions").
		Create(value).Error; err != nil {
		return err
	}
	falses := map[string]any{}
	for col, v := range defaultTrue {
		if !v {
			falses[col] = false
		}
	}
	if len(falses) == 0 {
		return nil
	}
	return im.dst.Model(value).UpdateColumns(falses).Error
}

func (im *importer) translate(entityType string, id uint, field string, value *string) error {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return im.dst.Create(&models.Translation{EntityID: id, EntityType: entityType, Locale: im.locale, Field: field, Value: *value}).Error
}

func timeOr(t *time.Time, fallback time.Time) time.Time {
	if t == nil || t.IsZero() {
		return fallback
	}
	return *t
}

func (im *importer) groups() error {
	var groups []cachetGroup
	if err := im.src.Order("id").Find(&groups).Error; err != nil {
		return err
	}
	collapse := map[int]string{0: models.GroupCollapseNever, 1: models.GroupCollapseAlways, 2: models.GroupCollapseOperational}
	now := time.Now()
	for _, g := range groups {
		c, ok := collapse[g.Collapsed]
		if !ok {
			c = models.GroupCollapseNever
			im.report.adapt("unknown collapse value, set to never collapsed", row("group", g.ID, g.Name))
		}
		visibility := models.GroupVisibilityPublic
		if g.Visible == 0 {
			visibility = models.GroupVisibilityAuthenticated
		}
		ng := models.ComponentGroup{ID: g.ID, Name: g.Name, Order: g.Order, Collapse: c, Visibility: visibility,
			CreatedAt: timeOr(g.CreatedAt, now), UpdatedAt: timeOr(g.UpdatedAt, now)}
		if err := im.create(&ng, nil); err != nil {
			return err
		}
		im.report.imported("component groups")
	}
	return nil
}

func (im *importer) componentsAndTags() error {
	var components []cachetComponent
	if err := im.src.Order("id").Find(&components).Error; err != nil {
		return err
	}
	var groupIDs []uint
	if err := im.dst.Model(&models.ComponentGroup{}).Pluck("id", &groupIDs).Error; err != nil {
		return err
	}
	groups := map[uint]bool{}
	for _, id := range groupIDs {
		groups[id] = true
	}
	statuses := map[int]string{1: "operational", 2: "degraded_performance", 3: "partial_outage", 4: "major_outage"}
	im.components = map[uint]bool{}
	now := time.Now()
	for _, c := range components {
		label := row("component", c.ID, c.Name)
		if c.DeletedAt != nil {
			im.report.skip("deleted in Cachet", label)
			continue
		}
		status, ok := statuses[c.Status]
		if !ok {
			status = "operational"
			im.report.adapt("status unknown (0) in Cachet, set to operational", label)
		}
		var groupID *uint
		if c.GroupID != nil && *c.GroupID != 0 {
			if groups[*c.GroupID] {
				groupID = c.GroupID
			} else {
				im.report.adapt("group missing in Cachet, imported without a group", label)
			}
		}
		meta := models.JSONMap(nil)
		if c.Meta != nil && strings.TrimSpace(*c.Meta) != "" && *c.Meta != "null" && *c.Meta != "[]" {
			raw := map[string]any{}
			if err := json.Unmarshal([]byte(*c.Meta), &raw); err != nil {
				im.report.adapt("meta is not a JSON object, left out", label)
			} else {
				meta = models.JSONMap{}
				for k, v := range raw {
					meta[k] = fmt.Sprint(v)
				}
			}
		}
		link := ""
		if c.Link != nil {
			link = *c.Link
		}
		nc := models.Component{ID: c.ID, Name: c.Name, Status: status, Order: c.Order, GroupID: groupID,
			Link: link, Enabled: c.Enabled, Meta: meta,
			CreatedAt: timeOr(c.CreatedAt, now), UpdatedAt: timeOr(c.UpdatedAt, now)}
		if err := im.create(&nc, map[string]bool{"enabled": c.Enabled}); err != nil {
			return err
		}
		if err := im.translate("component", c.ID, "description", c.Description); err != nil {
			return err
		}
		im.components[c.ID] = true
		im.report.imported("components")
	}
	return im.tags()
}

func (im *importer) tags() error {
	var tags []cachetTag
	if err := im.src.Find(&tags).Error; err != nil {
		return err
	}
	bySlug := map[string]models.Tag{}
	byCachetID := map[uint]models.Tag{}
	for _, t := range tags {
		slug := models.TagSlug(t.Name)
		if slug == "" {
			slug = models.TagSlug(t.Slug)
		}
		if slug == "" {
			im.report.skip("tag without letters or digits", row("tag", t.ID, t.Name))
			continue
		}
		tag, ok := bySlug[slug]
		if !ok {
			tag = models.Tag{Name: t.Name, Slug: slug}
			if err := im.dst.Create(&tag).Error; err != nil {
				return err
			}
			bySlug[slug] = tag
			im.report.imported("tags")
		}
		byCachetID[t.ID] = tag
	}
	var links []cachetTaggable
	if err := im.src.Where("taggable_type IN ?", []string{"components", "CachetHQ\\Cachet\\Models\\Component"}).Find(&links).Error; err != nil {
		return err
	}
	seen := map[[2]uint]bool{}
	for _, l := range links {
		tag, ok := byCachetID[l.TagID]
		if !ok || !im.components[l.TaggableID] {
			continue
		}
		key := [2]uint{l.TaggableID, tag.ID}
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := im.dst.Exec("INSERT INTO component_tags (component_id, tag_id) VALUES (?, ?)", l.TaggableID, tag.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (im *importer) importUsers() error {
	var users []cachetUser
	if err := im.src.Order("id").Find(&users).Error; err != nil {
		return err
	}
	im.users = map[uint]uint{}
	now := time.Now()
	for _, u := range users {
		label := fmt.Sprintf("user %s", u.Username)
		var existing models.User
		err := im.dst.Where("username = ? OR email = ?", u.Username, strings.ToLower(u.Email)).First(&existing).Error
		switch {
		case err == nil:
			im.users[u.ID] = existing.ID
			im.report.adapt("already exists in Vigie, kept as it is", label)
		case errors.Is(err, gorm.ErrRecordNotFound):
			role := models.RoleAdmin
			if u.Level != 1 {
				role = models.RoleViewer
				im.report.adapt("Cachet user (level 2) imported as viewer: Vigie has no role that can manage incidents without being admin", label)
			}
			nu := models.User{Username: u.Username, Email: strings.ToLower(u.Email), Password: u.Password, Role: role,
				Active: u.Active, CreatedAt: timeOr(u.CreatedAt, now), UpdatedAt: timeOr(u.UpdatedAt, now)}
			if err := im.create(&nu, map[string]bool{"active": u.Active}); err != nil {
				return err
			}
			im.users[u.ID] = nu.ID
			im.report.imported("users")
		default:
			return err
		}
		if u.Google2faSecret != nil && *u.Google2faSecret != "" {
			im.report.skip("two-factor secret (Vigie has no two-factor login)", label)
		}
		if u.APIKey != nil && *u.APIKey != "" {
			key := models.NewAPIKey("Imported from Cachet ("+u.Username+")", *u.APIKey, im.users[u.ID])
			if err := im.dst.Create(&key).Error; err != nil {
				return err
			}
			im.report.imported("API keys")
		}
	}
	return nil
}

func (im *importer) userID(id *uint) *uint {
	if id == nil {
		return nil
	}
	if v, ok := im.users[*id]; ok {
		return &v
	}
	return nil
}

func (im *importer) incidents() error {
	var incidents []cachetIncident
	if err := im.src.Order("id").Find(&incidents).Error; err != nil {
		return err
	}
	statuses := map[int]string{1: "investigating", 2: "identified", 3: "monitoring", 4: "resolved"}
	componentStatuses := map[int]string{1: "operational", 2: "degraded_performance", 3: "partial_outage", 4: "major_outage"}
	imported := map[uint]bool{}
	now := time.Now()
	for _, i := range incidents {
		label := row("incident", i.ID, i.Name)
		if i.DeletedAt != nil {
			im.report.skip("deleted in Cachet", label)
			continue
		}
		status, ok := statuses[i.Status]
		if !ok {
			status = "resolved"
			im.report.adapt("status 0 (scheduled, from Cachet 1.x) imported as resolved", label)
		}
		var componentID *uint
		if i.ComponentID != nil && *i.ComponentID != 0 {
			if im.components[*i.ComponentID] {
				componentID = i.ComponentID
			} else {
				im.report.adapt("component not imported, incident kept without it", label)
			}
		}
		created := timeOr(i.CreatedAt, now)
		occurred := timeOr(i.OccurredAt, created)
		ni := models.Incident{ID: i.ID, Name: i.Name, Status: status, ComponentID: componentID, Visible: i.Visible,
			Stickied: i.Stickied, OccurredAt: &occurred, Notifications: i.Notifications, UserID: im.userID(i.UserID),
			CreatedAt: created, UpdatedAt: timeOr(i.UpdatedAt, created)}
		if err := im.create(&ni, map[string]bool{"visible": i.Visible, "notifications": i.Notifications}); err != nil {
			return err
		}
		if err := im.translate("incident", i.ID, "message", i.Message); err != nil {
			return err
		}
		if componentID != nil {
			if err := im.dst.Create(&models.IncidentComponent{IncidentID: i.ID, ComponentID: *componentID}).Error; err != nil {
				return err
			}
		}
		imported[i.ID] = true
		im.report.imported("incidents")
	}

	var links []cachetIncidentComponent
	if err := im.src.Find(&links).Error; err != nil {
		return err
	}
	for _, l := range links {
		if !imported[l.IncidentID] || !im.components[l.ComponentID] {
			continue
		}
		status := ""
		if l.StatusID != nil {
			status = componentStatuses[*l.StatusID]
		}
		var existing models.IncidentComponent
		err := im.dst.Where("incident_id = ? AND component_id = ?", l.IncidentID, l.ComponentID).First(&existing).Error
		switch {
		case err == nil:
			if status != "" {
				if err := im.dst.Model(&existing).Update("component_status", status).Error; err != nil {
					return err
				}
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := im.dst.Create(&models.IncidentComponent{IncidentID: l.IncidentID, ComponentID: l.ComponentID, ComponentStatus: status}).Error; err != nil {
				return err
			}
		default:
			return err
		}
		// A general incident in Cachet with affected components gets its
		// first one as main component.
		if err := im.dst.Model(&models.Incident{}).Where("id = ? AND component_id IS NULL", l.IncidentID).
			Update("component_id", l.ComponentID).Error; err != nil {
			return err
		}
	}

	var updates []cachetIncidentUpdate
	if err := im.src.Order("id").Find(&updates).Error; err != nil {
		return err
	}
	for _, u := range updates {
		if !imported[u.IncidentID] {
			continue
		}
		status, ok := statuses[u.Status]
		if !ok {
			status = "resolved"
			im.report.adapt("update status 0 imported as resolved", fmt.Sprintf("update %d of incident %d", u.ID, u.IncidentID))
		}
		created := timeOr(u.CreatedAt, now)
		nu := models.IncidentUpdate{ID: u.ID, IncidentID: u.IncidentID, Status: status, UserID: im.userID(u.UserID),
			CreatedAt: created, UpdatedAt: timeOr(u.UpdatedAt, created)}
		if err := im.create(&nu, nil); err != nil {
			return err
		}
		if err := im.translate("incident_update", u.ID, "message", u.Message); err != nil {
			return err
		}
		im.report.imported("incident updates")
	}
	// As in Vigie, an incident's status follows its latest update.
	return im.dst.Exec(`UPDATE incidents SET status = (
			SELECT iu.status FROM incident_updates iu WHERE iu.incident_id = incidents.id
			ORDER BY iu.created_at DESC, iu.id DESC LIMIT 1)
		WHERE EXISTS (SELECT 1 FROM incident_updates iu WHERE iu.incident_id = incidents.id)`).Error
}

func (im *importer) schedules() error {
	var schedules []cachetSchedule
	if err := im.src.Order("id").Find(&schedules).Error; err != nil {
		return err
	}
	statuses := map[int]string{0: models.MaintenanceScheduled, 1: models.MaintenanceInProgress, 2: models.MaintenanceCompleted}
	var links []cachetScheduleComponent
	if err := im.src.Order("id").Find(&links).Error; err != nil {
		return err
	}
	componentsOf := map[uint][]uint{}
	for _, l := range links {
		if im.components[l.ComponentID] {
			componentsOf[l.ScheduleID] = append(componentsOf[l.ScheduleID], l.ComponentID)
		}
	}
	now := time.Now()
	for _, s := range schedules {
		label := row("schedule", s.ID, s.Name)
		if s.DeletedAt != nil {
			im.report.skip("deleted in Cachet", label)
			continue
		}
		if s.ScheduledAt == nil {
			im.report.skip("no scheduled date", label)
			continue
		}
		status, ok := statuses[s.Status]
		if !ok {
			status = models.MaintenanceScheduled
			im.report.adapt("unknown status, imported as scheduled", label)
		}
		end := timeOr(s.CompletedAt, time.Time{})
		if end.IsZero() {
			end = s.ScheduledAt.Add(time.Hour)
			im.report.adapt("no end date, end set one hour after the start", label)
		}
		ids := componentsOf[s.ID]
		var main *uint
		if len(ids) > 0 {
			main = models.OptionalID(ids[0])
		}
		created := timeOr(s.CreatedAt, now)
		nm := models.Maintenance{ID: s.ID, Name: s.Name, Status: status, ComponentID: main, StartAt: *s.ScheduledAt, EndAt: end,
			CreatedAt: created, UpdatedAt: timeOr(s.UpdatedAt, created)}
		if err := im.create(&nm, nil); err != nil {
			return err
		}
		if err := im.translate("maintenance", s.ID, "message", s.Message); err != nil {
			return err
		}
		for _, id := range ids {
			// Cachet does not change component statuses during schedules.
			// Recording the current status as the one to restore makes an
			// imported maintenance in progress end without changing it.
			var comp models.Component
			if err := im.dst.First(&comp, id).Error; err != nil {
				return err
			}
			if err := im.dst.Create(&models.MaintenanceComponent{MaintenanceID: s.ID, ComponentID: id, PreviousStatus: comp.Status}).Error; err != nil {
				return err
			}
		}
		im.report.imported("maintenances")
	}
	return nil
}

func (im *importer) subscribers() error {
	var subscribers []cachetSubscriber
	if err := im.src.Order("id").Find(&subscribers).Error; err != nil {
		return err
	}
	var subs []cachetSubscription
	if err := im.src.Find(&subs).Error; err != nil {
		return err
	}
	followed := map[uint][]uint{}
	for _, s := range subs {
		if im.components[s.ComponentID] {
			followed[s.SubscriberID] = append(followed[s.SubscriberID], s.ComponentID)
		}
	}
	now := time.Now()
	for _, s := range subscribers {
		label := fmt.Sprintf("subscriber %d", s.ID)
		email := strings.ToLower(strings.TrimSpace(s.Email))
		if email == "" {
			if s.PhoneNumber != nil && *s.PhoneNumber != "" {
				im.report.skip("phone-only subscriber (Vigie sends email only)", label)
			} else if s.SlackWebhookURL != nil && *s.SlackWebhookURL != "" {
				im.report.skip("Slack-only subscriber (Vigie sends email only)", label)
			} else {
				im.report.skip("no email address", label)
			}
			continue
		}
		unsubscribe, err := randomToken()
		if err != nil {
			return err
		}
		confirmation := ""
		if s.VerifyCode != nil {
			confirmation = *s.VerifyCode
		}
		global := s.Global
		ids := followed[s.ID]
		if !global && len(ids) == 0 {
			global = true
			im.report.adapt("followed no imported component, now follows everything", label)
		}
		ns := models.Subscriber{ID: s.ID, Email: email, Locale: im.locale, Confirmed: s.VerifiedAt != nil, Global: global,
			ConfirmationToken: confirmation, UnsubscribeToken: unsubscribe,
			CreatedAt: timeOr(s.CreatedAt, now), UpdatedAt: timeOr(s.UpdatedAt, now)}
		if err := im.create(&ns, map[string]bool{"global": global}); err != nil {
			return err
		}
		if !global {
			for _, id := range ids {
				if err := im.dst.Create(&models.Subscription{SubscriberID: s.ID, ComponentID: id}).Error; err != nil {
					return err
				}
			}
		}
		if !ns.Confirmed {
			im.report.adapt("never confirmed in Cachet, imported unconfirmed (receives nothing)", label)
		}
		if (s.PhoneNumber != nil && *s.PhoneNumber != "") || (s.SlackWebhookURL != nil && *s.SlackWebhookURL != "") {
			im.report.adapt("phone number or Slack webhook left out (Vigie sends email only)", label)
		}
		im.report.imported("subscribers")
	}
	return nil
}

// applySettings carries the page name over as the Vigie title, and lists
// the settings to redo by hand.
func (im *importer) applySettings() error {
	if name := strings.TrimSpace(im.settings["app_name"]); name != "" {
		branding, err := json.Marshal(map[string]string{"title_en": name, "title_fr": name})
		if err != nil {
			return err
		}
		var setting models.Settings
		err = im.dst.Where(&models.Settings{Key: "branding"}).First(&setting).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := im.dst.Create(&models.Settings{Key: "branding", Value: string(branding)}).Error; err != nil {
				return err
			}
			im.report.imported("settings")
		case err == nil:
			im.report.note("Vigie already has a branding: the Cachet page name %q was not applied.", name)
		default:
			return err
		}
	}
	if d := im.settings["app_domain"]; d != "" {
		im.report.note("Cachet was served at %s: set PUBLIC_URL to the address of the Vigie status page.", d)
	}
	if im.settings["app_about"] != "" {
		im.report.note("The Cachet \"about\" text has no equivalent in Vigie and was not imported.")
	}
	im.report.note("Text from Cachet was stored in %q: add the other language in the admin where needed.", im.locale)
	return nil
}

// leftovers reports Cachet data the importer does not bring over.
func (im *importer) leftovers() error {
	for _, t := range []struct {
		table, reason string
	}{
		{"metrics", "metrics (Vigie has no metrics)"},
		{"metric_points", "metric points (Vigie has no metrics)"},
		{"incident_templates", "incident templates (Cachet's use Twig; recreate them in Vigie)"},
		{"invites", "pending user invitations"},
	} {
		if !im.src.Migrator().HasTable(t.table) {
			continue
		}
		var n int64
		if err := im.src.Table(t.table).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			im.report.LeftOut[t.reason] = n
		}
	}
	return nil
}

// checkZone reports the zone the dates were read in, and warns when Cachet
// showed its times in another zone than the one they are read in: its
// APP_TIMEZONE may not be UTC either.
func (im *importer) checkZone() {
	if im.zone != nil && im.zone != time.UTC {
		im.report.note("Cachet's dates were read as %s times (--source-timezone). Set CACHET_API_TIMEZONE=%s, so that integrations see the same times as in Cachet.", im.zone, im.zone)
		return
	}
	if tz := im.settings["app_timezone"]; tz != "" && tz != "UTC" {
		im.report.note("Cachet's dates were read as UTC, but Cachet showed them in %s. If Cachet's .env sets APP_TIMEZONE to anything but UTC, run the import again with --source-timezone set to that value.", tz)
	}
}
