package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
)

// These tests extend the E30 stale-write checks to every writer that touches an
// encrypted row. Each one rotates the row between the service's load and its
// UPDATE (see rotateDuringUpdate). Writers that leave the secret columns out of
// their UPDATE must keep the rotated values. Writers that re-encrypt must store
// the ciphertext they produced, not the rotated one.

// staleEncrypt seals payload with the cipher and returns the ciphertext and tag.
func staleEncrypt(t *testing.T, cipher crypto.VersionedCipher, payload string) (ciphertext string, keyVersion int) {
	t.Helper()
	ciphertext, version, err := cipher.EncryptWithCurrentKey([]byte(payload))
	require.NoError(t, err)
	return ciphertext, version
}

// staleDecryptStored opens the row's stored ciphertext with its recorded tag.
func staleDecryptStored(t *testing.T, cipher crypto.VersionedCipher, ciphertext string, version int) string {
	t.Helper()
	plain, err := cipher.DecryptWithVersion(ciphertext, version)
	require.NoError(t, err)
	return string(plain)
}

func TestHecateService_StaleRowWrites(t *testing.T) {
	cipher := cryptoCipherForTest(t)

	createTunnel := func(t *testing.T, svc *HecateService) models.TunnelConfig {
		t.Helper()
		cfg := &models.TunnelConfig{Name: "before", Provider: models.ProviderCloudflare, Configuration: "{}"}
		require.NoError(t, svc.Create(cfg, `{"token":"original"}`))
		return *cfg
	}

	t.Run("update keeps rotated secret", func(t *testing.T) {
		db := newSecretWriteDB(t)
		svc := NewHecateService(db, cipher, hecate.NewTunnelManager(db, cipher))
		cfg := createTunnel(t, svc)
		rotateDuringUpdate(t, db, "tunnel_configs", cfg.ID)

		require.NoError(t, svc.Update(cfg.UUID, &models.TunnelConfig{Name: "after", Provider: cfg.Provider, Configuration: "{}"}, nil))
		assertRotatedSecret(t, db, "tunnel_configs", cfg.ID, "after")
	})

	t.Run("rotate credentials stores the new ciphertext", func(t *testing.T) {
		db := newSecretWriteDB(t)
		mgr := hecate.NewTunnelManager(db, cipher)
		t.Cleanup(func() { _ = mgr.Stop() })
		svc := NewHecateService(db, cipher, mgr)
		cfg := createTunnel(t, svc)
		rotateDuringUpdate(t, db, "tunnel_configs", cfg.ID)

		require.NoError(t, svc.RotateCredentials(cfg.UUID, `{"token":"fresh"}`))

		var reloaded models.TunnelConfig
		require.NoError(t, db.First(&reloaded, cfg.ID).Error)
		assert.Equal(t, `{"token":"fresh"}`, staleDecryptStored(t, cipher, reloaded.EncryptedCredentials, reloaded.KeyVersion))
		assert.Equal(t, "before", reloaded.Name)
	})
}

func TestCertificateService_StaleRowWrites(t *testing.T) {
	t.Run("rename keeps rotated key", func(t *testing.T) {
		db := newSecretWriteDB(t)
		svc := newTestCertificateService(t.TempDir(), db)
		row := models.SSLCertificate{UUID: uuid.NewString(), Name: "before", Provider: "custom", Domains: "rename.example.com", Certificate: "pem"}
		require.NoError(t, db.Create(&row).Error)
		rotateDuringUpdate(t, db, "ssl_certificates", row.ID)

		_, err := svc.UpdateCertificate(row.UUID, "after")
		require.NoError(t, err)
		assertRotatedSecret(t, db, "ssl_certificates", row.ID, "after")
	})

	t.Run("sync from disk keeps rotated key", func(t *testing.T) {
		db := newSecretWriteDB(t)
		dataDir := t.TempDir()
		certRoot := filepath.Join(dataDir, "certificates")
		require.NoError(t, os.MkdirAll(certRoot, 0o755)) //nolint:gosec // G301: 0755 standard for test directories
		domain := "sync-stale.example.com"
		certPEM, _ := generateTestCertAndKey(t, domain, time.Now().Add(24*time.Hour))
		require.NoError(t, os.WriteFile(filepath.Join(certRoot, domain+".crt"), certPEM, 0o600))

		row := models.SSLCertificate{UUID: uuid.NewString(), Name: domain, Provider: "letsencrypt", Domains: domain, Certificate: "stale-pem"}
		require.NoError(t, db.Create(&row).Error)
		rotateDuringUpdate(t, db, "ssl_certificates", row.ID)

		require.NoError(t, newTestCertificateService(dataDir, db).SyncFromDisk())

		var reloaded models.SSLCertificate
		require.NoError(t, db.First(&reloaded, row.ID).Error)
		assert.Equal(t, string(certPEM), reloaded.Certificate)
		assertRotatedSecret(t, db, "ssl_certificates", row.ID, "")
	})
}

