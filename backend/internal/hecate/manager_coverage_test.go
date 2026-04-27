package hecate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errorStopProvider wraps mockProvider but returns an error from Stop.
type errorStopProvider struct {
	*mockProvider
}

func (e *errorStopProvider) Stop() error {
	return errors.New("stop failed")
}

func TestTunnelManager_NewTunnelManager_Initialized(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	assert.NotNil(t, mgr.db)
	assert.NotNil(t, mgr.encSvc)
	assert.NotNil(t, mgr.factories)
	assert.NotNil(t, mgr.state)
	assert.NotNil(t, mgr.ctx)
}

func TestTunnelManager_RegisterProvider_UpdatesExistingInstance(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	p1 := newMockProvider("first")
	mgr.RegisterProvider("uuid-update", p1)

	p2 := newMockProvider("second")
	mgr.RegisterProvider("uuid-update", p2)

	mgr.mu.RLock()
	ps := mgr.state["uuid-update"]
	mgr.mu.RUnlock()

	require.NotNil(t, ps)
	assert.Equal(t, p2, ps.instance)
}

func TestTunnelManager_Stop_WithProviderStopError_ReturnsError(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)

	p := &errorStopProvider{mockProvider: newMockProvider("err-stop")}
	mgr.RegisterProvider("uuid-err", p)

	err := mgr.Stop()
	assert.Error(t, err)
}

func TestTunnelManager_Start_EmptyDB_ReturnsNil(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	err := mgr.Start(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, mgr.GetStatus())
}

func TestTunnelManager_Start_DBClosed_ReturnsError(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = mgr.Start(context.Background())
	assert.Error(t, err)
}

