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

// otherActiveMaintenances selects the ids of the other in-progress
// maintenances. It does not read maintenance_components, so it can be used
// in an update of that table on MySQL.
func otherActiveMaintenances(tx *gorm.DB, excludeMaintenanceID uint) *gorm.DB {
	return tx.Session(&gorm.Session{NewDB: true}).Model(&models.Maintenance{}).Select("id").
		Where("status = ? AND id <> ? AND deleted_at IS NULL", models.MaintenanceInProgress, excludeMaintenanceID)
}

// otherActivePreviousStatus looks for another in-progress maintenance
// covering the component. It returns the status that maintenance recorded,
// and whether there is one.
func otherActivePreviousStatus(tx *gorm.DB, componentID, excludeMaintenanceID uint) (string, bool, error) {
	var row models.MaintenanceComponent
	err := tx.Joins("JOIN maintenances ON maintenances.id = maintenance_components.maintenance_id").
		Where("maintenance_components.component_id = ? AND maintenances.status = ? AND maintenances.id <> ? AND maintenances.deleted_at IS NULL",
			componentID, models.MaintenanceInProgress, excludeMaintenanceID).
		Order("maintenances.id").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.PreviousStatus, true, nil
}

// MaintenanceComponents returns the components covered by a maintenance.
func MaintenanceComponents(tx *gorm.DB, maintenanceID uint) ([]models.MaintenanceComponent, error) {
	var rows []models.MaintenanceComponent
	err := tx.Where("maintenance_id = ?", maintenanceID).Order("id").Find(&rows).Error
	return rows, err
}

// EnterMaintenance puts every component of m under maintenance, recording on
// each the status to restore when the maintenance ends.
func EnterMaintenance(tx *gorm.DB, m *models.Maintenance) error {
	rows, err := MaintenanceComponents(tx, m.ID)
	if err != nil {
		return err
	}
	return EnterMaintenanceComponents(tx, m, rows)
}

// EnterMaintenanceComponents puts the given components of m under
// maintenance. When another maintenance is already in progress on a
// component and the component is still under maintenance, m inherits the
// status recorded by that maintenance, so the original status is restored
// whichever ends last. When someone set the status explicitly in between,
// that status is the one to restore, for every maintenance in progress.
func EnterMaintenanceComponents(tx *gorm.DB, m *models.Maintenance, rows []models.MaintenanceComponent) error {
	for i := range rows {
		row := &rows[i]
		var component models.Component
		if err := tx.First(&component, row.ComponentID).Error; err != nil {
			return err
		}
		previous := component.Status
		if previous == ComponentUnderMaintenance {
			inherited, found, err := otherActivePreviousStatus(tx, row.ComponentID, m.ID)
			if err != nil {
				return err
			}
			if found {
				previous = inherited
			}
		}
		if previous == "" || previous == ComponentUnderMaintenance {
			previous = ComponentOperational
		}
		row.PreviousStatus = previous
		if err := tx.Model(row).Update("previous_status", previous).Error; err != nil {
			return err
		}
		if component.Status != ComponentUnderMaintenance {
			if err := tx.Model(&models.MaintenanceComponent{}).
				Where("component_id = ? AND maintenance_id IN (?)", row.ComponentID, otherActiveMaintenances(tx, m.ID)).
				Update("previous_status", previous).Error; err != nil {
				return err
			}
		}
		if err := SetComponentStatus(tx, row.ComponentID, ComponentUnderMaintenance); err != nil {
			return err
		}
	}
	return nil
}

// LeaveMaintenance restores, for every component of m, the status it had
// before m started.
func LeaveMaintenance(tx *gorm.DB, m *models.Maintenance) error {
	rows, err := MaintenanceComponents(tx, m.ID)
	if err != nil {
		return err
	}
	return LeaveMaintenanceComponents(tx, m, rows)
}

// LeaveMaintenanceComponents restores the given components of m. A
// component is left alone while another maintenance is still in progress on
// it, or when someone changed its status during the maintenance: an explicit
// status always wins.
func LeaveMaintenanceComponents(tx *gorm.DB, m *models.Maintenance, rows []models.MaintenanceComponent) error {
	for _, row := range rows {
		_, found, err := otherActivePreviousStatus(tx, row.ComponentID, m.ID)
		if err != nil {
			return err
		}
		if found {
			continue
		}

		var component models.Component
		if err := tx.First(&component, row.ComponentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if component.Status != ComponentUnderMaintenance {
			continue
		}

		restore := row.PreviousStatus
		if restore == "" || restore == ComponentUnderMaintenance {
			restore = ComponentOperational
		}
		if err := SetComponentStatus(tx, row.ComponentID, restore); err != nil {
			return err
		}
	}
	return nil
}
