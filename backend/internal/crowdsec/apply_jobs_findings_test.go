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
	"gorm.io/gorm"
)

// A job's run holds the busy flag until its very last step (the retention prune). A Start that slips in
// between the final write and the release would otherwise lose its flag when the first run released late.
func TestApplyJobHoldsTheFlagUntilItsGoroutineIsDone(t *testing.T) {
	t.Parallel()
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir()})

	var deletes atomic.Int32
	var startErr atomic.Value
	require.NoError(t, h.db.Callback().Delete().Before("gorm:begin_transaction").Register("test:second-start-in-prune", func(*gorm.DB) {
		if deletes.Add(1) != 2 { // 1 = the prune of Start, 2 = the prune at the end of run
			return
		}
		_, err := h.mgr.Start(ApplyJobRequest{Slug: applyTestSlug, Type: "collections", CacheKey: "key"})
		startErr.Store(fmt.Sprint(err))
	}))

	id, err := h.Start(context.Background(), applyTestSlug, "collections", "key")
	require.NoError(t, err)
	done, err := h.Wait(id, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobSucceeded, done.State)

	require.Equal(t, ErrApplyJobBusy.Error(), startErr.Load(), "no job may start while the previous one is still finishing")
	_, busy := h.Busy()
	require.False(t, busy)
	next, err := h.Start(context.Background(), applyTestSlug, "collections", "key")
	require.NoError(t, err)
	_, err = h.Wait(next, 5*time.Second)
	require.NoError(t, err)
}

func TestApplyJobCancelOfAnActiveRowThisProcessDoesNotRun(t *testing.T) {
	t.Parallel()
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir()})
	require.NoError(t, h.Seed(pendingJob{ID: "leftover", State: jobRunning, StartedAt: time.Now()}))

	require.ErrorIs(t, h.Cancel("leftover"), ErrApplyJobNotActive)
}

func TestApplyJobStartRefusedAfterShutdown(t *testing.T) {
	t.Parallel()
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir()})
	h.mgr.Shutdown()

	_, err := h.Start(context.Background(), applyTestSlug, "collections", "key")

	require.ErrorIs(t, err, ErrApplyJobShutdown)
	_, busy := h.Busy()
	require.False(t, busy, "a refused start leaves no flag behind")
	require.Empty(t, h.List(), "and no job row")
}

func TestApplyJobFinalStateOnlyCancelsOnAContextCancel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		err           error
		cancelClicked bool
		state         string
		code          string
	}{
		{"cancel click with a genuine failure", ErrPresetCacheStale, true, jobFailed, "preset_cache_stale"},
		{"cancel with a failed rollback", rollbackFailure(ErrPresetApplyCancelled, errors.New("disk"), "/snap"), true, jobFailed, ""},
		{"timeout with a failed rollback", rollbackFailure(fmt.Errorf("%w: %w", ErrPresetApplyTimeout, context.DeadlineExceeded), errors.New("disk"), "/snap"), false, jobFailed, "preset_apply_timeout"},
		{"clean cancel", ErrPresetApplyCancelled, true, jobCancelled, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, pendingJobConfig{DataDir: t.TempDir(), Apply: func(context.Context, string, string, string, func(string)) (ApplyResult, error) {
				return ApplyResult{Status: "failed", BackupPath: "/snap"}, tc.err
			}})
			id, err := h.Start(context.Background(), applyTestSlug, "collections", "key")
			require.NoError(t, err)
			if tc.cancelClicked {
				_ = h.Cancel(id) // may race with the end of the job; the state decides below
			}
			done, err := h.Wait(id, 5*time.Second)
			require.NoError(t, err)
			require.Equal(t, tc.state, done.State)
			require.Equal(t, tc.code, done.ErrorCode)
		})
	}
}

func TestApplyJobResetsTheItemCountOfAnUnsuccessfulJob(t *testing.T) {
	t.Parallel()
	var count atomic.Int64
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir(), ProgressInterval: 5 * time.Millisecond, CountItems: func() int { return int(count.Load()) }})
	h.mgr.cfg.Apply = func(_ context.Context, req ApplyRequest) (ApplyResult, error) {
		req.OnPhase(PhaseInstall)
		count.Store(4)
		time.Sleep(60 * time.Millisecond) // long enough for a progress write of 4
		return ApplyResult{Status: "failed"}, ErrPresetHubUpdating
	}

	id, err := h.Start(context.Background(), applyTestSlug, "collections", "key")
	require.NoError(t, err)
	done, err := h.Wait(id, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobFailed, done.State)
	require.Zero(t, done.ItemsInstalled, "the rollback removed what was installed")
}

