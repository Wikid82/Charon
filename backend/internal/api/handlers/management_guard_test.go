package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
	"github.com/Wikid82/charon/backend/internal/services"
)

// guardRouter mounts the handler with an administrator caller at aclGuardRemoteAddr.
func guardRouter(t *testing.T, method, path string, handler gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		middleware.SetCaller(c, 1, string(models.RoleAdmin))
		c.Next()
	})
	r.Handle(method, path, handler)
	return r
}

func guardDo(t *testing.T, r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = aclGuardRemoteAddr + ":4242"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func guardCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	code, _ := body["error_code"].(string)
	return code
}

func TestManagementAccessGet_ReportsChoiceAndEnforcement(t *testing.T) {
	db := aclGuardDB(t, ptrString("192.0.2.0/24"))
	require.NoError(t, db.Create(&models.Setting{Key: models.SecurityACLEnabledSettingKey, Value: "true", Category: "security", Type: "bool"}).Error)
	h := NewManagementAccessHandler(db, "", nil)
	r := guardRouter(t, http.MethodGet, "/security/management-access", h.Get)

	w := guardDo(t, r, http.MethodGet, "/security/management-access", "")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Lists    []managementListSummary `json:"lists"`
		Enforced bool                    `json:"enforced"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Lists, 1)
	assert.Equal(t, "guard-list", body.Lists[0].UUID)
	assert.True(t, body.Enforced)
}

func TestManagementAccessGet_NoChoiceNotEnforced(t *testing.T) {
	db := aclGuardDB(t, nil)
	h := NewManagementAccessHandler(db, "enabled", nil)
	r := guardRouter(t, http.MethodGet, "/security/management-access", h.Get)

	w := guardDo(t, r, http.MethodGet, "/security/management-access", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"lists":[],"enforced":false}`, w.Body.String())
}

func TestManagementAccessGet_ReadErrorIs500(t *testing.T) {
	db := aclGuardDB(t, nil)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	h := NewManagementAccessHandler(db, "", nil)
	w := guardDo(t, guardRouter(t, http.MethodGet, "/security/management-access", h.Get), http.MethodGet, "/security/management-access", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestManagementAccessPut_Guard1RefusesLockout(t *testing.T) {
	db := aclGuardDB(t, nil)
	require.NoError(t, db.Create(&models.Setting{Key: models.SecurityACLEnabledSettingKey, Value: "true", Category: "security", Type: "bool"}).Error)
	require.NoError(t, db.Create(&models.AccessList{UUID: "elsewhere", Name: "elsewhere", Type: "whitelist", Enabled: true, IPRules: `[{"cidr":"203.0.113.0/24"}]`}).Error)
	h := NewManagementAccessHandler(db, "", nil)
	r := guardRouter(t, http.MethodPut, "/security/management-access", h.Put)

	w := guardDo(t, r, http.MethodPut, "/security/management-access", `{"list_uuids":["elsewhere"]}`)
	require.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, codeManagementLockout, guardCode(t, w))
	stored, err := services.ManagementListUUIDs(db)
	require.NoError(t, err)
	assert.Empty(t, stored, "a refused choice is not written")
}

func TestManagementAccessPut_SavesAndAnswers(t *testing.T) {
	db := aclGuardDB(t, nil)
	require.NoError(t, db.Create(&models.AccessList{UUID: "allowed", Name: "allowed", Type: "whitelist", Enabled: true, IPRules: `[{"cidr":"192.0.2.0/24"}]`}).Error)
	require.NoError(t, db.Create(&models.AccessList{UUID: "other", Name: "other", Type: "blacklist", Enabled: true, IPRules: `[{"cidr":"198.51.100.0/24"}]`}).Error)
	cache := &guardCacheCounter{}
	h := NewManagementAccessHandler(db, "", cache)
	r := guardRouter(t, http.MethodPut, "/security/management-access", h.Put)

	w := guardDo(t, r, http.MethodPut, "/security/management-access", `{"list_uuids":["other","allowed","other"]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	stored, err := services.ManagementListUUIDs(db)
	require.NoError(t, err)
	assert.Equal(t, []string{"allowed", "other"}, stored)
	assert.Equal(t, 1, cache.calls, "the Cerberus cache is refreshed")
}

func TestManagementAccessPut_UnknownListIs400(t *testing.T) {
	db := aclGuardDB(t, nil)
	h := NewManagementAccessHandler(db, "", nil)
	r := guardRouter(t, http.MethodPut, "/security/management-access", h.Put)

	w := guardDo(t, r, http.MethodPut, "/security/management-access", `{"list_uuids":["missing"]}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, codeManagementListUnknown, guardCode(t, w))
	assert.Contains(t, w.Body.String(), `"uuid":"missing"`)
}

func TestManagementAccessPut_MalformedBodyIs400(t *testing.T) {
	db := aclGuardDB(t, nil)
	h := NewManagementAccessHandler(db, "", nil)
	r := guardRouter(t, http.MethodPut, "/security/management-access", h.Put)
	assert.Equal(t, http.StatusBadRequest, guardDo(t, r, http.MethodPut, "/security/management-access", `{"list_uuids":`).Code)
}

func TestManagementAccessPut_WriteErrorIs500(t *testing.T) {
	db := aclGuardDB(t, nil)
	h := NewManagementAccessHandler(db, "", nil)
	r := guardRouter(t, http.MethodPut, "/security/management-access", h.Put)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	assert.Equal(t, http.StatusInternalServerError, guardDo(t, r, http.MethodPut, "/security/management-access", `{"list_uuids":[]}`).Code)
}

func TestAccessListUpdate_Guard3RefusesLockoutOfManagedList(t *testing.T) {
	db := aclGuardDB(t, ptrString("192.0.2.0/24"))
	h := NewAccessListHandlerWithDeps(db, nil).WithACLMode("enabled")
	r := guardRouter(t, http.MethodPut, "/access-lists/:id", h.Update)

	body := `{"name":"guard","type":"whitelist","enabled":true,"ip_rules":"[{\"cidr\":\"203.0.113.0/24\"}]"}`
	w := guardDo(t, r, http.MethodPut, "/access-lists/guard-list", body)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, codeManagementLockout, guardCode(t, w))

	var stored models.AccessList
	require.NoError(t, db.Where("uuid = ?", "guard-list").First(&stored).Error)
	assert.Contains(t, stored.IPRules, "192.0.2.0/24", "a refused edit is not saved")
}

