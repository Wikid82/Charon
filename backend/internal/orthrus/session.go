package orthrus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

// streamTypeDocker identifies Docker socket proxy streams opened toward the agent.
// Must match the constant in the Orthrus agent (agent/leash/leash.go).
const streamTypeDocker = byte(0x01)

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

// AgentSession represents a single connected Orthrus agent's active WebSocket
// and Yamux session.
type AgentSession struct {
	agentUUID string
	agentName string
	conn      *websocket.Conn
	session   *yamux.Session
	cancel    context.CancelFunc
	proxyPort int          // ephemeral port; 0 until StartDockerProxy succeeds
	listener  net.Listener // nil until StartDockerProxy succeeds; set atomically with proxyPort
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

// StartDockerProxy allocates an ephemeral loopback TCP listener and starts
// accepting connections. Each accepted connection is tunnelled to the agent's
// Docker socket via a new yamux stream of type streamTypeDocker (0x01).
// Returns an error if the OS cannot allocate a port, if the proxy is already
// running for this session (idempotency guard), or if the session is closed.
func (s *AgentSession) StartDockerProxy() error {
	s.mu.Lock()
	if s.listener != nil {
		s.mu.Unlock()
		return fmt.Errorf("orthrus: docker proxy already started for session %s", s.agentUUID)
	}
	if s.session.IsClosed() {
		s.mu.Unlock()
		return fmt.Errorf("orthrus: cannot start proxy on closed session %s", s.agentUUID)
	}
	s.mu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("orthrus: allocate proxy listener: %w", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port

	s.mu.Lock()
	if s.listener != nil { // re-check after net.Listen (double-check pattern)
		_ = ln.Close()
		s.mu.Unlock()
		return fmt.Errorf("orthrus: docker proxy already started for session %s", s.agentUUID)
	}
	s.listener = ln
	s.proxyPort = port
	s.mu.Unlock()

	go s.runProxyListener(ln)
	return nil
}

// runProxyListener accepts TCP connections from local Docker clients and
// dispatches each to proxyConn. Exits when ln is closed (normal shutdown
// via Close).
func (s *AgentSession) runProxyListener(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return // normal shutdown
			}
			logger.Log().WithField("uuid", s.agentUUID).WithError(err).Warn("orthrus: proxy listener accept error")
			return
		}
		go s.proxyConn(conn)
	}
}

// proxyConn opens a yamux stream, writes the Docker stream-type byte,
// then bidirectionally copies until either side closes.
func (s *AgentSession) proxyConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	stream, err := s.session.Open()
	if err != nil {
		// yamux session already closed; expected on disconnect.
		logger.Log().WithField("uuid", s.agentUUID).WithError(err).Debug("orthrus: open yamux stream failed")
		return
	}
	defer func() { _ = stream.Close() }()

	if _, err := stream.Write([]byte{streamTypeDocker}); err != nil {
		logger.Log().WithField("uuid", s.agentUUID).WithError(err).Warn("orthrus: write stream type byte failed")
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer func() { _ = stream.Close() }()
		io.Copy(stream, conn) //nolint:errcheck
	}()
	go func() {
		defer wg.Done()
		defer func() { _ = conn.Close() }()
		io.Copy(conn, stream) //nolint:errcheck
	}()
	wg.Wait()
}

// Close terminates the proxy listener, cancels the context, and closes the
// Yamux session (which also closes the underlying WebSocket via wsNetConn).
func (s *AgentSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
		// proxyPort is intentionally left non-zero for diagnostic purposes.
		// GetProxyAddr will not be called on a closed session: the session is
		// removed from the sessions map before any caller can observe it.
	}

	s.cancel()
	return s.session.Close()
}

// GetProxyAddr returns the local address of the proxy listener for this session.
// Returns an empty string when no proxy port has been allocated (PR 5).
func (s *AgentSession) GetProxyAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.proxyPort == 0 {
		return ""
	}
	return fmt.Sprintf("127.0.0.1:%d", s.proxyPort)
}

// IsAlive returns true if the Yamux session has not been closed.
func (s *AgentSession) IsAlive() bool {
	return !s.session.IsClosed()
}