func lightSnapshotFixture(t *testing.T) string {
	t.Helper()
	e := newApplyEnv(t)
	path, err := e.svc.snapshotWithoutHub(itemState{})
	require.NoError(t, err)
	return path
}

func TestApplyJobRecoveryRestoresBeforeItFailsTheJob(t *testing.T) {
	t.Parallel()
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir()})
	snap := lightSnapshotFixture(t)
	require.NoError(t, h.Seed(pendingJob{ID: "j", State: jobRunning, StartedAt: time.Now(), BackupPath: snap}))

	var stateDuringRestore string
	err := h.mgr.RecoverInterrupted(context.Background(), func(string) error {
		job, getErr := h.Get("j")
		require.NoError(t, getErr)
		stateDuringRestore = job.State
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, jobRunning, stateDuringRestore, "the job stays active until its restore ran, so a crash retries it")
	job, err := h.Get("j")
	require.NoError(t, err)
	require.Equal(t, jobFailed, job.State)
	require.Equal(t, "interrupted", job.ErrorCode)
	require.Empty(t, job.Warnings)
	require.NoFileExists(t, snap+retainMarkerSuffix, "a successful restore keeps nothing")
}

func TestApplyJobRecoveryRecordsAFailedRestoreTogetherWithTheFailedState(t *testing.T) {
	t.Parallel()
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir()})
	snap := lightSnapshotFixture(t)
	require.NoError(t, h.Seed(pendingJob{ID: "j", State: jobRunning, StartedAt: time.Now(), BackupPath: snap}))

	require.NoError(t, h.mgr.RecoverInterrupted(context.Background(), func(string) error { return errors.New("disk on fire") }))

	job, err := h.Get("j")
	require.NoError(t, err)
	require.Equal(t, jobFailed, job.State)
	require.Len(t, job.Warnings, 1)
	require.Contains(t, job.Warnings[0], "could not be restored")
	require.FileExists(t, snap+retainMarkerSuffix, "a failed restore keeps its snapshot beyond automatic cleanup")
}

func TestApplyJobRecoveryNeverRestoresAnImplausibleSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	require.NoError(t, os.MkdirAll(empty, 0o750))
	unrelated := filepath.Join(dir, "unrelated")
	require.NoError(t, os.MkdirAll(unrelated, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(unrelated, "notes.txt"), []byte("hi"), 0o600))
	file := filepath.Join(dir, "afile")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	h := newHarness(t, pendingJobConfig{DataDir: dir})
	for _, p := range []string{empty, unrelated, file} {
		require.NoError(t, h.Seed(pendingJob{ID: filepath.Base(p), State: jobRunning, StartedAt: time.Now(), BackupPath: p}))
	}

	err := h.mgr.RecoverInterrupted(context.Background(), func(p string) error {
		t.Errorf("restore must not be called for %s", p)
		return nil
	})

	require.NoError(t, err)
	for _, id := range []string{"empty", "unrelated", "afile"} {
		job, getErr := h.Get(id)
		require.NoError(t, getErr)
		require.Equal(t, jobFailed, job.State)
		require.Len(t, job.Warnings, 1)
		require.Contains(t, job.Warnings[0], "incomplete")
	}
}

func TestValidSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mk := func(rel string, content string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	mk("flat/config.yaml", "a: b\n")
	mk("nested/config/config.yaml", "a: b\n")
	mk("sidecar-no-manifest/.charon-apply/other", "x")
	mk("light/.charon-apply/hub-manifest.json", "[]")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "empty"), 0o750))
	mk("unrelated/readme.txt", "x")

	cases := map[string]bool{
		"flat": true, "nested": true, "light": true,
		"sidecar-no-manifest": false, "empty": false, "unrelated": false, "missing": false, "flat/config.yaml": false,
	}
	for name, want := range cases {
		require.Equal(t, want, ValidSnapshot(filepath.Join(dir, name)), name)
	}
	require.True(t, ValidSnapshot(lightSnapshotFixture(t)), "what snapshotWithoutHub produces is valid")
}
