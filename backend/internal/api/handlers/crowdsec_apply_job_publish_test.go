package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// TestBusyResponseOmitsJobIDBeforeTheRowIsRecorded runs a preset install whose job row is still being inserted and
// asks for a Start in that window: the 409 must not name the job, because the job id is not yet readable.
func TestBusyResponseOmitsJobIDBeforeTheRowIsRecorded(t *testing.T) {
	f := newBusyFixture(t)
	entered, unblockInsert := testutil.GateCreate[models.CrowdsecApplyJob](t, f.h.DB, nil)

	applyDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		applyDone <- f.do(http.MethodPost, "/admin/crowdsec/presets/apply", f.presetBody, "application/json")
	}()
	testutil.AwaitGate(t, entered)

	w := f.do(http.MethodPost, "/admin/crowdsec/start", nil, "")
	require.Equal(t, http.StatusConflict, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "crowdsec_busy", body["error_code"])
	_, named := body["job_id"]
	require.False(t, named, "a 409 raised before the job row exists must not carry a job_id")

	unblockInsert()
	select {
	case applied := <-applyDone:
		require.Equal(t, http.StatusAccepted, applied.Code)
	case <-time.After(10 * time.Second):
		t.Fatal("the preset install request never returned")
	}
	require.Eventually(t, func() bool { _, busy := f.h.ApplyJobs().Busy(); return !busy }, 30*time.Second, 10*time.Millisecond)
}
