package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

const hostPermsTestAdvancedConfig = `{"handle":[{"handler":"headers"}]}`

// hostPermsRouter serves the proxy and redirection host routes. The caller's
// role comes from the X-Test-Role header, standing in for the auth middleware.
type hostPermsRouter struct {
	engine *gin.Engine
	db     *gorm.DB
	dns    models.DNSProvider
}

func setupHostPermsRouter(t *testing.T) *hostPermsRouter {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.SecurityHeaderProfile{},
		&models.Notification{},
		&models.NotificationProvider{},
		&models.DNSProvider{},
		&models.RedirectionHost{},
	))

	dns := models.DNSProvider{UUID: uuid.NewString(), Name: "dns", ProviderType: "cloudflare", Enabled: true}
	require.NoError(t, db.Create(&dns).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			c.Set(middleware.RoleKey, role)
		}
		c.Next()
	})
	api := r.Group("/api/v1")
	ns := services.NewNotificationService(db, nil)
	NewProxyHostHandler(db, nil, ns, nil).RegisterRoutes(api)
	NewRedirectionHostHandler(db, nil).RegisterRoutes(api)

	return &hostPermsRouter{engine: r, db: db, dns: dns}
}

func (p *hostPermsRouter) do(t *testing.T, method, path, role string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if role != "" {
		req.Header.Set("X-Test-Role", role)
	}
	rec := httptest.NewRecorder()
	p.engine.ServeHTTP(rec, req)
	return rec
}

// seedProxyHost stores a host with an admin-bound DNS provider, DNS challenge and
// advanced config, bypassing the write checks.
func (p *hostPermsRouter) seedProxyHost(t *testing.T, name string) models.ProxyHost {
	t.Helper()
	dnsID := p.dns.ID
	host := models.ProxyHost{
		UUID:            uuid.NewString(),
		Name:            name,
		DomainNames:     name + ".example.com",
		ForwardScheme:   "http",
		ForwardHost:     "10.0.0.5",
		ForwardPort:     8080,
		Enabled:         true,
		DNSProviderID:   &dnsID,
		UseDNSChallenge: true,
		AdvancedConfig:  hostPermsTestAdvancedConfig,
	}
	require.NoError(t, p.db.Create(&host).Error)
	return host
}

func (p *hostPermsRouter) seedRedirectionHost(t *testing.T, name string) models.RedirectionHost {
	t.Helper()
	dnsID := p.dns.ID
	host := models.RedirectionHost{
		UUID:            uuid.NewString(),
		DomainNames:     name + ".example.com",
		TargetURL:       "https://target.example.net",
		StatusCode:      301,
		Enabled:         true,
		DNSProviderID:   &dnsID,
		UseDNSChallenge: true,
	}
	require.NoError(t, p.db.Create(&host).Error)
	return host
}

func (p *hostPermsRouter) countProxyHosts(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, p.db.Model(&models.ProxyHost{}).Count(&count).Error)
	return count
}

func assertAdminOnlyResponse(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "permissions_admin_only", body["error_code"])
}

func TestPermissionProxyHostCreateAdminOnlyFields(t *testing.T) {
	p := setupHostPermsRouter(t)
	dnsRef := p.dns.UUID

	tests := []struct {
		name    string
		payload map[string]any
	}{
		{
			name:    "dns provider binding",
			payload: map[string]any{"dns_provider_id": dnsRef},
		},
		{
			name:    "dns challenge toggle",
			payload: map[string]any{"dns_provider_id": dnsRef, "use_dns_challenge": true},
		},
		{
			name:    "advanced config",
			payload: map[string]any{"advanced_config": hostPermsTestAdvancedConfig},
		},
	}

	// Each subtest gets its own domain so a refused or accepted create never
	// collides with another subtest's host.
	build := func(index int, extra map[string]any) map[string]any {
		payload := map[string]any{
			"name":         fmt.Sprintf("create-%d", index),
			"domain_names": fmt.Sprintf("create-%d.example.com", index),
			"forward_host": "10.0.0.5",
			"forward_port": 8080,
		}
		for k, v := range extra {
			payload[k] = v
		}
		return payload
	}

	index := 0
	for _, tt := range tests {
		for _, role := range []string{"user", ""} {
			index++
			payload := build(index, tt.payload)
			t.Run(fmt.Sprintf("%s/role=%q", tt.name, role), func(t *testing.T) {
				before := p.countProxyHosts(t)
				rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts", role, payload)
				assertAdminOnlyResponse(t, rec)
				assert.Equal(t, before, p.countProxyHosts(t))
			})
		}

		index++
		adminPayload := build(index, tt.payload)
		t.Run(tt.name+"/admin", func(t *testing.T) {
			rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts", "admin", adminPayload)
			assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		})
	}
}

