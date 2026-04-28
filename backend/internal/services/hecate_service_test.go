package services_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const hecateTestKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func setupHecateTestDB(t *testing.T) (*gorm.DB, *crypto.EncryptionService, *hecate.TunnelManager) {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "hecate_svc_test.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.AutoMigrate(&models.TunnelConfig{}))

	encSvc, err := crypto.NewEncryptionService(hecateTestKey)
	require.NoError(t, err)

	mgr := hecate.NewTunnelManager(db, encSvc)
	t.Cleanup(func() { _ = mgr.Stop() })

	return db, encSvc, mgr
}

func TestHecateService_Create_And_Get(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	cfg := &models.TunnelConfig{
		Name:     "test-tunnel",
		Provider: models.ProviderCloudflare,
		IsActive: false,
	}

	err := svc.Create(cfg, `{"api_token":"secret"}`)
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.UUID)

	got, err := svc.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, "test-tunnel", got.Name)
	assert.NotEmpty(t, got.EncryptedCredentials)
}

func TestHecateService_List(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	for i := 0; i < 3; i++ {
		cfg := &models.TunnelConfig{
			Name:     "tunnel",
			Provider: models.ProviderTailscale,
		}
		require.NoError(t, svc.Create(cfg, "{}"))
	}

	all, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestHecateService_Delete(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	cfg := &models.TunnelConfig{Name: "del-test", Provider: models.ProviderCloudflare}
	require.NoError(t, svc.Create(cfg, "{}"))

	require.NoError(t, svc.Delete(cfg.UUID))

	_, err := svc.Get(cfg.UUID)
	assert.Error(t, err)
}

func TestHecateService_RotateCredentials(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	cfg := &models.TunnelConfig{Name: "rotate-test", Provider: models.ProviderCloudflare}
	require.NoError(t, svc.Create(cfg, `{"token":"old"}`))

	oldCreds := cfg.EncryptedCredentials

	err := svc.RotateCredentials(cfg.UUID, `{"token":"new"}`)
	require.NoError(t, err)

	updated, err := svc.Get(cfg.UUID)
	require.NoError(t, err)
	assert.NotEqual(t, oldCreds, updated.EncryptedCredentials)

	// Verify new credentials decrypt successfully.
	plain, err := encSvc.Decrypt(updated.EncryptedCredentials)
	require.NoError(t, err)
	assert.Equal(t, `{"token":"new"}`, string(plain))
}

func TestHecateService_GetStatus_Empty(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	statuses := svc.GetStatus()
	assert.Empty(t, statuses)
}

func TestHecateService_Get_NotFound(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	_, err := svc.Get("nonexistent-uuid")
	assert.Error(t, err)
}

func TestHecateService_Update(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	cfg := &models.TunnelConfig{Name: "update-test", Provider: models.ProviderCloudflare}
	require.NoError(t, svc.Create(cfg, `{"token":"original"}`))

	newCreds := `{"token":"updated"}`
	updated := &models.TunnelConfig{
		Name:     "update-test-renamed",
		Provider: models.ProviderTailscale,
		IsActive: false,
	}
	require.NoError(t, svc.Update(cfg.UUID, updated, &newCreds))

	got, err := svc.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, "update-test-renamed", got.Name)
}

// ---- Mock providers for service coverage tests ----

type svcNopProvider struct{}

func (p *svcNopProvider) Name() string                  { return "nop" }
func (p *svcNopProvider) Status() hecate.TunnelState    { return hecate.TunnelStateConnected }
func (p *svcNopProvider) Start(_ context.Context) error { return nil }
func (p *svcNopProvider) Stop() error                   { return nil }
func (p *svcNopProvider) GetAddress() string            { return "" }

type svcErrStopProvider struct{}

func (p *svcErrStopProvider) Name() string                  { return "errstop" }
func (p *svcErrStopProvider) Status() hecate.TunnelState    { return hecate.TunnelStateConnected }
func (p *svcErrStopProvider) Start(_ context.Context) error { return nil }
func (p *svcErrStopProvider) Stop() error                   { return fmt.Errorf("intentional stop error") }
func (p *svcErrStopProvider) GetAddress() string            { return "" }

// startSvcTunnel creates a tunnel in the DB via the service and starts it in the manager.
func startSvcTunnel(t *testing.T, svc *services.HecateService, mgr *hecate.TunnelManager, provider models.TunnelProviderType, factory hecate.ProviderFactory) string {
	t.Helper()
	mgr.RegisterFactory(provider, factory)
	cfg := &models.TunnelConfig{Name: "svc-running", Provider: provider}
	require.NoError(t, svc.Create(cfg, `{}`))
	require.NoError(t, mgr.StartTunnel(cfg.UUID))
	return cfg.UUID
}

// ---- Coverage tests ----

func TestHecateService_GetManager(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	got := svc.GetManager()
	require.NotNil(t, got)
}

func TestHecateService_Create_ActiveTunnel_StartSuccess(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	mgr.RegisterFactory(models.ProviderCloudflare, func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &svcNopProvider{}, nil
	})

	cfg := &models.TunnelConfig{
		Name:     "active-tunnel",
		Provider: models.ProviderCloudflare,
		IsActive: true,
	}
	err := svc.Create(cfg, `{}`)
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.UUID)
}

func TestHecateService_Create_ActiveTunnel_StartError(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	mgr.RegisterFactory(models.ProviderCloudflare, func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return nil, fmt.Errorf("factory error")
	})

	cfg := &models.TunnelConfig{
		Name:     "fail-active",
		Provider: models.ProviderCloudflare,
		IsActive: true,
	}
	err := svc.Create(cfg, `{}`)
	assert.Error(t, err)
}

func TestHecateService_List_DBError(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = svc.List()
	assert.Error(t, err)
}

func TestHecateService_Delete_StopError(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	errFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &svcErrStopProvider{}, nil
	})
	tunnelUUID := startSvcTunnel(t, svc, mgr, models.ProviderCloudflare, errFactory)

	err := svc.Delete(tunnelUUID)
	assert.Error(t, err)
}

func TestHecateService_Delete_DBError(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	cfg := &models.TunnelConfig{Name: "del-db-err", Provider: models.ProviderCloudflare}
	require.NoError(t, svc.Create(cfg, `{}`))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = svc.Delete(cfg.UUID)
	assert.Error(t, err)
}

func TestHecateService_RotateCredentials_StopError(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	errFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &svcErrStopProvider{}, nil
	})
	tunnelUUID := startSvcTunnel(t, svc, mgr, models.ProviderCloudflare, errFactory)

	err := svc.RotateCredentials(tunnelUUID, `{"token":"new"}`)
	assert.Error(t, err)
}

func TestHecateService_Update_GetError(t *testing.T) {
	db, encSvc, mgr := setupHecateTestDB(t)
	svc := services.NewHecateService(db, encSvc, mgr)

	err := svc.Update("nonexistent-uuid", &models.TunnelConfig{
		Name:     "x",
		Provider: models.ProviderCloudflare,
	}, nil)
	assert.Error(t, err)
}
