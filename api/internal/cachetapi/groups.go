package cachetapi

import (
	"fmt"
	"net/http"
	"strconv"

	"api/internal/handlers"
	"api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var groupSorts = map[string]string{"id": "id", "name": "name", "order": "order", "created_at": "created_at", "updated_at": "updated_at"}

// groupQuery reads groups with their components. Anonymous readers only see
// public groups, and their enabled components.
func groupQuery(c *gin.Context, db *gorm.DB) *gorm.DB {
	q := db.Model(&models.ComponentGroup{}).
		Preload("Components", func(tx *gorm.DB) *gorm.DB {
			q := models.ByDisplayOrder(tx).Preload("Translations").Preload("Tags")
			if !authenticated(c) {
				q = q.Where("enabled = ?", true)
			}
			return q
		})
	if !authenticated(c) {
		q = q.Where("visibility = ?", models.GroupVisibilityPublic)
	}
	return q
}

func listGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := sortQuery(c, groupQuery(c, db), groupSorts, by("order", false), by("id", false))
		if v, ok := c.GetQuery("name"); ok {
			q = q.Where("name = ?", v)
		}
		var groups []models.ComponentGroup
		meta, err := paginate(c, q, &groups)
		if err != nil {
			fail(c, http.StatusInternalServerError, "Failed to fetch component groups")
			return
		}
		data := make([]gin.H, len(groups))
		for i, g := range groups {
			data[i] = presentGroup(g)
		}
		list(c, meta, data)
	}
}

func findGroup(c *gin.Context, db *gorm.DB) (models.ComponentGroup, bool) {
	var g models.ComponentGroup
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err == nil {
		err = groupQuery(c, db).First(&g, id).Error
	}
	if err != nil {
		if notFound(err) {
			fail(c, http.StatusNotFound, "Component group not found")
		} else {
			fail(c, http.StatusInternalServerError, "Failed to fetch component group")
		}
		return g, false
	}
	return g, true
}

func showGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if g, ok := findGroup(c, db); ok {
			item(c, presentGroup(g))
		}
	}
}

// groupBody translates Cachet group fields. Cachet creates groups visible to
// logged-in users only unless told otherwise; creating keeps that default.
func groupBody(in input, creating bool) (map[string]any, error) {
	body := map[string]any{}
	if v, ok := in.str("name"); ok {
		body["name"] = v
	}
	if v, ok, err := in.integer("order"); err != nil {
		return nil, err
	} else if ok {
		body["order"] = v
	}
	if v, ok, err := in.integer("collapsed"); err != nil {
		return nil, err
	} else if ok {
		s, known := groupCollapseToVigie[v]
		if !known {
			return nil, fmt.Errorf("collapsed must be 0, 1 or 2")
		}
		body["collapse"] = s
	}
	if v, ok, err := in.integer("visible"); err != nil {
		return nil, err
	} else if ok {
		s, known := groupVisibleToVigie[v]
		if !known {
			return nil, fmt.Errorf("visible must be 0 or 1")
		}
		body["visibility"] = s
	} else if creating {
		body["visibility"] = models.GroupVisibilityAuthenticated
	}
	return body, nil
}

func createGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		body, err := groupBody(in, true)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		code, resp := forward(c, handlers.CreateComponentGroup(db), http.MethodPost, nil, body)
		if forwardError(c, code, resp) {
			return
		}
		c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(createdID(resp, "component_group")), 10)}}
		showGroup(db)(c)
	}
}

func updateGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := findGroup(c, db); !ok {
			return
		}
		in, err := readInput(c)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		body, err := groupBody(in, false)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		code, resp := forward(c, handlers.UpdateComponentGroup(db), http.MethodPut, c.Params, body)
		if forwardError(c, code, resp) {
			return
		}
		showGroup(db)(c)
	}
}

func deleteGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := findGroup(c, db); !ok {
			return
		}
		code, resp := forward(c, handlers.DeleteComponentGroup(db), http.MethodDelete, c.Params, nil)
		if forwardError(c, code, resp) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
