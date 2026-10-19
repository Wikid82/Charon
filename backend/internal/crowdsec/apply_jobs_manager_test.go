package crowdsec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// This file holds the tests of the background hub install job manager (#1615, PR 6). They drive the real
// ApplyJobManager through a small harness that fakes ApplyWith and reads the stored rows back.

// Job states and phase codes, as in section 8.2 of the spec.
const (
	jobQueued    = models.CrowdsecApplyJobQueued
	jobRunning   = models.CrowdsecApplyJobRunning
	jobVerifying = models.CrowdsecApplyJobVerifying
	jobSucceeded = models.CrowdsecApplyJobSucceeded
	jobFailed    = models.CrowdsecApplyJobFailed
	jobTimedOut  = models.CrowdsecApplyJobTimedOut
	jobCancelled = models.CrowdsecApplyJobCancelled

	phasePreflight = string(PhasePreflight)
	phaseSnapshot  = string(PhaseSnapshot)
	phaseInstall   = string(PhaseInstall)
	phaseVerify    = string(PhaseVerify)
	phaseReload    = string(PhaseReload)
)

// errPendingBusy is the manager's "a job is active" error.
var errPendingBusy = ErrApplyJobBusy

// pendingJob is the stored status of one job.
type pendingJob struct {
	ID             string
	Slug           string
	ItemType       string
	CacheKey       string
	State          string
	Phase          string
	ErrorCode      string
	BackupPath     string
	Warnings       []string
	ItemsInstalled int
	StartedAt      time.Time
	FinishedAt     *time.Time
}

// pendingApplyFunc is what the manager calls instead of HubService.ApplyWith; onPhase reports phase boundaries.
type pendingApplyFunc func(ctx context.Context, slug, itemType, cacheKey string, onPhase func(phase string)) (ApplyResult, error)

type pendingJobConfig struct {
	DataDir          string
	Apply            pendingApplyFunc
	CountItems       func() int // stands in for the InstalledHubItems scan
	CountDownloaded  func() int // stands in for the hub file count
	ProgressInterval time.Duration
	MaxDuration      time.Duration
	Now              func() time.Time
}

type pendingJobManager interface {
	Start(ctx context.Context, slug, itemType, cacheKey string) (jobID string, err error)
	Get(id string) (pendingJob, error)
	Current() (pendingJob, bool)
	Cancel(id string) error
	Busy() (activeJobID string, busy bool)
	Wait(id string, timeout time.Duration) (pendingJob, error)
	Seed(job pendingJob) error // inserts a row as the database would hold it
	List() []pendingJob        // every stored job, newest first
	Prune() error
	RecoverInterrupted() error
}

// jobHarness adapts ApplyJobManager to pendingJobManager.
type jobHarness struct {
	t       *testing.T
	mgr     *ApplyJobManager
	repo    *ApplyJobRepository
	db      *gorm.DB
	dataDir string
}

func newPendingJobManager(t *testing.T, cfg pendingJobConfig) pendingJobManager {
	t.Helper()
	db := testutil.OpenMemoryDB(t)
	sqlDB, dbErr := db.DB()
	require.NoError(t, dbErr)
	sqlDB.SetMaxOpenConns(1) // shared-cache SQLite answers concurrent connections with "table is locked"
	require.NoError(t, db.AutoMigrate(&models.CrowdsecApplyJob{}))
	repo := NewApplyJobRepository(db)
	apply := cfg.Apply
	if apply == nil {
		apply = func(context.Context, string, string, string, func(string)) (ApplyResult, error) {
			return ApplyResult{Status: "applied"}, nil
		}
	}
	mgr := NewApplyJobManager(nil)
	mgr.Configure(ApplyJobConfig{
		Repo: repo, CountItems: cfg.CountItems, CountDownloaded: cfg.CountDownloaded, ProgressInterval: cfg.ProgressInterval, MaxDuration: cfg.MaxDuration, Now: cfg.Now,
		Apply: func(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
			return apply(ctx, req.Slug, req.Type, req.CacheKey, func(p string) { req.OnPhase(Phase(p)) })
		},
	})
	t.Cleanup(mgr.Shutdown)
	return &jobHarness{t: t, mgr: mgr, repo: repo, db: db, dataDir: cfg.DataDir}
}

