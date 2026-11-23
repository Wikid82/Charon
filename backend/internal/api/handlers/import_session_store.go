package handlers

import (
	"sync"
	"time"
)

const (
	// importSessionTTL is how long a parsed JSON or NPM import preview waits for its commit.
	importSessionTTL = time.Hour
	// maxImportSessions bounds the previews one store holds; storing another evicts the oldest.
	maxImportSessions = 4
)

// importSessionEntry is one stored preview and the time it was stored.
type importSessionEntry[T any] struct {
	value    T
	storedAt time.Time
}

// importSessionStore holds parsed import previews keyed by session UUID until they are committed, cancelled or
// expire. Expired entries are dropped on every access. When the store is full, the oldest entry is evicted.
type importSessionStore[T any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	max     int
	now     func() time.Time
	entries map[string]importSessionEntry[T]
}

// newImportSessionStore returns a store that holds at most maxEntries entries. A bound below one would leave Put with
// no room for its entry, so the bound is at least one.
func newImportSessionStore[T any](ttl time.Duration, maxEntries int) *importSessionStore[T] {
	if maxEntries < 1 {
		maxEntries = 1
	}
	return &importSessionStore[T]{
		ttl:     ttl,
		max:     maxEntries,
		now:     time.Now,
		entries: make(map[string]importSessionEntry[T]),
	}
}

// Put stores v under id, replacing any entry with that id.
func (s *importSessionStore[T]) Put(id string, v T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	delete(s.entries, id)
	for len(s.entries) >= s.max {
		s.evictOldestLocked()
	}
	s.entries[id] = importSessionEntry[T]{value: v, storedAt: s.now()}
}

// Get returns the value stored under id. An expired or missing entry reports false.
func (s *importSessionStore[T]) Get(id string) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	entry, ok := s.entries[id]
	return entry.value, ok
}

// Delete removes the entry stored under id, if any.
func (s *importSessionStore[T]) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	delete(s.entries, id)
}

// sweepLocked drops every entry older than the TTL. The caller holds s.mu.
func (s *importSessionStore[T]) sweepLocked() {
	cutoff := s.now().Add(-s.ttl)
	for id, entry := range s.entries {
		if entry.storedAt.Before(cutoff) {
			delete(s.entries, id)
		}
	}
}

// evictOldestLocked drops the entry stored first. The caller holds s.mu and the store is not empty.
func (s *importSessionStore[T]) evictOldestLocked() {
	var oldestID string
	var oldest time.Time
	first := true
	for id, entry := range s.entries {
		if first || entry.storedAt.Before(oldest) {
			oldestID, oldest, first = id, entry.storedAt, false
		}
	}
	delete(s.entries, oldestID)
}
