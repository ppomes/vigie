package services

import (
	"errors"
	"testing"
	"time"

	"api/internal/models"
	"api/internal/testutil"

	"gorm.io/gorm"
)

func createMaintenance(t *testing.T, db *gorm.DB, componentID uint, status string, start, end time.Time) models.Maintenance {
	t.Helper()
	m := models.Maintenance{Name: "maint", Status: status, ComponentID: componentID, StartAt: start, EndAt: end}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	return m
}

func inTx(t *testing.T, db *gorm.DB, fn func(tx *gorm.DB) error) {
	t.Helper()
	if err := db.Transaction(fn); err != nil {
		t.Fatalf("transaction: %v", err)
	}
}

func TestMaintenanceRestoresPreviousStatus(t *testing.T) {
	db := testutil.NewDB(t)
	c := testutil.CreateComponent(t, db, "API", ComponentPartialOutage)
	now := time.Now()
	m := createMaintenance(t, db, c.ID, models.MaintenanceInProgress, now, now.Add(time.Hour))

	inTx(t, db, func(tx *gorm.DB) error { return EnterMaintenance(tx, &m) })
	if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentUnderMaintenance {
		t.Fatalf("during maintenance: got %q, want %q", got, ComponentUnderMaintenance)
	}

	inTx(t, db, func(tx *gorm.DB) error { return LeaveMaintenance(tx, &m, c.ID) })
	if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentPartialOutage {
		t.Fatalf("after maintenance: got %q, want the previous status %q", got, ComponentPartialOutage)
	}
}

func TestMaintenanceKeepsStatusSetDuringMaintenance(t *testing.T) {
	db := testutil.NewDB(t)
	c := testutil.CreateComponent(t, db, "API", ComponentOperational)
	now := time.Now()
	m := createMaintenance(t, db, c.ID, models.MaintenanceInProgress, now, now.Add(time.Hour))

	inTx(t, db, func(tx *gorm.DB) error { return EnterMaintenance(tx, &m) })
	// A monitoring tool reports an outage while the maintenance runs.
	inTx(t, db, func(tx *gorm.DB) error { return SetComponentStatus(tx, c.ID, ComponentMajorOutage) })
	inTx(t, db, func(tx *gorm.DB) error { return LeaveMaintenance(tx, &m, c.ID) })

	if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentMajorOutage {
		t.Fatalf("got %q, want the explicitly set %q", got, ComponentMajorOutage)
	}
}

func TestOverlappingMaintenancesRestoreOriginalStatus(t *testing.T) {
	for _, firstToEnd := range []string{"first", "second"} {
		t.Run(firstToEnd+" ends first", func(t *testing.T) {
			db := testutil.NewDB(t)
			c := testutil.CreateComponent(t, db, "DB", ComponentDegradedPerformance)
			now := time.Now()
			m1 := createMaintenance(t, db, c.ID, models.MaintenanceInProgress, now, now.Add(time.Hour))
			inTx(t, db, func(tx *gorm.DB) error { return EnterMaintenance(tx, &m1) })
			m2 := createMaintenance(t, db, c.ID, models.MaintenanceInProgress, now, now.Add(2*time.Hour))
			inTx(t, db, func(tx *gorm.DB) error { return EnterMaintenance(tx, &m2) })

			end := func(m *models.Maintenance) {
				inTx(t, db, func(tx *gorm.DB) error {
					if err := tx.Model(m).Update("status", models.MaintenanceCompleted).Error; err != nil {
						return err
					}
					return LeaveMaintenance(tx, m, c.ID)
				})
			}
			a, b := &m1, &m2
			if firstToEnd == "second" {
				a, b = &m2, &m1
			}

			end(a)
			if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentUnderMaintenance {
				t.Fatalf("one maintenance still running: got %q, want %q", got, ComponentUnderMaintenance)
			}
			end(b)
			if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentDegradedPerformance {
				t.Fatalf("both ended: got %q, want the original %q", got, ComponentDegradedPerformance)
			}
		})
	}
}