func TestAccessListUpdate_Guard3AllowsEditThatKeepsCaller(t *testing.T) {
	db := aclGuardDB(t, ptrString("192.0.2.0/24"))
	h := NewAccessListHandlerWithDeps(db, nil).WithACLMode("enabled")
	r := guardRouter(t, http.MethodPut, "/access-lists/:id", h.Update)

	body := `{"name":"guard-renamed","type":"whitelist","enabled":true,"ip_rules":"[{\"cidr\":\"192.0.2.0/24\"}]"}`
	w := guardDo(t, r, http.MethodPut, "/access-lists/guard-list", body)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestAccessListUpdate_Guard3DisablingIsNotRefused(t *testing.T) {
	db := aclGuardDB(t, ptrString("203.0.113.0/24"))
	h := NewAccessListHandlerWithDeps(db, nil).WithACLMode("enabled")
	r := guardRouter(t, http.MethodPut, "/access-lists/:id", h.Update)

	body := `{"name":"guard","type":"whitelist","enabled":false,"ip_rules":"[{\"cidr\":\"203.0.113.0/24\"}]"}`
	w := guardDo(t, r, http.MethodPut, "/access-lists/guard-list", body)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestAccessListDelete_ManagedListIs409(t *testing.T) {
	db := aclGuardDB(t, ptrString("192.0.2.0/24"))
	h := NewAccessListHandlerWithDeps(db, nil)
	r := guardRouter(t, http.MethodDelete, "/access-lists/:id", h.Delete)

	w := guardDo(t, r, http.MethodDelete, "/access-lists/guard-list", "")
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "Charon's own interface")
	assert.Equal(t, "access_list_in_use", guardCode(t, w))
}

func TestSecurityConfigUpdate_Guard4RefusesAdminAllowListLockout(t *testing.T) {
	db := aclGuardDB(t, ptrString("198.51.100.0/24"))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, AdminWhitelist: "192.0.2.10/32"}).Error)
	h := NewSecurityHandler(config.SecurityConfig{ACLMode: "enabled"}, db, nil)
	t.Cleanup(h.Close)
	r := guardRouter(t, http.MethodPost, "/security/config", h.UpdateConfig)

	w := guardDo(t, r, http.MethodPost, "/security/config", `{"admin_whitelist":"203.0.113.0/24"}`)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, codeManagementLockout, guardCode(t, w))
}

