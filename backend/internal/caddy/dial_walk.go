package caddy

import (
	"encoding/json"
	"errors"
	"net"
	"strconv"
)

// WalkDials calls visit for every "dial" string in a decoded JSON document,
// wherever it sits. The walk stops at the first error that visit returns.
func WalkDials(node any, visit func(dial string) error) error {
	switch v := node.(type) {
	case map[string]any:
		if dial, ok := v["dial"].(string); ok {
			if err := visit(dial); err != nil {
				return err
			}
		}
		for _, child := range v {
			if err := WalkDials(child, visit); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := WalkDials(child, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// HasDynamicUpstreams reports whether a decoded config sets dynamic upstreams
// anywhere. Their addresses are chosen at run time, so they cannot be checked.
func HasDynamicUpstreams(node any) bool {
	return hasKey(node, "dynamic_upstreams")
}

// HasTransportResolver reports whether a decoded config sets a custom resolver
// anywhere. A resolver decides which addresses a name reaches, so it is refused.
func HasTransportResolver(node any) bool {
	return hasKey(node, "resolver")
}

// errKeyFound stops the walk once a key has been seen.
var errKeyFound = errors.New("key found")

// hasKey reports whether any object under node has the key.
func hasKey(node any, key string) bool {
	err := walkObjects(node, func(obj map[string]any) error {
		if _, ok := obj[key]; ok {
			return errKeyFound
		}
		return nil
	})
	return errors.Is(err, errKeyFound)
}

// walkObjects calls visit for every JSON object under node. It stops at the first error.
func walkObjects(node any, visit func(map[string]any) error) error {
	switch v := node.(type) {
	case map[string]any:
		if err := visit(v); err != nil {
			return err
		}
		for _, child := range v {
			if err := walkObjects(child, visit); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := walkObjects(child, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// WalkAdvancedDials calls visit for every dial that a decoded advanced
// configuration makes: each "dial" (WalkDials) and each active health check
// target (WalkHealthCheckDials).
func WalkAdvancedDials(node any, visit func(dial string) error) error {
	if err := WalkDials(node, visit); err != nil {
		return err
	}
	return WalkHealthCheckDials(node, visit)
}

// WalkHealthCheckDials calls visit for every dial an active health check connects to:
// health_checks.active.upstream as given, and health_checks.active.port joined with the
// host of every upstream dial of the same reverse_proxy object. Caddy ignores port when
// upstream is set; both are visited so the stricter check covers either form.
// A port that is not an integer in 1-65535 is skipped: Caddy rejects it at load.
func WalkHealthCheckDials(node any, visit func(dial string) error) error {
	return walkObjects(node, func(obj map[string]any) error {
		active, ok := healthCheckActive(obj)
		if !ok {
			return nil
		}
		if upstream, isString := active["upstream"].(string); isString && upstream != "" {
			if err := visit(upstream); err != nil {
				return err
			}
		}
		port, ok := healthCheckPort(active["port"])
		if !ok {
			return nil
		}
		for _, dial := range upstreamDials(obj) {
			target, err := ParseDial(dial)
			if err != nil {
				continue
			}
			if err := visit(net.JoinHostPort(target.Host, strconv.Itoa(port))); err != nil {
				return err
			}
		}
		return nil
	})
}

// healthCheckActive returns health_checks.active of a handler object.
func healthCheckActive(obj map[string]any) (map[string]any, bool) {
	checks, ok := obj["health_checks"].(map[string]any)
	if !ok {
		return nil, false
	}
	active, ok := checks["active"].(map[string]any)
	return active, ok
}

// healthCheckPort reads a decoded JSON port: a number or a numeric string.
func healthCheckPort(raw any) (int, bool) {
	var port int
	switch v := raw.(type) {
	case float64:
		if v != float64(int(v)) {
			return 0, false
		}
		port = int(v)
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false
		}
		port = int(n)
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		port = n
	default:
		return 0, false
	}
	return port, port >= 1 && port <= 65535
}

// upstreamDials returns the dial strings of a reverse_proxy object's upstreams, in
// either the generated or the decoded form.
func upstreamDials(obj map[string]any) []string {
	var dials []string
	add := func(upstream map[string]any) {
		if dial, ok := upstream["dial"].(string); ok {
			dials = append(dials, dial)
		}
	}
	switch ups := obj["upstreams"].(type) {
	case []any:
		for _, raw := range ups {
			if upstream, ok := raw.(map[string]any); ok {
				add(upstream)
			}
		}
	case []map[string]any:
		for _, upstream := range ups {
			add(upstream)
		}
	}
	return dials
}
