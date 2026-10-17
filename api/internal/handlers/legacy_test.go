package handlers

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api/internal/models"
	"api/internal/testutil"

	"github.com/gin-gonic/gin"
)

func legacyRouter(t *testing.T) (*gin.Engine, func(path string) *httptest.ResponseRecorder) {
	t.Helper()
	t.Setenv("PUBLIC_URL", "https://status.example.com")
	t.Setenv("API_URL", "https://api.example.com")
	t.Setenv("ADMIN_URL", "https://admin.example.com")
	gin.SetMode(gin.TestMode)
	db := testutil.NewDB(t)
	r := gin.New()
	RegisterCachetRedirects(r, db)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.Create(&models.Subscriber{Email: "fan@example.com", Confirmed: true, ConfirmationToken: "cachet-code", UnsubscribeToken: "unsub-tok"}).Error)
	c := testutil.CreateComponent(t, db, "API", "operational")
	visible := models.Incident{Name: "API slow", Status: "monitoring", ComponentID: models.OptionalID(c.ID), Visible: true}
	must(db.Create(&visible).Error)
	must(db.Create(&models.Translation{EntityID: visible.ID, EntityType: "incident", Locale: "en", Field: "message", Value: "Looking <into> it"}).Error)
	internal := models.Incident{Name: "Internal", Status: "investigating"}
	must(db.Create(&internal).Error)
	must(db.Model(&internal).Update("visible", false).Error)
	must(db.Create(&models.Settings{Key: "branding", Value: `{"title_en":"Acme Status","title_fr":"Statut Acme"}`}).Error)
	return r, func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
}

func TestCachetURLRedirects(t *testing.T) {
	_, get := legacyRouter(t)
	for path, want := range map[string]string{
		"/incidents/42":                 "https://status.example.com/#incident-42",
		"/subscribe":                    "https://status.example.com/#subscribe",
		"/subscribe/manage/cachet-code": "https://status.example.com/?manage=unsub-tok",
		"/subscribe/manage/unknown":     "https://status.example.com/",
		"/unsubscribe/cachet-code":      "https://api.example.com/api/v1/unsubscribe?token=unsub-tok",
		"/dashboard":                    "https://admin.example.com/",
		"/dashboard/incidents/add":      "https://admin.example.com/",
	} {
		w := get(path)
		if w.Code != http.StatusMovedPermanently && w.Code != http.StatusFound {
			t.Errorf("%s: status %d", path, w.Code)
			continue
		}
		if got := w.Header().Get("Location"); got != want {
			t.Errorf("%s: redirects to %q, want %q", path, got, want)
		}
	}
}

func TestFeeds(t *testing.T) {
	_, get := legacyRouter(t)

	w := get("/rss")
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/rss+xml") {
		t.Fatalf("rss: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	var rss struct {
		Channel struct {
			Title string `xml:"title"`
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				Description string `xml:"description"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &rss); err != nil {
		t.Fatalf("rss is not valid XML: %v\n%s", err, w.Body.String())
	}
	if rss.Channel.Title != "Acme Status" || len(rss.Channel.Items) != 1 {
		t.Fatalf("rss channel: %+v", rss.Channel)
	}
	item := rss.Channel.Items[0]
	if item.Title != "API slow (monitoring)" || item.Description != "Looking <into> it" || !strings.HasSuffix(item.Link, "/#incident-1") {
		t.Fatalf("rss item: %+v", item)
	}

	w = get("/atom?locale=fr")
	var atom struct {
		Title   string `xml:"title"`
		Entries []struct {
			Title   string `xml:"title"`
			Content string `xml:"content"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &atom); err != nil {
		t.Fatalf("atom is not valid XML: %v", err)
	}
	if atom.Title != "Statut Acme" || len(atom.Entries) != 1 {
		t.Fatalf("atom feed: %+v", atom)
	}
	// Only an English message exists: the French feed falls back to it.
	if atom.Entries[0].Content != "Looking <into> it" {
		t.Fatalf("atom content %q, want the English message", atom.Entries[0].Content)
	}
}

func TestSubscriberLinksReportDatabaseFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewDB(t)
	r := gin.New()
	RegisterCachetRedirects(r, db)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.Close()
	for _, path := range []string{"/subscribe/manage/code", "/unsubscribe/code"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusInternalServerError {
			t.Errorf("%s with the database down: %d, want 500", path, w.Code)
		}
	}
}
