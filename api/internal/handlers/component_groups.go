package handlers

import (
	"errors"
	"net/http"

	"api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ComponentGroupRequest is the body of POST and PUT /admin/component-groups.
// On update, empty fields are left unchanged.
type ComponentGroupRequest struct {
	Name       string `json:"name" binding:"omitempty,max=255"`
	Order      *int   `json:"order"`
	Collapse   string `json:"collapse" binding:"omitempty,oneof=never always operational"`
	Visibility string `json:"visibility" binding:"omitempty,oneof=public authenticated"`
	// Translations holds the group name per locale. Only "name" is accepted.
	// On update, a present list, even empty, replaces the translations;
	// absent leaves them unchanged.
	Translations *[]TranslationRequest `json:"translations"`
}

func (r ComponentGroupRequest) translations() []TranslationRequest {
	if r.Translations == nil {
		return nil
	}
	return *r.Translations
}

func validateGroupTranslations(translations []TranslationRequest) string {
	for _, t := range translations {
		if t.Field != "name" {
			return "Component groups only accept the \"name\" field"
		}
	}
	return ""
}

func replaceGroupTranslations(tx *gorm.DB, groupID uint, translations []TranslationRequest) error {
	if err := tx.Unscoped().Where("entity_id = ? AND entity_type = ?", groupID, "component_group").
		Delete(&models.Translation{}).Error; err != nil {
		return err
	}
	for _, t := range translations {
		tr := models.Translation{EntityID: groupID, EntityType: "component_group", Locale: t.Locale, Field: t.Field, Value: t.Value}
		if err := tx.Create(&tr).Error; err != nil {
			return err
		}
	}
	return nil
}

// ListComponentGroups returns all groups with their components, in order.
// GET /api/v1/admin/component-groups
func ListComponentGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var groups []models.ComponentGroup
		if err := models.ByDisplayOrder(db).Preload("Translations").
			Preload("Components", models.ByDisplayOrder).
			Find(&groups).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch component groups"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"component_groups": groups})
	}
}

// CreateComponentGroup creates a group.
// POST /api/v1/admin/component-groups
func CreateComponentGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ComponentGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}
		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
			return
		}
		if msg := validateGroupTranslations(req.translations()); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}

		group := models.ComponentGroup{
			Name:       req.Name,
			Collapse:   models.GroupCollapseNever,
			Visibility: models.GroupVisibilityPublic,
		}
		if req.Order != nil {
			group.Order = *req.Order
		}
		if req.Collapse != "" {
			group.Collapse = req.Collapse
		}
		if req.Visibility != "" {
			group.Visibility = req.Visibility
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&group).Error; err != nil {
				return err
			}
			return replaceGroupTranslations(tx, group.ID, req.translations())
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create component group"})
			return
		}
		db.Preload("Translations").First(&group, group.ID)
		c.JSON(http.StatusCreated, gin.H{"message": "Component group created", "component_group": group})
	}
}

// UpdateComponentGroup edits a group. Translations, when given, replace the
// existing ones.
// PUT /api/v1/admin/component-groups/:id
func UpdateComponentGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c, "id", "component group")
		if !ok {
			return
		}
		var req ComponentGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}
		if msg := validateGroupTranslations(req.translations()); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}

		var group models.ComponentGroup
		if err := db.First(&group, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Component group not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch component group"})
			return
		}
		if req.Name != "" {
			group.Name = req.Name
		}
		if req.Order != nil {
			group.Order = *req.Order
		}
		if req.Collapse != "" {
			group.Collapse = req.Collapse
		}
		if req.Visibility != "" {
			group.Visibility = req.Visibility
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(&group).Error; err != nil {
				return err
			}
			if req.Translations != nil {
				return replaceGroupTranslations(tx, group.ID, *req.Translations)
			}
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save component group"})
			return
		}
		db.Preload("Translations").First(&group, group.ID)
		c.JSON(http.StatusOK, gin.H{"message": "Component group saved", "component_group": group})
	}
}

// DeleteComponentGroup deletes a group. Its components stay, ungrouped.
// DELETE /api/v1/admin/component-groups/:id
func DeleteComponentGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c, "id", "component group")
		if !ok {
			return
		}
		var group models.ComponentGroup
		if err := db.First(&group, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Component group not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch component group"})
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&models.Component{}).Where("group_id = ?", group.ID).Update("group_id", nil).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("entity_id = ? AND entity_type = ?", group.ID, "component_group").
				Delete(&models.Translation{}).Error; err != nil {
				return err
			}
			return tx.Delete(&group).Error
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete component group"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Component group deleted"})
	}
}

// resolveGroupID checks a group_id from a component request. 0 means "no
// group". It writes the error response and returns false when the group does
// not exist.
func resolveGroupID(c *gin.Context, db *gorm.DB, requested *uint) (*uint, bool) {
	if requested == nil || *requested == 0 {
		return nil, true
	}
	var group models.ComponentGroup
	if err := db.Select("id").First(&group, *requested).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Component group not found"})
			return nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify component group"})
		return nil, false
	}
	id := group.ID
	return &id, true
}
