package crowdsec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
)

// Defaults of the background hub preset install (#1615).
const (
	// ApplyJobMaxDuration is the time limit of one background install, rollback excluded.
	ApplyJobMaxDuration = 30 * time.Minute
	// ApplyJobProgressInterval is how often the installed-item count is scanned and written.
	ApplyJobProgressInterval = 2 * time.Second

	applyJobDBTimeout = 10 * time.Second
)

// ErrApplyJobBusy is returned by Start when another job, or the hub item restore, holds the busy flag.
var ErrApplyJobBusy = errors.New("crowdsec is busy with another long operation")

// ErrApplyJobNotConfigured is returned by Start when Configure has not supplied the repository and apply function.
var ErrApplyJobNotConfigured = errors.New("crowdsec apply job manager is not configured")

// ApplyFunc runs one preset apply. HubService.ApplyWith is the production implementation; tests substitute it.
type ApplyFunc func(ctx context.Context, req ApplyRequest) (ApplyResult, error)

// ApplyJobConfig wires the manager to its storage and to the apply it runs.
type ApplyJobConfig struct {
	Repo  *ApplyJobRepository
	Apply ApplyFunc
	// CountItems returns the number of hub items installed right now; the job reports the growth since the
	// install phase began. Nil disables progress counting.
	CountItems func() int
	// ProgressInterval and MaxDuration default to ApplyJobProgressInterval and ApplyJobMaxDuration.
	ProgressInterval time.Duration
	MaxDuration      time.Duration
	// Now overrides the clock of the repository (tests).
	Now func() time.Time
}

// HubApplyConfig returns the production configuration: svc.ApplyWith and a scan of the installed hub items.
func HubApplyConfig(svc *HubService, repo *ApplyJobRepository) ApplyJobConfig {
	return ApplyJobConfig{
		Repo:  repo,
		Apply: svc.ApplyWith,
		CountItems: func() int {
			items, err := InstalledHubItems(svc.DataDir)
			if err != nil {
				return 0
			}
			return len(items)
		},
	}
}

// ApplyJobRequest names the preset a job installs.
type ApplyJobRequest struct {
	Slug     string
	Type     string
	CacheKey string
}

// activeApplyJob is the in-memory side of the job that holds the busy flag.
type activeApplyJob struct {
	id              string
	cancel          context.CancelFunc
	cancelRequested atomic.Bool

	mu          sync.Mutex
	phase       string
	baseline    int
	lastWritten int
}

func (a *activeApplyJob) setPhase(phase string) {
	a.mu.Lock()
	a.phase = phase
	a.mu.Unlock()
}

// ApplyJobManager holds the busy flag of the background hub preset install (#1615) and runs the jobs. While
// a job holds the flag, every handler that changes DataDir or the engine process answers 409 crowdsec_busy
// instead of waiting. The job itself runs without any lock; the flag only decides who may start and who
// must wait.
type ApplyJobManager struct {
	// blockedBy reports a competing long operation (the hub item restore) that forbids starting a job.
	blockedBy func() bool

	// rootCtx is the manager-lifetime context every job derives from; Shutdown cancels it. It is never an
	// HTTP request context.
	rootCtx    context.Context
	rootCancel context.CancelFunc
	wg         sync.WaitGroup

	mu     sync.Mutex
	jobID  string
	busy   bool
	cfg    ApplyJobConfig
	active *activeApplyJob
}

// NewApplyJobManager returns a manager with no active job. blockedBy may be nil. Configure must be called
// before Start.
func NewApplyJobManager(blockedBy func() bool) *ApplyJobManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ApplyJobManager{blockedBy: blockedBy, rootCtx: ctx, rootCancel: cancel}
}