func TestPermissionProxyHostCreatePlainHostAllowedForUser(t *testing.T) {
	p := setupHostPermsRouter(t)
	rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts", "user", map[string]any{
		"name":         "plain",
		"domain_names": "plain-user.example.com",
		"forward_host": "10.0.0.5",
		"forward_port": 8080,
	})
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestPermissionProxyHostUpdateAdminOnlyFields(t *testing.T) {
	p := setupHostPermsRouter(t)
	stored := p.seedProxyHost(t, "upd")
	path := "/api/v1/proxy-hosts/" + stored.UUID

	t.Run("user edit leaving admin fields unchanged", func(t *testing.T) {
		rec := p.do(t, http.MethodPut, path, "user", map[string]any{
			"name":              "renamed",
			"dns_provider_id":   p.dns.UUID,
			"use_dns_challenge": true,
			"advanced_config":   hostPermsTestAdvancedConfig,
			"forward_port":      9090,
		})
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("user changing dns provider", func(t *testing.T) {
		rec := p.do(t, http.MethodPut, path, "user", map[string]any{"dns_provider_id": nil, "use_dns_challenge": false})
		assertAdminOnlyResponse(t, rec)
	})

	t.Run("user disabling dns challenge", func(t *testing.T) {
		rec := p.do(t, http.MethodPut, path, "user", map[string]any{"use_dns_challenge": false})
		assertAdminOnlyResponse(t, rec)
	})

	t.Run("user clearing advanced config", func(t *testing.T) {
		rec := p.do(t, http.MethodPut, path, "user", map[string]any{"advanced_config": ""})
		assertAdminOnlyResponse(t, rec)
	})

	t.Run("user setting new advanced config", func(t *testing.T) {
		rec := p.do(t, http.MethodPut, path, "user", map[string]any{
			"advanced_config": `{"handle":[{"handler":"static_response"}]}`,
		})
		assertAdminOnlyResponse(t, rec)
	})

	var unchanged models.ProxyHost
	require.NoError(t, p.db.Where("uuid = ?", stored.UUID).First(&unchanged).Error)
	assert.Equal(t, p.dns.ID, *unchanged.DNSProviderID)
	assert.True(t, unchanged.UseDNSChallenge)
	assert.Equal(t, hostPermsTestAdvancedConfig, unchanged.AdvancedConfig)

	t.Run("admin changing admin fields", func(t *testing.T) {
		rec := p.do(t, http.MethodPut, path, "admin", map[string]any{
			"dns_provider_id":   nil,
			"use_dns_challenge": false,
			"advanced_config":   "",
		})
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
}

func TestPermissionProxyHostBulkUpdatesKeepAdminFields(t *testing.T) {
	p := setupHostPermsRouter(t)
	stored := p.seedProxyHost(t, "bulk")

	rec := p.do(t, http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", "user", map[string]any{
		"host_uuids": []string{stored.UUID},
	})
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = p.do(t, http.MethodPut, "/api/v1/proxy-hosts/bulk-update-group", "user", map[string]any{
		"host_uuids": []string{stored.UUID},
	})
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var after models.ProxyHost
	require.NoError(t, p.db.Where("uuid = ?", stored.UUID).First(&after).Error)
	require.NotNil(t, after.DNSProviderID)
	assert.Equal(t, p.dns.ID, *after.DNSProviderID)
	assert.Equal(t, hostPermsTestAdvancedConfig, after.AdvancedConfig)
}

func TestPermissionProxyHostRefusesAdminListenerTargetForEveryRole(t *testing.T) {
	p := setupHostPermsRouter(t)

	for _, role := range []string{"admin", "user"} {
		t.Run(role, func(t *testing.T) {
			rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts", role, map[string]any{
				"name":         "loop-" + role,
				"domain_names": "loop-" + role + ".example.com",
				"forward_host": "localhost",
				"forward_port": 2019,
			})
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), services.ErrAdminListenerTarget.Error())
		})
	}
}

func TestPermissionRedirectionHostCreateAdminOnlyFields(t *testing.T) {
	p := setupHostPermsRouter(t)

	rec := p.do(t, http.MethodPost, "/api/v1/redirection-hosts", "user", map[string]any{
		"domain_names":      "redir-user.example.com",
		"target_url":        "https://target.example.net",
		"status_code":       301,
		"dns_provider_id":   p.dns.UUID,
		"use_dns_challenge": true,
	})
	assertAdminOnlyResponse(t, rec)

	rec = p.do(t, http.MethodPost, "/api/v1/redirection-hosts", "user", map[string]any{
		"domain_names": "redir-plain-user.example.com",
		"target_url":   "https://target.example.net",
		"status_code":  301,
	})
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = p.do(t, http.MethodPost, "/api/v1/redirection-hosts", "admin", map[string]any{
		"domain_names":      "redir-admin.example.com",
		"target_url":        "https://target.example.net",
		"status_code":       301,
		"dns_provider_id":   p.dns.UUID,
		"use_dns_challenge": true,
	})
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestPermissionRedirectionHostUpdateAdminOnlyFields(t *testing.T) {
	p := setupHostPermsRouter(t)
	stored := p.seedRedirectionHost(t, "redir-upd")
	path := "/api/v1/redirection-hosts/" + stored.UUID

	rec := p.do(t, http.MethodPut, path, "user", map[string]any{
		"name":              "renamed",
		"dns_provider_id":   p.dns.UUID,
		"use_dns_challenge": true,
	})
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = p.do(t, http.MethodPut, path, "user", map[string]any{"dns_provider_id": nil, "use_dns_challenge": false})
	assertAdminOnlyResponse(t, rec)

	rec = p.do(t, http.MethodPut, path, "user", map[string]any{"use_dns_challenge": false})
	assertAdminOnlyResponse(t, rec)

	rec = p.do(t, http.MethodPut, path, "admin", map[string]any{"dns_provider_id": nil, "use_dns_challenge": false})
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestPermissionRespondHostWriteErrorMapsValidationTo400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Set(middleware.RoleKey, "user")

	respondHostWriteError(ctx, fmt.Errorf("domain names is required"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "domain names is required")
}

// attachTestAdminRole stamps the administrator role on every request. The
// routers used by the host handler suites model an authenticated administrator.
func attachTestAdminRole(r *gin.Engine) {
	r.Use(func(c *gin.Context) {
		c.Set(middleware.RoleKey, string(models.RoleAdmin))
		c.Next()
	})
}
