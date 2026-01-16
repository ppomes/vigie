package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"

	"api/internal/services"

	"github.com/gin-gonic/gin"
)

// ListAuditLogs returns audit logs with optional filtering and pagination
func ListAuditLogs(auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse filters
		filters := services.AuditFilters{
			Action:   c.Query("action"),
			Resource: c.Query("resource"),
			From:     c.Query("from"),
			To:       c.Query("to"),
		}

		if userIDStr := c.Query("user_id"); userIDStr != "" {
			if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
				uid := uint(userID)
				filters.UserID = &uid
			}
		}

		// Parse pagination
		page := 1
		limit := 50

		if pageStr := c.Query("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		logs, total, err := auditService.GetLogs(filters, page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit logs"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"logs":  logs,
			"total": total,
			"page":  page,
			"limit": limit,
		})
	}
}

// ExportAuditLogs exports audit logs as CSV
func ExportAuditLogs(auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse filters (same as list)
		filters := services.AuditFilters{
			Action:   c.Query("action"),
			Resource: c.Query("resource"),
			From:     c.Query("from"),
			To:       c.Query("to"),
		}

		if userIDStr := c.Query("user_id"); userIDStr != "" {
			if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
				uid := uint(userID)
				filters.UserID = &uid
			}
		}

		// Get all logs (up to 10000)
		logs, _, err := auditService.GetLogs(filters, 1, 10000)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit logs"})
			return
		}

		// Set headers for CSV download
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=audit_logs.csv")

		writer := csv.NewWriter(c.Writer)

		// Write header
		writer.Write([]string{
			"ID", "Timestamp", "Username", "User ID", "Action", "Resource",
			"Resource ID", "IP Address", "Success", "Details",
		})

		// Write data
		for _, log := range logs {
			userIDStr := ""
			if log.UserID != nil {
				userIDStr = strconv.FormatUint(uint64(*log.UserID), 10)
			}

			resourceIDStr := ""
			if log.ResourceID != nil {
				resourceIDStr = strconv.FormatUint(uint64(*log.ResourceID), 10)
			}

			writer.Write([]string{
				strconv.FormatUint(uint64(log.ID), 10),
				log.CreatedAt.Format("2006-01-02 15:04:05"),
				log.Username,
				userIDStr,
				log.Action,
				log.Resource,
				resourceIDStr,
				log.IPAddress,
				fmt.Sprintf("%t", log.Success),
				log.Details,
			})
		}

		writer.Flush()
	}
}
