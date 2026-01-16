package services

import (
	"context"
	"log"
	"sync"
	"time"

	"api/internal/models"

	"gorm.io/gorm"
)

// MaintenanceWorker handles automatic status updates for maintenances
type MaintenanceWorker struct {
	DB           *gorm.DB
	workerCtx    context.Context
	workerCancel context.CancelFunc
	workerWg     sync.WaitGroup
}

// NewMaintenanceWorker creates a new maintenance worker
func NewMaintenanceWorker(db *gorm.DB) *MaintenanceWorker {
	return &MaintenanceWorker{
		DB: db,
	}
}

// Start begins the background maintenance status checking
func (m *MaintenanceWorker) Start() {
	m.workerCtx, m.workerCancel = context.WithCancel(context.Background())
	m.workerWg.Add(1)
	go m.processMaintenanceStatuses()
	log.Println("Maintenance status worker started")
}

// Stop gracefully stops the maintenance worker
func (m *MaintenanceWorker) Stop() {
	if m.workerCancel != nil {
		m.workerCancel()
		m.workerWg.Wait()
		log.Println("Maintenance status worker stopped")
	}
}

// processMaintenanceStatuses checks and updates maintenance statuses every minute
func (m *MaintenanceWorker) processMaintenanceStatuses() {
	defer m.workerWg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Run immediately on start
	m.updateMaintenanceStatuses()

	for {
		select {
		case <-m.workerCtx.Done():
			return
		case <-ticker.C:
			m.updateMaintenanceStatuses()
		}
	}
}

// updateMaintenanceStatuses checks all active maintenances and updates their statuses
func (m *MaintenanceWorker) updateMaintenanceStatuses() {
	now := time.Now()

	// Start transaction
	tx := m.DB.Begin()

	// Find scheduled maintenances that should now be in_progress
	var scheduledMaintenances []models.Maintenance
	if err := tx.Where("status = ? AND start_at <= ? AND end_at > ?",
		models.MaintenanceScheduled, now, now).
		Find(&scheduledMaintenances).Error; err != nil {
		tx.Rollback()
		log.Printf("Maintenance worker: Error fetching scheduled maintenances: %v", err)
		return
	}

	// Update scheduled -> in_progress
	for _, maintenance := range scheduledMaintenances {
		if err := tx.Model(&maintenance).Update("status", models.MaintenanceInProgress).Error; err != nil {
			log.Printf("Maintenance worker: Error updating maintenance %d to in_progress: %v", maintenance.ID, err)
			continue
		}

		// Update component status to under_maintenance
		if err := tx.Model(&models.Component{}).Where("id = ?", maintenance.ComponentID).
			Update("status", "under_maintenance").Error; err != nil {
			log.Printf("Maintenance worker: Error updating component %d status: %v", maintenance.ComponentID, err)
		}

		log.Printf("Maintenance %d (%s) started - now in_progress", maintenance.ID, maintenance.Name)
	}

	// Find in_progress maintenances that should now be completed
	var activeMaintenances []models.Maintenance
	if err := tx.Where("status = ? AND end_at <= ?",
		models.MaintenanceInProgress, now).
		Find(&activeMaintenances).Error; err != nil {
		tx.Rollback()
		log.Printf("Maintenance worker: Error fetching active maintenances: %v", err)
		return
	}

	// Update in_progress -> completed
	for _, maintenance := range activeMaintenances {
		if err := tx.Model(&maintenance).Update("status", models.MaintenanceCompleted).Error; err != nil {
			log.Printf("Maintenance worker: Error updating maintenance %d to completed: %v", maintenance.ID, err)
			continue
		}

		// Update component status - check for active incidents first
		componentID := maintenance.ComponentID
		newStatus := "operational"

		// Check if there are any active incidents for this component
		var worstIncident models.Incident
		err := tx.Where("component_id = ? AND status != ?", componentID, "resolved").
			Order("FIELD(status, 'investigating', 'identified', 'monitoring')").
			First(&worstIncident).Error

		if err == nil {
			// Active incident found - set status based on incident
			switch worstIncident.Status {
			case "investigating":
				newStatus = "major_outage"
			case "identified":
				newStatus = "partial_outage"
			case "monitoring":
				newStatus = "degraded_performance"
			}
		} else if err != gorm.ErrRecordNotFound {
			log.Printf("Maintenance worker: Error checking incidents for component %d: %v", componentID, err)
			continue
		}

		// Check if there are other active maintenances for this component
		var otherMaintenance models.Maintenance
		if err := tx.Where("id != ? AND component_id = ? AND status = ?",
			maintenance.ID, componentID, models.MaintenanceInProgress).
			First(&otherMaintenance).Error; err == nil {
			// Another maintenance is still active
			newStatus = "under_maintenance"
		}

		if err := tx.Model(&models.Component{}).Where("id = ?", componentID).
			Update("status", newStatus).Error; err != nil {
			log.Printf("Maintenance worker: Error updating component %d status: %v", componentID, err)
		}

		log.Printf("Maintenance %d (%s) completed - component status set to %s", maintenance.ID, maintenance.Name, newStatus)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Maintenance worker: Error committing transaction: %v", err)
		return
	}

	if len(scheduledMaintenances) > 0 || len(activeMaintenances) > 0 {
		log.Printf("Maintenance worker: Updated %d started, %d completed",
			len(scheduledMaintenances), len(activeMaintenances))
	}
}
