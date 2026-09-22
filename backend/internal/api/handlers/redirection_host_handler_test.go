package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
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

// --- parseStatusCodeField: direct table-driven coverage ---
//
// parseStatusCodeField's `case int` branch is unreachable via a real HTTP
// request (encoding/json always decodes JSON numbers into float64, never a
// Go int), so it can only be exercised by calling the function directly.
// This table also closes the float64-fractional-rejected and string
// valid/invalid branches, which the existing HTTP-level Update tests don't
// hit anywhere in the suite.
func TestParseStatusCodeField_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    int
		wantErr bool
	}{
		{name: "float64 valid whole number", input: float64(301), want: 301},
		{name: "float64 fractional rejected", input: float64(301.5), wantErr: true},
		{name: "int passthrough", input: 308, want: 308},
		{name: "string valid", input: "307", want: 307},
		{name: "string invalid", input: "not-a-number", wantErr: true},
		{name: "unsupported type", input: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStatusCodeField(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "status_code must be one of 301, 302, 307, 308")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- resolveCertificateReference / resolveDNSProviderReference coverage ---

func TestRedirectionHostHandler_Create_WithCertificateNumericID(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	cert := &models.SSLCertificate{UUID: "cert-numeric-1", Provider: "custom"}
	require.NoError(t, db.Create(cert).Error)

	payload := validRedirectionHostPayload()
	payload["certificate_id"] = cert.ID
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	require.Equal(t, http.StatusCreated, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, float64(cert.ID), result["certificate_id"])
}

func TestRedirectionHostHandler_Create_CertificateIDInvalidType_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["certificate_id"] = true
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "invalid certificate_id")
}

func TestRedirectionHostHandler_Create_CertificateLookupDBError_400(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	require.NoError(t, db.Migrator().DropTable(&models.SSLCertificate{}))

	payload := validRedirectionHostPayload()
	payload["certificate_id"] = "some-uuid-value"
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "failed to resolve certificate", result["error"])
}

func TestRedirectionHostHandler_Create_WithDNSProviderNumericID(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	provider := &models.DNSProvider{UUID: "dns-numeric-1", Name: "Test DNS", ProviderType: "cloudflare"}
	require.NoError(t, db.Create(provider).Error)

	payload := validRedirectionHostPayload()
	payload["dns_provider_id"] = provider.ID
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	require.Equal(t, http.StatusCreated, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, float64(provider.ID), result["dns_provider_id"])
}

func TestRedirectionHostHandler_Create_DNSProviderIDInvalidType_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["dns_provider_id"] = true
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "invalid dns_provider_id")
}

func TestRedirectionHostHandler_Create_DNSProviderLookupDBError_400(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	require.NoError(t, db.Migrator().DropTable(&models.DNSProvider{}))

	payload := validRedirectionHostPayload()
	payload["dns_provider_id"] = "some-uuid-value"
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "failed to resolve dns provider", result["error"])
}

func TestRedirectionHostHandler_Create_WithDNSProviderUUID_Success(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	provider := &models.DNSProvider{UUID: "dns-uuid-found", Name: "Test DNS", ProviderType: "cloudflare"}
	require.NoError(t, db.Create(provider).Error)

	payload := validRedirectionHostPayload()
	payload["dns_provider_id"] = "dns-uuid-found"
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	require.Equal(t, http.StatusCreated, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, float64(provider.ID), result["dns_provider_id"])
}

func TestRedirectionHostHandler_Create_UnknownDNSProviderUUID_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["dns_provider_id"] = "does-not-exist"
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "dns provider not found", result["error"])
}

// TestRedirectionHostHandler_Create_StatusCodeTypeMismatch_400 exercises
// Create's post-marshal json.Unmarshal(payloadBytes, &host) failure branch:
// models.RedirectionHost.StatusCode is a plain `int`, so a JSON string value
// (valid JSON, but the wrong Go type) fails to unmarshal into it, unlike the
// numeric-coercion path parseStatusCodeField handles for Update.
func TestRedirectionHostHandler_Create_StatusCodeTypeMismatch_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["status_code"] = "not-an-int"
	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "cannot unmarshal string")
}

// --- Create: ApplyConfig failure / rollback coverage ---

