package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// No Origin header — non-browser client or same-origin request.
			return true
		}
		originURL, err := url.Parse(origin)
		if err != nil {
			return false
		}
		requestHost := r.Host
		if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
			requestHost = forwardedHost
		}
		return originURL.Host == requestHost
	},
}

// upgradeWebSocket upgrades an HTTP connection to WebSocket using the
// shared, origin-checked upgrader above. All WS handlers in this package
// MUST go through this helper rather than calling upgrader.Upgrade
// directly, so the origin check stays visible to single-file static
// analysis at every call site.
func upgradeWebSocket(c *gin.Context) (*websocket.Conn, error) {
	return upgrader.Upgrade(c.Writer, c.Request, nil)
}

// logSocketReadLimit caps the size of a single client message.
const logSocketReadLimit = 4096

// Log socket timers. They are variables only so tests can shorten them.
var (
	// logSocketPongWait is how long a client may stay silent before its read fails.
	logSocketPongWait = 60 * time.Second
	// logSocketPingPeriod is how often the server pings the client.
	logSocketPingPeriod = 30 * time.Second
	// logSocketWriteWait bounds every write, including pings.
	logSocketWriteWait = 10 * time.Second
)

// configureLogSocket applies the read and write limits shared by every log
// WebSocket. Call it right after the upgrade. Each pong from the client
// extends the read deadline, so an idle but responsive client stays open.
func configureLogSocket(conn *websocket.Conn) {
	conn.SetReadLimit(logSocketReadLimit)
	if err := conn.SetReadDeadline(time.Now().Add(logSocketPongWait)); err != nil {
		logger.Log().WithError(err).Debug("Failed to set initial log socket read deadline")
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(logSocketPongWait))
	})
}

// armLogSocketWriteDeadline refreshes the write deadline before every write, so a
// stalled client cannot block the handler. gorilla's SetWriteDeadline only records
// the time and never fails, so there is no error to report here.
func armLogSocketWriteDeadline(conn *websocket.Conn) {
	_ = conn.SetWriteDeadline(time.Now().Add(logSocketWriteWait))
}

// writeLogSocketJSON sends v as a JSON text message under the write deadline.
func writeLogSocketJSON(conn *websocket.Conn, v any) error {
	armLogSocketWriteDeadline(conn)
	return conn.WriteJSON(v)
}

// writeLogSocketPing sends a ping frame under the same write deadline as data writes.
func writeLogSocketPing(conn *websocket.Conn) error {
	armLogSocketWriteDeadline(conn)
	return conn.WriteMessage(websocket.PingMessage, []byte{})
}

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
	tracker *services.WebSocketTracker
}

// NewLogsWSHandler creates a new handler for log streaming.
func NewLogsWSHandler(tracker *services.WebSocketTracker) *LogsWSHandler {
	return &LogsWSHandler{tracker: tracker}
}

// LogsWebSocketHandler handles WebSocket connections for live log streaming.
//
// Deprecated: Use NewLogsWSHandler().HandleWebSocket instead. Kept for backward compatibility.
func LogsWebSocketHandler(c *gin.Context) {
	// For backward compatibility, create a nil tracker if called directly
	handler := NewLogsWSHandler(nil)
	handler.HandleWebSocket(c)
}

// HandleWebSocket handles WebSocket connections for live log streaming.
func (h *LogsWSHandler) HandleWebSocket(c *gin.Context) {
	logger.Log().Info("WebSocket connection attempt received")

	// Upgrade HTTP connection to WebSocket
	conn, err := upgradeWebSocket(c)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Log().WithError(err).Error("Failed to close WebSocket connection")
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

	// Channel to signal when client disconnects
	done := make(chan struct{})

	// Goroutine to read from WebSocket (detect client disconnect)
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Main loop: stream logs to client
	ticker := time.NewTicker(logSocketPingPeriod)
	defer ticker.Stop()

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

		case <-ticker.C:
			// Send ping to keep connection alive
			if err := writeLogSocketPing(conn); err != nil {
				return
			}

		case <-done:
			// Client disconnected
			return
		}
	}
}
