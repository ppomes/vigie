package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"api/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// AuthRequired is a middleware that validates JWT tokens in the Authorization header
// It ensures only authenticated admin users can access protected routes
// Deprecated: Use AuthRequiredWithDB for full user loading
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>" format
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			// Bearer prefix was not found
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// CRITICAL: Validate algorithm to prevent algorithm confusion attacks
			// Only accept HS256 tokens
			if token.Method.Alg() != "HS256" {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(os.Getenv("JWT_SECRET_KEY")), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Extract claims and set user context for downstream handlers
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Set user_id in context for downstream handlers to access
		if userID, exists := claims["user_id"]; exists {
			c.Set("user_id", userID)
		}

		c.Next()
	}
}

// Authentication methods, stored in the context under AuthMethodKey.
const (
	AuthMethodKey    = "auth_method"
	AuthMethodJWT    = "jwt"
	AuthMethodAPIKey = "api_key"
)

// CachetTokenHeader is the header Cachet clients use to send their API key.
const CachetTokenHeader = "X-Cachet-Token"

// apiKeyTouchInterval limits how often last_used_at is written for a key.
const apiKeyTouchInterval = time.Minute

type authError struct {
	status  int
	message string
}

// AuthRequiredWithDB authenticates the request and loads the full user.
//
// It accepts either a session JWT, from the admin login, or a permanent API
// key, sent as "Authorization: Bearer <key>" or in the Cachet header
// X-Cachet-Token. An API key acts with the role of its user. The user must
// exist and be active in both cases.
func AuthRequiredWithDB(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, method, aerr := authenticate(c, db)
		if aerr != nil {
			c.JSON(aerr.status, gin.H{"error": aerr.message})
			c.Abort()
			return
		}

		if !user.Active {
			c.JSON(http.StatusForbidden, gin.H{"error": "User account is deactivated"})
			c.Abort()
			return
		}

		c.Set("user_id", float64(user.ID))
		c.Set("user", user)
		c.Set(AuthMethodKey, method)

		c.Next()
	}
}

// RequireSession rejects requests authenticated with an API key. It guards
// actions a machine credential should not perform, such as creating more keys.
func RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString(AuthMethodKey) != AuthMethodJWT {
			c.JSON(http.StatusForbidden, gin.H{"error": "This action requires an interactive login, not an API key"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func authenticate(c *gin.Context, db *gorm.DB) (*models.User, string, *authError) {
	if key := c.GetHeader(CachetTokenHeader); key != "" {
		user, aerr := authenticateAPIKey(db, key)
		return user, AuthMethodAPIKey, aerr
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil, "", &authError{http.StatusUnauthorized, "Authorization header required"}
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader || tokenString == "" {
		return nil, "", &authError{http.StatusUnauthorized, "Invalid authorization format"}
	}

	// A JWT has three dot-separated parts; API keys contain no dot.
	if strings.Count(tokenString, ".") != 2 {
		user, aerr := authenticateAPIKey(db, tokenString)
		return user, AuthMethodAPIKey, aerr
	}
	user, aerr := authenticateJWT(db, tokenString)
	return user, AuthMethodJWT, aerr
}

func authenticateJWT(db *gorm.DB, tokenString string) (*models.User, *authError) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != "HS256" {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(os.Getenv("JWT_SECRET_KEY")), nil
	})
	if err != nil || !token.Valid {
		return nil, &authError{http.StatusUnauthorized, "Invalid token"}
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, &authError{http.StatusUnauthorized, "Invalid token claims"}
	}
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return nil, &authError{http.StatusUnauthorized, "User ID not found in token"}
	}

	var user models.User
	if err := db.First(&user, uint(userIDFloat)).Error; err != nil {
		return nil, &authError{http.StatusUnauthorized, "User not found"}
	}
	return &user, nil
}

func authenticateAPIKey(db *gorm.DB, raw string) (*models.User, *authError) {
	var key models.APIKey
	err := db.Where("key_hash = ? AND revoked_at IS NULL", models.HashAPIKey(raw)).First(&key).Error
	if err != nil {
		return nil, &authError{http.StatusUnauthorized, "Invalid API key"}
	}

	var user models.User
	if err := db.First(&user, key.UserID).Error; err != nil {
		return nil, &authError{http.StatusUnauthorized, "Invalid API key"}
	}

	// Best effort: a failure to record usage must not reject the request.
	now := time.Now()
	if key.LastUsedAt == nil || now.Sub(*key.LastUsedAt) > apiKeyTouchInterval {
		db.Model(&key).UpdateColumn("last_used_at", now)
	}
	return &user, nil
}