// setupRedirectionHostHandlerRouterWithCaddy wires a real (non-nil)
// caddy.Manager pointed at the given admin API URL, so Create/Update/Delete
// exercise their actual ApplyConfig call sites instead of skipping them.
func setupRedirectionHostHandlerRouterWithCaddy(t *testing.T, adminAPIURL string) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RedirectionHost{}, &models.ProxyHost{}, &models.Location{}, &models.SSLCertificate{}, &models.DNSProvider{}, &models.Setting{}, &models.CaddyConfig{}))

	client := caddy.NewClientWithExpectedPort(adminAPIURL, expectedPortFromURL(t, adminAPIURL))
	manager := caddy.NewManager(client, db, t.TempDir(), "", false, config.SecurityConfig{})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewRedirectionHostHandler(db, manager)
	grp := router.Group("/")
	h.RegisterRoutes(grp)
	return router, db
}

func failingCaddyServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRedirectionHostHandler_Create_ApplyConfigFailure_RollsBack(t *testing.T) {
	caddyServer := failingCaddyServer(t)
	router, db := setupRedirectionHostHandlerRouterWithCaddy(t, caddyServer.URL)

	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "Failed to apply configuration")

	var count int64
	db.Model(&models.RedirectionHost{}).Count(&count)
	assert.Equal(t, int64(0), count, "host must be rolled back when ApplyConfig fails")
}

// TestRedirectionHostHandler_Create_ApplyConfigFailure_RollbackAlsoFails
// covers the double-failure branch: ApplyConfig fails AND the rollback
// delete itself fails. The handler must still respond with the original
// ApplyConfig error (not panic, not mask it) and merely log the rollback
// failure as critical.
func TestRedirectionHostHandler_Create_ApplyConfigFailure_RollbackAlsoFails(t *testing.T) {
	caddyServer := failingCaddyServer(t)
	router, db := setupRedirectionHostHandlerRouterWithCaddy(t, caddyServer.URL)

	const hookName = "test_redirection_host_rollback_delete_failure"
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(hookName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "redirection_hosts" {
			_ = tx.AddError(fmt.Errorf("forced rollback delete failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Delete().Remove(hookName) })

	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "Failed to apply configuration")

	var count int64
	db.Model(&models.RedirectionHost{}).Count(&count)
	assert.Equal(t, int64(1), count, "row survives when the rollback delete itself also fails")
}

// --- Update: field-assignment, resolve, validation and ApplyConfig coverage ---

// TestRedirectionHostHandler_Update_AllFields exercises every simple
// field-assignment branch in Update (name/domain_names/target_url/booleans)
// plus both certificate_id and dns_provider_id resolving successfully by
// UUID, asserting both the JSON response and the persisted DB row.
func TestRedirectionHostHandler_Update_AllFields(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)

	cert := &models.SSLCertificate{UUID: "cert-update-all", Provider: "custom"}
	require.NoError(t, db.Create(cert).Error)
	provider := &models.DNSProvider{UUID: "dns-update-all", Name: "Test DNS", ProviderType: "cloudflare"}
	require.NoError(t, db.Create(provider).Error)

	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	updatePayload := map[string]any{
		"name":              "Renamed redirect",
		"domain_names":      "renamed.example.com",
		"target_url":        "https://renamed-target.example.com",
		"preserve_path":     false,
		"ssl_forced":        false,
		"http2_support":     false,
		"hsts_enabled":      true,
		"hsts_subdomains":   true,
		"enabled":           false,
		"use_dns_challenge": true,
		"certificate_id":    "cert-update-all",
		"dns_provider_id":   "dns-update-all",
	}
	w2 := doRequest(router, http.MethodPut, "/redirection-hosts/"+created["uuid"].(string), updatePayload)
	require.Equal(t, http.StatusOK, w2.Code)

	var updated map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &updated))
	assert.Equal(t, "Renamed redirect", updated["name"])
	assert.Equal(t, "renamed.example.com", updated["domain_names"])
	assert.Equal(t, "https://renamed-target.example.com", updated["target_url"])
	assert.Equal(t, false, updated["preserve_path"])
	assert.Equal(t, false, updated["ssl_forced"])
	assert.Equal(t, false, updated["http2_support"])
	assert.Equal(t, true, updated["hsts_enabled"])
	assert.Equal(t, true, updated["hsts_subdomains"])
	assert.Equal(t, false, updated["enabled"])
	assert.Equal(t, true, updated["use_dns_challenge"])
	assert.Equal(t, float64(cert.ID), updated["certificate_id"])
	assert.Equal(t, float64(provider.ID), updated["dns_provider_id"])

	var persisted models.RedirectionHost
	require.NoError(t, db.Where("uuid = ?", created["uuid"]).First(&persisted).Error)
	assert.Equal(t, "Renamed redirect", persisted.Name)
	assert.False(t, persisted.Enabled)
	require.NotNil(t, persisted.CertificateID)
	assert.Equal(t, cert.ID, *persisted.CertificateID)
	require.NotNil(t, persisted.DNSProviderID)
	assert.Equal(t, provider.ID, *persisted.DNSProviderID)
}

