package crowdsec

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// errPresetApplyCancelledPending stands in for ErrPresetApplyCancelled until the Apply options refactor
// (PR 6 commit 2) adds it. Replace this variable with the real sentinel in that commit; the cancel
// subtests below stay skipped until then.
var errPresetApplyCancelledPending error

// rollbackExit is one place in HubService.Apply that restores the snapshot after the snapshot was taken.
// stall blocks until the apply context ends (a deadline expiring or the caller cancelling) and then returns
// the scripted answer, so the exit is reached with a finished context.
type rollbackExit struct {
	name string
	// script answers the command; stall must be called at the step that should outlast the context.
	script func(e *applyEnv, stall func(), cmd string, n int) *cscliReply
}

// olderInstalled is an inspect answer for an unmodified, older installed version, which sends Apply to upgrade.
func olderInstalled() *cscliReply {
	return inspectReply(true, false, false, strings.Repeat("1", 64))
}

// Inspect run 0 is the pre-flight inspect before the snapshot, run 1 the first one after the install and run 2
// the one after an upgrade; only the latter two sit behind a rollback.
//
// applyRollbackExits maps every rollback exit of Apply (hub_apply.go) to a script that reaches it:
//
//	rollBack(installFailure)        install fails
//	rollBack(inspect error)         first inspect after the install fails
//	rollBack(ErrPresetCacheStale)   item reported as not installed
//	rollBack(localModified)         item reported as edited
//	rollBackMismatch                first inspect shows a different hash
//	rollBack(upgrade error)         upgrade fails
//	rollBack(second inspect error)  inspect after the upgrade fails
//	rollBack(localModified)         item edited after the upgrade
//	rollBackMismatch                hash differs after the upgrade
//	rollBack(checkDependencies)     dependency check fails
func applyRollbackExits() []rollbackExit {
	failed := &cscliReply{err: errors.New("exit status 1")}
	return []rollbackExit{
		{"install fails", func(e *applyEnv, stall func(), cmd string, _ int) *cscliReply {
			if cmd == applyInstall {
				e.onInstall()
				stall()
				return failed
			}
			return nil
		}},
		{"first inspect fails", func(_ *applyEnv, stall func(), cmd string, n int) *cscliReply {
			if cmd == applyInspect && n == 1 {
				stall()
				return failed
			}
			return nil
		}},
		{"item not installed", func(_ *applyEnv, stall func(), cmd string, n int) *cscliReply {
			if cmd == applyInspect && n == 1 {
				stall()
				return inspectReply(false, false, true, "")
			}
			return nil
		}},
		{"item edited", func(_ *applyEnv, stall func(), cmd string, n int) *cscliReply {
			if cmd == applyInspect && n == 1 {
				stall()
				return inspectReply(true, true, true, strings.Repeat("2", 64))
			}
			return nil
		}},
		{"hash mismatch", func(_ *applyEnv, stall func(), cmd string, n int) *cscliReply {
			if cmd == applyInspect && n == 1 {
				stall()
				return inspectReply(true, false, true, strings.Repeat("3", 64))
			}
			return nil
		}},
		{"upgrade fails", func(_ *applyEnv, stall func(), cmd string, n int) *cscliReply {
			switch {
			case cmd == applyInspect && n == 1:
				return olderInstalled()
			case cmd == applyUpgrade:
				stall()
				return failed
			}
			return nil
		}},
		{"inspect after upgrade fails", func(_ *applyEnv, stall func(), cmd string, n int) *cscliReply {
			switch {
			case cmd == applyInspect && n == 1:
				return olderInstalled()
			case cmd == applyInspect && n >= 2:
				stall()
				return failed
			}
			return nil
		}},
		{"item edited after upgrade", func(_ *applyEnv, stall func(), cmd string, n int) *cscliReply {
			switch {
			case cmd == applyInspect && n == 1:
				return olderInstalled()
			case cmd == applyInspect && n >= 2:
				stall()
				return inspectReply(true, true, false, strings.Repeat("2", 64))
			}
			return nil
		}},
		{"hash mismatch after upgrade", func(_ *applyEnv, stall func(), cmd string, n int) *cscliReply {
			switch {
			case cmd == applyInspect && n == 1:
				return olderInstalled()
			case cmd == applyInspect && n >= 2:
				stall()
				return inspectReply(true, false, false, strings.Repeat("3", 64))
			}
			return nil
		}},
		{"dependency check fails", func(_ *applyEnv, stall func(), cmd string, _ int) *cscliReply {
			if cmd == applyList {
				stall()
				return failed
			}
			return nil
		}},
	}
}

