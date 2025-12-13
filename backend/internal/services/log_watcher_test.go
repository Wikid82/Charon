package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestNewLogWatcher verifies that NewLogWatcher creates a properly initialized instance.
func TestNewLogWatcher(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	assert.NotNil(t, watcher)
	assert.NotNil(t, watcher.subscribers)
	assert.Equal(t, "/tmp/test.log", watcher.logPath)
	assert.NotNil(t, watcher.ctx)
	assert.NotNil(t, watcher.cancel)
	assert.False(t, watcher.started)
}

// TestLogWatcherStartStop verifies that Start and Stop work correctly.
func TestLogWatcherStartStop(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "access.log")

	watcher := NewLogWatcher(logPath)

	// Start should succeed
	err := watcher.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, watcher.started)

	// Start should be idempotent
	err = watcher.Start(context.Background())
	require.NoError(t, err)

	// Stop should clean up
	watcher.Stop()
	assert.False(t, watcher.started)
	assert.Empty(t, watcher.subscribers)
}

// TestLogWatcherSubscribeUnsubscribe verifies subscriber management.
func TestLogWatcherSubscribeUnsubscribe(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	// Subscribe
	ch := watcher.Subscribe()
	assert.NotNil(t, ch)
	assert.Len(t, watcher.subscribers, 1)

	// Subscribe again
	ch2 := watcher.Subscribe()
	assert.NotNil(t, ch2)
	assert.Len(t, watcher.subscribers, 2)

	// Unsubscribe first
	watcher.Unsubscribe(ch)
	assert.Len(t, watcher.subscribers, 1)

	// Unsubscribe second
	watcher.Unsubscribe(ch2)
	assert.Empty(t, watcher.subscribers)
}

// TestLogWatcherBroadcast verifies that broadcast sends entries to all subscribers.
func TestLogWatcherBroadcast(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	ch1 := watcher.Subscribe()
	ch2 := watcher.Subscribe()

	entry := models.SecurityLogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     "info",
		ClientIP:  "192.168.1.100",
		Method:    "GET",
		URI:       "/api/test",
		Status:    200,
		Source:    "normal",
	}

	// Broadcast should send to both subscribers
	watcher.broadcast(entry)

	// Use timeout to prevent test hanging
	select {
	case received := <-ch1:
		assert.Equal(t, entry.ClientIP, received.ClientIP)
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for entry on ch1")
	}

	select {
	case received := <-ch2:
		assert.Equal(t, entry.ClientIP, received.ClientIP)
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for entry on ch2")
	}
}

// TestLogWatcherBroadcastNonBlocking verifies broadcast doesn't block on full channels.
func TestLogWatcherBroadcastNonBlocking(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")
	ch := watcher.Subscribe()

	// Fill the channel buffer
	for i := 0; i < 100; i++ {
		watcher.broadcast(models.SecurityLogEntry{
			Timestamp: time.Now().Format(time.RFC3339),
			Status:    200,
		})
	}

	// This should not block even though channel is full
	done := make(chan struct{})
	go func() {
		watcher.broadcast(models.SecurityLogEntry{
			Timestamp: time.Now().Format(time.RFC3339),
			Status:    201,
		})
		close(done)
	}()

	select {
	case <-done:
		// Good, broadcast didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("Broadcast blocked on full channel")
	}

	// Drain the channel
	for len(ch) > 0 {
		<-ch
	}
}

// TestParseLogEntryValidJSON verifies parsing of valid Caddy JSON log entries.
func TestParseLogEntryValidJSON(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	// Sample Caddy access log entry
	logLine := `{"level":"info","ts":1702406400.123,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","remote_port":"54321","method":"GET","uri":"/api/v1/test","host":"example.com","proto":"HTTP/2.0","headers":{"User-Agent":["Mozilla/5.0"]}},"status":200,"duration":0.001234,"size":512}`

	entry := watcher.ParseLogEntry(logLine)

	require.NotNil(t, entry)
	assert.Equal(t, "info", entry.Level)
	assert.Equal(t, "http.log.access", entry.Logger)
	assert.Equal(t, "192.168.1.100", entry.ClientIP)
	assert.Equal(t, "GET", entry.Method)
	assert.Equal(t, "/api/v1/test", entry.URI)
	assert.Equal(t, "example.com", entry.Host)
	assert.Equal(t, 200, entry.Status)
	assert.Equal(t, 0.001234, entry.Duration)
	assert.Equal(t, int64(512), entry.Size)
	assert.Equal(t, "Mozilla/5.0", entry.UserAgent)
	assert.Equal(t, "normal", entry.Source)
	assert.False(t, entry.Blocked)
}

