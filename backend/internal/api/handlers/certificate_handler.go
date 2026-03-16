package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// BackupServiceInterface defines the contract for backup service operations
type BackupServiceInterface interface {
	CreateBackup() (string, error)
	ListBackups() ([]services.BackupFile, error)
	DeleteBackup(filename string) error
	GetBackupPath(filename string) (string, error)
	RestoreBackup(filename string) error
	GetAvailableSpace() (int64, error)
}

type CertificateHandler struct {
	service             *services.CertificateService
	backupService       BackupServiceInterface
	notificationService *services.NotificationService
	// Rate limiting for notifications
	notificationMu       sync.Mutex
	lastNotificationTime map[uint]time.Time
}

func NewCertificateHandler(service *services.CertificateService, backupService BackupServiceInterface, ns *services.NotificationService) *CertificateHandler {
	return &CertificateHandler{
		service:              service,
		backupService:        backupService,
		notificationService:  ns,
		lastNotificationTime: make(map[uint]time.Time),
	}
}

func (h *CertificateHandler) List(c *gin.Context) {
	certs, err := h.service.ListCertificates()
	if err != nil {
		logger.Log().WithError(err).Error("failed to list certificates")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list certificates"})
		return
	}

	c.JSON(http.StatusOK, certs)
}

type UploadCertificateRequest struct {
	Name        string `form:"name" binding:"required"`
	Certificate string `form:"certificate"` // PEM content
	PrivateKey  string `form:"private_key"` // PEM content
}

func (h *CertificateHandler) Upload(c *gin.Context) {
	// Handle multipart form
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Read files
	certFile, err := c.FormFile("certificate_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "certificate_file is required"})
		return
	}

	keyFile, err := c.FormFile("key_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key_file is required"})
		return
	}

	// Open and read content
	certSrc, err := certFile.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open cert file"})
		return
	}
	defer func() {
		if errClose := certSrc.Close(); errClose != nil {
			logger.Log().WithError(errClose).Warn("failed to close certificate file")
		}
	}()

	keySrc, err := keyFile.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open key file"})
		return
	}
	defer func() {
		if errClose := keySrc.Close(); errClose != nil {
			logger.Log().WithError(errClose).Warn("failed to close key file")
		}
	}()

	// Read to string
	// Limit size to avoid DoS (e.g. 1MB)
	certBytes := make([]byte, 1024*1024)
	n, _ := certSrc.Read(certBytes)
	certPEM := string(certBytes[:n])

	keyBytes := make([]byte, 1024*1024)
	n, _ = keySrc.Read(keyBytes)
	keyPEM := string(keyBytes[:n])

	cert, err := h.service.UploadCertificate(name, certPEM, keyPEM)
	if err != nil {
		logger.Log().WithError(err).Error("failed to upload certificate")
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to upload certificate"})
		return
	}

	// Send Notification
	if h.notificationService != nil {
		h.notificationService.SendExternal(c.Request.Context(),
			"cert",
			"Certificate Uploaded",
			"A new custom certificate was successfully uploaded.",
			map[string]any{
				"Name":    util.SanitizeForLog(cert.Name),
				"Domains": util.SanitizeForLog(cert.Domains),
				"Action":  "uploaded",
			},
		)
	}

	c.JSON(http.StatusCreated, cert)
}

func (h *CertificateHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// Validate ID range
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// Check if certificate is in use before proceeding
	inUse, err := h.service.IsCertificateInUse(uint(id))
	if err != nil {
		logger.Log().WithError(err).WithField("certificate_id", id).Error("failed to check certificate usage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check certificate usage"})
		return
	}
	if inUse {
		c.JSON(http.StatusConflict, gin.H{"error": "certificate is in use by one or more proxy hosts"})
		return
	}

	// Create backup before deletion
	if h.backupService != nil {
		// Check disk space before backup (require at least 100MB free)
		if availableSpace, err := h.backupService.GetAvailableSpace(); err != nil {
			logger.Log().WithError(err).Warn("unable to check disk space, proceeding with backup")
		} else if availableSpace < 100*1024*1024 {
			logger.Log().WithField("available_bytes", availableSpace).Warn("low disk space, skipping backup")
			c.JSON(http.StatusInsufficientStorage, gin.H{"error": "insufficient disk space for backup"})
			return
		}

		if _, err := h.backupService.CreateBackup(); err != nil {
			logger.Log().WithError(err).Error("failed to create backup before deletion")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup before deletion"})
			return
		}
	}

	// Proceed with deletion
	if err := h.service.DeleteCertificate(uint(id)); err != nil {
		if err == services.ErrCertInUse {
			c.JSON(http.StatusConflict, gin.H{"error": "certificate is in use by one or more proxy hosts"})
			return
		}
		logger.Log().WithError(err).WithField("certificate_id", id).Error("failed to delete certificate")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete certificate"})
		return
	}

	// Send Notification with rate limiting (1 per cert per 10 seconds)
	if h.notificationService != nil {
		h.notificationMu.Lock()
		lastTime, exists := h.lastNotificationTime[uint(id)]
		if !exists || time.Since(lastTime) > 10*time.Second {
			h.lastNotificationTime[uint(id)] = time.Now()
			h.notificationMu.Unlock()
			h.notificationService.SendExternal(c.Request.Context(),
				"cert",
				"Certificate Deleted",
				fmt.Sprintf("Certificate ID %d deleted", id),
				map[string]any{
					"ID":     id,
					"Action": "deleted",
				},
			)
		} else {
			h.notificationMu.Unlock()
			logger.Log().WithField("certificate_id", id).Debug("notification rate limited")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "certificate deleted"})
}
