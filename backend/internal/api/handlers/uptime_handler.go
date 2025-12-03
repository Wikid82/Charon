package handlers

import (
	"net/http"
	"strconv"

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list monitors"})
		return
	}
	c.JSON(http.StatusOK, monitors)
}

func (h *UptimeHandler) GetHistory(c *gin.Context) {
	id := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	history, err := h.service.GetMonitorHistory(id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get history"})
		return
	}
	c.JSON(http.StatusOK, history)
}

func (h *UptimeHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	monitor, err := h.service.UpdateMonitor(id, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, monitor)
}

func (h *UptimeHandler) Sync(c *gin.Context) {
	if err := h.service.SyncMonitors(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync monitors"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sync started"})
}

// Delete removes a monitor and its associated data
func (h *UptimeHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteMonitor(id); err != nil {
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor not found"})
		return
	}

	// Trigger immediate check in background
	go h.service.CheckMonitor(*monitor)

	c.JSON(http.StatusOK, gin.H{"message": "Check triggered"})
}
