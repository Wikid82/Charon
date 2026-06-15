// Package services provides business logic services for the application.
package services

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// LogWatcher provides real-time tailing of Caddy access logs.
// It is a singleton service that can have multiple WebSocket clients subscribe
// to receive security-relevant log entries in real-time.
type LogWatcher struct {
	mu          sync.RWMutex
	subscribers map[chan models.SecurityLogEntry]struct{}
	logPath     string
	ctx         context.Context
	cancel      context.CancelFunc
	started     bool
	ingester    *StatsIngester // optional fan-out to StatsIngester; nil when not registered
}

// NewLogWatcher creates a new LogWatcher instance for the given log file path.
func NewLogWatcher(logPath string) *LogWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &LogWatcher{
		subscribers: make(map[chan models.SecurityLogEntry]struct{}),
		logPath:     logPath,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins tailing the log file. This method is idempotent.
func (w *LogWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return nil
	}
	w.started = true
	w.mu.Unlock()

	go w.tailFile()
	logger.Log().WithField("path", w.logPath).Info("LogWatcher started")
	return nil
}

// Stop halts the log watcher and closes all subscriber channels.
func (w *LogWatcher) Stop() {
	w.cancel()
	w.mu.Lock()
	defer w.mu.Unlock()

	for ch := range w.subscribers {
		close(ch)
		delete(w.subscribers, ch)
	}
	w.started = false
	logger.Log().Info("LogWatcher stopped")
}

// RegisterIngester wires a StatsIngester into the fan-out path.
// Every parsed log entry will be forwarded to the ingester via Send (non-blocking).
// This must be called before Start for the ingester to receive all entries.
func (w *LogWatcher) RegisterIngester(ing *StatsIngester) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ingester = ing
	logger.Log().Debug("StatsIngester registered with LogWatcher")
}

// Subscribe adds a new subscriber and returns a channel for receiving log entries.
// The caller is responsible for calling Unsubscribe when done.
func (w *LogWatcher) Subscribe() <-chan models.SecurityLogEntry {
	w.mu.Lock()
	defer w.mu.Unlock()

	ch := make(chan models.SecurityLogEntry, 100)
	w.subscribers[ch] = struct{}{}
	logger.Log().WithField("subscriber_count", len(w.subscribers)).Debug("New subscriber added to LogWatcher")
	return ch
}

// Unsubscribe removes a subscriber channel.
func (w *LogWatcher) Unsubscribe(ch <-chan models.SecurityLogEntry) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Type assert to get the writable channel for map lookup
	// The channel passed in is receive-only, but we stored the bidirectional channel
	for subCh := range w.subscribers {
		// Compare the underlying channel - convert bidirectional to receive-only for comparison
		recvOnlyCh := (<-chan models.SecurityLogEntry)(subCh) //nolint:gocritic // Type conversion required for channel comparison
		if recvOnlyCh == ch {
			close(subCh)
			delete(w.subscribers, subCh)
			logger.Log().WithField("subscriber_count", len(w.subscribers)).Debug("Subscriber removed from LogWatcher")
			return
		}
	}
}

// broadcast sends a log entry to all subscribers and, if registered, to the StatsIngester.
// Non-blocking: if a subscriber's channel is full, the entry is dropped for that subscriber.
func (w *LogWatcher) broadcast(entry models.SecurityLogEntry) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for ch := range w.subscribers {
		select {
		case ch <- entry:
			// Successfully sent
		default:
			// Channel is full, skip (prevents blocking other subscribers)
		}
	}

	// Fan-out to StatsIngester (non-blocking via Send).
	if w.ingester != nil {
		w.ingester.Send(entry)
	}
}

// tailFile continuously reads new entries from the log file.
// It handles file rotation and missing files gracefully.
func (w *LogWatcher) tailFile() {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		// Wait for file to exist
		if _, err := os.Stat(w.logPath); os.IsNotExist(err) {
			logger.Log().WithField("path", w.logPath).Debug("Log file not found, waiting...")
			time.Sleep(time.Second)
			continue
		}

		// Open the file
		file, err := os.Open(w.logPath)
		if err != nil {
			logger.Log().WithError(err).WithField("path", w.logPath).Error("Failed to open log file for tailing")
			time.Sleep(time.Second)
			continue
		}

		// Seek to end of file (we only want new entries)
		if _, err := file.Seek(0, io.SeekEnd); err != nil {
			logger.Log().WithError(err).Warn("Failed to seek to end of log file")
		}

		w.readLoop(file)
		if err := file.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close log file")
		}

		// Brief pause before reopening (handles log rotation)
		time.Sleep(time.Second)
	}
}

// readLoop reads lines from the file until EOF or error.
func (w *LogWatcher) readLoop(file *os.File) {
	reader := bufio.NewReader(file)
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// No new data, wait and retry
				time.Sleep(100 * time.Millisecond)
				continue
			}
			// File may have been rotated or truncated
			logger.Log().WithError(err).Debug("Error reading log file, will reopen")
			return
		}

		// Skip empty lines
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		entry := w.ParseLogEntry(line)
		if entry != nil {
			w.broadcast(*entry)
		}
	}
}