// Configure supplies the repository and apply function; defaults fill the unset limits.
func (m *ApplyJobManager) Configure(cfg ApplyJobConfig) {
	if cfg.ProgressInterval <= 0 {
		cfg.ProgressInterval = ApplyJobProgressInterval
	}
	if cfg.MaxDuration <= 0 {
		cfg.MaxDuration = ApplyJobMaxDuration
	}
	if cfg.Now != nil && cfg.Repo != nil {
		cfg.Repo.now = cfg.Now
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
}

// TryAcquire takes the busy flag for jobID. It fails when a job already holds it or when the competing
// operation reported by blockedBy is running. The flag is taken before blockedBy is consulted and released
// again on refusal, so a restore that starts at the same moment sees the flag or this call sees the restore.
func (m *ApplyJobManager) TryAcquire(jobID string) bool {
	m.mu.Lock()
	if m.busy {
		m.mu.Unlock()
		return false
	}
	m.busy, m.jobID = true, jobID
	m.mu.Unlock()

	if m.blockedBy != nil && m.blockedBy() {
		m.Release()
		return false
	}
	return true
}

// Release clears the busy flag; it is safe to call when no job holds it.
func (m *ApplyJobManager) Release() {
	m.mu.Lock()
	m.busy, m.jobID, m.active = false, "", nil
	m.mu.Unlock()
}

// Busy reports whether a job holds the flag and its id.
func (m *ApplyJobManager) Busy() (jobID string, busy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobID, m.busy
}

func (m *ApplyJobManager) config() ApplyJobConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func dbContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), applyJobDBTimeout)
}

// Start takes the busy flag, records a queued job and runs the install in the background. It returns the
// queued job, or ErrApplyJobBusy when the flag is taken. The job runs on the manager's own context with its
// own time limit, so it outlives the HTTP request that started it.
func (m *ApplyJobManager) Start(req ApplyJobRequest) (*models.CrowdsecApplyJob, error) {
	cfg := m.config()
	if cfg.Repo == nil || cfg.Apply == nil {
		return nil, ErrApplyJobNotConfigured
	}
	id := uuid.New().String()
	if !m.TryAcquire(id) {
		return nil, ErrApplyJobBusy
	}
	jobCtx, cancel := context.WithTimeout(m.rootCtx, cfg.MaxDuration)
	a := &activeApplyJob{id: id, cancel: cancel}
	m.mu.Lock()
	m.active = a
	m.mu.Unlock()

	ctx, dbCancel := dbContext()
	defer dbCancel()
	m.prune(ctx, cfg)
	job := &models.CrowdsecApplyJob{ID: id, Slug: req.Slug, ItemType: req.Type, CacheKey: req.CacheKey, State: models.CrowdsecApplyJobQueued}
	if err := cfg.Repo.Create(ctx, job); err != nil {
		cancel()
		m.Release()
		return nil, err
	}

	m.wg.Add(1)
	go m.run(jobCtx, cfg, a, req)
	return job, nil
}

func (m *ApplyJobManager) prune(ctx context.Context, cfg ApplyJobConfig) {
	if _, err := cfg.Repo.Prune(ctx, ApplyJobKeepFinished, ApplyJobMaxAge); err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).Warn("pruning crowdsec apply jobs failed")
	}
}

// Prune applies the job retention (newest 20 finished jobs plus anything younger than 7 days).
func (m *ApplyJobManager) Prune(ctx context.Context) error {
	cfg := m.config()
	if cfg.Repo == nil {
		return ErrApplyJobNotConfigured
	}
	_, err := cfg.Repo.Prune(ctx, ApplyJobKeepFinished, ApplyJobMaxAge)
	return err
}

// update writes a partial job update and logs a failure: a lost progress write must never stop an install.
func (m *ApplyJobManager) update(cfg ApplyJobConfig, id string, upd ApplyJobUpdate) {
	ctx, cancel := dbContext()
	defer cancel()
	if err := cfg.Repo.Update(ctx, id, upd); err != nil {
		logger.Log().WithField("job_id", id).WithField("error", util.SanitizeForLog(err.Error())).Warn("updating crowdsec apply job failed")
	}
}

// run executes one job to its end and always releases the busy flag.
func (m *ApplyJobManager) run(ctx context.Context, cfg ApplyJobConfig, a *activeApplyJob, req ApplyJobRequest) {
	defer m.wg.Done()
	defer m.Release()
	defer a.cancel()

	started := cfg.Repo.now()
	running, phase := models.CrowdsecApplyJobRunning, ""
	m.update(cfg, a.id, ApplyJobUpdate{State: &running, Phase: &phase, StartedAt: &started})

	stop, polled := make(chan struct{}), make(chan struct{})
	go m.pollProgress(cfg, a, stop, polled)

	res, panicked, err := m.execute(ctx, cfg, a, req)
	close(stop)
	<-polled

	upd := m.finalUpdate(cfg, a, res, err, panicked)
	m.update(cfg, a.id, upd)
	m.Release()

	pruneCtx, cancel := dbContext()
	defer cancel()
	m.prune(pruneCtx, cfg)
}

