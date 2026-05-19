package orthrus

import (
	"context"
	"fmt"
	"net/http"
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

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Log().WithError(err).Error("orthrus: WebSocket upgrade failed")
		return
	}

	session, err := NewAgentSession(agent.UUID, agent.Name, conn)
	if err != nil {
		logger.Log().WithError(err).Error("orthrus: create agent session failed")
		_ = conn.Close()
		return
	}

	s.sessions.Store(agent.UUID, session)

	if err := session.StartDockerProxy(); err != nil {
		logger.Log().WithField("uuid", util.SanitizeForLog(agent.UUID)).
			WithError(err).Warn("orthrus: failed to start docker proxy listener")
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
				s.markOffline(agentUUID)
				s.sessions.Delete(agentUUID)
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
