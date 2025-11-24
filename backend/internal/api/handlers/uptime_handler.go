package handlers

import (
	"net/http"
	"strconv"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
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
