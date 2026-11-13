package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

const aclGuardRemoteAddr = "192.0.2.10"

// aclGuardDB opens a migrated test database and seeds the default security config with the given admin allow-list.
// A nil whitelist pointer leaves the default row absent.
func aclGuardDB(t *testing.T, whitelist *string) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}, &models.SecurityAudit{}))
	if whitelist != nil {
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, AdminWhitelist: *whitelist}).Error)
	}
	return db
}

// aclGuardContext builds an admin request from the given remote address with an optional JSON body.
func aclGuardContext(t *testing.T, method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = aclGuardRemoteAddr + ":4242"
	c.Request = req
	c.Set("role", "admin")
	c.Set("userID", uint(1))
	return c, w
}

func aclGuardStoredValue(db *gorm.DB) string {
	var s models.Setting
	if err := db.Where("key = ?", models.SecurityACLEnabledSettingKey).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

func ptrString(s string) *string { return &s }

func TestACLEnableGuard_HelperAllowsWhenPolicyPermits(t *testing.T) {
	tests := []struct {
		name      string
		whitelist *string
		emergency bool
	}{
		{name: "no default row", whitelist: nil},
		{name: "empty allow-list", whitelist: ptrString("")},
		{name: "whitespace-only allow-list", whitelist: ptrString("   ")},
		{name: "client address listed", whitelist: ptrString("192.0.2.0/24")},
		{name: "emergency context with non-matching list", whitelist: ptrString("203.0.113.0/24"), emergency: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := aclGuardDB(t, tt.whitelist)
			c, w := aclGuardContext(t, http.MethodPost, "")
			if tt.emergency {
				c.Set(middleware.EmergencyBypassContextKey, true)
			}
			assert.True(t, allowACLEnable(c, db))
			assert.Equal(t, http.StatusOK, w.Code, "helper must not write a response when allowing")
		})
	}
}

func TestACLEnableGuard_HelperRefusesOutsideAllowList(t *testing.T) {
	db := aclGuardDB(t, ptrString("203.0.113.0/24"))
	c, w := aclGuardContext(t, http.MethodPost, "")
	assert.False(t, allowACLEnable(c, db))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "admin IP not present in admin_whitelist")
}

func TestACLEnableGuard_HelperReportsUnreadableConfig(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	c, w := aclGuardContext(t, http.MethodPost, "")
	assert.False(t, allowACLEnable(c, db))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestACLEnableGuard_ToggleSecurityModule covers the toggle route, which calls the helper before writing.
func TestACLEnableGuard_ToggleSecurityModule(t *testing.T) {
	tests := []struct {
		name       string
		whitelist  string
		enable     bool
		wantStatus int
		wantStored string
	}{
		{name: "empty allow-list allows", whitelist: "", enable: true, wantStatus: http.StatusOK, wantStored: "true"},
		{name: "client address listed allows", whitelist: "192.0.2.0/24", enable: true, wantStatus: http.StatusOK, wantStored: "true"},
		{name: "client address not listed refuses", whitelist: "203.0.113.0/24", enable: true, wantStatus: http.StatusForbidden},
		{name: "disabling is never refused", whitelist: "203.0.113.0/24", enable: false, wantStatus: http.StatusOK, wantStored: "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := aclGuardDB(t, &tt.whitelist)
			h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
			t.Cleanup(h.Close)

			c, w := aclGuardContext(t, http.MethodPost, "")
			if tt.enable {
				h.EnableACL(c)
			} else {
				h.DisableACL(c)
			}

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStored != "" {
				assert.Equal(t, tt.wantStored, aclGuardStoredValue(db))
			} else {
				assert.NotEqual(t, "true", aclGuardStoredValue(db))
			}
		})
	}
}

// TestACLEnableGuard_UpdateSetting covers the generic settings key form of the ACL switch.
func TestACLEnableGuard_UpdateSetting(t *testing.T) {
	tests := []struct {
		name       string
		whitelist  string
		value      string
		wantStatus int
	}{
		{name: "empty allow-list allows", whitelist: "", value: "true", wantStatus: http.StatusOK},
		{name: "client address listed allows", whitelist: "192.0.2.0/24", value: "true", wantStatus: http.StatusOK},
		{name: "client address not listed refuses", whitelist: "203.0.113.0/24", value: "true", wantStatus: http.StatusForbidden},
		{name: "disabling is never refused", whitelist: "203.0.113.0/24", value: "false", wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := aclGuardDB(t, &tt.whitelist)
			h := NewSettingsHandlerWithDeps(db, nil, nil, nil, "")

			body := `{"key":"security.acl.enabled","value":"` + tt.value + `"}`
			c, w := aclGuardContext(t, http.MethodPost, body)
			h.UpdateSetting(c)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				assert.Equal(t, tt.value, aclGuardStoredValue(db))
			} else {
				assert.NotEqual(t, "true", aclGuardStoredValue(db))
			}
		})
	}
}

// TestACLEnableGuard_PatchConfig covers the nested bulk config form of the ACL switch.
func TestACLEnableGuard_PatchConfig(t *testing.T) {
	tests := []struct {
		name       string
		whitelist  string
		value      string
		wantStatus int
	}{
		{name: "empty allow-list allows", whitelist: "", value: "true", wantStatus: http.StatusOK},
		{name: "client address listed allows", whitelist: "192.0.2.0/24", value: "true", wantStatus: http.StatusOK},
		{name: "client address not listed refuses", whitelist: "203.0.113.0/24", value: "true", wantStatus: http.StatusForbidden},
		{name: "disabling is never refused", whitelist: "203.0.113.0/24", value: "false", wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := aclGuardDB(t, &tt.whitelist)
			h := NewSettingsHandlerWithDeps(db, nil, nil, nil, "")

			body := `{"security":{"acl":{"enabled":` + tt.value + `}}}`
			c, w := aclGuardContext(t, http.MethodPatch, body)
			h.PatchConfig(c)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				assert.Equal(t, tt.value, aclGuardStoredValue(db))
			} else {
				assert.NotEqual(t, "true", aclGuardStoredValue(db))
			}
		})
	}
}
