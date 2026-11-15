package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/services"
)

// Coded answers of the trusted-proxy writer. Each code is answered through respondCoded.
const (
	codeTrustedProxyInvalid   = "trusted_proxy_invalid"
	codeTrustedProxyTooBroad  = "trusted_proxy_too_broad"
	codeTrustedProxiesTooMany = "trusted_proxies_too_many"
	msgTrustedProxyInvalid    = "Enter an IP address or a CIDR range such as 172.18.0.0/16."
	msgTrustedProxyTooBroad   = "That range is too wide to trust. Enter a narrower range."
	msgTrustedProxiesTooMany  = "Enter at most 64 entries."
)

// errTrustedProxyApplyFailed is returned when the proxy refuses the new trust list. The previous value is restored
// before it is returned.
var errTrustedProxyApplyFailed = errors.New("trusted proxy list: proxy apply failed")

// TrustedProxyHandler serves the proxy-layer trust list on the Security page. The list decides which peers may vouch
// for the visitor's address at the proxy.
type TrustedProxyHandler struct {
	db      *gorm.DB
	caddy   CaddyConfigManager
	aclMode string
	// apiLayer is the CHARON_TRUSTED_PROXIES list read at start. It is shown read-only and also seeds the proxy layer
	// until a list is saved.
	apiLayer []string
}

// NewTrustedProxyHandler creates the trusted-proxy handler. caddy may be nil, in which case a saved list is stored
// without a proxy apply.
func NewTrustedProxyHandler(db *gorm.DB, caddy CaddyConfigManager, apiLayer []string, aclMode string) *TrustedProxyHandler {
	return &TrustedProxyHandler{db: db, caddy: caddy, apiLayer: apiLayer, aclMode: aclMode}
}

type trustedProxiesRequest struct {
	Proxies []string `json:"proxies"`
}

type trustedProxiesResponse struct {
	Proxies        []string `json:"proxies"`
	Source         string   `json:"source"`
	DroppedEntries []string `json:"dropped_entries"`
	AlwaysTrusted  []string `json:"always_trusted"`
	APILayer       []string `json:"api_layer"`
	Warnings       []string `json:"warnings"`
}

// trustedProxyHiddenFields are the response keys a non-administrator does not see.
var trustedProxyHiddenFields = []string{"proxies", "dropped_entries", "api_layer", "warnings"}

// Get returns the trust list in force. Non-administrators learn only whether any proxies are configured.
func (h *TrustedProxyHandler) Get(c *gin.Context) {
	trust, warnings, err := h.currentTrust()
	if err != nil {
		logger.Log().WithError(err).Error("Failed to read the trusted proxy list")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read trusted proxies"})
		return
	}
	h.respond(c, http.StatusOK, trust, warnings)
}

// Put replaces the saved trust list. An empty list trusts loopback only.
func (h *TrustedProxyHandler) Put(c *gin.Context) {
	var req trustedProxiesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	canonical, _, err := security.NormalizeTrustedProxies(req.Proxies)
	if err != nil {
		respondTrustedProxyError(c, err)
		return
	}
	value := strings.Join(canonical, ",")
	if h.refuseTrustChange(c, &value) {
		return
	}
	h.saveAndRespond(c, &value)
}

// Delete removes the saved trust list, so the environment list or the loopback default applies again.
func (h *TrustedProxyHandler) Delete(c *gin.Context) {
	if h.refuseTrustChange(c, nil) {
		return
	}
	h.saveAndRespond(c, nil)
}

// saveAndRespond writes value (nil removes the saved list), applies it, and answers with the list now in force.
func (h *TrustedProxyHandler) saveAndRespond(c *gin.Context, value *string) {
	if err := h.writeTrustedProxies(c.Request.Context(), value); err != nil {
		if errors.Is(err, errTrustedProxyApplyFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": msgModuleApplyFailed})
			return
		}
		logger.Log().WithError(err).Error("Failed to save the trusted proxy list")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save trusted proxies"})
		return
	}
	trust, warnings, err := h.currentTrust()
	if err != nil {
		logger.Log().WithError(err).Error("Failed to read the trusted proxy list after saving")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read trusted proxies"})
		return
	}
	h.respond(c, http.StatusOK, trust, warnings)
}

