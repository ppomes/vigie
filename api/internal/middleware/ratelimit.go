package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter provides in-memory rate limiting per IP address
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string]*requestInfo
	limit    int           // max requests
	window   time.Duration // time window
}

type requestInfo struct {
	count     int
	firstSeen time.Time
}

// NewRateLimiter creates a new rate limiter
// limit: maximum number of requests allowed in the window
// window: time duration for the rate limit window
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*requestInfo),
		limit:    limit,
		window:   window,
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return rl
}

// cleanup removes expired entries periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, info := range rl.requests {
			if now.Sub(info.firstSeen) > rl.window {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// isAllowed checks if the IP is within rate limits
func (rl *RateLimiter) isAllowed(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	info, exists := rl.requests[ip]

	if !exists {
		rl.requests[ip] = &requestInfo{
			count:     1,
			firstSeen: now,
		}
		return true
	}

	// Reset if window has passed
	if now.Sub(info.firstSeen) > rl.window {
		info.count = 1
		info.firstSeen = now
		return true
	}

	// Check limit
	if info.count >= rl.limit {
		return false
	}

	info.count++
	return true
}

// Exceeded reports whether key has used up its limit, without counting
// anything.
func (rl *RateLimiter) Exceeded(key string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	info, ok := rl.requests[key]
	return ok && time.Since(info.firstSeen) <= rl.window && info.count >= rl.limit
}

// Record counts one event for key, such as a failed attempt.
func (rl *RateLimiter) Record(key string) {
	rl.isAllowed(key)
}

// Middleware returns a gin middleware that enforces rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !rl.isAllowed(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Global rate limiters for different endpoints
var (
	// LoginLimiter: 10 requests per 15 minutes per IP. A two-factor login
	// takes two requests (password, then code), so this is still about five
	// attempts.
	LoginLimiter = NewRateLimiter(10, 15*time.Minute)

	// TwoFactorLimiter: 5 wrong two-factor codes per 15 minutes per user.
	// The login limiter is per IP; this one bounds the guesses at one
	// account's codes however many addresses they come from.
	TwoFactorLimiter = NewRateLimiter(5, 15*time.Minute)

	// PasswordResetLimiter: 3 requests per hour per IP
	PasswordResetLimiter = NewRateLimiter(3, time.Hour)

	// SubscribeLimiter: 10 requests per hour per IP
	SubscribeLimiter = NewRateLimiter(10, time.Hour)

	// EmailTestLimiter: 5 requests per hour per IP
	EmailTestLimiter = NewRateLimiter(5, time.Hour)
)
