package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"api/internal/models"
	"api/internal/testutil"

	"github.com/gin-gonic/gin"
)

func createGroup(t *testing.T, r *gin.Engine, body map[string]any) uint {
	t.Helper()
	return createdID(t, call(t, r, http.MethodPost, "/component-groups", body), "component_group")
}

func createComponentIn(t *testing.T, r *gin.Engine, name string, order int, groupID uint) uint {
	t.Helper()
	return createdID(t, call(t, r, http.MethodPost, "/components", map[string]any{
		"name": name, "status": "operational", "order": order, "group_id": groupID,
	}), "component")
}

func get(t *testing.T, r *gin.Engine, path string, out any) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d, body %s", path, w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatal(err)
	}
}

func status(t *testing.T, r *gin.Engine, method, path string, body any) int {
	t.Helper()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestPublicGroupsInOrderWithTranslations(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	backend := createGroup(t, r, map[string]any{"name": "Backend", "order": 2})
	web := createGroup(t, r, map[string]any{
		"name": "Web", "order": 1, "collapse": "operational",
		"translations": []map[string]string{{"locale": "fr", "field": "name", "value": "Site"}},
	})
	createComponentIn(t, r, "Database", 1, backend)
	createComponentIn(t, r, "CDN", 2, web)
	createComponentIn(t, r, "Website", 1, web)

	var resp struct {
		Groups []models.ComponentGroup `json:"component_groups"`
	}
	get(t, r, "/public/component-groups?locale=fr", &resp)
	if len(resp.Groups) != 2 || resp.Groups[0].ID != web || resp.Groups[1].ID != backend {
		t.Fatalf("groups not in order: %+v", resp.Groups)
	}
	g := resp.Groups[0]
	if g.Collapse != "operational" || len(g.Translations) != 1 || g.Translations[0].Value != "Site" {
		t.Fatalf("group fields or French name missing: %+v", g)
	}
	if len(g.Components) != 2 || g.Components[0].Name != "Website" || g.Components[1].Name != "CDN" {
		t.Fatalf("components not in order: %+v", g.Components)
	}
}

func TestHiddenGroupsStayOffThePublicPage(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	internal := createGroup(t, r, map[string]any{"name": "Internal", "visibility": "authenticated"})
	public := createGroup(t, r, map[string]any{"name": "Web"})
	createComponentIn(t, r, "Billing DB", 1, internal)
	createComponentIn(t, r, "Website", 1, public)
	createComponentIn(t, r, "Ungrouped", 1, 0)

	var groups struct {
		Groups []models.ComponentGroup `json:"component_groups"`
	}
	get(t, r, "/public/component-groups", &groups)
	if len(groups.Groups) != 1 || groups.Groups[0].ID != public {
		t.Fatalf("hidden group exposed: %+v", groups.Groups)
	}

	var comps struct {
		Components []models.Component `json:"components"`
	}
	get(t, r, "/public/components", &comps)
	names := map[string]bool{}
	for _, c := range comps.Components {
		names[c.Name] = true
	}
	if names["Billing DB"] || !names["Website"] || !names["Ungrouped"] {
		t.Fatalf("public components: %v", names)
	}

	// The admin still sees everything.
	get(t, r, "/components", &comps)
	if len(comps.Components) != 3 {
		t.Fatalf("admin components: %d, want 3", len(comps.Components))
	}
}

func TestMoveComponentsBetweenGroups(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	web := createGroup(t, r, map[string]any{"name": "Web"})
	c := createComponentIn(t, r, "Website", 1, web)

	groupOf := func() *uint {
		var comp models.Component
		db.First(&comp, c)
		return comp.GroupID
	}

	// Updating another field leaves the group alone.
	call(t, r, http.MethodPut, fmt.Sprintf("/components/%d", c), map[string]any{"name": "Site"})
	if g := groupOf(); g == nil || *g != web {
		t.Fatalf("group changed by an unrelated update: %v", g)
	}
	// 0 removes the component from its group.
	call(t, r, http.MethodPut, fmt.Sprintf("/components/%d", c), map[string]any{"group_id": 0})
	if g := groupOf(); g != nil {
		t.Fatalf("group_id 0 should ungroup, got %d", *g)
	}
	// An unknown group is refused.
	if code := status(t, r, http.MethodPut, fmt.Sprintf("/components/%d", c), map[string]any{"group_id": 999}); code != http.StatusBadRequest {
		t.Fatalf("unknown group: status %d, want %d", code, http.StatusBadRequest)
	}

	// Deleting a group keeps its components, ungrouped.
	call(t, r, http.MethodPut, fmt.Sprintf("/components/%d", c), map[string]any{"group_id": web})
	call(t, r, http.MethodDelete, fmt.Sprintf("/component-groups/%d", web), nil)
	if g := groupOf(); g != nil {
		t.Fatalf("component still in a deleted group: %d", *g)
	}
	var count int64
	db.Model(&models.Translation{}).Where("entity_type = ? AND entity_id = ?", "component_group", web).Count(&count)
	if count != 0 {
		t.Fatalf("deleted group left %d translations", count)
	}
}

func TestComponentGroupValidation(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	for name, body := range map[string]map[string]any{
		"no name":        {"order": 1},
		"bad collapse":   {"name": "Web", "collapse": "sometimes"},
		"bad visibility": {"name": "Web", "visibility": "private"},
		"wrong field":    {"name": "Web", "translations": []map[string]string{{"locale": "fr", "field": "message", "value": "x"}}},
	} {
		if code := status(t, r, http.MethodPost, "/component-groups", body); code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want %d", name, code, http.StatusBadRequest)
		}
	}
}

func TestClearGroupTranslations(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	id := createGroup(t, r, map[string]any{
		"name": "Web", "translations": []map[string]string{{"locale": "fr", "field": "name", "value": "Site"}},
	})
	count := func() int64 {
		var n int64
		db.Model(&models.Translation{}).Where("entity_type = ? AND entity_id = ?", "component_group", id).Count(&n)
		return n
	}
	// Absent keeps them; an empty list clears them.
	call(t, r, http.MethodPut, fmt.Sprintf("/component-groups/%d", id), map[string]any{"order": 3})
	if count() != 1 {
		t.Fatal("an update without translations removed them")
	}
	call(t, r, http.MethodPut, fmt.Sprintf("/component-groups/%d", id), map[string]any{"translations": []any{}})
	if count() != 0 {
		t.Fatal("an empty translations list did not clear them")
	}
}
