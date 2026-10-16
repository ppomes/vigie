package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api/internal/middleware"
	"api/internal/models"
	"api/internal/services"
	"api/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const testJWTSecret = "test-secret"

// newAuthRouter wires the API key and component routes behind the real
// authentication middleware, as in cmd/api/main.go.
func newAuthRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	t.Setenv("JWT_SECRET_KEY", testJWTSecret)
	gin.SetMode(gin.TestMode)
	audit := services.NewAuditService(db)

	r := gin.New()
	admin := r.Group("/admin", middleware.AuthRequiredWithDB(db))
	admin.GET("/components", ListComponents(db))
	adminOnly := admin.Group("", middleware.RequireAdmin())
	adminOnly.PUT("/components/:id", UpdateComponent(db))
	keys := adminOnly.Group("/api-keys", middleware.RequireSession())
	keys.GET("", ListAPIKeys(db))
	keys.POST("", CreateAPIKey(db, audit))
	keys.DELETE("/:id", RevokeAPIKey(db, audit))
	return r
}

func createUser(t *testing.T, db *gorm.DB, name, role string) models.User {
	t.Helper()
	u := models.User{Username: name, Email: name + "@example.com", Password: "x", Role: role, Active: true}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func sessionToken(t *testing.T, userID uint) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	s, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type authRequest struct {
	method, path string
	body         any
	header       string // header name: "Authorization" or middleware.CachetTokenHeader
	value        string
}

func do(t *testing.T, r *gin.Engine, ar authRequest) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if ar.body != nil {
		json.NewEncoder(&buf).Encode(ar.body)
	}
	req := httptest.NewRequest(ar.method, ar.path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if ar.header != "" {
		req.Header.Set(ar.header, ar.value)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func bearer(token string) (string, string) { return "Authorization", "Bearer " + token }

// createKey creates a key through the API with an admin session and returns
// the raw key and its id.
func createKey(t *testing.T, r *gin.Engine, adminToken string, body map[string]any) (string, uint) {
	t.Helper()
	h, v := bearer(adminToken)
	w := do(t, r, authRequest{http.MethodPost, "/admin/api-keys", body, h, v})
	if w.Code != http.StatusCreated {
		t.Fatalf("create key: status %d, body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Key    string `json:"key"`
		APIKey struct {
			ID uint `json:"id"`
		} `json:"api_key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.Key, models.APIKeyPrefix) {
		t.Fatalf("key %q does not start with %q", resp.Key, models.APIKeyPrefix)
	}
	return resp.Key, resp.APIKey.ID
}

func TestAPIKeyAuthenticatesWithBothHeaders(t *testing.T) {
	db := testutil.NewDB(t)
	r := newAuthRouter(t, db)
	admin := createUser(t, db, "admin", models.RoleAdmin)
	c := testutil.CreateComponent(t, db, "API", "operational")
	raw, _ := createKey(t, r, sessionToken(t, admin.ID), map[string]any{"name": "zabbix"})

	path := fmt.Sprintf("/admin/components/%d", c.ID)
	for _, tc := range []struct{ header, value, status string }{
		{"Authorization", "Bearer " + raw, "partial_outage"},
		{middleware.CachetTokenHeader, raw, "major_outage"},
	} {
		w := do(t, r, authRequest{http.MethodPut, path, map[string]any{"status": tc.status}, tc.header, tc.value})
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d, body %s", tc.header, w.Code, w.Body.String())
		}
		if got := testutil.ComponentStatus(t, db, c.ID); got != tc.status {
			t.Fatalf("%s: component status %q, want %q", tc.header, got, tc.status)
		}
	}

	var key models.APIKey
	db.First(&key)
	if key.LastUsedAt == nil {
		t.Error("last_used_at was not recorded")
	}
}

func TestAPIKeyInheritsUserRole(t *testing.T) {
	db := testutil.NewDB(t)
	r := newAuthRouter(t, db)
	admin := createUser(t, db, "admin", models.RoleAdmin)
	viewer := createUser(t, db, "viewer", models.RoleViewer)
	c := testutil.CreateComponent(t, db, "API", "operational")
	raw, _ := createKey(t, r, sessionToken(t, admin.ID), map[string]any{"name": "read-only", "user_id": viewer.ID})

	h, v := bearer(raw)
	if w := do(t, r, authRequest{http.MethodGet, "/admin/components", nil, h, v}); w.Code != http.StatusOK {
		t.Fatalf("viewer key read: status %d", w.Code)
	}
	w := do(t, r, authRequest{http.MethodPut, fmt.Sprintf("/admin/components/%d", c.ID), map[string]any{"status": "major_outage"}, h, v})
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer key write: status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRejectedAPIKeys(t *testing.T) {
	db := testutil.NewDB(t)
	r := newAuthRouter(t, db)
	admin := createUser(t, db, "admin", models.RoleAdmin)
	inactive := createUser(t, db, "inactive", models.RoleAdmin)
	gone := createUser(t, db, "gone", models.RoleAdmin)
	adminToken := sessionToken(t, admin.ID)

	revoked, revokedID := createKey(t, r, adminToken, map[string]any{"name": "old"})
	h, v := bearer(adminToken)
	if w := do(t, r, authRequest{http.MethodDelete, fmt.Sprintf("/admin/api-keys/%d", revokedID), nil, h, v}); w.Code != http.StatusOK {
		t.Fatalf("revoke: status %d", w.Code)
	}

	inactiveKey, _ := createKey(t, r, adminToken, map[string]any{"name": "inactive", "user_id": inactive.ID})
	db.Model(&inactive).Update("active", false)

	goneKey, _ := createKey(t, r, adminToken, map[string]any{"name": "gone", "user_id": gone.ID})
	db.Delete(&gone)

	for _, tc := range []struct {
		name, key string
		want      int
	}{
		{"unknown key", "vigie_nope", http.StatusUnauthorized},
		{"revoked key", revoked, http.StatusUnauthorized},
		{"deleted user", goneKey, http.StatusUnauthorized},
		{"deactivated user", inactiveKey, http.StatusForbidden},
	} {
		h, v := bearer(tc.key)
		if w := do(t, r, authRequest{http.MethodGet, "/admin/components", nil, h, v}); w.Code != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, w.Code, tc.want)
		}
	}
}

func TestAPIKeyCannotManageKeys(t *testing.T) {
	db := testutil.NewDB(t)
	r := newAuthRouter(t, db)
	admin := createUser(t, db, "admin", models.RoleAdmin)
	raw, id := createKey(t, r, sessionToken(t, admin.ID), map[string]any{"name": "ci"})

	h, v := bearer(raw)
	for _, ar := range []authRequest{
		{http.MethodGet, "/admin/api-keys", nil, h, v},
		{http.MethodPost, "/admin/api-keys", map[string]any{"name": "more"}, h, v},
		{http.MethodDelete, fmt.Sprintf("/admin/api-keys/%d", id), nil, h, v},
	} {
		if w := do(t, r, ar); w.Code != http.StatusForbidden {
			t.Errorf("%s %s with an API key: status %d, want %d", ar.method, ar.path, w.Code, http.StatusForbidden)
		}
	}
}

func TestListAPIKeysHidesSecrets(t *testing.T) {
	db := testutil.NewDB(t)
	r := newAuthRouter(t, db)
	admin := createUser(t, db, "admin", models.RoleAdmin)
	token := sessionToken(t, admin.ID)
	raw, _ := createKey(t, r, token, map[string]any{"name": "zabbix"})

	h, v := bearer(token)
	w := do(t, r, authRequest{http.MethodGet, "/admin/api-keys", nil, h, v})
	if w.Code != http.StatusOK {
		t.Fatalf("list: status %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, raw) || strings.Contains(body, models.HashAPIKey(raw)) {
		t.Fatalf("list exposes the key or its hash: %s", body)
	}
	if !strings.Contains(body, `"name":"zabbix"`) || !strings.Contains(body, raw[:12]) {
		t.Fatalf("list should show the name and prefix: %s", body)
	}
	if strings.Contains(body, `"password"`) {
		t.Fatalf("list exposes user passwords: %s", body)
	}
}

func TestImportedCachetKeyAuthenticates(t *testing.T) {
	db := testutil.NewDB(t)
	r := newAuthRouter(t, db)
	admin := createUser(t, db, "admin", models.RoleAdmin)
	// Cachet 2.4 keys are 20-character alphanumeric strings without prefix.
	key := models.NewAPIKey("cachet import", "Ab3dEf6hIj9kLm2nOp5q", admin.ID)
	if err := db.Create(&key).Error; err != nil {
		t.Fatal(err)
	}

	w := do(t, r, authRequest{http.MethodGet, "/admin/components", nil, middleware.CachetTokenHeader, "Ab3dEf6hIj9kLm2nOp5q"})
	if w.Code != http.StatusOK {
		t.Fatalf("imported key: status %d, body %s", w.Code, w.Body.String())
	}
}

func TestAPIKeyAuditFailure(t *testing.T) {
	db := testutil.NewDB(t)
	r := newAuthRouter(t, db)
	admin := createUser(t, db, "admin", models.RoleAdmin)
	token := sessionToken(t, admin.ID)
	_, id := createKey(t, r, token, map[string]any{"name": "existing"})

	// Make every audit write fail.
	if err := db.Migrator().DropTable(&models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	h, v := bearer(token)

	// Creation is refused, and leaves no key behind.
	w := do(t, r, authRequest{http.MethodPost, "/admin/api-keys", map[string]any{"name": "unaudited"}, h, v})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("create without audit: status %d, want %d", w.Code, http.StatusInternalServerError)
	}
	var count int64
	db.Model(&models.APIKey{}).Where("name = ?", "unaudited").Count(&count)
	if count != 0 {
		t.Fatal("a key was created without its audit entry")
	}

	// Revocation still goes through.
	w = do(t, r, authRequest{http.MethodDelete, fmt.Sprintf("/admin/api-keys/%d", id), nil, h, v})
	if w.Code != http.StatusOK {
		t.Fatalf("revoke without audit: status %d, want %d", w.Code, http.StatusOK)
	}
	var key models.APIKey
	db.First(&key, id)
	if key.RevokedAt == nil {
		t.Fatal("key not revoked when the audit write failed")
	}
}
