// Package services provides business logic services for the application.
package services

import (
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// ConnectionInfo tracks information about a single WebSocket connection.
type ConnectionInfo struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"` // "logs" or "cerberus"
	ConnectedAt    time.Time `json:"connected_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
	RemoteAddr     string    `json:"remote_addr,omitempty"`
	UserAgent      string    `json:"user_agent,omitempty"`
	Filters        string    `json:"filters,omitempty"` // Query parameters used for filtering
}

// ConnectionStats provides aggregate statistics about WebSocket connections.
type ConnectionStats struct {
	TotalActive         int        `json:"total_active"`
	LogsConnections     int        `json:"logs_connections"`
	CerberusConnections int        `json:"cerberus_connections"`
	OldestConnection    *time.Time `json:"oldest_connection,omitempty"`
	LastUpdated         time.Time  `json:"last_updated"`
}

// WebSocketTracker tracks active WebSocket connections and provides statistics.
type WebSocketTracker struct {
	mu          sync.RWMutex
	connections map[string]*ConnectionInfo
}

// NewWebSocketTracker creates a new WebSocket connection tracker.
func NewWebSocketTracker() *WebSocketTracker {
	return &WebSocketTracker{
		connections: make(map[string]*ConnectionInfo),
	}
}

// Register adds a new WebSocket connection to tracking.
func (t *WebSocketTracker) Register(conn *ConnectionInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connections[conn.ID] = conn
	logger.Log().WithField("connection_id", conn.ID).
		WithField("type", conn.Type).
		WithField("remote_addr", conn.RemoteAddr).
		Debug("WebSocket connection registered")
}

// Unregister removes a WebSocket connection from tracking.
func (t *WebSocketTracker) Unregister(connectionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if conn, exists := t.connections[connectionID]; exists {
		duration := time.Since(conn.ConnectedAt)
		logger.Log().WithField("connection_id", connectionID).
			WithField("type", conn.Type).
			WithField("duration", duration.String()).
			Debug("WebSocket connection unregistered")
		delete(t.connections, connectionID)
	}
}

// UpdateActivity updates the last activity timestamp for a connection.
func (t *WebSocketTracker) UpdateActivity(connectionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if conn, exists := t.connections[connectionID]; exists {
		conn.LastActivityAt = time.Now()
	}
}

// GetConnection retrieves information about a specific connection.
func (t *WebSocketTracker) GetConnection(connectionID string) (*ConnectionInfo, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	conn, exists := t.connections[connectionID]
	return conn, exists
}

// GetAllConnections returns a slice of all active connections.
func (t *WebSocketTracker) GetAllConnections() []*ConnectionInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	connections := make([]*ConnectionInfo, 0, len(t.connections))
	for _, conn := range t.connections {
		// Create a copy to avoid race conditions
		connCopy := *conn
		connections = append(connections, &connCopy)
	}
	return connections
}

// GetStats returns aggregate statistics about WebSocket connections.
func (t *WebSocketTracker) GetStats() *ConnectionStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := &ConnectionStats{
		TotalActive:         len(t.connections),
		LogsConnections:     0,
		CerberusConnections: 0,
		LastUpdated:         time.Now(),
	}

	var oldestTime *time.Time
	for _, conn := range t.connections {
		switch conn.Type {
		case "logs":
			stats.LogsConnections++
		case "cerberus":
			stats.CerberusConnections++
		}

		if oldestTime == nil || conn.ConnectedAt.Before(*oldestTime) {
			t := conn.ConnectedAt
			oldestTime = &t
		}
	}

	stats.OldestConnection = oldestTime
	return stats
}

// GetCount returns the total number of active connections.
func (t *WebSocketTracker) GetCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.connections)
}
