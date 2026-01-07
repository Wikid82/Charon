package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPluginHandler_NewPluginHandler(t *testing.T) {
	db := OpenTestDB(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	handler := NewPluginHandler(db, pluginLoader)

	assert.NotNil(t, handler)
	assert.Equal(t, db, handler.db)
	assert.Equal(t, pluginLoader, handler.pluginLoader)
}

func TestPluginHandler_ListPlugins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	// Create a failed plugin in DB
	failedPlugin := models.Plugin{
		UUID:      "plugin-uuid-1",
		Name:      "Failed Plugin",
		Type:      "failed-type",
		Enabled:   false,
		Status:    models.PluginStatusError,
		Error:     "Failed to load",
		Version:   "1.0.0",
		Author:    "Test Author",
		FilePath:  "/path/to/plugin.so",
		LoadedAt:  nil,
	}
	db.Create(&failedPlugin)

	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.GET("/plugins", handler.ListPlugins)

	req := httptest.NewRequest(http.MethodGet, "/plugins", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var plugins []PluginInfo
	err := json.Unmarshal(w.Body.Bytes(), &plugins)
	assert.NoError(t, err)
	assert.NotEmpty(t, plugins)

	// Find the failed plugin
	var found *PluginInfo
	for i := range plugins {
		if plugins[i].Type == "failed-type" {
			found = &plugins[i]
			break
		}
	}
	assert.NotNil(t, found, "Failed plugin should be in list")
	assert.Equal(t, models.PluginStatusError, found.Status)
	assert.Equal(t, "Failed to load", found.Error)
}

func TestPluginHandler_GetPlugin_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)
	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.GET("/plugins/:id", handler.GetPlugin)

	req := httptest.NewRequest(http.MethodGet, "/plugins/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid plugin ID")
}

func TestPluginHandler_GetPlugin_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)
	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.GET("/plugins/:id", handler.GetPlugin)

	req := httptest.NewRequest(http.MethodGet, "/plugins/99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Plugin not found")
}

func TestPluginHandler_GetPlugin_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	// Create a plugin
	plugin := models.Plugin{
		UUID:     "plugin-uuid",
		Name:     "Test Plugin",
		Type:     "test-provider",
		Enabled:  true,
		Status:   models.PluginStatusLoaded,
		Version:  "1.0.0",
		Author:   "Test Author",
		FilePath: "/path/to/plugin.so",
	}
	db.Create(&plugin)

	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.GET("/plugins/:id", handler.GetPlugin)

	req := httptest.NewRequest(http.MethodGet, "/plugins/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result PluginInfo
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "Test Plugin", result.Name)
	assert.Equal(t, "test-provider", result.Type)
}

func TestPluginHandler_EnablePlugin_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)
	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/:id/enable", handler.EnablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/abc/enable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPluginHandler_EnablePlugin_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)
	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/:id/enable", handler.EnablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/99999/enable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPluginHandler_EnablePlugin_AlreadyEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	plugin := models.Plugin{
		UUID:     "plugin-enabled",
		Name:     "Enabled Plugin",
		Type:     "enabled-type",
		Enabled:  true,
		Status:   models.PluginStatusLoaded,
		FilePath: "/path/to/enabled.so",
	}
	db.Create(&plugin)

	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/:id/enable", handler.EnablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/1/enable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "already enabled")
}

func TestPluginHandler_EnablePlugin_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	plugin := models.Plugin{
		UUID:     "plugin-disabled",
		Name:     "Disabled Plugin",
		Type:     "disabled-type",
		Enabled:  false,
		Status:   models.PluginStatusError,
		FilePath: "/path/to/disabled.so",
	}
	db.Create(&plugin)

	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/:id/enable", handler.EnablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/1/enable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Expect 200 even if plugin fails to load
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify database was updated
	var updated models.Plugin
	db.First(&updated, plugin.ID)
	assert.True(t, updated.Enabled)
}

func TestPluginHandler_DisablePlugin_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)
	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/:id/disable", handler.DisablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/xyz/disable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPluginHandler_DisablePlugin_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)
	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/:id/disable", handler.DisablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/99999/disable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPluginHandler_DisablePlugin_AlreadyDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	plugin := models.Plugin{
		UUID:     "plugin-already-disabled",
		Name:     "Already Disabled",
		Type:     "already-disabled-type",
		Enabled:  false,
		FilePath: "/path/to/already.so",
	}
	db.Create(&plugin)

	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/:id/disable", handler.DisablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/1/disable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "already disabled")
}

