package handlers

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"api/internal/models"
	"api/internal/testutil"
)

func TestComponentLinkMetaTagsEnabled(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)

	id := createdID(t, call(t, r, http.MethodPost, "/components", map[string]any{
		"name": "API", "status": "operational",
		"link": "https://api.example.com", "enabled": false,
		"meta": map[string]string{"zabbix_host": "api-01"},
		"tags": []string{"Production", "production", "EU West"},
	}), "component")

	var c models.Component
	db.Preload("Tags").First(&c, id)
	if c.Link != "https://api.example.com" || c.Enabled || c.Meta["zabbix_host"] != "api-01" {
		t.Fatalf("fields not stored: %+v", c)
	}
	if len(c.Tags) != 2 {
		t.Fatalf("tags with the same slug should merge: %+v", c.Tags)
	}

	// Absent fields are left alone; explicit values replace.
	call(t, r, http.MethodPut, fmt.Sprintf("/components/%d", id), map[string]any{"name": "Public API"})
	db.Preload("Tags").First(&c, id)
	if c.Enabled || len(c.Tags) != 2 || c.Meta["zabbix_host"] != "api-01" {
		t.Fatalf("unrelated update changed fields: %+v", c)
	}
	call(t, r, http.MethodPut, fmt.Sprintf("/components/%d", id), map[string]any{
		"enabled": true, "tags": []string{}, "meta": map[string]string{}, "link": "",
	})
	var after models.Component
	db.Preload("Tags").First(&after, id)
	if !after.Enabled || len(after.Tags) != 0 || len(after.Meta) != 0 || after.Link != "" {
		t.Fatalf("explicit values not applied: %+v", after)
	}

	for name, body := range map[string]map[string]any{
		"javascript link": {"name": "x", "status": "operational", "link": "javascript:alert(1)"},
		"link, no host":   {"name": "x", "status": "operational", "link": "https:// "},
		"empty tag":       {"name": "x", "status": "operational", "tags": []string{"--"}},
	} {
		if code := status(t, r, http.MethodPost, "/components", body); code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want %d", name, code, http.StatusBadRequest)
		}
	}
}

func TestDisabledComponentsStayOffThePublicPage(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	group := createGroup(t, r, map[string]any{"name": "Web"})
	call(t, r, http.MethodPost, "/components", map[string]any{"name": "Old", "status": "major_outage", "enabled": false, "group_id": group})
	call(t, r, http.MethodPost, "/components", map[string]any{"name": "Website", "status": "operational", "group_id": group})

	var comps struct {
		Components []models.Component `json:"components"`
	}
	get(t, r, "/public/components", &comps)
	if len(comps.Components) != 1 || comps.Components[0].Name != "Website" {
		t.Fatalf("public components: %+v", comps.Components)
	}
	var groups struct {
		Groups []models.ComponentGroup `json:"component_groups"`
	}
	get(t, r, "/public/component-groups", &groups)
	if len(groups.Groups) != 1 || len(groups.Groups[0].Components) != 1 {
		t.Fatalf("disabled component inside a public group: %+v", groups.Groups)
	}
	// A disabled component in an outage does not make the page red.
	var st StatusResponse
	get(t, r, "/public/status", &st)
	if st.Status != "operational" || st.Components != 1 {
		t.Fatalf("status: %+v", st)
	}
}

func TestIncidentVisibilityStickinessAndOccurredAt(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")
	day := func(d int) string { return time.Date(2026, 1, d, 12, 0, 0, 0, time.UTC).Format(time.RFC3339) }

	call(t, r, http.MethodPost, "/incidents", map[string]any{"name": "Old", "status": "resolved", "component_id": c.ID, "occurred_at": day(1)})
	call(t, r, http.MethodPost, "/incidents", map[string]any{"name": "Pinned", "status": "resolved", "component_id": c.ID, "occurred_at": day(2), "stickied": true})
	call(t, r, http.MethodPost, "/incidents", map[string]any{"name": "Recent", "status": "investigating", "component_id": c.ID, "occurred_at": day(3)})
	call(t, r, http.MethodPost, "/incidents", map[string]any{"name": "Internal", "status": "investigating", "component_id": c.ID, "visible": false})

	var resp struct {
		Incidents []models.Incident `json:"incidents"`
	}
	get(t, r, "/public/incidents", &resp)
	var names []string
	for _, i := range resp.Incidents {
		names = append(names, i.Name)
	}
	if fmt.Sprint(names) != "[Pinned Recent Old]" {
		t.Fatalf("public incidents %v, want [Pinned Recent Old]", names)
	}

	// The internal incident is still listed in the admin, and does not
	// count on the public status.
	get(t, r, "/incidents", &resp)
	if len(resp.Incidents) != 4 {
		t.Fatalf("admin incidents: %d, want 4", len(resp.Incidents))
	}
	var st StatusResponse
	get(t, r, "/public/status", &st)
	if st.Incidents != 1 {
		t.Fatalf("active public incidents %d, want 1", st.Incidents)
	}

	// Leaving occurred_at out keeps it; null clears it.
	var old models.Incident
	db.Where("name = ?", "Old").First(&old)
	path := fmt.Sprintf("/incidents/%d", old.ID)
	call(t, r, http.MethodPut, path, map[string]any{"name": "Old"})
	db.First(&old, old.ID)
	if old.OccurredAt == nil || old.OccurredAt.Day() != 1 {
		t.Fatalf("occurred_at %v after an update without it, want Jan 1", old.OccurredAt)
	}
	call(t, r, http.MethodPut, path, map[string]any{"occurred_at": nil})
	var cleared models.Incident // fresh: GORM leaves a set pointer alone on NULL
	db.First(&cleared, old.ID)
	if cleared.OccurredAt != nil {
		t.Fatalf("occurred_at %v after null, want NULL", cleared.OccurredAt)
	}
}

func TestIncidentNotifyFlag(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")

	quiet := createdID(t, call(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "Quiet", "status": "investigating", "component_id": c.ID, "notify": false,
	}), "incident")
	loud := createdID(t, call(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "Loud", "status": "investigating", "component_id": c.ID,
	}), "incident")

	var q, l models.Incident
	db.First(&q, quiet)
	db.First(&l, loud)
	if q.Notifications || !l.Notifications {
		t.Fatalf("notifications flags: quiet=%v loud=%v", q.Notifications, l.Notifications)
	}
	if q.OccurredAt == nil {
		t.Fatal("occurred_at should default to the creation time")
	}
}
