package caddy

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedirectHandler_Basic(t *testing.T) {
	h := RedirectHandler("https://example.com", 301)
	assert.Equal(t, "static_response", h["handler"])
	assert.Equal(t, 301, h["status_code"])
	headers, ok := h["headers"].(map[string]any)
	require.True(t, ok, "headers must be a flat map, not nested under response.set")
	assert.Equal(t, []string{"https://example.com"}, headers["Location"])
}

func TestRedirectHandler_PreservePathPlaceholder(t *testing.T) {
	h := RedirectHandler("https://example.com{http.request.uri}", 308)
	assert.Equal(t, 308, h["status_code"])
	headers := h["headers"].(map[string]any)
	assert.Equal(t, []string{"https://example.com{http.request.uri}"}, headers["Location"])
}

func TestDedupeAndTrackDomains_SingleDomain(t *testing.T) {
	processed := make(map[string]bool)
	domains, isIPOnly := dedupeAndTrackDomains("Example.com", processed, "redirection_host", "rh-1")
	assert.Equal(t, []string{"example.com"}, domains)
	assert.False(t, isIPOnly)
	assert.True(t, processed["example.com"])
}

func TestDedupeAndTrackDomains_MultiDomain(t *testing.T) {
	processed := make(map[string]bool)
	domains, isIPOnly := dedupeAndTrackDomains("a.example.com, B.example.com ,,c.example.com", processed, "proxy_host", "ph-1")
	assert.Equal(t, []string{"a.example.com", "b.example.com", "c.example.com"}, domains)
	assert.False(t, isIPOnly)
}

func TestDedupeAndTrackDomains_IPOnly(t *testing.T) {
	processed := make(map[string]bool)
	domains, isIPOnly := dedupeAndTrackDomains("192.168.1.1", processed, "proxy_host", "ph-1")
	assert.Equal(t, []string{"192.168.1.1"}, domains)
	assert.True(t, isIPOnly)
}

func TestDedupeAndTrackDomains_GhostHostSkipsAlreadyClaimed(t *testing.T) {
	processed := map[string]bool{"claimed.example.com": true}
	domains, _ := dedupeAndTrackDomains("claimed.example.com,fresh.example.com", processed, "redirection_host", "rh-2")
	assert.Equal(t, []string{"fresh.example.com"}, domains)
}

func TestBuildRedirectRoutes_Basic(t *testing.T) {
	hosts := []models.RedirectionHost{
		{
			UUID:        "rh-1",
			DomainNames: "old.example.com",
			TargetURL:   "https://new.example.com",
			StatusCode:  301,
			Enabled:     true,
		},
	}
	processed := make(map[string]bool)
	routes, ipSubjects := BuildRedirectRoutes(hosts, processed)
	require.Len(t, routes, 1)
	assert.Empty(t, ipSubjects)

	route := routes[0]
	assert.True(t, route.Terminal)
	require.Len(t, route.Match, 1)
	assert.Equal(t, []string{"old.example.com"}, route.Match[0].Host)
	require.Len(t, route.Handle, 1)
	assert.Equal(t, "static_response", route.Handle[0]["handler"])
	assert.Equal(t, 301, route.Handle[0]["status_code"])
	headers := route.Handle[0]["headers"].(map[string]any)
	assert.Equal(t, []string{"https://new.example.com"}, headers["Location"])
}

func TestBuildRedirectRoutes_PreservePathAppendsPlaceholder(t *testing.T) {
	hosts := []models.RedirectionHost{
		{
			UUID:         "rh-1",
			DomainNames:  "old.example.com",
			TargetURL:    "https://new.example.com",
			StatusCode:   302,
			PreservePath: true,
			Enabled:      true,
		},
	}
	routes, _ := BuildRedirectRoutes(hosts, make(map[string]bool))
	require.Len(t, routes, 1)
	headers := routes[0].Handle[0]["headers"].(map[string]any)
	assert.Equal(t, []string{"https://new.example.com{http.request.uri}"}, headers["Location"])
}

func TestBuildRedirectRoutes_PreservePathOffUsesVerbatimTarget(t *testing.T) {
	hosts := []models.RedirectionHost{
		{
			UUID:         "rh-1",
			DomainNames:  "old.example.com",
			TargetURL:    "https://new.example.com/landing",
			StatusCode:   302,
			PreservePath: false,
			Enabled:      true,
		},
	}
	routes, _ := BuildRedirectRoutes(hosts, make(map[string]bool))
	require.Len(t, routes, 1)
	headers := routes[0].Handle[0]["headers"].(map[string]any)
	assert.Equal(t, []string{"https://new.example.com/landing"}, headers["Location"])
}

