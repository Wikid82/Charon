package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
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
	db                  *gorm.DB
	// Rate limiting for notifications
	notificationMu       sync.Mutex
	lastNotificationTime map[string]time.Time
}

func NewCertificateHandler(service *services.CertificateService, backupService BackupServiceInterface, ns *services.NotificationService) *CertificateHandler {
	return &CertificateHandler{
		service:              service,
		backupService:        backupService,
		notificationService:  ns,
		lastNotificationTime: make(map[string]time.Time),
	}
}

// SetDB sets the database connection for user lookups (export re-auth).
func (h *CertificateHandler) SetDB(db *gorm.DB) {
	h.db = db
}

// maxFileSize is 1MB for certificate file uploads.
const maxFileSize = 1 << 20

func (h *CertificateHandler) List(c *gin.Context) {
	certs, err := h.service.ListCertificates()
	if err != nil {
		logger.Log().WithError(err).Error("failed to list certificates")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list certificates"})
		return
	}

	c.JSON(http.StatusOK, certs)
}

func (h *CertificateHandler) Get(c *gin.Context) {
	certUUID := c.Param("uuid")
	if certUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	detail, err := h.service.GetCertificate(certUUID)
	if err != nil {
		if err == services.ErrCertNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
			return
		}
		logger.Log().WithError(err).Error("failed to get certificate")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get certificate"})
		return
	}

	c.JSON(http.StatusOK, detail)
}

func (h *CertificateHandler) Upload(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Read certificate file
	certFile, err := c.FormFile("certificate_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "certificate_file is required"})
		return
	}

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

	certBytes, err := io.ReadAll(io.LimitReader(certSrc, maxFileSize))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read certificate file"})
		return
	}
	certPEM := string(certBytes)

	// Read private key file (optional for PFX)
	var keyPEM string
	keyFile, err := c.FormFile("key_file")
	if err == nil {
		keySrc, errOpen := keyFile.Open()
		if errOpen != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open key file"})
			return
		}
		defer func() {
			if errClose := keySrc.Close(); errClose != nil {
				logger.Log().WithError(errClose).Warn("failed to close key file")
			}
		}()

		keyBytes, errRead := io.ReadAll(io.LimitReader(keySrc, maxFileSize))
		if errRead != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read key file"})
			return
		}
		keyPEM = string(keyBytes)
	} else if !strings.HasSuffix(strings.ToLower(certFile.Filename), ".pfx") &&
		!strings.HasSuffix(strings.ToLower(certFile.Filename), ".p12") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key_file is required for PEM certificates"})
		return
	}

	// Read chain file (optional)
	var chainPEM string
	chainFile, err := c.FormFile("chain_file")
	if err == nil {
		chainSrc, errOpen := chainFile.Open()
		if errOpen != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open chain file"})
			return
		}
		defer func() {
			if errClose := chainSrc.Close(); errClose != nil {
				logger.Log().WithError(errClose).Warn("failed to close chain file")
			}
		}()

		chainBytes, errRead := io.ReadAll(io.LimitReader(chainSrc, maxFileSize))
		if errRead != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read chain file"})
			return
		}
		chainPEM = string(chainBytes)
	}

	cert, err := h.service.UploadCertificate(name, certPEM, keyPEM, chainPEM)
	if err != nil {
		logger.Log().WithError(err).Error("failed to upload certificate")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

type updateCertificateRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *CertificateHandler) Update(c *gin.Context) {
	certUUID := c.Param("uuid")
	if certUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	var req updateCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	info, err := h.service.UpdateCertificate(certUUID, req.Name)
	if err != nil {
		if err == services.ErrCertNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
			return
		}
		logger.Log().WithError(err).Error("failed to update certificate")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update certificate"})
		return
	}

	c.JSON(http.StatusOK, info)
}

