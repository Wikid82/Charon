package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- API Client Tests ---

func newTestTSClient(t *testing.T, handler http.HandlerFunc) (*TailscaleClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewTailscaleClient("tskey-api-test", "example.com")
	c.baseURL = srv.URL
	return c, srv
}

func tsDevicesResponse(devices []TailscaleDevice) string {
	b, _ := json.Marshal(map[string]any{"devices": devices})
	return string(b)
}

func TestListDevices_Success(t *testing.T) {
	devices := []TailscaleDevice{
		{ID: "d1", Hostname: "server-1", Addresses: []string{"100.64.0.1"}, OS: "linux", Online: true},
	}
	c, _ := newTestTSClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify Basic auth: apiKey as username, empty password.
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "tskey-api-test", user)
		assert.Empty(t, pass)
		assert.Contains(t, r.URL.Path, "/devices")
		fmt.Fprint(w, tsDevicesResponse(devices)) //nolint:errcheck
	})

	result, err := c.ListDevices(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "d1", result[0].ID)
	assert.Equal(t, "100.64.0.1", result[0].Addresses[0])
}

func TestListDevices_AuthError(t *testing.T) {
	c, _ := newTestTSClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"unauthorized"}`) //nolint:errcheck
	})

	_, err := c.ListDevices(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestListDevices_Cache_SecondCallNoHTTP(t *testing.T) {
	var callCount atomic.Int32
	devices := []TailscaleDevice{{ID: "cached", Hostname: "cached-host"}}

	c, _ := newTestTSClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		fmt.Fprint(w, tsDevicesResponse(devices)) //nolint:errcheck
	})
	r1, err := c.ListDevices(context.Background())
	require.NoError(t, err)
	require.Len(t, r1, 1)
	assert.Equal(t, int32(1), callCount.Load())

	// Second call within TTL should return cached result without HTTP.
	r2, err := c.ListDevices(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load(), "cache should prevent second HTTP call")
	assert.Equal(t, r1[0].ID, r2[0].ID)
}

func TestListDevices_CacheExpires(t *testing.T) {
	var callCount atomic.Int32
	devices := []TailscaleDevice{{ID: "fresh"}}

	c, _ := newTestTSClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		fmt.Fprint(w, tsDevicesResponse(devices)) //nolint:errcheck
	})

	// Seed a stale cache entry.
	c.mu.Lock()
	c.cache = &cachedDevices{
		devices:   devices,
		fetchedAt: time.Now().Add(-2 * cacheTTL),
	}
	c.mu.Unlock()

	_, err := c.ListDevices(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load(), "stale cache must trigger HTTP call")
}

func TestForceRefresh_AlwaysHTTP(t *testing.T) {
	var callCount atomic.Int32
	devices := []TailscaleDevice{{ID: "refreshed"}}

	c, _ := newTestTSClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		fmt.Fprint(w, tsDevicesResponse(devices)) //nolint:errcheck
	})

	// Prime cache.
	_, err := c.ListDevices(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load())

	// ForceRefresh should bypass cache.
	r, err := c.ForceRefresh(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(2), callCount.Load(), "ForceRefresh must always make HTTP request")
	assert.Equal(t, "refreshed", r[0].ID)
}

func TestForceRefresh_UpdatesCache(t *testing.T) {
	updated := []TailscaleDevice{{ID: "updated-id"}}
	c, _ := newTestTSClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, tsDevicesResponse(updated)) //nolint:errcheck
	})

	_, err := c.ForceRefresh(context.Background())
	require.NoError(t, err)

	// Verify cache was updated (cached device count matches response).
	c.mu.RLock()
	defer c.mu.RUnlock()
	require.NotNil(t, c.cache)
	require.Len(t, c.cache.devices, 1)
	assert.Equal(t, "updated-id", c.cache.devices[0].ID)
}

// --- Provider Tests ---

func validTSCredsJSON() string {
	return `{"api_key":"tskey-api-test","tailnet":"example.com"}`
}

func TestNewTailscaleProvider_ParsesCredentials(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "ts-uuid", Name: "ts-test"}
	p, err := NewTailscaleProvider(cfg, validTSCredsJSON())
	require.NoError(t, err)
	assert.Equal(t, "tailscale", p.Name())
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
	assert.Empty(t, p.GetAddress())
}

func TestNewTailscaleProvider_MissingAPIKey(t *testing.T) {
	_, err := NewTailscaleProvider(&models.TunnelConfig{}, `{"tailnet":"x"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_key")
}

func TestNewTailscaleProvider_MissingTailnet(t *testing.T) {
	_, err := NewTailscaleProvider(&models.TunnelConfig{}, `{"api_key":"k"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tailnet")
}

func TestNewTailscaleProvider_InvalidJSON(t *testing.T) {
	_, err := NewTailscaleProvider(&models.TunnelConfig{}, "not-json")
	require.Error(t, err)
}

func TestTailscaleFactory_ReturnsProvider(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := Factory(cfg, validTSCredsJSON())
	require.NoError(t, err)
	assert.Equal(t, "tailscale", p.Name())
}

func TestStart_ValidatesAPIKey(t *testing.T) {
	devices := []TailscaleDevice{{ID: "d1"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, tsDevicesResponse(devices)) //nolint:errcheck
	}))
	defer srv.Close()

	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewTailscaleProvider(cfg, validTSCredsJSON())
	require.NoError(t, err)
	p.client.baseURL = srv.URL

	require.NoError(t, p.Start(context.Background()))
	assert.Equal(t, hecate.TunnelStateConnected, p.Status())
}

func TestStart_ErrorOnBadAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewTailscaleProvider(cfg, validTSCredsJSON())
	require.NoError(t, err)
	p.client.baseURL = srv.URL

	err = p.Start(context.Background())
	require.Error(t, err)
	assert.Equal(t, hecate.TunnelStateError, p.Status())
}

func TestStop_TransitionsToStopped(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewTailscaleProvider(cfg, validTSCredsJSON())
	require.NoError(t, err)

	// Force to connected state.
	p.mu.Lock()
	p.state = hecate.TunnelStateConnected
	p.mu.Unlock()

	require.NoError(t, p.Stop())
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
}

func TestGetClient_ReturnsClient(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewTailscaleProvider(cfg, validTSCredsJSON())
	require.NoError(t, err)
	assert.NotNil(t, p.GetClient())
}