func toPending(j *models.CrowdsecApplyJob) pendingJob {
	out := pendingJob{ID: j.ID, Slug: j.Slug, ItemType: j.ItemType, CacheKey: j.CacheKey, State: j.State, Phase: j.Phase,
		ErrorCode: j.ErrorCode, BackupPath: j.BackupPath, ItemsInstalled: j.ItemsInstalled, FinishedAt: j.FinishedAt}
	if j.StartedAt != nil {
		out.StartedAt = *j.StartedAt
	}
	if j.Warnings != "" {
		_ = json.Unmarshal([]byte(j.Warnings), &out.Warnings)
	}
	return out
}

func (h *jobHarness) Start(_ context.Context, slug, itemType, cacheKey string) (string, error) {
	job, err := h.mgr.Start(ApplyJobRequest{Slug: slug, Type: itemType, CacheKey: cacheKey})
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

func (h *jobHarness) Get(id string) (pendingJob, error) {
	job, err := h.mgr.Get(context.Background(), id)
	if err != nil {
		return pendingJob{}, err
	}
	return toPending(job), nil
}

func (h *jobHarness) Current() (pendingJob, bool) {
	job, err := h.mgr.Current(context.Background())
	if err != nil {
		return pendingJob{}, false
	}
	return toPending(job), true
}

func (h *jobHarness) Cancel(id string) error { return h.mgr.Cancel(id) }

func (h *jobHarness) Busy() (string, bool) { return h.mgr.Busy() }

// Wait returns once the job is finished and the busy flag is free again.
func (h *jobHarness) Wait(id string, timeout time.Duration) (pendingJob, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := h.Get(id)
		if err != nil {
			return pendingJob{}, err
		}
		if _, busy := h.mgr.Busy(); !models.IsCrowdsecApplyJobActive(job.State) && !busy {
			return job, nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return pendingJob{}, fmt.Errorf("job %s did not finish within %s", id, timeout)
}

func (h *jobHarness) Seed(job pendingJob) error {
	row := &models.CrowdsecApplyJob{ID: job.ID, Slug: job.Slug, State: job.State, BackupPath: job.BackupPath,
		CreatedAt: job.StartedAt, StartedAt: &job.StartedAt, FinishedAt: job.FinishedAt}
	if job.FinishedAt != nil {
		row.UpdatedAt = *job.FinishedAt
	}
	return h.repo.Create(context.Background(), row)
}

func (h *jobHarness) List() []pendingJob {
	jobs, err := h.mgr.Recent(context.Background(), 1000)
	require.NoError(h.t, err)
	out := make([]pendingJob, len(jobs))
	for i := range jobs {
		out[i] = toPending(&jobs[i])
	}
	return out
}

func (h *jobHarness) Prune() error { return h.mgr.Prune(context.Background()) }

func (h *jobHarness) RecoverInterrupted() error {
	return h.mgr.RecoverInterrupted(context.Background(), func(path string) error { return Restore(path, h.dataDir) })
}

// ---- helpers ----

// gatedApply is a fake Apply whose install step blocks until released or its context ends.
type gatedApply struct {
	started chan struct{}
	release chan struct{}
	result  ApplyResult
	err     error
	once    sync.Once
}

func newGatedApply() *gatedApply {
	return &gatedApply{started: make(chan struct{}), release: make(chan struct{}), result: ApplyResult{Status: "applied", UsedCSCLI: true}}
}

func (g *gatedApply) fn(ctx context.Context, _, _, _ string, onPhase func(string)) (ApplyResult, error) {
	for _, p := range []string{phasePreflight, phaseSnapshot, phaseInstall} {
		onPhase(p)
	}
	g.once.Do(func() { close(g.started) })
	select {
	case <-g.release:
	case <-ctx.Done():
		onPhase("rollback")
		return ApplyResult{Status: "failed"}, ctx.Err()
	}
	onPhase(phaseVerify)
	onPhase(phaseReload)
	return g.result, g.err
}

func startJob(t *testing.T, m pendingJobManager) string {
	t.Helper()
	id, err := m.Start(context.Background(), applyTestSlug, "collections", "key")
	require.NoError(t, err)
	return id
}

func awaitState(t *testing.T, m pendingJobManager, id string, states ...string) pendingJob {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := m.Get(id)
		require.NoError(t, err)
		for _, s := range states {
			if job.State == s {
				return job
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never reached %v", id, states)
	return pendingJob{}
}

// ---- (a) state machine ----

func TestApplyJobRunsThroughItsStatesAndPhases(t *testing.T) {
	t.Parallel()
	g := newGatedApply()
	items := 0
	var mu sync.Mutex
	m := newPendingJobManager(t, pendingJobConfig{
		DataDir: t.TempDir(), Apply: g.fn, ProgressInterval: 10 * time.Millisecond,
		CountItems: func() int { mu.Lock(); defer mu.Unlock(); return items },
	})

	id := startJob(t, m)
	<-g.started
	running := awaitState(t, m, id, jobRunning)
	require.Equal(t, phaseInstall, running.Phase)
	require.Nil(t, running.FinishedAt)

	mu.Lock()
	items = 12
	mu.Unlock()
	require.Eventually(t, func() bool { j, _ := m.Get(id); return j.ItemsInstalled == 12 }, 2*time.Second, 10*time.Millisecond, "items_installed follows the scan")

	close(g.release)
	done, err := m.Wait(id, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobSucceeded, done.State)
	require.NotNil(t, done.FinishedAt)
	_, busy := m.Busy()
	require.False(t, busy, "the flag is released at the end")
}

func TestApplyJobVerifyingStateIsVisible(t *testing.T) {
	t.Parallel()
	verifying := make(chan struct{})
	proceed := make(chan struct{})
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Apply: func(_ context.Context, _, _, _ string, onPhase func(string)) (ApplyResult, error) {
		onPhase(phaseVerify)
		close(verifying)
		<-proceed
		return ApplyResult{Status: "applied"}, nil
	}})

	id := startJob(t, m)
	<-verifying
	job := awaitState(t, m, id, jobVerifying)
	require.Equal(t, phaseVerify, job.Phase)
	close(proceed)
	done, err := m.Wait(id, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobSucceeded, done.State)
}

func TestApplyJobMapsEachFailureToStateAndCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		err       error
		state     string
		errorCode string
	}{
		{"stale cache", ErrPresetCacheStale, jobFailed, "preset_cache_stale"},
		{"hub updating", ErrPresetHubUpdating, jobFailed, "preset_hub_updating"},
		{"edited locally", &LocalModifiedError{Items: []string{"parsers/x"}}, jobFailed, "preset_local_modified"},
		{"cscli missing", ErrCSCLIUnavailable, jobFailed, "cscli_unavailable"},
		{"deadline", fmt.Errorf("%w: %w", ErrPresetApplyTimeout, context.DeadlineExceeded), jobTimedOut, "preset_apply_timeout"},
		{"cancelled", ErrPresetApplyCancelled, jobCancelled, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Apply: func(context.Context, string, string, string, func(string)) (ApplyResult, error) {
				return ApplyResult{Status: "failed", BackupPath: "/snap"}, tc.err
			}})

			done, err := m.Wait(startJob(t, m), 5*time.Second)
			require.NoError(t, err)
			require.Equal(t, tc.state, done.State)
			require.Equal(t, tc.errorCode, done.ErrorCode)
			require.Equal(t, "/snap", done.BackupPath, "the snapshot path is kept for recovery and support")
			_, busy := m.Busy()
			require.False(t, busy)
		})
	}
}

