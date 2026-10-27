package testutil

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
)

// gateWaitTimeout bounds how long a test waits for a gated insert to start, so a broken test fails instead of hanging.
// It is a variable so the timeout path can be exercised without waiting the full bound.
var gateWaitTimeout = 10 * time.Second

// gateSeq gives every registered gate callback a unique name, so two gates on one database never collide.
var gateSeq atomic.Uint64

// GateCreate makes the next insert of T on db wait before its transaction opens, so a test can observe the state
// between a caller's decision and the row it records. entered is closed when an insert of T starts waiting. The
// insert resumes when release is called (also a cleanup). When insertErr is non-nil the resumed insert fails with
// it and writes nothing. Inserts of other models are not affected.
func GateCreate[T any](t testing.TB, db *gorm.DB, insertErr error) (entered <-chan struct{}, release func()) {
	t.Helper()
	enteredCh, gate := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	release = func() { releaseOnce.Do(func() { close(gate) }) }
	t.Cleanup(release)

	name := fmt.Sprintf("testutil:gate_create_%d", gateSeq.Add(1))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(name) })
	err := db.Callback().Create().Before("gorm:begin_transaction").Register(name, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*T); !ok {
			return
		}
		enterOnce.Do(func() { close(enteredCh) })
		<-gate
		if insertErr != nil {
			_ = tx.AddError(insertErr) // AddError also sets tx.Error, which aborts the insert
		}
	})
	if err != nil {
		t.Fatalf("registering the create gate: %v", err)
	}
	return enteredCh, release
}

// AwaitGate waits until the gated insert has started, failing the test when it does not start in time.
func AwaitGate(t testing.TB, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(gateWaitTimeout):
		t.Fatal("the gated insert never started")
	}
}
