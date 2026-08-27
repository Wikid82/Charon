package services

import (
	"context"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- constructor: checker fallback when the service has none ---

func TestUptimeWorkerPool_New_BuildsOwnCheckerWhenServiceHasNone(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))
	svc.checker = nil // force the defensive fallback branch

	p := NewUptimeWorkerPool(svc)
	require.NotNil(t, p.checker, "pool must build its own checker when the service lacks one")
	assert.Positive(t, p.WorkerPoolSize())
}

// --- SeedState / ReseedState error branches ---

func TestUptimeWorkerPool_SeedState_MonStateQueryError(t *testing.T) {
	t.Parallel()
	p, _ := newTestPool(t, closedUptimeDB(t))
	err := p.SeedState(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seed monState")
}

func TestUptimeWorkerPool_SeedState_HostStateQueryError(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)
	// monState load succeeds, hostState load fails.
	require.NoError(t, db.Migrator().DropTable(&models.UptimeHost{}))

	err := p.SeedState(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seed hostState")
}

func TestUptimeWorkerPool_ReseedState_WrapsError(t *testing.T) {
	t.Parallel()
	p, _ := newTestPool(t, closedUptimeDB(t))
	err := p.ReseedState(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reseed state")
}

// --- SeedState: blank status rows default to "pending" ---

func TestUptimeWorkerPool_SeedState_BlankStatusDefaultsToPending(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)

	require.NoError(t, db.Create(&models.UptimeMonitor{ID: "m-blank", Name: "b", Type: "http", URL: "http://x", Enabled: true, Status: ""}).Error)
	require.NoError(t, db.Create(&models.UptimeHost{ID: "h-blank", Host: "10.9.9.9", Status: ""}).Error)

	require.NoError(t, p.SeedState(context.Background()))

	p.monMu.Lock()
	assert.Equal(t, "pending", p.monState["m-blank"].status)
	p.monMu.Unlock()
	st, ok := p.HostState("h-blank")
	require.True(t, ok)
	assert.Equal(t, "pending", st.status)
}

// --- Run: initial SeedState failure is logged, not fatal ---

func TestUptimeWorkerPool_Run_InitialSeedStateFailureIsNonFatal(t *testing.T) {
	t.Parallel()
	p, _ := newTestPool(t, closedUptimeDB(t))
	p.size = 2

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()
	time.Sleep(40 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// --- handle(): host-check dispatch arm ---

func TestUptimeWorkerPool_Handle_RoutesHostCheck(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)

	host := models.UptimeHost{ID: "h-route", Host: "127.0.0.1", Name: "h", Status: "pending"}
	require.NoError(t, db.Create(&host).Error)

	assert.NotPanics(t, func() {
		p.handle(UptimeJob{Kind: JobHostCheck, Host: &host})
	})
}

// --- handleMonitorCheck: guard + default branches ---

func TestUptimeWorkerPool_HandleMonitorCheck_DisabledNonManualIsDropped(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)

	m := models.UptimeMonitor{ID: "m-off", Name: "off", Type: "tcp", URL: "127.0.0.1:9", Enabled: false, MaxRetries: 2}
	p.handleMonitorCheck(UptimeJob{Kind: JobMonitorCheck, Monitor: m}) // not manual

	p.monMu.Lock()
	_, seen := p.monState[m.ID]
	p.monMu.Unlock()
	assert.False(t, seen, "a disabled non-manual monitor check emits nothing and touches no state")
}

func TestUptimeWorkerPool_HandleMonitorCheck_LegacyDefaultsAndPendingSeed(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)

	// MaxRetries 0 -> legacy default of 3; no pre-seeded monState -> prev.status "" -> "pending".
	m := models.UptimeMonitor{ID: "m-legacy", Name: "legacy", Type: "tcp", URL: "127.0.0.1:9", Enabled: true, MaxRetries: 0}
	require.NoError(t, db.Create(&m).Error)

	p.handleMonitorCheck(UptimeJob{Kind: JobMonitorCheck, Monitor: m})

	p.monMu.Lock()
	st := p.monState[m.ID]
	p.monMu.Unlock()
	assert.Equal(t, 1, st.failureCount, "one failure recorded")
	assert.NotEqual(t, "down", st.status, "a single failure with the legacy 3-retry default does not flip to down")
}

