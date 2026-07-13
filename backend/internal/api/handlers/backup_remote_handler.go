package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
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
		"uuid":               target.UUID,
		"name":               target.Name,
		"type":               target.Type,
		"enabled":            target.Enabled,
		"config":             config,
		"secrets_set":        target.SecretsEncrypted != "",
		"last_test_at":       target.LastTestAt,
		"last_test_status":   target.LastTestStatus,
		"last_error":         target.LastError,
		"oauth_status":       target.OAuthStatus,
		"oauth_connected_at": target.OAuthConnectedAt,
		"created_at":         target.CreatedAt,
		"updated_at":         target.UpdatedAt,
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
	case errors.Is(err, services.ErrPublicURLNotConfigured):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "public_url_not_configured"})
	case errors.Is(err, remotestorage.ErrOAuthNotConnected):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "error_code": "oauth_not_connected"})
	case errors.Is(err, remotestorage.ErrOAuthRevoked):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "error_code": "oauth_revoked"})
	default:
		code := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
	}
}

// OAuthStart handles POST /api/v1/backups/remote-targets/:uuid/oauth/start
// (admin). Issues a fresh CSRF state token and returns the provider's
// authorize URL (spec §3.3/R3).
func (h *BackupRemoteHandler) OAuthStart(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	authorizeURL, err := h.service.StartOAuth(c.Param("uuid"))
	if err != nil {
		h.respondRemoteTargetError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"authorize_url": authorizeURL})
}

// OAuthCallback handles
// GET /api/v1/backups/remote-targets/oauth/:provider/callback.
//
// SECURITY: this route is deliberately registered OUTSIDE
// RequireManagementAccess (see routes.go) — the browser arrives here
// directly from Dropbox/Google with no Charon session at all, so a
// session/JWT check here cannot succeed and is not the actual control. Its
// entire security rests on the single-use, time-bound `state` CSRF token
// issued by OAuthStart and consumed below (spec §3.8, R3) — never add an
// auth check to this handler; it is unreachable in the flow this route
// exists to serve.
func (h *BackupRemoteHandler) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	state := c.Query("state")
	code := c.Query("code")
	providerErr := c.Query("error")

	// provider is a path segment supplied by whatever redirected the
	// browser here — never trust it as-is. Pin it to the fixed, known
	// provider set before it's used anywhere else in this handler
	// (including the redirect URLs built below): an unrecognized value is
	// rejected outright, the same way an invalid `state` is.
	if provider != "dropbox" && provider != "google_drive" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported oauth provider", "error_code": "invalid_oauth_state"})
		return
	}

	targetUUID, stateProvider, ok := h.service.ConsumeOAuthState(state)
	if !ok || stateProvider != provider {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired authorization state", "error_code": "invalid_oauth_state"})
		return
	}

	baseURL, hasURL := h.service.ConfiguredPublicURL()
	if !hasURL {
		// Extremely unlikely (oauth/start already required this to exist),
		// but if the setting was cleared mid-flow there is no safe base to
		// redirect to.
		c.JSON(http.StatusBadRequest, gin.H{"error": "app.public_url is not configured", "error_code": "public_url_not_configured"})
		return
	}
	redirectBase := strings.TrimSuffix(baseURL, "/") + "/backups"

	if providerErr != "" {
		// User clicked "Deny" at the provider's consent screen — never a
		// partial-token state, target remains not_connected (spec §3.9).
		// nosemgrep: go.gin.ssrf.gin-tainted-url-host.gin-tainted-url-host -- redirectBase's host comes entirely from utils.GetConfiguredPublicURL (an admin-configured Setting row, never request-derived); provider is already pinned to the "dropbox"/"google_drive" allowlist above and is url.QueryEscape'd into the query string only, never the host, so no attacker input can redirect the browser off-origin.
		c.Redirect(http.StatusFound, redirectBase+"?oauth_result=error&provider="+url.QueryEscape(provider)+"&message=authorization_denied")
		return
	}

	if err := h.service.CompleteOAuth(c.Request.Context(), targetUUID, provider, code, baseURL); err != nil {
		// Never echo the raw provider error body into the redirect URL
		// (spec §3.8) — only a short, non-sensitive message code. The real
		// error is logged server-side for diagnosis.
		middleware.GetRequestLogger(c).WithField("action", "oauth_callback_failed").
			WithField("provider", util.SanitizeForLog(provider)).WithField("error", util.SanitizeForLog(err.Error())).
			Warn("OAuth callback token exchange failed")
		// nosemgrep: go.gin.ssrf.gin-tainted-url-host.gin-tainted-url-host -- see justification above; same allowlisted provider, same admin-configured redirectBase host.
		c.Redirect(http.StatusFound, redirectBase+"?oauth_result=error&provider="+url.QueryEscape(provider)+"&message=token_exchange_failed")
		return
	}

	// nosemgrep: go.gin.ssrf.gin-tainted-url-host.gin-tainted-url-host -- see justification above; targetUUID is server-generated (uuid.New(), BeforeCreate) not request-supplied, and provider is allowlisted; redirectBase's host is never attacker-influenced.
	c.Redirect(http.StatusFound, redirectBase+"?oauth_result=success&provider="+url.QueryEscape(provider)+"&target="+url.QueryEscape(targetUUID))
}

// OAuthDisconnect handles
// POST /api/v1/backups/remote-targets/:uuid/oauth/disconnect (admin).
func (h *BackupRemoteHandler) OAuthDisconnect(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	target, err := h.service.DisconnectOAuth(c.Param("uuid"))
	if err != nil {
		h.respondRemoteTargetError(c, err)
		return
	}

	middleware.GetRequestLogger(c).WithField("action", "disconnect_remote_target_oauth").
		WithField("target", util.SanitizeForLog(target.Name)).Info("Remote storage target OAuth disconnected")
	c.JSON(http.StatusOK, toRemoteTargetResponse(target))
}
