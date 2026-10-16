package models

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ComponentGroup gathers components on the status page, such as "Web" or
// "Backend". Its name is translated like the rest of Vigie.
type ComponentGroup struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	Name  string `gorm:"size:255;not null" json:"name"`
	Order int    `gorm:"default:0" json:"order"`
	// Collapse is how the group renders on the public page: one of the
	// GroupCollapse* values.
	Collapse string `gorm:"size:20;not null;default:'never'" json:"collapse"`
	// Visibility is who sees the group on the public page: one of the
	// GroupVisibility* values. Hidden groups hide their components.
	Visibility   string         `gorm:"size:20;not null;default:'public'" json:"visibility"`
	Translations []Translation  `gorm:"polymorphic:Entity;polymorphicValue:component_group" json:"translations,omitempty"`
	Components   []Component    `gorm:"foreignKey:GroupID" json:"components,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Collapse behaviours, as in Cachet: never collapsed, always collapsed, or
// collapsed only while all its components are operational.
const (
	GroupCollapseNever       = "never"
	GroupCollapseAlways      = "always"
	GroupCollapseOperational = "operational"
)

// Visibility values: shown to everyone, or only to logged-in users, which
// keeps the group off the anonymous public page.
const (
	GroupVisibilityPublic        = "public"
	GroupVisibilityAuthenticated = "authenticated"
)

// ByDisplayOrder sorts by the "order" column, then id. The column name is a
// reserved word: clause.OrderByColumn quotes it for each database, where a
// raw "\"order\"" is a string literal on MySQL and sorts nothing.
func ByDisplayOrder(db *gorm.DB) *gorm.DB {
	return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}}).Order("id")
}
