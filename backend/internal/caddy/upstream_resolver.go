package caddy

import (
	"errors"
	"net/netip"
	"strings"
)

// NameUpstreamWithoutResolver reports whether a decoded config has a reverse_proxy
// object whose transport cannot use the upstream resolver and whose upstreams or
// health-check upstream include a name. Such a name is resolved by a path the
// resolver never sees (HTTP/3, a network proxy, or a non-HTTP transport), so it is
// refused. Literal-only targets stay allowed with any transport.
func NameUpstreamWithoutResolver(node any) bool {
	found := false
	_ = walkObjects(node, func(obj map[string]any) error {
		if obj["handler"] != "reverse_proxy" || transportUsesResolver(obj) {
			return nil
		}
		if handlerHasNameTarget(obj) {
			found = true
			return errNameFound
		}
		return nil
	})
	return found
}

// errNameFound stops the walk once a name target is seen.
var errNameFound = errors.New("name target found")

// transportUsesResolver reports whether a reverse_proxy object's transport goes through
// the HTTP transport's dialer, which is the only dialer the upstream resolver reaches.
// A missing transport is the default HTTP transport.
func transportUsesResolver(handler map[string]any) bool {
	raw, present := handler["transport"]
	if !present || raw == nil {
		return true
	}
	transport, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	if protocol, _ := transport["protocol"].(string); protocol != "" && protocol != "http" {
		return false
	}
	if isSet(transport["network_proxy"]) || isSet(transport["forward_proxy_url"]) {
		return false
	}
	if versions, ok := transport["versions"].([]any); ok {
		for _, v := range versions {
			if v == "3" {
				return false
			}
		}
	}
	return true
}

// isSet reports whether a decoded optional value is present and not empty.
func isSet(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	default:
		return true
	}
}

// handlerHasNameTarget reports whether any upstream dial or the health-check upstream
// of a reverse_proxy object names a host. Dials that do not parse are not names here;
// they are refused by the dial checks.
func handlerHasNameTarget(handler map[string]any) bool {
	dials := upstreamDials(handler)
	if active, ok := healthCheckActive(handler); ok {
		if upstream, ok := active["upstream"].(string); ok && upstream != "" {
			dials = append(dials, upstream)
		}
	}
	for _, dial := range dials {
		target, err := ParseDial(dial)
		if err != nil {
			continue
		}
		if isNameHost(target.Host) {
			return true
		}
	}
	return false
}

// isNameHost reports whether a dial host is a name: it is not an IP literal. A zone
// suffix is removed before the literal check.
func isNameHost(host string) bool {
	addr, _, _ := strings.Cut(host, "%")
	_, err := netip.ParseAddr(addr)
	return err != nil
}
