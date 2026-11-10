// Package caddymatchtest is a test-only model of Caddy's matcher and subroute semantics. Import only from _test.go files.
//
// Rules modelled (Caddy docs, http.handlers.subroute and http.matchers.not):
//   - A route's match list is OR'd: the route matches when any matcher set matches. An absent or empty match list
//     matches every request.
//   - Matchers inside one set are AND'd. An empty set matches every request.
//   - not takes a list of matcher sets and is true when none of them matches.
//   - Routes in a subroute run in order. A matching route runs its handlers in order; a matching terminal route ends
//     the subroute (the outer chain continues).
//   - static_response ends the request whether or not its route is terminal. It refuses the request when its status
//     is 403.
//   - reverse_proxy ends the request without refusing it.
//
// The package does not import caddy (that would be an import cycle for caddy's internal tests). Every entry point takes
// the JSON that json.Marshal produces for a *caddy.Config or a caddy.Handler, which is also what Caddy receives.
//
// The model is strict: any matcher, expression or handler it does not know fails the test, so a new Caddy shape must be
// classified on purpose before the tests accept it.
package caddymatchtest

import (
	"encoding/json"
	"net/netip"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Probe describes one request as Caddy sees it: the connection address and the country the strict geoip2 handler
// found for it. Country "" means the lookup found nothing (not found or lookup error).
type Probe struct {
	Addr    netip.Addr
	Country string
}

// countryExpression is the only expression form the charon configuration emits.
var countryExpression = regexp.MustCompile(`^\{geoip2\.country_code\} in \[\s*(?:"[A-Z]{2}"(?:\s*,\s*"[A-Z]{2}")*)?\s*\]$`)

// countryCode extracts each quoted code from a country expression.
var countryCode = regexp.MustCompile(`"([A-Z]{2})"`)

// passThroughHandlers do not decide a request on their own; the walk continues past them.
var passThroughHandlers = map[string]bool{
	"headers":    true,
	"waf":        true,
	"crowdsec":   true,
	"rate_limit": true,
	"geoip2":     true,
	"vars":       true,
}

// chain is the per-request state of a walk through one handler chain.
type chain struct {
	geoipPassed bool
}

// HostRefuses finds the host's catch-all route (a route whose only match is {host: [...domain...]} with no path) in
// every server, walks its handlers in order, and reports whether the request is answered 403 before reverse_proxy.
// It is fatal when no such route exists or more than one does, and when the walk meets an expression matcher before the
// geoip2 handler that fills its placeholder.
func HostRefuses(t testing.TB, configJSON []byte, domain string, p Probe) bool {
	t.Helper()
	route := hostCatchAllRoute(t, decodeJSON(t, configJSON), domain)
	_, refused := runHandlers(t, listOf(t, route["handle"], "handle"), p, &chain{})
	return refused
}

// HandlerRefuses evaluates one handler (normally an ACL or decisions subroute) on its own and reports whether it answers
// 403. The geoip2 ordering rule is not checked here; HostRefuses covers it.
func HandlerRefuses(t testing.TB, handlerJSON []byte, p Probe) bool {
	t.Helper()
	_, refused := runHandlers(t, []any{decodeJSON(t, handlerJSON)}, p, nil)
	return refused
}

// MatchesAny reports whether a route's match list matches the probe. An absent or empty list matches everything.
func MatchesAny(t testing.TB, match []any, p Probe) bool {
	t.Helper()
	return matchesAny(t, match, p, nil)
}

// runHandlers walks handlers in order. ended reports that a static_response or reverse_proxy finished the request;
// refused reports that the finishing response was a 403. c is nil when the chain is evaluated on its own.
func runHandlers(t testing.TB, handlers []any, p Probe, c *chain) (ended, refused bool) {
	t.Helper()
	for _, raw := range handlers {
		h := asMap(t, raw, "handler")
		name, _ := h["handler"].(string)
		switch {
		case name == "static_response":
			return true, statusCode(t, h) == 403
		case name == "reverse_proxy":
			return true, false
		case name == "subroute":
			if subEnded, subRefused := runSubroute(t, h, p, c); subEnded {
				return true, subRefused
			}
		case name == "geoip2":
			if c != nil {
				c.geoipPassed = true
			}
		case passThroughHandlers[name]:
			// Does not decide the request.
		default:
			t.Fatalf("caddymatchtest: handler %q is not classified; add it to the model on purpose", name)
		}
	}
	return false, false
}

// runSubroute runs the routes of one subroute in order. A matching terminal route stops the subroute only.
func runSubroute(t testing.TB, h map[string]any, p Probe, c *chain) (ended, refused bool) {
	t.Helper()
	for _, raw := range listOf(t, h["routes"], "routes") {
		r := asMap(t, raw, "route")
		if !matchesAny(t, listOf(t, r["match"], "match"), p, c) {
			continue
		}
		if subEnded, subRefused := runHandlers(t, listOf(t, r["handle"], "handle"), p, c); subEnded {
			return true, subRefused
		}
		if terminal, _ := r["terminal"].(bool); terminal {
			return false, false
		}
	}
	return false, false
}

// matchesAny reports whether a route's match list matches. An empty list means the route has no match and matches
// everything.
func matchesAny(t testing.TB, match []any, p Probe, c *chain) bool {
	t.Helper()
	if len(match) == 0 {
		return true
	}
	return anySetMatches(t, match, p, c)
}

// anySetMatches reports whether at least one set matches. An empty list matches nothing; this is what "not" needs.
func anySetMatches(t testing.TB, sets []any, p Probe, c *chain) bool {
	t.Helper()
	for _, raw := range sets {
		if matchSet(t, asMap(t, raw, "matcher set"), p, c) {
			return true
		}
	}
	return false
}

// matchSet reports whether every matcher in the set matches. Keys are validated first, in sorted order, so an unknown
// matcher fails the test whatever the other matchers return.
func matchSet(t testing.TB, set map[string]any, p Probe, c *chain) bool {
	t.Helper()
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		switch key {
		case "remote_ip", "not", "expression":
		default:
			t.Fatalf("caddymatchtest: matcher %q is not classified; add it to the model on purpose", key)
		}
	}
	matched := true
	for _, key := range keys {
		switch key {
		case "remote_ip":
			matched = remoteIPMatches(t, set[key], p) && matched
		case "not":
			matched = !anySetMatches(t, listOf(t, set[key], "not"), p, c) && matched
		case "expression":
			matched = expressionMatches(t, set[key], p, c) && matched
		}
	}
	return matched
}

