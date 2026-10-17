package cachetapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"api/internal/models"
	"api/internal/testutil"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const token = "Ab3dEf6hIj9kLm2nOp5q" // a Cachet 2.4 style key

func setup(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewDB(t)
	admin := models.User{Username: "admin", Email: "admin@example.com", Password: "x", Role: models.RoleAdmin, Active: true}
	db.Create(&admin)
	key := models.NewAPIKey("cachet", token, admin.ID)
	db.Create(&key)
	r := gin.New()
	Register(r, db, nil)
	return r, db
}

type response struct {
	code int
	body map[string]any
	raw  string
}

func send(t *testing.T, r *gin.Engine, method, path string, body any, auth bool) response {
	t.Helper()
	var req *http.Request
	switch b := body.(type) {
	case nil:
		req = httptest.NewRequest(method, Prefix+path, nil)
	case url.Values: // a form, as curl -d sends it
		req = httptest.NewRequest(method, Prefix+path, strings.NewReader(b.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	default:
		var buf bytes.Buffer
		json.NewEncoder(&buf).Encode(b)
		req = httptest.NewRequest(method, Prefix+path, &buf)
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("X-Cachet-Token", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out := map[string]any{}
	json.Unmarshal(w.Body.Bytes(), &out)
	return response{w.Code, out, w.Body.String()}
}

func data(t *testing.T, res response) map[string]any {
	t.Helper()
	d, ok := res.body["data"].(map[string]any)
	if !ok {
		t.Fatalf("status %d, no data object in %s", res.code, res.raw)
	}
	return d
}

func dataList(t *testing.T, res response) []any {
	t.Helper()
	d, ok := res.body["data"].([]any)
	if !ok {
		t.Fatalf("status %d, no data list in %s", res.code, res.raw)
	}
	return d
}

func num(v any) int { f, _ := v.(float64); return int(f) }

func TestGeneralEndpoints(t *testing.T) {
	r, _ := setup(t)
	if res := send(t, r, http.MethodGet, "/ping", nil, false); res.body["data"] != "Pong!" {
		t.Fatalf("ping: %s", res.raw)
	}
	if res := send(t, r, http.MethodGet, "/version", nil, false); res.body["data"] != cachetVersion {
		t.Fatalf("version: %s", res.raw)
	}
}

func TestComponentsLikeACachetClient(t *testing.T) {
	r, db := setup(t)

	res := send(t, r, http.MethodPost, "/components", map[string]any{
		"name": "API", "status": 2, "description": "Public API", "link": "https://api.example.com",
		"tags": "Production,EU", "meta": map[string]any{"zabbix": "api-01"},
	}, true)
	c := data(t, res)
	if num(c["status"]) != 2 || c["status_name"] != "Performance Issues" || c["description"] != "Public API" {
		t.Fatalf("created component: %v", c)
	}
	if tags := c["tags"].(map[string]any); tags["production"] != "Production" || len(tags) != 2 {
		t.Fatalf("tags: %v", c["tags"])
	}
	id := num(c["id"])

	// A French translation added in the admin must survive a Cachet update.
	db.Create(&models.Translation{EntityID: uint(id), EntityType: "component", Locale: "fr", Field: "description", Value: "API publique"})

	// curl-style form update.
	res = send(t, r, http.MethodPut, "/components/"+itoa(id), url.Values{"status": {"4"}}, true)
	if c = data(t, res); num(c["status"]) != 4 || c["description"] != "Public API" {
		t.Fatalf("updated component: %v", c)
	}
	var fr int64
	db.Model(&models.Translation{}).Where("entity_id = ? AND locale = ?", id, "fr").Count(&fr)
	if fr != 1 {
		t.Fatal("the French description was lost")
	}

	// A disabled component is hidden from anonymous readers only.
	send(t, r, http.MethodPost, "/components", map[string]any{"name": "Old", "status": 1, "enabled": false}, true)
	if n := len(dataList(t, send(t, r, http.MethodGet, "/components", nil, false))); n != 1 {
		t.Fatalf("anonymous list: %d components, want 1", n)
	}
	res = send(t, r, http.MethodGet, "/components?per_page=1", nil, true)
	if n := len(dataList(t, res)); n != 1 {
		t.Fatalf("page of 1: %d components", n)
	}
	pag := res.body["meta"].(map[string]any)["pagination"].(map[string]any)
	if num(pag["total"]) != 2 || num(pag["total_pages"]) != 2 {
		t.Fatalf("pagination: %v", pag)
	}
	if n := len(dataList(t, send(t, r, http.MethodGet, "/components?status=4", nil, true))); n != 1 {
		t.Fatalf("filter by status: %d components", n)
	}
	if n := len(dataList(t, send(t, r, http.MethodGet, "/components?tags[]=production", nil, true))); n != 1 {
		t.Fatalf("filter by tag: %d components", n)
	}
}

func TestGroupsDefaultToLoggedInUsers(t *testing.T) {
	r, _ := setup(t)
	g := data(t, send(t, r, http.MethodPost, "/components/groups", map[string]any{"name": "Internal", "collapsed": 2}, true))
	if num(g["visible"]) != 0 || num(g["collapsed"]) != 2 {
		t.Fatalf("group: %v", g)
	}
	if n := len(dataList(t, send(t, r, http.MethodGet, "/components/groups", nil, false))); n != 0 {
		t.Fatalf("anonymous readers see %d hidden groups", n)
	}
	if n := len(dataList(t, send(t, r, http.MethodGet, "/components/groups", nil, true))); n != 1 {
		t.Fatalf("authenticated readers see %d groups, want 1", n)
	}
}

func TestIncidentLifecycle(t *testing.T) {
	r, db := setup(t)
	comp := testutil.CreateComponent(t, db, "API", "operational")

	inc := data(t, send(t, r, http.MethodPost, "/incidents", map[string]any{
		"name": "API down", "message": "Looking into it", "status": 1,
		"component_id": comp.ID, "component_status": 4, "visible": 1, "notify": true,
	}, true))
	if num(inc["status"]) != 1 || inc["message"] != "Looking into it" || inc["human_status"] != "Investigating" {
		t.Fatalf("incident: %v", inc)
	}
	if got := testutil.ComponentStatus(t, db, comp.ID); got != "major_outage" {
		t.Fatalf("component status %q, want major_outage", got)
	}
	id := itoa(num(inc["id"]))

	upd := data(t, send(t, r, http.MethodPost, "/incidents/"+id+"/updates", map[string]any{
		"status": 4, "message": "Fixed", "component_id": comp.ID, "component_status": 1,
	}, true))
	if num(upd["status"]) != 4 || upd["message"] != "Fixed" {
		t.Fatalf("update: %v", upd)
	}
	inc = data(t, send(t, r, http.MethodGet, "/incidents/"+id, nil, false))
	if num(inc["status"]) != 4 || num(inc["latest_update_id"]) != num(upd["id"]) {
		t.Fatalf("incident after update: %v", inc)
	}
	if got := testutil.ComponentStatus(t, db, comp.ID); got != "operational" {
		t.Fatalf("component status %q, want operational", got)
	}
	if n := len(dataList(t, send(t, r, http.MethodGet, "/incidents/"+id+"/updates", nil, false))); n != 1 {
		t.Fatalf("updates: %d", n)
	}

	// A general incident, with no component, as Cachet allows.
	gen := data(t, send(t, r, http.MethodPost, "/incidents", map[string]any{"name": "Notice", "message": "Hi", "status": 3}, true))
	if num(gen["component_id"]) != 0 {
		t.Fatalf("general incident: %v", gen)
	}

	// Hidden incidents are for authenticated readers only.
	send(t, r, http.MethodPost, "/incidents", map[string]any{"name": "Internal", "message": "x", "status": 1, "visible": 0}, true)
	if n := len(dataList(t, send(t, r, http.MethodGet, "/incidents", nil, false))); n != 2 {
		t.Fatalf("anonymous incidents: %d, want 2", n)
	}

	if res := send(t, r, http.MethodDelete, "/incidents/"+id, nil, true); res.code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", res.code, res.raw)
	}
	if res := send(t, r, http.MethodGet, "/incidents/"+id, nil, true); res.code != http.StatusNotFound {
		t.Fatalf("deleted incident: %d", res.code)
	}
}

func TestSchedules(t *testing.T) {
	r, db := setup(t)
	comp := testutil.CreateComponent(t, db, "DB", "partial_outage")
	now := time.Now().UTC()

	s := data(t, send(t, r, http.MethodPost, "/schedules", map[string]any{
		"name": "Upgrade", "message": "Upgrading", "status": 1,
		"scheduled_at": now.Format("2006-01-02 15:04"), "completed_at": now.Add(time.Hour).Format("2006-01-02 15:04"),
		"components": map[string]any{itoa(int(comp.ID)): 2},
	}, true))
	if num(s["status"]) != 1 || len(s["components"].([]any)) != 1 {
		t.Fatalf("schedule: %v", s)
	}
	if got := testutil.ComponentStatus(t, db, comp.ID); got != "under_maintenance" {
		t.Fatalf("component status %q, want under_maintenance", got)
	}
	c := data(t, send(t, r, http.MethodGet, "/components/"+itoa(int(comp.ID)), nil, true))
	if num(c["status"]) != 0 || c["status_name"] != "Under Maintenance" {
		t.Fatalf("component under maintenance, seen by Cachet clients: %v", c)
	}

	send(t, r, http.MethodPut, "/schedules/"+itoa(num(s["id"])), map[string]any{"status": 2}, true)
	if got := testutil.ComponentStatus(t, db, comp.ID); got != "partial_outage" {
		t.Fatalf("after completion %q, want partial_outage", got)
	}

	res := send(t, r, http.MethodPost, "/schedules", map[string]any{"name": "x", "scheduled_at": now.Format(time.RFC3339)}, true)
	if res.code != http.StatusBadRequest {
		t.Fatalf("schedule without completed_at: %d", res.code)
	}
}

func TestSchedulesHideHiddenComponentsFromAnonymousReaders(t *testing.T) {
	r, db := setup(t)
	shown := testutil.CreateComponent(t, db, "Website", "operational")
	hidden := testutil.CreateComponent(t, db, "Secret", "operational")
	db.Model(&hidden).UpdateColumn("enabled", false)
	now := time.Now().UTC()
	s := data(t, send(t, r, http.MethodPost, "/schedules", map[string]any{
		"name": "Upgrade", "message": "Upgrading", "status": 0,
		"scheduled_at": now.Add(time.Hour).Format("2006-01-02 15:04"), "completed_at": now.Add(2 * time.Hour).Format("2006-01-02 15:04"),
		"components": map[string]any{itoa(int(shown.ID)): 0, itoa(int(hidden.ID)): 0},
	}, true))
	path := "/schedules/" + itoa(num(s["id"]))

	if got := len(data(t, send(t, r, http.MethodGet, path, nil, true))["components"].([]any)); got != 2 {
		t.Fatalf("components seen with a key: %d, want 2", got)
	}
	anon := data(t, send(t, r, http.MethodGet, path, nil, false))["components"].([]any)
	if len(anon) != 1 || num(anon[0].(map[string]any)["id"]) != int(shown.ID) {
		t.Fatalf("components seen anonymously: %v, want only %d", anon, shown.ID)
	}
	list := dataList(t, send(t, r, http.MethodGet, "/schedules", nil, false))
	if got := len(list[0].(map[string]any)["components"].([]any)); got != 1 {
		t.Fatalf("components in the anonymous list: %d, want 1", got)
	}
}

func TestAuthenticatedGroupReadsIncludeDisabledComponents(t *testing.T) {
	r, db := setup(t)
	g := data(t, send(t, r, http.MethodPost, "/components/groups", map[string]any{"name": "Web", "visible": 1}, true))
	gid := uint(num(g["id"]))
	for _, enabled := range []bool{true, false} {
		comp := testutil.CreateComponent(t, db, fmt.Sprint("enabled ", enabled), "operational")
		db.Model(&comp).UpdateColumns(map[string]any{"group_id": gid, "enabled": enabled})
	}
	path := "/components/groups/" + itoa(int(gid))
	if n := len(data(t, send(t, r, http.MethodGet, path, nil, true))["enabled_components"].([]any)); n != 2 {
		t.Fatalf("components seen with a key: %d, want 2", n)
	}
	if n := len(data(t, send(t, r, http.MethodGet, path, nil, false))["enabled_components"].([]any)); n != 1 {
		t.Fatalf("components seen anonymously: %d, want 1", n)
	}
}

func TestScheduleUpdateKeepsTheMainComponent(t *testing.T) {
	r, db := setup(t)
	a := testutil.CreateComponent(t, db, "A", "operational")
	b := testutil.CreateComponent(t, db, "B", "operational")
	now := time.Now().UTC()
	s := data(t, send(t, r, http.MethodPost, "/schedules", map[string]any{
		"name": "Upgrade", "status": 0,
		"scheduled_at": now.Add(time.Hour).Format("2006-01-02 15:04"), "completed_at": now.Add(2 * time.Hour).Format("2006-01-02 15:04"),
		"components": map[string]any{itoa(int(b.ID)): 0},
	}, true))
	id := num(s["id"])
	send(t, r, http.MethodPut, "/schedules/"+itoa(id), map[string]any{
		"components": map[string]any{itoa(int(a.ID)): 0, itoa(int(b.ID)): 0},
	}, true)
	var m models.Maintenance
	db.First(&m, id)
	if models.IDValue(m.ComponentID) != b.ID {
		t.Fatalf("main component %d after adding A, want B (%d)", models.IDValue(m.ComponentID), b.ID)
	}
}

func TestBadIDsAndLargeBodies(t *testing.T) {
	r, _ := setup(t)
	for _, path := range []string{"/components/99999999999", "/components/groups/99999999999", "/incidents/99999999999", "/schedules/99999999999", "/incidents/x"} {
		if res := send(t, r, http.MethodGet, path, nil, false); res.code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", path, res.code)
		}
	}
	big := map[string]any{"name": strings.Repeat("x", 2<<20)}
	if res := send(t, r, http.MethodGet, "/components", big, false); res.code != http.StatusBadRequest {
		t.Fatalf("2 MiB body: %d, want 400", res.code)
	}
}

func TestErrors(t *testing.T) {
	r, _ := setup(t)
	res := send(t, r, http.MethodPost, "/components", map[string]any{"name": "x", "status": 1}, false)
	if res.code != http.StatusUnauthorized {
		t.Fatalf("write without a key: %d", res.code)
	}
	res = send(t, r, http.MethodPost, "/components", map[string]any{"name": "x", "status": 9}, true)
	errs, ok := res.body["errors"].([]any)
	if res.code != http.StatusBadRequest || !ok || len(errs) != 1 {
		t.Fatalf("bad status: %d %s", res.code, res.raw)
	}
	e := errs[0].(map[string]any)
	if e["status"] != "400" || e["title"] != "Bad Request" || !strings.Contains(e["detail"].(string), "status 9") {
		t.Fatalf("error format: %v", e)
	}
	if res := send(t, r, http.MethodGet, "/components/999", nil, false); res.code != http.StatusNotFound {
		t.Fatalf("missing component: %d", res.code)
	}
	if res := send(t, r, http.MethodGet, "/components", nil, false); res.code != http.StatusOK {
		t.Fatalf("anonymous read: %d", res.code)
	}
	req := httptest.NewRequest(http.MethodGet, Prefix+"/components", nil)
	req.Header.Set("X-Cachet-Token", "wrong")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("read with a wrong key: %d", w.Code)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
