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

// AgentSession represents a single connected Orthrus agent's active WebSocket
// and Yamux session. Full proxy stream forwarding is implemented in PR 5.
type AgentSession struct {
	agentUUID string
	agentName string
	conn      *websocket.Conn
	session   *yamux.Session
	cancel    context.CancelFunc
	proxyPort int // allocated in PR 5; 0 means no proxy listener yet
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

// Close terminates the Yamux session and the underlying WebSocket connection.
// Yamux closes the underlying net.Conn (wsNetConn) when the session is closed,
// which in turn closes the WebSocket connection; no second close is needed.
func (s *AgentSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

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
