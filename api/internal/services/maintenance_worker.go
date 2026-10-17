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

// updateMaintenanceStatuses starts scheduled maintenances whose window has
// begun, and completes in-progress ones whose window has ended.
//
// Each maintenance is handled in its own transaction: a failure rolls back
// that maintenance alone, which is retried on the next tick, and does not
// block the others.
func (m *MaintenanceWorker) updateMaintenanceStatuses() {
	now := time.Now()

	// Find scheduled maintenances that should now be in_progress
	var scheduledMaintenances []models.Maintenance
	if err := m.DB.Where("status = ? AND start_at <= ? AND end_at > ?",
		models.MaintenanceScheduled, now, now).
		Order("id").Find(&scheduledMaintenances).Error; err != nil {
		log.Printf("Maintenance worker: Error fetching scheduled maintenances: %v", err)
		return
	}

	started := 0
	for i := range scheduledMaintenances {
		maintenance := &scheduledMaintenances[i]
		err := m.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(maintenance).Update("status", models.MaintenanceInProgress).Error; err != nil {
				return err
			}
			maintenance.Status = models.MaintenanceInProgress
			return EnterMaintenance(tx, maintenance)
		})
		if err != nil {
			log.Printf("Maintenance worker: Error starting maintenance %d: %v", maintenance.ID, err)
			continue
		}
		started++
		log.Printf("Maintenance %d (%s) started - now in_progress", maintenance.ID, maintenance.Name)
	}

	// Find in_progress maintenances that should now be completed
	var activeMaintenances []models.Maintenance
	if err := m.DB.Where("status = ? AND end_at <= ?",
		models.MaintenanceInProgress, now).
		Order("id").Find(&activeMaintenances).Error; err != nil {
		log.Printf("Maintenance worker: Error fetching active maintenances: %v", err)
		return
	}

	completed := 0
	for i := range activeMaintenances {
		maintenance := &activeMaintenances[i]
		err := m.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(maintenance).Update("status", models.MaintenanceCompleted).Error; err != nil {
				return err
			}
			maintenance.Status = models.MaintenanceCompleted
			return LeaveMaintenance(tx, maintenance)
		})
		if err != nil {
			log.Printf("Maintenance worker: Error completing maintenance %d: %v", maintenance.ID, err)
			continue
		}
		completed++
		log.Printf("Maintenance %d (%s) completed", maintenance.ID, maintenance.Name)
	}

	if started > 0 || completed > 0 {
		log.Printf("Maintenance worker: Updated %d started, %d completed", started, completed)
	}
}
