package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
)

// useJobApplyWithHistory is useJobApply plus the production OnFinish hook that writes the preset history.
func useJobApplyWithHistory(t *testing.T, f *busyFixture, apply crowdsec.ApplyFunc, maxDuration time.Duration) {
	t.Helper()
	require.NoError(t, f.h.DB.AutoMigrate(&models.CrowdsecPresetEvent{}))
	f.h.ApplyJobs().Configure(crowdsec.ApplyJobConfig{
		Repo: crowdsec.NewApplyJobRepository(f.h.DB), Apply: apply, MaxDuration: maxDuration, OnFinish: f.h.recordApplyJobEvent,
	})
	f.h.applyJobsCfgMu.Lock()
	f.h.applyJobsHub = f.h.Hub
	f.h.applyJobsCfgMu.Unlock()
}

func startHistoryJob(t *testing.T, f *busyFixture) string {
	t.Helper()
	w := f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json")
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	return decodeBody(t, w)["job_id"].(string)
}

// waitHistoryEvent returns the only preset history row once the finished job has written it.
func waitHistoryEvent(t *testing.T, f *busyFixture) models.CrowdsecPresetEvent {
	t.Helper()
	var events []models.CrowdsecPresetEvent
	require.Eventually(t, func() bool {
		events = nil
		return f.h.DB.Find(&events).Error == nil && len(events) == 1
	}, 10*time.Second, 20*time.Millisecond)
	return events[0]
}

func TestAsyncApplySuccessIsRecordedInThePresetHistory(t *testing.T) {
	f := newBusyFixture(t)
	useJobApplyWithHistory(t, f, func(context.Context, crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		return crowdsec.ApplyResult{Status: "applied", BackupPath: "/data/crowdsec.backup.1"}, nil
	}, 0)
	startHistoryJob(t, f)

	ev := waitHistoryEvent(t, f)
	require.Equal(t, "crowdsecurity/sshd", ev.Slug)
	require.Equal(t, "apply", ev.Action)
	require.Equal(t, "applied", ev.Status)
	require.Equal(t, "k", ev.CacheKey)
	require.Equal(t, "/data/crowdsec.backup.1", ev.BackupPath)
	require.Empty(t, ev.Error)
}

func TestAsyncApplyFailureIsRecordedInThePresetHistory(t *testing.T) {
	f := newBusyFixture(t)
	useJobApplyWithHistory(t, f, func(context.Context, crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		return crowdsec.ApplyResult{Status: "failed", BackupPath: "/data/crowdsec.backup.2"}, errors.New("cscli exploded")
	}, 0)
	startHistoryJob(t, f)

	ev := waitHistoryEvent(t, f)
	require.Equal(t, "failed", ev.Status)
	require.Contains(t, ev.Error, "cscli exploded")
	require.Equal(t, "/data/crowdsec.backup.2", ev.BackupPath)
	require.Equal(t, "k", ev.CacheKey)
}

func TestAsyncApplyCancelIsRecordedInThePresetHistory(t *testing.T) {
	f := newBusyFixture(t)
	started := make(chan struct{})
	useJobApplyWithHistory(t, f, func(ctx context.Context, _ crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		close(started)
		<-ctx.Done()
		return crowdsec.ApplyResult{Status: "failed"}, crowdsec.ErrPresetApplyCancelled
	}, 0)
	id := startHistoryJob(t, f)
	<-started
	require.Equal(t, http.StatusAccepted, f.do(http.MethodDelete, applyJobsBase+"/"+id, nil, "").Code)

	ev := waitHistoryEvent(t, f)
	require.Equal(t, "failed", ev.Status)
	require.Contains(t, ev.Error, "cancelled")
}

func TestAsyncApplyTimeoutIsRecordedInThePresetHistory(t *testing.T) {
	f := newBusyFixture(t)
	useJobApplyWithHistory(t, f, func(ctx context.Context, _ crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		<-ctx.Done()
		return crowdsec.ApplyResult{Status: "failed"}, crowdsec.ErrPresetApplyTimeout
	}, 50*time.Millisecond)
	id := startHistoryJob(t, f)

	waitJobState(t, f, id, models.CrowdsecApplyJobTimedOut)
	ev := waitHistoryEvent(t, f)
	require.Equal(t, "failed", ev.Status)
	require.NotEmpty(t, ev.Error)
}

func TestAsyncApplyWithTheDefaultWiringRecordsTheHistory(t *testing.T) {
	f := newBusyFixture(t)
	require.NoError(t, f.h.DB.AutoMigrate(&models.CrowdsecPresetEvent{}))
	// The real hub service with an empty cache fails with a stale-cache error.
	id := startHistoryJob(t, f)
	waitJobState(t, f, id, models.CrowdsecApplyJobFailed)

	ev := waitHistoryEvent(t, f)
	require.Equal(t, "failed", ev.Status)
	require.Equal(t, "k", ev.CacheKey)
}

func TestRecordApplyJobEventFallsBackToTheStateWithoutAnErrorText(t *testing.T) {
	f := newBusyFixture(t)
	require.NoError(t, f.h.DB.AutoMigrate(&models.CrowdsecPresetEvent{}))
	f.h.recordApplyJobEvent(models.CrowdsecApplyJob{Slug: "s", State: models.CrowdsecApplyJobCancelled})

	ev := waitHistoryEvent(t, f)
	require.Equal(t, "failed", ev.Status)
	require.Equal(t, "install "+models.CrowdsecApplyJobCancelled, ev.Error)
}
