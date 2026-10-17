package database

import "testing"

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
