package handlers

import (
	"net/http"
	"os"
	"time"

	"api/internal/models"

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
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo represents basic user information returned after login
type UserInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Login handles user authentication and JWT token generation
// Validates credentials and returns a JWT token valid for 24 hours
func Login(db *gorm.DB) gin.HandlerFunc {
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}

		// Generate JWT token
		token, expiresAt, err := generateToken(user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		// Return token and user info
		c.JSON(http.StatusOK, LoginResponse{
			Token:     token,
			ExpiresAt: expiresAt,
			User: UserInfo{
				ID:       user.ID,
				Username: user.Username,
				Email:    user.Email,
			},
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
