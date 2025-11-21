package handlers

import (
	"net/http"
	"os"
	"strconv"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
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

	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := models.LogFilter{
		Search: c.Query("search"),
		Host:   c.Query("host"),
		Status: c.Query("status"),
		Limit:  limit,
		Offset: offset,
	}

	logs, total, err := h.service.QueryLogs(filename, filter)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read log"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"filename": filename,
		"logs":     logs,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *LogsHandler) Download(c *gin.Context) {
	filename := c.Param("filename")
	path, err := h.service.GetLogPath(filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		return
	}

	c.File(path)
}
