// Package importer moves a Cachet 2.4 installation into Vigie.
package importer

import "time"

// The Cachet 2.4 tables the importer reads. Only the columns it uses are
// mapped; the source database is never written.

type cachetGroup struct {
	ID        uint
	Name      string
	Order     int `gorm:"column:order"`
	Collapsed int
	Visible   int
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

func (cachetGroup) TableName() string { return "component_groups" }

type cachetComponent struct {
	ID          uint
	Name        string
	Description *string
	Status      int
	Order       int `gorm:"column:order"`
	GroupID     *uint
	Enabled     bool
	Link        *string
	Meta        *string
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
	DeletedAt   *time.Time
}

func (cachetComponent) TableName() string { return "components" }

type cachetTag struct {
	ID   uint
	Name string
	Slug string
}

func (cachetTag) TableName() string { return "tags" }

type cachetTaggable struct {
	TagID        uint
	TaggableType string
	TaggableID   uint
}

func (cachetTaggable) TableName() string { return "taggables" }

type cachetUser struct {
	ID              uint
	Username        string
	Password        string
	Email           string
	APIKey          *string `gorm:"column:api_key"`
	Active          bool
	Level           int
	Google2faSecret *string `gorm:"column:google_2fa_secret"`
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
}

func (cachetUser) TableName() string { return "users" }

type cachetIncident struct {
	ID            uint
	ComponentID   *uint
	Name          string
	Status        int
	Message       *string
	Visible       bool
	Stickied      bool
	OccurredAt    *time.Time
	UserID        *uint
	Notifications bool
	CreatedAt     *time.Time
	UpdatedAt     *time.Time
	DeletedAt     *time.Time
}

func (cachetIncident) TableName() string { return "incidents" }

type cachetIncidentComponent struct {
	IncidentID  uint
	ComponentID uint
	StatusID    *int
}

func (cachetIncidentComponent) TableName() string { return "incident_components" }

type cachetIncidentUpdate struct {
	ID         uint
	IncidentID uint
	Status     int
	Message    *string
	UserID     *uint
	CreatedAt  *time.Time
	UpdatedAt  *time.Time
}

func (cachetIncidentUpdate) TableName() string { return "incident_updates" }

type cachetSchedule struct {
	ID          uint
	Name        string
	Message     *string
	Status      int
	ScheduledAt *time.Time
	CompletedAt *time.Time
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
	DeletedAt   *time.Time
}

func (cachetSchedule) TableName() string { return "schedules" }

type cachetScheduleComponent struct {
	ScheduleID  uint
	ComponentID uint
}

func (cachetScheduleComponent) TableName() string { return "schedule_components" }

type cachetSubscriber struct {
	ID              uint
	Email           string
	VerifyCode      *string
	VerifiedAt      *time.Time
	Global          bool
	PhoneNumber     *string
	SlackWebhookURL *string `gorm:"column:slack_webhook_url"`
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
}

func (cachetSubscriber) TableName() string { return "subscribers" }

type cachetSubscription struct {
	SubscriberID uint
	ComponentID  uint
}

func (cachetSubscription) TableName() string { return "subscriptions" }

type cachetSetting struct {
	Name  string
	Value *string
}

func (cachetSetting) TableName() string { return "settings" }
