package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
)

// The tests below reload each row from the database after an update. They check
// that every field a request can change is written, not only applied to the
// in-memory struct that the handler echoes back.

func TestCredentialService_Update_PersistsEveryRequestField(t *testing.T) {
	db := newSecretWriteDB(t)
	svc := NewCredentialService(db, cryptoCipherForTest(t))
	provider := models.DNSProvider{UUID: uuid.NewString(), Name: "p", ProviderType: "cloudflare"}
	require.NoError(t, db.Create(&provider).Error)

	// newCredential creates a fresh row and returns its loaded state so each
	// subtest can request a value that differs from what is stored.
	newCredential := func(t *testing.T) models.DNSProviderCredential {
		t.Helper()
		row := models.DNSProviderCredential{UUID: uuid.NewString(), DNSProviderID: provider.ID, Label: "before"}
		require.NoError(t, db.Create(&row).Error)
		var loaded models.DNSProviderCredential
		require.NoError(t, db.First(&loaded, row.ID).Error)
		return loaded
	}
	reload := func(t *testing.T, id uint) models.DNSProviderCredential {
		t.Helper()
		var loaded models.DNSProviderCredential
		require.NoError(t, db.First(&loaded, id).Error)
		return loaded
	}

	t.Run("label", func(t *testing.T) {
		row := newCredential(t)
		_, err := svc.Update(context.Background(), provider.ID, row.ID, UpdateCredentialRequest{Label: writerPtr("after")})
		require.NoError(t, err)
		assert.Equal(t, "after", reload(t, row.ID).Label)
	})

	t.Run("zone_filter", func(t *testing.T) {
		row := newCredential(t)
		_, err := svc.Update(context.Background(), provider.ID, row.ID, UpdateCredentialRequest{ZoneFilter: writerPtr("example.com")})
		require.NoError(t, err)
		assert.Equal(t, "example.com", reload(t, row.ID).ZoneFilter)
	})

	t.Run("propagation_timeout", func(t *testing.T) {
		row := newCredential(t)
		_, err := svc.Update(context.Background(), provider.ID, row.ID, UpdateCredentialRequest{PropagationTimeout: writerPtr(777)})
		require.NoError(t, err)
		assert.Equal(t, 777, reload(t, row.ID).PropagationTimeout)
	})

	t.Run("polling_interval", func(t *testing.T) {
		row := newCredential(t)
		_, err := svc.Update(context.Background(), provider.ID, row.ID, UpdateCredentialRequest{PollingInterval: writerPtr(888)})
		require.NoError(t, err)
		assert.Equal(t, 888, reload(t, row.ID).PollingInterval)
	})

	t.Run("enabled", func(t *testing.T) {
		row := newCredential(t)
		_, err := svc.Update(context.Background(), provider.ID, row.ID, UpdateCredentialRequest{Enabled: writerPtr(!row.Enabled)})
		require.NoError(t, err)
		assert.Equal(t, !row.Enabled, reload(t, row.ID).Enabled)
	})
}

func TestDNSProviderService_Update_PersistsEveryRequestField(t *testing.T) {
	db := newSecretWriteDB(t)
	svc := NewDNSProviderService(db, cryptoCipherForTest(t))

	newProvider := func(t *testing.T) models.DNSProvider {
		t.Helper()
		row := models.DNSProvider{UUID: uuid.NewString(), Name: "before", ProviderType: "cloudflare"}
		require.NoError(t, db.Create(&row).Error)
		var loaded models.DNSProvider
		require.NoError(t, db.First(&loaded, row.ID).Error)
		return loaded
	}
	reload := func(t *testing.T, id uint) models.DNSProvider {
		t.Helper()
		var loaded models.DNSProvider
		require.NoError(t, db.First(&loaded, id).Error)
		return loaded
	}

	t.Run("name", func(t *testing.T) {
		row := newProvider(t)
		_, err := svc.Update(context.Background(), row.ID, UpdateDNSProviderRequest{Name: writerPtr("after")})
		require.NoError(t, err)
		assert.Equal(t, "after", reload(t, row.ID).Name)
	})

	t.Run("propagation_timeout", func(t *testing.T) {
		row := newProvider(t)
		_, err := svc.Update(context.Background(), row.ID, UpdateDNSProviderRequest{PropagationTimeout: writerPtr(777)})
		require.NoError(t, err)
		assert.Equal(t, 777, reload(t, row.ID).PropagationTimeout)
	})

	t.Run("polling_interval", func(t *testing.T) {
		row := newProvider(t)
		_, err := svc.Update(context.Background(), row.ID, UpdateDNSProviderRequest{PollingInterval: writerPtr(888)})
		require.NoError(t, err)
		assert.Equal(t, 888, reload(t, row.ID).PollingInterval)
	})

	t.Run("enabled", func(t *testing.T) {
		row := newProvider(t)
		_, err := svc.Update(context.Background(), row.ID, UpdateDNSProviderRequest{Enabled: writerPtr(!row.Enabled)})
		require.NoError(t, err)
		assert.Equal(t, !row.Enabled, reload(t, row.ID).Enabled)
	})

	t.Run("is_default", func(t *testing.T) {
		row := newProvider(t)
		_, err := svc.Update(context.Background(), row.ID, UpdateDNSProviderRequest{IsDefault: writerPtr(true)})
		require.NoError(t, err)
		assert.True(t, reload(t, row.ID).IsDefault)
	})
}

