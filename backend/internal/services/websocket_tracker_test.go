package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebSocketTracker(t *testing.T) {
	tracker := NewWebSocketTracker()
	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.connections)
	assert.Equal(t, 0, tracker.GetCount())
}

func TestWebSocketTracker_Register(t *testing.T) {
	tracker := NewWebSocketTracker()

	conn := &ConnectionInfo{
		ID:             "test-conn-1",
		Type:           "logs",
		ConnectedAt:    time.Now(),
		LastActivityAt: time.Now(),
		RemoteAddr:     "192.168.1.1:12345",
		UserAgent:      "Mozilla/5.0",
		Filters:        "level=error",
	}

	tracker.Register(conn)
	assert.Equal(t, 1, tracker.GetCount())

	// Verify the connection is retrievable
	retrieved, exists := tracker.GetConnection("test-conn-1")
	assert.True(t, exists)
	assert.Equal(t, conn.ID, retrieved.ID)
	assert.Equal(t, conn.Type, retrieved.Type)
}

func TestWebSocketTracker_Unregister(t *testing.T) {
	tracker := NewWebSocketTracker()

	conn := &ConnectionInfo{
		ID:          "test-conn-1",
		Type:        "cerberus",
		ConnectedAt: time.Now(),
	}

	tracker.Register(conn)
	assert.Equal(t, 1, tracker.GetCount())

	tracker.Unregister("test-conn-1")
	assert.Equal(t, 0, tracker.GetCount())

	// Verify the connection is no longer retrievable
	_, exists := tracker.GetConnection("test-conn-1")
	assert.False(t, exists)
}

func TestWebSocketTracker_UnregisterNonExistent(t *testing.T) {
	tracker := NewWebSocketTracker()

	// Should not panic
	tracker.Unregister("non-existent-id")
	assert.Equal(t, 0, tracker.GetCount())
}

func TestWebSocketTracker_UpdateActivity(t *testing.T) {
	tracker := NewWebSocketTracker()

	initialTime := time.Now().Add(-1 * time.Hour)
	conn := &ConnectionInfo{
		ID:             "test-conn-1",
		Type:           "logs",
		ConnectedAt:    initialTime,
		LastActivityAt: initialTime,
	}

	tracker.Register(conn)

	// Wait a moment to ensure time difference
	time.Sleep(10 * time.Millisecond)

	tracker.UpdateActivity("test-conn-1")

	retrieved, exists := tracker.GetConnection("test-conn-1")
	require.True(t, exists)
	assert.True(t, retrieved.LastActivityAt.After(initialTime))
}

func TestWebSocketTracker_UpdateActivityNonExistent(t *testing.T) {
	tracker := NewWebSocketTracker()

	// Should not panic
	tracker.UpdateActivity("non-existent-id")
}

func TestWebSocketTracker_GetAllConnections(t *testing.T) {
	tracker := NewWebSocketTracker()

	conn1 := &ConnectionInfo{
		ID:          "conn-1",
		Type:        "logs",
		ConnectedAt: time.Now(),
	}
	conn2 := &ConnectionInfo{
		ID:          "conn-2",
		Type:        "cerberus",
		ConnectedAt: time.Now(),
	}

	tracker.Register(conn1)
	tracker.Register(conn2)

	connections := tracker.GetAllConnections()
	assert.Equal(t, 2, len(connections))

	// Verify both connections are present (order may vary)
	ids := make(map[string]bool)
	for _, conn := range connections {
		ids[conn.ID] = true
	}
	assert.True(t, ids["conn-1"])
	assert.True(t, ids["conn-2"])
}

func TestWebSocketTracker_GetStats(t *testing.T) {
	tracker := NewWebSocketTracker()

	now := time.Now()
	oldestTime := now.Add(-10 * time.Minute)

	conn1 := &ConnectionInfo{
		ID:          "conn-1",
		Type:        "logs",
		ConnectedAt: now,
	}
	conn2 := &ConnectionInfo{
		ID:          "conn-2",
		Type:        "cerberus",
		ConnectedAt: oldestTime,
	}
	conn3 := &ConnectionInfo{
		ID:          "conn-3",
		Type:        "logs",
		ConnectedAt: now.Add(-5 * time.Minute),
	}

	tracker.Register(conn1)
	tracker.Register(conn2)
	tracker.Register(conn3)

	stats := tracker.GetStats()
	assert.Equal(t, 3, stats.TotalActive)
	assert.Equal(t, 2, stats.LogsConnections)
	assert.Equal(t, 1, stats.CerberusConnections)
	assert.NotNil(t, stats.OldestConnection)
	assert.True(t, stats.OldestConnection.Equal(oldestTime))
	assert.False(t, stats.LastUpdated.IsZero())
}

func TestWebSocketTracker_GetStatsEmpty(t *testing.T) {
	tracker := NewWebSocketTracker()

	stats := tracker.GetStats()
	assert.Equal(t, 0, stats.TotalActive)
	assert.Equal(t, 0, stats.LogsConnections)
	assert.Equal(t, 0, stats.CerberusConnections)
	assert.Nil(t, stats.OldestConnection)
	assert.False(t, stats.LastUpdated.IsZero())
}

func TestWebSocketTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewWebSocketTracker()

	// Test concurrent registration
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			conn := &ConnectionInfo{
				ID:          fmt.Sprintf("conn-%d", id),
				Type:        "logs",
				ConnectedAt: time.Now(),
			}
			tracker.Register(conn)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, tracker.GetCount())

	// Test concurrent read
	for i := 0; i < 10; i++ {
		go func() {
			_ = tracker.GetAllConnections()
			_ = tracker.GetStats()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Test concurrent unregister
	for i := 0; i < 10; i++ {
		go func(id int) {
			tracker.Unregister(fmt.Sprintf("conn-%d", id))
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 0, tracker.GetCount())
}
