package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

type noopCaddyClient struct{}

func (noopCaddyClient) Load(context.Context, *caddy.Config) error        { return nil }
func (noopCaddyClient) Ping(context.Context) error                       { return nil }
func (noopCaddyClient) GetConfig(context.Context) (*caddy.Config, error) { return &caddy.Config{}, nil }

func serveDisable(t *testing.T, h *SecurityHandler, remoteAddr, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.POST("/security/disable", h.Disable)
	req := httptest.NewRequest(http.MethodPost, "/security/disable", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSecurityHandler_Disable_LocalSuccessPersists(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true}).Error)
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)

	w := serveDisable(t, h, "127.0.0.1:5000", `{}`)
	assert.Equal(t, http.StatusOK, w.Code)

	var stored models.SecurityConfig
	require.NoError(t, db.Where("name = ?", "default").First(&stored).Error)
	assert.False(t, stored.Enabled)
}

func TestSecurityHandler_Disable_ReportsPersistenceFailure(t *testing.T) {
	db := setupTestDB(t)
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)
	require.NoError(t, db.Migrator().DropTable(&models.SecurityConfig{}))

	w := serveDisable(t, h, "127.0.0.1:5000", `{}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to disable Cerberus")
	assert.NotContains(t, w.Body.String(), `"enabled":false`)
}

func TestSecurityHandler_Disable_ReportsApplyFailure(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true}).Error)
	// The proxy host tables are absent, so generating the configuration fails.
	mgr := caddy.NewManager(noopCaddyClient{}, db, t.TempDir(), "", false, config.SecurityConfig{})
	h := NewSecurityHandler(config.SecurityConfig{}, db, mgr)
	t.Cleanup(h.Close)

	w := serveDisable(t, h, "127.0.0.1:5000", `{}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "saved, but applying the configuration failed")
}

func TestSecurityHandler_Disable_TokenPathReportsApplyFailure(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true}).Error)
	mgr := caddy.NewManager(noopCaddyClient{}, db, t.TempDir(), "", false, config.SecurityConfig{})
	h := NewSecurityHandler(config.SecurityConfig{}, db, mgr)
	t.Cleanup(h.Close)
	token, err := h.svc.GenerateBreakGlassToken("default")
	require.NoError(t, err)

	w := serveDisable(t, h, "198.51.100.7:5000", `{"break_glass_token":"`+token+`"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "saved, but applying the configuration failed")
}
