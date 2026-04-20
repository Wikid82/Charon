package caddy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mockGenerateConfigFunc creates a mock config generator that captures parameters
func mockGenerateConfigFunc(capturedProvider *string, capturedStaging *bool) func([]models.ProxyHost, string, string, string, string, bool, bool, bool, bool, bool, string, []models.SecurityRuleSet, map[string]string, []models.SecurityDecision, *models.SecurityConfig, []DNSProviderConfig, ...*crypto.EncryptionService) (*Config, error) {
	return func(hosts []models.ProxyHost, storageDir string, acmeEmail string, frontendDir string, sslProvider string, acmeStaging bool, crowdsecEnabled bool, wafEnabled bool, rateLimitEnabled bool, aclEnabled bool, adminWhitelist string, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string, decisions []models.SecurityDecision, secCfg *models.SecurityConfig, dnsProviderConfigs []DNSProviderConfig, encSvc ...*crypto.EncryptionService) (*Config, error) {
		*capturedProvider = sslProvider
		*capturedStaging = acmeStaging
		return &Config{Apps: Apps{HTTP: &HTTPApp{Servers: map[string]*Server{}}}}, nil
	}
}

// TestManager_ApplyConfig_SSLProvider_Auto tests the "auto" SSL provider setting
func TestManager_ApplyConfig_SSLProvider_Auto(t *testing.T) {
	// Track the parameters passed to generateConfigFunc
	var capturedProvider string
	var capturedStaging bool

	// Mock generateConfigFunc to capture parameters
	originalGenerateConfig := generateConfigFunc
	defer func() { generateConfigFunc = originalGenerateConfig }()
	generateConfigFunc = mockGenerateConfigFunc(&capturedProvider, &capturedStaging)

	// Mock Caddy Admin API
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == "POST" {
			var config Config
			err := json.NewDecoder(r.Body).Decode(&config)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	// Set SSL Provider to "auto"
	db.Create(&models.Setting{Key: "caddy.ssl_provider", Value: "auto"})

	// Setup Manager
	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})

	// Create a host
	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	// Apply Config
	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	// Verify that the correct parameters were passed
	assert.Equal(t, "", capturedProvider, "auto should map to empty provider (both)")
	assert.False(t, capturedStaging, "auto should default to production")
}

// TestManager_ApplyConfig_SSLProvider_LetsEncryptStaging tests the "letsencrypt-staging" SSL provider setting
func TestManager_ApplyConfig_SSLProvider_LetsEncryptStaging(t *testing.T) {
	var capturedProvider string
	var capturedStaging bool

	originalGenerateConfig := generateConfigFunc
	defer func() { generateConfigFunc = originalGenerateConfig }()
	generateConfigFunc = mockGenerateConfigFunc(&capturedProvider, &capturedStaging)

	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	db.Create(&models.Setting{Key: "caddy.ssl_provider", Value: "letsencrypt-staging"})

	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})

	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, "letsencrypt", capturedProvider)
	assert.True(t, capturedStaging, "letsencrypt-staging should enable staging")
}

// TestManager_ApplyConfig_SSLProvider_LetsEncryptProd tests the "letsencrypt-prod" SSL provider setting
func TestManager_ApplyConfig_SSLProvider_LetsEncryptProd(t *testing.T) {
	var capturedProvider string
	var capturedStaging bool

	originalGenerateConfig := generateConfigFunc
	defer func() { generateConfigFunc = originalGenerateConfig }()
	generateConfigFunc = mockGenerateConfigFunc(&capturedProvider, &capturedStaging)

	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	db.Create(&models.Setting{Key: "caddy.ssl_provider", Value: "letsencrypt-prod"})

	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})

	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, "letsencrypt", capturedProvider)
	assert.False(t, capturedStaging, "letsencrypt-prod should use production")
}

// TestManager_ApplyConfig_SSLProvider_ZeroSSL tests the "zerossl" SSL provider setting
func TestManager_ApplyConfig_SSLProvider_ZeroSSL(t *testing.T) {
	var capturedProvider string
	var capturedStaging bool

	originalGenerateConfig := generateConfigFunc
	defer func() { generateConfigFunc = originalGenerateConfig }()
	generateConfigFunc = mockGenerateConfigFunc(&capturedProvider, &capturedStaging)

	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	db.Create(&models.Setting{Key: "caddy.ssl_provider", Value: "zerossl"})

	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})

	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, "zerossl", capturedProvider)
	assert.False(t, capturedStaging, "zerossl should use production")
}

// TestManager_ApplyConfig_SSLProvider_Empty tests empty/missing SSL provider setting
func TestManager_ApplyConfig_SSLProvider_Empty(t *testing.T) {
	var capturedProvider string
	var capturedStaging bool

	originalGenerateConfig := generateConfigFunc
	defer func() { generateConfigFunc = originalGenerateConfig }()
	generateConfigFunc = mockGenerateConfigFunc(&capturedProvider, &capturedStaging)

	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	// No SSL provider setting created - should use env var for staging

	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	// Set acmeStaging to true via env var simulation
	manager := NewManager(client, db, tmpDir, "", true, config.SecurityConfig{})

	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, "", capturedProvider, "empty should default to auto (both)")
	assert.True(t, capturedStaging, "empty should respect env var for staging")
}

// TestManager_ApplyConfig_SSLProvider_EmptyWithNoStaging tests empty SSL provider with staging=false in env
func TestManager_ApplyConfig_SSLProvider_EmptyWithNoStaging(t *testing.T) {
	var capturedProvider string
	var capturedStaging bool

	originalGenerateConfig := generateConfigFunc
	defer func() { generateConfigFunc = originalGenerateConfig }()
	generateConfigFunc = mockGenerateConfigFunc(&capturedProvider, &capturedStaging)

	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})

	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, "", capturedProvider)
	assert.False(t, capturedStaging, "empty with staging=false should default to production")
}

// TestManager_ApplyConfig_SSLProvider_Unknown tests unrecognized SSL provider value
func TestManager_ApplyConfig_SSLProvider_Unknown(t *testing.T) {
	var capturedProvider string
	var capturedStaging bool

	originalGenerateConfig := generateConfigFunc
	defer func() { generateConfigFunc = originalGenerateConfig }()
	generateConfigFunc = mockGenerateConfigFunc(&capturedProvider, &capturedStaging)

	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	db.Create(&models.Setting{Key: "caddy.ssl_provider", Value: "unknown-provider"})

	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", true, config.SecurityConfig{})

	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, "", capturedProvider, "unknown value should default to auto (both)")
	assert.False(t, capturedStaging, "unknown value should default to production (not respect env var)")
}
