package dbmaint

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Phase is the maintenance gate state. Only checking and converting hold the
// database's single pool connection and answer requests with 503; planned only
// defers the background pipeline and scheduled backups.
type Phase string

// Gate phases (3.4 of the plan).
const (
	PhaseIdle       Phase = "idle"
	PhasePlanned    Phase = "planned"
	PhaseChecking   Phase = "checking"
	PhaseConverting Phase = "converting"
	PhaseDone       Phase = "done"
	PhaseSkipped    Phase = "skipped"
	PhaseFailed     Phase = "failed"
)

// deferring reports whether background work must wait (planned, checking,
// converting).
func (p Phase) deferring() bool {
	return p == PhasePlanned || p == PhaseChecking || p == PhaseConverting
}

// active reports whether the pool is held or being acquired (503 phases).
func (p Phase) active() bool { return p == PhaseChecking || p == PhaseConverting }

// Errors returned by Gate.AwaitReady and the Begin methods.
var (
	// ErrNotPlanned means the gate is not in a phase the call applies to.
	ErrNotPlanned = errors.New("maintenance gate is not planned")
	// ErrConfigNotApplied means the initial Caddy configuration was not applied.
	ErrConfigNotApplied = errors.New("initial caddy configuration was not applied")
	// ErrStartupTimeout means the readiness signals did not both arrive in time.
	ErrStartupTimeout = errors.New("timed out waiting for startup readiness")
)

// Snapshot is a point-in-time copy of the gate state. The status endpoint and
// the maintenance page read it; no database access is involved.
type Snapshot struct {
	Phase          Phase
	StartedAt      time.Time
	ElapsedSeconds int64
	BytesBefore    int64
	BytesAfter     int64
	Reason         Reason
	Message        string
}

// FinishInfo describes a terminal gate outcome.
type FinishInfo struct {
	Phase      Phase
	Reason     Reason
	Message    string
	BytesAfter int64
}

// Gate is the in-memory maintenance state machine. It is safe for concurrent
// use, and a nil *Gate behaves as a gate that is permanently idle.
type Gate struct {
	mu  sync.Mutex
	now func() time.Time

	phase       Phase
	startedAt   time.Time
	finishedAt  time.Time
	bytesBefore int64
	bytesAfter  int64
	reason      Reason
	message     string

	configDone    bool
	configApplied bool
	listenerBound bool

	// changed is closed and replaced on every state change (a broadcast).
	changed chan struct{}

	// runnerDone is closed once the runner goroutine has returned; it is
	// closed from the start because an unplanned gate has no runner.
	runnerDone chan struct{}
	runnerOpen bool
}

// NewGate returns an idle gate.
func NewGate() *Gate {
	done := make(chan struct{})
	close(done)
	return &Gate{
		now:        time.Now,
		phase:      PhaseIdle,
		changed:    make(chan struct{}),
		runnerDone: done,
	}
}

// notifyLocked wakes every waiter. The caller holds g.mu.
func (g *Gate) notifyLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}

// MarkPlanned moves an idle gate to planned and expects a runner to follow.
func (g *Gate) MarkPlanned() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.phase != PhaseIdle {
		return
	}
	g.phase = PhasePlanned
	g.runnerDone = make(chan struct{})
	g.runnerOpen = true
	g.notifyLocked()
}

// ConfigDone records the outcome of the initial Caddy configuration. It is one
// of the two readiness signals.
func (g *Gate) ConfigDone(applied bool) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.configDone, g.configApplied = true, applied
	g.notifyLocked()
}

// MarkListenerBound records that the main HTTP listener is bound. It is the
// second readiness signal.
func (g *Gate) MarkListenerBound() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.listenerBound = true
	g.notifyLocked()
}

