package handlers

import (
	"net/url"
	"strings"

	"api/internal/models"

	"gorm.io/gorm"
)

// componentExtras are the optional component fields shared by create and
// update requests. A nil field is left unchanged.
type componentExtras struct {
	// Link is a URL; an empty string removes it.
	Link *string `json:"link" binding:"omitempty,max=1024"`
	// Enabled false hides the component from the public page.
	Enabled *bool `json:"enabled"`
	// Meta replaces the component's key/value data; {} clears it.
	Meta *map[string]string `json:"meta"`
	// Tags replaces the component's tags, by name; [] clears them.
	Tags *[]string `json:"tags"`
}

// validate checks the extras and returns an error message, or "".
func (e componentExtras) validate() string {
	if e.Link != nil {
		if l := strings.TrimSpace(*e.Link); l != "" {
			u, err := url.Parse(l)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return "Link must be an http or https URL"
			}
		}
	}
	if e.Tags != nil {
		for _, t := range *e.Tags {
			if models.TagSlug(t) == "" {
				return "Tag names need at least one letter or digit"
			}
		}
	}
	return ""
}

// apply writes the extras on a saved component, in the caller's transaction.
func (e componentExtras) apply(tx *gorm.DB, component *models.Component) error {
	updates := map[string]any{}
	if e.Link != nil {
		updates["link"] = strings.TrimSpace(*e.Link)
	}
	if e.Enabled != nil {
		// Through a map, so false is written: GORM skips zero values in
		// structs and would keep the column default.
		updates["enabled"] = *e.Enabled
	}
	if e.Meta != nil {
		updates["meta"] = models.JSONMap(*e.Meta)
	}
	if len(updates) > 0 {
		if err := tx.Model(component).Updates(updates).Error; err != nil {
			return err
		}
	}
	if e.Tags != nil {
		return syncTags(tx, component, *e.Tags)
	}
	return nil
}

// syncTags sets a component's tags by name, creating missing tags. Names
// with the same slug are one tag.
func syncTags(tx *gorm.DB, component *models.Component, names []string) error {
	tags := []models.Tag{}
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		slug := models.TagSlug(name)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		tag := models.Tag{Name: name, Slug: slug}
		if err := tx.Where("slug = ?", slug).FirstOrCreate(&tag).Error; err != nil {
			return err
		}
		tags = append(tags, tag)
	}
	return tx.Model(component).Association("Tags").Replace(tags)
}