func TestTunnelManager_Start_ActiveConfig_FactoryFails_LogsWarning(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	mgr.RegisterFactory(models.ProviderCloudflare, func(_ *models.TunnelConfig, _ string) (TunnelProvider, error) {
		return nil, errors.New("factory boom")
	})

	creds, err := encSvc.Encrypt([]byte(`{"api_token":"test"}`))
	require.NoError(t, err)

	cfg := models.TunnelConfig{
		Name:                 "warn-tunnel",
		Provider:             models.ProviderCloudflare,
		EncryptedCredentials: creds,
		IsActive:             true,
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Start logs a warning for the failed tunnel but returns nil.
	err = mgr.Start(context.Background())
	assert.NoError(t, err)
}

func TestTunnelManager_StartTunnel_UUIDNotInDB_ReturnsError(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	err := mgr.StartTunnel("nonexistent-uuid")
	assert.Error(t, err)
}

func TestTunnelManager_StartTunnel_AlreadyConnected_ReturnsNil(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	creds, err := encSvc.Encrypt([]byte(`{"api_token":"test"}`))
	require.NoError(t, err)

	cfg := models.TunnelConfig{
		Name:                 "running-tunnel",
		Provider:             models.ProviderCloudflare,
		EncryptedCredentials: creds,
		IsActive:             true,
	}
	require.NoError(t, db.Create(&cfg).Error)

	p := newMockProvider("connected")
	p.mu.Lock()
	p.state = TunnelStateConnected
	p.mu.Unlock()

	mgr.mu.Lock()
	watchCtx, cancelFn := context.WithCancel(mgr.ctx) //nolint:gosec // G118: cancelFn stored in tunnelState.cancel and called during cleanup
	mgr.state[cfg.UUID] = &tunnelState{
		uuid:     cfg.UUID,
		instance: p,
		buffer:   NewRingBuffer(100),
		cancel:   cancelFn,
	}
	mgr.mu.Unlock()
	go mgr.runWatcher(watchCtx, cfg.UUID)

	err = mgr.StartTunnel(cfg.UUID)
	assert.NoError(t, err)
}

func TestTunnelManager_StartTunnel_AlreadyConnecting_ReturnsNil(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	creds, err := encSvc.Encrypt([]byte(`{"api_token":"test"}`))
	require.NoError(t, err)

	cfg := models.TunnelConfig{
		Name:                 "connecting-tunnel",
		Provider:             models.ProviderCloudflare,
		EncryptedCredentials: creds,
		IsActive:             true,
	}
	require.NoError(t, db.Create(&cfg).Error)

	p := newMockProvider("connecting")
	p.mu.Lock()
	p.state = TunnelStateConnecting
	p.mu.Unlock()

	mgr.mu.Lock()
	watchCtx, cancelFn := context.WithCancel(mgr.ctx) //nolint:gosec // G118: cancelFn stored in tunnelState.cancel and called during cleanup
	mgr.state[cfg.UUID] = &tunnelState{
		uuid:     cfg.UUID,
		instance: p,
		buffer:   NewRingBuffer(100),
		cancel:   cancelFn,
	}
	mgr.mu.Unlock()
	go mgr.runWatcher(watchCtx, cfg.UUID)

	err = mgr.StartTunnel(cfg.UUID)
	assert.NoError(t, err)
}

func TestTunnelManager_StartTunnel_FactoryReturnsError(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	mgr.RegisterFactory(models.ProviderCloudflare, func(_ *models.TunnelConfig, _ string) (TunnelProvider, error) {
		return nil, errors.New("factory error")
	})

	creds, err := encSvc.Encrypt([]byte(`{"api_token":"test"}`))
	require.NoError(t, err)

	cfg := models.TunnelConfig{
		Name:                 "factory-err",
		Provider:             models.ProviderCloudflare,
		EncryptedCredentials: creds,
		IsActive:             true,
	}
	require.NoError(t, db.Create(&cfg).Error)

	err = mgr.StartTunnel(cfg.UUID)
	assert.Error(t, err)
}

func TestTunnelManager_StartTunnel_ProviderStartError(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	p := newMockProvider("start-err")
	p.startErr = errors.New("start failed")

	mgr.RegisterFactory(models.ProviderCloudflare, func(_ *models.TunnelConfig, _ string) (TunnelProvider, error) {
		return p, nil
	})

	creds, err := encSvc.Encrypt([]byte(`{"api_token":"test"}`))
	require.NoError(t, err)

	cfg := models.TunnelConfig{
		Name:                 "start-err-tunnel",
		Provider:             models.ProviderCloudflare,
		EncryptedCredentials: creds,
		IsActive:             true,
	}
	require.NoError(t, db.Create(&cfg).Error)

	err = mgr.StartTunnel(cfg.UUID)
	assert.Error(t, err)
}

func TestTunnelManager_StopTunnel_ProviderStopError_ReturnsError(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	p := &errorStopProvider{mockProvider: newMockProvider("err-stop-tunnel")}
	mgr.RegisterProvider("uuid-stop-err", p)

	err := mgr.StopTunnel("uuid-stop-err")
	assert.Error(t, err)
}

func TestTunnelManager_GetStatus_EmptyWhenNoTunnels(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	statuses := mgr.GetStatus()
	assert.Empty(t, statuses)
}

func TestTunnelManager_StartTunnel_DecryptError_ReturnsError(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	// Create a config with corrupted (non-decryptable) credentials directly in DB.
	cfg := models.TunnelConfig{
		Name:                 "bad-creds",
		Provider:             models.ProviderCloudflare,
		EncryptedCredentials: "not-valid-ciphertext",
		IsActive:             true,
	}
	require.NoError(t, db.Create(&cfg).Error)

	err := mgr.StartTunnel(cfg.UUID)
	assert.Error(t, err)
}

// TestTunnelManager_RunWatcher_TickerFires_StateGone verifies that runWatcher
// exits cleanly when the tunnel state entry is removed before the first tick.
// This test waits for the 5-second ticker and runs in parallel to avoid
// blocking the overall test suite.
func TestTunnelManager_RunWatcher_TickerFires_StateGone(t *testing.T) {
	t.Parallel()

	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	watchCtx, cancelFn := context.WithCancel(mgr.ctx)
	defer cancelFn()

	// Start runWatcher for a uuid that is NOT in m.state.
	// On the first tick it will find !ok and return.
	done := make(chan struct{})
	go func() {
		defer close(done)
		mgr.runWatcher(watchCtx, "ghost-tunnel-uuid")
	}()

	select {
	case <-done:
		// runWatcher exited after the ticker fired and found no state entry.
	case <-time.After(7 * time.Second):
		t.Fatal("runWatcher did not exit within 7 seconds")
	}
}
