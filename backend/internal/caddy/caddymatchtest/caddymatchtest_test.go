package caddymatchtest

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeTB records a fatal failure instead of stopping the test. Fatalf ends the goroutine it runs on (as testing.T
// does), so expectFatal runs the evaluator on its own goroutine. Only Helper and Fatalf are ever called.
type fakeTB struct {
	testing.TB
	failed bool
	msg    string
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// expectFatal runs fn against a fake TB and returns the fatal message. It fails the test when fn does not fail.
func expectFatal(t *testing.T, fn func(tb testing.TB)) string {
	t.Helper()
	fake := &fakeTB{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(fake)
	}()
	<-done
	if !fake.failed {
		t.Fatalf("expected the model to reject the input, it accepted it")
	}
	return fake.msg
}

func addr(s string) netip.Addr { return netip.MustParseAddr(s) }

// hostConfig builds a configuration JSON with one host route for app.example.com whose handlers are given.
func hostConfig(t *testing.T, handlers ...map[string]any) []byte {
	t.Helper()
	cfg := map[string]any{
		"apps": map[string]any{
			"http": map[string]any{
				"servers": map[string]any{
					"srv": map[string]any{
						"routes": []any{
							map[string]any{
								"match":    []any{map[string]any{"host": []any{"app.example.com"}}},
								"handle":   anyList(handlers),
								"terminal": true,
							},
						},
					},
				},
			},
		},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return b
}

func anyList(maps []map[string]any) []any {
	out := make([]any, 0, len(maps))
	for _, m := range maps {
		out = append(out, m)
	}
	return out
}

func handlerJSON(t *testing.T, h map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal handler: %v", err)
	}
	return b
}

// deny403 is a static 403 response.
func deny403() map[string]any {
	return map[string]any{"handler": "static_response", "status_code": 403, "body": "denied"}
}

// denySubroute is a subroute with one terminal route that answers 403 when its match list matches.
func denySubroute(match []any) map[string]any {
	return map[string]any{
		"handler": "subroute",
		"routes": []any{
			map[string]any{
				"match":    match,
				"handle":   []any{deny403()},
				"terminal": true,
			},
		},
	}
}

func remoteIP(ranges ...string) map[string]any {
	list := make([]any, 0, len(ranges))
	for _, r := range ranges {
		list = append(list, r)
	}
	return map[string]any{"remote_ip": map[string]any{"ranges": list}}
}

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		name    string
		match   []any
		probe   Probe
		matches bool
	}{
		{name: "absent match list matches everything", match: nil, probe: Probe{Addr: addr("203.0.113.9")}, matches: true},
		{name: "empty set matches everything", match: []any{map[string]any{}}, probe: Probe{Addr: addr("203.0.113.9")}, matches: true},
		{name: "remote_ip in range", match: []any{remoteIP("203.0.113.0/24")}, probe: Probe{Addr: addr("203.0.113.9")}, matches: true},
		{name: "remote_ip outside range", match: []any{remoteIP("203.0.113.0/24")}, probe: Probe{Addr: addr("198.51.100.1")}, matches: false},
		{name: "bare address is a host prefix", match: []any{remoteIP("198.51.100.7")}, probe: Probe{Addr: addr("198.51.100.7")}, matches: true},
		{name: "bare address does not match its neighbour", match: []any{remoteIP("198.51.100.7")}, probe: Probe{Addr: addr("198.51.100.8")}, matches: false},
		{name: "IPv6 range", match: []any{remoteIP("2001:db8::/32")}, probe: Probe{Addr: addr("2001:db8::1")}, matches: true},
		{name: "IPv4-mapped address matches IPv4 range", match: []any{remoteIP("10.0.0.0/8")}, probe: Probe{Addr: addr("::ffff:10.1.1.1")}, matches: true},
		{name: "IPv4 range does not match IPv6 address", match: []any{remoteIP("10.0.0.0/8")}, probe: Probe{Addr: addr("2001:db8::1")}, matches: false},
		{name: "invalid probe address matches no range", match: []any{remoteIP("0.0.0.0/0")}, probe: Probe{}, matches: false},
		{
			name:    "sets are OR'd",
			match:   []any{remoteIP("1.2.3.4"), remoteIP("203.0.113.0/24")},
			probe:   Probe{Addr: addr("203.0.113.9")},
			matches: true,
		},
		{
			name:    "matchers inside a set are AND'd",
			match:   []any{map[string]any{"remote_ip": map[string]any{"ranges": []any{"203.0.113.0/24"}}, "not": []any{remoteIP("203.0.113.9")}}},
			probe:   Probe{Addr: addr("203.0.113.9")},
			matches: false,
		},
		{
			name:    "not is true when none of its sets matches",
			match:   []any{map[string]any{"not": []any{remoteIP("10.0.0.0/8"), remoteIP("192.168.0.0/16")}}},
			probe:   Probe{Addr: addr("203.0.113.9")},
			matches: true,
		},
		{
			name:    "not is false when one of its sets matches",
			match:   []any{map[string]any{"not": []any{remoteIP("10.0.0.0/8"), remoteIP("203.0.113.0/24")}}},
			probe:   Probe{Addr: addr("203.0.113.9")},
			matches: false,
		},
		{
			name:    "empty not is true",
			match:   []any{map[string]any{"not": []any{}}},
			probe:   Probe{Addr: addr("203.0.113.9")},
			matches: true,
		},
		{
			name:    "country in list",
			match:   []any{map[string]any{"expression": `{geoip2.country_code} in ["DE", "US"]`}},
			probe:   Probe{Addr: addr("198.51.100.7"), Country: "US"},
			matches: true,
		},
		{
			name:    "country not in list",
			match:   []any{map[string]any{"expression": `{geoip2.country_code} in ["DE"]`}},
			probe:   Probe{Addr: addr("198.51.100.7"), Country: "US"},
			matches: false,
		},
		{
			name:    "empty country never matches a list",
			match:   []any{map[string]any{"expression": `{geoip2.country_code} in ["DE"]`}},
			probe:   Probe{Addr: addr("198.51.100.7")},
			matches: false,
		},
		{
			name:    "in [] never matches",
			match:   []any{map[string]any{"expression": `{geoip2.country_code} in []`}},
			probe:   Probe{Addr: addr("198.51.100.7"), Country: "US"},
			matches: false,
		},
		{
			name:    "negated in [] matches everything",
			match:   []any{map[string]any{"not": []any{map[string]any{"expression": `{geoip2.country_code} in []`}}}},
			probe:   Probe{Addr: addr("198.51.100.7"), Country: "US"},
			matches: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesAny(t, tt.match, tt.probe); got != tt.matches {
				t.Fatalf("MatchesAny = %v, want %v", got, tt.matches)
			}
		})
	}
}

