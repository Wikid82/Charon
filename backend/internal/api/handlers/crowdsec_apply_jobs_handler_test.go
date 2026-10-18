package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
)

const applyJobsBase = "/admin/crowdsec/presets/jobs"

var hubPresetBody = []byte(`{"slug":"crowdsecurity/sshd","type":"collections","cache_key":"k","async":true}`)

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m), w.Body.String())
	return m
}

// useJobApply replaces the apply the manager runs; it also pins the wiring so the lazy setup does not undo it.
func useJobApply(t *testing.T, f *busyFixture, apply crowdsec.ApplyFunc) {
	t.Helper()
	m := f.h.ApplyJobs()
	m.Configure(crowdsec.ApplyJobConfig{Repo: crowdsec.NewApplyJobRepository(f.h.DB), Apply: apply})
	f.h.applyJobsCfgMu.Lock()
	f.h.applyJobsHub = f.h.Hub
	f.h.applyJobsCfgMu.Unlock()
}

func waitJobState(t *testing.T, f *busyFixture, id string, want ...string) map[string]any {
	t.Helper()
	var body map[string]any
	require.Eventually(t, func() bool {
		body = decodeBody(t, f.do(http.MethodGet, applyJobsBase+"/"+id, nil, ""))
		for _, s := range want {
			if body["state"] == s {
				return true
			}
		}
		return false
	}, 10*time.Second, 20*time.Millisecond)
	return body
}

func TestAsyncApplyStartsAJobAndAnswers202(t *testing.T) {
	f := newBusyFixture(t)
	release := make(chan struct{})
	useJobApply(t, f, func(ctx context.Context, req crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		<-release
		return crowdsec.ApplyResult{Status: "applied", BackupPath: "/secret/backup/path"}, nil
	})

	w := f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json")
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	body := decodeBody(t, w)
	id, _ := body["job_id"].(string)
	_, err := uuid.Parse(id)
	require.NoError(t, err)
	require.Equal(t, models.CrowdsecApplyJobQueued, body["state"])
	require.Contains(t, body, "phase")
	require.Equal(t, "crowdsecurity/sshd", body["slug"])

	// While it runs: a second start is busy and names the job, and writes are refused.
	w = f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json")
	require.Equal(t, http.StatusConflict, w.Code)
	resp := decodeBody(t, w)
	require.Equal(t, codeCrowdsecBusy, resp["error_code"])
	require.Equal(t, id, resp["job_id"])

	close(release)
	done := waitJobState(t, f, id, models.CrowdsecApplyJobSucceeded)
	require.NotNil(t, done["finished_at"])
	raw := f.do(http.MethodGet, applyJobsBase+"/"+id, nil, "").Body.String()
	require.NotContains(t, raw, "backup_path")
	require.NotContains(t, raw, "/secret/backup/path")
	require.Equal(t, []any{}, done["warnings"])
}

func TestAsyncApplyFailureCarriesTheSharedErrorCode(t *testing.T) {
	f := newBusyFixture(t)
	// The real hub service with an empty cache fails fast with a stale-cache error.
	w := f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json")
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	id := decodeBody(t, w)["job_id"].(string)

	done := waitJobState(t, f, id, models.CrowdsecApplyJobFailed)
	require.Equal(t, codePresetCacheStale, done["error_code"])
	require.Eventually(t, func() bool { _, busy := f.h.ApplyJobs().Busy(); return !busy }, 5*time.Second, 10*time.Millisecond)
}

func TestAsyncApplyValidatesLikeTheSyncPath(t *testing.T) {
	f := newBusyFixture(t)
	for name, body := range map[string]string{
		"missing slug":      `{"type":"collections","cache_key":"k","async":true}`,
		"missing cache key": `{"slug":"crowdsecurity/sshd","type":"collections","async":true}`,
		"bad type":          `{"slug":"crowdsecurity/sshd","type":"nonsense","cache_key":"k","async":true}`,
		"bad slug":          `{"slug":"../etc","type":"collections","cache_key":"k","async":true}`,
		"bad json":          `{`,
	} {
		w := f.do(http.MethodPost, "/admin/crowdsec/presets/apply", []byte(body), "application/json")
		require.Equal(t, http.StatusBadRequest, w.Code, name+": "+w.Body.String())
	}
	_, busy := f.h.ApplyJobs().Busy()
	require.False(t, busy, "a rejected request starts nothing")
}

