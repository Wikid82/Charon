package orthrus

// Mirrored table: agent/muzzle/registry_test.go carries the same cases
// against the agent-side Filter. Keep the two tables identical.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

var registryLookupCases = []struct {
	ref     string
	allowed bool
}{
	// Allowed.
	{"nginx", true},
	{"library/nginx:1.27", true},
	{"ghcr.io/org/repo:tag", true},
	{"lscr.io/linuxserver/prowlarr", true},
	{"registry.example.com:5000/app@sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", true},
	{"intranet/x", true},

	// Refused: IP literals, localhost, single-label hosts, shorthand IPv4.
	{"127.0.0.1:5000/x", false},
	{"10.0.0.5/x", false},
	{"[::1]:5000/x", false},
	{"localhost/x", false},
	{"LOCALHOST:5000/x", false},
	{"foo.localhost/x", false},
	{"myregistry:5000/x", false},
	{"127.1:5000/x", false},
	{"10.1/x", false},
	{"0x7f.1/x", false},
	{"bad ref!/x", false},
}

func TestRegistryLookupAllowed(t *testing.T) {
	for _, tc := range registryLookupCases {
		t.Run(tc.ref, func(t *testing.T) {
			assert.Equal(t, tc.allowed, registryLookupAllowed(tc.ref))
		})
	}
}

func TestMuzzle_ServeHTTP_RegistryRuleReadOnly(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"loopback registry refused", "/distribution/127.0.0.1:5000/x/json", http.StatusForbidden},
		{"shorthand IPv4 refused", "/distribution/127.1:5000/x/json", http.StatusForbidden},
		{"localhost refused", "/distribution/localhost/x/json", http.StatusForbidden},
		{"versioned shorthand refused", "/v1.44/distribution/0x7f.1/x/json", http.StatusForbidden},
		{"Docker Hub short name forwarded", "/distribution/nginx/json", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMuzzle(passthroughHandler(), false, nil, nil, "agent-uuid")
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			m.ServeHTTP(rec, req)
			assert.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}

func TestMuzzle_ServeHTTP_RegistryRuleWriteEnabledPassesThrough(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), true, rate.NewLimiter(rate.Inf, 1), nil, "agent-uuid")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/distribution/127.0.0.1:5000/x/json", http.NoBody)
	m.ServeHTTP(rec, req)
	require.NotEqual(t, http.StatusForbidden, rec.Code)
}
