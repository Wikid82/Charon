package netbird

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

func newTestNBClient(t *testing.T, handler http.HandlerFunc) (*NetBirdClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := newNetBirdClientWithURL(context.Background(), "nb-test-token", srv.URL, true)
	require.NoError(t, err)
	return c, srv
}

func nbPeersResponse(peers []NetBirdPeer) string {
	b, _ := json.Marshal(peers)
	return string(b)
}

func validNBCredsJSON(managementURL string) string {
	if managementURL == "" {
		return `{"access_token":"nb-test-token"}`
	}
	return fmt.Sprintf(`{"access_token":"nb-test-token","management_url":%q}`, managementURL)
}

// --- API Client Tests ---

func TestListPeers_Success(t *testing.T) {
	peers := []NetBirdPeer{
		{ID: "p1", Name: "server-1", IP: "100.64.0.1", OS: "linux", Connected: true},
		{ID: "p2", Name: "server-2", IP: "100.64.0.2", OS: "darwin", Connected: false},
	}
	c, _ := newTestNBClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Token nb-test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "/api/peers", r.URL.Path)
		fmt.Fprint(w, nbPeersResponse(peers)) //nolint:errcheck
	})

	result, err := c.ListPeers(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "p1", result[0].ID)
	assert.Equal(t, "100.64.0.1", result[0].IP)
}

func TestListPeers_AuthError(t *testing.T) {
	c, _ := newTestNBClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"unauthorized"}`) //nolint:errcheck
	})

	_, err := c.ListPeers(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestListPeers_ServerError(t *testing.T) {
	c, _ := newTestNBClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.ListPeers(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestListPeers_MalformedJSON(t *testing.T) {
	c, _ := newTestNBClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `not-json`) //nolint:errcheck
	})

	_, err := c.ListPeers(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestListPeers_CacheHit(t *testing.T) {
	var callCount atomic.Int32
	peers := []NetBirdPeer{{ID: "cached"}}

	c, _ := newTestNBClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		fmt.Fprint(w, nbPeersResponse(peers)) //nolint:errcheck
	})

	r1, err := c.ListPeers(context.Background())
	require.NoError(t, err)
	require.Len(t, r1, 1)
	assert.Equal(t, int32(1), callCount.Load())

	// Second call within TTL should return cached result without HTTP.
	r2, err := c.ListPeers(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load(), "cache should prevent second HTTP call")
	assert.Equal(t, r1[0].ID, r2[0].ID)
}

func TestListPeers_CacheExpiry(t *testing.T) {
	var callCount atomic.Int32
	peers := []NetBirdPeer{{ID: "fresh"}}

	c, _ := newTestNBClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		fmt.Fprint(w, nbPeersResponse(peers)) //nolint:errcheck
	})

	// Seed a stale cache entry.
	c.mu.Lock()
	c.cache = peers
	c.cacheTime = time.Now().Add(-2 * cacheTTL)
	c.mu.Unlock()

	_, err := c.ListPeers(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load(), "stale cache must trigger HTTP call")
}

func TestForceRefresh_BypassesCache(t *testing.T) {
	var callCount atomic.Int32
	peers := []NetBirdPeer{{ID: "refreshed"}}

	c, _ := newTestNBClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		fmt.Fprint(w, nbPeersResponse(peers)) //nolint:errcheck
	})

	// Prime the cache.
	_, err := c.ListPeers(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load())

	// ForceRefresh should bypass cache.
	r, err := c.ForceRefresh(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(2), callCount.Load(), "ForceRefresh must always make HTTP request")
	assert.Equal(t, "refreshed", r[0].ID)
}

// --- SSRF Validation Tests ---

func TestNewNetBirdClient_DefaultURL(t *testing.T) {
	// Empty management URL defaults to api.netbird.io — SSRF runs but it's a real host.
	// We skip SSRF to avoid DNS calls in CI; just verify the base URL is set correctly.
	c, err := newNetBirdClientWithURL(context.Background(), "tok", "", true)
	require.NoError(t, err)
	assert.Equal(t, defaultManagementURL, c.baseURL)
}

func TestNewNetBirdClient_InvalidScheme(t *testing.T) {
	_, err := NewNetBirdClient(context.Background(), "tok", "http://api.netbird.io")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https scheme")
}

func TestNewNetBirdClient_LoopbackRejected(t *testing.T) {
	_, err := newNetBirdClientWithURL(context.Background(), "tok", "https://127.0.0.1", false)
	require.Error(t, err)
}

func TestNewNetBirdClient_PrivateRangeRejected(t *testing.T) {
	_, err := newNetBirdClientWithURL(context.Background(), "tok", "https://192.168.1.1", false)
	require.Error(t, err)
}

func TestNewNetBirdClient_LinkLocalRejected(t *testing.T) {
	_, err := NewNetBirdClient(context.Background(), "tok", "https://169.254.169.254")
	require.Error(t, err)
}

func TestNewNetBirdClient_SkipSSRFAllowsLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "[]") //nolint:errcheck
	}))
	defer srv.Close()

	c, err := newNetBirdClientWithURL(context.Background(), "tok", srv.URL, true)
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestIsPrivateIP_Ranges(t *testing.T) {
	tests := []struct {
		addr     string
		expected bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.addr)
		require.NotNil(t, ip, "failed to parse IP %s", tt.addr)
		assert.Equal(t, tt.expected, isPrivateIP(ip), "IP %s", tt.addr)
	}
}

// --- Provider Tests ---

func TestFactory_ValidCredentials(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "nb-uuid", Name: "nb-test"}
	p, err := Factory(cfg, validNBCredsJSON(""))
	require.NoError(t, err)
	assert.Equal(t, "netbird", p.Name())
}

func TestFactory_InvalidJSON(t *testing.T) {
	_, err := Factory(&models.TunnelConfig{}, "not-json")
	require.Error(t, err)
}

func TestFactory_MissingAccessToken(t *testing.T) {
	_, err := Factory(&models.TunnelConfig{}, `{"management_url":"https://api.netbird.io"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
}

