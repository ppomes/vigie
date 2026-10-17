package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api/internal/testutil"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// newRouter wires the handlers under test without authentication, which is
// covered by the middleware tests.
func newRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/components", CreateComponent(db))
	r.PUT("/components/:id", UpdateComponent(db))
	r.GET("/components", ListComponents(db))
	r.GET("/component-groups", ListComponentGroups(db))
	r.POST("/component-groups", CreateComponentGroup(db))
	r.PUT("/component-groups/:id", UpdateComponentGroup(db))
	r.DELETE("/component-groups/:id", DeleteComponentGroup(db))
	r.GET("/public/components", GetComponents(db))
	r.GET("/public/component-groups", GetComponentGroups(db))
	r.POST("/incidents", CreateIncident(db, nil))
	r.PUT("/incidents/:id", UpdateIncident(db, nil))
	r.DELETE("/incidents/:id", DeleteIncident(db))
	r.POST("/incidents/:id/updates", CreateIncidentUpdate(db, nil))
	r.PUT("/incidents/:id/updates/:update_id", UpdateIncidentUpdate(db))
	r.DELETE("/incidents/:id/updates/:update_id", DeleteIncidentUpdate(db))
	r.GET("/public/incidents", GetIncidents(db))
	r.GET("/public/status", GetStatus(db))
	r.GET("/public/maintenances", GetMaintenances(db))
	r.POST("/subscribe", Subscribe(db, nil))
	r.GET("/subscriptions", GetSubscription(db))
	r.PUT("/subscriptions", UpdateSubscription(db))
	r.GET("/incidents", ListIncidents(db))
	r.POST("/maintenances", CreateMaintenance(db, nil))
	r.PUT("/maintenances/:id", UpdateMaintenance(db, nil))
	r.DELETE("/maintenances/:id", DeleteMaintenance(db))
	return r
}

func call(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code >= 300 {
		t.Fatalf("%s %s: status %d, body %s", method, path, w.Code, w.Body.String())
	}
	return w
}

func createdID(t *testing.T, w *httptest.ResponseRecorder, key string) uint {
	t.Helper()
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var obj struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(resp[key], &obj); err != nil || obj.ID == 0 {
		t.Fatalf("no %q id in response %s", key, w.Body.String())
	}
	return obj.ID
}

func wantStatus(t *testing.T, db *gorm.DB, id uint, want string) {
	t.Helper()
	if got := testutil.ComponentStatus(t, db, id); got != want {
		t.Fatalf("component status: got %q, want %q", got, want)
	}
}

func TestComponentStatusSetByAPIIsKept(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")

	// A monitoring tool reports an outage, with no incident open.
	call(t, r, http.MethodPut, fmt.Sprintf("/components/%d", c.ID), map[string]any{"status": "major_outage"})
	wantStatus(t, db, c.ID, "major_outage")

	// Creating, updating, resolving and deleting an incident that does not
	// ask for a component status leaves it alone.
	w := call(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "Slow API", "status": "investigating", "component_id": c.ID,
	})
	incident := createdID(t, w, "incident")
	wantStatus(t, db, c.ID, "major_outage")

	call(t, r, http.MethodPut, fmt.Sprintf("/incidents/%d", incident), map[string]any{"status": "resolved"})
	wantStatus(t, db, c.ID, "major_outage")

	call(t, r, http.MethodDelete, fmt.Sprintf("/incidents/%d", incident), nil)
	wantStatus(t, db, c.ID, "major_outage")
}

func TestIncidentSetsComponentStatusWhenAsked(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")

	w := call(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "Outage", "status": "investigating", "component_id": c.ID, "component_status": "partial_outage",
	})
	wantStatus(t, db, c.ID, "partial_outage")

	incident := createdID(t, w, "incident")
	call(t, r, http.MethodPut, fmt.Sprintf("/incidents/%d", incident), map[string]any{
		"status": "resolved", "component_status": "operational",
	})
	wantStatus(t, db, c.ID, "operational")
}

func TestIncidentRejectsInvalidComponentStatus(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]any{
		"name": "x", "status": "investigating", "component_id": c.ID, "component_status": "under_maintenance",
	})
	req := httptest.NewRequest(http.MethodPost, "/incidents", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want %d", w.Code, http.StatusBadRequest)
	}
	wantStatus(t, db, c.ID, "operational")
}

func TestMaintenanceHandlersRestoreStatus(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "degraded_performance")
	now := time.Now()

	w := call(t, r, http.MethodPost, "/maintenances", map[string]any{
		"name": "Upgrade", "status": "in_progress", "component_id": c.ID,
		"start_at": now.Format(time.RFC3339), "end_at": now.Add(time.Hour).Format(time.RFC3339),
	})
	wantStatus(t, db, c.ID, "under_maintenance")

	m := createdID(t, w, "maintenance")
	call(t, r, http.MethodPut, fmt.Sprintf("/maintenances/%d", m), map[string]any{"status": "completed"})
	wantStatus(t, db, c.ID, "degraded_performance")
}

func TestMaintenanceMovedToAnotherComponent(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	a := testutil.CreateComponent(t, db, "A", "partial_outage")
	b := testutil.CreateComponent(t, db, "B", "operational")
	now := time.Now()

	w := call(t, r, http.MethodPost, "/maintenances", map[string]any{
		"name": "Upgrade", "status": "in_progress", "component_id": a.ID,
		"start_at": now.Format(time.RFC3339), "end_at": now.Add(time.Hour).Format(time.RFC3339),
	})
	m := createdID(t, w, "maintenance")

	call(t, r, http.MethodPut, fmt.Sprintf("/maintenances/%d", m), map[string]any{"component_id": b.ID})
	wantStatus(t, db, a.ID, "partial_outage")
	wantStatus(t, db, b.ID, "under_maintenance")

	call(t, r, http.MethodDelete, fmt.Sprintf("/maintenances/%d", m), nil)
	wantStatus(t, db, b.ID, "operational")
}

func TestUnchangedMaintenanceUpdateKeepsExplicitStatus(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	c := testutil.CreateComponent(t, db, "API", "operational")
	now := time.Now()

	w := call(t, r, http.MethodPost, "/maintenances", map[string]any{
		"name": "Upgrade", "status": "in_progress", "component_id": c.ID,
		"start_at": now.Format(time.RFC3339), "end_at": now.Add(time.Hour).Format(time.RFC3339),
	})
	m := createdID(t, w, "maintenance")

	call(t, r, http.MethodPut, fmt.Sprintf("/components/%d", c.ID), map[string]any{"status": "major_outage"})
	// Editing the maintenance text must not put the component back under maintenance.
	call(t, r, http.MethodPut, fmt.Sprintf("/maintenances/%d", m), map[string]any{"name": "Upgrade (extended)"})
	wantStatus(t, db, c.ID, "major_outage")
}
