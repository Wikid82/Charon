package hecate

import "sync"

// RingBuffer is a thread-safe circular log buffer with a fixed maximum capacity.
// When full, the oldest entry is dropped on Write.
type RingBuffer struct {
	mu          sync.RWMutex
	buf         []string
	tail        int // index of next write position
	count       int // number of valid entries
	cap         int
	subscribers []chan string
	closed      bool
}

// NewRingBuffer creates a new RingBuffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buf: make([]string, capacity),
		cap: capacity,
	}
}

// Write appends a line to the buffer, evicting the oldest entry when full.
// Blocked subscribers receive the new line on a best-effort basis.
func (rb *RingBuffer) Write(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return
	}

	rb.buf[rb.tail] = line
	rb.tail = (rb.tail + 1) % rb.cap
	if rb.count < rb.cap {
		rb.count++
	}

	for _, ch := range rb.subscribers {
		select {
		case ch <- line:
		default:
			// Subscriber channel full; drop notification rather than blocking writer.
		}
	}
}

// ReadAll returns all buffered lines in chronological order (oldest first).
func (rb *RingBuffer) ReadAll() []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	result := make([]string, rb.count)
	if rb.count < rb.cap {
		// Buffer not yet full: valid entries are buf[0..count-1].
		copy(result, rb.buf[:rb.count])
	} else {
		// Buffer full: oldest entry is at rb.tail (the next write position).
		n := copy(result, rb.buf[rb.tail:])
		copy(result[n:], rb.buf[:rb.tail])
	}
	return result
}

// Subscribe returns a channel that receives new lines as they are written.
// Callers should read from the channel promptly; slow consumers may miss events.
func (rb *RingBuffer) Subscribe() <-chan string {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	ch := make(chan string, 64)
	rb.subscribers = append(rb.subscribers, ch)
	return ch
}

// Close closes all subscriber channels. Further writes are silently dropped.
func (rb *RingBuffer) Close() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return
	}
	rb.closed = true
	for _, ch := range rb.subscribers {
		close(ch)
	}
	rb.subscribers = nil
}
