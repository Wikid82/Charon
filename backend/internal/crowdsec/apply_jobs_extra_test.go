package crowdsec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

func newHarness(t *testing.T, cfg pendingJobConfig) *jobHarness {
	t.Helper()
	h, ok := newPendingJobManager(t, cfg).(*jobHarness)
	require.True(t, ok)
	return h
}

func TestApplyErrorCode(t *testing.T) {
	t.Parallel()
	cases := map[string]error{
		models.CrowdsecApplyErrCacheStale:       fmt.Errorf("x: %w", ErrPresetCacheStale),
		models.CrowdsecApplyErrHubUpdating:      ErrPresetHubUpdating,
		models.CrowdsecApplyErrLocalModified:    &LocalModifiedError{Items: []string{"a"}},
		models.CrowdsecApplyErrTimeout:          ErrPresetApplyTimeout,
		models.CrowdsecApplyErrHubUnreachable:   ErrHubUnreachable,
		models.CrowdsecApplyErrCSCLIUnavailable: ErrCSCLIUnavailable,
		"":                                      errors.New("other"),
	}
	for want, err := range cases {
		require.Equal(t, want, ApplyErrorCode(err), want)
	}
}

func TestApplyJobManagerNeedsConfiguration(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager(nil)
	ctx := context.Background()
	_, err := m.Start(ApplyJobRequest{})
	require.ErrorIs(t, err, ErrApplyJobNotConfigured)
	require.ErrorIs(t, m.Cancel("x"), ErrApplyJobNotConfigured)
	require.ErrorIs(t, m.Prune(ctx), ErrApplyJobNotConfigured)
	require.ErrorIs(t, m.RecoverInterrupted(ctx, nil), ErrApplyJobNotConfigured)
	_, err = m.Get(ctx, "x")
	require.ErrorIs(t, err, ErrApplyJobNotConfigured)
	_, err = m.Current(ctx)
	require.ErrorIs(t, err, ErrApplyJobNotConfigured)
	_, err = m.Recent(ctx, 5)
	require.ErrorIs(t, err, ErrApplyJobNotConfigured)
	_, busy := m.Busy()
	require.False(t, busy)
}

func TestApplyJobStartRefusedWhileRestoreRuns(t *testing.T) {
	t.Parallel()
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir()})
	h.mgr.blockedBy = func() bool { return true }
	_, err := h.mgr.Start(ApplyJobRequest{Slug: applyTestSlug})
	require.ErrorIs(t, err, ErrApplyJobBusy)
	require.Empty(t, h.List(), "a refused start leaves no row")
}

func TestApplyJobCancelUnknownAndFinished(t *testing.T) {
	t.Parallel()
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir()})
	require.ErrorIs(t, h.Cancel("missing"), ErrApplyJobNotFound)
	_, ok := h.Current()
	require.False(t, ok)
}

func TestApplyJobStoresCancelRequestAndSnapshotEarly(t *testing.T) {
	t.Parallel()
	started, release := make(chan struct{}), make(chan struct{})
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir()})
	h.mgr.Configure(ApplyJobConfig{Repo: h.repo, Apply: func(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
		req.OnSnapshot("/snap/early")
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
		}
		return ApplyResult{Status: "failed", BackupPath: "/snap/early"}, ctx.Err()
	}})

	id, err := h.Start(context.Background(), applyTestSlug, "collections", "key")
	require.NoError(t, err)
	<-started
	current, ok := h.Current()
	require.True(t, ok)
	require.Equal(t, id, current.ID)
	require.Equal(t, "/snap/early", current.BackupPath, "the snapshot path is stored before the install ends")
	require.NoError(t, h.Cancel(id))
	done, err := h.Wait(id, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobCancelled, done.State)
	stored, err := h.mgr.Get(context.Background(), id)
	require.NoError(t, err)
	require.True(t, stored.CancelRequested)
	require.NotEmpty(t, stored.Error)
}

func TestApplyJobReportsInstalledItemsGrowthAndFinalCount(t *testing.T) {
	t.Parallel()
	var count atomic.Int64
	count.Store(7)
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir(), ProgressInterval: 5 * time.Millisecond, CountItems: func() int { return int(count.Load()) }})
	h.mgr.cfg.Apply = func(_ context.Context, req ApplyRequest) (ApplyResult, error) {
		req.OnPhase(PhaseInstall)
		count.Store(10)
		time.Sleep(60 * time.Millisecond)
		req.OnPhase(PhaseVerify)
		return ApplyResult{Status: "applied"}, nil
	}
	id, err := h.Start(context.Background(), applyTestSlug, "collections", "key")
	require.NoError(t, err)
	done, err := h.Wait(id, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobSucceeded, done.State)
	require.Equal(t, 3, done.ItemsInstalled, "growth since the install phase began")
}

