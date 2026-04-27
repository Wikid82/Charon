package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// --- API Client Tests ---

func newTestClient(t *testing.T, handler http.HandlerFunc) (*CloudflareClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewCloudflareClient("test-token", "test-account")
	c.baseURL = srv.URL
	return c, srv
}

func cfSuccessResponse(t *testing.T, result any) string {
	t.Helper()
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	resp := map[string]any{
		"success": true,
		"result":  json.RawMessage(raw),
		"errors":  []any{},
	}
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	return string(b)
}

func cfErrorResponse(code int, msg string) string {
	return fmt.Sprintf(`{"success":false,"result":null,"errors":[{"code":%d,"message":%q}]}`, code, msg)
}

func TestListTunnels_Success(t *testing.T) {
	tunnels := []CloudflareTunnel{
		{ID: "abc", Name: "my-tunnel", Status: "active", CreatedAt: time.Now()},
	}
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/cfd_tunnel")
		fmt.Fprint(w, cfSuccessResponse(t, tunnels)) //nolint:errcheck
	})

	result, err := c.ListTunnels(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "abc", result[0].ID)
	assert.Equal(t, "my-tunnel", result[0].Name)
}

func TestListTunnels_APIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, cfErrorResponse(1003, "Authorization required")) //nolint:errcheck
	})

	_, err := c.ListTunnels(context.Background())
	require.Error(t, err)
	var cfErr *CloudflareAPIError
	assert.ErrorAs(t, err, &cfErr)
	assert.Equal(t, 1003, cfErr.Code)
	assert.Contains(t, cfErr.Message, "Authorization")
}

func TestCreateTunnel_Success(t *testing.T) {
	created := CloudflareTunnel{ID: "new-id", Name: "new-tunnel", Status: "inactive"}
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "new-tunnel", body["name"])
		assert.NotEmpty(t, body["tunnel_secret"])
		fmt.Fprint(w, cfSuccessResponse(t, created)) //nolint:errcheck
	})

	result, err := c.CreateTunnel(context.Background(), "new-tunnel")
	require.NoError(t, err)
	assert.Equal(t, "new-id", result.ID)
}

func TestCreateTunnel_DuplicateNameError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, cfErrorResponse(1009, "Tunnel already exists with same name")) //nolint:errcheck
	})

	_, err := c.CreateTunnel(context.Background(), "duplicate")
	require.Error(t, err)
	var cfErr *CloudflareAPIError
	require.ErrorAs(t, err, &cfErr)
	assert.Equal(t, 1009, cfErr.Code)
}

func TestDeleteTunnel_Success(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Contains(t, r.URL.Path, "/tunnel-id-123")
		fmt.Fprint(w, cfSuccessResponse(t, nil)) //nolint:errcheck
	})

	err := c.DeleteTunnel(context.Background(), "tunnel-id-123")
	require.NoError(t, err)
}

func TestDeleteTunnel_NotFoundError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, cfErrorResponse(1100, "Tunnel not found")) //nolint:errcheck
	})

	err := c.DeleteTunnel(context.Background(), "missing-id")
	require.Error(t, err)
	var cfErr *CloudflareAPIError
	require.ErrorAs(t, err, &cfErr)
	assert.Equal(t, 1100, cfErr.Code)
}

// --- Config Generator Tests ---

func TestGenerateCloudflaredConfig_ValidYAML(t *testing.T) {
	rules := []IngressRule{
		{Hostname: "example.com", Service: "http://localhost:8080"},
	}
	output, err := GenerateCloudflaredConfig("my-tunnel-id", "/etc/cloudflared/creds.json", rules)
	require.NoError(t, err)

	var parsed CloudflaredConfig
	require.NoError(t, yaml.Unmarshal([]byte(output), &parsed))

	assert.Equal(t, "my-tunnel-id", parsed.Tunnel)
	assert.Equal(t, "/etc/cloudflared/creds.json", parsed.CredentialsFile)
	require.Len(t, parsed.Ingress, 2)
	assert.Equal(t, "example.com", parsed.Ingress[0].Hostname)
	assert.Equal(t, "http://localhost:8080", parsed.Ingress[0].Service)
	// Last rule must be the catch-all.
	assert.Empty(t, parsed.Ingress[1].Hostname)
	assert.Equal(t, "http_status:404", parsed.Ingress[1].Service)
}

func TestGenerateCloudflaredConfig_CatchAllAlwaysAppended(t *testing.T) {
	output, err := GenerateCloudflaredConfig("tid", "", nil)
	require.NoError(t, err)
	assert.Contains(t, output, "http_status:404")
}

func TestGenerateCloudflaredConfig_EmptyTunnelIDError(t *testing.T) {
	_, err := GenerateCloudflaredConfig("", "/path", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tunnelID is required")
}

// --- Provider Tests ---

func validCredsJSON() string {
	return `{"api_token":"tok","account_id":"acc","tunnel_token":"run-token"}`
}

func TestNewCloudflareProvider_ParsesCredentials(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "test-uuid", Name: "test"}
	p, err := NewCloudflareProvider(cfg, validCredsJSON())
	require.NoError(t, err)
	assert.Equal(t, "cloudflare", p.Name())
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
	assert.Empty(t, p.GetAddress())
}

