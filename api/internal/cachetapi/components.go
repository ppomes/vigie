package cachetapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"api/internal/handlers"
	"api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var componentSorts = map[string]string{
	"id": "id", "name": "name", "status": "status", "order": "order",
	"group_id": "group_id", "enabled": "enabled", "created_at": "created_at", "updated_at": "updated_at",
}

// componentQuery is the base query for reading components. Anonymous
// readers see what the public page shows.
func componentQuery(c *gin.Context, db *gorm.DB) *gorm.DB {
	q := db.Model(&models.Component{}).Preload("Translations").Preload("Tags")
	if !authenticated(c) {
		q = q.Scopes(handlers.PublicComponents)
	}
	return q
}

func listComponents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := componentQuery(c, db)
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if v, ok, err := in.integer("id"); err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		} else if ok {
			q = q.Where("id = ?", v)
		}
		if v, ok := in.str("name"); ok {
			q = q.Where("name = ?", v)
		}
		if v, ok, err := in.integer("status"); err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		} else if ok {
			q = q.Where("status = ?", componentStatusToVigie[v])
		}
		if v, ok, err := in.integer("group_id"); err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		} else if ok {
			if v == 0 {
				q = q.Where("group_id IS NULL")
			} else {
				q = q.Where("group_id = ?", v)
			}
		}
		if v, ok, err := in.boolean("enabled"); err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		} else if ok {
			q = q.Where("enabled = ?", v)
		}
		if tags, ok := in.list("tags"); ok && len(tags) > 0 {
			slugs := make([]string, len(tags))
			for i, t := range tags {
				slugs[i] = models.TagSlug(t)
			}
			q = q.Where("id IN (?)", db.Table("component_tags").Select("component_tags.component_id").
				Joins("JOIN tags ON tags.id = component_tags.tag_id").Where("tags.slug IN ?", slugs))
		}
		q = sortQuery(c, q, componentSorts, by("order", false), by("id", false))

		var components []models.Component
		meta, err := paginate(c, q, &components)
		if err != nil {
			fail(c, http.StatusInternalServerError, "Failed to fetch components")
			return
		}
		data := make([]gin.H, len(components))
		for i, comp := range components {
			data[i] = presentComponent(comp)
		}
		list(c, meta, data)
	}
}

func findComponent(c *gin.Context, db *gorm.DB) (models.Component, bool) {
	var comp models.Component
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		fail(c, http.StatusNotFound, "Component not found")
		return comp, false
	}
	if err := componentQuery(c, db).First(&comp, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusNotFound, "Component not found")
		} else {
			fail(c, http.StatusInternalServerError, "Failed to fetch component")
		}
		return comp, false
	}
	return comp, true
}

func showComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if comp, ok := findComponent(c, db); ok {
			item(c, presentComponent(comp))
		}
	}
}

// componentBody translates Cachet component fields into a Vigie component
// request. existing is nil on creation.
func componentBody(in input, existing *models.Component) (map[string]any, error) {
	body := map[string]any{}
	if v, ok := in.str("name"); ok {
		body["name"] = v
	}
	if v, ok, err := in.integer("status"); err != nil {
		return nil, err
	} else if ok {
		s, known := componentStatusToVigie[v]
		if !known {
			return nil, fmt.Errorf("status %d is not supported: use 1 (operational) to 4 (major outage)", v)
		}
		body["status"] = s
	}
	if v, ok, err := in.integer("order"); err != nil {
		return nil, err
	} else if ok {
		body["order"] = v
	}
	if v, ok, err := in.integer("group_id"); err != nil {
		return nil, err
	} else if ok {
		body["group_id"] = v // 0 means no group, in both APIs
	}
	if v, ok := in.str("link"); ok {
		body["link"] = v
	}
	if v, ok, err := in.boolean("enabled"); err != nil {
		return nil, err
	} else if ok {
		body["enabled"] = v
	}
	if v, ok := in.stringMap("meta"); ok {
		body["meta"] = v
	}
	if v, ok := in.list("tags"); ok {
		body["tags"] = v
	}
	if v, ok := in.str("description"); ok {
		var current []models.Translation
		if existing != nil {
			current = existing.Translations
		}
		body["translations"] = withTranslation(current, "description", v)
	}
	return body, nil
}

// withTranslation returns all translations, with field in the content
// locale set to value. Vigie replaces an entity's translations as a whole
// on update, so the others must be sent back unchanged.
func withTranslation(current []models.Translation, field, value string) []map[string]string {
	locale := contentLocale()
	out := []map[string]string{}
	for _, t := range current {
		if t.Field == field && t.Locale == locale {
			continue
		}
		out = append(out, map[string]string{"locale": t.Locale, "field": t.Field, "value": t.Value})
	}
	if value != "" {
		out = append(out, map[string]string{"locale": locale, "field": field, "value": value})
	}
	return out
}

func createComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if !in.has("name") || !in.has("status") {
			fail(c, http.StatusBadRequest, "name and status are required")
			return
		}
		body, err := componentBody(in, nil)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		code, resp := forward(c, handlers.CreateComponent(db), http.MethodPost, nil, body)
		if forwardError(c, code, resp) {
			return
		}
		c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(createdID(resp, "component")), 10)}}
		showComponent(db)(c)
	}
}

func updateComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		existing, ok := findComponent(c, db)
		if !ok {
			return
		}
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		body, err := componentBody(in, &existing)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		code, resp := forward(c, handlers.UpdateComponent(db), http.MethodPut, c.Params, body)
		if forwardError(c, code, resp) {
			return
		}
		showComponent(db)(c)
	}
}

func deleteComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := findComponent(c, db); !ok {
			return
		}
		code, resp := forward(c, handlers.DeleteComponent(db), http.MethodDelete, c.Params, nil)
		if forwardError(c, code, resp) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
