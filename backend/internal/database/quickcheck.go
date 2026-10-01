package database

import (
	"path/filepath"
	"sync"
)

// QuickCheckOK is the verdict of a boot quick_check that found no problem.
const QuickCheckOK = "ok"

// quickCheckEntry tracks one boot quick_check. result is written before done is
// closed, so a reader that has received from done sees it without a lock.
type quickCheckEntry struct {
	done   chan struct{}
	once   sync.Once
	result string
}

// quickChecks maps the cleaned database path to its latest boot quick_check.
var quickChecks sync.Map

func quickCheckKey(dbPath string) string { return filepath.Clean(dbPath) }

// registerQuickCheck creates the pending entry Connect publishes before it
// launches the check, replacing the entry of an earlier Connect to the same path.
func registerQuickCheck(dbPath string) {
	quickChecks.Store(quickCheckKey(dbPath), &quickCheckEntry{done: make(chan struct{})})
}

// completeQuickCheck records the verdict and releases waiters. It is a no-op for
// an unregistered path and idempotent. An empty verdict means the check could
// not run.
func completeQuickCheck(dbPath, verdict string) {
	v, ok := quickChecks.Load(quickCheckKey(dbPath))
	if !ok {
		return
	}
	e := v.(*quickCheckEntry)
	e.once.Do(func() {
		e.result = verdict
		close(e.done)
	})
}

// QuickCheckStatus exposes the boot quick_check that Connect started for
// dbPath: done closes when the check has finished (on every exit path), and
// result then returns QuickCheckOK, the integrity message of a damaged
// database, or "" if the check could not run. Both are nil when Connect never
// registered the path.
func QuickCheckStatus(dbPath string) (done <-chan struct{}, result func() string) {
	v, ok := quickChecks.Load(quickCheckKey(dbPath))
	if !ok {
		return nil, nil
	}
	e := v.(*quickCheckEntry)
	return e.done, func() string {
		select {
		case <-e.done:
			return e.result
		default:
			return ""
		}
	}
}
