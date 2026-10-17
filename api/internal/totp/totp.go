// Package totp implements time-based one-time passwords (RFC 6238), the
// six-digit codes shown by authenticator apps. It uses the settings every
// app supports and that Cachet used: HMAC-SHA1, 30-second steps, 6 digits.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	period = 30
	digits = 6
	// skew is how many steps before and after the current one are
	// accepted, for clocks that drift a little.
	skew = 1
)

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// ErrInvalidSecret is returned for a secret that is not base32.
var ErrInvalidSecret = errors.New("totp: secret is not valid base32")

// NewSecret returns a random 160-bit secret, base32 encoded.
func NewSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return encoding.EncodeToString(b), nil
}

// NormalizeSecret uppercases a base32 secret and removes spaces and
// padding, then checks that it decodes. Secrets made by other tools, such as
// Cachet's, go through it before they are stored.
func NormalizeSecret(secret string) (string, error) {
	s := strings.ToUpper(strings.NewReplacer(" ", "", "-", "", "=", "").Replace(strings.TrimSpace(secret)))
	key, err := encoding.DecodeString(s)
	if err != nil || len(key) < 10 {
		return "", ErrInvalidSecret
	}
	return s, nil
}

// Step returns the time step of t.
func Step(t time.Time) int64 {
	return t.Unix() / period
}

// Code returns the code of secret for the given time step.
func Code(secret string, step int64) (string, error) {
	key, err := encoding.DecodeString(secret)
	if err != nil {
		return "", ErrInvalidSecret
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", digits, n%1_000_000), nil
}

// Validate checks code against secret at time t, and returns the step it
// matched. Steps up to lastStep are refused, so a code cannot be used twice:
// callers store the returned step as the new lastStep.
func Validate(secret, code string, t time.Time, lastStep int64) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != digits {
		return 0, false
	}
	now := Step(t)
	for s := now - skew; s <= now+skew; s++ {
		if s <= lastStep {
			continue
		}
		want, err := Code(secret, s)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

// URL returns the otpauth:// address authenticator apps read from a QR
// code. issuer is the name the app shows, account the user's name.
func URL(issuer, account, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(digits))
	q.Set("period", fmt.Sprint(period))
	return "otpauth://totp/" + label + "?" + q.Encode()
}
