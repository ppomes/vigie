package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"api/internal/models"
	"api/internal/testutil"
)

func TestGeneralIncidentWithoutComponents(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")

	id := createdID(t, call(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "Planned DNS change", "status": "monitoring",
	}), "incident")
	var incident models.Incident
	db.First(&incident, id)
	if incident.ComponentID != nil {
		t.Fatalf("general incident has main component %d", *incident.ComponentID)
	}
	if got := affectedIDs(t, db, id); len(got) != 0 {
		t.Fatalf("general incident affects %v", got)
	}

	// A legacy component_status without a component is ignored, and posting
	// an update works.
	call(t, r, http.MethodPut, fmt.Sprintf("/incidents/%d", id), map[string]any{"component_status": "major_outage"})
	wantStatus(t, db, c.ID, "operational")
	call(t, r, http.MethodPost, fmt.Sprintf("/incidents/%d/updates", id), map[string]any{
		"status": "resolved", "translations": messages("Done", ""), "component_status": "major_outage",
	})
	wantStatus(t, db, c.ID, "operational")

	// It shows on the public page.
	var resp struct {
		Incidents []models.Incident `json:"incidents"`
	}
	get(t, r, "/public/incidents", &resp)
	if len(resp.Incidents) != 1 || resp.Incidents[0].ComponentID != nil {
		t.Fatalf("public general incident: %+v", resp.Incidents)
	}

	// Attaching a component later works, and removing them all again too.
	call(t, r, http.MethodPut, fmt.Sprintf("/incidents/%d", id), map[string]any{"component_id": c.ID})
	db.First(&incident, id)
	if models.IDValue(incident.ComponentID) != c.ID {
		t.Fatalf("main component after attach: %v", incident.ComponentID)
	}
	call(t, r, http.MethodPut, fmt.Sprintf("/incidents/%d", id), map[string]any{"components": []map[string]any{}})
	var after models.Incident
	db.First(&after, id)
	if after.ComponentID != nil || len(affectedIDs(t, db, id)) != 0 {
		t.Fatalf("after removing all components: main %v, affected %v", after.ComponentID, affectedIDs(t, db, id))
	}
}

func TestGeneralMaintenanceWithoutComponents(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")
	start, end := maintenanceWindow()

	id := createdID(t, call(t, r, http.MethodPost, "/maintenances", map[string]any{
		"name": "Office move", "status": "in_progress", "start_at": start, "end_at": end,
	}), "maintenance")
	wantStatus(t, db, c.ID, "operational")

	var pub struct {
		Maintenances []struct {
			ComponentID *uint    `json:"component_id"`
			Components  []string `json:"components"`
		} `json:"maintenances"`
	}
	get(t, r, "/public/maintenances", &pub)
	if len(pub.Maintenances) != 1 || pub.Maintenances[0].ComponentID != nil || len(pub.Maintenances[0].Components) != 0 {
		t.Fatalf("public general maintenance: %+v", pub.Maintenances)
	}

	call(t, r, http.MethodPut, fmt.Sprintf("/maintenances/%d", id), map[string]any{"status": "completed"})
	wantStatus(t, db, c.ID, "operational")
}