func (h *CertificateHandler) Validate(c *gin.Context) {
	// Read certificate file
	certFile, err := c.FormFile("certificate_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "certificate_file is required"})
		return
	}

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

	certBytes, err := io.ReadAll(io.LimitReader(certSrc, maxFileSize))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read certificate file"})
		return
	}

	// Read optional key file
	var keyPEM string
	keyFile, err := c.FormFile("key_file")
	if err == nil {
		keySrc, errOpen := keyFile.Open()
		if errOpen != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open key file"})
			return
		}
		defer func() {
			if errClose := keySrc.Close(); errClose != nil {
				logger.Log().WithError(errClose).Warn("failed to close key file")
			}
		}()

		keyBytes, errRead := io.ReadAll(io.LimitReader(keySrc, maxFileSize))
		if errRead != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read key file"})
			return
		}
		keyPEM = string(keyBytes)
	}

	// Read optional chain file
	var chainPEM string
	chainFile, err := c.FormFile("chain_file")
	if err == nil {
		chainSrc, errOpen := chainFile.Open()
		if errOpen != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open chain file"})
			return
		}
		defer func() {
			if errClose := chainSrc.Close(); errClose != nil {
				logger.Log().WithError(errClose).Warn("failed to close chain file")
			}
		}()

		chainBytes, errRead := io.ReadAll(io.LimitReader(chainSrc, maxFileSize))
		if errRead != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read chain file"})
			return
		}
		chainPEM = string(chainBytes)
	}

	result, err := h.service.ValidateCertificate(string(certBytes), keyPEM, chainPEM)
	if err != nil {
		logger.Log().WithError(err).Error("failed to validate certificate")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "validation failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

type exportCertificateRequest struct {
	Format      string `json:"format" binding:"required"`
	IncludeKey  bool   `json:"include_key"`
	PFXPassword string `json:"pfx_password"`
	Password    string `json:"password"`
}

func (h *CertificateHandler) Export(c *gin.Context) {
	certUUID := c.Param("uuid")
	if certUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	var req exportCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format is required"})
		return
	}

	// Re-authenticate when requesting private key
	if req.IncludeKey {
		if req.Password == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "password required to export private key"})
			return
		}

		userVal, exists := c.Get("user")
		if !exists || h.db == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "authentication required"})
			return
		}

		userMap, ok := userVal.(map[string]any)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid session"})
			return
		}

		userID, ok := userMap["id"]
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid session"})
			return
		}

		var user models.User
		if err := h.db.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "user not found"})
			return
		}

		if !user.CheckPassword(req.Password) {
			c.JSON(http.StatusForbidden, gin.H{"error": "incorrect password"})
			return
		}
	}

	data, filename, err := h.service.ExportCertificate(certUUID, req.Format, req.IncludeKey)
	if err != nil {
		if err == services.ErrCertNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
			return
		}
		logger.Log().WithError(err).Error("failed to export certificate")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export certificate"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func (h *CertificateHandler) Delete(c *gin.Context) {
	idStr := c.Param("uuid")

	// Support both numeric ID (legacy) and UUID
	if numID, err := strconv.ParseUint(idStr, 10, 32); err == nil && numID > 0 {
		inUse, err := h.service.IsCertificateInUse(uint(numID))
		if err != nil {
			logger.Log().WithError(err).WithField("certificate_id", numID).Error("failed to check certificate usage")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check certificate usage"})
			return
		}
		if inUse {
			c.JSON(http.StatusConflict, gin.H{"error": "certificate is in use by one or more proxy hosts"})
			return
		}

		if h.backupService != nil {
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

		if err := h.service.DeleteCertificateByID(uint(numID)); err != nil {
			if err == services.ErrCertInUse {
				c.JSON(http.StatusConflict, gin.H{"error": "certificate is in use by one or more proxy hosts"})
				return
			}
			logger.Log().WithError(err).WithField("certificate_id", numID).Error("failed to delete certificate")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete certificate"})
			return
		}

		h.sendDeleteNotification(c, fmt.Sprintf("%d", numID))
		c.JSON(http.StatusOK, gin.H{"message": "certificate deleted"})
		return
	}

	// UUID path - value isn't numeric, validate it looks like a UUID
	if idStr == "" || idStr == "0" || len(idStr) < 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	inUse, err := h.service.IsCertificateInUseByUUID(idStr)
	if err != nil {
		if err == services.ErrCertNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
			return
		}
		logger.Log().WithError(err).WithField("certificate_uuid", idStr).Error("failed to check certificate usage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check certificate usage"})
		return
	}
	if inUse {
		c.JSON(http.StatusConflict, gin.H{"error": "certificate is in use by one or more proxy hosts"})
		return
	}

	if h.backupService != nil {
		if availableSpace, err := h.backupService.GetAvailableSpace(); err != nil {
			logger.Log().WithError(err).Warn("unable to check disk space, proceeding with backup")
		} else if availableSpace < 100*1024*1024 {
			c.JSON(http.StatusInsufficientStorage, gin.H{"error": "insufficient disk space for backup"})
			return
		}

		if _, err := h.backupService.CreateBackup(); err != nil {
			logger.Log().WithError(err).Error("failed to create backup before deletion")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup before deletion"})
			return
		}
	}

	if err := h.service.DeleteCertificate(idStr); err != nil {
		if err == services.ErrCertInUse {
			c.JSON(http.StatusConflict, gin.H{"error": "certificate is in use by one or more proxy hosts"})
			return
		}
		if err == services.ErrCertNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
			return
		}
		logger.Log().WithError(err).WithField("certificate_uuid", idStr).Error("failed to delete certificate")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete certificate"})
		return
	}

	h.sendDeleteNotification(c, idStr)
	c.JSON(http.StatusOK, gin.H{"message": "certificate deleted"})
}

func (h *CertificateHandler) sendDeleteNotification(c *gin.Context, certRef string) {
	if h.notificationService == nil {
		return
	}

	h.notificationMu.Lock()
	lastTime, exists := h.lastNotificationTime[certRef]
	if exists && time.Since(lastTime) < 10*time.Second {
		h.notificationMu.Unlock()
		logger.Log().WithField("certificate_ref", certRef).Debug("notification rate limited")
		return
	}
	h.lastNotificationTime[certRef] = time.Now()
	h.notificationMu.Unlock()

	h.notificationService.SendExternal(c.Request.Context(),
		"cert",
		"Certificate Deleted",
		fmt.Sprintf("Certificate %s deleted", certRef),
		map[string]any{
			"Ref":    certRef,
			"Action": "deleted",
		},
	)
}
