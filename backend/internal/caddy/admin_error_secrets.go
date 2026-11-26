package caddy

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/Wikid82/charon/backend/internal/redact"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

// maxSecretWalkNodes bounds how many JSON values configSecretValues visits. A generated config is far
// smaller. Past the cap the pattern pass of redact.Text is the only protection for the rest of the text.
const maxSecretWalkNodes = 200_000

// sensitiveHeaderNames are the request headers whose values are credentials (compared case-insensitively).
var sensitiveHeaderNames = []string{"Authorization", "Proxy-Authorization", "Cookie", "X-Api-Key", selfhop.HeaderSecret}

// configSecretValues returns the credential values of a JSON config that was sent to Caddy: every string under a
// credential key name, every string of a DNS provider object except its name, every string under a sensitive
// header name, and the password of every URL with user info. A body that is not JSON yields none.
func configSecretValues(body []byte) []string {
	if len(body) == 0 {
		return nil
	}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil
	}
	c := &secretCollector{found: map[string]struct{}{}}
	c.walk(doc, "", false, false)
	return slices.Sorted(maps.Keys(c.found))
}

// secretCollector gathers the credential values of one decoded config.
type secretCollector struct {
	found map[string]struct{}
	nodes int
}

// walk visits v, the value under key. credential marks a value below a credential key; provider marks a value
// below a DNS provider object.
func (c *secretCollector) walk(v any, key string, credential, provider bool) {
	if c.nodes >= maxSecretWalkNodes {
		return
	}
	c.nodes++
	switch t := v.(type) {
	case map[string]any:
		inProvider := provider || (strings.EqualFold(key, "provider") && hasStringName(t))
		for k, child := range t {
			childCredential := credential || redact.IsCredentialKeyName(k) || isSensitiveHeader(k) || (inProvider && k != "name")
			c.walk(child, k, childCredential, inProvider)
		}
	case []any:
		for _, child := range t {
			c.walk(child, key, credential, provider)
		}
	case string:
		if credential && t != "" {
			c.found[t] = struct{}{}
		}
		if _, password, ok := redact.URLPassword(t, redact.Placeholder, nil); ok {
			c.found[password] = struct{}{}
		}
	}
}

// hasStringName reports whether a DNS provider object names its provider.
func hasStringName(m map[string]any) bool {
	_, ok := m["name"].(string)
	return ok
}

// isSensitiveHeader reports whether a key is the name of a sensitive request header.
func isSensitiveHeader(key string) bool {
	for _, h := range sensitiveHeaderNames {
		if strings.EqualFold(key, h) {
			return true
		}
	}
	return false
}
