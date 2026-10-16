package models

import (
	"time"
)

// Audit action types
const (
	ActionLoginSuccess = "login_success"
	ActionLoginFailed  = "login_failed"
	ActionLogout       = "logout"
	ActionCreate       = "create"
	ActionRead         = "read"
	ActionUpdate       = "update"
	ActionDelete       = "delete"
)

// Audit resource types
const (
	ResourceUser        = "users"
	ResourceComponent   = "components"
	ResourceIncident    = "incidents"
	ResourceMaintenance = "maintenances"
	ResourceTemplate    = "templates"
	ResourceSubscriber  = "subscribers"
	ResourceSettings    = "settings"
	ResourceAPIKey      = "api_keys"
)

// AuditLog represents an audit trail entry for tracking user actions
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     *uint     `json:"user_id,omitempty"`
	Username   string    `gorm:"size:255;not null" json:"username"`
	Action     string    `gorm:"size:50;not null" json:"action"`
	Resource   string    `gorm:"size:100" json:"resource,omitempty"`
	ResourceID *uint     `json:"resource_id,omitempty"`
	Details    string    `gorm:"type:text" json:"details,omitempty"`
	IPAddress  string    `gorm:"size:45" json:"ip_address"`
	UserAgent  string    `gorm:"size:500" json:"user_agent,omitempty"`
	Success    bool      `gorm:"default:true" json:"success"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName specifies the table name for AuditLog model
func (AuditLog) TableName() string {
	return "audit_logs"
}
