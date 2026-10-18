package crowdsec

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

func newJobRepo(t *testing.T) (*ApplyJobRepository, *gorm.DB, *time.Time) {
	t.Helper()
	db := testutil.OpenMemoryDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecApplyJob{}))
	repo := NewApplyJobRepository(db)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	repo.now = func() time.Time { return now }
	return repo, db, &now
}

func seedJob(t *testing.T, repo *ApplyJobRepository, state string, created, finished time.Time) *models.CrowdsecApplyJob {
	t.Helper()
	job := &models.CrowdsecApplyJob{Slug: "a/b", ItemType: "collections", CacheKey: "k", State: state, CreatedAt: created}
	if !finished.IsZero() {
		job.FinishedAt = &finished
		job.UpdatedAt = finished
	}
	require.NoError(t, repo.Create(context.Background(), job))
	return job
}

func TestApplyJobRepoCreateGetUpdate(t *testing.T) {
	ctx := context.Background()
	repo, _, now := newJobRepo(t)

	job := &models.CrowdsecApplyJob{Slug: "crowdsecurity/linux", ItemType: "collections", CacheKey: "k1"}
	require.NoError(t, repo.Create(ctx, job))
	require.NotEmpty(t, job.ID)
	require.Equal(t, models.CrowdsecApplyJobQueued, job.State)

	got, err := repo.Get(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, "crowdsecurity/linux", got.Slug)

	_, err = repo.Get(ctx, "missing")
	require.ErrorIs(t, err, ErrApplyJobNotFound)

	state, phase, n, total, cancel := models.CrowdsecApplyJobRunning, "install", 7, 12, true
	errText := fmt.Sprintf("%0800d", 1)
	started := now.Add(time.Second)
	require.NoError(t, repo.Update(ctx, job.ID, ApplyJobUpdate{
		State: &state, Phase: &phase, ItemsInstalled: &n, ItemsTotal: &total, CancelRequested: &cancel,
		Error: &errText, StartedAt: &started,
	}))
	got, err = repo.Get(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, state, got.State)
	require.Equal(t, "install", got.Phase)
	require.Equal(t, 7, got.ItemsInstalled)
	require.NotNil(t, got.ItemsTotal)
	require.Equal(t, 12, *got.ItemsTotal)
	require.True(t, got.CancelRequested)
	require.Len(t, got.Error, models.CrowdsecApplyMaxErrorLength)
	require.NotNil(t, got.StartedAt)
	require.Equal(t, "k1", got.CacheKey, "untouched columns stay")

	// partial update leaves other columns alone
	n2 := 9
	require.NoError(t, repo.Update(ctx, job.ID, ApplyJobUpdate{ItemsInstalled: &n2}))
	got, err = repo.Get(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, 9, got.ItemsInstalled)
	require.Equal(t, "install", got.Phase)

	require.NoError(t, repo.Update(ctx, job.ID, ApplyJobUpdate{}))
	require.ErrorIs(t, repo.Update(ctx, "missing", ApplyJobUpdate{Phase: &phase}), ErrApplyJobNotFound)
}