func TestHostRefuses(t *testing.T) {
	allowed := map[string]any{"handler": "reverse_proxy"}
	tests := []struct {
		name     string
		handlers []map[string]any
		probe    Probe
		refused  bool
	}{
		{
			name:     "pass-through handlers then reverse_proxy allow",
			handlers: []map[string]any{{"handler": "headers"}, {"handler": "crowdsec"}, {"handler": "waf"}, {"handler": "rate_limit"}, {"handler": "vars"}, allowed},
			probe:    Probe{Addr: addr("203.0.113.9")},
			refused:  false,
		},
		{
			name:     "subroute that refuses this address",
			handlers: []map[string]any{denySubroute([]any{remoteIP("203.0.113.0/24")}), allowed},
			probe:    Probe{Addr: addr("203.0.113.9")},
			refused:  true,
		},
		{
			name:     "subroute that does not match lets the chain continue",
			handlers: []map[string]any{denySubroute([]any{remoteIP("203.0.113.0/24")}), allowed},
			probe:    Probe{Addr: addr("198.51.100.1")},
			refused:  false,
		},
		{
			name: "non-terminal 403 route still refuses",
			handlers: []map[string]any{{
				"handler": "subroute",
				"routes": []any{map[string]any{
					"match":  []any{remoteIP("203.0.113.0/24")},
					"handle": []any{deny403()},
				}},
			}, allowed},
			probe:   Probe{Addr: addr("203.0.113.9")},
			refused: true,
		},
		{
			name: "static_response with a non-403 status ends the chain without refusing",
			handlers: []map[string]any{
				{"handler": "static_response", "status_code": 200},
				allowed,
			},
			probe:   Probe{Addr: addr("203.0.113.9")},
			refused: false,
		},
		{
			name: "matching terminal route stops only its subroute",
			handlers: []map[string]any{
				{
					"handler": "subroute",
					"routes": []any{
						map[string]any{"match": []any{remoteIP("203.0.113.0/24")}, "handle": []any{map[string]any{"handler": "vars"}}, "terminal": true},
						map[string]any{"handle": []any{deny403()}},
					},
				},
				allowed,
			},
			probe:   Probe{Addr: addr("203.0.113.9")},
			refused: false,
		},
		{
			name: "geoip2 precedes the country list; a listed country is refused",
			handlers: []map[string]any{
				{"handler": "geoip2", "enable": "strict"},
				denySubroute([]any{map[string]any{"expression": `{geoip2.country_code} in ["RU"]`}}),
				allowed,
			},
			probe:   Probe{Addr: addr("198.51.100.7"), Country: "RU"},
			refused: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HostRefuses(t, hostConfig(t, tt.handlers...), "app.example.com", tt.probe)
			if got != tt.refused {
				t.Fatalf("HostRefuses = %v, want %v", got, tt.refused)
			}
		})
	}
}