func TestBackupRemoteService_Update_PersistsEveryRequestField(t *testing.T) {
	db := newSecretWriteDB(t)
	svc := NewBackupRemoteService(db, cryptoCipherForTest(t), t.TempDir())

	newTarget := func(t *testing.T) models.RemoteStorageTarget {
		t.Helper()
		row := models.RemoteStorageTarget{UUID: uuid.NewString(), Name: "before", Type: "s3", Enabled: true}
		require.NoError(t, db.Create(&row).Error)
		var loaded models.RemoteStorageTarget
		require.NoError(t, db.First(&loaded, row.ID).Error)
		return loaded
	}
	reload := func(t *testing.T, id uint) models.RemoteStorageTarget {
		t.Helper()
		var loaded models.RemoteStorageTarget
		require.NoError(t, db.First(&loaded, id).Error)
		return loaded
	}

	t.Run("name", func(t *testing.T) {
		row := newTarget(t)
		_, err := svc.Update(row.UUID, writerPtr("after"), nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "after", reload(t, row.ID).Name)
	})

	t.Run("enabled", func(t *testing.T) {
		row := newTarget(t)
		_, err := svc.Update(row.UUID, nil, writerPtr(!row.Enabled), nil, nil)
		require.NoError(t, err)
		assert.Equal(t, !row.Enabled, reload(t, row.ID).Enabled)
	})

	t.Run("config", func(t *testing.T) {
		row := newTarget(t)
		config := RemoteTargetConfig{Endpoint: "203.0.113.10:9000", Bucket: "reloaded-bucket"}
		_, err := svc.Update(row.UUID, nil, nil, &config, nil)
		require.NoError(t, err)
		assert.Contains(t, reload(t, row.ID).ConfigJSON, "reloaded-bucket")
	})
}

func TestHecateService_Update_PersistsEveryRequestField(t *testing.T) {
	db := newSecretWriteDB(t)
	cipher := cryptoCipherForTest(t)
	svc := NewHecateService(db, cipher, hecate.NewTunnelManager(db, cipher))

	newTunnel := func(t *testing.T) models.TunnelConfig {
		t.Helper()
		cfg := &models.TunnelConfig{Name: "before", Provider: models.ProviderCloudflare, Configuration: "{}"}
		require.NoError(t, svc.Create(cfg, `{"token":"t"}`))
		return *cfg
	}
	reload := func(t *testing.T, id uint) models.TunnelConfig {
		t.Helper()
		var loaded models.TunnelConfig
		require.NoError(t, db.First(&loaded, id).Error)
		return loaded
	}

	t.Run("name", func(t *testing.T) {
		row := newTunnel(t)
		require.NoError(t, svc.Update(row.UUID, &models.TunnelConfig{Name: "after", Provider: row.Provider, Configuration: row.Configuration}, nil))
		assert.Equal(t, "after", reload(t, row.ID).Name)
	})

	t.Run("provider", func(t *testing.T) {
		row := newTunnel(t)
		require.NoError(t, svc.Update(row.UUID, &models.TunnelConfig{Name: row.Name, Provider: models.ProviderTailscale, Configuration: row.Configuration}, nil))
		assert.Equal(t, models.ProviderTailscale, reload(t, row.ID).Provider)
	})

	t.Run("configuration", func(t *testing.T) {
		row := newTunnel(t)
		require.NoError(t, svc.Update(row.UUID, &models.TunnelConfig{Name: row.Name, Provider: row.Provider, Configuration: `{"account_id":"a1"}`}, nil))
		assert.Equal(t, `{"account_id":"a1"}`, reload(t, row.ID).Configuration)
	})

	t.Run("is_active", func(t *testing.T) {
		row := newTunnel(t)
		require.NoError(t, svc.Update(row.UUID, &models.TunnelConfig{Name: row.Name, Provider: row.Provider, Configuration: row.Configuration, IsActive: !row.IsActive}, nil))
		assert.Equal(t, !row.IsActive, reload(t, row.ID).IsActive)
	})
}

func TestHecateService_Create_TagsRowWithKeyVersion(t *testing.T) {
	db := newSecretWriteDB(t)
	cipher := cryptoCipherForTest(t)
	svc := NewHecateService(db, cipher, hecate.NewTunnelManager(db, cipher))

	_, wantVersion, err := cipher.EncryptWithCurrentKey([]byte("probe"))
	require.NoError(t, err)
	require.NotZero(t, wantVersion)

	cfg := &models.TunnelConfig{Name: "tagged", Provider: models.ProviderCloudflare}
	require.NoError(t, svc.Create(cfg, `{"token":"t"}`))

	var reloaded models.TunnelConfig
	require.NoError(t, db.First(&reloaded, cfg.ID).Error)
	assert.Equal(t, wantVersion, reloaded.KeyVersion)
}

// writerPtr returns a pointer to v for building optional request fields.
func writerPtr[T any](v T) *T { return &v }
