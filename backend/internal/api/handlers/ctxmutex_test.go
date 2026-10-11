package handlers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// holdLock takes mu for a test, waiting as long as needed.
func holdLock(mu *ctxMutex) {
	_ = mu.LockContext(context.Background())
}

func TestCtxMutexZeroValueLockUnlockTryLock(t *testing.T) {
	t.Parallel()
	var m ctxMutex
	require.True(t, m.TryLock())
	require.False(t, m.TryLock(), "a held lock cannot be taken again")
	m.Unlock()
	holdLock(&m)
	require.False(t, m.TryLock())
	m.Unlock()
	require.True(t, m.TryLock())
	m.Unlock()
}

func TestCtxMutexUnlockOfUnlockedPanics(t *testing.T) {
	t.Parallel()
	var m ctxMutex
	require.Panics(t, m.Unlock)
}

func TestCtxMutexLockContextFreeLock(t *testing.T) {
	t.Parallel()
	var m ctxMutex
	require.NoError(t, m.LockContext(context.Background()))
	require.False(t, m.TryLock())
	m.Unlock()
}

func TestCtxMutexLockContextEndedContextDoesNotTakeAFreeLock(t *testing.T) {
	t.Parallel()
	var m ctxMutex
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, m.LockContext(ctx), context.Canceled)
	require.True(t, m.TryLock(), "the failed wait must not have taken the lock")
	m.Unlock()
}

func TestCtxMutexCancelledWaiterLeavesTheHoldersLockHeld(t *testing.T) {
	t.Parallel()
	var m ctxMutex
	holdLock(&m)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.LockContext(ctx) }()
	select {
	case err := <-done:
		t.Fatalf("returned while the lock was held: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled waiter did not return")
	}
	require.False(t, m.TryLock(), "the holder's lock must still be held")
	m.Unlock()
	require.True(t, m.TryLock())
	m.Unlock()
}

func TestCtxMutexLockContextDeadline(t *testing.T) {
	t.Parallel()
	var m ctxMutex
	holdLock(&m)
	defer m.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, m.LockContext(ctx), context.DeadlineExceeded)
}

func TestCtxMutexWaiterAcquiresAfterUnlock(t *testing.T) {
	t.Parallel()
	var m ctxMutex
	holdLock(&m)
	done := make(chan error, 1)
	go func() { done <- m.LockContext(context.Background()) }()
	m.Unlock()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("waiter did not acquire the released lock")
	}
	m.Unlock()
}

func TestCtxMutexExcludesConcurrentSections(t *testing.T) {
	t.Parallel()
	var (
		m     ctxMutex
		wg    sync.WaitGroup
		count int
	)
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.LockContext(context.Background()); err != nil {
				return
			}
			defer m.Unlock()
			count++
		}()
	}
	wg.Wait()
	require.Equal(t, 50, count)
}