func TestBackupRemoteService_StaleRowWrites(t *testing.T) {
	cipher := cryptoCipherForTest(t)

	t.Run("test outcome keeps rotated secret", func(t *testing.T) {
		db := newSecretWriteDB(t)
		target := models.RemoteStorageTarget{UUID: uuid.NewString(), Name: "probe", Type: "s3"}
		require.NoError(t, db.Create(&target).Error)
		svc := NewBackupRemoteService(db, cipher, t.TempDir())
		loaded, err := svc.Get(target.UUID)
		require.NoError(t, err)
		rotateDuringUpdate(t, db, "remote_storage_targets", target.ID)

		svc.recordTestOutcome(loaded, errors.New("probe failed"))

		assertRotatedSecret(t, db, "remote_storage_targets", target.ID, "")
		var reloaded models.RemoteStorageTarget
		require.NoError(t, db.First(&reloaded, target.ID).Error)
		assert.Equal(t, "failed", reloaded.LastTestStatus)
	})

	t.Run("complete oauth stores the new tokens", func(t *testing.T) {
		db := newSecretWriteDB(t)
		secretsJSON, err := json.Marshal(RemoteTargetSecrets{OAuthClientSecret: "client-secret"})
		require.NoError(t, err)
		ciphertext, version, err := cipher.EncryptWithCurrentKey(secretsJSON)
		require.NoError(t, err)
		configJSON, err := json.Marshal(RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}})
		require.NoError(t, err)
		target := models.RemoteStorageTarget{
			UUID: uuid.NewString(), Name: "oauth", Type: "dropbox", ConfigJSON: string(configJSON),
			SecretsEncrypted: ciphertext, KeyVersion: version, OAuthStatus: "not_connected",
		}
		require.NoError(t, db.Create(&target).Error)
		svc := NewBackupRemoteService(db, cipher, t.TempDir())
		rotateDuringUpdate(t, db, "remote_storage_targets", target.ID)

		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, staleOAuthTokenClient())
		require.NoError(t, svc.CompleteOAuth(ctx, target.UUID, "dropbox", "code", "https://charon.example"))

		var reloaded models.RemoteStorageTarget
		require.NoError(t, db.First(&reloaded, target.ID).Error)
		assert.Equal(t, "connected", reloaded.OAuthStatus)
		assert.Contains(t, staleDecryptStored(t, cipher, reloaded.SecretsEncrypted, reloaded.KeyVersion), "new-access")
	})

	t.Run("disconnect without secrets keeps rotated key", func(t *testing.T) {
		db := newSecretWriteDB(t)
		target := models.RemoteStorageTarget{UUID: uuid.NewString(), Name: "oauth", Type: "dropbox", OAuthStatus: "connected"}
		require.NoError(t, db.Create(&target).Error)
		svc := NewBackupRemoteService(db, cipher, t.TempDir())
		rotateDuringUpdate(t, db, "remote_storage_targets", target.ID)

		_, err := svc.DisconnectOAuth(target.UUID)
		require.NoError(t, err)

		assertRotatedSecret(t, db, "remote_storage_targets", target.ID, "")
		var reloaded models.RemoteStorageTarget
		require.NoError(t, db.First(&reloaded, target.ID).Error)
		assert.Equal(t, "not_connected", reloaded.OAuthStatus)
	})

	t.Run("disconnect with secrets stores cleared tokens", func(t *testing.T) {
		db := newSecretWriteDB(t)
		secretsJSON, err := json.Marshal(RemoteTargetSecrets{OAuthAccessToken: "live-access"})
		require.NoError(t, err)
		ciphertext, version, err := cipher.EncryptWithCurrentKey(secretsJSON)
		require.NoError(t, err)
		target := models.RemoteStorageTarget{
			UUID: uuid.NewString(), Name: "oauth", Type: "dropbox",
			SecretsEncrypted: ciphertext, KeyVersion: version, OAuthStatus: "connected",
		}
		require.NoError(t, db.Create(&target).Error)
		svc := NewBackupRemoteService(db, cipher, t.TempDir())
		rotateDuringUpdate(t, db, "remote_storage_targets", target.ID)

		_, err = svc.DisconnectOAuth(target.UUID)
		require.NoError(t, err)

		var reloaded models.RemoteStorageTarget
		require.NoError(t, db.First(&reloaded, target.ID).Error)
		assert.NotContains(t, staleDecryptStored(t, cipher, reloaded.SecretsEncrypted, reloaded.KeyVersion), "live-access")
	})
}

