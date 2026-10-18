package crowdsec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Retention of finished background install jobs: the newest ApplyJobKeepFinished stay, plus anything
// finished within ApplyJobMaxAge.
const (
	ApplyJobKeepFinished = 20
	ApplyJobMaxAge       = 7 * 24 * time.Hour
)

// ErrApplyJobNotFound is returned when no job has the requested id.
var ErrApplyJobNotFound = errors.New("crowdsec apply job not found")

// ApplyJobRepository stores CrowdsecApplyJob rows. All queries are parameterised GORM calls.
type ApplyJobRepository struct {
	db  *gorm.DB
	now func() time.Time
}

// NewApplyJobRepository returns a repository over db.
func NewApplyJobRepository(db *gorm.DB) *ApplyJobRepository {
	return &ApplyJobRepository{db: db, now: time.Now}
}

// ApplyJobUpdate is a partial update: only non-nil fields are written, so frequent progress writes touch
// few columns.
type ApplyJobUpdate struct {
	State           *string
	Phase           *string
	ErrorCode       *string
	Error           *string
	Warnings        *string
	BackupPath      *string
	ItemsInstalled  *int
	ItemsTotal      *int
	CancelRequested *bool
	StartedAt       *time.Time
	FinishedAt      *time.Time
}

func (u ApplyJobUpdate) columns() map[string]any {
	cols := map[string]any{}
	put := func(name string, v any, ok bool) {
		if ok {
			cols[name] = v
		}
	}
	var errText string
	if u.Error != nil {
		errText = truncateRunes(*u.Error, models.CrowdsecApplyMaxErrorLength)
	}
	put("state", deref(u.State), u.State != nil)
	put("phase", deref(u.Phase), u.Phase != nil)
	put("error_code", deref(u.ErrorCode), u.ErrorCode != nil)
	put("error", errText, u.Error != nil)
	put("warnings", deref(u.Warnings), u.Warnings != nil)
	put("backup_path", deref(u.BackupPath), u.BackupPath != nil)
	put("items_installed", deref(u.ItemsInstalled), u.ItemsInstalled != nil)
	put("items_total", deref(u.ItemsTotal), u.ItemsTotal != nil)
	put("cancel_requested", deref(u.CancelRequested), u.CancelRequested != nil)
	put("started_at", deref(u.StartedAt), u.StartedAt != nil)
	put("finished_at", deref(u.FinishedAt), u.FinishedAt != nil)
	return cols
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// Create inserts job, generating its id, and defaults the state to queued.
func (r *ApplyJobRepository) Create(ctx context.Context, job *models.CrowdsecApplyJob) error {
	if job.State == "" {
		job.State = models.CrowdsecApplyJobQueued
	}
	job.Error = truncateRunes(job.Error, models.CrowdsecApplyMaxErrorLength)
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("create crowdsec apply job: %w", err)
	}
	return nil
}

// Update applies a partial update and refreshes updated_at.
func (r *ApplyJobRepository) Update(ctx context.Context, id string, upd ApplyJobUpdate) error {
	cols := upd.columns()
	if len(cols) == 0 {
		return nil
	}
	cols["updated_at"] = r.now()
	res := r.db.WithContext(ctx).Model(&models.CrowdsecApplyJob{}).Where("id = ?", id).Updates(cols)
	if res.Error != nil {
		return fmt.Errorf("update crowdsec apply job: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrApplyJobNotFound
	}
	return nil
}

// Get returns the job with id, or ErrApplyJobNotFound.
func (r *ApplyJobRepository) Get(ctx context.Context, id string) (*models.CrowdsecApplyJob, error) {
	var job models.CrowdsecApplyJob
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplyJobNotFound
		}
		return nil, fmt.Errorf("get crowdsec apply job: %w", err)
	}
	return &job, nil
}

var activeApplyStates = []string{models.CrowdsecApplyJobQueued, models.CrowdsecApplyJobRunning, models.CrowdsecApplyJobVerifying}

