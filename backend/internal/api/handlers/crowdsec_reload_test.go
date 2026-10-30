package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
)

// reloadClock is a clock that moves only when a test advances it.
type reloadClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *reloadClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *reloadClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// reloadFixture drives a crowdsecReloader with a fake engine, a fake clock and a fake probe. The reloader's poll
// interval is short so queued deliveries are checked promptly; the timing constants keep their production values.
type reloadFixture struct {
	r     *crowdsecReloader
	clock *reloadClock
	ctx   context.Context

	mu       sync.Mutex
	running  bool
	pid      int
	spawned  time.Time // the engine's spawn time, measured against the fake clock
	ageKnown bool
	probeOK  bool
	probes   int
	signals  []syscall.Signal
	sigErr   error
	statErr  error
}

func newReloadFixture(t *testing.T) *reloadFixture {
	t.Helper()
	clock := &reloadClock{now: time.Unix(1_700_000_000, 0)}
	ctx, cancel := context.WithCancel(context.Background())
	f := &reloadFixture{
		clock:    clock,
		ctx:      ctx,
		running:  true,
		pid:      4321,
		spawned:  clock.Now().Add(-time.Hour), // old enough unless a test says otherwise
		ageKnown: true,
		probeOK:  true,
	}
	f.r = newCrowdsecReloader(reloadDeps{
		status: f.status,
		signal: f.signal,
		age:    f.age,
		probe:  f.probe,
		now:    clock.Now,
		appCtx: func() context.Context { return ctx },
	})
	f.r.poll = time.Millisecond
	t.Cleanup(func() {
		cancel()
		f.r.wait()
	})
	return f
}

func (f *reloadFixture) status(context.Context) (running bool, pid int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statErr != nil {
		return false, 0, f.statErr
	}
	return f.running, f.pid, nil
}

func (f *reloadFixture) signal(_ int, sig syscall.Signal) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.signals = append(f.signals, sig)
	return f.sigErr
}

func (f *reloadFixture) age(int) (time.Duration, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.ageKnown {
		return 0, false
	}
	return f.clock.Now().Sub(f.spawned), true
}

func (f *reloadFixture) probe(context.Context) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probes++
	return f.probeOK
}

// set changes the fake engine under the fixture's lock.
func (f *reloadFixture) set(change func(f *reloadFixture)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f)
}

func (f *reloadFixture) signalCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.signals)
}

func (f *reloadFixture) probeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.probes
}

// requireSignals waits until n signals were sent and then checks that no further one follows.
func (f *reloadFixture) requireSignals(t *testing.T, n int) {
	t.Helper()
	require.Eventually(t, func() bool { return f.signalCount() == n }, 2*time.Second, time.Millisecond,
		"want %d signal(s), have %d", n, f.signalCount())
	require.Never(t, func() bool { return f.signalCount() != n }, 30*time.Millisecond, time.Millisecond,
		"the signal count changed after %d", n)
}

// waitQueued waits for the queued deliveries to end, failing rather than hanging when one does not.
func (f *reloadFixture) waitQueued(t *testing.T) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		f.r.wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a queued reload did not end")
	}
}

func (f *reloadFixture) pendingNow() bool {
	f.r.mu.Lock()
	defer f.r.mu.Unlock()
	return f.r.pending
}

func TestReloaderNotRunningReturnsSentinel(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	f.set(func(f *reloadFixture) { f.running = false })

	require.ErrorIs(t, f.r.reload(f.ctx), crowdsec.ErrCrowdSecNotRunning)
	require.Zero(t, f.signalCount())
}

func TestReloaderReportsStatusFailure(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	statusErr := errors.New("proc unreadable")
	f.set(func(f *reloadFixture) { f.statErr = statusErr })

	err := f.r.reload(f.ctx)
	require.ErrorIs(t, err, statusErr)
	require.NotErrorIs(t, err, crowdsec.ErrCrowdSecNotRunning)
	require.Zero(t, f.signalCount())
}

func TestReloaderSignalsAtOnceWhenEngineIsReady(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)

	require.NoError(t, f.r.reload(f.ctx))
	require.Equal(t, []syscall.Signal{syscall.SIGHUP}, f.signals)
	require.False(t, f.pendingNow())
}

func TestReloaderReturnsSignalError(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	sigErr := errors.New("permission denied")
	f.set(func(f *reloadFixture) { f.sigErr = sigErr })

	require.ErrorIs(t, f.r.reload(f.ctx), sigErr)
}

