package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

// HecateWSHandler streams tunnel log entries over WebSocket.
type HecateWSHandler struct {
	svc     *services.HecateService
	tracker *services.WebSocketTracker
}

// NewHecateWSHandler creates a HecateWSHandler.
func NewHecateWSHandler(svc *services.HecateService, tracker *services.WebSocketTracker) *HecateWSHandler {
	return &HecateWSHandler{svc: svc, tracker: tracker}
}

// StreamLogs upgrades the request to WebSocket and streams log lines from
// the tunnel's ring buffer. It replays buffered lines first, then delivers
// new lines as they arrive. The connection is kept alive with pings.
func (h *HecateWSHandler) StreamLogs(c *gin.Context) {
	tunnelUUID := c.Param("uuid")

	// Validate the tunnel exists before upgrading so non-WS clients (and tests)
	// receive a proper HTTP 404 rather than a WebSocket close frame.
	buf, err := h.svc.GetManager().GetLogBuffer(tunnelUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
		return
	}

	conn, upgradeErr := upgradeWebSocket(c)
	if upgradeErr != nil {
		logger.Log().WithError(upgradeErr).Error("hecate ws: upgrade failed")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		if closeErr := conn.Close(websocket.StatusNormalClosure, ""); closeErr != nil {
			logger.Log().WithError(closeErr).Debug("hecate ws: close error")
		}
	}()

	subscriberID := uuid.New().String()
	if h.tracker != nil {
		connInfo := &services.ConnectionInfo{
			ID:             subscriberID,
			Type:           "hecate",
			ConnectedAt:    time.Now(),
			LastActivityAt: time.Now(),
			RemoteAddr:     c.Request.RemoteAddr,
			UserAgent:      c.Request.UserAgent(),
		}
		h.tracker.Register(connInfo)
		defer h.tracker.Unregister(subscriberID)
	}

	// Replay buffered lines.
	for _, line := range buf.ReadAll() {
		if writeErr := writeLogSocketText(conn, []byte(line)); writeErr != nil {
			return
		}
	}

	sub := buf.Subscribe()

	// Closes when the client disconnects or stops answering pings
	done := watchLogSocket(ctx, conn)

	for {
		select {
		case <-done:
			return
		case line, ok := <-sub:
			if !ok {
				return
			}
			if writeErr := writeLogSocketText(conn, []byte(line)); writeErr != nil {
				return
			}
		}
	}
}
