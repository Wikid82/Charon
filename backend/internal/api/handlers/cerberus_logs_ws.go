// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

// CerberusLogsHandler handles WebSocket connections for streaming security logs.
type CerberusLogsHandler struct {
	watcher *services.LogWatcher
	tracker *services.WebSocketTracker
}

// NewCerberusLogsHandler creates a new handler for Cerberus security log streaming.
func NewCerberusLogsHandler(watcher *services.LogWatcher, tracker *services.WebSocketTracker) *CerberusLogsHandler {
	return &CerberusLogsHandler{
		watcher: watcher,
		tracker: tracker,
	}
}

// LiveLogs handles WebSocket connections for Cerberus security log streaming.
// It upgrades the HTTP connection to WebSocket, subscribes to the LogWatcher,
// and streams SecurityLogEntry as JSON to connected clients.
//
// Query parameters for filtering:
//   - source: filter by source (waf, crowdsec, ratelimit, acl, normal)
//   - blocked_only: only show blocked requests (true/false)
//   - level: filter by log level (info, warn, error)
//   - ip: filter by client IP (partial match)
//   - host: filter by host (partial match)
func (h *CerberusLogsHandler) LiveLogs(c *gin.Context) {
	logger.Log().Info("Cerberus logs WebSocket connection attempt")

	// Upgrade HTTP connection to WebSocket
	// CheckOrigin is enforced on the shared upgrader in logs_ws.go (same package).
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil) // nosemgrep: go.gorilla.security.audit.websocket-missing-origin-check.websocket-missing-origin-check
	if err != nil {
		logger.Log().WithError(err).Error("Failed to upgrade Cerberus logs WebSocket")
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Log().WithError(err).Debug("Failed to close Cerberus logs WebSocket connection")
		}
	}()

	// Generate unique subscriber ID for logging
	subscriberID := uuid.New().String()
	logger.Log().WithField("subscriber_id", subscriberID).Info("Cerberus logs WebSocket connected")

	// Register connection with tracker if available
	if h.tracker != nil {
		filters := c.Request.URL.RawQuery
		connInfo := &services.ConnectionInfo{
			ID:             subscriberID,
			Type:           "cerberus",
			ConnectedAt:    time.Now(),
			LastActivityAt: time.Now(),
			RemoteAddr:     c.Request.RemoteAddr,
			UserAgent:      c.Request.UserAgent(),
			Filters:        filters,
		}
		h.tracker.Register(connInfo)
		defer h.tracker.Unregister(subscriberID)
	}

	// Parse query filters
	sourceFilter := strings.ToLower(c.Query("source")) // waf, crowdsec, ratelimit, acl, normal
	levelFilter := strings.ToLower(c.Query("level"))   // info, warn, error
	ipFilter := c.Query("ip")                          // Partial match on client IP
	hostFilter := strings.ToLower(c.Query("host"))     // Partial match on host
	blockedOnly := c.Query("blocked_only") == "true"   // Only show blocked requests

	// Subscribe to log watcher
	logChan := h.watcher.Subscribe()
	defer h.watcher.Unsubscribe(logChan)

	// Channel to detect client disconnect
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Keep-alive ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-logChan:
			if !ok {
				// Channel closed, log watcher stopped
				return
			}

			// Apply source filter
			if sourceFilter != "" && !strings.EqualFold(entry.Source, sourceFilter) {
				continue
			}

			// Apply level filter
			if levelFilter != "" && !strings.EqualFold(entry.Level, levelFilter) {
				continue
			}

			// Apply IP filter (partial match)
			if ipFilter != "" && !strings.Contains(entry.ClientIP, ipFilter) {
				continue
			}

			// Apply host filter (partial match, case-insensitive)
			if hostFilter != "" && !strings.Contains(strings.ToLower(entry.Host), hostFilter) {
				continue
			}

			// Apply blocked_only filter
			if blockedOnly && !entry.Blocked {
				continue
			}

			// Send to WebSocket client
			if err := conn.WriteJSON(entry); err != nil {
				logger.Log().WithError(err).WithField("subscriber_id", subscriberID).Debug("Failed to write Cerberus log to WebSocket")
				return
			}

			// Update activity timestamp
			if h.tracker != nil {
				h.tracker.UpdateActivity(subscriberID)
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				logger.Log().WithError(err).WithField("subscriber_id", subscriberID).Debug("Failed to send ping to Cerberus logs WebSocket")
				return
			}

		case <-done:
			// Client disconnected
			logger.Log().WithField("subscriber_id", subscriberID).Info("Cerberus logs WebSocket client disconnected")
			return
		}
	}
}
