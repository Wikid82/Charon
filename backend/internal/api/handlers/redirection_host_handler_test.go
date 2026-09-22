package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRedirectionHostHandlerRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RedirectionHost{}, &models.ProxyHost{}, &models.Location{}, &models.SSLCertificate{}, &models.DNSProvider{}))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewRedirectionHostHandler(db, nil) // nil caddyManager: ApplyConfig calls are skipped in these unit tests
	grp := router.Group("/")
	h.RegisterRoutes(grp)
	return router, db
}

func validRedirectionHostPayload() map[string]any {
	return map[string]any{
		"name":              "Old blog redirect",
		"domain_names":      "old-blog.example.com",
		"target_url":        "https://newblog.example.com",
		"status_code":       301,
		"preserve_path":     true,
		"ssl_forced":        true,
		"http2_support":     true,
		"hsts_enabled":      false,
		"hsts_subdomains":   false,
		"enabled":           true,
		"certificate_id":    nil,
		"dns_provider_id":   nil,
		"use_dns_challenge": false,
	}
}

func TestRedirectionHostHandler_List_Empty(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodGet, "/redirection-hosts", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var result []any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Empty(t, result)
}

func TestRedirectionHostHandler_Create_Valid(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	assert.Equal(t, http.StatusCreated, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "old-blog.example.com", result["domain_names"])
	assert.Equal(t, "https://newblog.example.com", result["target_url"])
	assert.Equal(t, float64(301), result["status_code"])
	assert.NotEmpty(t, result["uuid"])
	assert.Nil(t, result["id"]) // json:"-" on ID — never exposed
}

// TestRedirectionHostHandler_Create_PersistsExplicitFalseBooleans is a
// regression test for the GORM bool-zero-value/`gorm:"default:true"`
// collision: PreservePath/SSLForced/HTTP2Support/Enabled are non-pointer
// bools with a `default:true` tag, so an explicit `false` on the create
// payload is indistinguishable from an omitted field and was silently
// overridden to `true` by GORM on INSERT. Confirms both the API response
// AND the persisted DB row reflect the explicit `false` values.
func TestRedirectionHostHandler_Create_PersistsExplicitFalseBooleans(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)

	payload := validRedirectionHostPayload()
	payload["preserve_path"] = false
	payload["ssl_forced"] = false
	payload["http2_support"] = false
	payload["enabled"] = false

	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	require.Equal(t, http.StatusCreated, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, false, result["preserve_path"], "response body must reflect explicit false, not silently defaulted to true")
	assert.Equal(t, false, result["ssl_forced"], "response body must reflect explicit false, not silently defaulted to true")
	assert.Equal(t, false, result["http2_support"], "response body must reflect explicit false, not silently defaulted to true")
	assert.Equal(t, false, result["enabled"], "response body must reflect explicit false, not silently defaulted to true")

	// Read back from the DB directly to rule out an in-memory-only struct
	// value that never actually made it past GORM's INSERT.
	var persisted models.RedirectionHost
	require.NoError(t, db.Where("uuid = ?", result["uuid"]).First(&persisted).Error)
	assert.False(t, persisted.PreservePath, "preserve_path=false must be persisted, not overridden by gorm default:true")
	assert.False(t, persisted.SSLForced, "ssl_forced=false must be persisted, not overridden by gorm default:true")
	assert.False(t, persisted.HTTP2Support, "http2_support=false must be persisted, not overridden by gorm default:true")
	assert.False(t, persisted.Enabled, "enabled=false must be persisted, not overridden by gorm default:true")
}

// TestRedirectionHostHandler_Create_DefaultsBooleansWhenOmitted confirms the
// fix does not regress the "omitted → default true" behavior the
// `gorm:"default:true"` tags used to provide for these fields.
func TestRedirectionHostHandler_Create_DefaultsBooleansWhenOmitted(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)

	payload := validRedirectionHostPayload()
	delete(payload, "preserve_path")
	delete(payload, "ssl_forced")
	delete(payload, "http2_support")
	delete(payload, "enabled")

	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	require.Equal(t, http.StatusCreated, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["preserve_path"])
	assert.Equal(t, true, result["ssl_forced"])
	assert.Equal(t, true, result["http2_support"])
	assert.Equal(t, true, result["enabled"])

	var persisted models.RedirectionHost
	require.NoError(t, db.Where("uuid = ?", result["uuid"]).First(&persisted).Error)
	assert.True(t, persisted.PreservePath)
	assert.True(t, persisted.SSLForced)
	assert.True(t, persisted.HTTP2Support)
	assert.True(t, persisted.Enabled)
}

