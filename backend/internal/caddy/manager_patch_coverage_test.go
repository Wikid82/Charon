package caddy

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestManagerApplyConfig_DNSProviders_NoKey_SkipsDecryption(t *testing.T) {
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.Setting{},
		&models.CaddyConfig{},
		&models.SSLCertificate{},
		&models.SecurityConfig{},
		&models.SecurityRuleSet{},
		&models.SecurityDecision{},
		&models.DNSProvider{},
	))

	db.Create(&models.SecurityConfig{Name: "default", Enabled: true})
	db.Create(&models.DNSProvider{Name: "p", ProviderType: "cloudflare", Enabled: true, CredentialsEncrypted: "invalid"})

	_ = os.Unsetenv("CHARON_ENCRYPTION_KEY")
	_ = os.Unsetenv("ENCRYPTION_KEY")
	_ = os.Unsetenv("CERBERUS_ENCRYPTION_KEY")

	var capturedLen int
	origGen := generateConfigFunc
	origVal := validateConfigFunc
	defer func() {
		generateConfigFunc = origGen
		validateConfigFunc = origVal
	}()
	generateConfigFunc = func(_ []models.ProxyHost, _ string, _ string, _ string, _ string, _ bool, _ bool, _ bool, _ bool, _ bool, _ string, _ []models.SecurityRuleSet, _ map[string]string, _ []models.SecurityDecision, _ *models.SecurityConfig, dnsProviderConfigs []DNSProviderConfig) (*Config, error) {
		capturedLen = len(dnsProviderConfigs)
		return &Config{}, nil
	}
	validateConfigFunc = func(_ *Config) error { return nil }

	manager := NewManager(newTestClient(t, caddyServer.URL), db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	require.NoError(t, manager.ApplyConfig(context.Background()))
	require.Equal(t, 0, capturedLen)
}

func TestManagerApplyConfig_DNSProviders_UsesFallbackEnvKeys(t *testing.T) {
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	keyBytes := make([]byte, 32)
	keyB64 := base64.StdEncoding.EncodeToString(keyBytes)
	t.Setenv("ENCRYPTION_KEY", keyB64)

	encryptor, err := crypto.NewEncryptionService(keyB64)
	require.NoError(t, err)
	ciphertext, err := encryptor.Encrypt([]byte(`{"api_token":"tok"}`))
	require.NoError(t, err)

	dsn := "file:" + t.Name() + "_fallback?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.Setting{},
		&models.CaddyConfig{},
		&models.SSLCertificate{},
		&models.SecurityConfig{},
		&models.SecurityRuleSet{},
		&models.SecurityDecision{},
		&models.DNSProvider{},
	))
	db.Create(&models.SecurityConfig{Name: "default", Enabled: true})
	db.Create(&models.DNSProvider{ID: 11, Name: "p", ProviderType: "cloudflare", Enabled: true, CredentialsEncrypted: ciphertext, PropagationTimeout: 99})

	var captured []DNSProviderConfig
	origGen := generateConfigFunc
	origVal := validateConfigFunc
	defer func() {
		generateConfigFunc = origGen
		validateConfigFunc = origVal
	}()
	generateConfigFunc = func(_ []models.ProxyHost, _ string, _ string, _ string, _ string, _ bool, _ bool, _ bool, _ bool, _ bool, _ string, _ []models.SecurityRuleSet, _ map[string]string, _ []models.SecurityDecision, _ *models.SecurityConfig, dnsProviderConfigs []DNSProviderConfig) (*Config, error) {
		captured = append([]DNSProviderConfig(nil), dnsProviderConfigs...)
		return &Config{}, nil
	}
	validateConfigFunc = func(_ *Config) error { return nil }

	manager := NewManager(newTestClient(t, caddyServer.URL), db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	require.NoError(t, manager.ApplyConfig(context.Background()))

	require.Len(t, captured, 1)
	require.Equal(t, uint(11), captured[0].ID)
	require.Equal(t, "cloudflare", captured[0].ProviderType)
	require.Equal(t, "tok", captured[0].Credentials["api_token"])
}

