package crowdsec

import (
	"context"
	"errors"
	"io"
	"net/http"
	neturl "net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mirrorSSHDItemURL is the GitHub mirror location of the sshd item in the CDN fixture.
const mirrorSSHDItemURL = rawItemPrefix + "collections/crowdsecurity/sshd.yaml"

// cdnSSHDItemURL is the CDN location of the sshd item in the CDN fixture.
const cdnSSHDItemURL = cdnItemPrefix + "collections/crowdsecurity/sshd.yaml"

// mirrorTestService serves the CDN fixture index and lets the test choose the answers for the item downloads.
func mirrorTestService(t *testing.T, transport http.RoundTripper) *HubService {
	t.Helper()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	svc := NewHubService(nil, cache, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}
	return svc
}

// cdnIndexOnly answers the CDN index with the fixture and every other URL with a transport failure.
func cdnIndexOnly(t *testing.T, extra map[string]string, errs map[string]error, status map[string]int) *hubTransport {
	t.Helper()
	bodies := map[string]string{cdnIndexURL: string(readCDNFixture(t, "hub_index_cdn_compact.json"))}
	for k, v := range extra {
		bodies[k] = v
	}
	return &hubTransport{bodies: bodies, errs: errs, status: status}
}

func TestPullMirrorFallsBackWhenCDNItemResets(t *testing.T) {
	t.Parallel()
	item := string(readCDNFixture(t, "hub_item_cdn_sshd.yaml"))
	transport := cdnIndexOnly(t, map[string]string{mirrorSSHDItemURL: item},
		map[string]error{cdnSSHDItemURL: errors.New("connection reset by peer")}, nil)
	svc := mirrorTestService(t, transport)

	res, err := svc.Pull(context.Background(), "crowdsecurity/sshd", "collections")
	require.NoError(t, err)
	require.Equal(t, item, res.Preview)
	require.Equal(t, []string{cdnIndexURL, cdnSSHDItemURL, mirrorSSHDItemURL}, transport.seen())
}

func TestPullMirrorFallsBackOnServerErrorStatus(t *testing.T) {
	t.Parallel()
	item := string(readCDNFixture(t, "hub_item_cdn_sshd.yaml"))
	transport := cdnIndexOnly(t, map[string]string{mirrorSSHDItemURL: item}, nil,
		map[string]int{cdnSSHDItemURL: http.StatusServiceUnavailable})
	svc := mirrorTestService(t, transport)

	res, err := svc.Pull(context.Background(), "crowdsecurity/sshd", "collections")
	require.NoError(t, err)
	require.Equal(t, item, res.Preview)
	require.Contains(t, transport.seen(), mirrorSSHDItemURL)
}

func TestPullMirrorFallsBackWhenCDNBodyBreaks(t *testing.T) {
	t.Parallel()
	item := string(readCDNFixture(t, "hub_item_cdn_sshd.yaml"))
	inner := cdnIndexOnly(t, map[string]string{mirrorSSHDItemURL: item}, nil, nil)
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == cdnSSHDItemURL {
			return &http.Response{StatusCode: http.StatusOK, Body: brokenBody{}, Header: make(http.Header)}, nil
		}
		return inner.RoundTrip(req)
	})
	svc := mirrorTestService(t, transport)

	res, err := svc.Pull(context.Background(), "crowdsecurity/sshd", "collections")
	require.NoError(t, err)
	require.Equal(t, item, res.Preview)
	require.Contains(t, inner.seen(), mirrorSSHDItemURL)
}

func TestPullMirrorRefusesMirrorItemWithWrongDigest(t *testing.T) {
	t.Parallel()
	transport := cdnIndexOnly(t, map[string]string{mirrorSSHDItemURL: pullDemoItem},
		map[string]error{cdnSSHDItemURL: errors.New("connection reset by peer")}, nil)
	svc := mirrorTestService(t, transport)

	_, err := svc.Pull(context.Background(), "crowdsecurity/sshd", "collections")
	require.ErrorIs(t, err, ErrHubItemUnverified)
	require.Contains(t, transport.seen(), mirrorSSHDItemURL)
}

