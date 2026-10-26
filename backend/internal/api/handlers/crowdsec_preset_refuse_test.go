package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// blockDataLock holds dataMu for the test and returns an idempotent release. The lock is also released at cleanup,
// so a request that waits on it cannot leak past the test.
func blockDataLock(t *testing.T, h *CrowdsecHandler) func() {
	t.Helper()
	require.NoError(t, h.dataMu.LockContext(context.Background()))
	var once sync.Once
	release := func() { once.Do(h.dataMu.Unlock) }
	t.Cleanup(release)
	return release
}

// postPresetAsync runs the preset apply in a goroutine and returns its recorder channel, so a request that waits on
// dataMu shows up as a timeout in the test instead of hanging it.
func postPresetAsync(f *busyFixture, body []byte) <-chan *httptest.ResponseRecorder {
	ch := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		ch <- f.do(http.MethodPost, "/admin/crowdsec/presets/apply", body, "application/json")
	}()
	return ch
}

// TestAsyncPresetApplyRefusesAtOnceWhileAHoldIsOut pins T2: a hold (a request or a hub item restore) is out and
// dataMu is busy, so an async non-curated apply must answer 409 without waiting for dataMu.
func TestAsyncPresetApplyRefusesAtOnceWhileAHoldIsOut(t *testing.T) {
	f := newBusyFixture(t)
	releaseData := blockDataLock(t, f.h)
	releaseHold, _, ok := f.h.ApplyJobs().Hold()
	require.True(t, ok)
	t.Cleanup(releaseHold)

	var w *httptest.ResponseRecorder
	select {
	case w = <-postPresetAsync(f, f.presetBody):
	case <-time.After(3 * time.Second):
		t.Fatal("the async apply waited for dataMu instead of refusing while a hold is out")
	}
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Equal(t, codeCrowdsecBusy, decodeBody(t, w)["error_code"])

	var n int64
	require.NoError(t, f.h.DB.Model(&models.CrowdsecApplyJob{}).Count(&n).Error)
	require.Zero(t, n, "a refused apply records no job")

	releaseData()
	releaseHold()
	_, busy := f.h.ApplyJobs().Busy()
	require.False(t, busy)
}

// TestCuratedPresetApplyWhileAHoldIsOutStaysSynchronous pins the unchanged curated path (m6): an async request for a
// curated preset is not refused by a hold. It waits for dataMu and applies synchronously, and creates no job.
func TestCuratedPresetApplyWhileAHoldIsOutStaysSynchronous(t *testing.T) {
	f := newBusyFixture(t)
	curated := curatedPresetSlug()
	require.NotEmpty(t, curated)
	releaseData := blockDataLock(t, f.h)
	releaseHold, _, ok := f.h.ApplyJobs().Hold()
	require.True(t, ok)
	t.Cleanup(releaseHold)

	body := []byte(`{"slug":"` + curated + `","cache_key":"curated-` + curated + `","async":true}`)
	done := postPresetAsync(f, body)
	select {
	case <-done:
		t.Fatal("the curated apply answered before dataMu was released")
	case <-time.After(200 * time.Millisecond):
	}

	releaseData()
	var w *httptest.ResponseRecorder
	select {
	case w = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the curated apply did not finish after dataMu was released")
	}
	require.NotEqual(t, http.StatusAccepted, w.Code, "curated presets never become jobs: %s", w.Body.String())
	require.NotContains(t, w.Body.String(), codeCrowdsecBusy, "a hold does not refuse a curated apply")

	var n int64
	require.NoError(t, f.h.DB.Model(&models.CrowdsecApplyJob{}).Count(&n).Error)
	require.Zero(t, n)
}
