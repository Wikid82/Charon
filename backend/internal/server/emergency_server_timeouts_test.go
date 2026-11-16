package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	charonlogger "github.com/Wikid82/charon/backend/internal/logger"
)

// delayedApply is a CaddyConfigManager whose apply takes d to finish.
type delayedApply struct{ d time.Duration }

func (a delayedApply) ApplyConfig(context.Context) error {
	time.Sleep(a.d)
	return nil
}

// TestEmergencyServer_ResetReplyOutlivesServerWriteTimeout: the Tier 2 write timeout is shortened below the reset's
// apply time. The reset extends its own write deadline, so the reply still arrives with 200.
func TestEmergencyServer_ResetReplyOutlivesServerWriteTimeout(t *testing.T) {
	db := setupTestDB(t)
	emergencyToken := "test-emergency-token-for-testing-32chars"
	require.NoError(t, os.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_EMERGENCY_TOKEN")) }()

	previous := emergencyWriteTimeout
	emergencyWriteTimeout = 200 * time.Millisecond
	defer func() { emergencyWriteTimeout = previous }()

	cfg := config.EmergencyConfig{Enabled: true, BindAddress: "127.0.0.1:0"}
	server := NewEmergencyServerWithDeps(db, cfg, delayedApply{d: 600 * time.Millisecond}, nil, nil)
	require.NoError(t, server.Start())
	defer func() { _ = server.Stop(context.Background()) }()
	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest(http.MethodPost, "http://"+server.GetAddr()+"/emergency/security-reset", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("X-Emergency-Token", emergencyToken)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	require.NoError(t, err, "the reply must not be cut off by the server write timeout")
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewEmergencyHTTPServer_KeepsTimeouts(t *testing.T) {
	srv := newEmergencyHTTPServer(http.NewServeMux())
	assert.Equal(t, 5*time.Second, srv.ReadHeaderTimeout)
	assert.Equal(t, 10*time.Second, srv.ReadTimeout)
	assert.Equal(t, 10*time.Second, srv.WriteTimeout, "routes other than the reset keep the 10 s write timeout")
	assert.Equal(t, 30*time.Second, srv.IdleTimeout)
}

func TestIsLoopbackBind(t *testing.T) {
	tests := []struct {
		address string
		want    bool
	}{
		{"127.0.0.1:2020", true},
		{"[::1]:2020", true},
		{"localhost:2020", true},
		{"0.0.0.0:2020", false},
		{":2020", false},
		{"[::]:2020", false},
		{"192.168.1.5:2020", false},
		{"not-an-address", false},
	}
	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			assert.Equal(t, tt.want, isLoopbackBind(tt.address))
		})
	}
}

func TestWarnIfBeyondLoopback(t *testing.T) {
	var buf bytes.Buffer
	charonlogger.Init(false, &buf)
	t.Cleanup(func() { charonlogger.Init(false, io.Discard) })

	buf.Reset()
	assert.False(t, warnIfBeyondLoopback("127.0.0.1:2020"))
	assert.NotContains(t, buf.String(), "beyond loopback")

	buf.Reset()
	assert.True(t, warnIfBeyondLoopback("0.0.0.0:2020"))
	assert.Contains(t, buf.String(), "beyond loopback")
}
