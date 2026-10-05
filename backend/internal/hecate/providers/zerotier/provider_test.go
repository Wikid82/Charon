package zerotier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

// newTestZTClient creates a ZeroTierClient pointing to a test HTTP server,
// bypassing address validation (the server uses a loopback address).
//
//nolint:unparam // *httptest.Server returned for future test variants
func newTestZTClient(t *testing.T, handler http.HandlerFunc) (*ZeroTierClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	// httptest listens on loopback, so the production safe client is bypassed.
	c, err := newZeroTierClientWithHTTP(context.Background(), "test-token", srv.URL, srv.Client())
	require.NoError(t, err)
	return c, srv
}

// --- API Client Tests ---

func TestListNetworks_Success(t *testing.T) {
	networks := []ZeroTierNetwork{
		{ID: "a1b2c3d4e5f60718", Name: "my-network", Private: true},
	}
	c, _ := newTestZTClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header.
		assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "/api/v1/network", r.URL.Path)
		json.NewEncoder(w).Encode(networks) //nolint:errcheck,gosec
	})

	result, err := c.ListNetworks(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "a1b2c3d4e5f60718", result[0].ID)
	assert.True(t, result[0].Private)
}

func TestListNetworks_AuthError(t *testing.T) {
	c, _ := newTestZTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"not authorized"}`) //nolint:errcheck
	})

	_, err := c.ListNetworks(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestListMembers_Success(t *testing.T) {
	members := []ZeroTierMember{
		{ID: "m1", Name: "node-1", IPAssignments: []string{"10.147.17.1"}, Online: true},
	}
	c, _ := newTestZTClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/network/a1b2c3d4e5f60718/member", r.URL.Path)
		json.NewEncoder(w).Encode(members) //nolint:errcheck,gosec,gosec
	})

	result, err := c.ListMembers(context.Background(), "a1b2c3d4e5f60718")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "m1", result[0].ID)
	assert.Equal(t, "10.147.17.1", result[0].IPAssignments[0])
}

func TestListMembers_NetworkNotFound(t *testing.T) {
	c, _ := newTestZTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.ListMembers(context.Background(), "0123456789abcdef")
	require.Error(t, err)
}

// --- Provider Tests ---

//nolint:unparam // controllerURL kept for future test variants
func validZTCredsJSON(controllerURL string) string {
	if controllerURL == "" {
		return `{"api_token":"zt-test-token"}`
	}
	return fmt.Sprintf(`{"api_token":"zt-test-token","controller_url":%q}`, controllerURL)
}

func TestNewZeroTierProvider_ParsesCredentials(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "zt-uuid", Name: "zt-test"}
	p, err := NewZeroTierProvider(cfg, validZTCredsJSON(""))
	require.NoError(t, err)
	assert.Equal(t, "zerotier", p.Name())
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
	assert.Empty(t, p.GetAddress())
}

func TestNewZeroTierProvider_MissingAPIToken(t *testing.T) {
	_, err := NewZeroTierProvider(&models.TunnelConfig{}, `{"controller_url":"https://x.com"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_token")
}

func TestNewZeroTierProvider_InvalidJSON(t *testing.T) {
	_, err := NewZeroTierProvider(&models.TunnelConfig{}, "not-json")
	require.Error(t, err)
}

func TestZeroTierFactory_ReturnsProvider(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := Factory(cfg, validZTCredsJSON(""))
	require.NoError(t, err)
	assert.Equal(t, "zerotier", p.Name())
}

func TestStart_ValidatesToken(t *testing.T) {
	networks := []ZeroTierNetwork{{ID: "n1"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(networks) //nolint:errcheck,gosec
	}))
	defer srv.Close()

	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewZeroTierProvider(cfg, validZTCredsJSON(""))
	require.NoError(t, err)

	// Inject factory that produces a test client bypassing SSRF.
	p.newClientFn = func(ctx context.Context, apiToken, controllerURL string) (*ZeroTierClient, error) {
		return newZeroTierClientWithHTTP(context.Background(), apiToken, srv.URL, srv.Client())
	}

	require.NoError(t, p.Start(context.Background()))
	assert.Equal(t, hecate.TunnelStateConnected, p.Status())
	assert.NotNil(t, p.GetClient())
}

func TestStart_ErrorWhenClientCreationFails(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewZeroTierProvider(cfg, validZTCredsJSON(""))
	require.NoError(t, err)

	p.newClientFn = func(ctx context.Context, apiToken, controllerURL string) (*ZeroTierClient, error) {
		return nil, fmt.Errorf("injected factory error")
	}

	err = p.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create client")
	assert.Equal(t, hecate.TunnelStateError, p.Status())
}

func TestStart_ErrorOnInvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewZeroTierProvider(cfg, validZTCredsJSON(""))
	require.NoError(t, err)

	p.newClientFn = func(ctx context.Context, apiToken, controllerURL string) (*ZeroTierClient, error) {
		return newZeroTierClientWithHTTP(context.Background(), "bad-token", srv.URL, srv.Client())
	}

	err = p.Start(context.Background())
	require.Error(t, err)
	assert.Equal(t, hecate.TunnelStateError, p.Status())
}

func TestStop_TransitionsToStopped(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewZeroTierProvider(cfg, validZTCredsJSON(""))
	require.NoError(t, err)

	p.mu.Lock()
	p.state = hecate.TunnelStateConnected
	p.mu.Unlock()

	require.NoError(t, p.Stop())
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
}

func TestGetClient_NilBeforeStart(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewZeroTierProvider(cfg, validZTCredsJSON(""))
	require.NoError(t, err)
	assert.Nil(t, p.GetClient())
}
