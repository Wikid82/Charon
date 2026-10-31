package crowdsec

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// captureHubIndexLogs routes the global logger to a buffer for the test and restores stdout afterwards.
func captureHubIndexLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })
	return &buf
}

// A CDN index that answers is logged with the URL that served it, and no mirror is named.
func TestHubIndexLogNamesServingCDNURL(t *testing.T) {
	logs := captureHubIndexLogs(t)
	transport := &hubTransport{bodies: map[string]string{cdnIndexURL: string(readCDNFixture(t, "hub_index_cdn_compact.json"))}}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	_, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)

	out := logs.String()
	require.Contains(t, out, `"hub_index":"`+cdnIndexURL+`"`)
	require.Contains(t, out, `"fallback_used":false`)
	require.NotContains(t, out, rawIndexURL, "the mirror was never asked, so it is not named")
}

// A failed CDN attempt is logged with the CDN URL, and the success that follows names the mirror that served.
func TestHubIndexLogNamesFailedCDNAndServingMirror(t *testing.T) {
	logs := captureHubIndexLogs(t)
	transport := &hubTransport{
		status: map[string]int{cdnIndexURL: http.StatusServiceUnavailable},
		bodies: map[string]string{rawIndexURL: readCSCLIFixture(t, "hub_index_raw_sample.json")},
	}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, idx.Items)

	out := logs.String()
	require.Contains(t, out, `"hub index fetch failed, trying mirror"`)
	require.Contains(t, out, `"hub_index":"`+cdnIndexURL+`"`, "the failed CDN attempt is named")
	require.Contains(t, out, `"hub index fetched"`)
	require.Contains(t, out, `"hub_index":"`+rawIndexURL+`"`, "the mirror that served the index is named")
	require.Contains(t, out, `"fallback_used":true`)
}

// When every candidate fails, the returned error names each URL that was requested, in attempt order.
func TestHubIndexErrorNamesEveryFailedCandidate(t *testing.T) {
	logs := captureHubIndexLogs(t)
	transport := &hubTransport{status: map[string]int{
		cdnIndexURL: http.StatusServiceUnavailable,
		rawIndexURL: http.StatusServiceUnavailable,
	}}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	_, err := svc.FetchIndex(context.Background())
	require.Error(t, err)

	msg := err.Error()
	require.Contains(t, msg, cdnIndexURL)
	require.Contains(t, msg, rawIndexURL)
	require.Equal(t, []string{cdnIndexURL, rawIndexURL}, transport.seen())

	out := logs.String()
	require.Equal(t, 2, strings.Count(out, `"hub index fetch failed, trying mirror"`), "each failed candidate is logged once")
	require.NotContains(t, out, `"hub index fetched"`)
}

// Credentials in an operator-set hub URL never reach the logs or the returned error.
func TestHubIndexLogAndErrorOmitUserinfo(t *testing.T) {
	const withUserinfo = "https://operator:s3cret@cdn-hub.crowdsec.net/crowdsecurity/master" //nolint:gosec // G101: test credential
	const indexWithUserinfo = withUserinfo + "/.index.json"

	t.Run("failed candidate is logged without credentials", func(t *testing.T) {
		logs := captureHubIndexLogs(t)
		transport := &hubTransport{
			status: map[string]int{indexWithUserinfo: http.StatusServiceUnavailable},
			bodies: map[string]string{cdnIndexURL: string(readCDNFixture(t, "hub_index_cdn_compact.json"))},
		}
		svc := NewHubService(nil, nil, t.TempDir())
		svc.HubBaseURL = withUserinfo
		svc.HTTPClient = &http.Client{Transport: transport}

		_, err := svc.FetchIndex(context.Background())
		require.NoError(t, err)
		require.NotContains(t, logs.String(), "s3cret")
		require.NotContains(t, logs.String(), "operator:")
		require.Contains(t, logs.String(), `"hub_index":"https://cdn-hub.crowdsec.net/crowdsecurity/master/.index.json"`)
	})

	t.Run("served index is logged without credentials", func(t *testing.T) {
		logs := captureHubIndexLogs(t)
		transport := &hubTransport{bodies: map[string]string{
			indexWithUserinfo: string(readCDNFixture(t, "hub_index_cdn_compact.json")),
		}}
		svc := NewHubService(nil, nil, t.TempDir())
		svc.HubBaseURL = withUserinfo
		svc.HTTPClient = &http.Client{Transport: transport}

		_, err := svc.FetchIndex(context.Background())
		require.NoError(t, err)
		require.NotContains(t, logs.String(), "s3cret")
		require.NotContains(t, logs.String(), "operator:")
	})

	t.Run("aggregated error carries no credentials", func(t *testing.T) {
		captureHubIndexLogs(t)
		transport := &hubTransport{status: map[string]int{
			indexWithUserinfo: http.StatusServiceUnavailable,
			cdnIndexURL:       http.StatusServiceUnavailable,
			rawIndexURL:       http.StatusServiceUnavailable,
		}}
		svc := NewHubService(nil, nil, t.TempDir())
		svc.HubBaseURL = withUserinfo
		svc.HTTPClient = &http.Client{Transport: transport}

		_, err := svc.FetchIndex(context.Background())
		require.Error(t, err)
		require.NotContains(t, err.Error(), "s3cret")
		require.NotContains(t, err.Error(), "operator:")
		require.Contains(t, err.Error(), "https://cdn-hub.crowdsec.net/crowdsecurity/master/.index.json")
	})
}

