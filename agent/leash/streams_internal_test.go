package leash

import (
	"errors"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/agent/muzzle"
)

// yamuxPair returns a peer session (opens streams, like Charon) and an agent
// session built with agentCfg (accepts streams, like the agent), joined over
// a loopback TCP connection.
func yamuxPair(t *testing.T, agentCfg *yamux.Config) (peer, agent *yamux.Session) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()

	peerConn, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	agentConn := <-accepted

	peerCfg := yamux.DefaultConfig()
	peerCfg.LogOutput = io.Discard
	peer, err = yamux.Server(peerConn, peerCfg)
	require.NoError(t, err)
	agent, err = yamux.Client(agentConn, agentCfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = agent.Close()
		_ = peer.Close()
	})
	return peer, agent
}

// shortHeaderTimeout lowers streamHeaderTimeout for one test.
func shortHeaderTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	prev := streamHeaderTimeout
	streamHeaderTimeout = d
	t.Cleanup(func() { streamHeaderTimeout = prev })
}

func silentLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

// serveAcceptedStream accepts one stream on agent and runs handleStream on it.
func serveAcceptedStream(l *Leash, agent *yamux.Session) {
	go func() {
		accepted, acceptErr := agent.AcceptStream()
		if acceptErr == nil {
			l.handleStream(accepted, muzzle.New(true))
		}
	}()
}

func TestNewSessionConfig_Bounds(t *testing.T) {
	cfg := newSessionConfig()
	assert.Equal(t, 64, cfg.AcceptBacklog)
	assert.Equal(t, uint32(256<<10), cfg.MaxStreamWindowSize)
	require.NoError(t, yamux.VerifyConfig(cfg))
}

// TestAcceptStreams_CapsConcurrentHandlers opens more streams than the
// concurrency cap while every handler is blocked. Exactly maxConcurrentStreams
// handlers may run at once; the rest wait, and are all served once the gate opens.
func TestAcceptStreams_CapsConcurrentHandlers(t *testing.T) {
	peer, agent := yamuxPair(t, newSessionConfig())

	const total = maxConcurrentStreams + 8
	var running, peak, handled atomic.Int32
	gate := make(chan struct{})

	go func() {
		_ = acceptStreams(agent, func(s *yamux.Stream) {
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			<-gate
			running.Add(-1)
			handled.Add(1)
			_ = s.Close()
		})
	}()

	// Pace the opens: yamux resets streams that overflow the accept backlog
	// while the agent is still accepting, which is expected behaviour but would
	// blur what this test measures (the concurrency cap).
	go func() {
		for i := 0; i < total; i++ {
			if _, err := peer.OpenStream(); err != nil {
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	require.Eventually(t, func() bool {
		return running.Load() == maxConcurrentStreams
	}, 5*time.Second, 10*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(maxConcurrentStreams), peak.Load(), "concurrent handlers must never exceed the cap")

	close(gate)
	require.Eventually(t, func() bool {
		return handled.Load() == total
	}, 10*time.Second, 10*time.Millisecond, "queued streams must still be served once capacity frees")
}

// TestAcceptStreams_ReturnsOnSessionClose confirms the accept loop ends with an
// error when the session closes, so connect can reconnect.
func TestAcceptStreams_ReturnsOnSessionClose(t *testing.T) {
	peer, agent := yamuxPair(t, newSessionConfig())
	done := make(chan error, 1)
	go func() {
		done <- acceptStreams(agent, func(*yamux.Stream) {})
	}()
	require.NoError(t, peer.Close())
	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("accept loop did not return after the session closed")
	}
}

// TestHandleStream_IdleStreamClosedAfterHeaderTimeout proves a stream that
// never sends its type byte is closed by the agent once the header deadline
// passes, instead of holding a slot indefinitely.
func TestHandleStream_IdleStreamClosedAfterHeaderTimeout(t *testing.T) {
	shortHeaderTimeout(t, 100*time.Millisecond)
	peer, agent := yamuxPair(t, newSessionConfig())
	l := New(Config{DockerSock: "/nonexistent.sock", Log: silentLogger()})

	peerStream, err := peer.OpenStream()
	require.NoError(t, err)
	serveAcceptedStream(l, agent)

	require.NoError(t, peerStream.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, readErr := peerStream.Read(make([]byte, 1))
	require.Error(t, readErr)
	assert.True(t, errors.Is(readErr, io.EOF), "agent must close the idle stream, got %v", readErr)
}

// TestHandleStream_UnknownTypeClosed keeps the unknown-type path closing the
// stream after the type byte is read.
func TestHandleStream_UnknownTypeClosed(t *testing.T) {
	peer, agent := yamuxPair(t, newSessionConfig())
	l := New(Config{DockerSock: "/nonexistent.sock", Log: silentLogger()})

	peerStream, err := peer.OpenStream()
	require.NoError(t, err)
	_, err = peerStream.Write([]byte{0x7f})
	require.NoError(t, err)
	serveAcceptedStream(l, agent)

	require.NoError(t, peerStream.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, readErr := peerStream.Read(make([]byte, 1))
	assert.True(t, errors.Is(readErr, io.EOF), "unknown stream type must be closed, got %v", readErr)
}

// TestHandleStream_LateBodyStillForwarded shows the header deadline is cleared
// once the type byte is read, so a request body that arrives after the timeout
// still reaches the Docker socket.
func TestHandleStream_LateBodyStillForwarded(t *testing.T) {
	shortHeaderTimeout(t, 100*time.Millisecond)

	dockerSock := t.TempDir() + "/d.sock"
	dockerLn, err := net.Listen("unix", dockerSock)
	require.NoError(t, err)
	defer func() { _ = dockerLn.Close() }()

	gotBody := make(chan string, 1)
	dockerSrv := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			gotBody <- string(body)
			w.WriteHeader(http.StatusCreated)
		}),
	}
	go func() { _ = dockerSrv.Serve(dockerLn) }()
	defer func() { _ = dockerSrv.Close() }()

	peer, agent := yamuxPair(t, newSessionConfig())
	l := New(Config{DockerSock: dockerSock, Log: silentLogger()})

	peerStream, err := peer.OpenStream()
	require.NoError(t, err)
	head := append([]byte{streamTypeDocker}, []byte("POST /containers/create HTTP/1.1\r\nHost: d\r\nContent-Length: 5\r\n\r\n")...)
	_, err = peerStream.Write(head)
	require.NoError(t, err)
	serveAcceptedStream(l, agent)

	// Wait well past the header timeout before the body arrives.
	time.Sleep(400 * time.Millisecond)
	_, err = peerStream.Write([]byte("hello"))
	require.NoError(t, err)

	select {
	case body := <-gotBody:
		assert.Equal(t, "hello", body)
	case <-time.After(5 * time.Second):
		t.Fatal("a late body never reached the Docker socket")
	}
}
