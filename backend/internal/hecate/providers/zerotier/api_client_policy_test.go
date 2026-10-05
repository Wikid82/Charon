package zerotier

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wikid82/charon/backend/pkg/safehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestNewZeroTierClient_DefaultURL(t *testing.T) {
	c, err := newZeroTierClientWithHTTP(context.Background(), "tok", "", http.DefaultClient)
	require.NoError(t, err)
	assert.Equal(t, defaultControllerURL, c.controllerURL.String())
}

func TestNewZeroTierClient_RejectsDisallowedURLs(t *testing.T) {
	blocked := []string{
		"https://127.0.0.1", "https://127.0.0.2:8443", "https://[::1]", "https://[::ffff:127.0.0.1]",
		"https://10.0.0.5", "https://192.168.1.1", "https://172.16.0.1",
		"https://169.254.169.254", "https://[fd00::1]", "https://[fe80::1]",
		"https://100.64.0.1", "https://100.127.255.254",
		"https://[64:ff9b::1]", "https://[2002::1]", "https://192.0.0.1", "https://198.18.0.1",
		"https://0.0.0.0", "https://localhost", "https://localhost.",
	}
	for _, raw := range blocked {
		_, err := NewZeroTierClient(context.Background(), "tok", raw)
		require.Error(t, err, raw)
		assert.True(t, errors.Is(err, safehttp.ErrBlockedAddress), "%s: expected blocked-address error, got %v", raw, err)
		assert.True(t, strings.HasPrefix(err.Error(), "zerotier: "), "%s: missing prefix: %v", raw, err)
	}

	other := []string{
		"http://api.zerotier.com", "https://user:pw@api.zerotier.com", "https://good.com@169.254.169.254",
		"ftp://api.zerotier.com", "https://api.zerotier.com/?x=1", "https://api.zerotier.com/#frag", "https://[::1",
	}
	for _, raw := range other {
		_, err := NewZeroTierClient(context.Background(), "tok", raw)
		require.Error(t, err, raw)
		assert.True(t, strings.HasPrefix(err.Error(), "zerotier: "), "%s: missing prefix: %v", raw, err)
	}
}

func TestNewZeroTierClient_ProductionClientIsHardened(t *testing.T) {
	c, err := NewZeroTierClient(context.Background(), "tok", "https://8.8.8.8/ztc/")
	require.NoError(t, err)

	tr, ok := c.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Nil(t, tr.Proxy, "proxy environment must be ignored")
	assert.ErrorIs(t, c.httpClient.CheckRedirect(&http.Request{}, nil), http.ErrUseLastResponse)
	assert.Equal(t, "8.8.8.8", c.controllerURL.Hostname())
}

func TestZeroTierClient_ProductionClientBlocksLoopbackAtDial(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c, err := NewZeroTierClient(context.Background(), "tok", "https://8.8.8.8")
	require.NoError(t, err)
	// Re-point the validated client at a loopback server: only the dialer stands in the way.
	c.controllerURL = mustParseURL(t, srv.URL)

	_, err = c.ListNetworks(context.Background())
	require.ErrorIs(t, err, safehttp.ErrBlockedAddress)
	_, err = c.ListMembers(context.Background(), "a1b2c3d4e5f60718")
	require.ErrorIs(t, err, safehttp.ErrBlockedAddress)
	assert.Zero(t, hits.Load(), "no connection may reach the loopback server")
}

// Uses a test-supplied CheckRedirect, so it does not prove production wiring;
// TestNewZeroTierClient_ProductionClientIsHardened covers the constructor.
func TestZeroTierClient_StatusErrorOnRedirectResponse(t *testing.T) {
	var secondHits atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondHits.Add(1)
		_, _ = w.Write([]byte("[]"))
	}))
	defer second.Close()

	for _, code := range []int{http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, second.URL, code)
		}))
		hc := first.Client()
		hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		c, err := newZeroTierClientWithHTTP(context.Background(), "tok", first.URL, hc)
		require.NoError(t, err)

		_, err = c.ListNetworks(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status")
		_, err = c.ListMembers(context.Background(), "a1b2c3d4e5f60718")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status")
		first.Close()
	}
	assert.Zero(t, secondHits.Load())
}

func TestListMembers_NetworkIDValidation(t *testing.T) {
	var hits atomic.Int32
	var gotPath atomic.Value
	c, _ := newTestZTClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		gotPath.Store(r.URL.EscapedPath())
		_, _ = w.Write([]byte("[]"))
	})

	invalid := []string{
		"net1", "../x", "a1b2c3d4e5f6071", "a1b2c3d4e5f607189", "A1B2C3D4E5F60718",
		"%2e%2e", "a1b2c3d4e5f6071/", "", "a1b2c3d4e5f6071g", "a1b2c3d4e5f60718\n", "a1b2c3d4/../ab",
	}
	for _, id := range invalid {
		_, err := c.ListMembers(context.Background(), id)
		require.Error(t, err, "%q", id)
		assert.ErrorIs(t, err, ErrInvalidNetworkID, "%q", id)
		assert.Zero(t, hits.Load(), "invalid id %q must not produce a request", id)
	}

	members, err := c.ListMembers(context.Background(), "a1b2c3d4e5f60718")
	require.NoError(t, err)
	assert.Empty(t, members)
	assert.Equal(t, int32(1), hits.Load())
	assert.Equal(t, "/api/v1/network/a1b2c3d4e5f60718/member", gotPath.Load())
}

func TestZeroTierClient_WithHTTPRejectsUnparsableURL(t *testing.T) {
	_, err := newZeroTierClientWithHTTP(context.Background(), "tok", "https://[::1", http.DefaultClient)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "zerotier: "), "unexpected error: %v", err)
}

// The empty-URL default is applied before validation. Whether the default host
// then resolves depends on the machine, so only the "not rejected as empty"
// property is asserted.
func TestNewZeroTierClient_EmptyURLUsesDefault(t *testing.T) {
	c, err := NewZeroTierClient(context.Background(), "tok", "")
	if err != nil {
		assert.NotContains(t, err.Error(), "url is required")
		return
	}
	assert.Equal(t, defaultControllerURL, c.controllerURL.String())
}

func TestZeroTierClient_ListNetworks_RejectsUnusableBaseURL(t *testing.T) {
	c := &ZeroTierClient{controllerURL: &url.URL{}, httpClient: http.DefaultClient}
	_, err := c.ListNetworks(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build request url")
}
