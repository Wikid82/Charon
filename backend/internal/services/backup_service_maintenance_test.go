package services

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// fakeDeferrer is a hand-driven maintenance gate.
type fakeDeferrer struct {
	mu        sync.Mutex
	deferring bool
	released  chan struct{}
}

func newFakeDeferrer() *fakeDeferrer {
	return &fakeDeferrer{deferring: true, released: make(chan struct{})}
}

func (f *fakeDeferrer) Deferring() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deferring
}

func (f *fakeDeferrer) release() {
	f.mu.Lock()
	f.deferring = false
	f.mu.Unlock()
	close(f.released)
}

func (f *fakeDeferrer) WaitReleased(ctx context.Context) bool {
	select {
	case <-f.released:
		return true
	case <-ctx.Done():
		return false
	}
}

func newDeferralTestService(t *testing.T) (*BackupService, *atomic.Int32) {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	svc := NewBackupService(&config.Config{DatabasePath: dbPath}, nil, nil)
	t.Cleanup(svc.Stop)

	var runs atomic.Int32
	svc.createBackupOpts = func(BackupOptions) (*models.BackupRecord, error) {
		runs.Add(1)
		return &models.BackupRecord{Filename: "backup_2026-01-01_00-00-00.zip"}, nil
	}
	svc.cleanupOld = func(int) (int, error) { return 0, nil }
	return svc, &runs
}

func TestRunScheduledBackup_DeferredWhileMaintenanceThenRunsOnceAfterRelease(t *testing.T) {
	svc, runs := newDeferralTestService(t)
	gate := newFakeDeferrer()
	svc.SetMaintenanceDeferrer(context.Background(), gate)

	// Two cron ticks inside the window start only one waiter.
	svc.RunScheduledBackup()
	svc.RunScheduledBackup()
	assert.Zero(t, runs.Load(), "deferred, not run, while the gate defers")

	gate.release()
	require.Eventually(t, func() bool { return runs.Load() == 1 }, 2*time.Second, 5*time.Millisecond,
		"the deferred backup runs once after release")
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), runs.Load(), "a single run-after-release waiter")
}

func TestRunScheduledBackup_DeferralWaiterExitsOnAppContextCancel(t *testing.T) {
	svc, runs := newDeferralTestService(t)
	gate := newFakeDeferrer()
	appCtx, cancel := context.WithCancel(context.Background())
	svc.SetMaintenanceDeferrer(appCtx, gate)

	svc.RunScheduledBackup()
	require.Eventually(t, func() bool { return svc.deferredPending.Load() }, time.Second, time.Millisecond)

	cancel()
	require.Eventually(t, func() bool { return !svc.deferredPending.Load() }, 2*time.Second, 5*time.Millisecond,
		"the waiter returns on shutdown instead of leaking")
	assert.Zero(t, runs.Load(), "and does not run the backup")
}

func TestRunScheduledBackup_RunsNormallyWhenTheGateIsNotDeferring(t *testing.T) {
	svc, runs := newDeferralTestService(t)
	gate := newFakeDeferrer()
	gate.release()
	svc.SetMaintenanceDeferrer(context.Background(), gate)

	svc.RunScheduledBackup()
	assert.Equal(t, int32(1), runs.Load())
}

func TestRunScheduledBackup_NilDeferrerRunsNormally(t *testing.T) {
	svc, runs := newDeferralTestService(t)
	svc.RunScheduledBackup()
	assert.Equal(t, int32(1), runs.Load())
}
