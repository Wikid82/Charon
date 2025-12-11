// Package logger provides logging functionality with broadcast capabilities for real-time log streaming.
package logger

import (
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

var _log = logrus.New()
var _broadcastHook *BroadcastHook

// Init initializes the global logger with output writer and debug level.
func Init(debug bool, out io.Writer) {
	if out == nil {
		out = os.Stdout
	}
	_log.SetOutput(out)
	if debug {
		_log.SetLevel(logrus.DebugLevel)
		_log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	} else {
		_log.SetLevel(logrus.InfoLevel)
		_log.SetFormatter(&logrus.JSONFormatter{})
	}

	// Initialize and add broadcast hook
	_broadcastHook = NewBroadcastHook()
	_log.AddHook(_broadcastHook)
}

// Log returns a standard logger entry to use across packages.
func Log() *logrus.Entry {
	return logrus.NewEntry(_log)
}

// WithFields returns a logger entry with provided fields.
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Log().WithFields(fields)
}

// GetBroadcastHook returns the global broadcast hook instance.
func GetBroadcastHook() *BroadcastHook {
	if _broadcastHook == nil {
		_broadcastHook = NewBroadcastHook()
		_log.AddHook(_broadcastHook)
	}
	return _broadcastHook
}

// BroadcastHook implements logrus.Hook to broadcast log entries to active listeners.
type BroadcastHook struct {
	mu        sync.RWMutex
	listeners map[string]chan *logrus.Entry
}

// NewBroadcastHook creates a new BroadcastHook instance.
func NewBroadcastHook() *BroadcastHook {
	return &BroadcastHook{
		listeners: make(map[string]chan *logrus.Entry),
	}
}

// Levels returns all log levels that this hook should fire for.
func (h *BroadcastHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire broadcasts the log entry to all active listeners.
func (h *BroadcastHook) Fire(entry *logrus.Entry) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Broadcast to all listeners (non-blocking)
	for _, ch := range h.listeners {
		select {
		case ch <- entry:
		default:
			// Skip if channel is full (prevents blocking)
		}
	}
	return nil
}

// Subscribe adds a new listener and returns a channel for receiving log entries.
// The caller must call Unsubscribe when done to prevent resource leaks.
func (h *BroadcastHook) Subscribe(id string) <-chan *logrus.Entry {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan *logrus.Entry, 100) // Buffer to prevent blocking
	h.listeners[id] = ch
	return ch
}

// Unsubscribe removes a listener and closes its channel.
func (h *BroadcastHook) Unsubscribe(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, ok := h.listeners[id]; ok {
		close(ch)
		delete(h.listeners, id)
	}
}

// ActiveListeners returns the count of active listeners.
func (h *BroadcastHook) ActiveListeners() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.listeners)
}

// ListenerIDs returns the IDs of all active listeners. Intended for tests/observability only.
func (h *BroadcastHook) ListenerIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids := make([]string, 0, len(h.listeners))
	for id := range h.listeners {
		ids = append(ids, id)
	}

	return ids
}
