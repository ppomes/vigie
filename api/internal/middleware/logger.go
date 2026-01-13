package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger is a custom logging middleware that provides detailed request logging
// Note: gin.Default() already includes basic logging, this provides additional detail
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate request duration
		duration := time.Since(start)

		// Build log message
		if raw != "" {
			path = path + "?" + raw
		}

		// Log request details
		log.Printf(
			"[API] %s | %3d | %13v | %15s | %-7s %s",
			start.Format("2006/01/02 15:04:05"),
			c.Writer.Status(),
			duration,
			c.ClientIP(),
			c.Request.Method,
			path,
		)

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				log.Printf("[ERROR] %s", e.Error())
			}
		}
	}
}