// TestParseLogEntryInvalidJSON verifies handling of invalid JSON.
func TestParseLogEntryInvalidJSON(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	testCases := []struct {
		name string
		line string
	}{
		{"empty", ""},
		{"not json", "this is not json"},
		{"incomplete json", `{"level":"info"`},
		{"array instead of object", `["item1", "item2"]`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := watcher.ParseLogEntry(tc.line)
			assert.Nil(t, entry)
		})
	}
}

// TestParseLogEntryBlockedByWAF verifies detection of WAF blocked requests.
func TestParseLogEntryBlockedByWAF(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	// WAF blocked request (403 with waf logger)
	logLine := `{"level":"info","ts":1702406400.123,"logger":"http.handlers.waf","msg":"request blocked","request":{"remote_ip":"192.168.1.100","method":"POST","uri":"/api/admin","host":"example.com","headers":{}},"status":403,"duration":0.001,"size":0,"resp_headers":{"X-Coraza-Id":["942100"]}}`

	entry := watcher.ParseLogEntry(logLine)

	require.NotNil(t, entry)
	assert.Equal(t, 403, entry.Status)
	assert.True(t, entry.Blocked)
	assert.Equal(t, "waf", entry.Source)
	assert.Equal(t, "WAF rule triggered", entry.BlockReason)
	assert.Equal(t, "warn", entry.Level)
	assert.Equal(t, "942100", entry.Details["rule_id"])
}

// TestParseLogEntryBlockedByRateLimit verifies detection of rate-limited requests.
func TestParseLogEntryBlockedByRateLimit(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	// Rate limited request (429)
	logLine := `{"level":"info","ts":1702406400.123,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"/api/search","host":"example.com","headers":{}},"status":429,"duration":0.001,"size":0,"resp_headers":{"X-Ratelimit-Remaining":["0"],"X-Ratelimit-Reset":["60"]}}`

	entry := watcher.ParseLogEntry(logLine)

	require.NotNil(t, entry)
	assert.Equal(t, 429, entry.Status)
	assert.True(t, entry.Blocked)
	assert.Equal(t, "ratelimit", entry.Source)
	assert.Equal(t, "Rate limit exceeded", entry.BlockReason)
	assert.Equal(t, "warn", entry.Level)
	assert.Equal(t, "0", entry.Details["ratelimit_remaining"])
	assert.Equal(t, "60", entry.Details["ratelimit_reset"])
}

// TestParseLogEntry403CrowdSec verifies detection of CrowdSec blocked requests.
func TestParseLogEntry403CrowdSec(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	// CrowdSec blocked request
	logLine := `{"level":"info","ts":1702406400.123,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"/","host":"example.com","headers":{}},"status":403,"duration":0.001,"size":0,"resp_headers":{"X-Crowdsec-Decision":["ban"]}}`

	entry := watcher.ParseLogEntry(logLine)

	require.NotNil(t, entry)
	assert.True(t, entry.Blocked)
	assert.Equal(t, "crowdsec", entry.Source)
	assert.Equal(t, "CrowdSec decision", entry.BlockReason)
}

// TestParseLogEntry401Auth verifies detection of authentication failures.
func TestParseLogEntry401Auth(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	logLine := `{"level":"info","ts":1702406400.123,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"POST","uri":"/api/login","host":"example.com","headers":{}},"status":401,"duration":0.001,"size":0}`

	entry := watcher.ParseLogEntry(logLine)

	require.NotNil(t, entry)
	assert.Equal(t, 401, entry.Status)
	assert.Equal(t, "warn", entry.Level)
	assert.Equal(t, "auth", entry.Source)
	assert.Equal(t, true, entry.Details["auth_failure"])
}

