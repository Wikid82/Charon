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
		// --- Allowed: health check ---
		{"GET", "/_ping", true},
		{"GET", "/v1.44/_ping", true},
		{"HEAD", "/_ping", true},
		{"HEAD", "/v1.47/_ping", true},
		{"HEAD", "/containers/json", false},       // HEAD blocked for non-ping paths
		{"HEAD", "/v1.47/containers/json", false}, // HEAD blocked for non-ping paths

		// --- Allowed: GET on versioned paths ---
		{"GET", "/v1.47/containers/json", true},
		{"GET", "/v1.24/containers/json", true},
		{"GET", "/v1.47/containers/abc123/json", true},
		{"GET", "/v1.47/containers/abc123/logs", true},
		{"GET", "/v1.47/containers/abc123/stats", true},
		{"GET", "/v1.47/containers/abc123/top", true},
		{"GET", "/v1.47/info", true},
		{"GET", "/v1.47/images/json", true},
		{"GET", "/v1.47/version", true},
		{"GET", "/v1.47/events", true},
		{"GET", "/v1.47/volumes", true},
		{"GET", "/v1.47/volumes/myvolume", true},
		{"GET", "/v1.47/networks", true},
		{"GET", "/v1.47/networks/mynet", true},
		{"GET", "/v1.47/system/df", true},
		{"GET", "/v1.41/volumes/myvol", true},
		{"GET", "/v1.41/networks/mynet", true},

		// --- Allowed: GET on UNVERSIONED paths (RC8/RC9 fix) ---
		{"GET", "/containers/json", true},
		{"GET", "/containers/abc123/json", true},
		{"GET", "/containers/abc123/logs", true},
		{"GET", "/containers/abc123/stats", true},
		{"GET", "/containers/abc123/top", true},
		{"GET", "/info", true},
		{"GET", "/images/json", true},
		{"GET", "/version", true},
		{"GET", "/events", true},
		{"GET", "/volumes", true},
		{"GET", "/volumes/myvolume", true},
		{"GET", "/networks", true},
		{"GET", "/networks/mynet", true},
		{"GET", "/system/df", true},

		// --- Blocked: mutating methods ---
		{"POST", "/containers/create", false},
		{"DELETE", "/containers/abc123", false},
		{"PUT", "/containers/abc123/start", false},
		{"PATCH", "/v1.47/containers/abc123", false},
		{"POST", "/v1.41/containers/create", false},
		{"DELETE", "/v1.41/containers/abc", false},
		{"PUT", "/v1.41/networks/abc", false},
		{"PATCH", "/v1.41/containers/abc/update", false},

		// --- Blocked: paths not on allowlist ---
		{"GET", "/containers/abc123/start", false},
		{"GET", "/containers/abc123/stop", false},
		{"GET", "/exec/abc123/start", false},
		{"GET", "/build", false},
		{"GET", "/v1.47/exec/abc123", false},
		{"GET", "/v1.41/exec/abc/start", false},
		{"GET", "/v1.41/containers/prune", false},

		// --- Path traversal: path.Clean normalises before matching ---
		{"GET", "/v1.47/../containers/json", true},   // resolves to /containers/json — allowed
		{"GET", "/containers/../../etc/passwd", false}, // resolves to /etc/passwd — blocked
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

func TestFilter_ServeProxy_Blocked_UnversionedPost(t *testing.T) {
	f := muzzle.New()

	reqStr := "POST /containers/create HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\n\r\n"
	var buf bytes.Buffer

	err := f.ServeProxy("/tmp/nonexistent.sock", strings.NewReader(reqStr), &buf)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "403")
}
