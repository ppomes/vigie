package services

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"api/internal/models"
)

// AuditService handles audit log operations
type AuditService struct {
	DB *gorm.DB
}

// NewAuditService creates a new audit service
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{DB: db}
}

// Log creates an audit log entry for a user action
func (s *AuditService) Log(c *gin.Context, action, resource string, resourceID *uint, details map[string]interface{}, success bool) {
	user, _ := GetCurrentUser(c)

	log := models.AuditLog{
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Success:    success,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	}

	if user != nil {
		log.UserID = &user.ID
		log.Username = user.Username
	} else {
		log.Username = "anonymous"
	}

	if details != nil {
		detailsJSON, _ := json.Marshal(details)
		log.Details = string(detailsJSON)
	}

	s.DB.Create(&log)
}

// LogLogin creates an audit log entry for login attempts
func (s *AuditService) LogLogin(c *gin.Context, username string, userID *uint, success bool) {
	action := models.ActionLoginSuccess
	if !success {
		action = models.ActionLoginFailed
	}

	log := models.AuditLog{
		UserID:    userID,
		Username:  username,
		Action:    action,
		Success:   success,
		IPAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}

	s.DB.Create(&log)
}

// LogLogout creates an audit log entry for logout
func (s *AuditService) LogLogout(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	if user == nil {
		return
	}

	log := models.AuditLog{
		UserID:    &user.ID,
		Username:  user.Username,
		Action:    models.ActionLogout,
		Success:   true,
		IPAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}

	s.DB.Create(&log)
}

// AuditFilters contains filter options for querying audit logs
type AuditFilters struct {
	UserID   *uint
	Action   string
	Resource string
	From     string
	To       string
}

// GetLogs retrieves audit logs with optional filters and pagination
func (s *AuditService) GetLogs(filters AuditFilters, page, limit int) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	query := s.DB.Model(&models.AuditLog{})

	if filters.UserID != nil {
		query = query.Where("user_id = ?", *filters.UserID)
	}
	if filters.Action != "" {
		query = query.Where("action = ?", filters.Action)
	}
	if filters.Resource != "" {
		query = query.Where("resource = ?", filters.Resource)
	}
	if filters.From != "" {
		query = query.Where("created_at >= ?", filters.From)
	}
	if filters.To != "" {
		query = query.Where("created_at <= ?", filters.To)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetCurrentUser retrieves the current user from the context
func GetCurrentUser(c *gin.Context) (*models.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}
	u, ok := user.(*models.User)
	return u, ok
}
