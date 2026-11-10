package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// geoipStatusResponse is the body of GET /security/geoip/status. The first four fields and reason are the original
// contract; the database card fields follow, flattened from services.GeoIPStatus. reason is present only when the
// database is unavailable, so the card can show why country lists cannot match.
type geoipStatusResponse struct {
	Available bool   `json:"available"`
	Loaded    bool   `json:"loaded"`
	DBPath    string `json:"db_path"`
	Message   string `json:"message"`
	Reason    string `json:"reason,omitempty"`
	services.GeoIPStatus
}

// GetGeoIPStatus returns the current status of the GeoIP service and the database card.
func (h *SecurityHandler) GetGeoIPStatus(c *gin.Context) {
	available := h.geoipSvc.Available()
	body := geoipStatusResponse{
		Available:   available,
		Message:     "GeoIP service not initialized",
		GeoIPStatus: h.geoipStatus(),
	}
	if h.geoipSvc != nil {
		body.Loaded = h.geoipSvc.IsLoaded()
		body.DBPath = h.geoipSvc.GetDatabasePath()
		body.Message = "GeoIP database unavailable"
	}
	if available {
		body.Message = "GeoIP service available"
	} else {
		body.Reason = caddy.ReasonGeoIPUnavailable
	}
	c.JSON(http.StatusOK, body)
}

// geoipStatus returns the database card status with the number of country lists the proxy is not enforcing. Without a
// store (no GeoIP service is wired) it reports no database.
func (h *SecurityHandler) geoipStatus() services.GeoIPStatus {
	if h.geoipStore == nil {
		return services.GeoIPStatus{Source: string(geoip.SourceNone)}
	}
	st := h.geoipStore.Status()
	st.UnenforcedListsTotal = h.unenforcedCountryListsTotal()
	return st
}

// unenforcedCountryListsTotal counts the country lists the proxy is not enforcing. Without a database there is no
// access-list state to read, so it reports zero.
func (h *SecurityHandler) unenforcedCountryListsTotal() int {
	if h.db == nil {
		return 0
	}
	return len(h.countryListsNotEnforced(h.aclEnabledForAlerts()))
}

// Stable error_code values and fixed messages of the GeoIP reload endpoint. The error text of the underlying failure
// (which can name filesystem paths) is logged server-side only and never returned to the client.
const (
	codeGeoIPReloadFailed       = "geoip_reload_failed"
	codeGeoIPServiceUnavailable = "geoip_service_unavailable"
	codeGeoIPBusy               = "geoip_busy"
	codeProxyApplyFailed        = "proxy_apply_failed"
	msgGeoIPReloadFailed        = "Failed to reload the GeoIP database"
	msgGeoIPBusy                = "Another GeoIP database change is in progress"
)

// ReloadGeoIP reloads the GeoIP database from disk.
func (h *SecurityHandler) ReloadGeoIP(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	if h.geoipStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":      "GeoIP service not initialized",
			"error_code": codeGeoIPServiceUnavailable,
		})
		return
	}

	// The store loads the file and applies the proxy config, so country lists follow the reload at once.
	if err := h.geoipStore.Reload(c.Request.Context()); err != nil {
		h.writeGeoIPReloadError(c, err)
		return
	}

	// Log audit event
	actor := auditActor(c)
	_ = h.svc.LogAudit(&models.SecurityAudit{Actor: actor, Action: "reload_geoip", Details: "GeoIP database reloaded successfully"})

	c.JSON(http.StatusOK, gin.H{
		"message": "GeoIP database reloaded successfully",
		"loaded":  h.geoipSvc.IsLoaded(),
		"db_path": h.geoipSvc.GetDatabasePath(),
		"status":  h.geoipStatus(),
	})
}

// writeGeoIPReloadError maps a failed reload to its status and error_code. The detail is logged, never returned.
func (h *SecurityHandler) writeGeoIPReloadError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrGeoIPBusy) {
		c.JSON(http.StatusConflict, gin.H{
			"error":      msgGeoIPBusy,
			"error_code": codeGeoIPBusy,
		})
		return
	}
	log.WithField("detail", util.SanitizeForLog(err.Error())).Error("Failed to reload GeoIP database")
	if errors.Is(err, services.ErrGeoIPReloadFailed) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      msgGeoIPReloadFailed,
			"error_code": codeGeoIPReloadFailed,
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":      msgModuleApplyFailed,
		"error_code": codeProxyApplyFailed,
	})
}

// LookupGeoIP performs a GeoIP lookup for a given IP address.
func (h *SecurityHandler) LookupGeoIP(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req struct {
		IPAddress string `json:"ip_address" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip_address is required"})
		return
	}

	if h.geoipSvc == nil || !h.geoipSvc.IsLoaded() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GeoIP service not available",
		})
		return
	}

	country, err := h.geoipSvc.LookupCountry(req.IPAddress)
	if err != nil {
		if errors.Is(err, services.ErrInvalidGeoIP) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid IP address"})
			return
		}
		if errors.Is(err, services.ErrCountryNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"ip_address":   req.IPAddress,
				"country_code": "",
				"found":        false,
				"message":      "No country found for this IP address",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "GeoIP lookup failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ip_address":   req.IPAddress,
		"country_code": country,
		"found":        true,
	})
}
