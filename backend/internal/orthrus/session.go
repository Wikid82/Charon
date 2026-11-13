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
	"strconv"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
	"golang.org/x/time/rate"
)

// writeLimiterEvery and writeLimiterBurst configure the token-bucket rate
// limiter applied to write-path traffic on any write-enabled session
// (Section 3.3.6 of the write-mode spec). Steady-state 0.5 req/s (30/min),
// burst of 5: a single compliant update cycle (pull → stop → remove →
// create → start) is exactly 5 write calls, so the burst allows one full
// update to proceed immediately, while the steady-state rate bounds
// sustained container churn well below anything a legitimate periodic
// update-checker would ever need. Fixed constants for v1 — no DB-backed
// per-agent tuning (see Section 7, Explicit Out-of-Scope).
const (
	writeLimiterEvery = 2 * time.Second
	writeLimiterBurst = 5
)

// streamTypeDocker identifies Docker socket proxy streams opened toward the agent.
// Must match the constant in the Orthrus agent (agent/leash/leash.go).
const streamTypeDocker = byte(0x01)

// newWSNetConn adapts a WebSocket connection to net.Conn so that yamux can use
// it as a byte-stream transport. Each Write sends one binary message. ctx bounds
// the adapter's reads and writes: cancelling it closes the WebSocket.
func newWSNetConn(ctx context.Context, conn *websocket.Conn) net.Conn {
	return websocket.NetConn(ctx, conn, websocket.MessageBinary)
}

// ExternalProxyStatus describes the runtime state of the external Docker proxy
// for this agent session. It intentionally carries no
// connection-string/hostname field: AgentSession is a long-lived per-tunnel
// object with no request-scoped data (no *gin.Context), so it cannot resolve
// a caller-appropriate hostname. That resolution happens one layer up, in
// OrthrusHandler.GetProxyStatus, which does have request context.
type ExternalProxyStatus struct {
	ConfiguredPort int      `json:"configured_port"` // port passed to StartExternalProxy
	ActivePort     int      `json:"active_port"`     // actual bound port (0 if not active)
	BoundAddress   string   `json:"bind_address"`    // actual listener address, e.g. "172.18.0.4:9999"
	ActiveBind     string   `json:"active_bind"`     // listen setting the session was started with
	BindMode       BindMode `json:"bind_mode"`       // how ActiveBind was interpreted; "" if unresolved
	BindFallback   bool     `json:"bind_fallback"`   // automatic mode fell back to loopback
	Active         bool     `json:"active"`
	WriteEnabled   bool     `json:"write_enabled"`   // negotiated value for this live session
	Error          string   `json:"error,omitempty"` // last start error, if any
}

// AgentSession represents a single connected Orthrus agent's active WebSocket
// and Yamux session.
type AgentSession struct {
	agentUUID    string
	agentName    string
	session      *yamux.Session
	cancel       context.CancelFunc
	env          netEnv       // host networking view used to resolve the external proxy bind
	proxyPort    int          // ephemeral port; 0 until StartDockerProxy succeeds
	listener     net.Listener // nil until StartDockerProxy succeeds; set atomically with proxyPort
	extServer    *http.Server // nil until StartExternalProxy succeeds
	extListener  net.Listener // nil until StartExternalProxy succeeds
	extProxyPort int          // port passed to StartExternalProxy; 0 if never started
	extBind      string       // listen setting passed to StartExternalProxy; "" if never started
	extBindMode  BindMode     // mode of the last successfully resolved bind
	// extBindFallback is true when the last successful resolution was automatic
	// and fell back to loopback.
	extBindFallback bool
	extErr          error // last error from StartExternalProxy
	// writeEnabled is the write-mode value negotiated at connect time (from
	// OrthrusAgent.WriteEnabled at the moment HandleWebSocket ran). Fixed for
	// the life of this session, exactly like extProxyPort — a DB toggle only
	// takes effect on the agent's next reconnect.
	writeEnabled bool
	// writeLimiter bounds write-request throughput for this session; nil
	// unless writeEnabled (lazily constructed in NewAgentSession).
	writeLimiter *rate.Limiter
	// auditLogger records write-path audit entries; nil is a safe no-op
	// (see Muzzle.logAudit), used by tests and read-only sessions.
	auditLogger AuditLogger
	mu          sync.Mutex
}

