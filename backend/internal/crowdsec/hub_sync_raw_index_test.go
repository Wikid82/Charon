package crowdsec

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchIndexParsesRawIndexFormat(t *testing.T) {
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = "http://example.com"

	// This JSON represents the "raw" index format (map of maps) which has no "items" field.
	// json.Unmarshal into HubIndex will succeed but result in empty Items.
	// The fix should detect this and fall back to parseRawIndex.
	rawIndexBody := `{
		"collections": {
			"crowdsecurity/base-http-scenarios": {
				"path": "collections/crowdsecurity/base-http-scenarios.yaml",
				"version": "0.1",
				"description": "Base HTTP scenarios"
			}
		},
		"parsers": {
			"crowdsecurity/nginx-logs": {
				"path": "parsers/s01-parse/crowdsecurity/nginx-logs.yaml",
				"version": "1.0",
				"description": "Parse Nginx logs"
			}
		}
	}`

	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "http://example.com"+defaultHubIndexPath {
			resp := newResponse(http.StatusOK, rawIndexBody)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}
		return newResponse(http.StatusNotFound, ""), nil
	})}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, idx.Items)

	// Verify we found the items
	foundCollection := false
	foundParser := false
	for _, item := range idx.Items {
		if item.Name == "crowdsecurity/base-http-scenarios" {
			foundCollection = true
			require.Equal(t, "collections", item.Type)
			require.Equal(t, "0.1", item.Version)
		}
		if item.Name == "crowdsecurity/nginx-logs" {
			foundParser = true
			require.Equal(t, "parsers", item.Type)
			require.Equal(t, "1.0", item.Version)
		}
	}
	require.True(t, foundCollection, "should find collection from raw index")
	require.True(t, foundParser, "should find parser from raw index")
}