// An engine whose age the executor cannot tell counts as old enough.
func TestReloaderUnknownAgeCountsAsOldEnough(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	f.set(func(f *reloadFixture) {
		f.ageKnown = false
		f.spawned = f.clock.Now() // would be too young if the age were known
	})

	require.NoError(t, f.r.reload(f.ctx))
	require.Len(t, f.signals, 1)
}

func TestReloaderDefersUntilSettleWindowPasses(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	f.set(func(f *reloadFixture) { f.spawned = f.clock.Now() })

	require.ErrorIs(t, f.r.reload(f.ctx), crowdsec.ErrReloadDeferred)
	require.Zero(t, f.signalCount(), "a request inside the settle window is not signalled")

	f.clock.Advance(engineSettleWindow - time.Second)
	require.Zero(t, f.signalCount(), "still inside the settle window")

	f.clock.Advance(time.Second)
	f.requireSignals(t, 1)
}

func TestReloaderCoalescesPendingRequests(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	f.set(func(f *reloadFixture) { f.spawned = f.clock.Now() })

	for range 3 {
		require.ErrorIs(t, f.r.reload(f.ctx), crowdsec.ErrReloadDeferred)
	}
	f.clock.Advance(engineSettleWindow)
	f.requireSignals(t, 1)
}

func TestReloaderHonoursMinimumIntervalBetweenSignals(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)

	require.NoError(t, f.r.reload(f.ctx))
	f.clock.Advance(time.Second)

	require.ErrorIs(t, f.r.reload(f.ctx), crowdsec.ErrReloadDeferred)
	require.Len(t, f.signals, 1, "a second signal inside the minimum interval is queued")

	f.clock.Advance(reloadMinInterval - time.Second)
	f.requireSignals(t, 2)
}

// A failed signal still counts towards the minimum interval.
func TestReloaderFailedSignalStillSpacesSignals(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	f.set(func(f *reloadFixture) { f.sigErr = errors.New("gone") })

	require.Error(t, f.r.reload(f.ctx))
	require.ErrorIs(t, f.r.reload(f.ctx), crowdsec.ErrReloadDeferred)
	require.Len(t, f.signals, 1)
}

func TestReloaderDropsQueuedReloadWhenEngineExits(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	f.set(func(f *reloadFixture) { f.spawned = f.clock.Now() })
	require.ErrorIs(t, f.r.reload(f.ctx), crowdsec.ErrReloadDeferred)

	f.set(func(f *reloadFixture) { f.running = false })
	f.waitQueued(t)

	f.clock.Advance(engineSettleWindow)
	require.Zero(t, f.signalCount(), "the reload is dropped, not delivered to a new engine")
	require.False(t, f.pendingNow())
}

func TestReloaderDropsQueuedReloadAfterBound(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	f.set(func(f *reloadFixture) { f.probeOK = false })

	require.ErrorIs(t, f.r.reload(f.ctx), crowdsec.ErrReloadDeferred)
	f.clock.Advance(reloadPendingBound + time.Second)
	f.waitQueued(t)

	require.Zero(t, f.signalCount())
	require.False(t, f.pendingNow())
}

func TestReloaderShutdownCancelsQueuedReload(t *testing.T) {
	t.Parallel()
	clock := &reloadClock{now: time.Unix(1_700_000_000, 0)}
	ctx, cancel := context.WithCancel(context.Background())
	f := &reloadFixture{clock: clock, ctx: ctx, running: true, pid: 9, spawned: clock.Now(), ageKnown: true}
	f.r = newCrowdsecReloader(reloadDeps{
		status: f.status, signal: f.signal, age: f.age, probe: f.probe, now: clock.Now,
		appCtx: func() context.Context { return ctx },
	})
	f.r.poll = time.Millisecond
	t.Cleanup(f.r.wait)

	require.ErrorIs(t, f.r.reload(ctx), crowdsec.ErrReloadDeferred)
	cancel()
	f.waitQueued(t)

	require.Zero(t, f.signalCount())
	require.False(t, f.pendingNow())
}

// A queued request is not a second queue: while one is pending, further requests are answered without a probe.
func TestReloaderProbeIsCachedPerEngineWithinSettleWindow(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	ctx := f.ctx

	f.r.mu.Lock()
	now := f.clock.Now()
	f.r.probeLocked(ctx, 7, now)
	f.r.probeLocked(ctx, 7, now.Add(engineSettleWindow-time.Millisecond))
	require.Equal(t, 1, f.probeCount(), "one probe per settle window")

	f.r.probeLocked(ctx, 7, now.Add(engineSettleWindow))
	require.Equal(t, 2, f.probeCount(), "a new window probes again")

	f.r.probeLocked(ctx, 8, now.Add(engineSettleWindow))
	require.Equal(t, 3, f.probeCount(), "a different engine is probed afresh")
	f.r.mu.Unlock()
}