func TestBuildRedirectRoutes_DisabledHostSkipped(t *testing.T) {
	hosts := []models.RedirectionHost{
		{UUID: "rh-1", DomainNames: "old.example.com", TargetURL: "https://new.example.com", StatusCode: 301, Enabled: false},
	}
	routes, _ := BuildRedirectRoutes(hosts, make(map[string]bool))
	assert.Empty(t, routes)
}

func TestBuildRedirectRoutes_EmptyDomainOrTargetSkipped(t *testing.T) {
	hosts := []models.RedirectionHost{
		{UUID: "rh-1", DomainNames: "", TargetURL: "https://new.example.com", StatusCode: 301, Enabled: true},
		{UUID: "rh-2", DomainNames: "old.example.com", TargetURL: "", StatusCode: 301, Enabled: true},
	}
	routes, _ := BuildRedirectRoutes(hosts, make(map[string]bool))
	assert.Empty(t, routes)
}

func TestBuildRedirectRoutes_IPOnlyDomainTracked(t *testing.T) {
	hosts := []models.RedirectionHost{
		{UUID: "rh-1", DomainNames: "192.168.1.50", TargetURL: "https://new.example.com", StatusCode: 301, Enabled: true},
	}
	routes, ipSubjects := BuildRedirectRoutes(hosts, make(map[string]bool))
	require.Len(t, routes, 1)
	assert.Equal(t, []string{"192.168.1.50"}, ipSubjects)
}

func TestBuildRedirectRoutes_HSTSHeaderPresent(t *testing.T) {
	hosts := []models.RedirectionHost{
		{
			UUID:           "rh-1",
			DomainNames:    "old.example.com",
			TargetURL:      "https://new.example.com",
			StatusCode:     301,
			Enabled:        true,
			HSTSEnabled:    true,
			HSTSSubdomains: true,
		},
	}
	routes, _ := BuildRedirectRoutes(hosts, make(map[string]bool))
	require.Len(t, routes, 1)
	require.Len(t, routes[0].Handle, 2)
	assert.Equal(t, "headers", routes[0].Handle[0]["handler"])
	response := routes[0].Handle[0]["response"].(map[string]any)
	setHeaders := response["set"].(map[string][]string)
	assert.Equal(t, []string{"max-age=31536000; includeSubDomains"}, setHeaders["Strict-Transport-Security"])
	assert.Equal(t, "static_response", routes[0].Handle[1]["handler"])
}

func TestBuildRedirectRoutes_HSTSHeaderAbsentByDefault(t *testing.T) {
	hosts := []models.RedirectionHost{
		{UUID: "rh-1", DomainNames: "old.example.com", TargetURL: "https://new.example.com", StatusCode: 301, Enabled: true, HSTSEnabled: false},
	}
	routes, _ := BuildRedirectRoutes(hosts, make(map[string]bool))
	require.Len(t, routes, 1)
	require.Len(t, routes[0].Handle, 1)
	assert.Equal(t, "static_response", routes[0].Handle[0]["handler"])
}

func TestBuildRedirectRoutes_GhostHostDedup_NewestWins(t *testing.T) {
	// BuildRedirectRoutes iterates newest-first (highest index first), so
	// when two hosts claim the same domain, the one later in the slice
	// (index 1, "newer") wins and the earlier one (index 0) is dropped.
	hosts := []models.RedirectionHost{
		{UUID: "rh-old", DomainNames: "dup.example.com", TargetURL: "https://old-target.example.com", StatusCode: 301, Enabled: true},
		{UUID: "rh-new", DomainNames: "dup.example.com", TargetURL: "https://new-target.example.com", StatusCode: 301, Enabled: true},
	}
	routes, _ := BuildRedirectRoutes(hosts, make(map[string]bool))
	require.Len(t, routes, 1)
	headers := routes[0].Handle[0]["headers"].(map[string]any)
	assert.Equal(t, []string{"https://new-target.example.com"}, headers["Location"])
}

func TestBuildRedirectRoutes_MultiDomainHost(t *testing.T) {
	hosts := []models.RedirectionHost{
		{UUID: "rh-1", DomainNames: "a.example.com,b.example.com", TargetURL: "https://new.example.com", StatusCode: 301, Enabled: true},
	}
	routes, _ := BuildRedirectRoutes(hosts, make(map[string]bool))
	require.Len(t, routes, 1)
	assert.Equal(t, []string{"a.example.com", "b.example.com"}, routes[0].Match[0].Host)
}