// Current returns the newest active job, or ErrApplyJobNotFound when none is active.
func (r *ApplyJobRepository) Current(ctx context.Context) (*models.CrowdsecApplyJob, error) {
	var job models.CrowdsecApplyJob
	err := r.db.WithContext(ctx).Where("state IN ?", activeApplyStates).Order("created_at DESC").First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplyJobNotFound
		}
		return nil, fmt.Errorf("get current crowdsec apply job: %w", err)
	}
	return &job, nil
}

// ListRecent returns up to limit jobs, newest first.
func (r *ApplyJobRepository) ListRecent(ctx context.Context, limit int) ([]models.CrowdsecApplyJob, error) {
	if limit <= 0 {
		limit = ApplyJobKeepFinished
	}
	var jobs []models.CrowdsecApplyJob
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("list crowdsec apply jobs: %w", err)
	}
	return jobs, nil
}

// ActiveJobs returns every job still in an active state, oldest first.
func (r *ApplyJobRepository) ActiveJobs(ctx context.Context) ([]models.CrowdsecApplyJob, error) {
	var jobs []models.CrowdsecApplyJob
	if err := r.db.WithContext(ctx).Where("state IN ?", activeApplyStates).Order("created_at ASC").Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("find active crowdsec apply jobs: %w", err)
	}
	return jobs, nil
}

// MarkInterrupted fails the still-active job id with the interrupted code (the process that ran it is gone),
// storing warnings (may be empty) in the same write. A job that is no longer active is left alone.
func (r *ApplyJobRepository) MarkInterrupted(ctx context.Context, id, warnings string) error {
	now := r.now()
	cols := map[string]any{
		"state":       models.CrowdsecApplyJobFailed,
		"error_code":  models.CrowdsecApplyErrInterrupted,
		"finished_at": now,
		"updated_at":  now,
	}
	if warnings != "" {
		cols["warnings"] = warnings
	}
	err := r.db.WithContext(ctx).Model(&models.CrowdsecApplyJob{}).
		Where("id = ? AND state IN ?", id, activeApplyStates).Updates(cols).Error
	if err != nil {
		return fmt.Errorf("mark crowdsec apply job interrupted: %w", err)
	}
	return nil
}

// Prune deletes finished jobs beyond the newest keepFinished that are also older than maxAge. Active jobs
// are never deleted. It returns the number of rows removed.
func (r *ApplyJobRepository) Prune(ctx context.Context, keepFinished int, maxAge time.Duration) (int64, error) {
	if keepFinished < 0 {
		keepFinished = 0
	}
	cutoff := r.now().Add(-maxAge)
	keep := r.db.WithContext(ctx).Model(&models.CrowdsecApplyJob{}).Select("id").
		Where("state NOT IN ?", activeApplyStates).Order("created_at DESC").Limit(keepFinished)
	res := r.db.WithContext(ctx).
		Where("state NOT IN ?", activeApplyStates).
		Where("COALESCE(finished_at, updated_at) < ?", cutoff).
		Where("id NOT IN (?)", keep).
		Delete(&models.CrowdsecApplyJob{})
	if res.Error != nil {
		return 0, fmt.Errorf("prune crowdsec apply jobs: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// EncodeApplyWarnings serialises warnings as a JSON array capped at models.CrowdsecApplyMaxWarnings
// entries; the last entry says how many were dropped.
func EncodeApplyWarnings(warnings []string) (string, error) {
	if len(warnings) == 0 {
		return "", nil
	}
	out := warnings
	if len(warnings) > models.CrowdsecApplyMaxWarnings {
		out = append(append([]string{}, warnings[:models.CrowdsecApplyMaxWarnings]...),
			fmt.Sprintf("and %d more", len(warnings)-models.CrowdsecApplyMaxWarnings))
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("encode apply warnings: %w", err)
	}
	return string(b), nil
}

func truncateRunes(s string, limit int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > limit {
		return string(r[:limit])
	}
	return s
}