func TestRedirectionHostHandler_Create_InvalidJSON_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	req, _ := http.NewRequest(http.MethodPost, "/redirection-hosts", bytes.NewBufferString("not valid json{{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRedirectionHostHandler_Create_MissingDomainNames_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["domain_names"] = ""
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "domain names is required")
}

func TestRedirectionHostHandler_Create_MissingTargetURL_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["target_url"] = ""
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "target_url is required")
}

func TestRedirectionHostHandler_Create_InvalidStatusCode_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["status_code"] = 303
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "status_code must be one of 301, 302, 307, 308")
}

func TestRedirectionHostHandler_Create_SelfRedirect_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["domain_names"] = "loop.example.com"
	payload["target_url"] = "https://loop.example.com"
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "redirect target cannot point back to one of this host's own domains")
}

func TestRedirectionHostHandler_Create_DuplicateDomain_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)

	w2 := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestRedirectionHostHandler_Create_ConflictsWithProxyHostDomain_400(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID:        "ph-1",
		DomainNames: "claimed.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}).Error)

	payload := validRedirectionHostPayload()
	payload["domain_names"] = "claimed.example.com"
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "domain already in use by another host")
}

func TestRedirectionHostHandler_Create_WithCertificateUUID(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	cert := &models.SSLCertificate{UUID: "cert-uuid-1", Provider: "custom"}
	require.NoError(t, db.Create(cert).Error)

	payload := validRedirectionHostPayload()
	payload["certificate_id"] = "cert-uuid-1"
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusCreated, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.NotNil(t, result["certificate_id"])
}

func TestRedirectionHostHandler_Create_UnknownCertificateUUID_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["certificate_id"] = "does-not-exist"
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRedirectionHostHandler_Get_Found(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := doRequest(router, http.MethodGet, "/redirection-hosts/"+created["uuid"].(string), nil)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestRedirectionHostHandler_Get_NotFound_404(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodGet, "/redirection-hosts/nonexistent-uuid", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "redirection host not found", result["error"])
}

func TestRedirectionHostHandler_Update_PartialFields(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := doRequest(router, http.MethodPut, "/redirection-hosts/"+created["uuid"].(string), map[string]any{
		"status_code": 308,
	})
	assert.Equal(t, http.StatusOK, w2.Code)
	var updated map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &updated))
	assert.Equal(t, float64(308), updated["status_code"])
	// Untouched fields preserved
	assert.Equal(t, "old-blog.example.com", updated["domain_names"])
}

func TestRedirectionHostHandler_Update_NotFound_404(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPut, "/redirection-hosts/nonexistent-uuid", map[string]any{"name": "New"})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRedirectionHostHandler_Update_InvalidJSON_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	req, _ := http.NewRequest(http.MethodPut, "/redirection-hosts/"+created["uuid"].(string), bytes.NewBufferString("not valid json{{"))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestRedirectionHostHandler_Update_InvalidStatusCodeType_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := doRequest(router, http.MethodPut, "/redirection-hosts/"+created["uuid"].(string), map[string]any{
		"status_code": []any{"not-a-number"},
	})
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestRedirectionHostHandler_Update_RejectsUnknownDNSProviderUUID_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := doRequest(router, http.MethodPut, "/redirection-hosts/"+created["uuid"].(string), map[string]any{
		"dns_provider_id": "missing-provider",
	})
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestRedirectionHostHandler_Delete_OK(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := doRequest(router, http.MethodDelete, "/redirection-hosts/"+created["uuid"].(string), nil)
	assert.Equal(t, http.StatusOK, w2.Code)

	var count int64
	db.Model(&models.RedirectionHost{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestRedirectionHostHandler_Delete_NotFound_404(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodDelete, "/redirection-hosts/nonexistent-uuid", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRedirectionHostHandler_List_ServiceError_500(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	require.NoError(t, db.Migrator().DropTable(&models.RedirectionHost{}))
	w := doRequest(router, http.MethodGet, "/redirection-hosts", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
