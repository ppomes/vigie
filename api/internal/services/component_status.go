package services

import (
	"errors"

	"api/internal/models"

	"gorm.io/gorm"
)

// Component statuses that the API, the admin and incidents may set directly.
// "under_maintenance" is only set while a maintenance is in progress.
const (
	ComponentOperational         = "operational"
	ComponentDegradedPerformance = "degraded_performance"
	ComponentPartialOutage       = "partial_outage"
	ComponentMajorOutage         = "major_outage"
	ComponentUnderMaintenance    = "under_maintenance"
)

// Component status is explicit: it only changes when someone sets it, through
// the component API, through an incident that asks for it, or for the duration
// of a maintenance. Nothing recomputes it from incidents.

// SetComponentStatus sets the status of a component. It returns
// gorm.ErrRecordNotFound when the component does not exist.
//
// Existence is checked separately: MySQL reports zero affected rows for an
// update that changes nothing, so RowsAffected cannot tell a missing component
// from a status that is already set.
func SetComponentStatus(tx *gorm.DB, componentID uint, status string) error {
	var component models.Component
	if err := tx.Select("id").First(&component, componentID).Error; err != nil {
		return err
	}
	return tx.Model(&component).Update("status", status).Error
}

// otherActiveMaintenance returns another in-progress maintenance on the same
// component, or nil if there is none.
func otherActiveMaintenance(tx *gorm.DB, componentID, excludeID uint) (*models.Maintenance, error) {
	var other models.Maintenance
	err := tx.Where("component_id = ? AND status = ? AND id <> ?",
		componentID, models.MaintenanceInProgress, excludeID).
		Order("id").First(&other).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &other, nil
}

// EnterMaintenance puts the component of m under maintenance, and records on m
// the status to restore when the maintenance ends. The caller sets m.Status.
//
// When another maintenance is already in progress on the same component, the
// component is already under maintenance: m inherits the status recorded by
// that maintenance, so the original status is restored whichever ends last.
func EnterMaintenance(tx *gorm.DB, m *models.Maintenance) error {
	previous := ""
	other, err := otherActiveMaintenance(tx, m.ComponentID, m.ID)
	if err != nil {
		return err
	}
	if other != nil {
		previous = other.PreviousComponentStatus
	} else {
		var component models.Component
		if err := tx.First(&component, m.ComponentID).Error; err != nil {
			return err
		}
		previous = component.Status
	}
	if previous == "" || previous == ComponentUnderMaintenance {
		previous = ComponentOperational
	}

	m.PreviousComponentStatus = previous
	if err := tx.Model(m).Update("previous_component_status", previous).Error; err != nil {
		return err
	}
	return SetComponentStatus(tx, m.ComponentID, ComponentUnderMaintenance)
}

// LeaveMaintenance restores the status the component had before m started,
// for the given component (the one m was attached to while in progress).
//
// Nothing changes while another maintenance is still in progress on that
// component, or when someone changed the component status during the
// maintenance: an explicit status always wins.
func LeaveMaintenance(tx *gorm.DB, m *models.Maintenance, componentID uint) error {
	other, err := otherActiveMaintenance(tx, componentID, m.ID)
	if err != nil {
		return err
	}
	if other != nil {
		return nil
	}

	var component models.Component
	if err := tx.First(&component, componentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if component.Status != ComponentUnderMaintenance {
		return nil
	}

	restore := m.PreviousComponentStatus
	if restore == "" || restore == ComponentUnderMaintenance {
		restore = ComponentOperational
	}
	return SetComponentStatus(tx, componentID, restore)
}
