package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

const redactionTestEncryptionKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

// roleStampRouter returns a gin engine whose requests carry the given role,
// standing in for the auth middleware.
func roleStampRouter(role string) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.RoleKey, role)
		c.Next()
	})
	return r
}

func getJSON(t *testing.T, r http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func mustMarshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestPermissionJSONForRoleWritesRedactedBodyOrServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("non-admin body is redacted", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Set(middleware.RoleKey, "user")

		jsonForRole(ctx, http.StatusOK, permissionRedactSample{Name: "a", Secret: "s"}, []string{"secret"}, nil)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NotContains(t, rec.Body.String(), "secret")
	})

	t.Run("encoding failure is a server error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Set(middleware.RoleKey, "user")

		jsonForRole(ctx, http.StatusOK, make(chan int), nil, nil)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("list encoding failure is a server error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Set(middleware.RoleKey, "user")

		jsonListForRole(ctx, http.StatusOK, []chan int{make(chan int)}, nil, nil)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("admin list is written as is", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Set(middleware.RoleKey, "admin")

		jsonListForRole(ctx, http.StatusOK, []permissionRedactSample{{Name: "a", Secret: "s"}}, []string{"secret"}, nil)
		assert.Contains(t, rec.Body.String(), `"secret":"s"`)
	})
}

func TestPermissionNotificationProvidersListRedactsForNonAdmin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	withURL := models.NotificationProvider{
		ID:            uuid.NewString(),
		Name:          "with-url",
		Type:          "discord",
		URL:           "https://discord.example.test/api/webhooks/secret-part",
		Config:        `{"template":"x"}`,
		ServiceConfig: `{"chat":"123"}`,
		LegacyURL:     "https://legacy.example.test/hook",
		Token:         "token-value",
		Enabled:       true,
	}
	withoutURL := models.NotificationProvider{ID: uuid.NewString(), Name: "no-url", Type: "gotify", Enabled: true}
	require.NoError(t, db.Create(&withURL).Error)
	require.NoError(t, db.Create(&withoutURL).Error)

	h := NewNotificationProviderHandler(services.NewNotificationService(db, nil))

	t.Run("admin keeps every field", func(t *testing.T) {
		r := roleStampRouter("admin")
		r.GET("/providers", h.List)
		rec := getJSON(t, r, "/providers")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), withURL.URL)
		assert.Contains(t, rec.Body.String(), withURL.LegacyURL)
		assert.Contains(t, rec.Body.String(), `"service_config":"{\"chat\":\"123\"}"`)
		assert.NotContains(t, rec.Body.String(), "token-value")
	})

	t.Run("non-admin loses secret-bearing fields", func(t *testing.T) {
		r := roleStampRouter("user")
		r.GET("/providers", h.List)
		rec := getJSON(t, r, "/providers")
		require.Equal(t, http.StatusOK, rec.Code)

		var rows []map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
		require.Len(t, rows, 2)

		byName := map[string]map[string]any{}
		for _, row := range rows {
			byName[row["name"].(string)] = row
		}

		withURLRow := byName["with-url"]
		for _, key := range []string{"url", "config", "service_config", "legacy_url"} {
			assert.NotContains(t, withURLRow, key, key)
		}
		assert.Equal(t, true, withURLRow["has_url"])
		assert.Equal(t, true, withURLRow["has_token"])
		assert.Equal(t, "discord", withURLRow["type"])
		assert.NotContains(t, rec.Body.String(), withURL.URL)
		assert.NotContains(t, rec.Body.String(), "token-value")

		assert.Equal(t, false, byName["no-url"]["has_url"])
	})
}

// hostPermsRoleRouter serves the proxy host routes against the seeded database of p,
// with the caller's role stamped on every request.
func hostPermsRoleRouter(p *hostPermsRouter, role string) *gin.Engine {
	r := roleStampRouter(role)
	NewProxyHostHandler(p.db, nil, services.NewNotificationService(p.db, nil), nil).RegisterRoutes(r.Group("/api/v1"))
	return r
}

func TestPermissionProxyHostReadsRedactAdvancedConfig(t *testing.T) {
	p := setupHostPermsRouter(t)
	stored := p.seedProxyHost(t, "read")
	require.NoError(t, p.db.Model(&models.ProxyHost{}).Where("uuid = ?", stored.UUID).
		Update("advanced_config_backup", `{"handle":[{"handler":"old"}]}`).Error)

	t.Run("admin sees advanced config and backup", func(t *testing.T) {
		r := hostPermsRoleRouter(p, "admin")

		rec := getJSON(t, r, "/api/v1/proxy-hosts/"+stored.UUID)
		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, hostPermsTestAdvancedConfig, body["advanced_config"])
		assert.Contains(t, body, "advanced_config_backup")
		assert.NotContains(t, body, "has_advanced_config")
	})

	t.Run("non-admin get is redacted", func(t *testing.T) {
		r := hostPermsRoleRouter(p, "user")
		rec := getJSON(t, r, "/api/v1/proxy-hosts/"+stored.UUID)
		require.Equal(t, http.StatusOK, rec.Code)
		assertProxyHostRedacted(t, rec, true)
	})

	t.Run("non-admin list is redacted", func(t *testing.T) {
		r := hostPermsRoleRouter(p, "user")
		rec := getJSON(t, r, "/api/v1/proxy-hosts")
		require.Equal(t, http.StatusOK, rec.Code)

		var rows []map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
		require.Len(t, rows, 1)
		assertProxyHostRowRedacted(t, rows[0], true)
		assert.NotContains(t, rec.Body.String(), "handler")
	})

	t.Run("admin list keeps advanced config", func(t *testing.T) {
		r := hostPermsRoleRouter(p, "admin")
		rec := getJSON(t, r, "/api/v1/proxy-hosts")
		require.Equal(t, http.StatusOK, rec.Code)

		var rows []map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
		require.Len(t, rows, 1)
		assert.Equal(t, hostPermsTestAdvancedConfig, rows[0]["advanced_config"])
		assert.NotContains(t, rows[0], "has_advanced_config")
	})
}