// NewAgentSession wraps the WebSocket connection in a Yamux server session.
// The session owns conn: Close tears the WebSocket down.
func NewAgentSession(agentUUID, agentName string, writeEnabled bool, auditLogger AuditLogger, conn *websocket.Conn) (*AgentSession, error) {
	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard

	ctx, cancel := context.WithCancel(context.Background())
	session, err := yamux.Server(newWSNetConn(ctx, conn), cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("orthrus: create yamux session: %w", err)
	}

	var writeLimiter *rate.Limiter
	if writeEnabled {
		writeLimiter = rate.NewLimiter(rate.Every(writeLimiterEvery), writeLimiterBurst)
	}

	return &AgentSession{
		agentUUID:    agentUUID,
		agentName:    agentName,
		session:      session,
		env:          systemNetEnv(),
		writeLimiter: writeLimiter,
		auditLogger:  auditLogger,
		cancel:       cancel,
		writeEnabled: writeEnabled,
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

// StartExternalProxy binds an HTTP listener on the address selected by bind
// (see ResolveBind) and port. Requests reverse-proxy through the existing
// loopback proxy to the agent's Docker socket, filtered by Muzzle.
// A port of 0 is a no-op (returns nil). Returns an error if the loopback proxy is
// not yet running, if an external proxy is already active, if the session is closed,
// if bind cannot be resolved to a local address, or if the OS cannot bind it.
func (s *AgentSession) StartExternalProxy(port int, bind string) error {
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

	// Attempt details are recorded before any step can fail, so the status
	// shows what was tried.
	s.mu.Lock()
	s.extProxyPort = port
	s.extBind = bind
	s.extBindMode = ""
	s.extBindFallback = false
	s.mu.Unlock()

	addr, mode, source, err := s.env.ResolveBind(bind)
	if err != nil {
		err = fmt.Errorf("orthrus: resolve external proxy bind: %w", err)
		s.recordExtErr(err)
		return err
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(addr.String(), strconv.Itoa(port)))
	if err != nil {
		err = fmt.Errorf("orthrus: bind external proxy port %d: %w", port, err)
		s.recordExtErr(err)
		return err
	}

	loopbackTarget := fmt.Sprintf("127.0.0.1:%d", loopbackPort)
	targetURL := &url.URL{Scheme: "http", Host: loopbackTarget}

	// Clone DefaultTransport to preserve its sane dial/TLS/idle-conn
	// defaults, then disable keep-alives so the external proxy never holds
	// open connections to the loopback target longer than a single request.
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.DisableKeepAlives = true
	// Do not let the transport request gzip on its own: the redactor reads plain
	// JSON, and a compressed inspect body must never reach it.
	baseTransport.DisableCompression = true

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(targetURL)
			pr.Out.Host = ""
		},
		// Environment redaction runs only on flagged container inspects (see
		// Muzzle.ServeHTTP). A failure there answers 502 via ErrorHandler.
		ModifyResponse: redactInspectEnv,
		ErrorHandler:   s.handleExternalProxyError,
		FlushInterval:  -1,
		Transport:      baseTransport,
	}

	srv := &http.Server{
		Handler:           NewMuzzle(rp, s.writeEnabled, s.writeLimiter, s.auditLogger, s.agentUUID),
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
	s.extBindMode = mode
	s.extBindFallback = source == BindSourceLoopbackFallback
	s.extErr = nil
	s.mu.Unlock()

	logger.Log().WithField("uuid", s.agentUUID).
		WithField("port", port).
		WithField("bind_mode", string(mode)).
		WithField("bind_address", addr.String()).
		Info("orthrus: external docker proxy started")

	go func() { _ = srv.Serve(ln) }()
	return nil
}

// recordExtErr stores a failed start attempt for GetExternalProxyStatus.
func (s *AgentSession) recordExtErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extErr = err
	s.extBindMode = ""
	s.extBindFallback = false
}

// handleExternalProxyError answers a failed external proxy request with a bare
// 502. The error is logged; the response body is never echoed.
func (s *AgentSession) handleExternalProxyError(w http.ResponseWriter, _ *http.Request, err error) {
	logger.Log().WithField("uuid", s.agentUUID).WithError(err).Warn("orthrus: external proxy request failed")
	http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
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

	return ExternalProxyStatus{
		ConfiguredPort: s.extProxyPort,
		ActivePort:     activePort,
		BoundAddress:   boundAddr,
		ActiveBind:     s.extBind,
		BindMode:       s.extBindMode,
		BindFallback:   s.extBindFallback,
		Active:         active,
		WriteEnabled:   s.writeEnabled,
		Error:          errStr,
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
