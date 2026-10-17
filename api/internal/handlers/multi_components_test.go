package handlers

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"api/internal/models"
	"api/internal/testutil"

	"gorm.io/gorm"
)

func affectedIDs(t *testing.T, db *gorm.DB, incidentID uint) []uint {
	t.Helper()
	var rows []models.IncidentComponent
	db.Where("incident_id = ?", incidentID).Order("component_id").Find(&rows)
	ids := []uint{}
	for _, r := range rows {
		ids = append(ids, r.ComponentID)
	}
	return ids
}

func TestIncidentAffectsSeveralComponents(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	api := testutil.CreateComponent(t, db, "API", "operational")
	web := testutil.CreateComponent(t, db, "Website", "operational")
	dbc := testutil.CreateComponent(t, db, "Database", "operational")

	id := createdID(t, call(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "DB down", "status": "investigating",
		"components": []map[string]any{
			{"component_id": dbc.ID, "status": "major_outage"},
			{"component_id": api.ID, "status": "partial_outage"},
			{"component_id": web.ID},
		},
	}), "incident")

	wantStatus(t, db, dbc.ID, "major_outage")
	wantStatus(t, db, api.ID, "partial_outage")
	wantStatus(t, db, web.ID, "operational")
	var incident models.Incident
	db.First(&incident, id)
	if incident.ComponentID != dbc.ID {
		t.Fatalf("main component %d, want the first listed %d", incident.ComponentID, dbc.ID)
	}

	// The public page lists every affected component.
	var resp struct {
		Incidents []models.Incident `json:"incidents"`
	}
	get(t, r, "/public/incidents", &resp)
	if len(resp.Incidents) != 1 || len(resp.Incidents[0].Components) != 3 {
		t.Fatalf("public incident components: %+v", resp.Incidents)
	}

	// Replacing the list drops the website; it keeps its status.
	call(t, r, http.MethodPut, fmt.Sprintf("/incidents/%d", id), map[string]any{
		"components": []map[string]any{{"component_id": dbc.ID}, {"component_id": api.ID, "status": "operational"}},
	})
	if got := fmt.Sprint(affectedIDs(t, db, id)); got != fmt.Sprint([]uint{api.ID, dbc.ID}) {
		t.Fatalf("affected after update: %s", got)
	}
	wantStatus(t, db, api.ID, "operational")
	wantStatus(t, db, dbc.ID, "major_outage")

	// An update sets statuses on affected components only.
	call(t, r, http.MethodPost, fmt.Sprintf("/incidents/%d/updates", id), map[string]any{
		"status": "resolved", "translations": messages("Fixed", ""),
		"components": []map[string]any{{"component_id": dbc.ID, "status": "operational"}},
	})
	wantStatus(t, db, dbc.ID, "operational")
	if code := status(t, r, http.MethodPost, fmt.Sprintf("/incidents/%d/updates", id), map[string]any{
		"status": "resolved", "translations": messages("x", ""),
		"components": []map[string]any{{"component_id": web.ID, "status": "major_outage"}},
	}); code != http.StatusBadRequest {
		t.Fatalf("update on a component outside the incident: status %d, want %d", code, http.StatusBadRequest)
	}
	wantStatus(t, db, web.ID, "operational")

	// Unknown components are refused.
	if code := status(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "x", "status": "investigating", "components": []map[string]any{{"component_id": 999}},
	}); code != http.StatusBadRequest {
		t.Fatalf("unknown component: status %d, want %d", code, http.StatusBadRequest)
	}

	call(t, r, http.MethodDelete, fmt.Sprintf("/incidents/%d", id), nil)
	if left := affectedIDs(t, db, id); len(left) != 0 {
		t.Fatalf("deleted incident left components %v", left)
	}
}

func TestLegacyIncidentRequestsStillWork(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	a := testutil.CreateComponent(t, db, "A", "operational")
	b := testutil.CreateComponent(t, db, "B", "operational")

	id := createdID(t, call(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "x", "status": "investigating", "component_id": a.ID, "component_status": "partial_outage",
	}), "incident")
	wantStatus(t, db, a.ID, "partial_outage")
	if got := affectedIDs(t, db, id); len(got) != 1 || got[0] != a.ID {
		t.Fatalf("affected: %v", got)
	}

	// Moving the main component replaces the set.
	call(t, r, http.MethodPut, fmt.Sprintf("/incidents/%d", id), map[string]any{"component_id": b.ID})
	if got := affectedIDs(t, db, id); len(got) != 1 || got[0] != b.ID {
		t.Fatalf("affected after move: %v", got)
	}
	wantStatus(t, db, a.ID, "partial_outage")
}

func maintenanceWindow() (string, string) {
	now := time.Now()
	return now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)
}

