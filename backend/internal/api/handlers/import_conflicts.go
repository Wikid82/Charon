package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// proxyHostLister is the subset of the proxy host service needed for import
// conflict detection.
type proxyHostLister interface {
	List() ([]models.ProxyHost, error)
}

// listExistingHostsForImport loads the current proxy hosts for conflict
// detection. On failure it logs the error server-side, writes a generic 500
// response and returns ok=false so callers fail closed instead of treating the
// result as "no conflicts".
func listExistingHostsForImport(c *gin.Context, svc proxyHostLister) ([]models.ProxyHost, bool) {
	hosts, err := svc.List()
	if err != nil {
		middleware.GetRequestLogger(c).WithError(err).Error("Import: failed to load existing hosts for conflict detection")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check for existing hosts"})
		return nil, false
	}
	return hosts, true
}

// findOverlappingHost returns the first existing proxy host that shares at
// least one (canonicalized) domain with domainNames.
func findOverlappingHost(existing []models.ProxyHost, domainNames string) (models.ProxyHost, bool) {
	for _, eh := range existing {
		if services.DomainsShareAny(eh.DomainNames, domainNames) {
			return eh, true
		}
	}
	return models.ProxyHost{}, false
}

// detectImportConflicts returns the imported domain strings that overlap an
// existing host, plus per-domain existing/imported comparison details.
func detectImportConflicts(existing []models.ProxyHost, imported []caddy.ParsedHost) ([]string, map[string]gin.H) {
	conflicts := []string{}
	details := make(map[string]gin.H)
	for _, ph := range imported {
		eh, found := findOverlappingHost(existing, ph.DomainNames)
		if !found {
			continue
		}
		conflicts = append(conflicts, ph.DomainNames)
		details[ph.DomainNames] = gin.H{
			"existing": gin.H{
				"forward_scheme": eh.ForwardScheme,
				"forward_host":   eh.ForwardHost,
				"forward_port":   eh.ForwardPort,
				"ssl_forced":     eh.SSLForced,
				"websocket":      eh.WebsocketSupport,
				"enabled":        eh.Enabled,
			},
			"imported": gin.H{
				"forward_scheme": ph.ForwardScheme,
				"forward_host":   ph.ForwardHost,
				"forward_port":   ph.ForwardPort,
				"ssl_forced":     ph.SSLForced,
				"websocket":      ph.WebsocketSupport,
			},
		}
	}
	return conflicts, details
}
