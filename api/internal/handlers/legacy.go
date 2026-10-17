package handlers

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Cachet 2.4 public URLs, so that links and bookmarks from before the move
// keep working. The public site forwards these paths to the API.

func envURL(name, fallback string) string {
	if v := strings.TrimRight(os.Getenv(name), "/"); v != "" {
		return v
	}
	return fallback
}

func publicPageURL() string { return envURL("PUBLIC_URL", "http://localhost:3000") }

// RegisterCachetRedirects mounts Cachet's public URLs at the root.
func RegisterCachetRedirects(r *gin.Engine, db *gorm.DB) {
	r.GET("/incidents/:id", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s/#incident-%s", publicPageURL(), url.PathEscape(c.Param("id"))))
	})
	r.GET("/subscribe", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, publicPageURL()+"/#subscribe")
	})
	// Cachet's links carry the subscriber's verify code, which the importer
	// keeps as the confirmation token.
	r.GET("/subscribe/manage/:code", func(c *gin.Context) {
		s, ok := subscriberByCachetCode(c, db)
		if c.IsAborted() {
			return
		}
		if !ok {
			c.Redirect(http.StatusFound, publicPageURL()+"/")
			return
		}
		c.Redirect(http.StatusFound, publicPageURL()+"/?manage="+url.QueryEscape(s.UnsubscribeToken))
	})
	r.GET("/unsubscribe/:code", func(c *gin.Context) {
		s, ok := subscriberByCachetCode(c, db)
		if c.IsAborted() {
			return
		}
		if !ok {
			c.Redirect(http.StatusFound, publicPageURL()+"/")
			return
		}
		c.Redirect(http.StatusFound, envURL("API_URL", "http://localhost:8080")+"/api/v1/unsubscribe?token="+url.QueryEscape(s.UnsubscribeToken))
	})
	dashboard := func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, envURL("ADMIN_URL", "http://localhost:3001")+"/")
	}
	r.GET("/dashboard", dashboard)
	r.GET("/dashboard/*path", dashboard)
	r.GET("/rss", feed(db, renderRSS))
	r.GET("/atom", feed(db, renderAtom))
}

// subscriberByCachetCode finds the subscriber of the link's code. When it
// returns false, the caller sends an unknown code to the status page; a
// database failure has already been answered with a 500.
func subscriberByCachetCode(c *gin.Context, db *gorm.DB) (models.Subscriber, bool) {
	var s models.Subscriber
	code := c.Param("code")
	if code == "" {
		return s, false
	}
	err := db.Where("confirmation_token = ?", code).First(&s).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.String(http.StatusInternalServerError, "Failed to look up the subscription")
		c.Abort()
	}
	return s, err == nil
}

// feedItem is an incident as the feeds show it, in the requested locale.
type feedItem struct {
	ID      uint
	Title   string
	Content string
	Link    string
	Updated time.Time
}

const feedSize = 50

// feed renders the latest visible incidents, newest first, with render.
func feed(db *gorm.DB, render func(c *gin.Context, title string, items []feedItem)) gin.HandlerFunc {
	return func(c *gin.Context) {
		locale := getLocale(c)
		var incidents []models.Incident
		// English too: getTranslationValue falls back to it.
		locales := []string{locale, "en"}
		err := db.Scopes(PublicIncidents).Preload("Translations", "locale IN ?", locales).
			Preload("Updates", updatesNewestFirst).Preload("Updates.Translations", "locale IN ?", locales).
			Order("COALESCE(occurred_at, created_at) DESC").Order("id DESC").Limit(feedSize).
			Find(&incidents).Error
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to fetch incidents")
			return
		}
		items := make([]feedItem, 0, len(incidents))
		for _, i := range incidents {
			content := getTranslationValue(i.Translations, "message", locale, "")
			updated := i.UpdatedAt
			if len(i.Updates) > 0 {
				if msg := getTranslationValue(i.Updates[0].Translations, "message", locale, ""); msg != "" {
					content = msg
				}
				if i.Updates[0].CreatedAt.After(updated) {
					updated = i.Updates[0].CreatedAt
				}
			}
			items = append(items, feedItem{
				ID:      i.ID,
				Title:   fmt.Sprintf("%s (%s)", getTranslationValue(i.Translations, "name", locale, i.Name), i.Status),
				Content: content,
				Link:    fmt.Sprintf("%s/#incident-%d", publicPageURL(), i.ID),
				Updated: updated,
			})
		}
		render(c, siteTitle(db, locale), items)
	}
}

// siteTitle is the page title from the branding, or "Status".
func siteTitle(db *gorm.DB, locale string) string {
	var setting models.Settings
	if err := db.Where(&models.Settings{Key: "branding"}).First(&setting).Error; err == nil {
		var b BrandingConfig
		if json.Unmarshal([]byte(setting.Value), &b) == nil {
			if locale == "fr" && b.TitleFR != "" {
				return b.TitleFR
			}
			if b.TitleEN != "" {
				return b.TitleEN
			}
		}
	}
	return "Status"
}

func renderRSS(c *gin.Context, title string, items []feedItem) {
	type rssItem struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		GUID        string `xml:"guid"`
		PubDate     string `xml:"pubDate"`
		Description string `xml:"description"`
	}
	type channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Items       []rssItem `xml:"item"`
	}
	type rss struct {
		XMLName xml.Name `xml:"rss"`
		Version string   `xml:"version,attr"`
		Channel channel  `xml:"channel"`
	}
	out := rss{Version: "2.0", Channel: channel{Title: title, Link: publicPageURL() + "/", Description: title}}
	for _, it := range items {
		out.Channel.Items = append(out.Channel.Items, rssItem{
			Title: it.Title, Link: it.Link, GUID: it.Link, PubDate: it.Updated.UTC().Format(time.RFC1123Z), Description: it.Content,
		})
	}
	writeXML(c, "application/rss+xml; charset=utf-8", out)
}

func renderAtom(c *gin.Context, title string, items []feedItem) {
	type link struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr,omitempty"`
	}
	type entry struct {
		Title   string `xml:"title"`
		ID      string `xml:"id"`
		Link    link   `xml:"link"`
		Updated string `xml:"updated"`
		Content string `xml:"content"`
	}
	type atom struct {
		XMLName xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
		Title   string   `xml:"title"`
		ID      string   `xml:"id"`
		Link    []link   `xml:"link"`
		Updated string   `xml:"updated"`
		Entries []entry  `xml:"entry"`
	}
	// The newest entry: items are ordered by when incidents occurred, and
	// an older incident can have the latest update.
	updated := time.Now()
	for i, it := range items {
		if i == 0 || it.Updated.After(updated) {
			updated = it.Updated
		}
	}
	out := atom{Title: title, ID: publicPageURL() + "/", Updated: updated.UTC().Format(time.RFC3339),
		Link: []link{{Href: publicPageURL() + "/"}, {Href: publicPageURL() + "/atom", Rel: "self"}}}
	for _, it := range items {
		out.Entries = append(out.Entries, entry{
			Title: it.Title, ID: it.Link, Link: link{Href: it.Link}, Updated: it.Updated.UTC().Format(time.RFC3339), Content: it.Content,
		})
	}
	writeXML(c, "application/atom+xml; charset=utf-8", out)
}

func writeXML(c *gin.Context, contentType string, v any) {
	body, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to render feed")
		return
	}
	c.Data(http.StatusOK, contentType, append([]byte(xml.Header), body...))
}
