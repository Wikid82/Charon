package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/dbmaint"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

const pluginReloadPath = "/api/v1/admin/plugins/reload"

// pluginWiringFixture holds a database, an unlisted plugin file in its own folder, and an admin session. Reload
// writes the loader's outcome into the plugin record, so the record's status and error show which list applied.
type pluginWiringFixture struct {
	db         *gorm.DB
	cfg        config.Config
	pluginPath string
	adminToken string
}

func newPluginWiringFixture(t *testing.T, cfg config.Config) *pluginWiringFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)

	pluginDir := t.TempDir()
	// t.TempDir can be group-writable under the umask; the loader refuses such a plugin folder.
	require.NoError(t, os.Chmod(pluginDir, 0o755))
	pluginPath := filepath.Join(pluginDir, "unlisted.so")
	require.NoError(t, os.WriteFile(pluginPath, []byte("not a real plugin"), 0o600))

	cfg.PluginsDir = pluginDir
	cfg.JWTSecret = "test-secret"
	// The admin plugin routes are registered only when an encryption key is configured.
	cfg.EncryptionKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	return &pluginWiringFixture{db: db, cfg: cfg, pluginPath: pluginPath}
}

// seedAdmin creates an admin account and a plugin record for the unlisted file. Call it after the schema exists.
func (f *pluginWiringFixture) seedAdmin(t *testing.T) {
	t.Helper()
	acct := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: "admin-plugin-wiring@example.com", Role: models.RoleAdmin, Enabled: true}
	require.NoError(t, f.db.Create(acct).Error)
	token, err := services.NewAuthService(f.db, f.cfg).GenerateToken(acct)
	require.NoError(t, err)
	f.adminToken = token

	require.NoError(t, f.db.Create(&models.Plugin{
		UUID: uuid.NewString(), Name: "unlisted", Type: "unlisted", FilePath: f.pluginPath,
		Enabled: true, Status: models.PluginStatusLoaded,
	}).Error)
}

// reload posts to the admin reload endpoint and returns the status code and the recorded plugin row.
func (f *pluginWiringFixture) reload(t *testing.T, router *gin.Engine) (int, models.Plugin) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, pluginReloadPath, http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: f.adminToken, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var record models.Plugin
	require.NoError(t, f.db.Where("file_path = ?", f.pluginPath).First(&record).Error)
	return w.Code, record
}

// TestRegister_PluginReloadRefusesUnlistedPluginWhenAllowlistIsEmpty pins that the reload endpoint uses the signature
// list from the configuration: an empty list refuses a plugin file that is not on it.
func TestRegister_PluginReloadRefusesUnlistedPluginWhenAllowlistIsEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newPluginWiringFixture(t, config.Config{PluginSignatures: map[string]string{}})
	router := gin.New()
	require.NoError(t, Register(context.Background(), router, fixture.db, fixture.cfg))
	fixture.seedAdmin(t)

	code, record := fixture.reload(t, router)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, models.PluginStatusError, record.Status)
	assert.Contains(t, record.Error, dnsprovider.ErrPluginNotInAllowlist.Error())
}

// TestRegisterWithDeps_PluginReloadUsesPassedLoader pins that a loader handed to RegisterWithDeps is the one the reload
// endpoint uses. The configuration is permissive, so only the passed loader's empty list can refuse the file.
func TestRegisterWithDeps_PluginReloadUsesPassedLoader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newPluginWiringFixture(t, config.Config{})
	fixture.cfg.FrontendDir = t.TempDir()
	router := gin.New()
	caddyManager := caddy.NewManager(caddy.NewClient(fixture.cfg.CaddyAdminAPI), fixture.db, fixture.cfg.CaddyConfigDir, fixture.cfg.FrontendDir, fixture.cfg.ACMEStaging, fixture.cfg.Security)
	cerb := cerberus.New(fixture.cfg.Security, fixture.db)
	loader := services.NewPluginLoaderService(fixture.db, fixture.cfg.PluginsDir, t.TempDir(), map[string]string{})

	_, err := RegisterWithDeps(context.Background(), router, fixture.db, fixture.cfg, caddyManager, cerb, dbmaint.NewGate(), loader)
	require.NoError(t, err)
	fixture.seedAdmin(t)

	code, record := fixture.reload(t, router)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, models.PluginStatusError, record.Status)
	assert.Contains(t, record.Error, dnsprovider.ErrPluginNotInAllowlist.Error())
}