func TestRedirectionHostHandler_Update_RejectsUnknownCertificateUUID_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := doRequest(router, http.MethodPut, "/redirection-hosts/"+created["uuid"].(string), map[string]any{
		"certificate_id": "missing-cert",
	})
	assert.Equal(t, http.StatusBadRequest, w2.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &result))
	assert.Equal(t, "certificate not found", result["error"])
}

// TestRedirectionHostHandler_Update_ServiceValidationError_400 confirms
// Update surfaces the service layer's validation error (as opposed to the
// handler's own pre-checks like parseStatusCodeField) with a 400.
func TestRedirectionHostHandler_Update_ServiceValidationError_400(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := doRequest(router, http.MethodPut, "/redirection-hosts/"+created["uuid"].(string), map[string]any{
		"target_url": "ftp://invalid-scheme.example.com",
	})
	assert.Equal(t, http.StatusBadRequest, w2.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "target_url must be a valid http(s) URL")
}

func TestRedirectionHostHandler_Update_ApplyConfigFailure_500(t *testing.T) {
	caddyServer := failingCaddyServer(t)
	router, db := setupRedirectionHostHandlerRouterWithCaddy(t, caddyServer.URL)

	// Insert directly, bypassing the Create handler (which would itself
	// fail against this same failing caddy server).
	host := models.RedirectionHost{
		UUID:        "rh-update-apply-fail",
		DomainNames: "update-apply-fail.example.com",
		TargetURL:   "https://target.example.com",
		StatusCode:  301,
		Enabled:     true,
	}
	require.NoError(t, db.Create(&host).Error)

	w := doRequest(router, http.MethodPut, "/redirection-hosts/"+host.UUID, map[string]any{"name": "Renamed"})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "Failed to apply configuration")
}

// --- Delete: service error and ApplyConfig failure coverage ---

func TestRedirectionHostHandler_Delete_ServiceError_500(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)

	host := models.RedirectionHost{
		UUID:        "rh-delete-service-fail",
		DomainNames: "delete-service-fail.example.com",
		TargetURL:   "https://target.example.com",
		StatusCode:  301,
		Enabled:     true,
	}
	require.NoError(t, db.Create(&host).Error)

	const hookName = "test_redirection_host_delete_service_error"
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(hookName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "redirection_hosts" {
			_ = tx.AddError(fmt.Errorf("forced delete failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Delete().Remove(hookName) })

	w := doRequest(router, http.MethodDelete, "/redirection-hosts/"+host.UUID, nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRedirectionHostHandler_Delete_ApplyConfigFailure_500(t *testing.T) {
	caddyServer := failingCaddyServer(t)
	router, db := setupRedirectionHostHandlerRouterWithCaddy(t, caddyServer.URL)

	host := models.RedirectionHost{
		UUID:        "rh-delete-apply-fail",
		DomainNames: "delete-apply-fail.example.com",
		TargetURL:   "https://target.example.com",
		StatusCode:  301,
		Enabled:     true,
	}
	require.NoError(t, db.Create(&host).Error)

	w := doRequest(router, http.MethodDelete, "/redirection-hosts/"+host.UUID, nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "Failed to apply configuration")

	// Delete's own DB deletion is not rolled back on ApplyConfig failure
	// (unlike Create) — matches the handler's existing Delete semantics.
	var count int64
	db.Model(&models.RedirectionHost{}).Count(&count)
	assert.Equal(t, int64(0), count)
}
