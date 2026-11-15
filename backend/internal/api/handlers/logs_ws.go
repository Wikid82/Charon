package handlers

import (
	"context"
	"github.com/Wikid82/charon/backend/internal/security"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

// LogEntry represents a structured log entry sent over WebSocket.
type LogEntry struct {
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Timestamp string         `json:"timestamp"`
	Source    string         `json:"source"`
	Fields    map[string]any `json:"fields"`
}

// LogsWSHandler handles WebSocket connections for live log streaming.
type LogsWSHandler struct {
	tracker        *services.WebSocketTracker
	publicOrigins  PublicOriginSource           // may be nil: no public origin is trusted
	trustedProxies security.TrustedProxyMatcher // API-layer trusted proxies for forwarded host headers
}

// WithOriginPolicy wires the configured public origin and the trusted-proxy list into the WebSocket origin check.
func (h *LogsWSHandler) WithOriginPolicy(s PublicOriginSource, trusted security.TrustedProxyMatcher) *LogsWSHandler {
	h.publicOrigins = s
	h.trustedProxies = trusted
	return h
}

// NewLogsWSHandler creates a new handler for log streaming.
func NewLogsWSHandler(tracker *services.WebSocketTracker) *LogsWSHandler {
	return &LogsWSHandler{tracker: tracker}
}

// HandleWebSocket handles WebSocket connections for live log streaming.
func (h *LogsWSHandler) HandleWebSocket(c *gin.Context) {
	logger.Log().Info("WebSocket connection attempt received")

	// Upgrade HTTP connection to WebSocket
	conn, err := upgradeWebSocket(c, h.publicOrigins, h.trustedProxies)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
			logger.Log().WithError(err).Debug("Failed to close WebSocket connection")
		}
	}()

	configureLogSocket(conn)

	// Generate unique subscriber ID
	subscriberID := uuid.New().String()

	logger.Log().WithField("subscriber_id", subscriberID).Info("WebSocket connection established successfully")

	// Register connection with tracker if available
	if h.tracker != nil {
		filters := trackedFilters(c.Request.URL.Query(), logsTrackedFilterKeys)
		connInfo := &services.ConnectionInfo{
			ID:             subscriberID,
			Type:           "logs",
			ConnectedAt:    time.Now(),
			LastActivityAt: time.Now(),
			RemoteAddr:     c.Request.RemoteAddr,
			UserAgent:      c.Request.UserAgent(),
			Filters:        filters,
		}
		h.tracker.Register(connInfo)
		defer h.tracker.Unregister(subscriberID)
	}

	// Parse query parameters for filtering
	levelFilter := strings.ToLower(c.Query("level"))
	sourceFilter := strings.ToLower(c.Query("source"))

	// Subscribe to log broadcasts
	hook := logger.GetBroadcastHook()
	logChan := hook.Subscribe(subscriberID)
	defer hook.Unsubscribe(subscriberID)

	// Closes when the client disconnects or stops answering pings
	done := watchLogSocket(ctx, conn)

	// Main loop: stream logs to client
	for {
		select {
		case entry, ok := <-logChan:
			if !ok {
				// Channel closed
				return
			}

			// Apply filters
			if levelFilter != "" && !strings.EqualFold(entry.Level.String(), levelFilter) {
				continue
			}

			source := ""
			if s, ok := entry.Data["source"]; ok {
				source = s.(string)
			}

			if sourceFilter != "" && !strings.Contains(strings.ToLower(source), sourceFilter) {
				continue
			}

			// Convert logrus entry to LogEntry
			logEntry := LogEntry{
				Level:     entry.Level.String(),
				Message:   entry.Message,
				Timestamp: entry.Time.Format(time.RFC3339),
				Source:    source,
				Fields:    entry.Data,
			}

			// Send to WebSocket client
			if err := writeLogSocketJSON(conn, logEntry); err != nil {
				logger.Log().WithError(err).Debug("Failed to write to WebSocket")
				return
			}

			// Update activity timestamp
			if h.tracker != nil {
				h.tracker.UpdateActivity(subscriberID)
			}

		case <-done:
			// Client disconnected
			return
		}
	}
}
