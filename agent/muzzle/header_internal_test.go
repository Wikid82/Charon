package muzzle

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFilter_ServeProxy_SlowHeaderCut proves a request header that never
// finishes is cut by the header deadline instead of blocking ServeProxy.
func TestFilter_ServeProxy_SlowHeaderCut(t *testing.T) {
	f := New(true)
	f.headerTimeout = 100 * time.Millisecond

	server, client := net.Pipe()
	defer func() { _ = client.Close() }()

	errCh := make(chan error, 1)
	go func() {
		errCh <- f.ServeProxy("/nonexistent.sock", server, io.Discard)
	}()

	// Send half a request line, then stall.
	_, err := client.Write([]byte("GET /_ping HTT"))
	require.NoError(t, err)

	select {
	case err := <-errCh:
		assert.Error(t, err, "a stalled header must fail the proxy")
	case <-time.After(5 * time.Second):
		t.Fatal("ServeProxy blocked on a stalled header")
	}
}

// TestFilter_ServeProxy_LateBodyNotCut proves the header deadline is cleared
// once the header is read, so a body that arrives after the header timeout is
// still forwarded to the Docker socket.
func TestFilter_ServeProxy_LateBodyNotCut(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "d.sock")
	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	gotBody := make(chan string, 1)
	srv := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			gotBody <- string(body)
			w.WriteHeader(http.StatusCreated)
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	f := New(true)
	f.headerTimeout = 100 * time.Millisecond

	server, client := net.Pipe()
	defer func() { _ = client.Close() }()

	var out bytes.Buffer
	var outMu sync.Mutex
	errCh := make(chan error, 1)
	go func() {
		errCh <- f.ServeProxy(sock, server, &lockedWriter{w: &out, mu: &outMu})
	}()

	_, err = client.Write([]byte("POST /containers/create HTTP/1.1\r\nHost: d\r\nContent-Length: 5\r\n\r\n"))
	require.NoError(t, err)
	time.Sleep(400 * time.Millisecond)
	_, err = client.Write([]byte("hello"))
	require.NoError(t, err)

	select {
	case body := <-gotBody:
		assert.Equal(t, "hello", body)
	case <-time.After(5 * time.Second):
		t.Fatal("late body never reached the Docker socket")
	}
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ServeProxy did not finish")
	}
	outMu.Lock()
	defer outMu.Unlock()
	assert.Contains(t, out.String(), "201")
}

// lockedWriter serialises writes so the test can read the buffer safely.
type lockedWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
