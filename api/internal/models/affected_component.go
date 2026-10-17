package models

import "time"

// IncidentComponent is a component affected by an incident. Every incident
// has one for its main ComponentID, and may have more.
type IncidentComponent struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	IncidentID  uint       `gorm:"not null;uniqueIndex:idx_incident_component" json:"incident_id"`
	ComponentID uint       `gorm:"not null;uniqueIndex:idx_incident_component;index" json:"component_id"`
	Component   *Component `gorm:"foreignKey:ComponentID" json:"component,omitempty"`
	// ComponentStatus is the status the incident set on the component, or
	// empty when it left it unchanged.
	ComponentStatus string    `gorm:"size:50" json:"component_status,omitempty"`
	CreatedAt       time.Time `json:"-"`
	UpdatedAt       time.Time `json:"-"`
}

// MaintenanceComponent is a component covered by a maintenance. Every
// maintenance has one for its main ComponentID, and may have more.
type MaintenanceComponent struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	MaintenanceID uint       `gorm:"not null;uniqueIndex:idx_maintenance_component" json:"maintenance_id"`
	ComponentID   uint       `gorm:"not null;uniqueIndex:idx_maintenance_component;index" json:"component_id"`
	Component     *Component `gorm:"foreignKey:ComponentID" json:"component,omitempty"`
	// PreviousStatus is the component status to restore when the
	// maintenance ends. It is set when the maintenance starts.
	PreviousStatus string    `gorm:"size:50" json:"-"`
	CreatedAt      time.Time `json:"-"`
	UpdatedAt      time.Time `json:"-"`
}

// OptionalID turns 0 into nil, for nullable foreign keys.
func OptionalID(id uint) *uint {
	if id == 0 {
		return nil
	}
	return &id
}

// IDValue turns nil into 0.
func IDValue(id *uint) uint {
	if id == nil {
		return 0
	}
	return *id
}
