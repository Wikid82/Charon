package handlers

import (
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type LogsHandler struct {
	service *services.LogService
}

var createTempFile = os.CreateTemp

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

	var filter models.LogFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters: " + err.Error()})
		return
	}
	if err := filter.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logs, total, skipped, err := h.service.QueryLogs(filename, filter)
	if err != nil {
		if errors.Is(err, services.ErrInvalidFilename) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read log"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"filename":      filename,
		"logs":          logs,
		"total":         total,
		"limit":         filter.Limit,
		"offset":        filter.Offset,
		"skipped_lines": skipped,
	})
}

func (h *LogsHandler) Download(c *gin.Context) {
	filename := c.Param("filename")
	path, err := h.service.GetLogPath(filename)
	if err != nil {
		if errors.Is(err, services.ErrInvalidFilename) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		return
	}

	// Create a temporary file to serve a consistent snapshot
	// This prevents Content-Length mismatches if the live log file grows during download
	tmpFile, err := createTempFile("", "charon-log-*.log")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			logger.Log().WithError(removeErr).Warn("failed to remove temp file")
		}
	}()

	// #nosec G304 -- path is the symlink-RESOLVED location returned by
	// LogService.GetLogPath, which enforces filepath.Base equality, a raw
	// directory-entry allowlist, and both-sides EvalSymlinks containment
	// inside the configured log directories; opening the resolved path closes
	// the TOCTOU window between validation and open.
	srcFile, err := os.Open(path) //nolint:gosec // nosemgrep: go.gin.path-traversal.gin-path-traversal-taint.gin-path-traversal-taint
	if err != nil {
		if err := tmpFile.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close temp file")
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open log file"})
		return
	}
	defer func() {
		if err := srcFile.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close source log file")
		}
	}()

	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		if err := tmpFile.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close temp file after copy error")
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy log file"})
		return
	}
	if err := tmpFile.Close(); err != nil {
		logger.Log().WithError(err).Warn("failed to close temp file after copy")
	}

	// Explicit text/plain prevents HTML content-sniffing of attacker-influenced
	// log content; FileAttachment emits an RFC 6266 quoted Content-Disposition.
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.FileAttachment(tmpFile.Name(), filename)
}
