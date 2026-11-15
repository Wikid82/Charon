package caddy

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

// placeholderToken matches a Caddy placeholder such as {http.vars.client_ip}. JSON objects always contain quotes, so
// they never match.
var placeholderToken = regexp.MustCompile(`\{[A-Za-z][A-Za-z0-9_.-]*\}`)

// knownPlaceholders are the Caddy placeholders the generator may emit, reviewed once. Each one is either a request
// value Caddy fills per request or a variable a handler sets. Anything else fails TestGeneratedPlaceholdersAreKnown,
// so a new placeholder is reviewed before it ships.
var knownPlaceholders = map[string]bool{
	"http.vars.client_ip":      true, // the visitor address Caddy decided on (every proxy check and forwarded client header)
	"http.request.remote.host": true, // the TCP peer, sent on the self-proxy hop only
	"http.request.host":        true,
	"http.request.scheme":      true,
	"http.request.uri":         true,
	"http.request.local.port":  true,
	"geoip2.country_code":      true, // set by the geoip2 handler for country lists
}

// knownPlaceholderPrefixes cover families of request values: headers the proxy forwards by name.
var knownPlaceholderPrefixes = []string{
	"http.request.header.", // the self-proxy X-Forwarded-For hop header and the application header passthroughs
}

// unknownPlaceholders returns each unknown placeholder in the JSON text with the path of its string.
func unknownPlaceholders(t *testing.T, raw []byte) []string {
	t.Helper()
	var doc any
	require.NoError(t, json.Unmarshal(raw, &doc))
	var found []string
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch node := v.(type) {
		case map[string]any:
			for k, child := range node {
				walk(path+"."+k, child)
			}
		case []any:
			for i, child := range node {
				walk(path+"["+strconv.Itoa(i)+"]", child)
			}
		case string:
			for _, token := range placeholderToken.FindAllString(node, -1) {
				name := strings.Trim(token, "{}")
				if knownPlaceholders[name] || hasKnownPrefix(name) {
					continue
				}
				found = append(found, path+": "+token)
			}
		}
	}
	walk("", doc)
	sort.Strings(found)
	return found
}

func hasKnownPrefix(name string) bool {
	for _, prefix := range knownPlaceholderPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// featureRichConfig generates a configuration with every feature that emits placeholders: every access-list type,
// the rate limit with a bypass list, a self-proxy host with the standard and application headers, a WebSocket host,
// CrowdSec and the WAF.
func featureRichConfig(t *testing.T) []byte {
	t.Helper()
	secret, err := selfhop.NewSecret()
	require.NoError(t, err)

	lists := []*models.AccessList{
		{Name: "allow", Type: "whitelist", IPRules: `[{"cidr":"198.51.100.0/24"}]`, Enabled: true},
		{Name: "block", Type: "blacklist", IPRules: `[{"cidr":"203.0.113.0/24"}]`, Enabled: true},
		{Name: "local", Type: "whitelist", LocalNetworkOnly: true, Enabled: true},
		{Name: "geo", Type: "geo_blacklist", CountryCodes: "RU", Enabled: true},
	}
	var hosts []models.ProxyHost
	for i, list := range lists {
		id := uint(i + 1)
		hosts = append(hosts, models.ProxyHost{
			UUID:             "acl-" + list.Name,
			DomainNames:      list.Name + ".example.com",
			ForwardScheme:    "http",
			ForwardHost:      "app.internal",
			ForwardPort:      8000,
			Enabled:          true,
			AccessListID:     &id,
			AccessList:       list,
			WebsocketSupport: true,
		})
	}
	hosts = append(hosts, models.ProxyHost{
		UUID:          "self",
		DomainNames:   "self.example.com",
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		Enabled:       true,
		Application:   "plex",
	}, models.ProxyHost{
		UUID:        "plain",
		DomainNames: "plain.example.com",
		ForwardHost: "app.internal",
		ForwardPort: 9000,
		Enabled:     true,
	})

	sec := &models.SecurityConfig{
		CrowdSecMode:        "local",
		CrowdSecAPIURL:      "http://cs.local",
		RateLimitRequests:   10,
		RateLimitWindowSec:  60,
		RateLimitBypassList: "10.0.0.0/8",
	}
	cfg, err := GenerateConfig(hosts, "/data/caddy/data", "", "/frontend/dist", "", false,
		true, true, true, true, "203.0.113.10", nil, nil, nil, sec, nil,
		WithSelfHop(secret, "8080"), WithGeoIPDatabase("/data/geoip/GeoLite2-Country.mmdb", true),
		WithBouncerKeyPath(t.TempDir()+"/bouncer_key"))
	require.NoError(t, err)
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	return raw
}

// TestGeneratedPlaceholdersAreKnown: every placeholder in a feature-rich generated configuration is one of the reviewed
// names, and the names that the proxy depends on are present.
func TestGeneratedPlaceholdersAreKnown(t *testing.T) {
	raw := featureRichConfig(t)
	assert.Empty(t, unknownPlaceholders(t, raw), "unknown placeholders in the generated configuration")

	for _, must := range []string{"{http.vars.client_ip}", "{http.request.remote.host}", "{http.request.header.X-Forwarded-For}", "{geoip2.country_code}"} {
		assert.Contains(t, string(raw), must)
	}
	assert.NotContains(t, string(raw), "{http.request.client_ip}", "the placeholder for the client address is http.vars.client_ip")
	assert.NotContains(t, string(raw), `"remote_ip"`, "proxy checks judge the client_ip matcher")
}

// TestUnknownPlaceholdersAreReported: a placeholder outside the allowlist, including the non-existent request form, is
// reported with the path of the string that carries it.
func TestUnknownPlaceholdersAreReported(t *testing.T) {
	raw := []byte(`{"apps":{"http":{"servers":{"s":{"routes":[{"handle":[{"handler":"headers","request":{"set":{"X-Real-IP":["{http.request.client_ip}"]}}}]}]}}}}}`)
	got := unknownPlaceholders(t, raw)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "{http.request.client_ip}")
	assert.Contains(t, got[0], "X-Real-IP")
}

func TestUnknownPlaceholdersAcceptsKnownNames(t *testing.T) {
	raw := []byte(`{"a":"{http.vars.client_ip}","b":["{http.request.header.X-Plex-Token}"],"c":"{geoip2.country_code}"}`)
	assert.Empty(t, unknownPlaceholders(t, raw))
}
