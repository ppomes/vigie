package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api/internal/models"
	"api/internal/testutil"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func messages(en, fr string) []map[string]string {
	var t []map[string]string
	if en != "" {
		t = append(t, map[string]string{"locale": "en", "field": "message", "value": en})
	}
	if fr != "" {
		t = append(t, map[string]string{"locale": "fr", "field": "message", "value": fr})
	}
	return t
}

func newIncident(t *testing.T, r *gin.Engine, componentID uint) uint {
	t.Helper()
	w := call(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "Slow API", "status": "investigating", "component_id": componentID,
	})
	return createdID(t, w, "incident")
}

func incidentStatus(t *testing.T, db *gorm.DB, id uint) string {
	t.Helper()
	var i models.Incident
	if err := db.First(&i, id).Error; err != nil {
		t.Fatal(err)
	}
	return i.Status
}

func postUpdate(t *testing.T, r *gin.Engine, incidentID uint, body map[string]any) uint {
	t.Helper()
	w := call(t, r, http.MethodPost, fmt.Sprintf("/incidents/%d/updates", incidentID), body)
	return createdID(t, w, "update")
}

func TestIncidentTimeline(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")
	id := newIncident(t, r, c.ID)

	postUpdate(t, r, id, map[string]any{"status": "identified", "translations": messages("Cause found", "Cause trouvée")})
	// Keep created_at strictly increasing, as in real use.
	time.Sleep(10 * time.Millisecond)
	postUpdate(t, r, id, map[string]any{"status": "monitoring", "translations": messages("Fix deployed", "Correctif déployé")})

	if got := incidentStatus(t, db, id); got != "monitoring" {
		t.Fatalf("incident status %q, want the latest update's %q", got, "monitoring")
	}
	wantStatus(t, db, c.ID, "operational")

	// The public page gets the timeline, newest first, in the requested locale.
	req := httptest.NewRequest(http.MethodGet, "/public/incidents?locale=fr", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp struct {
		Incidents []models.Incident `json:"incidents"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Incidents) != 1 || len(resp.Incidents[0].Updates) != 2 {
		t.Fatalf("want one incident with two updates, got %s", w.Body.String())
	}
	first := resp.Incidents[0].Updates[0]
	if first.Status != "monitoring" || len(first.Translations) != 1 || first.Translations[0].Value != "Correctif déployé" {
		t.Fatalf("newest update first, in French: got %+v", first)
	}
	if bytes.Contains(w.Body.Bytes(), []byte(`"user":`)) {
		t.Fatalf("public timeline must not expose authors: %s", w.Body.String())
	}
}

func TestIncidentUpdateValidation(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")
	id := newIncident(t, r, c.ID)

	for name, body := range map[string]map[string]any{
		"no message":    {"status": "identified"},
		"empty message": {"status": "identified", "translations": []map[string]string{{"locale": "en", "field": "message", "value": "  "}}},
		"wrong field":   {"status": "identified", "translations": []map[string]string{{"locale": "en", "field": "name", "value": "x"}}},
		"bad status":    {"status": "fixed", "translations": messages("x", "")},
	} {
		var buf bytes.Buffer
		json.NewEncoder(&buf).Encode(body)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/incidents/%d/updates", id), &buf)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want %d", name, w.Code, http.StatusBadRequest)
		}
	}
	if got := incidentStatus(t, db, id); got != "investigating" {
		t.Fatalf("rejected updates changed the incident status to %q", got)
	}
}

func TestIncidentUpdateSetsComponentStatusWhenAsked(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "major_outage")
	id := newIncident(t, r, c.ID)

	postUpdate(t, r, id, map[string]any{"status": "resolved", "translations": messages("Fixed", ""), "component_status": "operational"})
	wantStatus(t, db, c.ID, "operational")
}

func TestEditAndDeleteIncidentUpdates(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")
	id := newIncident(t, r, c.ID)
	other := newIncident(t, r, c.ID)

	first := postUpdate(t, r, id, map[string]any{"status": "identified", "translations": messages("Cause found", "")})
	time.Sleep(10 * time.Millisecond)
	latest := postUpdate(t, r, id, map[string]any{"status": "resolved", "translations": messages("Fixed", "")})

	// Editing the latest update's status moves the incident status with it.
	call(t, r, http.MethodPut, fmt.Sprintf("/incidents/%d/updates/%d", id, latest), map[string]any{"status": "monitoring"})
	if got := incidentStatus(t, db, id); got != "monitoring" {
		t.Fatalf("after edit: incident status %q, want %q", got, "monitoring")
	}

	// An update cannot be reached through another incident.
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/incidents/%d/updates/%d", other, latest), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("update through another incident: status %d, want %d", w.Code, http.StatusNotFound)
	}

	// Deleting the latest update falls back to the previous one.
	call(t, r, http.MethodDelete, fmt.Sprintf("/incidents/%d/updates/%d", id, latest), nil)
	if got := incidentStatus(t, db, id); got != "identified" {
		t.Fatalf("after delete: incident status %q, want %q", got, "identified")
	}

	// Deleting the incident removes its timeline and messages.
	call(t, r, http.MethodDelete, fmt.Sprintf("/incidents/%d", id), nil)
	var updates, translations int64
	db.Model(&models.IncidentUpdate{}).Where("incident_id = ?", id).Count(&updates)
	db.Model(&models.Translation{}).Where("entity_type = ? AND entity_id = ?", "incident_update", first).Count(&translations)
	if updates != 0 || translations != 0 {
		t.Fatalf("after incident delete: %d updates and %d messages left", updates, translations)
	}
}
