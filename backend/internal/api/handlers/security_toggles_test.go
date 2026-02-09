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

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

func setupToggleTest(t *testing.T) (*SecurityHandler, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	// Create default SecurityConfig
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true}).Error)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil) // caddyManager nil to avoid reload logic
	return h, db
}

func TestSecurityToggles(t *testing.T) {
	h, db := setupToggleTest(t)

	tests := []struct {
		name       string
		method     string
		path       string
		handler    gin.HandlerFunc
		settingKey string
		expectVal  string
		body       string
	}{
		// ACL
		{"EnableACL", "POST", "/api/v1/security/acl/enable", h.EnableACL, "security.acl.enabled", "true", ""},
		{"DisableACL", "POST", "/api/v1/security/acl/disable", h.DisableACL, "security.acl.enabled", "false", ""},
		// ACL Patch
		{"PatchACL_True", "PATCH", "/api/v1/security/acl", h.PatchACL, "security.acl.enabled", "true", `{"enabled": true}`},
		{"PatchACL_False", "PATCH", "/api/v1/security/acl", h.PatchACL, "security.acl.enabled", "false", `{"enabled": false}`},

		// WAF
		{"EnableWAF", "POST", "/api/v1/security/waf/enable", h.EnableWAF, "security.waf.enabled", "true", ""},
		{"DisableWAF", "POST", "/api/v1/security/waf/disable", h.DisableWAF, "security.waf.enabled", "false", ""},
		// WAF Patch
		{"PatchWAF_True", "PATCH", "/api/v1/security/waf", h.PatchWAF, "security.waf.enabled", "true", `{"enabled": true}`},
		{"PatchWAF_False", "PATCH", "/api/v1/security/waf", h.PatchWAF, "security.waf.enabled", "false", `{"enabled": false}`},

		// Cerberus
		{"EnableCerberus", "POST", "/api/v1/security/cerberus/enable", h.EnableCerberus, "feature.cerberus.enabled", "true", ""},
		{"DisableCerberus", "POST", "/api/v1/security/cerberus/disable", h.DisableCerberus, "feature.cerberus.enabled", "false", ""},

		// CrowdSec
		{"EnableCrowdSec", "POST", "/api/v1/security/crowdsec/enable", h.EnableCrowdSec, "security.crowdsec.enabled", "true", ""},
		{"DisableCrowdSec", "POST", "/api/v1/security/crowdsec/disable", h.DisableCrowdSec, "security.crowdsec.enabled", "false", ""},
		// CrowdSec Patch
		{"PatchCrowdSec_True", "PATCH", "/api/v1/security/crowdsec", h.PatchCrowdSec, "security.crowdsec.enabled", "true", `{"enabled": true}`},
		{"PatchCrowdSec_False", "PATCH", "/api/v1/security/crowdsec", h.PatchCrowdSec, "security.crowdsec.enabled", "false", `{"enabled": false}`},

		// RateLimit
		{"EnableRateLimit", "POST", "/api/v1/security/rate-limit/enable", h.EnableRateLimit, "security.rate_limit.enabled", "true", ""},
		{"DisableRateLimit", "POST", "/api/v1/security/rate-limit/disable", h.DisableRateLimit, "security.rate_limit.enabled", "false", ""},
		// RateLimit Patch
		{"PatchRateLimit_True", "PATCH", "/api/v1/security/rate-limit", h.PatchRateLimit, "security.rate_limit.enabled", "true", `{"enabled": true}`},
		{"PatchRateLimit_False", "PATCH", "/api/v1/security/rate-limit", h.PatchRateLimit, "security.rate_limit.enabled", "false", `{"enabled": false}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			var req *http.Request
			if tc.body != "" {
				req, _ = http.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(tc.method, tc.path, nil)
			}

			c, _ := gin.CreateTestContext(w)
			c.Request = req
			// Mock Admin Role
			c.Set("role", "admin")

			tc.handler(c)

			require.Equal(t, http.StatusOK, w.Code)

			// Verify Setting
			var setting models.Setting
			err := db.Where("key = ?", tc.settingKey).First(&setting).Error
			assert.NoError(t, err)
			assert.Equal(t, tc.expectVal, setting.Value)
		})
	}
}

func TestSecurityToggles_Forbidden(t *testing.T) {
	h, _ := setupToggleTest(t)

	// Just test one endpoint to verify role check
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/security/acl/enable", nil)
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	// No role set

	h.EnableACL(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPatchACL_InvalidBody(t *testing.T) {
	h, _ := setupToggleTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/acl", strings.NewReader("invalid"))
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	h.PatchACL(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchWAF_InvalidBody(t *testing.T) {
	h, _ := setupToggleTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/waf", strings.NewReader("invalid"))
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	h.PatchWAF(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchRateLimit_InvalidBody(t *testing.T) {
	h, _ := setupToggleTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/rate-limit", strings.NewReader("invalid"))
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	h.PatchRateLimit(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchCrowdSec_InvalidBody(t *testing.T) {
	h, _ := setupToggleTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/crowdsec", strings.NewReader("invalid"))
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	h.PatchCrowdSec(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestACLForbiddenIfIPNotWhitelisted(t *testing.T) {
	h, db := setupToggleTest(t)

	// Update config to have whitelist
	err := db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Update("admin_whitelist", "10.0.0.1").Error
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/security/acl/enable", nil)
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")
	c.Request.RemoteAddr = "192.168.1.5:1234" // Different IP

	h.EnableACL(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestACLEnabledIfIPWhitelisted(t *testing.T) {
	h, db := setupToggleTest(t)

	// Update config to have whitelist
	err := db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Update("admin_whitelist", "1.2.3.4").Error
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/security/acl/enable", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4") // Trusted proxy simulation needed or direct RemoteAddr
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Request.RemoteAddr = "1.2.3.4:1234"
	c.Set("role", "admin")

	h.EnableACL(c)

	assert.Equal(t, http.StatusOK, w.Code)
}
