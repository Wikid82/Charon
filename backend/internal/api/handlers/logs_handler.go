package handlers

import (
	"net/http"
	"strconv"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type LogsHandler struct {
	service *services.LogService
}

func NewLogsHandler(service *services.LogService) *LogsHandler {
	return &LogsHandler{service: service}
}

func (h *LogsHandler) List(c *gin.Context) {
	logs, err := h.service.ListLogs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list logs"})
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (h *LogsHandler) Read(c *gin.Context) {
	filename := c.Param("filename")
	linesStr := c.DefaultQuery("lines", "100")
	lines, _ := strconv.Atoi(linesStr)

	content, err := h.service.ReadLog(filename, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read log"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"filename": filename, "lines": content})
}