func TestHandlerRefuses(t *testing.T) {
	block := denySubroute([]any{remoteIP("1.2.3.4")})
	if !HandlerRefuses(t, handlerJSON(t, block), Probe{Addr: addr("1.2.3.4")}) {
		t.Fatalf("listed address must be refused")
	}
	if HandlerRefuses(t, handlerJSON(t, block), Probe{Addr: addr("203.0.113.9")}) {
		t.Fatalf("unlisted address must not be refused")
	}
}

// oldTwoSetBlockList is the shape that refused every non-admin visitor (F9): the block set and the admin exclusion are
// two sets, which Caddy ORs.
func oldTwoSetBlockList() map[string]any {
	return denySubroute([]any{
		remoteIP("1.2.3.0/24"),
		map[string]any{"not": []any{remoteIP("10.0.0.0/8")}},
	})
}

// newOneSetBlockList is the shape after the fix: one set, the admin exclusion inside its not.
func newOneSetBlockList() map[string]any {
	return denySubroute([]any{
		map[string]any{
			"remote_ip": map[string]any{"ranges": []any{"1.2.3.0/24"}},
			"not":       []any{remoteIP("10.0.0.0/8")},
		},
	})
}

// TestOldTwoSetShapeIsCaught proves the model would fail a semantic table on the F9 shape: the unlisted address is
// refused by the old shape, so a table that expects it to be allowed fails.
func TestOldTwoSetShapeIsCaught(t *testing.T) {
	unlisted := Probe{Addr: addr("203.0.113.9")}
	if !HandlerRefuses(t, handlerJSON(t, oldTwoSetBlockList()), unlisted) {
		t.Fatalf("old two-set shape must refuse an unlisted address; the self-test would not catch it")
	}
	if HandlerRefuses(t, handlerJSON(t, newOneSetBlockList()), unlisted) {
		t.Fatalf("one-set shape must allow an unlisted address")
	}
	if HandlerRefuses(t, handlerJSON(t, newOneSetBlockList()), Probe{Addr: addr("10.1.1.1")}) {
		t.Fatalf("one-set shape must allow an admin address")
	}
	if !HandlerRefuses(t, handlerJSON(t, newOneSetBlockList()), Probe{Addr: addr("1.2.3.4")}) {
		t.Fatalf("one-set shape must refuse a listed non-admin address")
	}
	// The same unlisted probe must also be caught through a whole host chain.
	host := hostConfig(t, oldTwoSetBlockList(), map[string]any{"handler": "reverse_proxy"})
	if !HostRefuses(t, host, "app.example.com", unlisted) {
		t.Fatalf("old two-set shape must refuse through HostRefuses as well")
	}
}

func TestHostRefusesFailsOnUnknownShapes(t *testing.T) {
	tests := []struct {
		name    string
		config  []byte
		domain  string
		probe   Probe
		message string
	}{
		{
			name:    "unknown matcher",
			config:  hostConfig(t, denySubroute([]any{map[string]any{"path": []any{"/x"}}}), map[string]any{"handler": "reverse_proxy"}),
			domain:  "app.example.com",
			message: "matcher \"path\"",
		},
		{
			name:    "unknown expression",
			config:  hostConfig(t, denySubroute([]any{map[string]any{"expression": `{http.request.uri} == "/"`}}), map[string]any{"handler": "reverse_proxy"}),
			domain:  "app.example.com",
			message: "not classified",
		},
		{
			name:    "unknown handler",
			config:  hostConfig(t, map[string]any{"handler": "encode"}, map[string]any{"handler": "reverse_proxy"}),
			domain:  "app.example.com",
			message: "handler \"encode\"",
		},
		{
			name:    "unknown key on remote_ip",
			config:  hostConfig(t, denySubroute([]any{map[string]any{"remote_ip": map[string]any{"ranges": []any{"1.2.3.4"}, "forwarded": true}}}), map[string]any{"handler": "reverse_proxy"}),
			domain:  "app.example.com",
			message: "ranges key",
		},
		{
			name:    "no catch-all route for the domain",
			config:  hostConfig(t, map[string]any{"handler": "reverse_proxy"}),
			domain:  "other.example.com",
			message: "exactly one catch-all route",
		},
		{
			name: "two catch-all routes for the domain",
			config: func() []byte {
				route := map[string]any{
					"match":  []any{map[string]any{"host": []any{"app.example.com"}}},
					"handle": []any{map[string]any{"handler": "reverse_proxy"}},
				}
				cfg := map[string]any{"apps": map[string]any{"http": map[string]any{"servers": map[string]any{
					"a": map[string]any{"routes": []any{route}},
					"b": map[string]any{"routes": []any{route}},
				}}}}
				b, _ := json.Marshal(cfg)
				return b
			}(),
			domain:  "app.example.com",
			message: "found 2",
		},
		{
			name: "expression reached before geoip2",
			config: hostConfig(t,
				denySubroute([]any{map[string]any{"expression": `{geoip2.country_code} in ["RU"]`}}),
				map[string]any{"handler": "geoip2", "enable": "strict"},
				map[string]any{"handler": "reverse_proxy"},
			),
			domain:  "app.example.com",
			message: "before the geoip2 handler",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := expectFatal(t, func(tb testing.TB) {
				HostRefuses(tb, tt.config, tt.domain, Probe{Addr: addr("198.51.100.7"), Country: "RU"})
			})
			if !strings.Contains(msg, tt.message) {
				t.Fatalf("fatal message %q does not contain %q", msg, tt.message)
			}
		})
	}
}

