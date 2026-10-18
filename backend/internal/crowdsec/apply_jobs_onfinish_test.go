package crowdsec

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

func TestApplyJobOnFinishReceivesTheStoredFinalJob(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecApplyJob{}))

	got := make(chan models.CrowdsecApplyJob, 1)
	mgr := NewApplyJobManager(nil)
	mgr.Configure(ApplyJobConfig{
		Repo:  NewApplyJobRepository(db),
		Apply: func(context.Context, ApplyRequest) (ApplyResult, error) { return ApplyResult{}, errors.New("boom") },
		OnFinish: func(job models.CrowdsecApplyJob) {
			_, busy := mgr.Busy()
			require.True(t, busy, "the flag is still held while the hook runs")
			got <- job
		},
	})
	t.Cleanup(mgr.Shutdown)
	started, err := mgr.Start(ApplyJobRequest{Slug: "crowdsecurity/sshd", Type: "collections", CacheKey: "k"})
	require.NoError(t, err)

	select {
	case job := <-got:
		require.Equal(t, started.ID, job.ID)
		require.Equal(t, models.CrowdsecApplyJobFailed, job.State)
		require.NotNil(t, job.FinishedAt)
	case <-time.After(10 * time.Second):
		t.Fatal("OnFinish was not called")
	}
}

func TestApplyJobOnFinishPanicDoesNotBreakTheJob(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecApplyJob{}))

	mgr := NewApplyJobManager(nil)
	mgr.Configure(ApplyJobConfig{
		Repo:     NewApplyJobRepository(db),
		Apply:    func(context.Context, ApplyRequest) (ApplyResult, error) { return ApplyResult{Status: "applied"}, nil },
		OnFinish: func(models.CrowdsecApplyJob) { panic("hook") },
	})
	t.Cleanup(mgr.Shutdown)
	job, err := mgr.Start(ApplyJobRequest{Slug: "s", CacheKey: "k"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		stored, getErr := mgr.Get(context.Background(), job.ID)
		_, busy := mgr.Busy()
		return getErr == nil && stored.State == models.CrowdsecApplyJobSucceeded && !busy
	}, 10*time.Second, 10*time.Millisecond)
}
