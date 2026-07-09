package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/services/remotestorage"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BackupRemoteHandler exposes remote storage target CRUD and connection
// testing (spec §3.3.2). Every route is admin-gated (see the auth policy
// table in spec §3.3): hostnames/usernames/bucket names are sensitive even
// with secrets omitted, so this is not management-level like plain backup
// listing.
type BackupRemoteHandler struct {
	service *services.BackupRemoteService
}

func NewBackupRemoteHandler(service *services.BackupRemoteService) *BackupRemoteHandler {
	return &BackupRemoteHandler{service: service}
}

// toRemoteTargetResponse builds the wire shape for a remote target (spec
// §3.3.2) — secrets are never included, only a secrets_set boolean.
func toRemoteTargetResponse(target *models.RemoteStorageTarget) gin.H {
	var config services.RemoteTargetConfig
	if target.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(target.ConfigJSON), &config)
	}

	return gin.H{
		"uuid":             target.UUID,
		"name":             target.Name,
		"type":             target.Type,
		"enabled":          target.Enabled,
		"config":           config,
		"secrets_set":      target.SecretsEncrypted != "",
		"last_test_at":     target.LastTestAt,
		"last_test_status": target.LastTestStatus,
		"last_error":       target.LastError,
		"created_at":       target.CreatedAt,
		"updated_at":       target.UpdatedAt,
	}
}

type remoteTargetRequest struct {
	Name    string                       `json:"name"`
	Type    string                       `json:"type"`
	Enabled *bool                        `json:"enabled"`
	Config  services.RemoteTargetConfig  `json:"config"`
	Secrets services.RemoteTargetSecrets `json:"secrets"`
}

// List handles GET /api/v1/backups/remote-targets (admin).
func (h *BackupRemoteHandler) List(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	targets, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list remote targets"})
		return
	}

	responses := make([]gin.H, 0, len(targets))
	for i := range targets {
		responses = append(responses, toRemoteTargetResponse(&targets[i]))
	}
	c.JSON(http.StatusOK, responses)
}

// Create handles POST /api/v1/backups/remote-targets (admin).
func (h *BackupRemoteHandler) Create(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req remoteTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	target, err := h.service.Create(req.Name, req.Type, enabled, req.Config, req.Secrets)
	if err != nil {
		h.respondRemoteTargetError(c, err)
		return
	}

	middleware.GetRequestLogger(c).WithField("action", "create_remote_target").
		WithField("target", util.SanitizeForLog(target.Name)).Info("Remote storage target created")
	c.JSON(http.StatusCreated, toRemoteTargetResponse(target))
}

// Update handles PUT /api/v1/backups/remote-targets/:uuid (admin). Secret
// fields left empty in the request mean "keep existing" (spec §3.3.2).
func (h *BackupRemoteHandler) Update(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req remoteTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var namePtr *string
	if req.Name != "" {
		namePtr = &req.Name
	}

	target, err := h.service.Update(c.Param("uuid"), namePtr, req.Enabled, &req.Config, &req.Secrets)
	if err != nil {
		h.respondRemoteTargetError(c, err)
		return
	}

	middleware.GetRequestLogger(c).WithField("action", "update_remote_target").
		WithField("target", util.SanitizeForLog(target.Name)).Info("Remote storage target updated")
	c.JSON(http.StatusOK, toRemoteTargetResponse(target))
}

// Delete handles DELETE /api/v1/backups/remote-targets/:uuid (admin).
func (h *BackupRemoteHandler) Delete(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	if err := h.service.Delete(c.Param("uuid")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete remote target"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Remote target deleted"})
}

// Test handles POST /api/v1/backups/remote-targets/:uuid/test (admin).
func (h *BackupRemoteHandler) Test(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	start := time.Now()
	err := h.service.Test(c.Request.Context(), c.Param("uuid"))
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		h.respondRemoteTargetError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Connection successful", "latency_ms": latencyMs})
}

// testDraftRequest is the request body for POST
// /api/v1/backups/remote-targets/test-draft. Config reuses the exact same
// services.RemoteTargetConfig shape (and json tags) accepted by
// Create/Update, so the frontend can build the request from the same draft
// form state it already holds before the target has been saved/has a UUID.
type testDraftRequest struct {
	Type   string                      `json:"type"`
	Config services.RemoteTargetConfig `json:"config"`
}

// TestDraft handles POST /api/v1/backups/remote-targets/test-draft (admin).
// It is the stateless counterpart of Test: unlike Test, it never looks up a
// persisted RemoteStorageTarget, so it can run SFTP host-key discovery
// against a draft config the user hasn't saved yet (spec §3.7). Only
// type "sftp" is supported — S3 has no host-key-pinning discovery step, so
// draft-testing an S3 config already works via the create/update SSRF
// validation and needs no stateless endpoint.
//
// remotestorage.DiscoverSFTPHostKey never attempts authentication (it
// aborts the SSH handshake as soon as the host key is offered) and applies
// the same SSRF policy as every other remote-target entry point before
// dialing — see TestBackupRemoteHandler_TestDraft_NeverAuthenticates and
// TestBackupRemoteHandler_TestDraft_SSRFRejected.
func (h *BackupRemoteHandler) TestDraft(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req testDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Type != "sftp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": `test-draft only supports type "sftp"`})
		return
	}

	start := time.Now()
	fingerprint, err := remotestorage.DiscoverSFTPHostKey(remotestorage.SFTPConfig{
		Host:     req.Config.Host,
		Port:     req.Config.Port,
		Path:     req.Config.Path,
		Username: req.Config.Username,
	})
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                true,
		"message":                "Host key discovered — confirm the fingerprint before saving",
		"discovered_fingerprint": fingerprint,
		"latency_ms":             latencyMs,
	})
}

func (h *BackupRemoteHandler) respondRemoteTargetError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrEncryptionKeyMissing):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "error_code": "encryption_key_missing"})
	default:
		code := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
	}
}