// assertProxyHostRedacted checks a single-host JSON body for a non-admin caller.
func assertProxyHostRedacted(t *testing.T, rec *httptest.ResponseRecorder, hasAdvanced bool) {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assertProxyHostRowRedacted(t, body, hasAdvanced)
}

func assertProxyHostRowRedacted(t *testing.T, row map[string]any, hasAdvanced bool) {
	t.Helper()
	assert.NotContains(t, row, "advanced_config")
	assert.NotContains(t, row, "advanced_config_backup")
	assert.Equal(t, hasAdvanced, row["has_advanced_config"])
	assert.Contains(t, row, "dns_provider_id", "non-secret DNS binding stays visible")
}

func TestPermissionProxyHostWriteResponsesRedactAdvancedConfig(t *testing.T) {
	p := setupHostPermsRouter(t)
	r := hostPermsRoleRouter(p, "user")

	rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts", "user", map[string]any{
		"name":         "plain-redact",
		"domain_names": "plain-redact.example.com",
		"forward_host": "10.0.0.9",
		"forward_port": 8080,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.NotContains(t, created, "advanced_config")
	assert.Equal(t, false, created["has_advanced_config"])

	stored := p.seedProxyHost(t, "write-redact")
	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+stored.UUID,
		bytes.NewReader(mustMarshalJSON(t, map[string]any{"name": "renamed-redact"})))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assertProxyHostRedacted(t, rec, true)
}

func setupCredentialRedactionRouter(t *testing.T, role string) (*gin.Engine, *gorm.DB, models.DNSProvider) {
	t.Helper()
	t.Setenv("CHARON_ENCRYPTION_KEY", redactionTestEncryptionKey)

	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t, "_journal_mode=WAL")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{}))

	encryptor, err := crypto.NewEncryptionService(redactionTestEncryptionKey)
	require.NoError(t, err)
	credService := services.NewCredentialService(db, encryptor)
	dnsService := services.NewDNSProviderService(db, encryptor)

	provider := models.DNSProvider{
		UUID:                 uuid.NewString(),
		Name:                 "cred-provider",
		ProviderType:         "cloudflare",
		Enabled:              true,
		UseMultiCredentials:  true,
		CredentialsEncrypted: "",
		KeyVersion:           1,
		PropagationTimeout:   120,
		PollingInterval:      5,
	}
	require.NoError(t, db.Create(&provider).Error)

	created, err := credService.Create(context.Background(), provider.ID, services.CreateCredentialRequest{
		Label:       "cred-one",
		ZoneFilter:  "example.test",
		Credentials: map[string]string{"api_token": "t"},
		Enabled:     true,
	})
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.DNSProviderCredential{}).Where("uuid = ?", created.UUID).
		Update("last_error", "upstream rejected the token").Error)
	time.Sleep(10 * time.Millisecond)

	credHandler := NewCredentialHandler(credService, dnsService)
	r := roleStampRouter(role)
	r.GET("/api/v1/dns-providers/:id/credentials", credHandler.List)
	r.GET("/api/v1/dns-providers/:id/credentials/:cred_id", credHandler.Get)
	return r, db, provider
}

func TestPermissionCredentialReadsRedactForNonAdmin(t *testing.T) {
	for _, tc := range []struct {
		role       string
		redacted   bool
		descriptor string
	}{
		{role: "admin", redacted: false, descriptor: "admin"},
		{role: "user", redacted: true, descriptor: "non-admin"},
	} {
		t.Run(tc.descriptor, func(t *testing.T) {
			r, db, provider := setupCredentialRedactionRouter(t, tc.role)

			listRec := getJSON(t, r, fmt.Sprintf("/api/v1/dns-providers/%d/credentials", provider.ID))
			require.Equal(t, http.StatusOK, listRec.Code, listRec.Body.String())
			var rows []map[string]any
			require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &rows))
			require.Len(t, rows, 1)
			assertCredentialRedaction(t, rows[0], tc.redacted)

			var stored models.DNSProviderCredential
			require.NoError(t, db.Where("dns_provider_id = ?", provider.ID).First(&stored).Error)
			getRec := getJSON(t, r, fmt.Sprintf("/api/v1/dns-providers/%d/credentials/%d", provider.ID, stored.ID))
			require.Equal(t, http.StatusOK, getRec.Code, getRec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &body))
			assertCredentialRedaction(t, body, tc.redacted)
		})
	}
}

func assertCredentialRedaction(t *testing.T, row map[string]any, redacted bool) {
	t.Helper()
	if redacted {
		assert.NotContains(t, row, "last_error")
		assert.NotContains(t, row, "zone_filter")
		assert.NotContains(t, row, "upstream rejected the token")
		assert.NotContains(t, row, "example.test")
	} else {
		assert.Equal(t, "upstream rejected the token", row["last_error"])
		assert.Equal(t, "example.test", row["zone_filter"])
	}
	assert.Equal(t, "cred-one", row["label"], "label stays visible to every role")
}
