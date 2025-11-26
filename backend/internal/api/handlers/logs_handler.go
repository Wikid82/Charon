package handlers

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

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
		Level:  c.Query("level"),
		Limit:  limit,
		Offset: offset,
		Sort:   c.DefaultQuery("sort", "desc"),
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
		if strings.Contains(err.Error(), "invalid filename") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		return
	}

	// Create a temporary file to serve a consistent snapshot
	// This prevents Content-Length mismatches if the live log file grows during download
	tmpFile, err := os.CreateTemp("", "cpmp-log-*.log")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}
	defer os.Remove(tmpFile.Name())

	srcFile, err := os.Open(path)
	if err != nil {
		tmpFile.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open log file"})
		return
	}
	defer srcFile.Close()

	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		tmpFile.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy log file"})
		return
	}
	tmpFile.Close()

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.File(tmpFile.Name())
}
