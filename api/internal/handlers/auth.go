package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"time"

	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// LoginRequest represents the login credentials submitted by a user
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the response returned after successful authentication
type LoginResponse struct {
	Token               string    `json:"token"`
	ExpiresAt           time.Time `json:"expires_at"`
	User                UserInfo  `json:"user"`
	ForcePasswordChange bool      `json:"force_password_change"`
}

// UserInfo represents basic user information returned after login
type UserInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// Login handles user authentication and JWT token generation
// Validates credentials and returns a JWT token valid for 24 hours
func Login(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest

		// Bind and validate request JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		// Look up user by username
		var user models.User
		if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Log failed login attempt
				auditService.LogLogin(c, req.Username, nil, false)
				// Don't reveal whether username exists (security best practice)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
				return
			}
			// Database error
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
			return
		}

		// Verify password using bcrypt
		if !user.CheckPassword(req.Password) {
			// Log failed login attempt
			auditService.LogLogin(c, req.Username, &user.ID, false)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}

		// Check if user is active
		if !user.Active {
			auditService.LogLogin(c, req.Username, &user.ID, false)
			c.JSON(http.StatusForbidden, gin.H{"error": "User account is deactivated"})
			return
		}

		// Generate JWT token
		token, expiresAt, err := generateToken(user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		// Update last_login
		now := time.Now()
		user.LastLogin = &now
		db.Save(&user)

		// Log successful login
		auditService.LogLogin(c, req.Username, &user.ID, true)

		// Return token and user info
		c.JSON(http.StatusOK, LoginResponse{
			Token:     token,
			ExpiresAt: expiresAt,
			User: UserInfo{
				ID:       user.ID,
				Username: user.Username,
				Email:    user.Email,
				Role:     user.Role,
			},
			ForcePasswordChange: user.ForcePasswordChange,
		})
	}
}

// generateToken creates a JWT token with user_id claim and 24-hour expiration
// Uses HS256 algorithm as required by the authentication middleware
func generateToken(userID uint) (string, time.Time, error) {
	// Token expires in 24 hours
	expiresAt := time.Now().Add(24 * time.Hour)

	// Create JWT claims with user_id and expiration
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
	}

	// Create token with HS256 algorithm (matches middleware validation)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token with secret key from environment
	secretKey := os.Getenv("JWT_SECRET_KEY")
	if secretKey == "" {
		return "", time.Time{}, jwt.ErrInvalidKey
	}

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// ForgotPasswordRequest represents the forgot password request
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// PublicResetPasswordRequest represents the public reset password request (with token)
type PublicResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ForgotPassword handles password reset requests
// Generates a reset token and sends email (always returns success for security)
func ForgotPassword(db *gorm.DB, emailService services.EmailService, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ForgotPasswordRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email address"})
			return
		}

		// Always return success message for security (don't reveal if email exists)
		successMsg := gin.H{"message": "If an account with that email exists, a password reset link has been sent"}

		// Look up user by email
		var user models.User
		if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
			// User not found, but don't reveal this
			c.JSON(http.StatusOK, successMsg)
			return
		}

		// Check if user is active
		if !user.Active {
			// Don't send reset for deactivated accounts
			c.JSON(http.StatusOK, successMsg)
			return
		}

		// Generate reset token (32 bytes = 64 hex characters)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
			return
		}
		resetToken := hex.EncodeToString(tokenBytes)

		// Set expiry to 1 hour from now
		expiry := time.Now().Add(1 * time.Hour)
		user.PasswordResetToken = &resetToken
		user.PasswordResetExpiry = &expiry

		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reset token"})
			return
		}

		// Send reset email (if email service is configured)
		if emailService != nil {
			if err := emailService.SendPasswordResetEmail(user.Email, resetToken); err != nil {
				// Log error but don't reveal to user
				// In production, this should be logged properly
			}
		}

		// Log the password reset request
		auditService.Log(c, "password_reset_request", "users", &user.ID, map[string]interface{}{
			"email": req.Email,
		}, true)

		c.JSON(http.StatusOK, successMsg)
	}
}

// ResetPassword handles password reset with token
func ResetPassword(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req PublicResetPasswordRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Find user by reset token
		var user models.User
		if err := db.Where("password_reset_token = ?", req.Token).First(&user).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
			return
		}

		// Check if token is expired
		if user.PasswordResetExpiry == nil || time.Now().After(*user.PasswordResetExpiry) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
			return
		}

		// Validate password strength
		if err := models.ValidatePasswordStrength(req.Password, user.Username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Hash and save new password
		if err := user.HashPassword(req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
			return
		}

		// Clear reset token and expiry, clear force password change flag
		user.PasswordResetToken = nil
		user.PasswordResetExpiry = nil
		user.ForcePasswordChange = false

		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save password"})
			return
		}

		// Log password reset completion
		auditService.Log(c, "password_reset_completed", "users", &user.ID, map[string]interface{}{
			"username": user.Username,
		}, true)

		c.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully"})
	}
}