func TestPullMirrorBothFailsIsUnreachable(t *testing.T) {
	t.Parallel()
	transport := cdnIndexOnly(t, nil, map[string]error{
		cdnSSHDItemURL:    errors.New("connection reset by peer"),
		mirrorSSHDItemURL: errors.New("dial tcp: no route to host"),
	}, nil)
	svc := mirrorTestService(t, transport)

	_, err := svc.Pull(context.Background(), "crowdsecurity/sshd", "collections")
	require.ErrorIs(t, err, ErrHubUnreachable)
	require.Equal(t, []string{cdnIndexURL, cdnSSHDItemURL, mirrorSSHDItemURL}, transport.seen())
}

func TestPullMirrorCallerCancelIsNotRetried(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inner := cdnIndexOnly(t, nil, nil, nil)
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == cdnSSHDItemURL {
			cancel()
			return nil, &neturl.Error{Op: req.Method, URL: req.URL.String(), Err: context.Canceled}
		}
		return inner.RoundTrip(req)
	})
	svc := mirrorTestService(t, transport)

	_, err := svc.Pull(ctx, "crowdsecurity/sshd", "collections")
	require.Error(t, err)
	require.NotContains(t, inner.seen(), mirrorSSHDItemURL)
}

func TestPullMirrorNonFallbackStatusIsNotRetried(t *testing.T) {
	t.Parallel()
	transport := cdnIndexOnly(t, nil, nil, map[string]int{cdnSSHDItemURL: http.StatusBadRequest})
	svc := mirrorTestService(t, transport)

	_, err := svc.Pull(context.Background(), "crowdsecurity/sshd", "collections")
	require.Error(t, err)
	require.NotContains(t, transport.seen(), mirrorSSHDItemURL)
}

func TestPullMirrorSkippedWhenMirrorIsTheDownloadBase(t *testing.T) {
	t.Parallel()
	transport := cdnIndexOnly(t, nil, map[string]error{cdnSSHDItemURL: errors.New("connection reset by peer")}, nil)
	svc := mirrorTestService(t, transport)
	svc.MirrorBaseURL = cdnItemPrefix[:len(cdnItemPrefix)-1]

	_, err := svc.Pull(context.Background(), "crowdsecurity/sshd", "collections")
	require.Error(t, err)
	require.Len(t, transport.seen(), 2, "the item must be requested once, not again on the same base")
}

func TestPullMirrorSkippedWhenItemIsOutsideEveryBase(t *testing.T) {
	t.Parallel()
	index := `{"items":[{"name":"crowdsecurity/demo","title":"Demo","type":"collections","etag":"etag1","download_url":"https://hub.crowdsec.net/demo.yaml"}]}`
	transport := &hubTransport{
		bodies: map[string]string{cdnIndexURL: index},
		errs:   map[string]error{"https://hub.crowdsec.net/demo.yaml": errors.New("connection reset by peer")},
	}
	svc := mirrorTestService(t, transport)

	_, err := svc.Pull(context.Background(), "crowdsecurity/demo", "collections")
	require.Error(t, err)
	require.Equal(t, []string{cdnIndexURL, "https://hub.crowdsec.net/demo.yaml"}, transport.seen())
}

func TestPullMirrorItemURLDerivesPathUnderMirrorBase(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = defaultHubBaseURL
	svc.MirrorBaseURL = defaultHubMirrorBaseURL

	got, ok := svc.mirrorItemURL(cdnSSHDItemURL)
	require.True(t, ok)
	require.Equal(t, mirrorSSHDItemURL, got)

	_, ok = svc.mirrorItemURL("https://hub.crowdsec.net/demo.yaml")
	require.False(t, ok, "an item outside every hub base has no mirror location")

	svc.MirrorBaseURL = cdnItemPrefix[:len(cdnItemPrefix)-1]
	_, ok = svc.mirrorItemURL(cdnSSHDItemURL)
	require.False(t, ok, "a download already on the mirror base has nothing to retry")
}

// brokenBody is a response body that fails after the status line, as a connection reset mid-download does.
type brokenBody struct{}

func (brokenBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (brokenBody) Close() error             { return nil }