// execute runs the apply and turns a panic into an error so the caller always reaches the final write.
func (m *ApplyJobManager) execute(ctx context.Context, cfg ApplyJobConfig, a *activeApplyJob, req ApplyJobRequest) (res ApplyResult, panicked bool, err error) {
	var backupPath string
	defer func() {
		if rec := recover(); rec != nil {
			logger.Log().WithField("job_id", a.id).WithField("panic", util.SanitizeForLog(fmt.Sprint(rec))).Error("crowdsec preset install panicked")
			res, panicked, err = ApplyResult{Status: "failed", BackupPath: backupPath}, true, errors.New("internal error during the preset install")
		}
	}()
	res, err = cfg.Apply(ctx, ApplyRequest{
		Slug: req.Slug, Type: req.Type, CacheKey: req.CacheKey, Timeout: cfg.MaxDuration,
		OnPhase: func(p Phase) { m.onPhase(cfg, a, p) },
		OnSnapshot: func(path string) {
			backupPath = path
			m.update(cfg, a.id, ApplyJobUpdate{BackupPath: &path})
		},
	})
	return res, false, err
}

// onPhase records the phase boundary: the phase code, the verifying state, and the install baseline.
func (m *ApplyJobManager) onPhase(cfg ApplyJobConfig, a *activeApplyJob, p Phase) {
	phase := string(p)
	state := models.CrowdsecApplyJobRunning
	if p == PhaseVerify || p == PhaseReload {
		state = models.CrowdsecApplyJobVerifying
	}
	if p == PhaseInstall && cfg.CountItems != nil {
		base := cfg.CountItems()
		a.mu.Lock()
		a.baseline = base
		a.mu.Unlock()
	}
	a.setPhase(phase)
	m.update(cfg, a.id, ApplyJobUpdate{State: &state, Phase: &phase})
}

// pollProgress writes the number of items installed since the install phase began, once per interval and
// only when it changed, until stop is closed.
func (m *ApplyJobManager) pollProgress(cfg ApplyJobConfig, a *activeApplyJob, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	if cfg.CountItems == nil {
		<-stop
		return
	}
	t := time.NewTicker(cfg.ProgressInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			a.mu.Lock()
			inInstall := a.phase == string(PhaseInstall)
			a.mu.Unlock()
			if inInstall {
				m.writeProgress(cfg, a)
			}
		}
	}
}

// writeProgress writes the installed-item growth when it differs from the last written value.
func (m *ApplyJobManager) writeProgress(cfg ApplyJobConfig, a *activeApplyJob) {
	a.mu.Lock()
	base := a.baseline
	a.mu.Unlock()
	n := max(0, cfg.CountItems()-base)
	a.mu.Lock()
	changed := n != a.lastWritten
	a.lastWritten = n
	a.mu.Unlock()
	if changed {
		m.update(cfg, a.id, ApplyJobUpdate{ItemsInstalled: &n})
	}
}

// finalUpdate maps the apply outcome to the job's terminal state, code, message, warnings and snapshot path.
func (m *ApplyJobManager) finalUpdate(cfg ApplyJobConfig, a *activeApplyJob, res ApplyResult, err error, panicked bool) ApplyJobUpdate {
	finished := cfg.Repo.now()
	state := models.CrowdsecApplyJobSucceeded
	var code, message string
	switch {
	case err == nil:
	case !panicked && (errors.Is(err, ErrPresetApplyTimeout) || errors.Is(err, context.DeadlineExceeded)):
		state, code = models.CrowdsecApplyJobTimedOut, models.CrowdsecApplyErrTimeout
		message = RedactPaths(err.Error())
	case !panicked && (errors.Is(err, ErrPresetApplyCancelled) || errors.Is(err, context.Canceled) || a.cancelRequested.Load()):
		state = models.CrowdsecApplyJobCancelled
		message = RedactPaths(err.Error())
	default:
		state, code = models.CrowdsecApplyJobFailed, ApplyErrorCode(err)
		message = RedactPaths(err.Error())
	}
	upd := ApplyJobUpdate{State: &state, FinishedAt: &finished}
	if code != "" {
		upd.ErrorCode = &code
	}
	if message != "" {
		upd.Error = &message
	}
	if res.BackupPath != "" {
		upd.BackupPath = &res.BackupPath
	}
	if warnings, encErr := EncodeApplyWarnings(res.Warnings); encErr == nil && warnings != "" {
		upd.Warnings = &warnings
	}
	if state == models.CrowdsecApplyJobSucceeded && cfg.CountItems != nil {
		a.mu.Lock()
		n := max(0, cfg.CountItems()-a.baseline)
		a.mu.Unlock()
		upd.ItemsInstalled = &n
	}
	return upd
}

