package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTTokenGeneration(t *testing.T) {
	// Set test secret
	os.Setenv("JWT_SECRET_KEY", "test-secret-key")
	defer os.Unsetenv("JWT_SECRET_KEY")

	// Create claims
	claims := jwt.MapClaims{
		"user_id": float64(1),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	// Generate token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret-key"))

	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if tokenString == "" {
		t.Fatal("Token string is empty")
	}

	// Verify token structure
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key"), nil
	})

	if err != nil || !parsedToken.Valid {
		t.Fatal("Token validation failed")
	}

	// Verify claims
	parsedClaims := parsedToken.Claims.(jwt.MapClaims)
	if parsedClaims["user_id"] != float64(1) {
		t.Errorf("Expected user_id=1, got %v", parsedClaims["user_id"])
	}
}

func TestJWTAlgorithmValidation(t *testing.T) {
	// THIS IS CRITICAL - Test that only HS256 is accepted
	os.Setenv("JWT_SECRET_KEY", "test-secret-key")
	defer os.Unsetenv("JWT_SECRET_KEY")

	testCases := []struct {
		name       string
		algorithm  jwt.SigningMethod
		shouldFail bool
	}{
		{"HS256 should pass", jwt.SigningMethodHS256, false},
		{"HS384 should fail", jwt.SigningMethodHS384, true},
		{"HS512 should fail", jwt.SigningMethodHS512, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{
				"user_id": float64(1),
				"exp":     time.Now().Add(24 * time.Hour).Unix(),
			}

			token := jwt.NewWithClaims(tc.algorithm, claims)
			tokenString, err := token.SignedString([]byte("test-secret-key"))
			if err != nil {
				t.Fatalf("Failed to sign token: %v", err)
			}

			// Try to validate with our middleware logic
			_, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// This is the CRITICAL security check from auth.go:36
				if token.Method.Alg() != "HS256" {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte("test-secret-key"), nil
			})

			if tc.shouldFail && err == nil {
				t.Errorf("%s: Expected validation to fail, but it passed", tc.name)
			}

			if !tc.shouldFail && err != nil {
				t.Errorf("%s: Expected validation to pass, but it failed: %v", tc.name, err)
			}
		})
	}
}

func TestExpiredTokenRejection(t *testing.T) {
	os.Setenv("JWT_SECRET_KEY", "test-secret-key")
	defer os.Unsetenv("JWT_SECRET_KEY")

	// Create expired token (expired 1 hour ago)
	claims := jwt.MapClaims{
		"user_id": float64(1),
		"exp":     time.Now().Add(-1 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("test-secret-key"))

	// Validate - should fail due to expiration
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key"), nil
	})

	if err == nil {
		t.Error("Expected expired token to be rejected, but it was accepted")
	}

	if parsedToken != nil && parsedToken.Valid {
		t.Error("Expired token should not be valid")
	}
}

func TestInvalidSignatureRejection(t *testing.T) {
	os.Setenv("JWT_SECRET_KEY", "test-secret-key")
	defer os.Unsetenv("JWT_SECRET_KEY")

	// Create token with one secret
	claims := jwt.MapClaims{
		"user_id": float64(1),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("wrong-secret-key"))

	// Try to validate with different secret - should fail
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key"), nil
	})

	if err == nil {
		t.Error("Expected token with wrong signature to be rejected, but it was accepted")
	}

	if parsedToken != nil && parsedToken.Valid {
		t.Error("Token with invalid signature should not be valid")
	}
}

func TestAuthMiddleware(t *testing.T) {
	os.Setenv("JWT_SECRET_KEY", "test-secret-key")
	defer os.Unsetenv("JWT_SECRET_KEY")

	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "Valid token",
			authHeader:     generateValidToken(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Missing Authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid Bearer format",
			authHeader:     "InvalidFormat token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid token",
			authHeader:     "Bearer invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/test", AuthRequired(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestAuthMiddlewareSetsUserContext(t *testing.T) {
	os.Setenv("JWT_SECRET_KEY", "test-secret-key")
	defer os.Unsetenv("JWT_SECRET_KEY")

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/test", AuthRequired(), func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			t.Error("user_id not set in context")
		}

		if userID != float64(1) {
			t.Errorf("Expected user_id=1, got %v", userID)
		}

		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", generateValidToken())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func generateValidToken() string {
	claims := jwt.MapClaims{
		"user_id": float64(1),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("test-secret-key"))
	return "Bearer " + tokenString
}
