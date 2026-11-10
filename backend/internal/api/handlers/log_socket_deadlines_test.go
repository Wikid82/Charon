package handlers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// withLogSocketTimings overrides the log socket deadlines for one test and
// restores the production values on cleanup. Tests in this file do not run in
// parallel, so the package-level values are never read concurrently with a write.
func withLogSocketTimings(t *testing.T, pongWait, writeWait, pingPeriod time.Duration) {
	t.Helper()
	prevPong, prevWrite, prevPing := logSocketPongWait, logSocketWriteWait, logSocketPingPeriod
	logSocketPongWait, logSocketWriteWait, logSocketPingPeriod = pongWait, writeWait, pingPeriod
	t.Cleanup(func() {
		logSocketPongWait, logSocketWriteWait, logSocketPingPeriod = prevPong, prevWrite, prevPing
	})
}

// newLogSocketTestServer serves one WebSocket connection, configures it with
// configureLogSocket and hands it to handle.
func newLogSocketTestServer(t *testing.T, handle func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	testUpgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		configureLogSocket(conn)
		handle(conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func dialLogSocketTestServer(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	return dialWithDialer(t, srv, websocket.DefaultDialer)
}

func dialWithDialer(t *testing.T, srv *httptest.Server, dialer *websocket.Dialer) *websocket.Conn {
	t.Helper()
	conn, resp, err := dialer.Dial(toWebSocketURL(srv.URL), nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestConfigureLogSocket_OversizeClientMessageClosesConnection(t *testing.T) {
	serverErr := make(chan error, 1)
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		_, _, err := conn.ReadMessage()
		serverErr <- err
	})
	client := dialLogSocketTestServer(t, srv)

	oversize := strings.Repeat("a", logSocketReadLimit+1)
	require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(oversize)))

	select {
	case err := <-serverErr:
		require.ErrorIs(t, err, websocket.ErrReadLimit)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not reject the oversize message")
	}

	// The server closed the connection, so the client's next read must fail.
	require.NoError(t, client.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err := client.ReadMessage()
	require.Error(t, err)
}

func TestConfigureLogSocket_MessageWithinLimitIsAccepted(t *testing.T) {
	serverMsg := make(chan string, 1)
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		_, data, err := conn.ReadMessage()
		if err == nil {
			serverMsg <- string(data)
		}
	})
	client := dialLogSocketTestServer(t, srv)

	payload := strings.Repeat("b", logSocketReadLimit)
	require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(payload)))

	select {
	case got := <-serverMsg:
		require.Len(t, got, logSocketReadLimit)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive the in-limit message")
	}
}

func TestConfigureLogSocket_SilentPeerHitsReadDeadline(t *testing.T) {
	withLogSocketTimings(t, 200*time.Millisecond, 10*time.Second, 30*time.Second)

	serverErr := make(chan error, 1)
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		// The ping is never answered because the client does not read.
		if err := writeLogSocketPing(conn); err != nil {
			serverErr <- err
			return
		}
		_, _, err := conn.ReadMessage()
		serverErr <- err
	})
	_ = dialLogSocketTestServer(t, srv)

	select {
	case err := <-serverErr:
		var netErr net.Error
		require.True(t, errors.As(err, &netErr) && netErr.Timeout(), "expected read timeout, got %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("read deadline did not fire without a pong")
	}
}

func TestConfigureLogSocket_PongRefreshesReadDeadline(t *testing.T) {
	const pongWait = 300 * time.Millisecond
	withLogSocketTimings(t, pongWait, 10*time.Second, 50*time.Millisecond)

	// The pinger reads the package timers, so the cleanup must wait for it to
	// exit before withLogSocketTimings restores the production values.
	stop := make(chan struct{})
	pingerDone := make(chan struct{})
	t.Cleanup(func() {
		close(stop)
		<-pingerDone
	})

	serverErr := make(chan error, 1)
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		// Keep pinging for longer than one pong wait. Each pong must extend the deadline.
		go func() {
			defer close(pingerDone)
			ticker := time.NewTicker(logSocketPingPeriod)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					if err := writeLogSocketPing(conn); err != nil {
						return
					}
				}
			}
		}()
		_, _, err := conn.ReadMessage()
		serverErr <- err
	})
	client := dialLogSocketTestServer(t, srv)

	// The default ping handler answers every ping with a pong while the client reads.
	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				return
			}
		}
	}()

	select {
	case err := <-serverErr:
		t.Fatalf("connection dropped although the peer answered pings: %v", err)
	case <-time.After(3 * pongWait):
	}
	require.NoError(t, client.Close())
	select {
	case err := <-serverErr:
		require.Error(t, err, "server read must fail once the client closes")
	case <-time.After(5 * time.Second):
		t.Fatal("server read did not end after the client closed")
	}
}

