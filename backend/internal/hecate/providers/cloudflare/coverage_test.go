package cloudflare

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDo_EmptyErrorsInFailureResponse covers the "unknown api error" branch in do()
// when the API returns success=false with an empty errors array.
func TestDo_EmptyErrorsInFailureResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":false,"result":null,"errors":[]}`) //nolint:errcheck
	})

	_, err := c.ListTunnels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown api error")
}

// TestListTunnels_UnmarshalResultError covers the json.Unmarshal error path
// in ListTunnels when the API returns a malformed result payload.
func TestListTunnels_UnmarshalResultError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":true,"result":"not-an-array","errors":[]}`) //nolint:errcheck
	})

	_, err := c.ListTunnels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse tunnels")
}

// TestCreateTunnel_UnmarshalResultError covers the json.Unmarshal error path
// in CreateTunnel when the API returns a malformed result payload.
func TestCreateTunnel_UnmarshalResultError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":true,"result":"not-a-tunnel","errors":[]}`) //nolint:errcheck
	})

	_, err := c.CreateTunnel(context.Background(), "my-tunnel")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse tunnel")
}

// TestStop_WhenProcessAlreadyExited covers the "select case <-done" branch in Stop
// when the process has already exited before Stop is called (done is pre-closed).
func TestStop_WhenProcessAlreadyExited(t *testing.T) {
	trueBin, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true binary not available")
	}

	cfg := &models.TunnelConfig{UUID: "uuid"}
	p := &CloudflareTunnelProvider{
		cfg:   cfg,
		buf:   hecate.NewRingBuffer(1000),
		state: hecate.TunnelStateConnected,
	}

	cmd := exec.Command(trueBin) //nolint:gosec
	require.NoError(t, cmd.Start())
	require.NoError(t, cmd.Wait())

	done := make(chan struct{})
	close(done)

	p.mu.Lock()
	p.cmd = cmd
	p.done = done
	p.mu.Unlock()

	require.NoError(t, p.Stop())
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
}

// TestStart_ExecFormatError covers the cmd.Start() error block in Start when the
// binary exists on disk with execute permission but is not a valid executable format.
func TestStart_ExecFormatError(t *testing.T) {
	f, err := os.CreateTemp("", "fake-cloudflared-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	_, err = f.WriteString("this is not a valid executable binary\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, os.Chmod(f.Name(), 0o755)) //nolint:gosec

	cfg := &models.TunnelConfig{UUID: "uuid", Name: "test"}
	p, err := NewCloudflareProvider(cfg, validCredsJSON())
	require.NoError(t, err)
	p.binaryPath = f.Name()

	err = p.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cloudflared")
	assert.Equal(t, hecate.TunnelStateError, p.Status())
}

// TestStart_CapturesStdoutOutput covers the p.buf.Write branch in the stdout
// scanner goroutine by using a binary that writes to stdout before exiting.
func TestStart_CapturesStdoutOutput(t *testing.T) {
	echoBin, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo binary not available")
	}

	cfg := &models.TunnelConfig{UUID: "uuid", Name: "test"}
	p, err := NewCloudflareProvider(cfg, validCredsJSON())
	require.NoError(t, err)
	p.binaryPath = echoBin

	require.NoError(t, p.Start(context.Background()))

	p.mu.RLock()
	done := p.done
	p.mu.RUnlock()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for echo process to exit")
	}

	// Give scanner goroutines a brief window to drain the pipe before asserting.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(p.buf.ReadAll()) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	assert.NotEmpty(t, p.buf.ReadAll(), "stdout scanner goroutine must have written output to the ring buffer")
}

// TestGenerateCloudflaredConfig_MultipleIngressRules exercises the happy path
// with a non-nil rules slice to confirm the catch-all follows user-defined rules.
func TestGenerateCloudflaredConfig_NilRules(t *testing.T) {
	out, err := GenerateCloudflaredConfig("tid", "/creds.json", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "tid")
	assert.Contains(t, out, "http_status:404")
	assert.Contains(t, out, "/creds.json")
}

// TestStop_WithNilDone covers the "cmd != nil but done == nil" guard in Stop.
func TestStop_WithNilDone(t *testing.T) {
	p := &CloudflareTunnelProvider{
		buf:   hecate.NewRingBuffer(100),
		state: hecate.TunnelStateConnected,
	}
	trueBin, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true binary not available")
	}
	cmd := exec.Command(trueBin) //nolint:gosec
	require.NoError(t, cmd.Start())
	require.NoError(t, cmd.Wait())

	p.mu.Lock()
	p.cmd = cmd
	p.done = nil
	p.mu.Unlock()

	require.NoError(t, p.Stop())
	assert.Equal(t, hecate.TunnelStateStopped, p.Status())
}

// TestCloudflareAPIError_ErrorMethod verifies CloudflareAPIError.Error() formatting.
// This is a direct unit test of the Error() method on the exported type.
func TestCloudflareAPIError_MultipleFormats(t *testing.T) {
	tests := []struct {
		code    int
		msg     string
		wantFmt string
	}{
		{1003, "Authorization required", "cloudflare api error 1003: Authorization required"},
		{0, "", "cloudflare api error 0: "},
		{9999, "some long error message", "cloudflare api error 9999: some long error message"},
	}
	for _, tc := range tests {
		e := &CloudflareAPIError{Code: tc.code, Message: tc.msg}
		assert.Equal(t, tc.wantFmt, e.Error())
	}
}

// TestListTunnels_RequestBuildError covers the http request build failure in do()
// by using an invalid base URL that causes http.NewRequestWithContext to fail.
func TestListTunnels_RequestBuildError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	c := NewCloudflareClient("tok", "acc")
	// An invalid base URL with a control character causes request construction to fail.
	c.baseURL = "http://\x00invalid"

	_, err := c.ListTunnels(context.Background())
	require.Error(t, err)
}
