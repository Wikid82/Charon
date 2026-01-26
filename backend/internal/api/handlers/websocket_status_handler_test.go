package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

func TestWebSocketStatusHandler_GetConnections(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tracker := services.NewWebSocketTracker()
	handler := NewWebSocketStatusHandler(tracker)

	// Register test connections
	conn1 := &services.ConnectionInfo{
		ID:             "conn-1",
		Type:           "logs",
		ConnectedAt:    time.Now(),
		LastActivityAt: time.Now(),
		RemoteAddr:     "192.168.1.1:12345",
		UserAgent:      "Mozilla/5.0",
		Filters:        "level=error",
	}
	conn2 := &services.ConnectionInfo{
		ID:             "conn-2",
		Type:           "cerberus",
		ConnectedAt:    time.Now(),
		LastActivityAt: time.Now(),
		RemoteAddr:     "192.168.1.2:54321",
		UserAgent:      "Chrome/90.0",
		Filters:        "source=waf",
	}

	tracker.Register(conn1)
	tracker.Register(conn2)

	// Create test request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/websocket/connections", http.NoBody)

	// Call handler
	handler.GetConnections(c)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["count"])
	connections, ok := response["connections"].([]any)
	require.True(t, ok)
	assert.Len(t, connections, 2)
}

func TestWebSocketStatusHandler_GetConnectionsEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tracker := services.NewWebSocketTracker()
	handler := NewWebSocketStatusHandler(tracker)

	// Create test request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/websocket/connections", http.NoBody)

	// Call handler
	handler.GetConnections(c)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(0), response["count"])
	connections, ok := response["connections"].([]any)
	require.True(t, ok)
	assert.Len(t, connections, 0)
}

func TestWebSocketStatusHandler_GetStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tracker := services.NewWebSocketTracker()
	handler := NewWebSocketStatusHandler(tracker)

	// Register test connections
	conn1 := &services.ConnectionInfo{
		ID:          "conn-1",
		Type:        "logs",
		ConnectedAt: time.Now(),
	}
	conn2 := &services.ConnectionInfo{
		ID:          "conn-2",
		Type:        "logs",
		ConnectedAt: time.Now(),
	}
	conn3 := &services.ConnectionInfo{
		ID:          "conn-3",
		Type:        "cerberus",
		ConnectedAt: time.Now(),
	}

	tracker.Register(conn1)
	tracker.Register(conn2)
	tracker.Register(conn3)

	// Create test request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/websocket/stats", http.NoBody)

	// Call handler
	handler.GetStats(c)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var stats services.ConnectionStats
	err := json.Unmarshal(w.Body.Bytes(), &stats)
	require.NoError(t, err)

	assert.Equal(t, 3, stats.TotalActive)
	assert.Equal(t, 2, stats.LogsConnections)
	assert.Equal(t, 1, stats.CerberusConnections)
	assert.NotNil(t, stats.OldestConnection)
	assert.False(t, stats.LastUpdated.IsZero())
}

func TestWebSocketStatusHandler_GetStatsEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tracker := services.NewWebSocketTracker()
	handler := NewWebSocketStatusHandler(tracker)

	// Create test request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/websocket/stats", http.NoBody)

	// Call handler
	handler.GetStats(c)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var stats services.ConnectionStats
	err := json.Unmarshal(w.Body.Bytes(), &stats)
	require.NoError(t, err)

	assert.Equal(t, 0, stats.TotalActive)
	assert.Equal(t, 0, stats.LogsConnections)
	assert.Equal(t, 0, stats.CerberusConnections)
	assert.Nil(t, stats.OldestConnection)
	assert.False(t, stats.LastUpdated.IsZero())
}
