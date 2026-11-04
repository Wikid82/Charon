package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/stretchr/testify/require"
)

// TestApplyFlavoursRefuseAtOnceWhileARestoreRuns pins #1796: while a hub restore runs, every preset apply flavour
// answers 409 crowdsec_busy within 100 ms instead of waiting for dataMu, which the restore holds for its hub update.
func TestApplyFlavoursRefuseAtOnceWhileARestoreRuns(t *testing.T) {
	f := newBusyFixture(t)
	curated := curatedPresetSlug()
	require.NotEmpty(t, curated)

	syncBody, err := json.Marshal(map[string]any{"slug": "crowdsecurity/sshd", "type": "collections", "cache_key": f.cacheKey})
	require.NoError(t, err)
	curatedBody, err := json.Marshal(map[string]any{"slug": curated, "cache_key": "curated-" + curated})
	require.NoError(t, err)

	flavours := map[string][]byte{
		"curated synchronous":      curatedBody,
		"non-curated synchronous":  syncBody,
		"non-curated asynchronous": f.presetBody,
	}

	g := gateHeldRoutes(t, f)
	restoreDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		restoreDone <- f.do(http.MethodPost, "/admin/crowdsec/hub-items/restore",
			restoreBody(crowdsec.HubItem{Type: "scenarios", Name: "a/one"}), "application/json")
	}()
	select {
	case <-g.entered:
	case <-time.After(holdWaitForRequest):
		require.FailNow(t, "the restore never started")
	}
	t.Cleanup(g.open)

	for name, body := range flavours {
		t.Run(name, func(t *testing.T) {
			ch := make(chan *httptest.ResponseRecorder, 1)
			go func() { ch <- f.do(http.MethodPost, "/admin/crowdsec/presets/apply", body, "application/json") }()

			var w *httptest.ResponseRecorder
			select {
			case w = <-ch:
			case <-time.After(100 * time.Millisecond):
				require.FailNow(t, "the apply waited instead of refusing while a restore runs")
			}
			require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
			require.Equal(t, codeCrowdsecBusy, decodeBody(t, w)["error_code"])
			require.Equal(t, msgCrowdsecBusyOther, decodeBody(t, w)["error"])
		})
	}

	g.open()
	<-restoreDone
}
