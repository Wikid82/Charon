package caddy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestManager_ApplyConfig_NoRedirectionHostTable confirms ApplyConfig still
// works against a DB that has never migrated RedirectionHost (e.g. every
// pre-existing manager_test.go/manager_additional_test.go setup) — the
// HasTable guard in ApplyConfig must make this a no-op, not an error.
func TestManager_ApplyConfig_NoRedirectionHostTable(t *testing.T) {
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})

	require.NoError(t, db.Create(&models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}).Error)

	assert.NoError(t, manager.ApplyConfig(context.Background()))
}

// TestManager_ApplyConfig_FetchesAndAppliesRedirectionHosts confirms
// ApplyConfig fetches RedirectionHost rows and passes them through to
// GenerateConfig, ending up in the applied Caddy config as a
// static_response route (docs/plans/current_spec.md §4.2/§6).
func TestManager_ApplyConfig_FetchesAndAppliesRedirectionHosts(t *testing.T) {
	var capturedConfig Config
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedConfig))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.RedirectionHost{}, &models.DNSProvider{}))

	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})

	require.NoError(t, db.Create(&models.RedirectionHost{
		DomainNames: "old.example.com",
		TargetURL:   "https://new.example.com",
		StatusCode:  301,
		Enabled:     true,
	}).Error)

	require.NoError(t, manager.ApplyConfig(context.Background()))

	server := capturedConfig.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.NotEmpty(t, server.Routes)
	assert.Equal(t, []string{"old.example.com"}, server.Routes[0].Match[0].Host)
	assert.Equal(t, "static_response", server.Routes[0].Handle[0]["handler"])

	var caddyConfig models.CaddyConfig
	require.NoError(t, db.First(&caddyConfig).Error)
	assert.True(t, caddyConfig.Success)
}

// TestManager_ApplyConfig_RedirectionHostsFetchError confirms ApplyConfig
// surfaces a wrapped "fetch redirection hosts" error (rather than silently
// proceeding with an empty/partial list) when the RedirectionHost table
// exists but the Preload("DNSProvider") query it runs against a row with a
// non-nil DNSProviderID fails — simulated here by dropping the DNS provider
// table out from under an already-inserted row.
func TestManager_ApplyConfig_RedirectionHostsFetchError(t *testing.T) {
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.RedirectionHost{}, &models.DNSProvider{}))

	provider := models.DNSProvider{UUID: "dns-fetch-err", Name: "Test DNS", ProviderType: "cloudflare"}
	require.NoError(t, db.Create(&provider).Error)

	require.NoError(t, db.Create(&models.RedirectionHost{
		DomainNames:   "fetcherr.example.com",
		TargetURL:     "https://target.example.com",
		StatusCode:    301,
		Enabled:       true,
		DNSProviderID: &provider.ID,
	}).Error)

	// Drop the DNS provider table AFTER inserting the row so
	// Preload("DNSProvider") has a non-nil foreign key to look up but no
	// table left to satisfy the lookup.
	require.NoError(t, db.Migrator().DropTable(&models.DNSProvider{}))

	tmpDir := t.TempDir()
	client := newTestClient(t, caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})

	err = manager.ApplyConfig(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch redirection hosts")
}