func TestApplyJobRecoveryRecordsAFailedOrMissingRestore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	snap := filepath.Join(dir, "snap")
	require.NoError(t, os.MkdirAll(snap, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(snap, "config.yaml"), []byte("a: b\n"), 0o600))
	h := newHarness(t, pendingJobConfig{DataDir: dir})
	require.NoError(t, h.Seed(pendingJob{ID: "bad", State: jobRunning, StartedAt: time.Now(), BackupPath: snap}))
	require.NoError(t, h.Seed(pendingJob{ID: "gone", State: jobRunning, StartedAt: time.Now(), BackupPath: filepath.Join(dir, "nope")}))
	var restored []string

	err := h.mgr.RecoverInterrupted(context.Background(), func(p string) error {
		restored = append(restored, p)
		return errors.New("disk on fire")
	})

	require.NoError(t, err)
	require.Equal(t, []string{snap}, restored, "a missing snapshot never reaches restore")
	bad, err := h.Get("bad")
	require.NoError(t, err)
	require.Contains(t, bad.Warnings[0], "could not be restored")
	gone, err := h.Get("gone")
	require.NoError(t, err)
	require.Contains(t, gone.Warnings[0], "no longer exists")
}

func TestApplyJobRunsHubServiceWithItsOwnTimeLimit(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	db := testutil.OpenMemoryDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecApplyJob{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	m := NewApplyJobManager(nil)
	t.Cleanup(m.Shutdown)
	m.Configure(HubApplyConfig(e.svc, NewApplyJobRepository(db)))

	job, err := m.Start(ApplyJobRequest{Slug: applyTestSlug, Type: "collections", CacheKey: e.meta.CacheKey})
	require.NoError(t, err)
	require.Equal(t, models.CrowdsecApplyJobQueued, job.State)
	require.Eventually(t, func() bool {
		got, getErr := m.Get(context.Background(), job.ID)
		_, busy := m.Busy()
		return getErr == nil && !busy && got.State == models.CrowdsecApplyJobSucceeded
	}, 10*time.Second, 10*time.Millisecond)
	got, err := m.Get(context.Background(), job.ID)
	require.NoError(t, err)
	require.NotEmpty(t, got.BackupPath)
	require.Equal(t, string(PhaseReload), got.Phase)
}

func TestApplyWithReportsSnapshotPath(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	var path string
	req := e.request(nil)
	req.OnSnapshot = func(p string) { path = p }
	res, err := e.svc.ApplyWith(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, path)
	require.Equal(t, res.BackupPath, path)

	req.OnSnapshot = func(string) { panic("callback") }
	e2 := newApplyEnv(t)
	_, err = e2.svc.ApplyWith(context.Background(), ApplyRequest{Slug: applyTestSlug, Type: "collections", CacheKey: e2.meta.CacheKey, OnSnapshot: req.OnSnapshot})
	require.NoError(t, err, "a panicking callback never fails the apply")
}

func TestApplyWithRestoresTheSnapshotWhenSomethingPanics(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.onInstall = func() {
		writeFile(t, filepath.Join(e.dir, "config", "config.yaml"), "half: installed\n")
		writeFile(t, filepath.Join(e.dir, "config", "cscli-leftover.yaml"), "partial")
		panic("cscli stub exploded")
	}

	require.PanicsWithValue(t, "cscli stub exploded", func() {
		_, _ = e.svc.ApplyWith(context.Background(), e.request(nil))
	}, "the panic still reaches the caller's recoverer")

	e.requireRolledBack()
}

func TestApplyJobTurnsAPanickingInstallIntoAFailedJobWithTheTreeRestored(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.onInstall = func() {
		writeFile(t, filepath.Join(e.dir, "config", "cscli-leftover.yaml"), "partial")
		panic("boom")
	}
	db := testutil.OpenMemoryDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecApplyJob{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	m := NewApplyJobManager(nil)
	t.Cleanup(m.Shutdown)
	m.Configure(HubApplyConfig(e.svc, NewApplyJobRepository(db)))

	job, err := m.Start(ApplyJobRequest{Slug: applyTestSlug, Type: "collections", CacheKey: e.meta.CacheKey})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		got, getErr := m.Get(context.Background(), job.ID)
		_, busy := m.Busy()
		return getErr == nil && !busy && got.State == models.CrowdsecApplyJobFailed
	}, 10*time.Second, 10*time.Millisecond)
	e.requireRolledBack()
}
