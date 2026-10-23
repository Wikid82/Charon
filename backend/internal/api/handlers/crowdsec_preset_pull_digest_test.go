package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPullPresetRefusesHubItemThatDoesNotMatchIndex covers a hub whose index publishes a digest that the served
// item does not match. The pull must answer 409 preset_hub_updating and must not cache the item.
func TestPullPresetRefusesHubItemThatDoesNotMatchIndex(t *testing.T) {
	dataDir := t.TempDir()
	hub, cache := integrationHub(t, dataDir)
	index := `{"collections":{"test/preset":{"path":"test.yaml","version":"1.0","versions":{"1.0":{"digest":"` +
		strings.Repeat("0", 64) + `"}}}}}`
	hub.HTTPClient = &http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://hub.crowdsec.net/api/index.json":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(index)), Header: make(http.Header)}, nil
		case "https://hub.crowdsec.net/test.yaml":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(integrationItem)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}
	r, _ := integrationRouter(t, hub, dataDir)

	w := postPresetAny(r, presetPullURL, map[string]any{"slug": "test/preset"})
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, codePresetHubUpdating, resp["error_code"])

	_, _, err := cache.LoadItem(context.Background(), "collections", "test/preset")
	require.Error(t, err, "a refused item must not be cached")
}