func TestMaintenanceCoversSeveralComponents(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	a := testutil.CreateComponent(t, db, "A", "partial_outage")
	b := testutil.CreateComponent(t, db, "B", "operational")
	c := testutil.CreateComponent(t, db, "C", "degraded_performance")
	start, end := maintenanceWindow()

	id := createdID(t, call(t, r, http.MethodPost, "/maintenances", map[string]any{
		"name": "Upgrade", "status": "in_progress", "component_ids": []uint{a.ID, b.ID},
		"start_at": start, "end_at": end,
	}), "maintenance")
	wantStatus(t, db, a.ID, "under_maintenance")
	wantStatus(t, db, b.ID, "under_maintenance")

	var pub struct {
		Maintenances []struct {
			Components []string `json:"components"`
		} `json:"maintenances"`
	}
	get(t, r, "/public/maintenances", &pub)
	if len(pub.Maintenances) != 1 || fmt.Sprint(pub.Maintenances[0].Components) != "[A B]" {
		t.Fatalf("public maintenance components: %+v", pub.Maintenances)
	}

	// While in progress, swap B for C: B is restored, C goes under.
	call(t, r, http.MethodPut, fmt.Sprintf("/maintenances/%d", id), map[string]any{"component_ids": []uint{a.ID, c.ID}})
	wantStatus(t, db, a.ID, "under_maintenance")
	wantStatus(t, db, b.ID, "operational")
	wantStatus(t, db, c.ID, "under_maintenance")

	// Completing restores each component's own previous status.
	call(t, r, http.MethodPut, fmt.Sprintf("/maintenances/%d", id), map[string]any{"status": "completed"})
	wantStatus(t, db, a.ID, "partial_outage")
	wantStatus(t, db, c.ID, "degraded_performance")
}

func TestOverlappingMaintenancesOnDifferentSets(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	a := testutil.CreateComponent(t, db, "A", "operational")
	b := testutil.CreateComponent(t, db, "B", "partial_outage")
	start, end := maintenanceWindow()

	m1 := createdID(t, call(t, r, http.MethodPost, "/maintenances", map[string]any{
		"name": "M1", "status": "in_progress", "component_ids": []uint{a.ID, b.ID}, "start_at": start, "end_at": end,
	}), "maintenance")
	m2 := createdID(t, call(t, r, http.MethodPost, "/maintenances", map[string]any{
		"name": "M2", "status": "in_progress", "component_id": b.ID, "start_at": start, "end_at": end,
	}), "maintenance")

	call(t, r, http.MethodPut, fmt.Sprintf("/maintenances/%d", m1), map[string]any{"status": "completed"})
	wantStatus(t, db, a.ID, "operational")
	wantStatus(t, db, b.ID, "under_maintenance")

	call(t, r, http.MethodDelete, fmt.Sprintf("/maintenances/%d", m2), nil)
	wantStatus(t, db, b.ID, "partial_outage")
}

func TestHiddenComponentsStayOffPublicIncidentsAndMaintenances(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	shown := testutil.CreateComponent(t, db, "Website", "operational")
	hidden := createdID(t, call(t, r, http.MethodPost, "/components", map[string]any{
		"name": "Secret", "status": "operational", "enabled": false,
	}), "component")
	start, end := maintenanceWindow()

	call(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "Both", "status": "investigating",
		"components": []map[string]any{{"component_id": hidden}, {"component_id": shown.ID}},
	})
	call(t, r, http.MethodPost, "/maintenances", map[string]any{
		"name": "Both", "status": "scheduled", "component_ids": []uint{hidden, shown.ID}, "start_at": start, "end_at": end,
	})
	call(t, r, http.MethodPost, "/maintenances", map[string]any{
		"name": "Hidden only", "status": "scheduled", "component_id": hidden, "start_at": start, "end_at": end,
	})

	var incidents struct {
		Incidents []models.Incident `json:"incidents"`
	}
	get(t, r, "/public/incidents", &incidents)
	if len(incidents.Incidents) != 1 {
		t.Fatalf("public incidents: %d, want 1", len(incidents.Incidents))
	}
	i := incidents.Incidents[0]
	if i.Component.Name != "" || len(i.Components) != 1 || i.Components[0].ComponentID != shown.ID {
		t.Fatalf("public incident components: main %q, affected %+v", i.Component.Name, i.Components)
	}

	var maintenances struct {
		Maintenances []struct {
			Name       string   `json:"name"`
			Component  string   `json:"component"`
			Components []string `json:"components"`
		} `json:"maintenances"`
	}
	get(t, r, "/public/maintenances", &maintenances)
	got := map[string]string{}
	for _, m := range maintenances.Maintenances {
		got[m.Name] = m.Component + fmt.Sprint(m.Components)
	}
	if got["Both"] != "[Website]" || got["Hidden only"] != "[]" {
		t.Fatalf("public maintenances: %v", got)
	}
}
