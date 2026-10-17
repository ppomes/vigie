package cachetapi

import (
	"fmt"
	"sort"

	"api/internal/models"

	"github.com/gin-gonic/gin"
)

// translation returns a translated field in the content locale, falling
// back to English, then to any locale.
func translation(trs []models.Translation, field string) string {
	byLocale := map[string]string{}
	for _, t := range trs {
		if t.Field == field && t.Value != "" {
			byLocale[t.Locale] = t.Value
		}
	}
	for _, l := range []string{contentLocale(), "en"} {
		if v, ok := byLocale[l]; ok {
			return v
		}
	}
	locales := make([]string, 0, len(byLocale))
	for l := range byLocale {
		locales = append(locales, l)
	}
	sort.Strings(locales)
	if len(locales) > 0 {
		return byLocale[locales[0]]
	}
	return ""
}

func idOrZero(id *uint) uint { return models.IDValue(id) }

func presentComponent(c models.Component) gin.H {
	tags := gin.H{}
	for _, t := range c.Tags {
		tags[t.Slug] = t.Name
	}
	var meta any
	if len(c.Meta) > 0 {
		meta = c.Meta
	}
	var groupID any
	if c.GroupID != nil {
		groupID = *c.GroupID
	}
	return gin.H{
		"id":          c.ID,
		"name":        c.Name,
		"description": translation(c.Translations, "description"),
		"link":        c.Link,
		"status":      cachetComponentStatus(c.Status),
		"order":       c.Order,
		"group_id":    groupID,
		"enabled":     c.Enabled,
		"meta":        meta,
		"created_at":  cachetTime(c.CreatedAt),
		"updated_at":  cachetTime(c.UpdatedAt),
		"deleted_at":  nil,
		"status_name": cachetComponentStatusName(c.Status),
		"tags":        tags,
	}
}

func presentGroup(g models.ComponentGroup) gin.H {
	lowest := 0
	for _, c := range g.Components {
		if s := cachetComponentStatus(c.Status); s > lowest {
			lowest = s
		}
	}
	components := make([]gin.H, 0, len(g.Components))
	for _, c := range g.Components {
		components = append(components, presentComponent(c))
	}
	return gin.H{
		"id":                  g.ID,
		"name":                g.Name,
		"order":               g.Order,
		"collapsed":           groupCollapseFromVigie[g.Collapse],
		"visible":             groupVisibleFromVigie[g.Visibility],
		"created_at":          cachetTime(g.CreatedAt),
		"updated_at":          cachetTime(g.UpdatedAt),
		"lowest_human_status": componentStatusNames[lowest],
		"enabled_components":  components,
	}
}

func presentUpdate(u models.IncidentUpdate) gin.H {
	status := incidentStatusFromVigie[u.Status]
	return gin.H{
		"id":           u.ID,
		"incident_id":  u.IncidentID,
		"status":       status,
		"message":      translation(u.Translations, "message"),
		"user_id":      idOrZero(u.UserID),
		"created_at":   cachetTime(u.CreatedAt),
		"updated_at":   cachetTime(u.UpdatedAt),
		"human_status": incidentStatusNames[status],
		"permalink":    fmt.Sprintf("%s/#incident-%d", publicURL(), u.IncidentID),
	}
}

// presentIncident renders an incident. Its updates must be loaded newest
// first.
func presentIncident(i models.Incident) gin.H {
	status := incidentStatusFromVigie[i.Status]
	visible := 0
	if i.Visible {
		visible = 1
	}
	occurred := i.CreatedAt
	if i.OccurredAt != nil {
		occurred = *i.OccurredAt
	}
	h := gin.H{
		"id":                  i.ID,
		"component_id":        idOrZero(i.ComponentID),
		"name":                i.Name,
		"status":              status,
		"visible":             visible,
		"stickied":            i.Stickied,
		"notifications":       i.Notifications,
		"message":             translation(i.Translations, "message"),
		"user_id":             idOrZero(i.UserID),
		"occurred_at":         cachetTime(occurred),
		"created_at":          cachetTime(i.CreatedAt),
		"updated_at":          cachetTime(i.UpdatedAt),
		"deleted_at":          nil,
		"human_status":        incidentStatusNames[status],
		"latest_update_id":    nil,
		"latest_status":       status,
		"latest_human_status": incidentStatusNames[status],
		"permalink":           fmt.Sprintf("%s/#incident-%d", publicURL(), i.ID),
		"meta":                gin.H{},
	}
	if len(i.Updates) > 0 {
		h["latest_update_id"] = i.Updates[0].ID
	}
	return h
}

func presentSchedule(m models.Maintenance) gin.H {
	status := scheduleStatusFromVigie[m.Status]
	components := make([]gin.H, 0, len(m.Components))
	for _, mc := range m.Components {
		if mc.Component != nil {
			components = append(components, presentComponent(*mc.Component))
		}
	}
	return gin.H{
		"id":           m.ID,
		"name":         m.Name,
		"message":      translation(m.Translations, "message"),
		"status":       status,
		"scheduled_at": cachetTime(m.StartAt),
		"completed_at": cachetTime(m.EndAt),
		"created_at":   cachetTime(m.CreatedAt),
		"updated_at":   cachetTime(m.UpdatedAt),
		"deleted_at":   nil,
		"human_status": scheduleStatusNames[status],
		"components":   components,
	}
}
