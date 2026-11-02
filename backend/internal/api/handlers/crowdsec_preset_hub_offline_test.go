package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	hubDownMsg       = "dial tcp: lookup hub.crowdsec.net: no such host"
	cscliIndexNginx  = `{"collections":[{"name":"crowdsecurity/nginx","title":"Nginx","description":"nginx","type":"collections"}]}`
	curatedHubBacked = "crowdsecurity/base-http-scenarios"
	curatedLocal     = "honeypot-friendly-defaults"
)

// hubOfflineHTTPDown answers every hub HTTP request with a transport failure.
func hubOfflineHTTPDown(*http.Request) (*http.Response, error) {
	return nil, errors.New(hubDownMsg)
}

// cscliIndexOnly answers the local cscli listing with a small index and fails every other cscli command.
func cscliIndexOnly(cmd string) ([]byte, error) {
	if cmd == "hub list -a -o json" {
		return []byte(cscliIndexNginx), nil
	}
	return []byte("Error: hub unreachable"), errors.New("exit status 1")
}

// cscliDown fails every cscli command, so no local index answers.
func cscliDown(string) ([]byte, error) {
	return []byte("Error: hub unreachable"), errors.New("exit status 1")
}

// getPresetList calls the list endpoint and returns the hub_offline flag (and whether the key is present) and
// the rows keyed by slug, or by type/slug for hub rows that carry a type.
func getPresetList(t *testing.T, e *presetEnv) (offline, hasKey bool, rows map[string]map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, presetsURL, http.NoBody))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	var body struct {
		Presets []map[string]any `json:"presets"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	rawFlag, hasKey := raw["hub_offline"]
	if hasKey {
		require.NoError(t, json.Unmarshal(rawFlag, &offline))
	}

	rows = map[string]map[string]any{}
	for _, p := range body.Presets {
		key, _ := p["slug"].(string)
		if typ, _ := p["type"].(string); typ != "" {
			key = typ + "/" + key
		}
		rows[key] = p
	}
	return offline, hasKey, rows
}

// The hub answers over HTTP: the flag is false, the hub rows are pullable and the cache-only row keeps the baseline.
func TestListPresetsHubOfflineFalseWhenHubIsUp(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	_, err := e.hub.Cache.Store(context.Background(), "crowdsecurity/legacy-only", "collections", "etag", "hub", []byte("name: crowdsecurity/legacy-only\n"))
	require.NoError(t, err)

	offline, hasKey, rows := getPresetList(t, e)

	assert.True(t, hasKey, "hub_offline is always present")
	assert.False(t, offline)
	assert.Equal(t, true, rows["appsec-rules/crowdsecurity/crs"]["available"])
	assert.Equal(t, true, rows["collections/crowdsecurity/nginx"]["available"])
	assert.Equal(t, true, rows[curatedHubBacked]["available"], "online, a hub-backed curated preset keeps its baseline")
	assert.Equal(t, true, rows[curatedLocal]["available"])
	assert.Equal(t, false, rows["collections/crowdsecurity/legacy-only"]["available"], "online, a cache-only row keeps the baseline")
	assert.Equal(t, true, rows["collections/crowdsecurity/legacy-only"]["cached"])
}

// The HTTP index fails but the local cscli index answers: the list is marked offline, the cscli rows cannot be
// pulled (no cache entry), and the curated rows follow their own rule.
func TestListPresetsHubOfflineTrueWhenCSCLIIndexAnswers(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	e.network = hubOfflineHTTPDown
	e.cscli = cscliIndexOnly

	offline, hasKey, rows := getPresetList(t, e)

	assert.True(t, hasKey)
	assert.True(t, offline)
	assert.Equal(t, false, rows["collections/crowdsecurity/nginx"]["available"], "an offline hub row with no cache entry is not pullable")
	assert.Equal(t, false, rows[curatedHubBacked]["available"], "a hub-backed curated preset with no cache entry is not pullable")
	assert.Equal(t, true, rows[curatedLocal]["available"], "a curated preset that needs no hub stays available")
}

// Every hub source is down: the list still answers with the curated presets and the cached rows. A cached row
// works offline, a hub row that is not cached does not.
func TestListPresetsHubOfflineTrueWhenEverySourceIsDown(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	ctx := context.Background()
	_, err := e.hub.Cache.Store(ctx, "crowdsecurity/nginx", "collections", "etag", "hub", []byte(itemNginx))
	require.NoError(t, err)
	_, err = e.hub.Cache.Store(ctx, "crowdsecurity/ghost", "collections", "etag", "hub", []byte("name: crowdsecurity/ghost\n"))
	require.NoError(t, err)
	e.network = hubOfflineHTTPDown
	e.cscli = cscliDown

	offline, hasKey, rows := getPresetList(t, e)

	assert.True(t, hasKey)
	assert.True(t, offline)
	assert.Equal(t, true, rows["collections/crowdsecurity/nginx"]["available"], "a cached row works offline")
	assert.Equal(t, true, rows["collections/crowdsecurity/nginx"]["cached"])
	assert.Equal(t, true, rows["collections/crowdsecurity/ghost"]["available"], "a cache-only row works offline")
	assert.NotContains(t, rows, "appsec-rules/crowdsecurity/crs", "an index row that is not cached is not listed without the index")
	assert.Equal(t, false, rows[curatedHubBacked]["available"])
	assert.Equal(t, true, rows[curatedLocal]["available"])
	assert.Equal(t, true, rows["geoip-enrichment"]["available"])
}

// A handler without a hub service serves the curated list and reports the hub as not used.
func TestListPresetsHubOfflineTrueWithoutHubService(t *testing.T) {
	t.Parallel()
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = nil
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, presetsURL, http.NoBody))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["hub_offline"])
}