func TestPluginHandler_DisablePlugin_InUse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	plugin := models.Plugin{
		UUID:     "plugin-in-use",
		Name:     "In Use Plugin",
		Type:     "in-use-type",
		Enabled:  true,
		FilePath: "/path/to/inuse.so",
	}
	db.Create(&plugin)

	// Create a DNS provider using this plugin
	dnsProvider := models.DNSProvider{
		UUID:                 "dns-provider-uuid",
		Name:                 "Test DNS Provider",
		ProviderType:         "in-use-type",
		CredentialsEncrypted: "encrypted-data",
	}
	db.Create(&dnsProvider)

	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/:id/disable", handler.DisablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/1/disable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Cannot disable plugin")
	assert.Contains(t, w.Body.String(), "DNS provider(s) are using it")
}

func TestPluginHandler_DisablePlugin_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	plugin := models.Plugin{
		UUID:     "plugin-to-disable",
		Name:     "To Disable",
		Type:     "to-disable-type",
		Enabled:  true,
		FilePath: "/path/to/disable.so",
	}
	db.Create(&plugin)

	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/:id/disable", handler.DisablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/1/disable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "disabled successfully")

	// Verify database was updated
	var updated models.Plugin
	db.First(&updated, plugin.ID)
	assert.False(t, updated.Enabled)
}

func TestPluginHandler_ReloadPlugins_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	pluginLoader := services.NewPluginLoaderService(db, "/nonexistent/plugins", nil)
	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.POST("/plugins/reload", handler.ReloadPlugins)

	req := httptest.NewRequest(http.MethodPost, "/plugins/reload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should succeed even if no plugins found
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "reloaded successfully")
}

// TestPluginHandler_ListPlugins_WithBuiltInProviders tests listing when built-in providers are registered
func TestPluginHandler_ListPlugins_WithBuiltInProviders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	// Create test provider and register it
	testProvider := &mockDNSProvider{
		providerType: "cloudflare",
		metadata: dnsprovider.ProviderMetadata{
			Name:        "Cloudflare",
			Version:     "1.0.0",
			Author:      "Built-in",
			IsBuiltIn:   true,
			Description: "Cloudflare DNS provider",
		},
	}
	dnsprovider.Global().Register(testProvider)
	defer dnsprovider.Global().Unregister("cloudflare")

	handler := NewPluginHandler(db, pluginLoader)

	router := gin.New()
	router.GET("/plugins", handler.ListPlugins)

	req := httptest.NewRequest(http.MethodGet, "/plugins", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var plugins []PluginInfo
	err := json.Unmarshal(w.Body.Bytes(), &plugins)
	assert.NoError(t, err)

	// Find cloudflare provider
	found := false
	for _, p := range plugins {
		if p.Type == "cloudflare" {
			found = true
			assert.True(t, p.IsBuiltIn)
			assert.Equal(t, "Cloudflare", p.Name)
			break
		}
	}
	assert.True(t, found, "Cloudflare provider should be listed")
}

// mockDNSProvider for testing
type mockDNSProvider struct {
	providerType string
	metadata     dnsprovider.ProviderMetadata
}

func (m *mockDNSProvider) Type() string {
	return m.providerType
}

func (m *mockDNSProvider) Metadata() dnsprovider.ProviderMetadata {
	return m.metadata
}

func (m *mockDNSProvider) Init() error {
	return nil
}

func (m *mockDNSProvider) Cleanup() error {
	return nil
}

func (m *mockDNSProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return nil
}

func (m *mockDNSProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return nil
}

func (m *mockDNSProvider) ValidateCredentials(map[string]string) error {
	return nil
}

func (m *mockDNSProvider) TestCredentials(map[string]string) error {
	return nil
}

func (m *mockDNSProvider) SupportsMultiCredential() bool {
	return false
}

func (m *mockDNSProvider) CreateRecord(domain, recordType, name, value string, ttl int) error {
	return nil
}

func (m *mockDNSProvider) DeleteRecord(domain, recordType, name, value string) error {
	return nil
}

func (m *mockDNSProvider) BuildCaddyConfig(credentials map[string]string) map[string]any {
	return nil
}

func (m *mockDNSProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return nil
}

func (m *mockDNSProvider) PropagationTimeout() time.Duration {
	return 60
}

func (m *mockDNSProvider) PollingInterval() time.Duration {
	return 2
}
