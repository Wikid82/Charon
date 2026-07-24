package orthrus

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var wsUpgrader = websocket.Upgrader{
	// Allow all origins; the real trust boundary is the bearer token.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// OrthrusServer manages incoming agent WebSocket connections and tracks
// active sessions indexed by agent UUID.
type OrthrusServer struct {
	db               *gorm.DB
	ca               *InternalCA
	sessions         sync.Map // agentUUID → *AgentSession
	heartbeatTimeout time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	// auditLogger records write-path audit entries for every AgentSession
	// this server creates. nil until SetAuditLogger is called (e.g. at
	// startup in routes.go, once the concrete *services.SecurityService is
	// available) — a nil auditLogger is a safe no-op throughout Muzzle.
	auditLogger AuditLogger
}

// NewOrthrusServer creates an OrthrusServer with the given database and internal CA.
func NewOrthrusServer(db *gorm.DB, ca *InternalCA) (*OrthrusServer, error) {
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel ownership transferred to struct; Stop() is responsible for calling it
	return &OrthrusServer{
		db:               db,
		ca:               ca,
		heartbeatTimeout: 10 * time.Second,
		ctx:              ctx,
		cancel:           cancel,
	}, nil
}

// SetAuditLogger wires the write-path audit logger used by every
// AgentSession this server creates from this point forward. Called once at
// startup (routes.go) with the concrete *services.SecurityService, which
// satisfies the AuditLogger interface structurally — orthrus never imports
// services directly (see the AuditLogger doc comment in muzzle.go for why).
func (s *OrthrusServer) SetAuditLogger(al AuditLogger) {
	s.auditLogger = al
}

// Stop cancels the server context, closes all active agent sessions, and waits
// for all background goroutines to exit. Closing sessions explicitly ensures
// yamux's internal goroutines terminate promptly regardless of the underlying
// transport state, which prevents flaky TempDir cleanup failures in tests.
func (s *OrthrusServer) Stop() {
	s.cancel()
	s.sessions.Range(func(key, value any) bool {
		sess := value.(*AgentSession)
		_ = sess.Close()
		s.sessions.Delete(key)
		return true
	})
	s.wg.Wait()
}

// HandleWebSocket is the Gin handler for GET /api/v1/ws/orthrus/connect.
// It authenticates the agent via bearer token, upgrades the connection to
// WebSocket, and starts session management goroutines.
func (s *OrthrusServer) HandleWebSocket(c *gin.Context) {
	token := extractBearer(c.GetHeader("Authorization"))
	if token == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	agent, err := s.findAgentByToken(token)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// X-Orthrus-Write-Enabled is delivered atomically as part of the 101
	// Switching Protocols response that completes the handshake — the agent
	// reads it off the *http.Response returned by its dial call (see
	// leash.go:connect) and uses it to construct a connection-scoped
	// muzzle.Filter for the life of this one connection. This is Option A
	// from Section 3.3.1 of the write-mode spec: no new yamux control-stream
	// type is introduced, avoiding a third hand-maintained stream-type
	// constant on top of the two (streamTypeDocker, streamTypePortForward)
	// that already have to be kept in sync between this package and
	// agent/leash/leash.go.
	respHeader := http.Header{}
	respHeader.Set("X-Orthrus-Write-Enabled", strconv.FormatBool(agent.WriteEnabled))
	// X-Orthrus-Allowed-Volumes-From carries the operator-configured
	// VolumesFrom source allowlist as a comma-joined list. A plain comma join
	// (no JSON/base64 encoding) is safe because Docker container names/IDs
	// are restricted to [a-zA-Z0-9][a-zA-Z0-9_.-]* — none of those characters
	// is a comma, so no source name can itself contain the separator. Empty
	// entries are dropped agent-side (see leash.go:connect).
	respHeader.Set("X-Orthrus-Allowed-Volumes-From", strings.Join(agent.AllowedVolumesFromSources, ","))

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, respHeader)
	if err != nil {
		logger.Log().WithError(err).Error("orthrus: WebSocket upgrade failed")
		return
	}

	session, err := NewAgentSession(agent.UUID, agent.Name, agent.WriteEnabled, agent.AllowedVolumesFromSources, s.auditLogger, conn)
	if err != nil {
		logger.Log().WithError(err).Error("orthrus: create agent session failed")
		_ = conn.Close()
		return
	}

	// Displace the prior session for this UUID BEFORE binding the new proxy
	// listeners so the old session's extListener is closed and its port is freed
	// before StartDockerProxy / StartExternalProxy attempt to bind.
	if old, loaded := s.sessions.LoadAndDelete(agent.UUID); loaded {
		if oldSess, ok := old.(*AgentSession); ok {
			_ = oldSess.Close()
		}
	}

	if err := session.StartDockerProxy(); err != nil {
		logger.Log().WithField("uuid", util.SanitizeForLog(agent.UUID)).
			WithError(err).Warn("orthrus: failed to start docker proxy listener")
	}

	if agent.ExternalProxyPort > 0 {
		if err := session.StartExternalProxy(agent.ExternalProxyPort); err != nil {
			logger.Log().WithField("uuid", util.SanitizeForLog(agent.UUID)).
				WithField("port", agent.ExternalProxyPort).
				WithError(err).Warn("orthrus: failed to start external proxy")
		}
	}

	now := time.Now()
	if err := s.db.Model(agent).Updates(map[string]interface{}{
		"status":    models.OrthrusStatusOnline,
		"last_seen": &now,
	}).Error; err != nil {
		logger.Log().WithField("uuid", util.SanitizeForLog(agent.UUID)).WithError(err).Warn("orthrus: update agent status failed")
	}

	logger.Log().WithFields(logrus.Fields{
		"uuid": util.SanitizeForLog(agent.UUID),
		"name": util.SanitizeForLog(agent.Name),
	}).Info("orthrus: agent connected")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.watchHeartbeat(agent.UUID, session)
	}()

	s.sessions.Store(agent.UUID, session)
}