// writeTrustedProxies stores value inside a write-locked transaction and applies the proxy. A refused apply restores
// the previous value, including its absence, and returns errTrustedProxyApplyFailed.
func (h *TrustedProxyHandler) writeTrustedProxies(ctx context.Context, value *string) error {
	var previous *string
	err := services.WithWriteLock(h.db, &models.Setting{}, func(tx *gorm.DB) error {
		var readErr error
		if previous, readErr = models.ReadTrustedProxiesSetting(tx); readErr != nil {
			return readErr
		}
		return models.WriteTrustedProxiesSetting(tx, value)
	})
	if err != nil {
		return fmt.Errorf("store trusted proxy list: %w", err)
	}
	if h.caddy == nil {
		return nil
	}
	if err := applyProxyConfigDetached(ctx, h.caddy, proxyApplyTimeout); err != nil {
		logger.Log().WithError(err).Warn("Failed to reload Caddy config after trusted proxy change")
		restoreErr := services.WithWriteLock(h.db, &models.Setting{}, func(tx *gorm.DB) error {
			return models.WriteTrustedProxiesSetting(tx, previous)
		})
		if restoreErr != nil {
			logger.Log().WithError(restoreErr).Error("Failed to restore the trusted proxy list after the apply failed")
		}
		return errTrustedProxyApplyFailed
	}
	return nil
}

// currentTrust returns the trust list in force together with the warnings that apply to the saved value.
func (h *TrustedProxyHandler) currentTrust() (security.EffectiveTrust, []string, error) {
	stored, err := models.ReadTrustedProxiesSetting(h.db)
	if err != nil {
		return security.EffectiveTrust{}, nil, err
	}
	trust := security.EffectiveTrustedProxies(stored, h.apiLayer)
	var warnings []string
	if stored != nil {
		if _, saved, parseErr := security.ParseStoredTrustedProxies(*stored); parseErr != nil {
			warnings = []string{parseErr.Error()}
		} else {
			warnings = saved
		}
	}
	return trust, warnings, nil
}

// respond writes the trust list in force, redacted for non-administrators.
func (h *TrustedProxyHandler) respond(c *gin.Context, status int, trust security.EffectiveTrust, warnings []string) {
	body := trustedProxiesResponse{
		Proxies:        nonNilStrings(trust.Configured),
		Source:         trust.Source,
		DroppedEntries: nonNilStrings(trust.Dropped),
		AlwaysTrusted:  security.AlwaysTrustedProxies(),
		APILayer:       nonNilStrings(h.apiLayer),
		Warnings:       nonNilStrings(warnings),
	}
	out, err := redactForRole(c, body, trustedProxyHiddenFields, func(r trustedProxiesResponse) map[string]any {
		return map[string]any{"has_proxies": len(r.Proxies) > 0}
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode trusted proxies"})
		return
	}
	c.JSON(status, out)
}

// respondTrustedProxyError answers a refused entry with its code and the entry it names.
func respondTrustedProxyError(c *gin.Context, err error) {
	var entryErr *security.TrustedProxyEntryError
	entry := ""
	if errors.As(err, &entryErr) {
		entry = entryErr.Entry
	}
	switch {
	case errors.Is(err, security.ErrTrustedProxyInvalid):
		respondCoded(c, http.StatusBadRequest, codeTrustedProxyInvalid, msgTrustedProxyInvalid, gin.H{"entry": entry})
	case errors.Is(err, security.ErrTrustedProxyTooBroad):
		respondCoded(c, http.StatusBadRequest, codeTrustedProxyTooBroad, msgTrustedProxyTooBroad, gin.H{"entry": entry})
	case errors.Is(err, security.ErrTooManyTrustedProxies):
		respondCoded(c, http.StatusBadRequest, codeTrustedProxiesTooMany, msgTrustedProxiesTooMany, nil)
	default:
		logger.Log().WithError(err).Error("Trusted proxy validation failed unexpectedly")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save trusted proxies"})
	}
}

// nonNilStrings returns s, or an empty slice so the JSON value is [] rather than null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// refuseTrustChange judges a trust list change before it is saved. The caller address is recomputed under the list
// that would be in force, so a change that stops trusting the caller's proxy cannot lock the caller out of a
// management list. It returns true when the change was refused and the response written.
func (h *TrustedProxyHandler) refuseTrustChange(c *gin.Context, stored *string) bool {
	trust := security.EffectiveTrustedProxies(stored, h.apiLayer)
	guard := newManagementGuard(h.db, h.aclMode)
	st, err := guard.current(callerAddressUnder(c, parseTrustedRanges(trust.Ranges)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check management access"})
		return true
	}
	return guard.refuses(c, st)
}

// parseTrustedRanges returns the prefixes of the canonical trust ranges. Ranges that do not parse are skipped; the
// canonical form always parses.
func parseTrustedRanges(ranges []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(ranges))
	for _, r := range ranges {
		if p, err := netip.ParsePrefix(r); err == nil {
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
}