// AwaitReady blocks, in the planned phase, until the initial Caddy
// configuration was applied AND the listener is bound. It returns
// ErrConfigNotApplied when Caddy never got its configuration,
// ErrStartupTimeout after timeout with a signal missing, ErrNotPlanned when the
// gate left planned, or the context error.
func (g *Gate) AwaitReady(ctx context.Context, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		g.mu.Lock()
		phase, cfgDone, applied, bound, ch := g.phase, g.configDone, g.configApplied, g.listenerBound, g.changed
		g.mu.Unlock()

		switch {
		case phase != PhasePlanned:
			return ErrNotPlanned
		case cfgDone && !applied:
			return ErrConfigNotApplied
		case applied && bound:
			return nil
		}

		select {
		case <-ch:
		case <-timer.C:
			return ErrStartupTimeout
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// BeginChecking flips planned to checking: from here the management plane
// answers 503 while the runner acquires the pool connection.
func (g *Gate) BeginChecking() error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.phase != PhasePlanned {
		return ErrNotPlanned
	}
	g.phase = PhaseChecking
	g.startedAt = g.now()
	g.notifyLocked()
	return nil
}

// BeginConverting flips checking to converting and records the file size.
func (g *Gate) BeginConverting(bytesBefore int64) error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.phase != PhaseChecking {
		return ErrNotPlanned
	}
	g.phase = PhaseConverting
	g.bytesBefore = bytesBefore
	g.notifyLocked()
	return nil
}

// Finish ends a deferring gate with a terminal outcome and releases every
// waiter. Only the first call takes effect.
func (g *Gate) Finish(info FinishInfo) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.phase.deferring() {
		return
	}
	g.phase = info.Phase
	g.reason, g.message, g.bytesAfter = info.Reason, info.Message, info.BytesAfter
	g.finishedAt = g.now()
	g.notifyLocked()
}

// Release makes sure a deferring gate is released. It is idempotent and is
// deferred by the runner so no exit path can leave the gate stuck.
func (g *Gate) Release() {
	g.Finish(FinishInfo{Phase: PhaseSkipped})
}

// RunnerExited records that the runner goroutine returned.
func (g *Gate) RunnerExited() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.runnerOpen {
		close(g.runnerDone)
		g.runnerOpen = false
	}
}

// WaitRunner waits up to d for the runner to return and reports whether it did.
// It returns true at once when no runner was started.
func (g *Gate) WaitRunner(d time.Duration) bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	done := g.runnerDone
	g.mu.Unlock()

	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

// Snapshot returns the current state.
func (g *Gate) Snapshot() Snapshot {
	if g == nil {
		return Snapshot{Phase: PhaseIdle}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	snap := Snapshot{
		Phase:       g.phase,
		StartedAt:   g.startedAt,
		BytesBefore: g.bytesBefore,
		BytesAfter:  g.bytesAfter,
		Reason:      g.reason,
		Message:     g.message,
	}
	if !g.startedAt.IsZero() {
		end := g.finishedAt
		if end.IsZero() {
			end = g.now()
		}
		snap.ElapsedSeconds = int64(end.Sub(g.startedAt) / time.Second)
	}
	return snap
}

// Deferring reports whether background work (pipeline, scheduled backups) must
// wait: phases planned, checking and converting.
func (g *Gate) Deferring() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.phase.deferring()
}

// Active reports whether the management plane answers 503 (checking or
// converting).
func (g *Gate) Active() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.phase.active()
}

// WaitIdle blocks until the gate no longer defers background work and returns
// true, or returns false when ctx ends first. It returns at once on an idle
// gate, the overwhelmingly common case.
func (g *Gate) WaitIdle(ctx context.Context) bool {
	if g == nil {
		return true
	}
	for {
		g.mu.Lock()
		deferring, ch := g.phase.deferring(), g.changed
		g.mu.Unlock()
		if !deferring {
			return true
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return false
		}
	}
}

// WaitReleased is WaitIdle under the name the scheduled-backup deferral uses:
// it returns once the gate left the active and planned phases, including every
// terminal outcome.
func (g *Gate) WaitReleased(ctx context.Context) bool { return g.WaitIdle(ctx) }
