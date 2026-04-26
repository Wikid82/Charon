package hecate

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRingBuffer_WriteAndReadAll(t *testing.T) {
	tests := []struct {
		name     string
		cap      int
		writes   []string
		expected []string
	}{
		{
			name:     "empty buffer",
			cap:      5,
			writes:   nil,
			expected: nil,
		},
		{
			name:     "partial fill",
			cap:      5,
			writes:   []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "exact fill",
			cap:      3,
			writes:   []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "overflow drops oldest",
			cap:      3,
			writes:   []string{"a", "b", "c", "d"},
			expected: []string{"b", "c", "d"},
		},
		{
			name:     "double overflow",
			cap:      3,
			writes:   []string{"a", "b", "c", "d", "e", "f", "g"},
			expected: []string{"e", "f", "g"},
		},
		{
			name:     "capacity one",
			cap:      1,
			writes:   []string{"a", "b", "c"},
			expected: []string{"c"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rb := NewRingBuffer(tc.cap)
			for _, line := range tc.writes {
				rb.Write(line)
			}
			assert.Equal(t, tc.expected, rb.ReadAll())
		})
	}
}

func TestRingBuffer_Subscribe_ReceivesWrites(t *testing.T) {
	rb := NewRingBuffer(10)
	ch := rb.Subscribe()

	rb.Write("hello")
	rb.Write("world")

	assert.Equal(t, "hello", <-ch)
	assert.Equal(t, "world", <-ch)
}

func TestRingBuffer_Subscribe_ChannelClosed_OnClose(t *testing.T) {
	rb := NewRingBuffer(10)
	ch := rb.Subscribe()

	rb.Close()

	_, open := <-ch
	assert.False(t, open, "subscriber channel should be closed")
}

func TestRingBuffer_WriteAfterClose_Ignored(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Close()
	rb.Write("should be ignored") // must not panic
	assert.Nil(t, rb.ReadAll())
}

func TestRingBuffer_DoubleClose_NoPanic(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Close()
	require.NotPanics(t, func() { rb.Close() })
}

func TestRingBuffer_ConcurrentWrites(t *testing.T) {
	rb := NewRingBuffer(1000)
	var wg sync.WaitGroup
	workers := 10
	writesPerWorker := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerWorker; j++ {
				rb.Write("line")
			}
		}(i)
	}
	wg.Wait()

	all := rb.ReadAll()
	assert.Len(t, all, workers*writesPerWorker)
}

func TestRingBuffer_ConcurrentWriteAndRead(t *testing.T) {
	rb := NewRingBuffer(100)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			rb.Write("x")
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = rb.ReadAll()
			time.Sleep(10 * time.Microsecond)
		}
	}()

	wg.Wait()
}

func TestRingBuffer_Subscribe_SlowConsumerDropsEvents(t *testing.T) {
	rb := NewRingBuffer(100)
	// Subscriber channel capacity is 64; writing 200 lines must not block the writer.
	ch := rb.Subscribe()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			rb.Write("line")
		}
		close(done)
	}()

	select {
	case <-done:
		// drain whatever arrived
		for len(ch) > 0 {
			<-ch
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Write goroutine blocked by slow subscriber")
	}
}
