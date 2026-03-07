package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

// AuditLogHandler handles audit log API requests.
type AuditLogHandler struct {
	securityService *services.SecurityService
}

// NewAuditLogHandler creates a new audit log handler.
func NewAuditLogHandler(securityService *services.SecurityService) *AuditLogHandler {
	return &AuditLogHandler{
		securityService: securityService,
	}
}

// List handles GET /api/v1/audit-logs
// Returns audit logs with pagination and filtering.
func (h *AuditLogHandler) List(c *gin.Context) {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	// Parse filter parameters
	filter := services.AuditLogFilter{
		Actor:         c.Query("actor"),
		Action:        c.Query("action"),
		EventCategory: c.Query("event_category"),
		ResourceUUID:  c.Query("resource_uuid"),
	}

	// Parse date filters
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			filter.StartDate = &startDate
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			filter.EndDate = &endDate
		}
	}

	// Retrieve audit logs
	audits, total, err := h.securityService.ListAuditLogs(filter, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit logs"})
		return
	}

	// Calculate pagination metadata
	var totalPages int
	if limit > 0 {
		totalPages = (int(total) + limit - 1) / limit
	}

	c.JSON(http.StatusOK, gin.H{
		"audit_logs": audits,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// Get handles GET /api/v1/audit-logs/:uuid
// Returns a single audit log entry.
func (h *AuditLogHandler) Get(c *gin.Context) {
	auditUUID := c.Param("uuid")
	if auditUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Audit UUID is required"})
		return
	}

	audit, err := h.securityService.GetAuditLogByUUID(auditUUID)
	if err != nil {
		if err.Error() == "audit log not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Audit log not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit log"})
		return
	}

	c.JSON(http.StatusOK, audit)
}

// ListByProvider handles GET /api/v1/dns-providers/:id/audit-logs
// Returns audit logs for a specific DNS provider.
func (h *AuditLogHandler) ListByProvider(c *gin.Context) {
	// Parse provider ID
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	// Retrieve audit logs for provider
	audits, total, err := h.securityService.ListAuditLogsByProvider(uint(providerID), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit logs"})
		return
	}

	// Calculate pagination metadata
	var totalPages int
	if limit > 0 {
		totalPages = (int(total) + limit - 1) / limit
	}

	c.JSON(http.StatusOK, gin.H{
		"audit_logs": audits,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}
