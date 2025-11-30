package handlers

import (
	"net/http"
	"os"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type BackupHandler struct {
	service *services.BackupService
}

func NewBackupHandler(service *services.BackupService) *BackupHandler {
	return &BackupHandler{service: service}
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
	filename, err := h.service.CreateBackup()
	if err != nil {
		middleware.GetRequestLogger(c).WithField("action", "create_backup").WithError(err).Error("Failed to create backup")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create backup: " + err.Error()})
		return
	}
	middleware.GetRequestLogger(c).WithField("action", "create_backup").WithField("filename", filename).Info("Backup created successfully")
	c.JSON(http.StatusCreated, gin.H{"filename": filename, "message": "Backup created successfully"})
}

func (h *BackupHandler) Delete(c *gin.Context) {
	filename := c.Param("filename")
	if err := h.service.DeleteBackup(filename); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
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
	filename := c.Param("filename")
	if err := h.service.RestoreBackup(filename); err != nil {
		middleware.GetRequestLogger(c).WithField("action", "restore_backup").WithField("filename", filename).WithError(err).Error("Failed to restore backup")
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore backup: " + err.Error()})
		return
	}
	middleware.GetRequestLogger(c).WithField("action", "restore_backup").WithField("filename", filename).Info("Backup restored successfully")
	// In a real scenario, we might want to trigger a restart here
	c.JSON(http.StatusOK, gin.H{"message": "Backup restored successfully. Please restart the container."})
}