func TestSettingsAdminWhitelist_Guard4RefusesLockout(t *testing.T) {
	db := aclGuardDB(t, ptrString("198.51.100.0/24"))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, AdminWhitelist: "192.0.2.10/32"}).Error)
	h := NewSettingsHandlerWithDeps(db, nil, nil, nil, "").WithACLMode("enabled")
	r := guardRouter(t, http.MethodPost, "/settings", h.UpdateSetting)

	w := guardDo(t, r, http.MethodPost, "/settings", `{"key":"security.admin_whitelist","value":"203.0.113.0/24"}`)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestSettingsPatchConfig_Guard4RefusesLockout(t *testing.T) {
	db := aclGuardDB(t, ptrString("198.51.100.0/24"))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, AdminWhitelist: "192.0.2.10/32"}).Error)
	h := NewSettingsHandlerWithDeps(db, nil, nil, nil, "").WithACLMode("enabled")
	r := guardRouter(t, http.MethodPatch, "/config", h.PatchConfig)

	w := guardDo(t, r, http.MethodPatch, "/config", `{"security":{"admin_whitelist":"203.0.113.0/24"}}`)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestRefuseAdminAllowList_UnreadableStateIs500(t *testing.T) {
	db := aclGuardDB(t, nil)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	c, w := aclGuardContext(t, http.MethodPost, "")
	assert.True(t, refuseAdminAllowList(c, db, "", "203.0.113.0/24"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestTrustChange_Guard5RecomputesCallerUnderNewTrust: a caller that reaches Charon through a proxy in a range the
// change stops trusting is judged from its proxy's own address, and a list that admits only the visitor refuses it.
func TestTrustChange_Guard5RecomputesCallerUnderNewTrust(t *testing.T) {
	db := aclGuardDB(t, ptrString("198.51.100.0/24"))
	secret, err := selfhop.NewSecret()
	require.NoError(t, err)
	h := NewTrustedProxyHandler(db, nil, nil, "enabled")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SelfHop(secret, security.TrustedProxyMatcher{}))
	r.Use(func(c *gin.Context) {
		middleware.SetCaller(c, 1, string(models.RoleAdmin))
		c.Next()
	})
	r.PUT("/security/trusted-proxies", h.Put)

	hop := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/security/trusted-proxies", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:4000"
		req.Header.Set(selfhop.HeaderSecret, secret.Reveal())
		req.Header.Set(selfhop.HeaderClient, "198.51.100.7")
		req.Header.Set(selfhop.HeaderPeer, "10.0.0.2")
		req.Header.Set(selfhop.HeaderForwarded, "198.51.100.7")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// Trusting the proxy keeps the visitor, who is listed, as the caller.
	kept := hop(`{"proxies":["10.0.0.0/8"]}`)
	assert.Equal(t, http.StatusOK, kept.Code, kept.Body.String())

	// Dropping the proxy makes the proxy's own address the caller, which the list does not admit.
	refused := hop(`{"proxies":[]}`)
	require.Equal(t, http.StatusForbidden, refused.Code, refused.Body.String())
	assert.Equal(t, codeManagementLockout, guardCode(t, refused))
	stored, err := models.ReadTrustedProxiesSetting(db)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "10.0.0.0/8", *stored, "the refused change is not written")
}

func TestCallerAddressUnder_DirectRequestKeepsConnectionAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	c.Request.RemoteAddr = "192.0.2.10:4000"
	assert.Equal(t, "192.0.2.10", callerAddressUnder(c, nil))
	assert.Equal(t, "192.0.2.10", callerAddress(c))
}

// guardCacheCounter counts cache refreshes.
type guardCacheCounter struct{ calls int }

func (g *guardCacheCounter) InvalidateCache() { g.calls++ }
