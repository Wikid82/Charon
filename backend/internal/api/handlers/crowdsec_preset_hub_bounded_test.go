package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// hangingHubTransport holds every hub request until its context ends, the way a dropped connection does.
func hangingHubTransport(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

// newHangingHubPresetRouter wires a handler whose hub never answers and whose overall pull budget is short.
func newHangingHubPresetRouter(t *testing.T, budget time.Duration) *gin.Engine {
	t.Helper()
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	hub := crowdsec.NewHubService(nil, cache, t.TempDir())
	hub.PullTimeout = budget
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(hangingHubTransport)}

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = hub
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return r
}

// The preset list answers inside the hub's overall budget when every hub address hangs, with the curated presets.
func TestListPresetsBoundedWhenEveryHubAddressHangs(t *testing.T) {
	const budget = 6 * time.Second
	r := newHangingHubPresetRouter(t, budget)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets", http.NoBody)
	start := time.Now()
	r.ServeHTTP(w, req)
	elapsed := time.Since(start)

	require.Equal(t, http.StatusOK, w.Code)
	require.Less(t, elapsed, budget+time.Second, "the list waits no longer than the hub's overall budget")
	var body struct {
		Presets []map[string]any `json:"presets"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotEmpty(t, body.Presets, "curated presets are still listed")
}

// A pull whose hub addresses both hang ends as hub_unreachable inside the budget, not as a generic gateway error.
func TestPullPresetHubUnreachableWhenEveryHubAddressHangs(t *testing.T) {
	const budget = 6 * time.Second
	r := newHangingHubPresetRouter(t, budget)

	body, err := json.Marshal(map[string]string{"slug": "crowdsecurity/missing"})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), "hub_unreachable")
	require.Less(t, time.Since(start), budget+time.Second)
}
