package models

import (
	"crypto/rand"
	"strings"
	"time"
)

// RecoveryCodeCount is how many recovery codes a user gets at a time.
const RecoveryCodeCount = 8

// RecoveryCode is a one-time code that replaces a two-factor code, for a
// user who lost their phone. Only its hash is stored, like API keys.
type RecoveryCode struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	CodeHash  string     `gorm:"size:64;not null" json:"-"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// TableName specifies the table name for RecoveryCode.
func (RecoveryCode) TableName() string {
	return "recovery_codes"
}

// recoveryAlphabet is Crockford's base32: 32 symbols, without i, l, o and u,
// which are easy to misread.
const recoveryAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// NewRecoveryCodes returns RecoveryCodeCount random codes, as shown to the
// user ("xxxxx-xxxxx", 50 random bits each), and their stored rows.
func NewRecoveryCodes(userID uint) ([]string, []RecoveryCode, error) {
	codes := make([]string, RecoveryCodeCount)
	rows := make([]RecoveryCode, RecoveryCodeCount)
	buf := make([]byte, 10)
	for i := range codes {
		if _, err := rand.Read(buf); err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		for j, c := range buf {
			if j == 5 {
				b.WriteByte('-')
			}
			b.WriteByte(recoveryAlphabet[c&31])
		}
		codes[i] = b.String()
		rows[i] = RecoveryCode{UserID: userID, CodeHash: HashRecoveryCode(codes[i])}
	}
	return codes, rows, nil
}

// HashRecoveryCode returns the stored form of a recovery code. Case, spaces
// and the dash do not matter, so a code typed by hand still matches.
func HashRecoveryCode(code string) string {
	c := strings.ToLower(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(code)))
	return HashAPIKey(c)
}