// stalledSocketBufferBytes is the send buffer on the server and the receive buffer
// on the client of a stalled-write test. Setting SO_SNDBUF and SO_RCVBUF locks both
// sizes, so the kernel cannot autotune them upward and the queued bytes stay bounded.
const stalledSocketBufferBytes = 4096

// stalledWriteBudget caps how long a stalled-write test keeps writing before it
// gives up. A write that hits its deadline ends the loop well before this budget.
const stalledWriteBudget = 10 * time.Second

// shrinkServerSendBuffer bounds the server-side send buffer of conn.
func shrinkServerSendBuffer(conn *websocket.Conn) error {
	tcpConn, ok := conn.NetConn().(*net.TCPConn)
	if !ok {
		return fmt.Errorf("log socket test connection is %T, want *net.TCPConn", conn.NetConn())
	}
	if err := tcpConn.SetWriteBuffer(stalledSocketBufferBytes); err != nil {
		return fmt.Errorf("shrink server send buffer: %w", err)
	}
	return nil
}

// dialStalledReaderClient dials srv with a small receive buffer on the client side.
// The buffer is set inside NetDialContext, before the HTTP upgrade request is sent.
func dialStalledReaderClient(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	dialer := &websocket.Dialer{
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var d net.Dialer
			conn, err := d.DialContext(ctx, network, addr)
			if err != nil {
				return nil, fmt.Errorf("dial stalled reader client: %w", err)
			}
			tcpConn, ok := conn.(*net.TCPConn)
			if !ok {
				_ = conn.Close()
				return nil, fmt.Errorf("stalled reader connection is %T, want *net.TCPConn", conn)
			}
			if err := tcpConn.SetReadBuffer(stalledSocketBufferBytes); err != nil {
				_ = conn.Close()
				return nil, fmt.Errorf("shrink client receive buffer: %w", err)
			}
			return conn, nil
		},
	}
	return dialWithDialer(t, srv, dialer)
}

// fillSocketUntilWriteError calls write until it fails. The client never reads,
// so the bounded buffers fill and a write fails on its deadline.
func fillSocketUntilWriteError(write func() error) error {
	deadline := time.Now().Add(stalledWriteBudget)
	for time.Now().Before(deadline) {
		if err := write(); err != nil {
			return err
		}
	}
	return fmt.Errorf("writes never failed within %s", stalledWriteBudget)
}

// runStalledWriteTest runs write on the server side of a connection whose client
// never reads, and returns the first write error. The caller must have applied
// withLogSocketTimings first: the cleanup registered here runs before that restore,
// so the handler goroutine has finished reading the timers.
func runStalledWriteTest(t *testing.T, write func(conn *websocket.Conn) error) error {
	t.Helper()
	var writeErr error
	resultReady := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-resultReady:
		case <-time.After(stalledWriteBudget + 5*time.Second):
		}
	})

	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		defer close(resultReady)
		if err := shrinkServerSendBuffer(conn); err != nil {
			writeErr = err
			return
		}
		writeErr = fillSocketUntilWriteError(func() error { return write(conn) })
	})
	// Registered after the wait cleanup, so the client closes before the handler is awaited.
	_ = dialStalledReaderClient(t, srv)

	select {
	case <-resultReady:
		return writeErr
	case <-time.After(stalledWriteBudget + 5*time.Second):
		t.Fatal("stalled write handler did not finish")
		return nil
	}
}

// requireWriteTimeout asserts err is a network timeout from a write deadline.
func requireWriteTimeout(t *testing.T, err error) {
	t.Helper()
	var netErr net.Error
	require.True(t, errors.As(err, &netErr) && netErr.Timeout(), "expected write timeout, got %v", err)
}

func TestWriteLogSocketJSON_SetsWriteDeadline(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, 100*time.Millisecond, 30*time.Second)

	payload := map[string]string{"data": strings.Repeat("c", 64*1024)}
	err := runStalledWriteTest(t, func(conn *websocket.Conn) error {
		return writeLogSocketJSON(conn, payload)
	})
	requireWriteTimeout(t, err)
}

func TestWriteLogSocketPing_SetsWriteDeadline(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, 100*time.Millisecond, 30*time.Second)

	err := runStalledWriteTest(t, writeLogSocketPing)
	requireWriteTimeout(t, err)
}