// ParseLogEntry converts a Caddy JSON log line into a SecurityLogEntry.
// Returns nil if the line cannot be parsed.
func (w *LogWatcher) ParseLogEntry(line string) *models.SecurityLogEntry {
	var caddyLog models.CaddyAccessLog
	if err := json.Unmarshal([]byte(line), &caddyLog); err != nil {
		logger.Log().WithError(err).WithField("line", line[:minInt(100, len(line))]).Debug("Failed to parse log line as JSON")
		return nil
	}

	// Convert Caddy timestamp (Unix float) to RFC3339
	timestamp := time.Unix(int64(caddyLog.Ts), int64((caddyLog.Ts-float64(int64(caddyLog.Ts)))*1e9))

	// Extract User-Agent from headers
	userAgent := ""
	if ua, ok := caddyLog.Request.Headers["User-Agent"]; ok && len(ua) > 0 {
		userAgent = ua[0]
	}

	entry := &models.SecurityLogEntry{
		Timestamp: timestamp.Format(time.RFC3339),
		Level:     caddyLog.Level,
		Logger:    caddyLog.Logger,
		ClientIP:  caddyLog.Request.RemoteIP,
		Method:    caddyLog.Request.Method,
		URI:       caddyLog.Request.URI,
		Status:    caddyLog.Status,
		Duration:  caddyLog.Duration,
		Size:      int64(caddyLog.Size),
		UserAgent: userAgent,
		Host:      caddyLog.Request.Host,
		Source:    "normal",
		Blocked:   false,
		Details:   make(map[string]any),
	}

	// Detect security events based on status codes and response headers
	w.detectSecurityEvent(entry, &caddyLog)

	return entry
}

// detectSecurityEvent analyzes the log entry and sets security-related fields.
func (w *LogWatcher) detectSecurityEvent(entry *models.SecurityLogEntry, caddyLog *models.CaddyAccessLog) {
	loggerLower := strings.ToLower(caddyLog.Logger)

	// Check for WAF/Coraza indicators (highest priority for 403s)
	if strings.Contains(loggerLower, "waf") ||
		strings.Contains(loggerLower, "coraza") ||
		hasHeader(caddyLog.RespHeaders, "X-Coraza-Id") ||
		hasHeader(caddyLog.RespHeaders, "X-Coraza-Rule-Id") {
		entry.Blocked = true
		entry.Source = "waf"
		entry.Level = "warn"
		entry.BlockReason = "WAF rule triggered"

		// Try to extract rule ID from headers
		if ruleID, ok := caddyLog.RespHeaders["X-Coraza-Id"]; ok && len(ruleID) > 0 {
			entry.Details["rule_id"] = ruleID[0]
		}
		if ruleID, ok := caddyLog.RespHeaders["X-Coraza-Rule-Id"]; ok && len(ruleID) > 0 {
			entry.Details["rule_id"] = ruleID[0]
		}
		return
	}

	// Check for CrowdSec indicators
	if strings.Contains(loggerLower, "crowdsec") ||
		strings.Contains(loggerLower, "bouncer") ||
		hasHeader(caddyLog.RespHeaders, "X-Crowdsec-Decision") ||
		hasHeader(caddyLog.RespHeaders, "X-Crowdsec-Origin") {
		entry.Blocked = true
		entry.Source = "crowdsec"
		entry.Level = "warn"
		entry.BlockReason = "CrowdSec decision"

		// Extract CrowdSec-specific headers
		if origin, ok := caddyLog.RespHeaders["X-Crowdsec-Origin"]; ok && len(origin) > 0 {
			entry.Details["crowdsec_origin"] = origin[0]
		}
		return
	}

	// Check for ACL blocks
	if strings.Contains(loggerLower, "acl") ||
		hasHeader(caddyLog.RespHeaders, "X-Acl-Denied") ||
		hasHeader(caddyLog.RespHeaders, "X-Blocked-By-Acl") {
		entry.Blocked = true
		entry.Source = "acl"
		entry.Level = "warn"
		entry.BlockReason = "Access list denied"
		return
	}

	// Check for rate limiting (429 Too Many Requests)
	if caddyLog.Status == 429 {
		entry.Blocked = true
		entry.Source = "ratelimit"
		entry.Level = "warn"
		entry.BlockReason = "Rate limit exceeded"

		// Extract rate limit headers if present
		if remaining, ok := caddyLog.RespHeaders["X-Ratelimit-Remaining"]; ok && len(remaining) > 0 {
			entry.Details["ratelimit_remaining"] = remaining[0]
		}
		if reset, ok := caddyLog.RespHeaders["X-Ratelimit-Reset"]; ok && len(reset) > 0 {
			entry.Details["ratelimit_reset"] = reset[0]
		}
		if limit, ok := caddyLog.RespHeaders["X-Ratelimit-Limit"]; ok && len(limit) > 0 {
			entry.Details["ratelimit_limit"] = limit[0]
		}
		return
	}

	// Check for other 403s (generic security block)
	if caddyLog.Status == 403 {
		entry.Blocked = true
		entry.Source = "cerberus"
		entry.Level = "warn"
		entry.BlockReason = "Access denied"
		return
	}

	// Check for authentication failures
	if caddyLog.Status == 401 {
		entry.Level = "warn"
		entry.Source = "auth"
		entry.Details["auth_failure"] = true
		return
	}

	// Check for server errors
	if caddyLog.Status >= 500 {
		entry.Level = "error"
		return
	}

	// Normal traffic - set appropriate level based on status
	entry.Source = "normal"
	entry.Blocked = false
	if caddyLog.Status >= 400 {
		entry.Level = "warn"
	} else {
		entry.Level = "info"
	}
}

// hasHeader checks if a header map contains a specific key (case-insensitive).
func hasHeader(headers map[string][]string, key string) bool {
	if headers == nil {
		return false
	}
	// Direct lookup first
	if _, ok := headers[key]; ok {
		return true
	}
	// Case-insensitive fallback
	for k := range headers {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
