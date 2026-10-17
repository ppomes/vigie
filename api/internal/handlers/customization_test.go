package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"api/internal/customization"
	"api/internal/models"
	"api/internal/testutil"

	"github.com/gin-gonic/gin"
)

func TestCustomization(t *testing.T) {
	db := testutil.NewDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/settings/customization", GetCustomization(db))
	r.PUT("/admin/settings/customization", UpdateCustomization(db))

	get := func() customization.Customization {
		t.Helper()
		w := do(t, r, authRequest{method: http.MethodGet, path: "/settings/customization"})
		if w.Code != http.StatusOK {
			t.Fatalf("get: %d %s", w.Code, w.Body)
		}
		var c customization.Customization
		if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
			t.Fatalf("get: %v in %s", err, w.Body)
		}
		return c
	}
	if c := get(); c != (customization.Customization{}) {
		t.Fatalf("unset: %+v", c)
	}

	w := do(t, r, authRequest{method: http.MethodPut, path: "/admin/settings/customization", body: map[string]string{
		"css":         ".card { border-radius: 0 }",
		"header_html": `<a href="https://acme.test">Acme</a><script>alert(1)</script>`,
		"footer_html": `<p onmouseover="steal()">© Acme</p>`,
	}})
	if w.Code != http.StatusOK {
		t.Fatalf("put: %d %s", w.Code, w.Body)
	}
	var saved customization.Customization
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatalf("put: %v in %s", err, w.Body)
	}
	want := customization.Customization{
		CSS:        ".card { border-radius: 0 }",
		HeaderHTML: `<a href="https://acme.test">Acme</a>`,
		FooterHTML: `<p>© Acme</p>`,
	}
	if saved != want || get() != want {
		t.Fatalf("saved %+v, public %+v", saved, get())
	}

	w = do(t, r, authRequest{method: http.MethodPut, path: "/admin/settings/customization",
		body: map[string]string{"css": strings.Repeat("a", customization.MaxCSS+1)}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("too large: %d", w.Code)
	}

	// The generic settings endpoint can write the row without sanitizing:
	// the public endpoint sanitizes again.
	err := db.Model(&models.Settings{}).Where("`key` = ?", customization.SettingKey).
		Update("value", `{"header_html":"<img src=x onerror=alert(1)>"}`).Error
	if err != nil {
		t.Fatal(err)
	}
	if c := get(); c.HeaderHTML != `<img src="x">` {
		t.Fatalf("raw row: %q", c.HeaderHTML)
	}
}