func TestApplyJobRepoCurrentAndList(t *testing.T) {
	ctx := context.Background()
	repo, _, now := newJobRepo(t)

	_, err := repo.Current(ctx)
	require.ErrorIs(t, err, ErrApplyJobNotFound)

	old := seedJob(t, repo, models.CrowdsecApplyJobSucceeded, now.Add(-2*time.Hour), now.Add(-time.Hour))
	active := seedJob(t, repo, models.CrowdsecApplyJobVerifying, now.Add(-time.Minute), time.Time{})
	cur, err := repo.Current(ctx)
	require.NoError(t, err)
	require.Equal(t, active.ID, cur.ID)

	list, err := repo.ListRecent(ctx, 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, active.ID, list[0].ID)
	require.Equal(t, old.ID, list[1].ID)

	list, err = repo.ListRecent(ctx, 1)
	require.NoError(t, err)
	require.Len(t, list, 1)
	list, err = repo.ListRecent(ctx, 0)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestApplyJobRepoActiveJobsAndMarkInterrupted(t *testing.T) {
	ctx := context.Background()
	repo, _, now := newJobRepo(t)

	none, err := repo.ActiveJobs(ctx)
	require.NoError(t, err)
	require.Empty(t, none)

	done := seedJob(t, repo, models.CrowdsecApplyJobSucceeded, now.Add(-time.Hour), now.Add(-time.Hour))
	q := seedJob(t, repo, models.CrowdsecApplyJobQueued, now.Add(-3*time.Minute), time.Time{})
	r := seedJob(t, repo, models.CrowdsecApplyJobRunning, now.Add(-2*time.Minute), time.Time{})
	v := seedJob(t, repo, models.CrowdsecApplyJobVerifying, now.Add(-time.Minute), time.Time{})
	backup := "/data/backup.tar.gz"
	require.NoError(t, repo.Update(ctx, r.ID, ApplyJobUpdate{BackupPath: &backup}))

	active, err := repo.ActiveJobs(ctx)
	require.NoError(t, err)
	require.Len(t, active, 3)
	for _, a := range active {
		if a.ID == r.ID {
			require.Equal(t, backup, a.BackupPath, "caller gets the backup path to restore")
		}
		require.NotEqual(t, done.ID, a.ID)
	}
	// Listing marks nothing: a crash before MarkInterrupted must leave the jobs active for the next start.
	_, err = repo.Current(ctx)
	require.NoError(t, err)

	warnings := `["w"]`
	require.NoError(t, repo.MarkInterrupted(ctx, r.ID, warnings))
	require.NoError(t, repo.MarkInterrupted(ctx, q.ID, ""))
	require.NoError(t, repo.MarkInterrupted(ctx, v.ID, ""))
	require.NoError(t, repo.MarkInterrupted(ctx, done.ID, `["x"]`), "a finished job is left alone")
	for _, id := range []string{q.ID, r.ID, v.ID} {
		got, gerr := repo.Get(ctx, id)
		require.NoError(t, gerr)
		require.Equal(t, models.CrowdsecApplyJobFailed, got.State)
		require.Equal(t, models.CrowdsecApplyErrInterrupted, got.ErrorCode)
		require.NotNil(t, got.FinishedAt)
	}
	got, err := repo.Get(ctx, r.ID)
	require.NoError(t, err)
	require.Equal(t, warnings, got.Warnings)
	got, err = repo.Get(ctx, done.ID)
	require.NoError(t, err)
	require.Equal(t, models.CrowdsecApplyJobSucceeded, got.State)
	require.Empty(t, got.Warnings)
	_, err = repo.Current(ctx)
	require.ErrorIs(t, err, ErrApplyJobNotFound)
}

func TestApplyJobRepoPrune(t *testing.T) {
	ctx := context.Background()
	repo, _, now := newJobRepo(t)

	active := seedJob(t, repo, models.CrowdsecApplyJobRunning, now.Add(-30*24*time.Hour), time.Time{})
	// 25 finished jobs, all older than 7 days: only the newest 20 survive.
	var ids []string
	for i := 0; i < 25; i++ {
		at := now.Add(-time.Duration(10+i) * 24 * time.Hour)
		ids = append(ids, seedJob(t, repo, models.CrowdsecApplyJobFailed, at, at).ID)
	}
	// recent finished jobs are kept even beyond the count
	recent := seedJob(t, repo, models.CrowdsecApplyJobSucceeded, now.Add(-time.Hour), now.Add(-time.Hour))

	removed, err := repo.Prune(ctx, ApplyJobKeepFinished, ApplyJobMaxAge)
	require.NoError(t, err)
	// finished: 26 total, newest 20 kept by count (recent + 19 of the old), so 6 removed.
	require.EqualValues(t, 6, removed)

	_, err = repo.Get(ctx, active.ID)
	require.NoError(t, err, "active job is never pruned")
	_, err = repo.Get(ctx, recent.ID)
	require.NoError(t, err)
	_, err = repo.Get(ctx, ids[0])
	require.NoError(t, err)
	_, err = repo.Get(ctx, ids[24])
	require.ErrorIs(t, err, ErrApplyJobNotFound)

	// Age rule: with a tiny count, jobs newer than maxAge still survive.
	removed, err = repo.Prune(ctx, 0, ApplyJobMaxAge)
	require.NoError(t, err)
	require.EqualValues(t, 19, removed)
	_, err = repo.Get(ctx, recent.ID)
	require.NoError(t, err)
	_, err = repo.Get(ctx, active.ID)
	require.NoError(t, err)

	removed, err = repo.Prune(ctx, -1, ApplyJobMaxAge)
	require.NoError(t, err)
	require.Zero(t, removed)
}

func TestEncodeApplyWarnings(t *testing.T) {
	out, err := EncodeApplyWarnings(nil)
	require.NoError(t, err)
	require.Empty(t, out)

	out, err = EncodeApplyWarnings([]string{"a", "b"})
	require.NoError(t, err)
	require.JSONEq(t, `["a","b"]`, out)

	many := make([]string, models.CrowdsecApplyMaxWarnings+13)
	for i := range many {
		many[i] = fmt.Sprintf("w%d", i)
	}
	out, err = EncodeApplyWarnings(many)
	require.NoError(t, err)
	var decoded []string
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Len(t, decoded, models.CrowdsecApplyMaxWarnings+1)
	require.Equal(t, "and 13 more", decoded[len(decoded)-1])
}

func TestApplyJobRepoErrorsOnClosedDB(t *testing.T) {
	ctx := context.Background()
	repo, db, _ := newJobRepo(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	phase := "x"
	require.Error(t, repo.Create(ctx, &models.CrowdsecApplyJob{}))
	require.Error(t, repo.Update(ctx, "id", ApplyJobUpdate{Phase: &phase}))
	_, err = repo.Get(ctx, "id")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrApplyJobNotFound)
	_, err = repo.Current(ctx)
	require.Error(t, err)
	_, err = repo.ListRecent(ctx, 1)
	require.Error(t, err)
	_, err = repo.ActiveJobs(ctx)
	require.Error(t, err)
	err = repo.MarkInterrupted(ctx, "id", "")
	require.Error(t, err)
	_, err = repo.Prune(ctx, 1, time.Hour)
	require.Error(t, err)
}