func TestNetBirdProvider_Start_Success(t *testing.T) {
	peers := []NetBirdPeer{
		{ID: "p1", Name: "node-1"},
		{ID: "p2", Name: "node-2"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, nbPeersResponse(peers)) //nolint:errcheck
	}))
	defer srv.Close()

	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)

	p.newClientFn = func(ctx context.Context, token, url string) (*NetBirdClient, error) {
		return newNetBirdClientWithURL(ctx, token, srv.URL, true)
	}

	require.NoError(t, p.Start(context.Background()))
	assert.Equal(t, hecate.TunnelStateConnected, p.Status())
	assert.NotNil(t, p.GetClient())
}

func TestNetBirdProvider_Start_InvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)

	p.newClientFn = func(ctx context.Context, token, url string) (*NetBirdClient, error) {
		return newNetBirdClientWithURL(ctx, "bad-token", srv.URL, true)
	}

	err = p.Start(context.Background())
	require.Error(t, err)
	assert.Equal(t, hecate.TunnelStateError, p.Status())
}

func TestNetBirdProvider_Start_EmptyPeerList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "[]") //nolint:errcheck
	}))
	defer srv.Close()

	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)

	p.newClientFn = func(ctx context.Context, token, url string) (*NetBirdClient, error) {
		return newNetBirdClientWithURL(ctx, token, srv.URL, true)
	}

	require.NoError(t, p.Start(context.Background()))
	assert.Equal(t, hecate.TunnelStateConnected, p.Status())
}

func TestNetBirdProvider_Start_ClientCreationFails(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)

	p.newClientFn = func(ctx context.Context, token, url string) (*NetBirdClient, error) {
		return nil, fmt.Errorf("injected factory error")
	}

	err = p.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create client")
	assert.Equal(t, hecate.TunnelStateError, p.Status())
}

func TestNetBirdProvider_Stop(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)

	p.mu.Lock()
	p.state = hecate.TunnelStateConnected
	p.mu.Unlock()

	require.NoError(t, p.Stop())
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
}

func TestNetBirdProvider_Name(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)
	assert.Equal(t, "netbird", p.Name())
}

func TestNetBirdProvider_GetAddress(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)
	assert.Empty(t, p.GetAddress())
}

func TestNetBirdProvider_GetClient(t *testing.T) {
	peers := []NetBirdPeer{{ID: "p1"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, nbPeersResponse(peers)) //nolint:errcheck
	}))
	defer srv.Close()

	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)
	assert.Nil(t, p.GetClient())

	p.newClientFn = func(ctx context.Context, token, url string) (*NetBirdClient, error) {
		return newNetBirdClientWithURL(ctx, token, srv.URL, true)
	}

	require.NoError(t, p.Start(context.Background()))
	assert.NotNil(t, p.GetClient())
}

func TestNetBirdProvider_Status_ThreadSafe(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.Status()
		}()
	}
	wg.Wait()
}

func TestNewNetBirdProvider_ParsesCredentials(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "nb-uuid", Name: "nb-test"}
	p, err := NewNetBirdProvider(cfg, validNBCredsJSON(""))
	require.NoError(t, err)
	assert.Equal(t, "netbird", p.Name())
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
	assert.Empty(t, p.GetAddress())
}

func TestNewNetBirdProvider_MissingAccessToken(t *testing.T) {
	_, err := NewNetBirdProvider(&models.TunnelConfig{}, `{"management_url":"https://api.netbird.io"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
}

func TestNewNetBirdProvider_InvalidJSON(t *testing.T) {
	_, err := NewNetBirdProvider(&models.TunnelConfig{}, "not-json")
	require.Error(t, err)
}