// remoteIPMatches reports whether the probe address is inside one of the ranges. Only the ranges key is supported.
func remoteIPMatches(t testing.TB, raw any, p Probe) bool {
	t.Helper()
	m := asMap(t, raw, "remote_ip")
	if len(m) != 1 {
		t.Fatalf("caddymatchtest: remote_ip supports only the ranges key, got %d keys", len(m))
	}
	addr := p.Addr.Unmap()
	for _, rawRange := range listOf(t, m["ranges"], "remote_ip ranges") {
		if parsePrefix(t, rawRange).Contains(addr) {
			return true
		}
	}
	return false
}

// expressionMatches evaluates the only supported expression form: {geoip2.country_code} in ["XX", ...]. The probe
// country is compared with each listed code; an empty list never matches.
func expressionMatches(t testing.TB, raw any, p Probe, c *chain) bool {
	t.Helper()
	expr, ok := raw.(string)
	if !ok || !countryExpression.MatchString(expr) {
		t.Fatalf("caddymatchtest: expression %v is not classified; only the country list form is modelled", raw)
	}
	if c != nil && !c.geoipPassed {
		t.Fatalf("caddymatchtest: expression %q reached before the geoip2 handler; the placeholder would be empty", expr)
	}
	if p.Country == "" {
		return false
	}
	for _, m := range countryCode.FindAllStringSubmatch(expr, -1) {
		if m[1] == p.Country {
			return true
		}
	}
	return false
}

// hostCatchAllRoute returns the single route whose match is exactly {host: [...domain...]} with no path.
func hostCatchAllRoute(t testing.TB, cfg any, domain string) map[string]any {
	t.Helper()
	httpApp := asMap(t, asMap(t, cfg, "config")["apps"], "apps")
	http := asMap(t, httpApp["http"], "apps.http")
	servers := asMap(t, http["servers"], "servers")
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)

	var found []map[string]any
	for _, name := range names {
		server := asMap(t, servers[name], "server")
		for _, raw := range listOf(t, server["routes"], "routes") {
			route := asMap(t, raw, "route")
			if isHostCatchAll(t, route, domain) {
				found = append(found, route)
			}
		}
	}
	if len(found) != 1 {
		t.Fatalf("caddymatchtest: want exactly one catch-all route for %q, found %d", domain, len(found))
	}
	return found[0]
}

// isHostCatchAll reports whether the route's only match is a host matcher naming domain, with no path matcher.
func isHostCatchAll(t testing.TB, route map[string]any, domain string) bool {
	t.Helper()
	match := listOf(t, route["match"], "match")
	if len(match) != 1 {
		return false
	}
	item := asMap(t, match[0], "match item")
	if _, hasPath := item["path"]; hasPath {
		return false
	}
	for _, host := range listOf(t, item["host"], "host") {
		if s, _ := host.(string); s == domain {
			return true
		}
	}
	return false
}

// parsePrefix accepts a CIDR or a bare address (as a host-length prefix).
func parsePrefix(t testing.TB, raw any) netip.Prefix {
	t.Helper()
	s, ok := raw.(string)
	if !ok {
		t.Fatalf("caddymatchtest: range %v is not a string", raw)
	}
	if strings.Contains(s, "/") {
		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			t.Fatalf("caddymatchtest: range %q: %v", s, err)
		}
		return prefix
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("caddymatchtest: range %q: %v", s, err)
	}
	return netip.PrefixFrom(addr, addr.BitLen())
}

func statusCode(t testing.TB, h map[string]any) float64 {
	t.Helper()
	code, ok := h["status_code"].(float64)
	if !ok {
		t.Fatalf("caddymatchtest: static_response without a numeric status_code: %v", h["status_code"])
	}
	return code
}

func decodeJSON(t testing.TB, data []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("caddymatchtest: decode config JSON: %v", err)
	}
	return v
}

func asMap(t testing.TB, v any, what string) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("caddymatchtest: %s is %T, want an object", what, v)
	}
	return m
}

func listOf(t testing.TB, v any, what string) []any {
	t.Helper()
	if v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok {
		t.Fatalf("caddymatchtest: %s is %T, want an array", what, v)
	}
	return list
}
