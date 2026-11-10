package handlers

import (
	"errors"
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
	conn, resp, err := websocket.DefaultDialer.Dial(toWebSocketURL(srv.URL), nil)
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

func TestWriteLogSocketJSON_SetsWriteDeadline(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, 100*time.Millisecond, 30*time.Second)

	writeErr := make(chan error, 1)
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		// The client never reads, so socket buffers fill and the write must hit its deadline.
		payload := map[string]string{"data": strings.Repeat("c", 64*1024)}
		var err error
		for i := 0; i < 10000 && err == nil; i++ {
			err = writeLogSocketJSON(conn, payload)
		}
		writeErr <- err
	})
	_ = dialLogSocketTestServer(t, srv)

	select {
	case err := <-writeErr:
		var netErr net.Error
		require.True(t, errors.As(err, &netErr) && netErr.Timeout(), "expected write timeout, got %v", err)
	case <-time.After(20 * time.Second):
		t.Fatal("writes never hit the write deadline")
	}
}

func TestWriteLogSocketPing_SetsWriteDeadline(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, 100*time.Millisecond, 30*time.Second)

	writeErr := make(chan error, 1)
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		var err error
		for i := 0; i < 2000000 && err == nil; i++ {
			err = writeLogSocketPing(conn)
		}
		writeErr <- err
	})
	_ = dialLogSocketTestServer(t, srv)

	select {
	case err := <-writeErr:
		var netErr net.Error
		require.True(t, errors.As(err, &netErr) && netErr.Timeout(), "expected write timeout, got %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("ping writes never hit the write deadline")
	}
}
