package server

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/config"
)

// gatedStopEngine is a CrowdsecExecutor whose Stop waits until the gate is opened, and which records that it ran.
type gatedStopEngine struct {
	gate    chan struct{}
	open    sync.Once
	mu      sync.Mutex
	stopped bool
}

func newGatedStopEngine() *gatedStopEngine {
	return &gatedStopEngine{gate: make(chan struct{})}
}

func (e *gatedStopEngine) openGate() {
	e.open.Do(func() { close(e.gate) })
}

func (e *gatedStopEngine) Start(context.Context, string, string) (int, error) {
	return 0, nil
}

func (e *gatedStopEngine) Stop(context.Context, string) error {
	<-e.gate
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopped = true
	return nil
}

func (e *gatedStopEngine) Status(context.Context, string) (running bool, pid int, err error) {
	return false, 0, nil
}

func (e *gatedStopEngine) wasStopped() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stopped
}

// TestEmergencyServer_ResetReplyDoesNotWaitForEngineStop: the Tier 2 reset answers before its engine stop finishes,
// which matters because the Tier 2 write timeout is shorter than a slow stop. The stop then runs on the same
// lifecycle the API uses, and the wait for it ends once it is done.
func TestEmergencyServer_ResetReplyDoesNotWaitForEngineStop(t *testing.T) {
	db := setupTestDB(t)
	emergencyToken := "test-emergency-token-for-testing-32chars"
	require.NoError(t, os.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_EMERGENCY_TOKEN")) }()

	engine := newGatedStopEngine()
	defer engine.openGate()
	lifecycle := handlers.NewCrowdsecHandler(db, engine, "/bin/false", t.TempDir(), nil)

	cfg := config.EmergencyConfig{Enabled: true, BindAddress: "127.0.0.1:0"}
	server := NewEmergencyServer(db, cfg).WithCrowdsecLifecycle(lifecycle)
	require.NoError(t, server.Start())
	defer func() { _ = server.Stop(context.Background()) }()
	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest(http.MethodPost, "http://"+server.GetAddr()+"/emergency/security-reset", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("X-Emergency-Token", emergencyToken)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Do(req)
	require.NoError(t, err, "the reply must arrive while the engine stop is still gated")
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.False(t, engine.wasStopped(), "the engine stop runs after the reply")

	engine.openGate()
	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, lifecycle.WaitResetStops(waitCtx))
	require.True(t, engine.wasStopped())
}
