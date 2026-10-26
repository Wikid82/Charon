package crowdsec

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// A hub index answered with a redirect is reported as a redirect and never followed. The Location target carries
// userinfo and a query string, and neither may reach the error text.
func TestFetchIndexHTTPReportsRedirectWithoutFollowing(t *testing.T) {
	location := (&neturl.URL{
		Scheme:   "https",
		User:     neturl.UserPassword("mirror-user", "pw-s3cret"),
		Host:     "mirror.example.test",
		Path:     "/index.json",
		RawQuery: "token=tok-s3cret",
	}).String()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	svc := NewHubService(nil, nil, t.TempDir())
	svc.validateURL = func(string) error { return nil } // the local httptest server is not an allowlisted hub host
	svc.HTTPClient = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	_, err := svc.fetchIndexHTTPFromURL(context.Background(), server.URL+"/index.json")
	require.Error(t, err)

	var herr hubHTTPError
	require.ErrorAs(t, err, &herr)
	require.Equal(t, http.StatusFound, herr.statusCode)
	require.True(t, herr.fallback, "a redirect must let the candidate loop try the next hub base")
	require.ErrorContains(t, err, "redirect")
	require.ErrorContains(t, err, "mirror.example.test/index.json", "the sanitized redirect host is still named")
	require.NotContains(t, err.Error(), "pw-s3cret")
	require.NotContains(t, err.Error(), "mirror-user")
	require.NotContains(t, err.Error(), "tok-s3cret")
	require.NotContains(t, err.Error(), "?token")
}

// failingMidReadBody yields a prefix of a payload and then fails, as a connection reset during the body would.
type failingMidReadBody struct {
	sent bool
}

func (b *failingMidReadBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		return copy(p, `{"items":`), nil
	}
	return 0, errors.New("connection reset mid-body")
}

func (b *failingMidReadBody) Close() error { return nil }

// A body that fails partway through is reported as a read error that names the URL without its userinfo or query.
func TestFetchWithLimitFromURLReportsBodyReadFailure(t *testing.T) {
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusOK, "")
		resp.Body = &failingMidReadBody{}
		return resp, nil
	})}

	_, err := svc.fetchWithLimitFromURL(context.Background(), "https://user:pw-s3cret@hub.crowdsec.net/x.yaml?token=tok-s3cret", 1024)
	require.Error(t, err)
	require.ErrorContains(t, err, "read ")
	require.ErrorContains(t, err, "connection reset mid-body")
	require.ErrorContains(t, err, "hub.crowdsec.net/x.yaml")
	require.NotContains(t, err.Error(), "pw-s3cret")
	require.NotContains(t, err.Error(), "tok-s3cret")
	require.NotContains(t, err.Error(), "?token")
}
