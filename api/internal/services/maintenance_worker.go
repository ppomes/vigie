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
	for i := range scheduledMaintenances {
		maintenance := &scheduledMaintenances[i]
		if err := tx.Model(maintenance).Update("status", models.MaintenanceInProgress).Error; err != nil {
			log.Printf("Maintenance worker: Error updating maintenance %d to in_progress: %v", maintenance.ID, err)
			continue
		}
		maintenance.Status = models.MaintenanceInProgress

		if err := EnterMaintenance(tx, maintenance); err != nil {
			log.Printf("Maintenance worker: Error putting component %d under maintenance: %v", maintenance.ComponentID, err)
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
	for i := range activeMaintenances {
		maintenance := &activeMaintenances[i]
		if err := tx.Model(maintenance).Update("status", models.MaintenanceCompleted).Error; err != nil {
			log.Printf("Maintenance worker: Error updating maintenance %d to completed: %v", maintenance.ID, err)
			continue
		}
		maintenance.Status = models.MaintenanceCompleted

		if err := LeaveMaintenance(tx, maintenance, maintenance.ComponentID); err != nil {
			log.Printf("Maintenance worker: Error restoring component %d status: %v", maintenance.ComponentID, err)
			continue
		}

		log.Printf("Maintenance %d (%s) completed", maintenance.ID, maintenance.Name)
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