func TestNewCloudflareProvider_MissingTunnelToken(t *testing.T) {
	cfg := &models.TunnelConfig{}
	_, err := NewCloudflareProvider(cfg, `{"api_token":"tok","account_id":"acc"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tunnel_token")
}

func TestNewCloudflareProvider_InvalidJSON(t *testing.T) {
	cfg := &models.TunnelConfig{}
	_, err := NewCloudflareProvider(cfg, "not-json")
	require.Error(t, err)
}

func TestFactory_ReturnsProvider(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := Factory(cfg, validCredsJSON())
	require.NoError(t, err)
	assert.Equal(t, "cloudflare", p.Name())
}

func TestStart_BinaryNotFound(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewCloudflareProvider(cfg, validCredsJSON())
	require.NoError(t, err)
	p.binaryPath = "definitely-not-a-real-binary-xyz"

	err = p.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStart_WithStubBinary(t *testing.T) {
	// Use /bin/true as the cloudflared stub — exits immediately with code 0.
	trueBin, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true binary not available")
	}

	cfg := &models.TunnelConfig{UUID: "uuid", Name: "test"}
	p, err := NewCloudflareProvider(cfg, validCredsJSON())
	require.NoError(t, err)
	p.binaryPath = trueBin

	require.NoError(t, p.Start(context.Background()))

	// Immediately after Start() returns, state is Connected.
	// The process may exit right away; wait for done to be closed.
	p.mu.RLock()
	done := p.done
	p.mu.RUnlock()

	select {
	case <-done:
		// Process exited normally. State should be error (process died).
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for stub process to exit")
	}

	// After the process exits, the monitor goroutine sets state to Error
	// (since it was not Stopped by Stop()).
	assert.Equal(t, hecate.TunnelStateError, p.Status())
}

func TestStop_Idempotent_WhenNotStarted(t *testing.T) {
	cfg := &models.TunnelConfig{UUID: "uuid"}
	p, err := NewCloudflareProvider(cfg, validCredsJSON())
	require.NoError(t, err)

	err = p.Stop()
	require.NoError(t, err)
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
}

func TestStop_SendsSIGTERM(t *testing.T) {
	sleepBin, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep binary not available")
	}

	cfg := &models.TunnelConfig{UUID: "uuid", Name: "test"}
	// Build a provider but inject a sleep process directly to simulate a running cloudflared.
	p := &CloudflareTunnelProvider{
		cfg:   cfg,
		buf:   hecate.NewRingBuffer(1000),
		state: hecate.TunnelStateStopped,
	}

	// Start a sleep process to act as the cloudflared stub.
	cmd := exec.Command(sleepBin, "30") //nolint:gosec
	require.NoError(t, cmd.Start())
	done := make(chan struct{})
	p.mu.Lock()
	p.cmd = cmd
	p.done = done
	p.state = hecate.TunnelStateConnected
	p.mu.Unlock()

	// Monitor process exit.
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	start := time.Now()
	require.NoError(t, p.Stop())
	elapsed := time.Since(start)

	// Should have terminated well under 10 seconds via SIGTERM.
	assert.Less(t, elapsed, 5*time.Second)
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
}

func TestAPIError_ErrorMethod(t *testing.T) {
	e := &CloudflareAPIError{Code: 7003, Message: "No route for the URI"}
	assert.Equal(t, "cloudflare api error 7003: No route for the URI", e.Error())
}

func TestListTunnels_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error") //nolint:errcheck
	}))
	defer srv.Close()

	c := NewCloudflareClient("tok", "acc")
	c.baseURL = srv.URL

	_, err := c.ListTunnels(context.Background())
	require.Error(t, err)
}

func TestListTunnels_SuccessWithEmptyResult(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":true,"result":[],"errors":[]}`) //nolint:errcheck
	})
	c, _ := newTestClient(t, handler)

	result, err := c.ListTunnels(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGenerateConfig_MultipleRules(t *testing.T) {
	rules := []IngressRule{
		{Hostname: "a.example.com", Service: "http://localhost:3000"},
		{Hostname: "b.example.com", Service: "http://localhost:4000"},
	}
	out, err := GenerateCloudflaredConfig("tunnel-xyz", "/creds.json", rules)
	require.NoError(t, err)

	assert.Contains(t, out, "a.example.com")
	assert.Contains(t, out, "b.example.com")

	// Verify catch-all is last.
	idx404 := strings.LastIndex(out, "http_status:404")
	idxB := strings.LastIndex(out, "b.example.com")
	assert.Greater(t, idx404, idxB, "catch-all must appear after all hostname rules")
}
