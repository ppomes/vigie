package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"api/internal/models"
	"api/internal/testutil"

	"gorm.io/gorm"
)

func createMaintenance(t *testing.T, db *gorm.DB, componentID uint, status string, start, end time.Time) models.Maintenance {
	t.Helper()
	m := models.Maintenance{Name: "maint", Status: status, ComponentID: models.OptionalID(componentID), StartAt: start, EndAt: end}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	if err := db.Create(&models.MaintenanceComponent{MaintenanceID: m.ID, ComponentID: componentID}).Error; err != nil {
		t.Fatalf("create maintenance component: %v", err)
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

	inTx(t, db, func(tx *gorm.DB) error { return LeaveMaintenance(tx, &m) })
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
	inTx(t, db, func(tx *gorm.DB) error { return LeaveMaintenance(tx, &m) })

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
					return LeaveMaintenance(tx, m)
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
	if err := db.Create(&models.Incident{Name: "slow", Status: "investigating", ComponentID: models.OptionalID(c.ID)}).Error; err != nil {
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

func TestIncidentUpdateNotificationUsesUpdateMessage(t *testing.T) {
	db := testutil.NewDB(t)
	c := testutil.CreateComponent(t, db, "API", ComponentOperational)
	incident := models.Incident{Name: "Slow API", Status: "monitoring", ComponentID: models.OptionalID(c.ID)}
	if err := db.Create(&incident).Error; err != nil {
		t.Fatal(err)
	}
	update := models.IncidentUpdate{IncidentID: incident.ID, Status: "monitoring"}
	db.Create(&update)
	db.Create(&models.Translation{EntityID: update.ID, EntityType: "incident_update", Locale: "fr", Field: "message", Value: "Correctif déployé"})
	db.Create(&models.Subscriber{Email: "fr@example.com", Locale: "fr", Confirmed: true, UnsubscribeToken: "u1"})

	n := NewNotificationService(db, nil)
	queued, err := n.QueueIncidentUpdateNotifications(&incident, &update)
	if err != nil || queued != 1 {
		t.Fatalf("queued %d, err %v", queued, err)
	}
	var q models.NotificationQueue
	db.First(&q)
	if q.Subject != "[monitoring] Slow API" || !strings.Contains(q.Body, "Correctif déployé") {
		t.Fatalf("subject %q, body %q", q.Subject, q.Body)
	}
}

func TestStatusSetBetweenOverlappingMaintenancesIsRestored(t *testing.T) {
	for _, firstToEnd := range []string{"first", "second"} {
		t.Run(firstToEnd+" ends first", func(t *testing.T) {
			db := testutil.NewDB(t)
			c := testutil.CreateComponent(t, db, "DB", ComponentOperational)
			now := time.Now()
			m1 := createMaintenance(t, db, c.ID, models.MaintenanceInProgress, now, now.Add(time.Hour))
			inTx(t, db, func(tx *gorm.DB) error { return EnterMaintenance(tx, &m1) })
			// A monitoring tool reports an outage, then a second maintenance starts.
			inTx(t, db, func(tx *gorm.DB) error { return SetComponentStatus(tx, c.ID, ComponentMajorOutage) })
			m2 := createMaintenance(t, db, c.ID, models.MaintenanceInProgress, now, now.Add(2*time.Hour))
			inTx(t, db, func(tx *gorm.DB) error { return EnterMaintenance(tx, &m2) })

			end := func(m *models.Maintenance) {
				inTx(t, db, func(tx *gorm.DB) error {
					if err := tx.Model(m).Update("status", models.MaintenanceCompleted).Error; err != nil {
						return err
					}
					return LeaveMaintenance(tx, m)
				})
			}
			a, b := &m1, &m2
			if firstToEnd == "second" {
				a, b = &m2, &m1
			}
			end(a)
			end(b)
			if got := testutil.ComponentStatus(t, db, c.ID); got != ComponentMajorOutage {
				t.Fatalf("both ended: got %q, want the explicitly set %q", got, ComponentMajorOutage)
			}
		})
	}
}

func TestNotificationsReachOnlyFollowers(t *testing.T) {
	db := testutil.NewDB(t)
	api := testutil.CreateComponent(t, db, "API", ComponentOperational)
	web := testutil.CreateComponent(t, db, "Website", ComponentOperational)

	global := models.Subscriber{Email: "all@example.com", Confirmed: true, Global: true, UnsubscribeToken: "g"}
	apiFan := models.Subscriber{Email: "api@example.com", Confirmed: true, UnsubscribeToken: "a"}
	webFan := models.Subscriber{Email: "web@example.com", Confirmed: true, UnsubscribeToken: "w"}
	for _, s := range []*models.Subscriber{&global, &apiFan, &webFan} {
		db.Create(s)
	}
	// Global defaults to true: make the followers explicit.
	db.Model(&apiFan).Update("global", false)
	db.Model(&webFan).Update("global", false)
	db.Create(&models.Subscription{SubscriberID: apiFan.ID, ComponentID: api.ID})
	db.Create(&models.Subscription{SubscriberID: webFan.ID, ComponentID: web.ID})

	incident := models.Incident{Name: "API slow", Status: "investigating", ComponentID: models.OptionalID(api.ID)}
	db.Create(&incident)
	db.Create(&models.IncidentComponent{IncidentID: incident.ID, ComponentID: api.ID})

	t.Setenv("PUBLIC_URL", "https://status.example.com")
	t.Setenv("API_URL", "https://api.example.com")
	n := NewNotificationService(db, nil)
	if _, err := n.QueueIncidentNotifications(&incident, "created"); err != nil {
		t.Fatal(err)
	}

	var queued []models.NotificationQueue
	db.Order("email").Find(&queued)
	var emails []string
	for _, q := range queued {
		emails = append(emails, q.Email)
	}
	if strings.Join(emails, ",") != "all@example.com,api@example.com" {
		t.Fatalf("notified %v, want the global subscriber and the API follower", emails)
	}

	body := queued[0].Body
	for _, want := range []string{
		"https://status.example.com/?manage=",
		"https://api.example.com/api/v1/unsubscribe?token=",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("email body lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "{{") {
		t.Errorf("email body has an unreplaced placeholder:\n%s", body)
	}
}
