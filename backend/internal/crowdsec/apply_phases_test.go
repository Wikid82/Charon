package crowdsec

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// phaseLog records the phases an apply reports.
type phaseLog struct {
	mu     sync.Mutex
	phases []Phase
}

func (l *phaseLog) record(p Phase) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.phases = append(l.phases, p)
}

func (l *phaseLog) got() []Phase {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Phase(nil), l.phases...)
}

func (e *applyEnv) request(onPhase func(Phase)) ApplyRequest {
	return ApplyRequest{Slug: applyTestSlug, Type: "collections", CacheKey: e.meta.CacheKey, OnPhase: onPhase}
}

func TestApplyWithReportsPhasesInOrder(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	var log phaseLog

	res, err := e.svc.ApplyWith(context.Background(), e.request(log.record))
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.Equal(t, []Phase{PhasePreflight, PhaseSnapshot, PhaseInstall, PhaseVerify, PhaseReload}, log.got())
}

func TestApplyWithReportsRollbackPhase(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInstall {
			return &cscliReply{err: errors.New("exit status 1")}
		}
		return nil
	}
	var log phaseLog

	_, err := e.svc.ApplyWith(context.Background(), e.request(log.record))
	require.Error(t, err)
	require.Equal(t, []Phase{PhasePreflight, PhaseSnapshot, PhaseInstall, PhaseRollback}, log.got())
	e.requireRolledBack()
}

func TestApplyWithReportsRollbackAfterVerifyFails(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.script = func(_ context.Context, cmd string, n int) *cscliReply {
		if cmd == applyInspect && n == 1 {
			return inspectReply(true, false, true, "deadbeef")
		}
		return nil
	}
	var log phaseLog

	_, err := e.svc.ApplyWith(context.Background(), e.request(log.record))
	require.ErrorIs(t, err, ErrPresetHubUpdating)
	require.Equal(t, []Phase{PhasePreflight, PhaseSnapshot, PhaseInstall, PhaseVerify, PhaseRollback}, log.got())
}

func TestApplyWithAcceptsNilOnPhase(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)

	res, err := e.svc.ApplyWith(context.Background(), e.request(nil))
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
}

func TestApplyWithSurvivesPanickingOnPhase(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	calls := 0

	res, err := e.svc.ApplyWith(context.Background(), e.request(func(Phase) {
		calls++
		panic("callback bug")
	}))
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.Equal(t, 5, calls, "a panic in one phase does not stop the later ones")
	require.Equal(t, 1, e.reloads)
}

func TestApplyWithTimeoutOverridesApplyTimeout(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.svc.ApplyTimeout = 50 * time.Millisecond
	e.script = func(ctx context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInstall {
			select {
			case <-ctx.Done():
				return &cscliReply{err: ctx.Err()}
			case <-time.After(300 * time.Millisecond):
			}
		}
		return nil
	}
	req := e.request(nil)
	req.Timeout = 10 * time.Second

	res, err := e.svc.ApplyWith(context.Background(), req)
	require.NoError(t, err, "the longer per-request limit beats the 50 ms default")
	require.Equal(t, "applied", res.Status)
}

func TestApplyWithShorterTimeoutStillApplies(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.script = func(ctx context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInstall {
			<-ctx.Done()
			return &cscliReply{err: ctx.Err()}
		}
		return nil
	}
	req := e.request(nil)
	req.Timeout = 100 * time.Millisecond

	_, err := e.svc.ApplyWith(context.Background(), req)
	require.ErrorIs(t, err, ErrPresetApplyTimeout)
	e.requireRolledBack()
}

func TestApplyWrapperKeepsDefaultTimeout(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.svc.ApplyTimeout = 100 * time.Millisecond
	e.script = func(ctx context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInstall {
			<-ctx.Done()
			return &cscliReply{err: ctx.Err()}
		}
		return nil
	}

	_, err := e.apply()
	require.ErrorIs(t, err, ErrPresetApplyTimeout)
	e.requireRolledBack()
}

func TestErrPresetApplyCancelledWrapsCanceled(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, ErrPresetApplyCancelled, context.Canceled)
	require.NotErrorIs(t, ErrPresetApplyCancelled, ErrPresetApplyTimeout)
}
