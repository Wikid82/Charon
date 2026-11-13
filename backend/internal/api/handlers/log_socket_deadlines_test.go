package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

// withLogSocketTimings overrides the log socket timers for one test and restores
// the previous values on cleanup. Handlers read the timers when they run, so a
// test that expects a handler to see these timers must wait for that handler
// before its cleanup restores the values.
func withLogSocketTimings(t *testing.T, pongWait, writeWait, pingPeriod time.Duration) {
	t.Helper()
	prev := logSocketTiming.Load()
	logSocketTiming.Store(&logSocketTimers{pongWait: pongWait, pingPeriod: pingPeriod, writeWait: writeWait})
	t.Cleanup(func() { logSocketTiming.Store(prev) })
}

// newLogSocketTestServer serves one WebSocket connection, configures it with
// configureLogSocket and hands it to handle.
func newLogSocketTestServer(t *testing.T, handle func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		configureLogSocket(conn)
		handle(conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// dialLogSocketTestServer dials srv and closes the connection at test cleanup.
func dialLogSocketTestServer(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.Dial(context.Background(), toWebSocketURL(srv.URL), nil) //nolint:bodyclose // coder/websocket manages the handshake response body
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

// watchLogSocketUntilDone runs watchLogSocket on conn and returns a channel that
// closes once the watch ends, which happens when the connection drops.
func watchLogSocketUntilDone(conn *websocket.Conn) <-chan struct{} {
	ctx, cancel := context.WithCancel(context.Background())
	done := watchLogSocket(ctx, conn)
	stopped := make(chan struct{})
	go func() {
		<-done
		cancel()
		close(stopped)
	}()
	return stopped
}

func TestConfigureLogSocket_OversizeClientMessageClosesConnection(t *testing.T) {
	serverErr := make(chan error, 1)
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		_, _, err := conn.Read(context.Background())
		serverErr <- err
	})
	client := dialLogSocketTestServer(t, srv)

	oversize := strings.Repeat("a", logSocketReadLimit+1)
	require.NoError(t, client.Write(context.Background(), websocket.MessageText, []byte(oversize)))

	select {
	case srvErr := <-serverErr:
		require.Error(t, srvErr, "server read should fail on an oversize message")
	case <-time.After(5 * time.Second):
		t.Fatal("server did not reject the oversize message")
	}

	// The server closes with 1009 (message too big), which the client sees as its next read error.
	_, err := readWithin(t, client, 5*time.Second)
	require.Equal(t, websocket.StatusMessageTooBig, websocket.CloseStatus(err), "expected message-too-big close, got %v", err)
}

func TestConfigureLogSocket_MessageWithinLimitIsAccepted(t *testing.T) {
	serverMsg := make(chan string, 1)
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		_, data, err := conn.Read(context.Background())
		if err == nil {
			serverMsg <- string(data)
		}
	})
	client := dialLogSocketTestServer(t, srv)

	payload := strings.Repeat("b", logSocketReadLimit)
	require.NoError(t, client.Write(context.Background(), websocket.MessageText, []byte(payload)))

	select {
	case got := <-serverMsg:
		require.Len(t, got, logSocketReadLimit)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive the in-limit message")
	}
}

// TestWatchLogSocket_SilentPeerIsDisconnected covers a peer that never answers
// pings. The client never reads, so no pong is sent and the watch must end once
// the pong deadline passes.
func TestWatchLogSocket_SilentPeerIsDisconnected(t *testing.T) {
	withLogSocketTimings(t, 200*time.Millisecond, 10*time.Second, 20*time.Millisecond)

	dropped := make(chan struct{})
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		<-watchLogSocketUntilDone(conn)
		close(dropped)
	})
	_ = dialLogSocketTestServer(t, srv)

	select {
	case <-dropped:
	case <-time.After(5 * time.Second):
		t.Fatal("keep-alive did not drop a peer that never answered pings")
	}
}

// TestWatchLogSocket_PongsKeepConnectionAlive covers a responsive peer. Each
// answered ping must move the pong deadline forward, so the connection survives
// several pong windows.
func TestWatchLogSocket_PongsKeepConnectionAlive(t *testing.T) {
	const pongWait = 300 * time.Millisecond
	withLogSocketTimings(t, pongWait, 10*time.Second, 50*time.Millisecond)

	dropped := make(chan struct{})
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		<-watchLogSocketUntilDone(conn)
		close(dropped)
	})
	client := dialLogSocketTestServer(t, srv)

	// Reading makes the client answer every ping with a pong.
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := client.Read(context.Background()); err != nil {
				return
			}
		}
	}()

	select {
	case <-dropped:
		t.Fatal("connection dropped although the peer answered pings")
	case <-time.After(3 * pongWait):
	}

	require.NoError(t, client.CloseNow())
	select {
	case <-dropped:
	case <-time.After(5 * time.Second):
		t.Fatal("keep-alive did not end after the client closed")
	}
	<-readDone
}

// stalledWriteBudget caps how long a stalled-write test keeps writing before it
// gives up. A write that hits its deadline ends the loop well before this budget.
const stalledWriteBudget = 10 * time.Second

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
		writeErr = fillSocketUntilWriteError(func() error { return write(conn) })
	})
	// The client is dialed but never reads, so the server's writes back up.
	_ = dialLogSocketTestServer(t, srv)

	select {
	case <-resultReady:
		return writeErr
	case <-time.After(stalledWriteBudget + 5*time.Second):
		t.Fatal("stalled write handler did not finish")
		return nil
	}
}

// fillSocketUntilWriteError calls write until it fails. The client never reads,
// so the kernel buffers fill and a write fails on its deadline.
func fillSocketUntilWriteError(write func() error) error {
	deadline := time.Now().Add(stalledWriteBudget)
	for time.Now().Before(deadline) {
		if err := write(); err != nil {
			return err
		}
	}
	return errors.New("writes never failed within " + stalledWriteBudget.String())
}

// requireWriteTimeout asserts err comes from a write deadline. A coder write that
// hits its context deadline reports context.DeadlineExceeded.
func requireWriteTimeout(t *testing.T, err error) {
	t.Helper()
	require.ErrorIs(t, err, context.DeadlineExceeded, "expected write timeout, got %v", err)
}

func TestWriteLogSocketJSON_SetsWriteDeadline(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, 100*time.Millisecond, 30*time.Second)

	payload := map[string]string{"data": strings.Repeat("c", 64*1024)}
	err := runStalledWriteTest(t, func(conn *websocket.Conn) error {
		return writeLogSocketJSON(conn, payload)
	})
	requireWriteTimeout(t, err)
}

func TestWriteLogSocketText_SetsWriteDeadline(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, 100*time.Millisecond, 30*time.Second)

	payload := []byte(strings.Repeat("d", 64*1024))
	err := runStalledWriteTest(t, func(conn *websocket.Conn) error {
		return writeLogSocketText(conn, payload)
	})
	requireWriteTimeout(t, err)
}
