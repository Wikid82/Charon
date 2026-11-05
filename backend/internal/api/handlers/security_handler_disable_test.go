package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
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

// disableCallers are the two ways Disable accepts a request: from localhost with no token, or from a remote address
// with the break-glass token.
var disableCallers = []struct {
	name  string
	token bool
}{
	{name: "localhost", token: false},
	{name: "token", token: true},
}

// disableRequest returns the remote address and body for one caller. The token caller gets a fresh break-glass token.
func disableRequest(t *testing.T, h *SecurityHandler, token bool) (remoteAddr, body string) {
	t.Helper()
	if !token {
		return "127.0.0.1:5000", `{}`
	}
	tok, err := h.svc.GenerateBreakGlassToken(models.DefaultSecurityConfigName)
	require.NoError(t, err)
	return "198.51.100.7:5000", `{"break_glass_token":"` + tok + `"}`
}

// settingValueOf reads one settings key, failing when it is not stored.
func settingValueOf(t *testing.T, db *gorm.DB, key string) string {
	t.Helper()
	var setting models.Setting
	require.NoError(t, db.Where("key = ?", key).First(&setting).Error)
	return setting.Value
}

// failingApplyManager stands in for the proxy apply and always refuses it. beforeFail, when set, runs first so a test can
// simulate a concurrent write that lands while the apply is in flight.
type failingApplyManager struct {
	beforeFail func()
}

func (m failingApplyManager) ApplyConfig(context.Context) error {
	if m.beforeFail != nil {
		m.beforeFail()
	}
	return errors.New("caddy reload refused")
}

func TestSecurityHandler_Disable_ClearsStaleCerberusKeys(t *testing.T) {
	for _, caller := range disableCallers {
		t.Run(caller.name, func(t *testing.T) {
			setReaderEnv(t, "")
			db := seedReaderState(t, "enabled", "true", "true")
			static := readerStatics["off"]
			h := NewSecurityHandler(static, db, nil)
			t.Cleanup(h.Close)

			remoteAddr, body := disableRequest(t, h, caller.token)
			w := serveDisable(t, h, remoteAddr, body)
			require.Equal(t, http.StatusOK, w.Code)

			assert.Equal(t, "false", settingValueOf(t, db, models.CerberusFeatureSettingKey))
			assert.Equal(t, "false", settingValueOf(t, db, models.CerberusLegacySettingKey))
			stored, err := models.FindDefaultSecurityConfig(db)
			require.NoError(t, err)
			assert.False(t, stored.Enabled)
			assert.Equal(t, readerAnswers{proxy: false, status: false, middleware: false, presets: false},
				readReaderAnswers(t, db, static))
		})
	}
}

func TestSecurityHandler_Disable_FreshInstallStillDisables(t *testing.T) {
	setReaderEnv(t, "")
	db := seedReaderState(t, "absent", readerKeyAbsent, readerKeyAbsent)
	h := NewSecurityHandler(readerStatics["off"], db, nil)
	t.Cleanup(h.Close)

	w := serveDisable(t, h, "127.0.0.1:5000", `{}`)
	require.Equal(t, http.StatusOK, w.Code)

	stored, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.False(t, stored.Enabled)
	assert.Equal(t, "false", settingValueOf(t, db, models.CerberusFeatureSettingKey))
	assert.Equal(t, "false", settingValueOf(t, db, models.CerberusLegacySettingKey))
}

func TestSecurityHandler_Disable_RejectedApplyRestoresKeysAndRow(t *testing.T) {
	for _, caller := range disableCallers {
		t.Run(caller.name, func(t *testing.T) {
			setReaderEnv(t, "")
			db := seedReaderState(t, "enabled", "true", "true")
			h := NewSecurityHandler(readerStatics["off"], db, failingApplyManager{})
			t.Cleanup(h.Close)

			remoteAddr, body := disableRequest(t, h, caller.token)
			w := serveDisable(t, h, remoteAddr, body)
			require.Equal(t, http.StatusInternalServerError, w.Code)
			assert.Contains(t, w.Body.String(), "saved, but applying the configuration failed")

			assert.Equal(t, "true", settingValueOf(t, db, models.CerberusFeatureSettingKey))
			assert.Equal(t, "true", settingValueOf(t, db, models.CerberusLegacySettingKey))
			stored, err := models.FindDefaultSecurityConfig(db)
			require.NoError(t, err)
			assert.True(t, stored.Enabled)
		})
	}
}

func TestSecurityHandler_Disable_RestoreSkippedWarnsOnConcurrentWrite(t *testing.T) {
	setReaderEnv(t, "")
	db := seedReaderState(t, "enabled", "true", "true")
	// The concurrent writer changes the feature key while the apply is in flight. The restore must keep that value.
	concurrent := failingApplyManager{beforeFail: func() {
		require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", models.CerberusFeatureSettingKey).
			Update("value", "garbage").Error)
	}}
	h := NewSecurityHandler(readerStatics["off"], db, concurrent)
	t.Cleanup(h.Close)
	hook := logtest.NewLocal(logger.Log().Logger)

	w := serveDisable(t, h, "127.0.0.1:5000", `{}`)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	assert.Equal(t, "garbage", settingValueOf(t, db, models.CerberusFeatureSettingKey))
	assert.Equal(t, "true", settingValueOf(t, db, models.CerberusLegacySettingKey))
	var warned bool
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "security state restore skipped") {
			warned = true
		}
	}
	assert.True(t, warned, "a skipped restore must log a warning")
}