func TestAsyncApplyOfACuratedPresetStaysSynchronous(t *testing.T) {
	f := newBusyFixture(t)
	curated := curatedPresetSlug()
	require.NotEmpty(t, curated)
	w := f.do(http.MethodPost, "/admin/crowdsec/presets/apply",
		[]byte(`{"slug":"`+curated+`","cache_key":"curated-`+curated+`","async":true}`), "application/json")
	require.NotEqual(t, http.StatusAccepted, w.Code, "curated presets never become jobs: %s", w.Body.String())
	var n int64
	require.NoError(t, f.h.DB.Model(&models.CrowdsecApplyJob{}).Count(&n).Error)
	require.Zero(t, n)
}

func TestSyncApplyIsUnchangedWithoutAsync(t *testing.T) {
	f := newBusyFixture(t)
	w := f.do(http.MethodPost, "/admin/crowdsec/presets/apply",
		[]byte(`{"slug":"crowdsecurity/sshd","type":"collections","cache_key":"k"}`), "application/json")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Equal(t, codePresetCacheStale, decodeBody(t, w)["error_code"])
	var n int64
	require.NoError(t, f.h.DB.Model(&models.CrowdsecApplyJob{}).Count(&n).Error)
	require.Zero(t, n, "the synchronous path creates no job")
}

func TestAsyncApplyWithoutHubOrDBAnswers503(t *testing.T) {
	f := newBusyFixture(t)
	f.h.Hub = nil
	w := f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	for _, path := range []string{applyJobsBase + "/current", applyJobsBase, applyJobsBase + "/" + busyJobID} {
		w = f.do(http.MethodGet, path, nil, "")
		require.Equal(t, http.StatusServiceUnavailable, w.Code, path)
		require.Equal(t, codeCrowdsecUnavailable, decodeBody(t, w)["error_code"])
	}
}

func TestAsyncApplyFailsCleanlyWhenTheJobTableIsMissing(t *testing.T) {
	f := newBusyFixture(t)
	require.NoError(t, f.h.DB.Migrator().DropTable(&models.CrowdsecApplyJob{}))
	w := f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	_, busy := f.h.ApplyJobs().Busy()
	require.False(t, busy, "the flag is released when the job row cannot be stored")
}

func TestCurrentJobEndpoint(t *testing.T) {
	f := newBusyFixture(t)
	w := f.do(http.MethodGet, applyJobsBase+"/current", nil, "")
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeBody(t, w)
	require.Contains(t, body, "job")
	require.Nil(t, body["job"])

	release := make(chan struct{})
	useJobApply(t, f, func(ctx context.Context, req crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		<-release
		return crowdsec.ApplyResult{Status: "applied"}, nil
	})
	id := decodeBody(t, f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json"))["job_id"]
	job := decodeBody(t, f.do(http.MethodGet, applyJobsBase+"/current", nil, ""))["job"].(map[string]any)
	require.Equal(t, id, job["job_id"])

	close(release)
	waitJobState(t, f, id.(string), models.CrowdsecApplyJobSucceeded)
	job = decodeBody(t, f.do(http.MethodGet, applyJobsBase+"/current", nil, ""))["job"].(map[string]any)
	require.Equal(t, models.CrowdsecApplyJobSucceeded, job["state"], "a job that just finished is still reported")

	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, f.h.DB.Model(&models.CrowdsecApplyJob{}).Where("id = ?", id).Update("finished_at", old).Error)
	require.Nil(t, decodeBody(t, f.do(http.MethodGet, applyJobsBase+"/current", nil, ""))["job"], "an old result is not resumed")
}

