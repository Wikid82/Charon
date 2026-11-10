package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// GetGeoIPStatus returns the current status of the GeoIP service.
// The reason is present only when the database is unavailable, so the card can show why country lists cannot match.
func (h *SecurityHandler) GetGeoIPStatus(c *gin.Context) {
	available := h.geoipSvc.Available()
	body := gin.H{
		"available": available,
		"loaded":    false,
		"db_path":   "",
		"message":   "GeoIP service not initialized",
	}
	if h.geoipSvc != nil {
		body["loaded"] = h.geoipSvc.IsLoaded()
		body["db_path"] = h.geoipSvc.GetDatabasePath()
		body["message"] = "GeoIP database unavailable"
	}
	if available {
		body["message"] = "GeoIP service available"
	} else {
		body["reason"] = caddy.ReasonGeoIPUnavailable
	}
	c.JSON(http.StatusOK, body)
}

// Stable error_code values and fixed messages of the GeoIP reload endpoint. The error text of the underlying failure
// (which can name filesystem paths) is logged server-side only and never returned to the client.
const (
	codeGeoIPReloadFailed       = "geoip_reload_failed"
	codeGeoIPServiceUnavailable = "geoip_service_unavailable"
	codeProxyApplyFailed        = "proxy_apply_failed"
	msgGeoIPReloadFailed        = "Failed to reload the GeoIP database"
)

// ReloadGeoIP reloads the GeoIP database from disk.
func (h *SecurityHandler) ReloadGeoIP(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	if h.geoipSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":      "GeoIP service not initialized",
			"error_code": codeGeoIPServiceUnavailable,
		})
		return
	}

	loadErr := h.geoipSvc.Load()
	if loadErr != nil {
		log.WithField("detail", util.SanitizeForLog(loadErr.Error())).Error("Failed to reload GeoIP database")
	}

	// The proxy reads the database only when its config is generated, so apply now and country lists follow the reload
	// at once. The apply runs even when the load failed: the proxy then converges on the database state it can see
	// (country lists are not enforced without a usable database) instead of keeping a config that no longer matches.
	if h.caddyManager != nil {
		if err := applyProxyConfigDetached(c.Request.Context(), h.caddyManager, proxyApplyTimeout); err != nil {
			log.WithField("detail", util.SanitizeForLog(err.Error())).Warn("failed to apply proxy config after GeoIP reload")
			if loadErr == nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":      msgModuleApplyFailed,
					"error_code": codeProxyApplyFailed,
				})
				return
			}
		}
	}

	if loadErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      msgGeoIPReloadFailed,
			"error_code": codeGeoIPReloadFailed,
		})
		return
	}

	// Log audit event
	actor := auditActor(c)
	_ = h.svc.LogAudit(&models.SecurityAudit{Actor: actor, Action: "reload_geoip", Details: "GeoIP database reloaded successfully"})

	c.JSON(http.StatusOK, gin.H{
		"message": "GeoIP database reloaded successfully",
		"loaded":  h.geoipSvc.IsLoaded(),
		"db_path": h.geoipSvc.GetDatabasePath(),
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
