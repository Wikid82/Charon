package handlers

import (
	"context"
	"sync"
)

// ctxMutex is a mutual-exclusion lock whose waiters can give up when their context ends. The zero
// value is an unlocked mutex, so it works as a plain struct field. A ctxMutex must not be copied.
//
// Unlike sync.Mutex it is not owned by a goroutine, but the contract is the same: only the holder
// calls Unlock, and a LockContext that returned an error did NOT take the lock, so the caller must
// return before any deferred Unlock is registered.
type ctxMutex struct {
	once sync.Once
	slot chan struct{} // holds one token while the lock is taken
}

func (m *ctxMutex) init() {
	m.once.Do(func() { m.slot = make(chan struct{}, 1) })
}

// TryLock takes the lock if it is free and reports whether it did.
func (m *ctxMutex) TryLock() bool {
	m.init()
	select {
	case m.slot <- struct{}{}:
		return true
	default:
		return false
	}
}

// LockContext waits for the lock until ctx ends. It returns ctx.Err() without having taken the lock
// when ctx ends first; an already-ended ctx never takes a free lock.
func (m *ctxMutex) LockContext(ctx context.Context) error {
	m.init()
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case m.slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Unlock releases the lock and panics if it is not held, like sync.Mutex does (there as a fatal error).
func (m *ctxMutex) Unlock() {
	m.init()
	select {
	case <-m.slot:
	default:
		panic("handlers: unlock of unlocked ctxMutex")
	}
}
