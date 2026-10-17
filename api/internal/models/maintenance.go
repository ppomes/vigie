package models

import (
	"time"

	"gorm.io/gorm"
)

// Maintenance represents a scheduled maintenance window
type Maintenance struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:255;not null" json:"name"`
	Status       string         `gorm:"size:50;not null;default:'scheduled'" json:"status"`
	ComponentID  uint           `gorm:"not null" json:"component_id"`
	Component    Component      `gorm:"foreignKey:ComponentID" json:"component,omitempty"`
	StartAt      time.Time      `gorm:"not null" json:"start_at"`
	EndAt        time.Time      `gorm:"not null" json:"end_at"`
	// PreviousComponentStatus is superseded by MaintenanceComponent.
	// PreviousStatus; it is only read to backfill older maintenances.
	PreviousComponentStatus string `gorm:"size:50" json:"-"`
	// Components lists every covered component, including ComponentID.
	Components []MaintenanceComponent `gorm:"foreignKey:MaintenanceID" json:"components,omitempty"`
	Translations []Translation  `gorm:"polymorphic:Entity;polymorphicValue:maintenance" json:"translations,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// MaintenanceStatus constants
const (
	MaintenanceScheduled  = "scheduled"
	MaintenanceInProgress = "in_progress"
	MaintenanceCompleted  = "completed"
)

// ValidMaintenanceStatuses returns valid maintenance status values
func ValidMaintenanceStatuses() []string {
	return []string{MaintenanceScheduled, MaintenanceInProgress, MaintenanceCompleted}
}

// IsValidMaintenanceStatus checks if a status is valid
func IsValidMaintenanceStatus(status string) bool {
	for _, s := range ValidMaintenanceStatuses() {
		if s == status {
			return true
		}
	}
	return false
}