// Cancel asks the active job to stop; it rolls back and ends cancelled. It is idempotent and a no-op for a
// finished job, and returns ErrApplyJobNotFound for an unknown id.
func (m *ApplyJobManager) Cancel(jobID string) error {
	cfg := m.config()
	if cfg.Repo == nil {
		return ErrApplyJobNotConfigured
	}
	m.mu.Lock()
	a := m.active
	m.mu.Unlock()
	if a != nil && a.id == jobID {
		a.cancelRequested.Store(true)
		requested := true
		m.update(cfg, jobID, ApplyJobUpdate{CancelRequested: &requested})
		a.cancel()
		return nil
	}
	ctx, cancel := dbContext()
	defer cancel()
	_, err := cfg.Repo.Get(ctx, jobID)
	return err
}

// Get returns the job with id, or ErrApplyJobNotFound.
func (m *ApplyJobManager) Get(ctx context.Context, id string) (*models.CrowdsecApplyJob, error) {
	cfg := m.config()
	if cfg.Repo == nil {
		return nil, ErrApplyJobNotConfigured
	}
	return cfg.Repo.Get(ctx, id)
}

// Current returns the newest active job, or ErrApplyJobNotFound when none is active.
func (m *ApplyJobManager) Current(ctx context.Context) (*models.CrowdsecApplyJob, error) {
	cfg := m.config()
	if cfg.Repo == nil {
		return nil, ErrApplyJobNotConfigured
	}
	return cfg.Repo.Current(ctx)
}

// Recent returns up to limit jobs, newest first.
func (m *ApplyJobManager) Recent(ctx context.Context, limit int) ([]models.CrowdsecApplyJob, error) {
	cfg := m.config()
	if cfg.Repo == nil {
		return nil, ErrApplyJobNotConfigured
	}
	return cfg.Repo.ListRecent(ctx, limit)
}

// RecoverInterrupted fails every job a previous process left active and calls restore with each job's
// snapshot path, because the install died half way. A failed restore is recorded as a warning on the job
// (the snapshot path stays on the row for manual recovery) and never stops the start.
func (m *ApplyJobManager) RecoverInterrupted(ctx context.Context, restore func(backupPath string) error) error {
	cfg := m.config()
	if cfg.Repo == nil {
		return ErrApplyJobNotConfigured
	}
	jobs, err := cfg.Repo.MarkRunningAsInterrupted(ctx)
	if err != nil {
		return err
	}
	for i := range jobs {
		job := &jobs[i]
		if job.BackupPath == "" || restore == nil {
			continue
		}
		// Restore empties the data directory before it copies, so a snapshot that is gone must never reach it.
		if _, statErr := os.Stat(job.BackupPath); statErr != nil {
			m.recordRecoveryWarning(ctx, cfg, job.ID, "the snapshot of the interrupted install no longer exists, so nothing was restored")
			continue
		}
		restoreErr := restore(job.BackupPath)
		if restoreErr == nil {
			logger.Log().WithField("job_id", job.ID).Info("restored the configuration snapshot of an interrupted crowdsec install")
			continue
		}
		msg := "the snapshot could not be restored: " + RedactPaths(restoreErr.Error())
		logger.Log().WithField("job_id", job.ID).WithField("error", util.SanitizeForLog(msg)).Error("restoring an interrupted crowdsec install failed")
		m.recordRecoveryWarning(ctx, cfg, job.ID, msg)
	}
	return nil
}

// recordRecoveryWarning stores msg as the warning of an interrupted job; a failed write is only logged.
func (m *ApplyJobManager) recordRecoveryWarning(ctx context.Context, cfg ApplyJobConfig, id, msg string) {
	warnings, err := EncodeApplyWarnings([]string{msg})
	if err == nil {
		err = cfg.Repo.Update(ctx, id, ApplyJobUpdate{Warnings: &warnings})
	}
	if err != nil {
		logger.Log().WithField("job_id", id).WithField("error", util.SanitizeForLog(err.Error())).Warn("recording the recovery warning failed")
	}
}

// Shutdown cancels every running job (each rolls back) and waits for them to end.
func (m *ApplyJobManager) Shutdown() {
	m.rootCancel()
	m.wg.Wait()
}
