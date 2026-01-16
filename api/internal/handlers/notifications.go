package handlers

import (
	"net/http"
	"strconv"

	"api/internal/services"

	"github.com/gin-gonic/gin"
)

// GetNotificationStats returns queue statistics
// GET /api/v1/admin/notifications/stats
func GetNotificationStats(notificationService *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if notificationService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Notification service is not configured",
			})
			return
		}

		stats, err := notificationService.GetQueueStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to get queue statistics",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"stats": stats,
		})
	}
}

// GetRecentNotifications returns recent notifications
// GET /api/v1/admin/notifications/recent?limit=50
func GetRecentNotifications(notificationService *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if notificationService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Notification service is not configured",
			})
			return
		}

		limit := 50
		if l := c.Query("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
				limit = parsed
			}
		}

		notifications, err := notificationService.GetRecentNotifications(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to get recent notifications",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"notifications": notifications,
		})
	}
}

// RetryFailedNotifications resets failed notifications to pending
// POST /api/v1/admin/notifications/retry
func RetryFailedNotifications(notificationService *services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if notificationService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Notification service is not configured",
			})
			return
		}

		count, err := notificationService.RetryFailedNotifications()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retry notifications",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Failed notifications reset for retry",
			"count":   count,
		})
	}
}
