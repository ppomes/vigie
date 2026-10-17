package database

import (
	"testing"
	"time"

	"api/internal/models"
	"api/internal/testutil"
)

func TestCheckAllowsAll(t *testing.T) {
	// Clauses exactly as MySQL 8.4 returns them in information_schema.
	old := "(`entity_type` in (_utf8mb4\\'component\\',_utf8mb4\\'incident\\',_utf8mb4\\'maintenance\\'))"
	current := "(`entity_type` in (_utf8mb4\\'component\\',_utf8mb4\\'incident\\',_utf8mb4\\'maintenance\\',_utf8mb4\\'incident_update\\'))"
	want := []string{"component", "incident", "maintenance", "incident_update"}

	if checkAllowsAll(old, want) {
		t.Error("old clause reported as complete: the constraint would never be widened")
	}
	if !checkAllowsAll(current, want) {
		t.Error("current clause reported as incomplete: the table would be rebuilt on every start")
	}
	// "incident" must not match inside "incident_update".
	if checkAllowsAll("(`entity_type` in (_utf8mb4\\'incident_update\\'))", []string{"incident"}) {
		t.Error("partial match on a longer value")
	}
}

func TestBackfillAffectedComponents(t *testing.T) {
	db := testutil.NewDB(t)
	c := testutil.CreateComponent(t, db, "API", "under_maintenance")
	incident := models.Incident{Name: "old", Status: "resolved", ComponentID: models.OptionalID(c.ID)}
	if err := db.Create(&incident).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	m := models.Maintenance{Name: "old", Status: models.MaintenanceInProgress, ComponentID: models.OptionalID(c.ID),
		StartAt: now, EndAt: now.Add(time.Hour), PreviousComponentStatus: "partial_outage"}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ { // twice: the backfill is idempotent
		if err := EnsureSchema(db); err != nil {
			t.Fatal(err)
		}
	}

	var ic []models.IncidentComponent
	db.Where("incident_id = ?", incident.ID).Find(&ic)
	if len(ic) != 1 || ic[0].ComponentID != c.ID {
		t.Fatalf("incident components: %+v", ic)
	}
	var mc []models.MaintenanceComponent
	db.Where("maintenance_id = ?", m.ID).Find(&mc)
	if len(mc) != 1 || mc[0].PreviousStatus != "partial_outage" {
		t.Fatalf("maintenance components: %+v", mc)
	}
}