func TestApplyJobCapsTaintedDependencyWarnings(t *testing.T) {
	t.Parallel()
	warnings := make([]string, 80)
	for i := range warnings {
		warnings[i] = fmt.Sprintf("dependency test/dep%d was edited locally", i)
	}
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Apply: func(context.Context, string, string, string, func(string)) (ApplyResult, error) {
		return ApplyResult{Status: "applied", Warnings: warnings}, nil
	}})

	done, err := m.Wait(startJob(t, m), 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobSucceeded, done.State, "a tainted dependency only warns")
	require.Len(t, done.Warnings, 51)
	require.Equal(t, "and 30 more", done.Warnings[50])
}

func TestApplyJobCancelRollsBackAndIsIdempotent(t *testing.T) {
	t.Parallel()
	g := newGatedApply()
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Apply: g.fn})

	id := startJob(t, m)
	<-g.started
	require.NoError(t, m.Cancel(id))
	require.NoError(t, m.Cancel(id), "a second cancel is accepted")
	done, err := m.Wait(id, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobCancelled, done.State)
	require.NoError(t, m.Cancel(id), "cancelling a finished job is a no-op")
	_, busy := m.Busy()
	require.False(t, busy)
}

func TestApplyJobStopsAtTheTimeLimit(t *testing.T) {
	t.Parallel()
	g := newGatedApply()
	var gotDeadline bool
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), MaxDuration: 50 * time.Millisecond, Apply: func(ctx context.Context, s, ty, k string, on func(string)) (ApplyResult, error) {
		_, gotDeadline = ctx.Deadline()
		res, err := g.fn(ctx, s, ty, k, on)
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("%w: %w", ErrPresetApplyTimeout, err)
		}
		return res, err
	}})

	done, err := m.Wait(startJob(t, m), 5*time.Second)
	require.NoError(t, err)
	require.True(t, gotDeadline, "the job context carries the limit")
	require.Equal(t, jobTimedOut, done.State)
	require.Equal(t, "preset_apply_timeout", done.ErrorCode)
}

