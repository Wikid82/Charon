package services

import "sync"

// StreamQuota caps concurrent live streams (WebSocket feeds) per key and in total. A key is normally the caller's
// user ID. A nil *StreamQuota admits every stream, so handlers built without a quota keep working.
type StreamQuota struct {
	mu      sync.Mutex
	perKey  int
	total   int
	active  int
	perUser map[string]int
}

// NewStreamQuota returns a quota that allows perKey concurrent streams for one key and total across all keys.
func NewStreamQuota(perKey, total int) *StreamQuota {
	return &StreamQuota{perKey: perKey, total: total, perUser: make(map[string]int)}
}

// Acquire reserves one stream slot for key. ok is false when the key or the total is at its limit; the returned
// release is then a no-op. When ok is true, release frees the slot and is safe to call more than once.
func (q *StreamQuota) Acquire(key string) (release func(), ok bool) {
	if q == nil {
		return func() {}, true
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.active >= q.total || q.perUser[key] >= q.perKey {
		return func() {}, false
	}
	q.active++
	q.perUser[key]++

	var once sync.Once
	return func() {
		once.Do(func() { q.release(key) })
	}, true
}

// release frees one slot held by key and drops the key once it holds none.
func (q *StreamQuota) release(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.active--
	if q.perUser[key] <= 1 {
		delete(q.perUser, key)
		return
	}
	q.perUser[key]--
}

// Live stream limits the API applies to its WebSocket feeds: per user and in total.
const (
	MaxStreamsPerUser = 8
	MaxStreamsTotal   = 64
)
