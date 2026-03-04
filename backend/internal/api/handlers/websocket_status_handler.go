package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/services"
)

// WebSocketStatusHandler provides endpoints for WebSocket connection monitoring.
type WebSocketStatusHandler struct {
	tracker *services.WebSocketTracker
}

// NewWebSocketStatusHandler creates a new handler for WebSocket status monitoring.
func NewWebSocketStatusHandler(tracker *services.WebSocketTracker) *WebSocketStatusHandler {
	return &WebSocketStatusHandler{tracker: tracker}
}

// GetConnections returns a list of all active WebSocket connections.
func (h *WebSocketStatusHandler) GetConnections(c *gin.Context) {
	connections := h.tracker.GetAllConnections()
	c.JSON(http.StatusOK, gin.H{
		"connections": connections,
		"count":       len(connections),
	})
}

// GetStats returns aggregate statistics about WebSocket connections.
func (h *WebSocketStatusHandler) GetStats(c *gin.Context) {
	stats := h.tracker.GetStats()
	c.JSON(http.StatusOK, stats)
}