// GetExternalProxyStatus returns the external proxy status for a connected agent.
// Returns (ExternalProxyStatus{}, false) when no active session exists.
func (s *OrthrusServer) GetExternalProxyStatus(agentUUID string) (ExternalProxyStatus, bool) {
	raw, ok := s.sessions.Load(agentUUID)
	if !ok {
		return ExternalProxyStatus{}, false
	}
	sess := raw.(*AgentSession)
	return sess.GetExternalProxyStatus(), true
}

// GetProxyAddr returns the proxy listener address for a connected agent.
// Returns ("", false) when no active session or no proxy port is allocated.
func (s *OrthrusServer) GetProxyAddr(agentUUID string) (string, bool) {
	raw, ok := s.sessions.Load(agentUUID)
	if !ok {
		return "", false
	}
	sess := raw.(*AgentSession)
	addr := sess.GetProxyAddr()
	if addr == "" {
		return "", false
	}
	return addr, true
}

// GetSession returns the active AgentSession for the given UUID.
func (s *OrthrusServer) GetSession(agentUUID string) (*AgentSession, bool) {
	raw, ok := s.sessions.Load(agentUUID)
	if !ok {
		return nil, false
	}
	return raw.(*AgentSession), true
}

// DisconnectAgent closes the active session and removes it from the session map.
func (s *OrthrusServer) DisconnectAgent(agentUUID string) error {
	raw, ok := s.sessions.LoadAndDelete(agentUUID)
	if !ok {
		return nil
	}
	sess := raw.(*AgentSession)
	if err := sess.Close(); err != nil {
		return fmt.Errorf("orthrus: disconnect agent %s: %w", agentUUID, err)
	}
	return nil
}

// watchHeartbeat monitors a connected agent session. If the Yamux session
// closes (indicating the agent disconnected), the agent is marked offline.
// Full heartbeat message validation is implemented in PR 5.
func (s *OrthrusServer) watchHeartbeat(agentUUID string, sess *AgentSession) {
	ticker := time.NewTicker(s.heartbeatTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if !sess.IsAlive() {
				_ = sess.Close() // stops runProxyListener goroutine; idempotent
				// Only remove from the map and mark offline when this goroutine's
				// session pointer is still the current one. A stale goroutine
				// holding an old pointer will find CompareAndDelete returns false
				// and exits without corrupting the new session's state.
				if s.sessions.CompareAndDelete(agentUUID, sess) {
					s.markOffline(agentUUID)
				}
				return
			}
		}
	}
}

func (s *OrthrusServer) markOffline(agentUUID string) {
	if err := s.db.Model(&models.OrthrusAgent{}).
		Where("uuid = ?", agentUUID).
		Update("status", models.OrthrusStatusOffline).Error; err != nil {
		logger.Log().WithField("uuid", agentUUID).WithError(err).Warn("orthrus: mark agent offline failed")
	}
	logger.Log().WithField("uuid", agentUUID).Info("orthrus: agent marked offline")
}

// findAgentByToken iterates all agents and bcrypt-compares the provided token
// against each agent's stored hash. O(n) in the number of agents; acceptable
// because the number of registered agents is expected to be small.
func (s *OrthrusServer) findAgentByToken(token string) (*models.OrthrusAgent, error) {
	var agents []models.OrthrusAgent
	if err := s.db.Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("orthrus: query agents: %w", err)
	}

	for i := range agents {
		// Truncate to match Provision's bcrypt input (≤71 bytes).
		tokenBytes := []byte(token)
		if len(tokenBytes) >= 72 {
			tokenBytes = tokenBytes[:71]
		}
		if err := bcrypt.CompareHashAndPassword([]byte(agents[i].AuthKeyHash), tokenBytes); err == nil {
			return &agents[i], nil
		}
	}
	return nil, fmt.Errorf("orthrus: no agent matched token")
}

func extractBearer(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}