// ---- (b) busy flag ----

func TestApplyJobSecondStartIsRefusedWhileActive(t *testing.T) {
	t.Parallel()
	g := newGatedApply()
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Apply: g.fn})

	id := startJob(t, m)
	<-g.started
	active, busy := m.Busy()
	require.True(t, busy)
	require.Equal(t, id, active)

	_, err := m.Start(context.Background(), applyTestSlug, "collections", "key")
	require.ErrorIs(t, err, errPendingBusy)

	close(g.release)
	_, err = m.Wait(id, 5*time.Second)
	require.NoError(t, err)
	_, err = m.Start(context.Background(), applyTestSlug, "collections", "key")
	require.NoError(t, err, "the flag is free after the job ended")
}

func TestApplyJobReleasesTheFlagAfterAPanic(t *testing.T) {
	t.Parallel()
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Apply: func(context.Context, string, string, string, func(string)) (ApplyResult, error) {
		panic("boom")
	}})

	done, err := m.Wait(startJob(t, m), 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobFailed, done.State)
	_, busy := m.Busy()
	require.False(t, busy, "a panicking install cannot leave the flag set")
}

func TestApplyJobConcurrentStartCancelAndPoll(t *testing.T) {
	t.Parallel()
	g := newGatedApply()
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Apply: g.fn})
	id := startJob(t, m)
	<-g.started

	var wg sync.WaitGroup
	var started int32
	var mu sync.Mutex
	for range 20 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_, err := m.Start(context.Background(), applyTestSlug, "collections", "key")
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				started++
			} else {
				require.ErrorIs(t, err, errPendingBusy)
			}
		}()
		go func() { defer wg.Done(); _, _ = m.Get(id); _, _ = m.Current() }()
		go func() { defer wg.Done(); _ = m.Cancel(id) }()
	}
	wg.Wait()
	// A Start that raced in after the first job ended may hold the flag now; nobody else cancels it.
	if current, busy := m.Busy(); busy {
		_ = m.Cancel(current)
	}
	_, err := m.Wait(id, 5*time.Second)
	require.NoError(t, err)
	require.LessOrEqual(t, started, int32(1), "only one job can ever be accepted")
}

// ---- (c) restart recovery ----

func TestRecoverInterruptedRestoresTheSnapshotAndMarksTheJob(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	backup, err := e.svc.snapshot()
	require.NoError(t, err)
	// The install died half way: the live tree no longer matches the snapshot.
	writeFile(t, filepath.Join(e.dir, "config", "config.yaml"), "original: false\n")
	writeFile(t, filepath.Join(e.dir, "config", "cscli-leftover.yaml"), "partial")

	m := newPendingJobManager(t, pendingJobConfig{DataDir: e.dir})
	for _, state := range []string{jobQueued, jobRunning, jobVerifying} {
		require.NoError(t, m.Seed(pendingJob{ID: "job-" + state, Slug: applyTestSlug, State: state, BackupPath: backup, StartedAt: time.Now()}))
	}
	finished := time.Now().Add(-time.Hour)
	require.NoError(t, m.Seed(pendingJob{ID: "job-done", State: jobSucceeded, StartedAt: finished, FinishedAt: &finished}))

	require.NoError(t, m.RecoverInterrupted())

	for _, state := range []string{jobQueued, jobRunning, jobVerifying} {
		job, getErr := m.Get("job-" + state)
		require.NoError(t, getErr)
		require.Equal(t, jobFailed, job.State)
		require.Equal(t, "interrupted", job.ErrorCode)
		require.NotNil(t, job.FinishedAt)
	}
	done, err := m.Get("job-done")
	require.NoError(t, err)
	require.Equal(t, jobSucceeded, done.State, "finished jobs are left alone")
	require.Equal(t, "original: true\n", readFile(t, filepath.Join(e.dir, "config", "config.yaml")), "Restore(job.BackupPath, dataDir) ran")
	require.NoFileExists(t, filepath.Join(e.dir, "config", "cscli-leftover.yaml"))
	_, busy := m.Busy()
	require.False(t, busy, "the flag starts clear after a restart")
}