// A transport failure carries the request URL inside url.Error; that URL must not leak credentials either.
func TestHubIndexTransportErrorOmitsUserinfo(t *testing.T) {
	captureHubIndexLogs(t)
	const withUserinfo = "https://operator:s3cret@cdn-hub.crowdsec.net/crowdsecurity/master" //nolint:gosec // G101: test credential
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = withUserinfo
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, &neturl.Error{Op: "Get", URL: req.URL.String(), Err: errors.New("connection refused")}
	})}

	_, err := svc.FetchIndex(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "s3cret")
	require.NotContains(t, err.Error(), "operator:")
	require.Contains(t, err.Error(), "connection refused")
}

// An operator-set hub URL that cannot be parsed is rejected without echoing it: neither the returned error nor the
// log carries the raw string, its credentials or the offending escape sequence.
func TestHubURLUnparseableOmitsRawCredentials(t *testing.T) {
	const badBase = "https://operator:s3cret%zz@cdn-hub.crowdsec.net/crowdsecurity/master" //nolint:gosec // G101: test credential

	err := validateHubURL(badBase + "/.index.json")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "s3cret")
	require.NotContains(t, err.Error(), "operator")
	require.NotContains(t, err.Error(), "%zz")

	logs := captureHubIndexLogs(t)
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = badBase
	svc.HTTPClient = &http.Client{Transport: &hubTransport{}}

	_, err = svc.FetchIndex(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "s3cret")
	require.NotContains(t, err.Error(), "operator")
	require.NotContains(t, err.Error(), "%zz")
	require.NotContains(t, logs.String(), "s3cret")
	require.NotContains(t, logs.String(), "%zz")
}

// A transport failure while downloading a hub resource names the URL without credentials.
func TestHubResourceTransportErrorOmitsUserinfo(t *testing.T) {
	const withUserinfo = "https://operator:s3cret@cdn-hub.crowdsec.net/crowdsecurity/master" //nolint:gosec // G101: test credential
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = withUserinfo
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, &neturl.Error{Op: "Get", URL: req.URL.String(), Err: errors.New("connection refused")}
	})}

	_, err := svc.fetchWithLimitFromURL(context.Background(), withUserinfo+"/collections/crowdsecurity/sshd.yaml", 1024)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "s3cret")
	require.NotContains(t, err.Error(), "operator:")
	require.Contains(t, err.Error(), "connection refused")
	require.Contains(t, err.Error(), "https://cdn-hub.crowdsec.net/crowdsecurity/master/collections/crowdsecurity/sshd.yaml")
}

// When validation is bypassed and the request cannot be built, the download fails before any HTTP call is made.
func TestFetchWithLimitFailsWhenRequestCannotBeBuilt(t *testing.T) {
	svc := NewHubService(nil, nil, t.TempDir())
	svc.validateURL = func(string) error { return nil }
	svc.HTTPClient = &http.Client{Transport: &hubTransport{}}

	_, err := svc.fetchWithLimitFromURL(context.Background(), "https://[::1", 1024)
	require.Error(t, err)
}
