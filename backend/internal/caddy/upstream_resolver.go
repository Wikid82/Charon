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
			return errStopWalk
		}
		return nil
	})
	return found
}

// errStopWalk stops the walk once a name target is seen.
var errStopWalk = errors.New("name target found")

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

// ErrUpstreamResolverUnusable means a reverse_proxy object cannot take the upstream
// resolver and names a target. Such a config cannot be loaded with its name checks in place.
var ErrUpstreamResolverUnusable = errors.New("upstream transport cannot use the upstream resolver")

// UpstreamResolver holds the loopback addresses of the upstream name answerer,
// e.g. "udp/127.0.0.1:41234". Handlers that connect to a reserved port use Reserved.
type UpstreamResolver struct {
	General  string
	Reserved string
}

// ReservedPortSource is optionally implemented by a ForwardTargetGuard. It returns the
// ports Charon uses for itself, read at the moment of the call.
type ReservedPortSource interface {
	ReservedPorts() (map[int]struct{}, error)
}

// ApplyUpstreamResolver sets the resolver on every reverse_proxy object under node, a
// decoded JSON document whose numbers are json.Number. A handler whose transport
// uses the resolver gets transport.resolver (a missing transport becomes the default
// HTTP transport). A handler on the reserved listener is one with any dial, or health
// check target, on a port in reserved, or with a dial that does not parse. A handler
// whose transport cannot use the resolver is left as it is, unless it names a target:
// that returns ErrUpstreamResolverUnusable.
//
// Caddy v2.11.9 modules/caddyhttp/reverseproxy/httptransport.go:258-274 reads the
// resolver option and dials the configured address for every name lookup.
func ApplyUpstreamResolver(node any, r UpstreamResolver, reserved map[int]struct{}) error {
	return walkObjects(node, func(obj map[string]any) error {
		if obj["handler"] != "reverse_proxy" {
			return nil
		}
		if !transportUsesResolver(obj) {
			if handlerHasNameTarget(obj) {
				return ErrUpstreamResolverUnusable
			}
			return nil
		}
		address := r.General
		if r.Reserved != "" && handlerUsesReservedPort(obj, reserved) {
			address = r.Reserved
		}
		resolver := map[string]any{"addresses": []any{address}}
		if raw, ok := obj["transport"].(map[string]any); ok {
			raw["resolver"] = resolver
			return nil
		}
		obj["transport"] = map[string]any{"protocol": "http", "resolver": resolver}
		return nil
	})
}

// handlerUsesReservedPort reports whether any dial the handler makes, including health
// checks, reaches a port in reserved. A dial that does not parse counts as reserved.
func handlerUsesReservedPort(handler map[string]any, reserved map[int]struct{}) bool {
	found := false
	_ = WalkAdvancedDials(handler, func(dial string) error {
		target, err := ParseDial(dial)
		if err != nil {
			found = true
			return errStopWalk
		}
		for port := range reserved {
			if port >= target.PortStart && port <= target.PortEnd {
				found = true
				return errStopWalk
			}
		}
		return nil
	})
	return found
}