// --- handleHostCheck: guard + default branches ---

func TestUptimeWorkerPool_HandleHostCheck_NilHostIsDropped(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)
	assert.NotPanics(t, func() { p.handleHostCheck(UptimeJob{Kind: JobHostCheck, Host: nil}) })
}

func TestUptimeWorkerPool_HandleHostCheck_ThresholdDefaultAndPendingSeed(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)
	p.svc.config.FailureThreshold = 0 // -> default 2 inside handleHostCheck

	// Host with blank status and no pre-seeded hostState -> prev.status "" -> host.Status "" -> "pending".
	host := models.UptimeHost{ID: "h-def", Host: "127.0.0.1", Name: "h", Status: ""}
	require.NoError(t, db.Create(&host).Error)
	require.NoError(t, db.Create(&models.UptimeMonitor{ID: "hc", UptimeHostID: &host.ID, Name: "hc", Type: "tcp", URL: "127.0.0.1:9", Enabled: true}).Error)

	p.handleHostCheck(UptimeJob{Kind: JobHostCheck, Host: &host})

	st, ok := p.HostState(host.ID)
	require.True(t, ok)
	assert.Equal(t, 1, st.failureCount, "first failed connectivity check increments but does not flip (threshold defaulted to 2)")
	assert.NotEqual(t, "down", st.status)
}

// --- fanOutHostDown: child load error + child legacy/pending defaults ---

func TestUptimeWorkerPool_FanOutHostDown_ChildLoadErrorIsLogged(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)

	host := models.UptimeHost{ID: "h-fanerr", Host: "127.0.0.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	// Drop the monitors table: probeHost fails (host check fails) AND the fan-out
	// child query fails once the host flips to down.
	require.NoError(t, db.Migrator().DropTable(&models.UptimeMonitor{}))

	p.hostMu.Lock()
	p.hostState[host.ID] = hostDebounce{status: "up", failureCount: 1} // next failure -> down
	p.hostMu.Unlock()

	assert.NotPanics(t, func() { p.handleHostCheck(UptimeJob{Kind: JobHostCheck, Host: &host}) })
	st, _ := p.HostState(host.ID)
	assert.Equal(t, "down", st.status)
}

func TestUptimeWorkerPool_FanOutHostDown_ChildLegacyDefaultsAndPendingSeed(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)

	host := models.UptimeHost{ID: "h-fandef", Host: "127.0.0.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	// Child with MaxRetries 0 (-> legacy 3) and NO pre-seeded monState (-> "pending").
	require.NoError(t, db.Create(&models.UptimeMonitor{
		ID: "child-legacy", UptimeHostID: &host.ID, Name: "cl", Type: "tcp", URL: "127.0.0.1:9",
		Enabled: true, MaxRetries: 0, Status: "up",
	}).Error)

	p.hostMu.Lock()
	p.hostState[host.ID] = hostDebounce{status: "up", failureCount: 1}
	p.hostMu.Unlock()

	p.handleHostCheck(UptimeJob{Kind: JobHostCheck, Host: &host}) // 2nd failure -> host down -> fan-out

	p.monMu.Lock()
	child := p.monState["child-legacy"]
	p.monMu.Unlock()
	assert.Equal(t, "down", child.status, "fan-out forces the child down")
	assert.Equal(t, 3, child.failureCount, "child failureCount maxed to the legacy default of 3 (MaxRetries 0)")
	// prev.status seeded from "" -> "pending", so this is not a transition and no
	// notification fires (the transition+notify path is covered by B2_HostDownFanout).
}
