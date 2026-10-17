// Package customization holds the status page's custom CSS, header and
// footer, set by admins and imported from Cachet.
package customization

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"api/internal/models"

	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"
)

// SettingKey is the settings row that stores the customization, as JSON.
const SettingKey = "customization"

// Size limits, generous for hand-written CSS and HTML.
const (
	MaxCSS  = 100 * 1024
	MaxHTML = 50 * 1024
)

// Customization is what the public page adds to its theme.
type Customization struct {
	// CSS is added after the theme's styles, so its rules win.
	CSS string `json:"css"`
	// HeaderHTML is shown above the page, FooterHTML below it. Both are
	// sanitized: no scripts, event handlers or forms.
	HeaderHTML string `json:"header_html"`
	FooterHTML string `json:"footer_html"`
}

var policy = sync.OnceValue(func() *bluemonday.Policy {
	// UGCPolicy allows formatting, links, images, lists and tables, and
	// drops scripts, styles, iframes, forms and event handlers.
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("class").Globally()
	// These are the site owner's own links, not user content.
	p.RequireNoFollowOnLinks(false)
	p.AllowDataURIImages()
	// Inline styles for layout and colours. bluemonday checks each value,
	// and url() is not allowed in any of these.
	p.AllowStyles("color", "background-color", "text-align", "vertical-align",
		"font-size", "font-weight", "font-style", "text-decoration", "line-height",
		"margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
		"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
		"border", "border-radius", "width", "height", "max-width", "display").Globally()
	return p
})

// SanitizeHTML removes from html what could run code or change the page
// outside the header or footer.
func SanitizeHTML(html string) string {
	return policy().Sanitize(html)
}

// Validate checks the sizes.
func (c Customization) Validate() error {
	if len(c.CSS) > MaxCSS {
		return fmt.Errorf("custom CSS is too large (max %d KB)", MaxCSS/1024)
	}
	if len(c.HeaderHTML) > MaxHTML || len(c.FooterHTML) > MaxHTML {
		return fmt.Errorf("header and footer HTML are limited to %d KB each", MaxHTML/1024)
	}
	return nil
}

// Sanitized returns c with its HTML sanitized.
func (c Customization) Sanitized() Customization {
	c.HeaderHTML = SanitizeHTML(c.HeaderHTML)
	c.FooterHTML = SanitizeHTML(c.FooterHTML)
	return c
}

// Load reads the customization; an unset one is empty.
func Load(db *gorm.DB) (Customization, error) {
	var c Customization
	var setting models.Settings
	err := db.Where(&models.Settings{Key: SettingKey}).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal([]byte(setting.Value), &c); err != nil {
		return c, err
	}
	return c, nil
}

// Save stores c, sanitized, and returns what was stored.
func Save(db *gorm.DB, c Customization) (Customization, error) {
	c = c.Sanitized()
	value, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	var setting models.Settings
	err = db.Where(&models.Settings{Key: SettingKey}).First(&setting).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return c, db.Create(&models.Settings{Key: SettingKey, Value: string(value)}).Error
	case err != nil:
		return c, err
	}
	return c, db.Model(&setting).Update("value", string(value)).Error
}
