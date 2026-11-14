package orthrus

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveRawDockerOnYamux answers every Docker request on clientYamux with the
// given raw HTTP response bytes, so tests control status and headers.
func serveRawDockerOnYamux(t *testing.T, clientYamux *yamux.Session, raw string) {
	t.Helper()
	go func() {
		for {
			stream, err := clientYamux.AcceptStream()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close() //nolint:errcheck
				if _, err := io.ReadFull(conn, make([]byte, 1)); err != nil {
					return
				}
				req, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil {
					return
				}
				_ = req.Body.Close()
				_, _ = io.WriteString(conn, raw)
			}(stream)
		}
	}()
}

// startBoundProxy starts the external proxy with the given bind setting on a
// fresh port and waits until it is active.
func startBoundProxy(t *testing.T, sess *AgentSession, bind string) (int, error) {
	t.Helper()
	port := findFreePort(t)
	err := sess.StartExternalProxy(port, bind)
	if err == nil {
		assert.Eventually(t, func() bool {
			return sess.GetExternalProxyStatus().Active
		}, 2*time.Second, 10*time.Millisecond)
	}
	return port, err
}

func TestStartExternalProxy_ExplicitLoopbackBind(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()
	sess.env = fakeEnv(routeTableHeader, fakeIface{name: "lo", flags: up | net.FlagLoopback, addrs: []string{"127.0.0.1/8"}})

	port, err := startBoundProxy(t, sess, "127.0.0.1")
	require.NoError(t, err)

	status := sess.GetExternalProxyStatus()
	assert.Equal(t, fmt.Sprintf("127.0.0.1:%d", port), status.BoundAddress)
	assert.Equal(t, "127.0.0.1", status.ActiveBind)
	assert.Equal(t, BindModeAddress, status.BindMode)
	assert.False(t, status.BindFallback)
}

func TestStartExternalProxy_WildcardBind(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	port, err := startBoundProxy(t, sess, "0.0.0.0")
	require.NoError(t, err)

	status := sess.GetExternalProxyStatus()
	assert.Equal(t, BindModeAll, status.BindMode)
	// Dual-stack hosts report the wildcard listener as [::]; only the port is fixed.
	assert.True(t, strings.HasSuffix(status.BoundAddress, fmt.Sprintf(":%d", port)), status.BoundAddress)
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	require.NoError(t, err)
	_ = c.Close()
}

func TestStartExternalProxy_AutomaticFallsBackToLoopback(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()
	sess.env = netEnv{
		interfaces: func() ([]net.Interface, error) { return nil, fmt.Errorf("denied") },
		addrs:      func(net.Interface) ([]net.Addr, error) { return nil, nil },
		routeTable: func() ([]byte, error) { return nil, fmt.Errorf("unused") },
	}

	port, err := startBoundProxy(t, sess, "")
	require.NoError(t, err)

	status := sess.GetExternalProxyStatus()
	assert.Equal(t, fmt.Sprintf("127.0.0.1:%d", port), status.BoundAddress)
	assert.Equal(t, BindModeAuto, status.BindMode)
	assert.True(t, status.BindFallback)
	assert.Empty(t, status.ActiveBind)
}

func TestStartExternalProxy_UnresolvableBindRecordsStatus(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()
	sess.env = fakeEnv(routeTableHeader, fakeIface{name: "lo", flags: up | net.FlagLoopback, addrs: []string{"127.0.0.1/8"}})

	port := findFreePort(t)
	err := sess.StartExternalProxy(port, "192.0.2.0/24")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no local address in 192.0.2.0/24")

	status := sess.GetExternalProxyStatus()
	assert.False(t, status.Active)
	assert.NotEmpty(t, status.Error)
	assert.Equal(t, "192.0.2.0/24", status.ActiveBind)
	assert.Equal(t, port, status.ConfiguredPort)
	assert.Empty(t, status.BindMode)
	assert.False(t, status.BindFallback)
}

func TestStartExternalProxy_InvalidBindSettingIsRefused(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	err := sess.StartExternalProxy(findFreePort(t), "not-an-address")
	require.Error(t, err)
	assert.False(t, sess.GetExternalProxyStatus().Active)
}

func TestStartExternalProxy_RedactsInspectEnvEndToEnd(t *testing.T) {
	sess, clientYamux, cleanup := sessionWithLoopback(t)
	defer cleanup()
	serveMockDockerOnYamux(clientYamux, map[string]string{
		"/containers/abc/json": inspectBody,
	})

	port, err := startBoundProxy(t, sess, "127.0.0.1")
	require.NoError(t, err)

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/containers/abc/json", port), http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, string(body), "hunter2")
	assert.Contains(t, string(body), "DB_PASSWORD=[redacted]")
	assert.Contains(t, string(body), `"Image":"nginx:1.27"`)
}

func TestStartExternalProxy_InspectFailsClosedWithoutBodyLeak(t *testing.T) {
	sess, clientYamux, cleanup := sessionWithLoopback(t)
	defer cleanup()

	// Not JSON, so redaction cannot run. The client must get a bare 502.
	payload := `DB_PASSWORD=hunter2 not json`
	raw := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		len(payload), payload)
	serveRawDockerOnYamux(t, clientYamux, raw)

	port, err := startBoundProxy(t, sess, "127.0.0.1")
	require.NoError(t, err)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/containers/abc/json", port))
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.False(t, bytes.Contains(body, []byte("hunter2")), "502 body must not echo upstream content")
	assert.False(t, strings.Contains(string(body), "DB_PASSWORD"))
}

// TestStartExternalProxy_DaemonNeverAskedForCompression proves the loopback
// request carries no Accept-Encoding, even when the client sent none, so the
// transport cannot add gzip on its own.
func TestStartExternalProxy_DaemonNeverAskedForCompression(t *testing.T) {
	sess, clientYamux, cleanup := sessionWithLoopback(t)
	defer cleanup()

	seen := make(chan string, 1)
	go func() {
		for {
			stream, err := clientYamux.AcceptStream()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close() //nolint:errcheck
				if _, err := io.ReadFull(conn, make([]byte, 1)); err != nil {
					return
				}
				req, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil {
					return
				}
				_ = req.Body.Close()
				select {
				case seen <- req.Header.Get("Accept-Encoding"):
				default:
				}
				raw := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
					len(inspectBody), inspectBody)
				_, _ = io.WriteString(conn, raw)
			}(stream)
		}
	}()

	port, err := startBoundProxy(t, sess, "127.0.0.1")
	require.NoError(t, err)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/containers/abc/json", port))
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	_, _ = io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	select {
	case got := <-seen:
		assert.Empty(t, got, "daemon request must not ask for a compressed body")
	case <-time.After(2 * time.Second):
		t.Fatal("daemon request not observed")
	}
}
