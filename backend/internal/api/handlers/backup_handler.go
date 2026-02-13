package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BackupHandler struct {
	service         *services.BackupService
	securityService *services.SecurityService
	db              *gorm.DB
}

func NewBackupHandler(service *services.BackupService) *BackupHandler {
	return NewBackupHandlerWithDeps(service, nil, nil)
}

func NewBackupHandlerWithDeps(service *services.BackupService, securityService *services.SecurityService, db *gorm.DB) *BackupHandler {
	return &BackupHandler{service: service, securityService: securityService, db: db}
}

func (h *BackupHandler) List(c *gin.Context) {
	backups, err := h.service.ListBackups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list backups"})
		return
	}
	c.JSON(http.StatusOK, backups)
}

func (h *BackupHandler) Create(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	filename, err := h.service.CreateBackup()
	if err != nil {
		middleware.GetRequestLogger(c).WithField("action", "create_backup").WithError(err).Error("Failed to create backup")
		if respondPermissionError(c, h.securityService, "backup_create_failed", err, h.service.BackupDir) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create backup: " + err.Error()})
		return
	}
	middleware.GetRequestLogger(c).WithField("action", "create_backup").WithField("filename", util.SanitizeForLog(filepath.Base(filename))).Info("Backup created successfully")
	c.JSON(http.StatusCreated, gin.H{"filename": filename, "message": "Backup created successfully"})
}

func (h *BackupHandler) Delete(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	filename := c.Param("filename")
	if err := h.service.DeleteBackup(filename); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
			return
		}
		if respondPermissionError(c, h.securityService, "backup_delete_failed", err, h.service.BackupDir) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete backup"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Backup deleted"})
}

func (h *BackupHandler) Download(c *gin.Context) {
	filename := c.Param("filename")
	path, err := h.service.GetBackupPath(filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.File(path)
}

func (h *BackupHandler) Restore(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	filename := c.Param("filename")
	if err := h.service.RestoreBackup(filename); err != nil {
		// codeql[go/log-injection] Safe: User input sanitized via util.SanitizeForLog()
		// which removes control characters (0x00-0x1F, 0x7F) including CRLF
		middleware.GetRequestLogger(c).WithField("action", "restore_backup").WithField("filename", util.SanitizeForLog(filepath.Base(filename))).WithError(err).Error("Failed to restore backup")
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
			return
		}
		if respondPermissionError(c, h.securityService, "backup_restore_failed", err, h.service.BackupDir) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore backup: " + err.Error()})
		return
	}
	middleware.GetRequestLogger(c).WithField("action", "restore_backup").WithField("filename", util.SanitizeForLog(filepath.Base(filename))).Info("Backup restored successfully")

	restartRequired := true
	rehydrated := false

	if h.db != nil {
		var rehydrateErr error
		for attempt := 0; attempt < 5; attempt++ {
			rehydrateErr = h.service.RehydrateLiveDatabase(h.db)
			if rehydrateErr == nil {
				break
			}

			if !isSQLiteTransientRehydrateError(rehydrateErr) || attempt == 4 {
				break
			}

			time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
		}

		if rehydrateErr != nil {
			middleware.GetRequestLogger(c).WithField("action", "restore_backup_rehydrate").WithError(rehydrateErr).Warn("Backup restored but live database rehydrate failed")
		} else {
			restartRequired = false
			rehydrated = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                "Backup restored successfully",
		"restart_required":       restartRequired,
		"live_rehydrate_applied": rehydrated,
	})
}

func isSQLiteTransientRehydrateError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database is busy") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "table is locked") ||
		strings.Contains(message, "resource busy")
}
