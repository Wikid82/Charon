package muzzle_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/agent/muzzle"
)

func TestFilter_Allow(t *testing.T) {
	f := muzzle.New()

	tests := []struct {
		method  string
		reqPath string
		allowed bool
	}{
		// Allowed: health check endpoints
		{"GET", "/_ping", true},
		{"GET", "/v1.44/_ping", true},
		{"HEAD", "/_ping", true},                  // HEAD allowed for ping (Docker SDK connectivity check)
		{"HEAD", "/v1.47/_ping", true},            // HEAD allowed for versioned ping
		{"HEAD", "/containers/json", false},       // HEAD blocked for non-ping paths
		{"HEAD", "/v1.47/containers/json", false}, // HEAD blocked for non-ping paths

		// Allowed: read-only GET endpoints
		{"GET", "/v1.41/containers/json", true},
		{"GET", "/v1.41/info", true},
		{"GET", "/v1.41/version", true},
		{"GET", "/v1.41/images/json", true},
		{"GET", "/v1.41/events", true},
		{"GET", "/v1.44/containers/abc123/json", true},
		{"GET", "/v1.24/containers/json", true},

		// Allowed: volumes and networks (read-only listing and inspection)
		{"GET", "/v1.41/volumes", true},
		{"GET", "/v1.41/volumes/myvol", true},
		{"GET", "/v1.41/networks", true},
		{"GET", "/v1.41/networks/mynet", true},

		// Blocked: mutating methods
		{"POST", "/v1.41/containers/create", false},
		{"DELETE", "/v1.41/containers/abc", false},
		{"PUT", "/v1.41/networks/abc", false},
		{"PATCH", "/v1.41/containers/abc/update", false},

		// Blocked: paths not on allowlist
		{"GET", "/v1.41/exec/abc/start", false},
		{"GET", "/v1.41/containers/prune", false},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.reqPath, func(t *testing.T) {
			got := f.Allow(tt.method, tt.reqPath)
			assert.Equal(t, tt.allowed, got)
		})
	}
}

func TestFilter_ServeProxy_Blocked_POST(t *testing.T) {
	f := muzzle.New()

	reqStr := "POST /v1.41/containers/create HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\n\r\n"
	var buf bytes.Buffer

	err := f.ServeProxy("/tmp/nonexistent.sock", strings.NewReader(reqStr), &buf)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "403")
}

func TestFilter_ServeProxy_Blocked_DELETE(t *testing.T) {
	f := muzzle.New()

	reqStr := "DELETE /v1.41/containers/abc HTTP/1.1\r\nHost: localhost\r\n\r\n"
	var buf bytes.Buffer

	err := f.ServeProxy("/tmp/nonexistent.sock", strings.NewReader(reqStr), &buf)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "403")
}

func TestFilter_ServeProxy_Blocked_PUT(t *testing.T) {
	f := muzzle.New()

	reqStr := "PUT /v1.41/networks/abc HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\n\r\n"
	var buf bytes.Buffer

	err := f.ServeProxy("/tmp/nonexistent.sock", strings.NewReader(reqStr), &buf)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "403")
}
