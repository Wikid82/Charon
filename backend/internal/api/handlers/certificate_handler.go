package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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
}

type CertificateHandler struct {
	service             *services.CertificateService
	backupService       BackupServiceInterface
	notificationService *services.NotificationService
}

func NewCertificateHandler(service *services.CertificateService, backupService BackupServiceInterface, ns *services.NotificationService) *CertificateHandler {
	return &CertificateHandler{
		service:             service,
		backupService:       backupService,
		notificationService: ns,
	}
}

func (h *CertificateHandler) List(c *gin.Context) {
	certs, err := h.service.ListCertificates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	defer func() { _ = certSrc.Close() }()

	keySrc, err := keyFile.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open key file"})
		return
	}
	defer func() { _ = keySrc.Close() }()

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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Send Notification
	if h.notificationService != nil {
		h.notificationService.SendExternal(c.Request.Context(),
			"cert",
			"Certificate Uploaded",
			fmt.Sprintf("Certificate %s uploaded", util.SanitizeForLog(cert.Name)),
			map[string]interface{}{
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

	// Check if certificate is in use before proceeding
	inUse, err := h.service.IsCertificateInUse(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check certificate usage"})
		return
	}
	if inUse {
		c.JSON(http.StatusConflict, gin.H{"error": "certificate is in use by one or more proxy hosts"})
		return
	}

	// Create backup before deletion
	if h.backupService != nil {
		if _, err := h.backupService.CreateBackup(); err != nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Send Notification
	if h.notificationService != nil {
		h.notificationService.SendExternal(c.Request.Context(),
			"cert",
			"Certificate Deleted",
			fmt.Sprintf("Certificate ID %d deleted", id),
			map[string]interface{}{
				"ID":     id,
				"Action": "deleted",
			},
		)
	}

	c.JSON(http.StatusOK, gin.H{"message": "certificate deleted"})
}
