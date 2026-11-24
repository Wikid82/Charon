package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// Fixed client-facing texts. The detail of every failure goes to the server log.
const (
	encryptionStatusErrorText     = "Could not read the encryption status."
	encryptionRotationNotNeeded   = "Nothing to re-encrypt. Every stored secret already uses the current key; to move to a new key, set CHARON_ENCRYPTION_KEY_NEXT and restart."
	encryptionRotationFailedText  = "Key rotation failed. No stored secret was changed."
	encryptionValidationFailed    = "One or more encryption keys failed validation."
	encryptionHistoryErrorText    = "Could not read the encryption history."
	encryptionRotationTimeout     = 2 * time.Minute
	encryptionRotationNotNeededID = "rotation_not_needed"
	encryptionRotationFailedID    = "rotation_failed"
)

// EncryptionHandler manages encryption key operations and rotation.
type EncryptionHandler struct {
	rotationService *crypto.RotationService
	securityService *services.SecurityService
}

// NewEncryptionHandler creates a new encryption handler.
func NewEncryptionHandler(rotationService *crypto.RotationService, securityService *services.SecurityService) *EncryptionHandler {
	return &EncryptionHandler{
		rotationService: rotationService,
		securityService: securityService,
	}
}

// GetStatus returns the current encryption key status, verified by decryption.
// GET /api/v1/admin/encryption/status
func (h *EncryptionHandler) GetStatus(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	status, err := h.rotationService.GetStatus(c.Request.Context())
	if err != nil {
		logger.Log().WithError(err).Error("Encryption status could not be read")
		c.JSON(http.StatusInternalServerError, gin.H{"error": encryptionStatusErrorText})
		return
	}

	c.JSON(http.StatusOK, status)
}

// Rotate re-encrypts every stored secret to the rotation target.
// POST /api/v1/admin/encryption/rotate
func (h *EncryptionHandler) Rotate(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	h.logAudit(c, "encryption_key_rotation_started", map[string]any{})

	// A closed browser tab must not roll back a nearly finished run, so the run
	// uses the request's values without its cancellation, under a fixed bound.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), encryptionRotationTimeout)
	defer cancel()

	result, err := h.rotationService.RotateAll(ctx)
	if err != nil {
		if errors.Is(err, crypto.ErrRotationNotNeeded) {
			h.logAudit(c, "encryption_key_rotation_failed", map[string]any{"error_code": encryptionRotationNotNeededID})
			c.JSON(http.StatusConflict, gin.H{"error": encryptionRotationNotNeeded, "error_code": encryptionRotationNotNeededID})
			return
		}
		logger.Log().WithError(err).Error("Encryption key rotation failed; the transaction was rolled back")
		h.logAudit(c, "encryption_key_rotation_failed", map[string]any{"error_code": encryptionRotationFailedID})
		c.JSON(http.StatusInternalServerError, gin.H{"error": encryptionRotationFailedText, "error_code": encryptionRotationFailedID})
		return
	}

	h.logAudit(c, "encryption_key_rotation_completed", map[string]any{
		"target_version":        result.TargetVersion,
		"stores":                result.Stores,
		"total_records":         result.TotalRecords,
		"rotated_count":         result.RotatedCount,
		"already_current_count": result.AlreadyCurrentCount,
		"unreadable_count":      result.UnreadableCount,
		"duration":              result.Duration,
	})

	c.JSON(http.StatusOK, result)
}

// logAudit writes one encryption audit entry. A failed audit write is logged and
// never changes the response.
func (h *EncryptionHandler) logAudit(c *gin.Context, action string, details map[string]any) {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}
	if auditErr := h.securityService.LogAudit(&models.SecurityAudit{
		Actor:         auditActor(c),
		Action:        action,
		EventCategory: "encryption",
		Details:       string(detailsJSON),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	}); auditErr != nil {
		logger.Log().WithError(auditErr).WithField("action", action).Warn("Failed to log audit event")
	}
}

// GetHistory returns audit logs related to encryption key operations.
// GET /api/v1/admin/encryption/history
func (h *EncryptionHandler) GetHistory(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	page := 1
	limit := 50
	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	filter := services.AuditLogFilter{EventCategory: "encryption"}
	audits, total, err := h.securityService.ListAuditLogs(filter, page, limit)
	if err != nil {
		logger.Log().WithError(err).Error("Encryption history could not be read")
		c.JSON(http.StatusInternalServerError, gin.H{"error": encryptionHistoryErrorText})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"audits": audits,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

// Validate checks every configured encryption key.
// POST /api/v1/admin/encryption/validate
func (h *EncryptionHandler) Validate(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	if err := h.rotationService.ValidateKeyConfiguration(); err != nil {
		var keyErr *crypto.KeyValidationError
		failed := []string{}
		if errors.As(err, &keyErr) {
			failed = keyErr.Slots
		}
		logger.Log().WithError(err).Warn("Encryption key validation failed")
		h.logAudit(c, "encryption_key_validation_failed", map[string]any{"slots": failed})

		c.JSON(http.StatusBadRequest, gin.H{
			"valid":  false,
			"error":  encryptionValidationFailed,
			"errors": failed,
		})
		return
	}

	h.logAudit(c, "encryption_key_validation_success", map[string]any{})
	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "All encryption keys are valid",
	})
}