func TestGetJobEndpointValidatesTheID(t *testing.T) {
	f := newBusyFixture(t)
	w := f.do(http.MethodGet, applyJobsBase+"/not-a-uuid", nil, "")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, codeInvalidJobID, decodeBody(t, w)["error_code"])

	w = f.do(http.MethodGet, applyJobsBase+"/"+uuid.NewString(), nil, "")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Equal(t, codeJobNotFound, decodeBody(t, w)["error_code"])

	w = f.do(http.MethodDelete, applyJobsBase+"/not-a-uuid", nil, "")
	require.Equal(t, http.StatusBadRequest, w.Code)
	w = f.do(http.MethodDelete, applyJobsBase+"/"+uuid.NewString(), nil, "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestJobEndpointsStayOpenWhileAJobRuns(t *testing.T) {
	f := newBusyFixture(t)
	release := make(chan struct{})
	useJobApply(t, f, func(ctx context.Context, req crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		<-release
		return crowdsec.ApplyResult{Status: "applied"}, nil
	})
	id := decodeBody(t, f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json"))["job_id"].(string)
	for _, p := range []string{"/current", "", "/" + id} {
		require.Equal(t, http.StatusOK, f.do(http.MethodGet, applyJobsBase+p, nil, "").Code, p)
	}
	close(release)
	waitJobState(t, f, id, models.CrowdsecApplyJobSucceeded)
}

func TestCancelJobEndpoint(t *testing.T) {
	f := newBusyFixture(t)
	started := make(chan struct{})
	useJobApply(t, f, func(ctx context.Context, req crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		close(started)
		<-ctx.Done()
		return crowdsec.ApplyResult{Status: "failed"}, crowdsec.ErrPresetApplyCancelled
	})
	id := decodeBody(t, f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json"))["job_id"].(string)
	<-started

	w := f.do(http.MethodDelete, applyJobsBase+"/"+id, nil, "")
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	require.Equal(t, id, decodeBody(t, w)["job_id"])

	waitJobState(t, f, id, models.CrowdsecApplyJobCancelled)
	w = f.do(http.MethodDelete, applyJobsBase+"/"+id, nil, "")
	require.Equal(t, http.StatusConflict, w.Code)
	body := decodeBody(t, w)
	require.Equal(t, codeJobNotActive, body["error_code"])
	require.Equal(t, models.CrowdsecApplyJobCancelled, body["state"])
}

func TestListJobsEndpointLimits(t *testing.T) {
	f := newBusyFixture(t)
	repo := crowdsec.NewApplyJobRepository(f.h.DB)
	base := time.Now().Add(-time.Hour)
	for i := range 60 {
		job := &models.CrowdsecApplyJob{ID: uuid.NewString(), Slug: "a/b", State: models.CrowdsecApplyJobSucceeded, CreatedAt: base.Add(time.Duration(i) * time.Second)}
		require.NoError(t, repo.Create(context.Background(), job))
	}
	count := func(query string) int {
		w := f.do(http.MethodGet, applyJobsBase+query, nil, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		return len(decodeBody(t, w)["jobs"].([]any))
	}
	require.Equal(t, 20, count(""))
	require.Equal(t, 5, count("?limit=5"))
	require.Equal(t, 50, count("?limit=500"))
	for _, bad := range []string{"?limit=0", "?limit=-1", "?limit=abc"} {
		w := f.do(http.MethodGet, applyJobsBase+bad, nil, "")
		require.Equal(t, http.StatusBadRequest, w.Code, bad)
		require.Equal(t, codeInvalidLimit, decodeBody(t, w)["error_code"])
	}
}

func TestJobViewMapsWarningsAndElapsed(t *testing.T) {
	now := time.Now().UTC()
	started := now.Add(-90 * time.Second)
	job := &models.CrowdsecApplyJob{ID: "x", State: models.CrowdsecApplyJobRunning, Warnings: `["a","b"]`, BackupPath: "/p", StartedAt: &started}
	v := newApplyJobView(job, now)
	require.Equal(t, []string{"a", "b"}, v.Warnings)
	require.EqualValues(t, 90, v.ElapsedSeconds)

	job.Warnings = "not json"
	job.StartedAt = nil
	job.CreatedAt = now.Add(time.Hour)
	v = newApplyJobView(job, now)
	require.Equal(t, []string{}, v.Warnings)
	require.Zero(t, v.ElapsedSeconds, "elapsed never goes negative")
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	require.False(t, strings.Contains(string(raw), "backup"))
}

func TestShutdownApplyJobsCancelsTheRunningJobAndRollsBack(t *testing.T) {
	f := newBusyFixture(t)
	started := make(chan struct{})
	useJobApply(t, f, func(ctx context.Context, req crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		close(started)
		<-ctx.Done()
		return crowdsec.ApplyResult{Status: "failed"}, errors.Join(crowdsec.ErrPresetApplyCancelled, ctx.Err())
	})
	id := decodeBody(t, f.do(http.MethodPost, "/admin/crowdsec/presets/apply", hubPresetBody, "application/json"))["job_id"].(string)
	<-started

	f.h.ShutdownApplyJobs()

	job, err := crowdsec.NewApplyJobRepository(f.h.DB).Get(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, models.CrowdsecApplyJobCancelled, job.State)
}