// TestParseLogEntry500Error verifies detection of server errors.
func TestParseLogEntry500Error(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	logLine := `{"level":"info","ts":1702406400.123,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"/api/crash","host":"example.com","headers":{}},"status":500,"duration":0.001,"size":0}`

	entry := watcher.ParseLogEntry(logLine)

	require.NotNil(t, entry)
	assert.Equal(t, 500, entry.Status)
	assert.Equal(t, "error", entry.Level)
}

// TestHasHeader verifies case-insensitive header lookup.
func TestHasHeader(t *testing.T) {
	t.Parallel()

	headers := map[string][]string{
		"Content-Type":   {"application/json"},
		"X-Custom-Header": {"value"},
	}

	assert.True(t, hasHeader(headers, "Content-Type"))
	assert.True(t, hasHeader(headers, "content-type"))
	assert.True(t, hasHeader(headers, "CONTENT-TYPE"))
	assert.True(t, hasHeader(headers, "X-Custom-Header"))
	assert.True(t, hasHeader(headers, "x-custom-header"))
	assert.False(t, hasHeader(headers, "X-Missing"))
	assert.False(t, hasHeader(nil, "Content-Type"))
}

// TestLogWatcherIntegration tests the full flow of tailing a log file.
func TestLogWatcherIntegration(t *testing.T) {
	t.Parallel()

	// Create temp directory and log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "access.log")

	// Create the log file
	file, err := os.Create(logPath)
	require.NoError(t, err)
	defer file.Close()

	// Create and start watcher
	watcher := NewLogWatcher(logPath)
	err = watcher.Start(context.Background())
	require.NoError(t, err)
	defer watcher.Stop()

	// Subscribe
	ch := watcher.Subscribe()

	// Give the watcher time to open the file and seek to end
	time.Sleep(200 * time.Millisecond)

	// Write a log entry to the file
	logEntry := models.CaddyAccessLog{
		Level:  "info",
		Ts:     float64(time.Now().Unix()),
		Logger: "http.log.access",
		Msg:    "handled request",
		Status: 200,
	}
	logEntry.Request.RemoteIP = "10.0.0.1"
	logEntry.Request.Method = "GET"
	logEntry.Request.URI = "/test"
	logEntry.Request.Host = "test.example.com"

	logJSON, err := json.Marshal(logEntry)
	require.NoError(t, err)

	_, err = file.WriteString(string(logJSON) + "\n")
	require.NoError(t, err)
	file.Sync()

	// Wait for the entry to be broadcast
	select {
	case received := <-ch:
		assert.Equal(t, "10.0.0.1", received.ClientIP)
		assert.Equal(t, "GET", received.Method)
		assert.Equal(t, "/test", received.URI)
		assert.Equal(t, "test.example.com", received.Host)
		assert.Equal(t, 200, received.Status)
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for log entry")
	}
}

// TestLogWatcherConcurrentSubscribers tests concurrent subscribe/unsubscribe operations.
func TestLogWatcherConcurrentSubscribers(t *testing.T) {
	t.Parallel()

	watcher := NewLogWatcher("/tmp/test.log")

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrently subscribe and unsubscribe
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := watcher.Subscribe()
			time.Sleep(10 * time.Millisecond)
			watcher.Unsubscribe(ch)
		}()
	}

	// Also broadcast concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			watcher.broadcast(models.SecurityLogEntry{
				Timestamp: time.Now().Format(time.RFC3339),
				Status:    idx,
			})
		}(i)
	}

	wg.Wait()

	// Should not panic and subscribers should be empty or minimal
	assert.LessOrEqual(t, len(watcher.subscribers), numGoroutines)
}

// TestLogWatcherMissingFile tests behavior when log file doesn't exist.
func TestLogWatcherMissingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "nonexistent", "access.log")

	watcher := NewLogWatcher(logPath)
	err := watcher.Start(context.Background())
	require.NoError(t, err)

	// Give it time to attempt reading
	time.Sleep(200 * time.Millisecond)

	// Should still be running (just waiting for file)
	assert.True(t, watcher.started)

	watcher.Stop()
}

// TestMin verifies the min helper function.
func TestMin(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, min(1, 2))
	assert.Equal(t, 1, min(2, 1))
	assert.Equal(t, 0, min(0, 0))
	assert.Equal(t, -1, min(-1, 0))
}