func TestManagerApplyConfig_DNSProviders_SkipsDecryptOrJSONFailures(t *testing.T) {
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	keyBytes := make([]byte, 32)
	keyB64 := base64.StdEncoding.EncodeToString(keyBytes)
	t.Setenv("CHARON_ENCRYPTION_KEY", keyB64)

	encryptor, err := crypto.NewEncryptionService(keyB64)
	require.NoError(t, err)
	goodCiphertext, err := encryptor.Encrypt([]byte(`{"api_token":"tok"}`))
	require.NoError(t, err)
	badJSONCiphertext, err := encryptor.Encrypt([]byte(`not-json`))
	require.NoError(t, err)

	dsn := "file:" + t.Name() + "_skip?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.Setting{},
		&models.CaddyConfig{},
		&models.SSLCertificate{},
		&models.SecurityConfig{},
		&models.SecurityRuleSet{},
		&models.SecurityDecision{},
		&models.DNSProvider{},
	))
	db.Create(&models.SecurityConfig{Name: "default", Enabled: true})

	db.Create(&models.DNSProvider{ID: 21, UUID: "uuid-empty-21", Name: "empty", ProviderType: "cloudflare", Enabled: true, CredentialsEncrypted: ""})
	db.Create(&models.DNSProvider{ID: 22, UUID: "uuid-bad-22", Name: "bad", ProviderType: "cloudflare", Enabled: true, CredentialsEncrypted: "not-base64"})
	db.Create(&models.DNSProvider{ID: 23, UUID: "uuid-badjson-23", Name: "badjson", ProviderType: "cloudflare", Enabled: true, CredentialsEncrypted: badJSONCiphertext})
	db.Create(&models.DNSProvider{ID: 24, UUID: "uuid-good-24", Name: "good", ProviderType: "cloudflare", Enabled: true, CredentialsEncrypted: goodCiphertext, PropagationTimeout: 7})

	var captured []DNSProviderConfig
	origGen := generateConfigFunc
	origVal := validateConfigFunc
	defer func() {
		generateConfigFunc = origGen
		validateConfigFunc = origVal
	}()
	generateConfigFunc = func(_ []models.ProxyHost, _ string, _ string, _ string, _ string, _ bool, _ bool, _ bool, _ bool, _ bool, _ string, _ []models.SecurityRuleSet, _ map[string]string, _ []models.SecurityDecision, _ *models.SecurityConfig, dnsProviderConfigs []DNSProviderConfig) (*Config, error) {
		captured = append([]DNSProviderConfig(nil), dnsProviderConfigs...)
		return &Config{}, nil
	}
	validateConfigFunc = func(_ *Config) error { return nil }

	manager := NewManager(newTestClient(t, caddyServer.URL), db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	require.NoError(t, manager.ApplyConfig(context.Background()))

	require.Len(t, captured, 1)
	require.Equal(t, uint(24), captured[0].ID)
}

func TestManagerApplyConfig_MapsKeepaliveSettingsToGeneratedServer(t *testing.T) {
	var loadBody []byte
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			payload, _ := io.ReadAll(r.Body)
			loadBody = append([]byte(nil), payload...)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.Setting{},
		&models.CaddyConfig{},
		&models.SSLCertificate{},
		&models.SecurityConfig{},
		&models.SecurityRuleSet{},
		&models.SecurityDecision{},
		&models.DNSProvider{},
	))

	db.Create(&models.ProxyHost{DomainNames: "keepalive.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true})
	db.Create(&models.SecurityConfig{Name: "default", Enabled: true})
	db.Create(&models.Setting{Key: settingCaddyKeepaliveIdle, Value: "45s"})
	db.Create(&models.Setting{Key: settingCaddyKeepaliveCnt, Value: "8"})

	origVal := validateConfigFunc
	defer func() { validateConfigFunc = origVal }()
	validateConfigFunc = func(_ *Config) error { return nil }

	manager := NewManager(newTestClient(t, caddyServer.URL), db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	require.NoError(t, manager.ApplyConfig(context.Background()))
	require.NotEmpty(t, loadBody)

	require.True(t, bytes.Contains(loadBody, []byte(`"keepalive_idle":"45s"`)))
	require.True(t, bytes.Contains(loadBody, []byte(`"keepalive_count":8`)))
}

func TestManagerApplyConfig_InvalidKeepaliveSettingsFallbackToDefaults(t *testing.T) {
	var loadBody []byte
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			payload, _ := io.ReadAll(r.Body)
			loadBody = append([]byte(nil), payload...)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	dsn := "file:" + t.Name() + "_invalid?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.Setting{},
		&models.CaddyConfig{},
		&models.SSLCertificate{},
		&models.SecurityConfig{},
		&models.SecurityRuleSet{},
		&models.SecurityDecision{},
		&models.DNSProvider{},
	))

	db.Create(&models.ProxyHost{DomainNames: "invalid-keepalive.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true})
	db.Create(&models.SecurityConfig{Name: "default", Enabled: true})
	db.Create(&models.Setting{Key: settingCaddyKeepaliveIdle, Value: "bad"})
	db.Create(&models.Setting{Key: settingCaddyKeepaliveCnt, Value: "-1"})

	origVal := validateConfigFunc
	defer func() { validateConfigFunc = origVal }()
	validateConfigFunc = func(_ *Config) error { return nil }

	manager := NewManager(newTestClient(t, caddyServer.URL), db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	require.NoError(t, manager.ApplyConfig(context.Background()))
	require.NotEmpty(t, loadBody)

	require.False(t, bytes.Contains(loadBody, []byte(`"keepalive_idle"`)))
	require.False(t, bytes.Contains(loadBody, []byte(`"keepalive_count"`)))
}
