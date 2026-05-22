package orthrus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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

// ExternalProxyStatus describes the runtime state of the external Docker proxy
// bound on 0.0.0.0 for this agent session.
type ExternalProxyStatus struct {
	ConfiguredPort   int    `json:"configured_port"`             // port passed to StartExternalProxy
	ActivePort       int    `json:"active_port"`                 // actual bound port (0 if not active)
	BoundAddress     string `json:"bind_address"`                // e.g. "0.0.0.0:9999"
	ConnectionString string `json:"connection_string,omitempty"` // e.g. "tcp://charon:9999"
	Active           bool   `json:"active"`
	Error            string `json:"error,omitempty"` // last start error, if any
}

// AgentSession represents a single connected Orthrus agent's active WebSocket
// and Yamux session.
type AgentSession struct {
	agentUUID    string
	agentName    string
	conn         *websocket.Conn
	session      *yamux.Session
	cancel       context.CancelFunc
	proxyPort    int          // ephemeral port; 0 until StartDockerProxy succeeds
	listener     net.Listener // nil until StartDockerProxy succeeds; set atomically with proxyPort
	extServer    *http.Server // nil until StartExternalProxy succeeds
	extListener  net.Listener // nil until StartExternalProxy succeeds
	extProxyPort int          // port passed to StartExternalProxy; 0 if never started
	extErr       error        // last error from StartExternalProxy
	mu           sync.Mutex
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
	if s.session.IsClosed() {
		_ = ln.Close()
		s.mu.Unlock()
		return fmt.Errorf("orthrus: cannot start proxy on closed session %s", s.agentUUID)
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
	const maxTransientRetries = 5
	var transientRetries int

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return // normal shutdown
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				transientRetries++
				if transientRetries <= maxTransientRetries {
					logger.Log().WithField("uuid", s.agentUUID).WithError(err).Warn("orthrus: proxy listener transient error, retrying")
					time.Sleep(time.Duration(transientRetries*100) * time.Millisecond)
					continue
				}
			}
			logger.Log().WithField("uuid", s.agentUUID).WithError(err).Warn("orthrus: proxy listener accept error")
			return
		}
		transientRetries = 0 // reset on successful accept
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
		io.Copy(stream, conn) //nolint:errcheck,gosec
	}()
	go func() {
		defer wg.Done()
		defer func() { _ = conn.Close() }()
		io.Copy(conn, stream) //nolint:errcheck,gosec
	}()
	wg.Wait()
}

// StartExternalProxy binds an HTTP listener on 0.0.0.0:port that reverse-proxies
// Docker API requests through the existing loopback proxy to the agent's Docker socket.
// Requests are filtered by Muzzle to the read-only allowlist.
// A port of 0 is a no-op (returns nil). Returns an error if the loopback proxy is
// not yet running, if an external proxy is already active, if the session is closed,
// or if the OS cannot bind the port.
func (s *AgentSession) StartExternalProxy(port int) error {
	if port == 0 {
		return nil
	}

	s.mu.Lock()
	if s.proxyPort == 0 {
		s.mu.Unlock()
		return errors.New("orthrus: loopback proxy not started")
	}
	if s.extListener != nil {
		s.mu.Unlock()
		return errors.New("orthrus: external proxy already started")
	}
	if s.session.IsClosed() {
		s.mu.Unlock()
		return errors.New("orthrus: cannot start external proxy on closed session")
	}
	loopbackPort := s.proxyPort
	s.mu.Unlock()

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		s.mu.Lock()
		s.extProxyPort = port // record attempted port for diagnostics
		s.extErr = fmt.Errorf("orthrus: bind external proxy port %d: %w", port, err)
		s.mu.Unlock()
		return s.extErr
	}

	loopbackTarget := fmt.Sprintf("127.0.0.1:%d", loopbackPort)
	targetURL := &url.URL{Scheme: "http", Host: loopbackTarget}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(targetURL)
			pr.Out.Host = ""
		},
		FlushInterval: -1,
	}

	srv := &http.Server{
		Handler:           NewMuzzle(rp),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0,
	}

	s.mu.Lock()
	if s.extListener != nil { // double-check after bind
		_ = ln.Close()
		s.mu.Unlock()
		return errors.New("orthrus: external proxy already started")
	}
	s.extListener = ln
	s.extServer = srv
	s.extProxyPort = port
	s.extErr = nil
	s.mu.Unlock()

	logger.Log().WithField("uuid", s.agentUUID).
		WithField("port", port).
		Info("orthrus: external docker proxy started")

	go func() { _ = srv.Serve(ln) }()
	return nil
}

// GetExternalProxyStatus returns the runtime state of the external proxy for this session.
func (s *AgentSession) GetExternalProxyStatus() ExternalProxyStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	active := s.extListener != nil
	boundAddr := ""
	activePort := 0
	if active {
		boundAddr = s.extListener.Addr().String()
		if tcpAddr, ok := s.extListener.Addr().(*net.TCPAddr); ok {
			activePort = tcpAddr.Port
		}
	}

	errStr := ""
	if s.extErr != nil {
		errStr = s.extErr.Error()
	}

	connStr := ""
	if active && activePort > 0 {
		connStr = fmt.Sprintf("tcp://charon:%d", activePort)
	}

	return ExternalProxyStatus{
		ConfiguredPort:   s.extProxyPort,
		ActivePort:       activePort,
		BoundAddress:     boundAddr,
		ConnectionString: connStr,
		Active:           active,
		Error:            errStr,
	}
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

	if s.extServer != nil {
		_ = s.extServer.Close()
		s.extServer = nil
	}
	if s.extListener != nil {
		_ = s.extListener.Close()
		s.extListener = nil
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