func TestHandlerRefusesFailsOnUnknownHandler(t *testing.T) {
	msg := expectFatal(t, func(tb testing.TB) {
		HandlerRefuses(tb, []byte(`{"handler":"rewrite"}`), Probe{Addr: addr("1.2.3.4")})
	})
	if !strings.Contains(msg, "handler \"rewrite\"") {
		t.Fatalf("unexpected fatal message %q", msg)
	}
}

// TestHelperGuardsFailOnMalformedInput checks that each helper stops the test on input it cannot model and accepts
// the shapes Caddy emits.
func TestHelperGuardsFailOnMalformedInput(t *testing.T) {
	const wantFatal = "caddymatchtest"
	fatalCases := map[string]func(tb testing.TB){
		"isHostCatchAll: match item is not an object": func(tb testing.TB) {
			isHostCatchAll(tb, map[string]any{"match": []any{"bad"}}, "app.example.com")
		},
		"isHostCatchAll: host list is not an array": func(tb testing.TB) {
			isHostCatchAll(tb, map[string]any{"match": []any{map[string]any{"host": "app.example.com"}}}, "app.example.com")
		},
		"parsePrefix: non-string range": func(tb testing.TB) {
			parsePrefix(tb, 42)
		},
		"parsePrefix: malformed CIDR": func(tb testing.TB) {
			parsePrefix(tb, "192.0.2.0/99")
		},
		"parsePrefix: malformed bare address": func(tb testing.TB) {
			parsePrefix(tb, "not-an-address")
		},
		"statusCode: missing status": func(tb testing.TB) {
			statusCode(tb, map[string]any{})
		},
		"decodeJSON: invalid JSON": func(tb testing.TB) {
			decodeJSON(tb, []byte("{"))
		},
		"asMap: not an object": func(tb testing.TB) {
			asMap(tb, []any{}, "value")
		},
		"listOf: not an array": func(tb testing.TB) {
			listOf(tb, "text", "value")
		},
	}
	for name, fn := range fatalCases {
		t.Run(name, func(t *testing.T) {
			msg := expectFatal(t, fn)
			assert.Contains(t, msg, wantFatal)
		})
	}

	t.Run("accepts well-formed shapes", func(t *testing.T) {
		assert.True(t, isHostCatchAll(t, map[string]any{"match": []any{map[string]any{"host": []any{"app.example.com"}}}}, "app.example.com"))
		assert.False(t, isHostCatchAll(t, map[string]any{"match": []any{map[string]any{"host": []any{42}}}}, "app.example.com"))
		assert.False(t, isHostCatchAll(t, map[string]any{"match": []any{map[string]any{"host": []any{"app.example.com"}}, map[string]any{}}}, "app.example.com"))
		assert.False(t, isHostCatchAll(t, map[string]any{"match": []any{map[string]any{"host": []any{"app.example.com"}, "path": []any{"/x"}}}}, "app.example.com"))
		assert.Equal(t, netip.MustParsePrefix("192.0.2.0/24"), parsePrefix(t, "192.0.2.0/24"))
		assert.Equal(t, netip.MustParsePrefix("192.0.2.1/32"), parsePrefix(t, "192.0.2.1"))
		assert.Equal(t, float64(403), statusCode(t, map[string]any{"status_code": float64(403)}))
		assert.Equal(t, map[string]any{"a": "b"}, decodeJSON(t, []byte(`{"a":"b"}`)))
		assert.Equal(t, map[string]any{}, asMap(t, map[string]any{}, "value"))
		assert.Nil(t, listOf(t, nil, "value"))
		assert.Equal(t, []any{"x"}, listOf(t, []any{"x"}, "value"))
	})
}
