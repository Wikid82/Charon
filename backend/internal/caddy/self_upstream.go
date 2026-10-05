package caddy

import (
	"bytes"
	"net"
	"strconv"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

// selfHopConfig carries what GenerateConfig needs to mark requests whose
// upstream is Charon's own API.
type selfHopConfig struct {
	secret *selfhop.Secret
	// port is the port the API listens on.
	port  string
	local *selfhop.LocalAddrs
}

// isSelfUpstream reports whether dial points at this process's own API: a host
// naming this machine on the API's listen port. Upstreams marked remote (for
// example agent-resolved targets) never qualify.
func (s *selfHopConfig) isSelfUpstream(dial string, remote bool) bool {
	if s == nil || s.secret == nil || remote {
		return false
	}
	host, port, err := net.SplitHostPort(dial)
	if err != nil {
		return false
	}
	if p, err := strconv.Atoi(port); err != nil || strconv.Itoa(p) != s.port {
		return false
	}
	return s.local.IsLocalHost(host)
}

// selfHopHeaders are the request headers set on a self upstream. They are set
// (never added), so a client-supplied value cannot survive. The client address
// is the connecting peer as the proxy sees it; scheme and host are included so
// the API reads them from a proxy-set source.
func (s *selfHopConfig) selfHopHeaders() map[string][]string {
	return map[string][]string{
		selfhop.HeaderSecret: {s.secret.Reveal()},
		selfhop.HeaderClient: {selfhop.ClientPlaceholder},
		"X-Forwarded-Proto":  {"{http.request.scheme}"},
		"X-Forwarded-Host":   {"{http.request.host}"},
	}
}

// proxyHandler builds the reverse_proxy handler for dial, adding the hop headers
// only when the upstream is Charon itself.
func (o *generateConfigOptions) proxyHandler(dial string, remote, enableWS bool, application string, enableStandardHeaders bool) Handler {
	h := ReverseProxyHandler(dial, enableWS, application, enableStandardHeaders)
	if !o.selfHop.isSelfUpstream(dial, remote) {
		return h
	}
	headers, _ := h["headers"].(map[string]any)
	if headers == nil {
		headers = make(map[string]any)
		h["headers"] = headers
	}
	request, _ := headers["request"].(map[string]any)
	if request == nil {
		request = make(map[string]any)
		headers["request"] = request
	}
	set, _ := request["set"].(map[string][]string)
	if set == nil {
		set = make(map[string][]string)
		request["set"] = set
	}
	for k, v := range o.selfHop.selfHopHeaders() {
		set[k] = v
	}
	return h
}

// snapshotSecretPlaceholder stands in for the hop secret in this manager's snapshots.
const snapshotSecretPlaceholder = "CHARON_SELF_HOP_SECRET_REDACTED" //nolint:gosec // marker text, not a credential

// orthrusHostUUIDs returns the UUIDs of hosts whose upstream is an agent-resolved
// target. It must be called before the targets are rewritten to addresses.
func orthrusHostUUIDs(hosts []models.ProxyHost) map[string]struct{} {
	var out map[string]struct{}
	for _, h := range hosts {
		if strings.HasPrefix(h.ForwardHost, orthrusHostPrefix) {
			if out == nil {
				out = make(map[string]struct{})
			}
			out[h.UUID] = struct{}{}
		}
	}
	return out
}

// redactHopSecret replaces the live secret in a serialized snapshot. The secret
// changes at every restart, so a stored value would only go stale.
func (m *Manager) redactHopSecret(configJSON []byte) []byte {
	secret := m.hopSecret.Reveal()
	if secret == "" {
		return configJSON
	}
	return bytes.ReplaceAll(configJSON, []byte(secret), []byte(snapshotSecretPlaceholder))
}

// restoreHopSecret puts the live secret back into a snapshot read from disk.
func (m *Manager) restoreHopSecret(configJSON []byte) []byte {
	secret := m.hopSecret.Reveal()
	if secret == "" {
		return configJSON
	}
	return bytes.ReplaceAll(configJSON, []byte(snapshotSecretPlaceholder), []byte(secret))
}
