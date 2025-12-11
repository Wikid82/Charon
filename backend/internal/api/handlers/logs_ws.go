package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/Wikid82/charon/backend/internal/logger"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development. In production, this should check
		// against a whitelist of allowed origins.
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// LogEntry represents a structured log entry sent over WebSocket.
type LogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	Source    string                 `json:"source"`
	Fields    map[string]interface{} `json:"fields"`
}

// LogsWebSocketHandler handles WebSocket connections for live log streaming.
func LogsWebSocketHandler(c *gin.Context) {
	logger.Log().Info("WebSocket connection attempt received")

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Log().WithError(err).Error("Failed to close WebSocket connection")
		}
	}()

	// Generate unique subscriber ID
	subscriberID := uuid.New().String()

	logger.Log().WithField("subscriber_id", subscriberID).Info("WebSocket connection established successfully")

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
	ticker := time.NewTicker(30 * time.Second)
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
			if err := conn.WriteJSON(logEntry); err != nil {
				logger.Log().WithError(err).Debug("Failed to write to WebSocket")
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}

		case <-done:
			// Client disconnected
			return
		}
	}
}
