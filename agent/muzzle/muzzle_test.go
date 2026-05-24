package muzzle_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/agent/muzzle"
)

var errWriterClosed = errors.New("writer closed")

type closableWriter struct {
	mu         sync.Mutex
	buf        bytes.Buffer
	closed     bool
	firstWrite chan struct{}
	once       sync.Once
}

func newClosableWriter() *closableWriter {
	return &closableWriter{firstWrite: make(chan struct{})}
}

func (w *closableWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.firstWrite) })

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, errWriterClosed
	}
	return w.buf.Write(p)
}

func (w *closableWriter) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
}

func (w *closableWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func startUnixHTTPServer(t *testing.T, handler func(net.Conn)) (string, func()) {
	t.Helper()

	sockPath := filepath.Join(t.TempDir(), "docker.sock")
	ln, err := net.Listen("unix", sockPath)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		handler(conn)
	}()

	cleanup := func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("unix server goroutine did not exit")
		}
	}

	return sockPath, cleanup
}

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
		{"GET", "/v1.47/../containers/json", true},     // resolves to /containers/json — allowed
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

func TestServeProxy_ConnectionCloseSetOnRequest(t *testing.T) {
	f := muzzle.New()

	reqSeen := make(chan *http.Request, 1)
	serverErr := make(chan error, 1)

	sockPath, cleanup := startUnixHTTPServer(t, func(conn net.Conn) {
		defer conn.Close()

		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			serverErr <- err
			return
		}
		reqSeen <- req

		_, err = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 5\r\nConnection: close\r\n\r\nhello")
		if err != nil {
			serverErr <- err
		}
	})
	defer cleanup()

	var out bytes.Buffer
	req := "GET /containers/json HTTP/1.1\r\nHost: localhost\r\n\r\n"
	err := f.ServeProxy(sockPath, strings.NewReader(req), &out)
	require.NoError(t, err)

	seen := <-reqSeen
	select {
	case err := <-serverErr:
		require.NoError(t, err)
	default:
	}
	assert.True(t, seen.Close)
	assert.Equal(t, "close", strings.ToLower(seen.Header.Get("Connection")))
	assert.Contains(t, out.String(), "200 OK")
	assert.Contains(t, out.String(), "hello")
}

func TestServeProxy_CompletesAfterDockerResponse(t *testing.T) {
	f := muzzle.New()
	serverErr := make(chan error, 1)
	body := `{"status":"ok"}`

	sockPath, cleanup := startUnixHTTPServer(t, func(conn net.Conn) {
		defer conn.Close()

		_, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			serverErr <- err
			return
		}

		_, err = io.WriteString(conn, fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body))
		if err != nil {
			serverErr <- err
		}
	})
	defer cleanup()

	var out bytes.Buffer
	req := "GET /containers/json HTTP/1.1\r\nHost: localhost\r\n\r\n"

	done := make(chan error, 1)
	go func() {
		done <- f.ServeProxy(sockPath, strings.NewReader(req), &out)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("ServeProxy did not return after complete response")
	}

	assert.Contains(t, out.String(), body)
	select {
	case err := <-serverErr:
		require.NoError(t, err)
	default:
	}
}

func TestServeProxy_StreamingResponseTerminatesOnWriterClose(t *testing.T) {
	f := muzzle.New()

	serverWriteErr := make(chan error, 1)
	serverErr := make(chan error, 1)

	sockPath, cleanup := startUnixHTTPServer(t, func(conn net.Conn) {
		defer conn.Close()

		_, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			serverErr <- err
			return
		}

		_, err = io.WriteString(conn, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n")
		if err != nil {
			serverErr <- err
			return
		}

		for {
			if _, writeErr := io.WriteString(conn, "6\r\nhello!\r\n"); writeErr != nil {
				serverWriteErr <- writeErr
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	defer cleanup()

	w := newClosableWriter()
	req := "GET /events HTTP/1.1\r\nHost: localhost\r\n\r\n"

	done := make(chan error, 1)
	go func() {
		done <- f.ServeProxy(sockPath, strings.NewReader(req), w)
	}()

	select {
	case <-w.firstWrite:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not start")
	}

	w.Close()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("ServeProxy did not return after writer close")
	}

	select {
	case err := <-serverWriteErr:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("mock docker stream did not observe closed connection")
	}

	select {
	case err := <-serverErr:
		require.NoError(t, err)
	default:
	}
}
