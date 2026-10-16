package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// APIKey is a permanent credential for machine access, such as a monitoring
// tool pushing component statuses. It acts with the role of its user.
//
// Only a SHA-256 hash of the key is stored. Keys are long random strings, so a
// fast hash is enough; the raw key is shown once, when it is created.
type APIKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Name       string     `gorm:"size:255;not null" json:"name"`
	KeyHash    string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Prefix     string     `gorm:"size:16" json:"prefix"` // start of the key, to tell keys apart
	UserID     uint       `gorm:"not null;index" json:"user_id"`
	User       User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `gorm:"index" json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// APIKeyPrefix is the start of every generated key.
const APIKeyPrefix = "vigie_"

// HashAPIKey returns the stored form of a raw API key.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// GenerateAPIKey returns a new random API key.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return APIKeyPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// NewAPIKey builds an API key record for a raw key. It is also used to import
// existing keys, such as Cachet ones, which do not start with APIKeyPrefix.
func NewAPIKey(name, raw string, userID uint) APIKey {
	prefix := raw
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return APIKey{Name: name, KeyHash: HashAPIKey(raw), Prefix: prefix, UserID: userID}
}
