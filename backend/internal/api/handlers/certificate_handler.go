package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
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
	// passwordGuard throttles private-key export re-authentication; nil allows.
	passwordGuard PasswordAttemptGuard
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

// SetPasswordAttemptGuard charges private-key export password checks to the sign-in budget.
func (h *CertificateHandler) SetPasswordAttemptGuard(g PasswordAttemptGuard) {
	h.passwordGuard = g
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
		if errors.Is(err, services.ErrCertNotFound) {
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

	// Read private key file (optional — format detection is content-based in the service)
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

	// Require key_file for non-PFX formats (PFX embeds the private key)
	if keyPEM == "" {
		format := services.DetectFormat(certBytes)
		if format != services.FormatPFX {
			c.JSON(http.StatusBadRequest, gin.H{"error": "key_file is required for PEM/DER certificate uploads"})
			return
		}
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
		if errors.Is(err, services.ErrCertNotFound) {
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
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation failed",
			"errors": []string{err.Error()},
		})
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

// reauthenticateForKeyExport verifies the signed-in admin's own password before
// private key material is released. It writes the error response itself and
// returns false when the caller must stop. Emergency-bypass sessions have no
// user row to re-verify against, so they are refused outright.
func (h *CertificateHandler) reauthenticateForKeyExport(c *gin.Context, password string) bool {
	if !requireAdmin(c) {
		return false
	}
	userID, ok := requireUserID(c)
	if !ok {
		return false
	}
	if userID == 0 && middleware.IsEmergencyBypass(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "private key export requires an authenticated user session"})
		return false
	}
	if h.db == nil {
		logger.Log().Error("certificate export re-authentication unavailable: database not configured")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return false
	}
	if password == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "password required to export private key"})
		return false
	}
	if !allowPasswordAttempt(h.passwordGuard, c) {
		return false
	}

	var user models.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "user not found"})
			return false
		}
		logger.Log().WithError(fmt.Errorf("certificate export user lookup: %w", err)).Error("failed to load user for re-authentication")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return false
	}
	if !user.CheckPassword(password) {
		c.JSON(http.StatusForbidden, gin.H{"error": "incorrect password"})
		return false
	}
	return true
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
	if req.IncludeKey && !h.reauthenticateForKeyExport(c, req.Password) {
		return
	}

	data, filename, err := h.service.ExportCertificate(certUUID, req.Format, req.IncludeKey, req.PFXPassword)
	if err != nil {
		if errors.Is(err, services.ErrCertNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
			return
		}
		logger.Log().WithError(fmt.Errorf("%s", util.SanitizeForLog(err.Error()))).Error("failed to export certificate")
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

		if !h.backupBeforeDelete(c) {
			return
		}

		if err := h.service.DeleteCertificateByID(uint(numID)); err != nil {
			if errors.Is(err, services.ErrCertNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
				return
			}
			if errors.Is(err, services.ErrCertInUse) {
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

	// UUID path - parse to validate format and produce a canonical, safe string
	parsedUUID, parseErr := uuid.Parse(idStr)
	if parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	certUUID := parsedUUID.String()

	// The usage check runs before the backup so a certificate that cannot be
	// deleted never triggers one.
	if err := h.checkCertificateUsage(certUUID); err != nil {
		h.respondCertificateDeleteError(c, certUUID, err)
		return
	}

	if !h.backupBeforeDelete(c) {
		return
	}

	if err := h.service.DeleteCertificate(certUUID); err != nil {
		h.respondCertificateDeleteError(c, certUUID, err)
		return
	}

	h.sendDeleteNotification(c, certUUID)
	c.JSON(http.StatusOK, gin.H{"message": "certificate deleted"})
}

// minBackupFreeSpaceBytes is the free disk space required before a pre-delete backup is attempted.
const minBackupFreeSpaceBytes = 100 * 1024 * 1024

// errCertUsageCheck marks a failed in-use lookup so it keeps its own response message.
var errCertUsageCheck = errors.New("certificate usage check failed")

// backupBeforeDelete takes one safety backup ahead of a destructive delete, after
// confirming there is room for it. It writes the error response and returns false
// when the caller must not delete anything. A nil backup service skips the step.
func (h *CertificateHandler) backupBeforeDelete(c *gin.Context) bool {
	if h.backupService == nil {
		return true
	}

	if availableSpace, err := h.backupService.GetAvailableSpace(); err != nil {
		logger.Log().WithError(err).Warn("unable to check disk space, proceeding with backup")
	} else if availableSpace < minBackupFreeSpaceBytes {
		logger.Log().WithField("available_bytes", availableSpace).Warn("low disk space, skipping backup")
		c.JSON(http.StatusInsufficientStorage, gin.H{"error": "insufficient disk space for backup"})
		return false
	}

	if _, err := h.backupService.CreateBackup(); err != nil {
		logger.Log().WithError(err).Error("failed to create backup before deletion")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup before deletion"})
		return false
	}
	return true
}

// checkCertificateUsage returns nil when the certificate exists and nothing references it.
// It returns services.ErrCertNotFound, services.ErrCertInUse, or an error wrapping errCertUsageCheck.
func (h *CertificateHandler) checkCertificateUsage(certUUID string) error {
	inUse, err := h.service.IsCertificateInUseByUUID(certUUID)
	if err != nil {
		if errors.Is(err, services.ErrCertNotFound) {
			return err
		}
		return fmt.Errorf("%w: %w", errCertUsageCheck, err)
	}
	if inUse {
		return services.ErrCertInUse
	}
	return nil
}

// respondCertificateDeleteError writes the single-delete response for a failed usage check or deletion.
func (h *CertificateHandler) respondCertificateDeleteError(c *gin.Context, certUUID string, err error) {
	safeUUID := util.SanitizeForLog(certUUID)
	switch {
	case errors.Is(err, services.ErrCertNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
	case errors.Is(err, services.ErrCertInUse):
		c.JSON(http.StatusConflict, gin.H{"error": "certificate is in use by one or more proxy hosts"})
	case errors.Is(err, errCertUsageCheck):
		logger.Log().WithError(err).WithField("certificate_uuid", safeUUID).Error("failed to check certificate usage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check certificate usage"})
	default:
		logger.Log().WithError(err).WithField("certificate_uuid", safeUUID).Error("failed to delete certificate")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete certificate"})
	}
}

// maxBulkDeleteCertificates caps how many certificates one bulk delete request may name.
const maxBulkDeleteCertificates = 200

var (
	errBulkDeleteCount = errors.New("uuids must contain between 1 and 200 entries")
	errBulkDeleteUUID  = errors.New("uuids must contain valid UUIDs")
)

type bulkDeleteCertificatesRequest struct {
	UUIDs []string `json:"uuids"`
}

// bulkDeleteCertificateFailure reports one certificate that was not deleted.
// Reason is one of "in_use", "not_found" or "error".
type bulkDeleteCertificateFailure struct {
	UUID   string `json:"uuid"`
	Reason string `json:"reason"`
}

type bulkDeleteCertificatesResponse struct {
	Deleted []string                       `json:"deleted"`
	Failed  []bulkDeleteCertificateFailure `json:"failed"`
}

// normalizeBulkDeleteUUIDs validates the requested UUIDs, returns their canonical
// forms, and drops duplicates while keeping the request order.
func normalizeBulkDeleteUUIDs(raw []string) ([]string, error) {
	if len(raw) == 0 || len(raw) > maxBulkDeleteCertificates {
		return nil, errBulkDeleteCount
	}

	seen := make(map[string]struct{}, len(raw))
	uuids := make([]string, 0, len(raw))
	for _, candidate := range raw {
		parsed, err := uuid.Parse(candidate)
		if err != nil {
			return nil, errBulkDeleteUUID
		}
		canonical := parsed.String()
		if _, dup := seen[canonical]; dup {
			continue
		}
		seen[canonical] = struct{}{}
		uuids = append(uuids, canonical)
	}
	return uuids, nil
}

// bulkDeleteReason maps a per-certificate failure to the reason reported to the client.
func bulkDeleteReason(err error) string {
	switch {
	case errors.Is(err, services.ErrCertNotFound):
		return "not_found"
	case errors.Is(err, services.ErrCertInUse):
		return "in_use"
	default:
		return "error"
	}
}

// BulkDelete removes several certificates by UUID. One backup covers the whole
// batch and is taken before any deletion. Per-certificate failures are reported
// in the response and do not stop the remaining deletions.
func (h *CertificateHandler) BulkDelete(c *gin.Context) {
	var req bulkDeleteCertificatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	uuids, err := normalizeBulkDeleteUUIDs(req.UUIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.backupBeforeDelete(c) {
		return
	}

	resp := bulkDeleteCertificatesResponse{
		Deleted: make([]string, 0, len(uuids)),
		Failed:  make([]bulkDeleteCertificateFailure, 0),
	}
	for _, certUUID := range uuids {
		deleteErr := h.checkCertificateUsage(certUUID)
		if deleteErr == nil {
			deleteErr = h.service.DeleteCertificate(certUUID)
		}
		if deleteErr != nil {
			reason := bulkDeleteReason(deleteErr)
			logger.Log().WithError(deleteErr).
				WithField("certificate_uuid", util.SanitizeForLog(certUUID)).
				WithField("reason", reason).
				Warn("bulk certificate delete skipped certificate")
			resp.Failed = append(resp.Failed, bulkDeleteCertificateFailure{UUID: certUUID, Reason: reason})
			continue
		}

		h.sendDeleteNotification(c, certUUID)
		resp.Deleted = append(resp.Deleted, certUUID)
	}

	c.JSON(http.StatusOK, resp)
}

func (h *CertificateHandler) sendDeleteNotification(c *gin.Context, certRef string) {
	if h.notificationService == nil {
		return
	}

	// Re-validate to produce a CodeQL-safe value (breaks taint from user input).
	// Callers already pass validated data; this is defense-in-depth.
	safeRef := sanitizeCertRef(certRef)

	h.notificationMu.Lock()
	lastTime, exists := h.lastNotificationTime[certRef]
	if exists && time.Since(lastTime) < 10*time.Second {
		h.notificationMu.Unlock()
		logger.Log().WithField("certificate_ref", safeRef).Debug("notification rate limited")
		return
	}
	h.lastNotificationTime[certRef] = time.Now()
	h.notificationMu.Unlock()

	h.notificationService.SendExternal(c.Request.Context(),
		"cert",
		"Certificate Deleted",
		fmt.Sprintf("Certificate %s deleted", safeRef),
		map[string]any{
			"Ref":    safeRef,
			"Action": "deleted",
		},
	)
}

// sanitizeCertRef re-validates a certificate reference (UUID or numeric ID)
// and returns a safe string representation. Returns a placeholder if invalid.
func sanitizeCertRef(ref string) string {
	if parsed, err := uuid.Parse(ref); err == nil {
		return parsed.String()
	}
	if n, err := strconv.ParseUint(ref, 10, 64); err == nil {
		return strconv.FormatUint(n, 10)
	}
	return "[invalid-ref]"
}