func TestDNSProviderAndCredential_StaleUsageWrites(t *testing.T) {
	cipher := cryptoCipherForTest(t)

	t.Run("provider credential read keeps rotated key", func(t *testing.T) {
		db := newSecretWriteDB(t)
		ciphertext, version := staleEncrypt(t, cipher, `{"api_token":"t"}`)
		provider := models.DNSProvider{UUID: uuid.NewString(), Name: "p", ProviderType: "cloudflare", CredentialsEncrypted: ciphertext, KeyVersion: version}
		require.NoError(t, db.Create(&provider).Error)
		svc := NewDNSProviderService(db, cipher)
		rotateDuringUpdate(t, db, "dns_providers", provider.ID)

		creds, err := svc.GetDecryptedCredentials(context.Background(), provider.ID)
		require.NoError(t, err)
		assert.Equal(t, "t", creds["api_token"])
		assertRotatedSecret(t, db, "dns_providers", provider.ID, "")
	})

	t.Run("provider test outcome keeps rotated key", func(t *testing.T) {
		db := newSecretWriteDB(t)
		ciphertext, version := staleEncrypt(t, cipher, `{}`)
		provider := models.DNSProvider{UUID: uuid.NewString(), Name: "p", ProviderType: "cloudflare", CredentialsEncrypted: ciphertext, KeyVersion: version}
		require.NoError(t, db.Create(&provider).Error)
		svc := NewDNSProviderService(db, cipher)
		rotateDuringUpdate(t, db, "dns_providers", provider.ID)

		_, err := svc.Test(context.Background(), provider.ID)
		require.NoError(t, err)

		assertRotatedSecret(t, db, "dns_providers", provider.ID, "")
		var reloaded models.DNSProvider
		require.NoError(t, db.First(&reloaded, provider.ID).Error)
		assert.Equal(t, 1, reloaded.FailureCount)
	})

	t.Run("credential test outcome keeps rotated key", func(t *testing.T) {
		db := newSecretWriteDB(t)
		provider := models.DNSProvider{UUID: uuid.NewString(), Name: "p", ProviderType: "cloudflare"}
		require.NoError(t, db.Create(&provider).Error)
		ciphertext, version := staleEncrypt(t, cipher, `{}`)
		credential := models.DNSProviderCredential{UUID: uuid.NewString(), DNSProviderID: provider.ID, Label: "c", CredentialsEncrypted: ciphertext, KeyVersion: version}
		require.NoError(t, db.Create(&credential).Error)
		svc := NewCredentialService(db, cipher)
		rotateDuringUpdate(t, db, "dns_provider_credentials", credential.ID)

		_, err := svc.Test(context.Background(), provider.ID, credential.ID)
		require.NoError(t, err)

		assertRotatedSecret(t, db, "dns_provider_credentials", credential.ID, "")
		var reloaded models.DNSProviderCredential
		require.NoError(t, db.First(&reloaded, credential.ID).Error)
		assert.Equal(t, 1, reloaded.FailureCount)
	})
}

// staleStubTransport serves every request from a function, so OAuth exchanges
// run without any network access.
type staleStubTransport func(*http.Request) (*http.Response, error)

func (f staleStubTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// staleOAuthTokenClient returns an HTTP client whose token endpoint answers with a
// fixed token response.
func staleOAuthTokenClient() *http.Client {
	return &http.Client{Transport: staleStubTransport(func(req *http.Request) (*http.Response, error) {
		body := `{"access_token":"new-access","token_type":"bearer","refresh_token":"new-refresh","expires_in":3600}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}
}
