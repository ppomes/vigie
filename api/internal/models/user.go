package models

import (
	"errors"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User roles
const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

// User represents an admin user for authentication
// Supports bcrypt password hashing for secure authentication
type User struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	Username            string     `gorm:"size:255;not null;unique;index" json:"username"`
	Email               string     `gorm:"size:255;not null;unique;index" json:"email"`
	Password            string     `gorm:"size:255;not null" json:"-"` // Hidden from JSON for security
	Role                string     `gorm:"size:50;not null;default:'viewer'" json:"role"`
	Active              bool       `gorm:"default:1" json:"active"`
	LastLogin           *time.Time `json:"last_login,omitempty"`
	PasswordResetToken  *string    `gorm:"size:255;index" json:"-"`
	PasswordResetExpiry *time.Time `json:"-"`
	ForcePasswordChange bool       `gorm:"default:0" json:"force_password_change"`
	// TwoFactorSecret is the base32 TOTP secret once two-factor login is on.
	// It cannot be hashed: the server needs it to compute the codes.
	TwoFactorSecret *string `gorm:"size:64" json:"-"`
	// TwoFactorPendingSecret is the secret shown during setup, until the
	// user confirms a first code.
	TwoFactorPendingSecret *string `gorm:"size:64" json:"-"`
	// TwoFactorLastStep is the time step of the last code accepted, so a
	// code cannot be used twice.
	TwoFactorLastStep int64          `gorm:"not null;default:0" json:"-"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// IsAdmin returns true if the user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// TwoFactorEnabled reports whether login asks for a code.
func (u *User) TwoFactorEnabled() bool {
	return u.TwoFactorSecret != nil && *u.TwoFactorSecret != ""
}

// IsViewer returns true if the user has viewer role
func (u *User) IsViewer() bool {
	return u.Role == RoleViewer
}

// TableName specifies the table name for User model
func (User) TableName() string {
	return "users"
}

// HashPassword hashes the user's password using bcrypt with cost 10
func (u *User) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword compares a plain text password with the hashed password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// ValidatePasswordStrength validates password meets security requirements:
// - Minimum 12 characters
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one digit
// - At least one special character
// - Not equal to username
func ValidatePasswordStrength(password, username string) error {
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters")
	}

	if strings.EqualFold(password, username) {
		return errors.New("password cannot be the same as username")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	specialChars := "!@#$%^&*()_+-=[]{}|;':\",./<>?`~"

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case strings.ContainsRune(specialChars, char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}