func TestWorkerStartsAndEndsMaintenance(t *testing.T) {
	db := testutil.NewDB(t)
	c := testutil.CreateComponent(t, db, "API", ComponentPartialOutage)
	now := time.Now()
	m := createMaintenance(t, db, c.ID, models.MaintenanceScheduled, now.Add(-time.Minute), now.Add(time.Hour))
	w := NewMaintenanceWorker(db)

	w.updateMaintenanceStatuses()
	if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentUnderMaintenance {
		t.Fatalf("after start: got %q, want %q", got, ComponentUnderMaintenance)
	}

	// Move the end into the past, as if the window elapsed.
	if err := db.Model(&m).Update("end_at", now.Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	w.updateMaintenanceStatuses()

	var after models.Maintenance
	if err := db.First(&after, m.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Status != models.MaintenanceCompleted {
		t.Fatalf("maintenance status: got %q, want %q", after.Status, models.MaintenanceCompleted)
	}
	if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentPartialOutage {
		t.Fatalf("after end: got %q, want the previous status %q", got, ComponentPartialOutage)
	}
}

func TestWorkerIgnoresOpenIncidents(t *testing.T) {
	db := testutil.NewDB(t)
	c := testutil.CreateComponent(t, db, "API", ComponentOperational)
	if err := db.Create(&models.Incident{Name: "slow", Status: "investigating", ComponentID: c.ID}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	m := createMaintenance(t, db, c.ID, models.MaintenanceScheduled, now.Add(-time.Minute), now.Add(time.Hour))
	w := NewMaintenanceWorker(db)

	w.updateMaintenanceStatuses()
	if err := db.Model(&m).Update("end_at", now.Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	w.updateMaintenanceStatuses()

	// The open incident must not turn the component into an outage.
	if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentOperational {
		t.Fatalf("got %q, want %q", got, ComponentOperational)
	}
}

func TestSetComponentStatus(t *testing.T) {
	db := testutil.NewDB(t)
	c := testutil.CreateComponent(t, db, "API", ComponentOperational)

	// Setting the status it already has must succeed (MySQL reports zero
	// affected rows for such an update).
	inTx(t, db, func(tx *gorm.DB) error { return SetComponentStatus(tx, c.ID, ComponentOperational) })
	inTx(t, db, func(tx *gorm.DB) error { return SetComponentStatus(tx, c.ID, ComponentMajorOutage) })
	if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentMajorOutage {
		t.Fatalf("got %q, want %q", got, ComponentMajorOutage)
	}

	err := db.Transaction(func(tx *gorm.DB) error { return SetComponentStatus(tx, 999, ComponentOperational) })
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing component: got %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestWorkerFailureDoesNotBlockOtherMaintenances(t *testing.T) {
	db := testutil.NewDB(t)
	c := testutil.CreateComponent(t, db, "API", ComponentOperational)
	now := time.Now()
	// The first maintenance points to a component that does not exist, so
	// starting it fails.
	broken := createMaintenance(t, db, 999, models.MaintenanceScheduled, now.Add(-time.Minute), now.Add(time.Hour))
	good := createMaintenance(t, db, c.ID, models.MaintenanceScheduled, now.Add(-time.Minute), now.Add(time.Hour))

	NewMaintenanceWorker(db).updateMaintenanceStatuses()

	var got models.Maintenance
	if err := db.First(&got, broken.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != models.MaintenanceScheduled {
		t.Fatalf("broken maintenance: got %q, want it rolled back to %q", got.Status, models.MaintenanceScheduled)
	}
	var started models.Maintenance
	if err := db.First(&started, good.ID).Error; err != nil {
		t.Fatal(err)
	}
	if started.Status != models.MaintenanceInProgress {
		t.Fatalf("good maintenance: got %q, want %q", started.Status, models.MaintenanceInProgress)
	}
	if s := testutil.ComponentStatus(t, db, c.ID); s != ComponentUnderMaintenance {
		t.Fatalf("component: got %q, want %q", s, ComponentUnderMaintenance)
	}
}