func TestRecoverInterruptedWithoutASnapshotOnlyMarksTheJob(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(e.dir, "config"), 0o750))
	writeFile(t, filepath.Join(e.dir, "config", "config.yaml"), "edited: true\n")
	m := newPendingJobManager(t, pendingJobConfig{DataDir: e.dir})
	require.NoError(t, m.Seed(pendingJob{ID: "a", State: jobRunning, StartedAt: time.Now(), BackupPath: ""}))
	require.NoError(t, m.Seed(pendingJob{ID: "b", State: jobRunning, StartedAt: time.Now(), BackupPath: filepath.Join(e.dir, "gone.backup")}))

	require.NoError(t, m.RecoverInterrupted(), "a missing snapshot must not stop the start")

	for _, id := range []string{"a", "b"} {
		job, err := m.Get(id)
		require.NoError(t, err)
		require.Equal(t, jobFailed, job.State)
		require.Equal(t, "interrupted", job.ErrorCode)
	}
	require.Equal(t, "edited: true\n", readFile(t, filepath.Join(e.dir, "config", "config.yaml")), "nothing is restored without a snapshot")
}

// ---- (d) retention ----

func TestApplyJobRetentionKeeps20JobsAndSevenDays(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Now: func() time.Time { return now }})
	seed := func(id string, age time.Duration, state string) {
		finished := now.Add(-age)
		job := pendingJob{ID: id, State: state, StartedAt: finished.Add(-time.Minute)}
		if state != jobRunning {
			job.FinishedAt = &finished
		}
		require.NoError(t, m.Seed(job))
	}
	for i := range 25 { // ten days old: only the 20 newest of all finished jobs survive
		seed(fmt.Sprintf("old-%02d", i), 10*24*time.Hour+time.Duration(i)*time.Minute, jobSucceeded)
	}
	for i := range 3 { // within seven days: always kept
		seed(fmt.Sprintf("recent-%d", i), time.Duration(i+1)*time.Hour, jobFailed)
	}
	seed("active", 0, jobRunning)

	require.NoError(t, m.Prune())

	ids := map[string]bool{}
	for _, j := range m.List() {
		ids[j.ID] = true
	}
	require.Len(t, ids, 21, "20 finished jobs plus the active one")
	require.True(t, ids["active"], "an active job is never pruned")
	for i := range 3 {
		require.True(t, ids[fmt.Sprintf("recent-%d", i)])
	}
	require.True(t, ids["old-16"], "the 20th newest finished job stays")
	require.False(t, ids["old-17"], "older than 7 days and beyond the newest 20")
}

func TestApplyJobRetentionKeepsEverythingNewerThanSevenDays(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Now: func() time.Time { return now }})
	for i := range 30 {
		finished := now.Add(-time.Duration(i+1) * time.Hour)
		require.NoError(t, m.Seed(pendingJob{ID: fmt.Sprintf("j-%02d", i), State: jobSucceeded, StartedAt: finished, FinishedAt: &finished}))
	}

	require.NoError(t, m.Prune())

	require.Len(t, m.List(), 30, "thirty jobs from the last day are all newer than seven days")
}

func TestApplyJobStartPrunes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	g := newGatedApply()
	close(g.release)
	m := newPendingJobManager(t, pendingJobConfig{DataDir: t.TempDir(), Apply: g.fn, Now: func() time.Time { return now }})
	for i := range 30 {
		finished := now.Add(-(8*24*time.Hour + time.Duration(i)*time.Minute))
		require.NoError(t, m.Seed(pendingJob{ID: fmt.Sprintf("old-%02d", i), State: jobSucceeded, StartedAt: finished, FinishedAt: &finished}))
	}

	id := startJob(t, m)
	_, err := m.Wait(id, 5*time.Second)
	require.NoError(t, err)

	require.LessOrEqual(t, len(m.List()), 21, "starting a job prunes the stale history")
}