// A negative probe is cached for only reloadNegativeProbeTTL, so a queued reload is delivered soon after LAPI
// answers. A positive probe stays cached for the whole settle window.
func TestReloaderNegativeProbeIsCachedOnlyBriefly(t *testing.T) {
	t.Parallel()
	f := newReloadFixture(t)
	ctx := f.ctx
	f.set(func(f *reloadFixture) { f.probeOK = false })

	f.r.mu.Lock()
	defer f.r.mu.Unlock()
	now := f.clock.Now()
	require.False(t, f.r.probeLocked(ctx, 7, now))
	require.False(t, f.r.probeLocked(ctx, 7, now.Add(reloadNegativeProbeTTL-time.Millisecond)))
	require.Equal(t, 1, f.probeCount(), "a negative answer is reused inside its short TTL")

	require.False(t, f.r.probeLocked(ctx, 7, now.Add(reloadNegativeProbeTTL)))
	require.Equal(t, 2, f.probeCount(), "a negative answer is probed again once its TTL passes")

	f.set(func(f *reloadFixture) { f.probeOK = true })
	require.True(t, f.r.probeLocked(ctx, 8, now))
	require.True(t, f.r.probeLocked(ctx, 8, now.Add(engineSettleWindow-time.Millisecond)))
	require.Equal(t, 3, f.probeCount(), "a positive answer stays cached for the whole settle window")
}

// The sentinel is what the handler and the hub services compare against, so it must stay distinct.
func TestReloadDeferredIsDistinctFromNotRunning(t *testing.T) {
	t.Parallel()
	require.NotErrorIs(t, crowdsec.ErrReloadDeferred, crowdsec.ErrCrowdSecNotRunning)
	require.NotErrorIs(t, crowdsec.ErrCrowdSecNotRunning, crowdsec.ErrReloadDeferred)
}

// Every whitelist and hub sender goes through the one reloader, so each one produces a signal from it.
func TestReloadCrowdSecSendersAllReachTheReloader(t *testing.T) {
	t.Parallel()
	h, r, _ := setupWhitelistHandler(t)
	mock := &reloadRecorder{}
	mock.attach(h, true)
	h.reloader().minInterval = 0 // the four calls are back to back; the spacing has its own test

	require.NoError(t, h.Hub.Reload(t.Context()), "hub apply and restore reload through the handler")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(`{"ip_or_cidr":"5.5.5.5"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	var entry models.CrowdSecWhitelist
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/"+entry.UUID, http.NoBody)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	restored, _ := h.reloadAfterRestore(t.Context())
	require.True(t, restored)

	assert.Equal(t, []syscall.Signal{syscall.SIGHUP, syscall.SIGHUP, syscall.SIGHUP, syscall.SIGHUP}, mock.sigs,
		"hub reload, whitelist add, whitelist delete and restore reload each signal once")
}

// A whitelist change while the engine is starting is answered as before and queued, with no signal yet.
func TestWhitelistChangeWhileEngineStartsIsQueued(t *testing.T) {
	t.Parallel()
	h, r, _ := setupWhitelistHandler(t)
	mock := &reloadRecorder{}
	mock.attach(h, true)
	h.reloadProbe = func(context.Context) bool { return false }
	h.WithAppContext(t.Context())
	t.Cleanup(h.reloader().wait)

	require.ErrorIs(t, h.reloadCrowdSec(t.Context()), crowdsec.ErrReloadDeferred)
	require.Empty(t, mock.sigs)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(`{"ip_or_cidr":"8.8.8.8"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "a deferred reload does not fail the change")
	assert.Empty(t, mock.sigs)
}

func TestRestoreHubItemsReportsReloadPendingWhileEngineStarts(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	hh.h.reloadProbe = func(context.Context) bool { return false }
	hh.h.WithAppContext(t.Context())
	t.Cleanup(hh.h.reloader().wait)

	w := hh.post(restoreBody(hubItems("collections", "Azlaroc/sftpgo")...))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	res := decodeRestore(t, w)
	require.Len(t, res.Installed, 1)
	require.False(t, res.Reloaded)
	require.True(t, res.ReloadPending)
	require.Zero(t, hh.signal.Load(), "nothing is signalled before the engine is ready")

	var raw map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.Equal(t, false, raw["reloaded"])
	assert.Equal(t, true, raw["reload_pending"])
}