// requireRolledBackKeepingEntry asserts the shared outcome of every rollback exit: the snapshot was
// restored, the failed snapshot is kept, nothing was reloaded and the cache entry was not evicted.
func requireRolledBackKeepingEntry(t *testing.T, e *applyEnv, res ApplyResult) {
	t.Helper()
	require.Equal(t, "failed", res.Status)
	require.DirExists(t, res.BackupPath, "a failed apply keeps its snapshot")
	e.requireRolledBack()
	_, err := e.cache.Load(context.Background(), "collections", applyTestSlug)
	require.NoError(t, err, "a deadline or a cancel never evicts a good cache entry")
}

// TestApplyRollbackExitsReportDeadlineAsTimeout runs against the code as it is today: a deadline that has
// passed when any rollback exit is reached is reported as ErrPresetApplyTimeout and the snapshot is restored.
func TestApplyRollbackExitsReportDeadlineAsTimeout(t *testing.T) {
	t.Parallel()
	for _, exit := range applyRollbackExits() {
		t.Run(exit.name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.svc.ApplyTimeout = 300 * time.Millisecond
			e.leaveFileOnInstall()
			e.script = func(ctx context.Context, cmd string, n int) *cscliReply {
				return exit.script(e, func() { <-ctx.Done() }, cmd, n)
			}

			res, err := e.apply()
			require.ErrorIs(t, err, ErrPresetApplyTimeout)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			requireRolledBackKeepingEntry(t, e, res)
		})
	}
}

// TestApplyRollbackExitsReportCancelAsCancelled covers the same exits when the caller cancels instead of
// the deadline passing: each one restores the snapshot and reports ErrPresetApplyCancelled (which keeps
// matching context.Canceled), never a timeout and never the step's own failure.
func TestApplyRollbackExitsReportCancelAsCancelled(t *testing.T) {
	t.Parallel()
	for _, exit := range applyRollbackExits() {
		t.Run(exit.name, func(t *testing.T) {
			t.Parallel()
			t.Skip("pending background install implementation: ErrPresetApplyCancelled does not exist yet")
			e := newApplyEnv(t)
			e.leaveFileOnInstall()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e.script = func(stepCtx context.Context, cmd string, n int) *cscliReply {
				return exit.script(e, func() { cancel(); <-stepCtx.Done() }, cmd, n)
			}

			res, err := e.svc.Apply(ctx, applyTestSlug, "collections", e.meta.CacheKey)
			require.ErrorIs(t, err, errPresetApplyCancelledPending)
			require.ErrorIs(t, err, context.Canceled)
			require.NotErrorIs(t, err, ErrPresetApplyTimeout)
			requireRolledBackKeepingEntry(t, e, res)
		})
	}
}

// TestApplyRollbackExitsAreAllCovered fails when Apply gains a rollback exit that the table above does not
// reach: the number of rollBack/rollBackMismatch returns in hub_apply.go must equal the table size.
func TestApplyRollbackExitsAreAllCovered(t *testing.T) {
	t.Parallel()
	src := readFile(t, "hub_apply.go")
	apply := src[strings.Index(src, "func (s *HubService) Apply("):strings.Index(src, "func (s *HubService) rollBackMismatch(")]
	exits := strings.Count(apply, "return rollBack(") + strings.Count(apply, "return s.rollBackMismatch(")
	require.Equal(t, exits, len(applyRollbackExits()), "every rollback exit of Apply needs a row in applyRollbackExits")
}
