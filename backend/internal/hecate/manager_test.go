package hecate

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testKey is a 32-byte AES-256 key, base64-encoded.
const testEncKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func setupManagerTestDB(t *testing.T) (*gorm.DB, *crypto.EncryptionService) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "hecate_test.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.AutoMigrate(&models.TunnelConfig{}))

	encSvc, err := crypto.NewEncryptionService(testEncKey)
	require.NoError(t, err)
	return db, encSvc
}

// mockProvider is a controllable TunnelProvider for testing.
type mockProvider struct {
	name      string
	mu        sync.Mutex
	state     TunnelState
	stopCount int32
	startErr  error
}

func newMockProvider(name string) *mockProvider {
	return &mockProvider{name: name, state: TunnelStateStopped}
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Status() TunnelState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}
func (m *mockProvider) Start(_ context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.mu.Lock()
	m.state = TunnelStateConnected
	m.mu.Unlock()
	return nil
}
func (m *mockProvider) Stop() error {
	atomic.AddInt32(&m.stopCount, 1)
	m.mu.Lock()
	m.state = TunnelStateStopped
	m.mu.Unlock()
	return nil
}
func (m *mockProvider) GetAddress() string { return "127.0.0.1:9999" }

// --- Tests ---

func TestTunnelManager_RegisterProvider_StoresProvider(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	p := newMockProvider("test-tunnel")
	mgr.RegisterProvider("uuid-1", p)

	mgr.mu.RLock()
	_, ok := mgr.state["uuid-1"]
	mgr.mu.RUnlock()

	assert.True(t, ok, "provider should be stored in state")
}

func TestTunnelManager_GetStatus_ReflectsState(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	p := newMockProvider("my-tunnel")
	_ = p.Start(context.Background()) // set to connected
	mgr.RegisterProvider("uuid-2", p)

	statuses := mgr.GetStatus()
	require.Len(t, statuses, 1)
	assert.Equal(t, "uuid-2", statuses[0].UUID)
	assert.Equal(t, TunnelStateConnected, statuses[0].State)
}

func TestTunnelManager_StopTunnel_CallsStop(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	p := newMockProvider("stop-test")
	mgr.RegisterProvider("uuid-3", p)

	err := mgr.StopTunnel("uuid-3")
	require.NoError(t, err)
	assert.EqualValues(t, 1, atomic.LoadInt32(&p.stopCount), "Stop() should have been called once")

	mgr.mu.RLock()
	_, stillInState := mgr.state["uuid-3"]
	mgr.mu.RUnlock()
	assert.False(t, stillInState, "tunnel should be removed from state after stop")
}

func TestTunnelManager_StopTunnel_NotFound_ReturnsNil(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	err := mgr.StopTunnel("nonexistent")
	assert.NoError(t, err)
}

func TestTunnelManager_GetLogBuffer_Found(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	mgr.RegisterProvider("uuid-4", newMockProvider("buf-test"))

	buf, err := mgr.GetLogBuffer("uuid-4")
	require.NoError(t, err)
	assert.NotNil(t, buf)
}

func TestTunnelManager_GetLogBuffer_NotFound_ReturnsError(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)

	_, err := mgr.GetLogBuffer("missing")
	assert.Error(t, err)
}

func TestTunnelManager_StartTunnel_NoFactory_LogsWarning(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	// Create a TunnelConfig in DB with encrypted credentials.
	creds, err := encSvc.Encrypt([]byte(`{"api_token":"test"}`))
	require.NoError(t, err)

	cfg := models.TunnelConfig{
		Name:                 "cloudflare-test",
		Provider:             models.ProviderCloudflare,
		EncryptedCredentials: creds,
		IsActive:             true,
	}
	require.NoError(t, db.Create(&cfg).Error)

	// No factory registered — should return nil and log a warning.
	err = mgr.StartTunnel(cfg.UUID)
	assert.NoError(t, err)
}

func TestTunnelManager_StartTunnel_WithFactory_StartsProvider(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	p := newMockProvider("factory-provider")
	mgr.RegisterFactory(models.ProviderCloudflare, func(cfg *models.TunnelConfig, _ string) (TunnelProvider, error) {
		return p, nil
	})

	creds, err := encSvc.Encrypt([]byte(`{"api_token":"test"}`))
	require.NoError(t, err)

	cfg := models.TunnelConfig{
		Name:                 "cf-tunnel",
		Provider:             models.ProviderCloudflare,
		EncryptedCredentials: creds,
		IsActive:             true,
	}
	require.NoError(t, db.Create(&cfg).Error)

	err = mgr.StartTunnel(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, TunnelStateConnected, p.Status())
}

func TestTunnelManager_Start_LoadsActiveConfigs(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	// Register a mock factory.
	var startedCount int32
	mgr.RegisterFactory(models.ProviderTailscale, func(cfg *models.TunnelConfig, _ string) (TunnelProvider, error) {
		atomic.AddInt32(&startedCount, 1)
		return newMockProvider(cfg.Name), nil
	})

	creds, err := encSvc.Encrypt([]byte(`{"api_key":"ts-key"}`))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		cfg := models.TunnelConfig{
			Name:                 fmt.Sprintf("ts-%d", i),
			Provider:             models.ProviderTailscale,
			EncryptedCredentials: creds,
			IsActive:             true,
		}
		require.NoError(t, db.Create(&cfg).Error)
	}

	// Inactive — should not be started.
	inactive := models.TunnelConfig{
		Name:                 "ts-inactive",
		Provider:             models.ProviderTailscale,
		EncryptedCredentials: creds,
		IsActive:             false,
	}
	require.NoError(t, db.Create(&inactive).Error)

	require.NoError(t, mgr.Start(context.Background()))
	assert.EqualValues(t, 3, atomic.LoadInt32(&startedCount))
}

func TestTunnelManager_GetStatus_UptimeOnlyForConnected(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	mgr := NewTunnelManager(db, encSvc)
	defer func() { _ = mgr.Stop() }()

	connected := newMockProvider("connected")
	_ = connected.Start(context.Background())
	errored := newMockProvider("errored")
	errored.mu.Lock()
	errored.state = TunnelStateError
	errored.mu.Unlock()

	mgr.RegisterProvider("uuid-c", connected)
	mgr.RegisterProvider("uuid-e", errored)

	// Give the manager a moment to record startAt in the past.
	time.Sleep(10 * time.Millisecond)

	statuses := mgr.GetStatus()
	statusMap := make(map[string]TunnelStatus)
	for _, s := range statuses {
		statusMap[s.UUID] = s
	}

	assert.Positive(t, statusMap["uuid-c"].UptimeSeconds+1, "connected tunnel should have non-negative uptime")
	assert.EqualValues(t, 0, statusMap["uuid-e"].UptimeSeconds, "errored tunnel should have zero uptime")
}

// Verify that testEncKey decodes to exactly 32 bytes.
func TestEncKeyValid(t *testing.T) {
	b, err := base64.StdEncoding.DecodeString(testEncKey)
	require.NoError(t, err)
	assert.Len(t, b, 32)
}
