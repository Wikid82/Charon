package handlers

import (
	"net/http"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type UptimeHandler struct {
	service *services.UptimeService
}

func NewUptimeHandler(service *services.UptimeService) *UptimeHandler {
	return &UptimeHandler{service: service}
}

func (h *UptimeHandler) List(c *gin.Context) {
	monitors, err := h.service.ListMonitors()
	if err != nil {
		logger.Log().WithError(err).Error("Failed to list uptime monitors")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list monitors"})
		return
	}
	c.JSON(http.StatusOK, monitors)
}

// CreateMonitorRequest represents the JSON payload for creating a new monitor
type CreateMonitorRequest struct {
	Name       string `json:"name" binding:"required"`
	URL        string `json:"url" binding:"required"`
	Type       string `json:"type" binding:"required,oneof=http tcp https"`
	Interval   int    `json:"interval"`
	MaxRetries int    `json:"max_retries"`
}

// Create creates a new uptime monitor
func (h *UptimeHandler) Create(c *gin.Context) {
	var req CreateMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log().WithError(err).Warn("Invalid JSON payload for monitor creation")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	monitor, err := h.service.CreateMonitor(req.Name, req.URL, req.Type, req.Interval, req.MaxRetries)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to create uptime monitor")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, monitor)
}

func (h *UptimeHandler) GetHistory(c *gin.Context) {
	id := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	history, err := h.service.GetMonitorHistory(id, limit)
	if err != nil {
		logger.Log().WithError(err).WithField("monitor_id", id).Error("Failed to get monitor history")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get history"})
		return
	}
	c.JSON(http.StatusOK, history)
}

func (h *UptimeHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		logger.Log().WithError(err).WithField("monitor_id", id).Warn("Invalid JSON payload for monitor update")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	monitor, err := h.service.UpdateMonitor(id, updates)
	if err != nil {
		logger.Log().WithError(err).WithField("monitor_id", id).Error("Failed to update monitor")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, monitor)
}

func (h *UptimeHandler) Sync(c *gin.Context) {
	if err := h.service.SyncMonitors(); err != nil {
		logger.Log().WithError(err).Error("Failed to sync uptime monitors")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync monitors"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sync started"})
}

// Delete removes a monitor and its associated data
func (h *UptimeHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteMonitor(id); err != nil {
		logger.Log().WithError(err).WithField("monitor_id", id).Error("Failed to delete monitor")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete monitor"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Monitor deleted"})
}

// CheckMonitor triggers an immediate check for a specific monitor
func (h *UptimeHandler) CheckMonitor(c *gin.Context) {
	id := c.Param("id")
	monitor, err := h.service.GetMonitorByID(id)
	if err != nil {
		logger.Log().WithError(err).WithField("monitor_id", id).Warn("Monitor not found for check")
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor not found"})
		return
	}

	// Trigger immediate check in background
	go h.service.CheckMonitor(*monitor)

	c.JSON(http.StatusOK, gin.H{"message": "Check triggered"})
}
