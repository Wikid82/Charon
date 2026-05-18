package orthrus

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

// wsNetConn adapts a gorilla WebSocket connection to the net.Conn interface
// so that yamux can use it as a byte-stream transport.
// gorilla websocket supports one concurrent reader and one concurrent writer;
// rMu serialises reads and wMu serialises writes accordingly.
type wsNetConn struct {
	conn   *websocket.Conn
	rMu    sync.Mutex
	wMu    sync.Mutex
	reader io.Reader
}

func newWSNetConn(conn *websocket.Conn) net.Conn {
	return &wsNetConn{conn: conn}
}

func (c *wsNetConn) Read(b []byte) (int, error) {
	c.rMu.Lock()
	defer c.rMu.Unlock()

	for {
		if c.reader != nil {
			n, err := c.reader.Read(b)
			if err == io.EOF {
				c.reader = nil
				continue
			}
			return n, err
		}

		_, msg, err := c.conn.NextReader()
		if err != nil {
			return 0, err
		}
		c.reader = msg
	}
}

func (c *wsNetConn) Write(b []byte) (int, error) {
	c.wMu.Lock()
	defer c.wMu.Unlock()

	if err := c.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wsNetConn) Close() error {
	return c.conn.Close()
}

func (c *wsNetConn) LocalAddr() net.Addr  { return c.conn.NetConn().LocalAddr() }
func (c *wsNetConn) RemoteAddr() net.Addr { return c.conn.NetConn().RemoteAddr() }

func (c *wsNetConn) SetDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.conn.SetWriteDeadline(t)
}

func (c *wsNetConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *wsNetConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// streamTypeDocker is the first byte written to every yamux stream opened for
// Docker API proxying. The agent reads this byte to dispatch the stream to the
// Docker socket handler.
const streamTypeDocker = byte(0x01)

// AgentSession represents a single connected Orthrus agent's active WebSocket
// and Yamux session.
type AgentSession struct {
	agentUUID string
	agentName string
	conn      *websocket.Conn
	session   *yamux.Session
	cancel    context.CancelFunc
	listener  net.Listener // nil until StartDockerProxy succeeds
	proxyPort int          // ephemeral port allocated by StartDockerProxy
	mu        sync.Mutex
}

// NewAgentSession wraps the WebSocket connection in a Yamux server session.
func NewAgentSession(agentUUID, agentName string, conn *websocket.Conn) (*AgentSession, error) {
	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard

	session, err := yamux.Server(newWSNetConn(conn), cfg)
	if err != nil {
		return nil, fmt.Errorf("orthrus: create yamux session: %w", err)
	}

	_, cancel := context.WithCancel(context.Background())

	return &AgentSession{
		agentUUID: agentUUID,
		agentName: agentName,
		conn:      conn,
		session:   session,
		cancel:    cancel,
	}, nil
}

// Close terminates the proxy listener, the Yamux session, and the underlying
// WebSocket connection. Yamux closes the underlying net.Conn (wsNetConn) when
// the session is closed, which in turn closes the WebSocket connection; no
// second close is needed. Idempotent: a second call is a no-op for the
// listener.
func (s *AgentSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cancel()
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}
	return s.session.Close()
}

// GetProxyAddr returns the local address of the proxy listener for this session.
// Returns an empty string when no proxy listener is active.
func (s *AgentSession) GetProxyAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return ""
	}
	return fmt.Sprintf("127.0.0.1:%d", s.proxyPort)
}

// IsAlive returns true if the Yamux session has not been closed.
func (s *AgentSession) IsAlive() bool {
	return !s.session.IsClosed()
}

// StartDockerProxy allocates a loopback TCP listener on an ephemeral port and
// starts accepting connections. Each accepted connection opens a new yamux
// stream to the agent with a streamTypeDocker header byte. Returns an error if
// the proxy was already started or the listener could not be allocated.
func (s *AgentSession) StartDockerProxy() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return fmt.Errorf("orthrus: docker proxy already started for agent %s", s.agentUUID)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("orthrus: start docker proxy listener: %w", err)
	}

	s.listener = ln
	s.proxyPort = ln.Addr().(*net.TCPAddr).Port
	go s.runProxyListener(ln)
	return nil
}

// runProxyListener accepts TCP connections and spawns a proxyConn goroutine
// for each one. It exits when the listener is closed (by Close()).
func (s *AgentSession) runProxyListener(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go s.proxyConn(conn)
	}
}

// proxyConn forwards a single TCP connection through a new yamux stream.
// It writes the streamTypeDocker byte first so the agent can dispatch the
// stream to the Docker socket handler. io.Copy runs concurrently in both
// directions; proxyConn blocks until both directions complete.
func (s *AgentSession) proxyConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	stream, err := s.session.Open()
	if err != nil {
		return
	}
	defer func() { _ = stream.Close() }()

	if _, err := stream.Write([]byte{streamTypeDocker}); err != nil {
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(stream, conn)
	}()
	_, _ = io.Copy(conn, stream)
	<-done
}
