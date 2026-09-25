package caddy

import (
	"net"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// dedupeAndTrackDomains parses a comma-separated domain list, trims and
// lowercases each entry, and skips any domain already claimed by a
// previously-processed host or redirection host (Ghost Host detection),
// recording newly-claimed domains into processedDomains as a side effect.
// resourceType/resourceID are used only to make the collision warning log
// identify which resource lost the domain race.
//
// Extracted from the domain-parse/dedupe/processedDomains-check block that
// used to be inlined in GenerateConfig's per-ProxyHost loop, so both that
// loop and BuildRedirectRoutes share identical dedup semantics instead of
// maintaining two copies.
func dedupeAndTrackDomains(domainNames string, processedDomains map[string]bool, resourceType, resourceID string) (uniqueDomains []string, isIPOnly bool) {
	isIPOnly = true
	rawDomains := strings.Split(domainNames, ",")
	for _, d := range rawDomains {
		d = strings.TrimSpace(d)
		d = strings.ToLower(d)
		if d == "" {
			continue
		}
		if processedDomains[d] {
			logger.Log().WithField("domain", d).WithField("resource_type", resourceType).WithField("resource_id", resourceID).Warn("Skipping duplicate domain (Ghost Host detection)")
			continue
		}
		processedDomains[d] = true
		uniqueDomains = append(uniqueDomains, d)
		if net.ParseIP(d) == nil {
			isIPOnly = false
		}
	}
	return uniqueDomains, isIPOnly
}

// BuildRedirectRoutes converts enabled RedirectionHosts into Caddy routes.
// Mirrors the domain-parsing/dedup conventions in GenerateConfig but emits a
// single static_response handler per host instead of a reverse_proxy chain,
// since a redirect has no backend to proxy to. See
// docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md §4.2.
func BuildRedirectRoutes(redirectHosts []models.RedirectionHost, processedDomains map[string]bool) ([]*Route, []string) {
	routes := make([]*Route, 0, len(redirectHosts))
	ipSubjects := make([]string, 0)

	for i := len(redirectHosts) - 1; i >= 0; i-- { // newest-first, same as GenerateConfig
		rh := redirectHosts[i] // #nosec G602 -- bounds checked by loop condition
		if !rh.Enabled || rh.DomainNames == "" || rh.TargetURL == "" {
			continue
		}

		uniqueDomains, isIPOnly := dedupeAndTrackDomains(rh.DomainNames, processedDomains, "redirection_host", rh.UUID)
		if len(uniqueDomains) == 0 {
			continue
		}
		if isIPOnly {
			ipSubjects = append(ipSubjects, uniqueDomains...)
		}

		location := rh.TargetURL
		if rh.PreservePath {
			location += "{http.request.uri}"
		}

		handlers := []Handler{}
		if rh.HSTSEnabled {
			hstsValue := "max-age=31536000"
			if rh.HSTSSubdomains {
				hstsValue += "; includeSubDomains"
			}
			handlers = append(handlers, HeaderHandler(map[string][]string{
				"Strict-Transport-Security": {hstsValue},
			}))
		}
		handlers = append(handlers, RedirectHandler(location, rh.StatusCode))

		routes = append(routes, &Route{
			Match:    []Match{{Host: uniqueDomains}},
			Handle:   handlers,
			Terminal: true,
		})
	}

	return routes, ipSubjects
}
