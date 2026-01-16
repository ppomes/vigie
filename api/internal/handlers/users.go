package handlers

import (
	"net/http"
	"strconv"

	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=255"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=12"`
	Role     string `json:"role" binding:"required,oneof=admin viewer"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Email  string `json:"email" binding:"omitempty,email"`
	Role   string `json:"role" binding:"omitempty,oneof=admin viewer"`
	Active *bool  `json:"active"`
}

// ResetPasswordRequest represents the request to reset a user's password
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=12"`
}

// ChangePasswordRequest represents the request to change own password
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=12"`
}

// UserResponse represents user data returned in API responses
type UserResponse struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Active    bool   `json:"active"`
	LastLogin string `json:"last_login,omitempty"`
	CreatedAt string `json:"created_at"`
}

// toUserResponse converts a User model to UserResponse
func toUserResponse(user *models.User) UserResponse {
	resp := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		Active:    user.Active,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if user.LastLogin != nil {
		resp.LastLogin = user.LastLogin.Format("2006-01-02T15:04:05Z")
	}
	return resp
}

// ListUsers returns all users
func ListUsers(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var users []models.User
		if err := db.Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}

		response := make([]UserResponse, len(users))
		for i, user := range users {
			response[i] = toUserResponse(&user)
		}

		auditService.Log(c, models.ActionRead, models.ResourceUser, nil, nil, true)
		c.JSON(http.StatusOK, gin.H{"users": response})
	}
}

// GetUser returns a single user by ID
func GetUser(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		var user models.User
		if err := db.First(&user, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
			return
		}

		resourceID := uint(id)
		auditService.Log(c, models.ActionRead, models.ResourceUser, &resourceID, nil, true)
		c.JSON(http.StatusOK, gin.H{"user": toUserResponse(&user)})
	}
}

// CreateUser creates a new user
func CreateUser(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate password strength
		if err := models.ValidatePasswordStrength(req.Password, req.Username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if username or email already exists
		var existing models.User
		if err := db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
			return
		}

		user := models.User{
			Username: req.Username,
			Email:    req.Email,
			Role:     req.Role,
			Active:   true,
		}

		if err := user.HashPassword(req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		auditService.Log(c, models.ActionCreate, models.ResourceUser, &user.ID, map[string]interface{}{
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		}, true)

		c.JSON(http.StatusCreated, gin.H{"user": toUserResponse(&user)})
	}
}

// UpdateUser updates an existing user
func UpdateUser(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		var user models.User
		if err := db.First(&user, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
			return
		}

		var req UpdateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get current user to prevent self-demotion
		currentUser, _ := services.GetCurrentUser(c)
		if currentUser != nil && currentUser.ID == user.ID {
			// Prevent admin from demoting themselves
			if req.Role != "" && req.Role != models.RoleAdmin && user.IsAdmin() {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot demote yourself from admin"})
				return
			}
			// Prevent admin from deactivating themselves
			if req.Active != nil && !*req.Active {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot deactivate your own account"})
				return
			}
		}

		changes := make(map[string]interface{})

		if req.Email != "" && req.Email != user.Email {
			// Check if email already exists
			var existing models.User
			if err := db.Where("email = ? AND id != ?", req.Email, id).First(&existing).Error; err == nil {
				c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
				return
			}
			changes["email"] = map[string]string{"from": user.Email, "to": req.Email}
			user.Email = req.Email
		}

		if req.Role != "" && req.Role != user.Role {
			changes["role"] = map[string]string{"from": user.Role, "to": req.Role}
			user.Role = req.Role
		}

		if req.Active != nil && *req.Active != user.Active {
			changes["active"] = map[string]bool{"from": user.Active, "to": *req.Active}
			user.Active = *req.Active
		}

		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}

		resourceID := uint(id)
		auditService.Log(c, models.ActionUpdate, models.ResourceUser, &resourceID, changes, true)

		c.JSON(http.StatusOK, gin.H{"user": toUserResponse(&user)})
	}
}

// DeleteUser soft-deletes a user
func DeleteUser(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Prevent deleting yourself
		currentUser, _ := services.GetCurrentUser(c)
		if currentUser != nil && currentUser.ID == uint(id) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete your own account"})
			return
		}

		var user models.User
		if err := db.First(&user, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
			return
		}

		if err := db.Delete(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
			return
		}

		resourceID := uint(id)
		auditService.Log(c, models.ActionDelete, models.ResourceUser, &resourceID, map[string]interface{}{
			"username": user.Username,
		}, true)

		c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
	}
}

// ResetUserPassword resets a user's password (admin only)
func ResetUserPassword(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		var user models.User
		if err := db.First(&user, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
			return
		}

		var req ResetPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate password strength
		if err := models.ValidatePasswordStrength(req.Password, user.Username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := user.HashPassword(req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		// Set force password change flag so user must change on next login
		user.ForcePasswordChange = true

		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
			return
		}

		resourceID := uint(id)
		auditService.Log(c, models.ActionUpdate, models.ResourceUser, &resourceID, map[string]interface{}{
			"action":                "password_reset",
			"force_password_change": true,
		}, true)

		c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
	}
}

// GetCurrentUser returns the current authenticated user's profile
func GetCurrentUserProfile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := services.GetCurrentUser(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"user": toUserResponse(user)})
	}
}

// ChangeOwnPassword allows users to change their own password
func ChangeOwnPassword(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := services.GetCurrentUser(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		var req ChangePasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Verify old password
		if !user.CheckPassword(req.OldPassword) {
			auditService.Log(c, models.ActionUpdate, models.ResourceUser, &user.ID, map[string]interface{}{
				"action": "password_change_failed",
				"reason": "invalid_old_password",
			}, false)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid current password"})
			return
		}

		// Validate password strength
		if err := models.ValidatePasswordStrength(req.NewPassword, user.Username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := user.HashPassword(req.NewPassword); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		// Clear force password change flag
		user.ForcePasswordChange = false

		if err := db.Save(user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
			return
		}

		auditService.Log(c, models.ActionUpdate, models.ResourceUser, &user.ID, map[string]interface{}{
			"action": "password_changed",
		}, true)

		c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
	}
}
