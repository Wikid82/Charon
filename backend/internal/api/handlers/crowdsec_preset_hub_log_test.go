package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// When every hub candidate fails, the pull failure log names each URL that was requested and does not attribute the
// failure to the configured base alone, and it carries no credentials from the configured base.
func TestPullPresetHandlerLogNamesEveryAttemptedHubURL(t *testing.T) {
	var logs bytes.Buffer
	logger.Init(false, &logs)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })

	const cdnIndex = "https://cdn-hub.crowdsec.net/crowdsecurity/master/.index.json"
	const mirrorIndex = "https://raw.githubusercontent.com/crowdsecurity/hub/master/.index.json"
	const mirrorAPIIndex = "https://raw.githubusercontent.com/crowdsecurity/hub/master/api/index.json"

	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	hub := crowdsec.NewHubService(nil, cache, t.TempDir())
	hub.HubBaseURL = "https://operator:s3cret@cdn-hub.crowdsec.net/crowdsecurity/master"
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = hub

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	body, _ := json.Marshal(map[string]string{"slug": "crowdsecurity/missing"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)

	out := logs.String()
	require.Contains(t, out, `"crowdsec preset pull failed"`)
	require.Contains(t, out, cdnIndex, "the CDN attempt is named")
	require.Contains(t, out, mirrorIndex, "the mirror attempt is named")
	require.Contains(t, out, mirrorAPIIndex, "the second mirror attempt is named")
	require.NotContains(t, out, `"hub_base_url"`, "the configured base is not reported as the URL that failed")
	require.NotContains(t, out, "s3cret", "credentials in the configured base never reach the log")
	require.NotContains(t, out, "operator:")
}

// An operator-set HUB_BASE_URL that cannot be parsed never reaches the pull API error body or the log.
func TestPullPresetHandlerOmitsUnparseableHubBaseCredentials(t *testing.T) {
	var logs bytes.Buffer
	logger.Init(false, &logs)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })

	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	hub := crowdsec.NewHubService(nil, cache, t.TempDir())
	hub.HubBaseURL = "https://operator:s3cret%zz@cdn-hub.crowdsec.net/crowdsecurity/master"
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = hub

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	body, _ := json.Marshal(map[string]string{"slug": "crowdsecurity/missing"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.NotContains(t, w.Body.String(), "s3cret")
	require.NotContains(t, w.Body.String(), "%zz")
	require.NotContains(t, w.Body.String(), "operator")
	require.NotContains(t, logs.String(), "s3cret")
	require.NotContains(t, logs.String(), "%zz")
}
