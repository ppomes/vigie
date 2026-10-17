package models

import (
	"regexp"
	"strings"
	"time"
)

// Tag labels components, as in Cachet. Integrations filter components by tag.
type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Slug      string    `gorm:"size:255;not null;uniqueIndex" json:"slug"`
	CreatedAt time.Time `json:"-"`
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// TagSlug returns the slug identifying a tag name: lowercase letters and
// digits separated by dashes. Two names with the same slug are one tag.
func TagSlug(name string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
}
