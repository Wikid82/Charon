package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// serveJSON sends one JSON request to router and returns the recorded response.
func serveJSON(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestProxyHostCreate_SwitchedOffOptionsStayOff describes a host created with its switches off: the row and the answer
// keep false, and the host's locations are stored once.
func TestProxyHostCreate_SwitchedOffOptionsStayOff(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.SecurityConfig{}, &models.SecurityAudit{}, &models.SecurityHeaderProfile{}))
	router := roleStampRouter("admin")
	NewProxyHostHandler(db, nil, services.NewNotificationService(db, nil), nil).RegisterRoutes(router.Group("/api/v1"))
	body := `{"domain_names":"stored-as-sent.example.com","forward_scheme":"http","forward_host":"127.0.0.1","forward_port":8080,` +
		`"enabled":false,"security_headers_enabled":false,"http2_support":false,"block_exploits":false,` +
		`"locations":[{"path":"/api","forward_scheme":"http","forward_host":"127.0.0.1","forward_port":9000}]}`

	w := serveJSON(t, router, http.MethodPost, "/api/v1/proxy-hosts", body)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var host models.ProxyHost
	require.NoError(t, db.Where("domain_names = ?", "stored-as-sent.example.com").First(&host).Error)
	assert.False(t, host.Enabled)
	assert.False(t, host.SecurityHeadersEnabled)
	assert.False(t, host.HTTP2Support)
	assert.False(t, host.BlockExploits)
	var locations int64
	require.NoError(t, db.Model(&models.Location{}).Where("proxy_host_id = ?", host.ID).Count(&locations).Error)
	assert.Equal(t, int64(1), locations, "the location is stored once")
}

// TestProxyHostUpdate_TogglesSecurityHeadersEnabled describes the update branch for the security headers switch.
func TestProxyHostUpdate_TogglesSecurityHeadersEnabled(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.SecurityConfig{}, &models.SecurityAudit{}, &models.SecurityHeaderProfile{}))
	host := models.ProxyHost{
		UUID: uuid.NewString(), DomainNames: "toggle-headers.example.com", ForwardScheme: "http",
		ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true, SecurityHeadersEnabled: true,
	}
	require.NoError(t, db.Create(&host).Error)
	router := roleStampRouter("admin")
	NewProxyHostHandler(db, nil, services.NewNotificationService(db, nil), nil).RegisterRoutes(router.Group("/api/v1"))

	w := serveJSON(t, router, http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, `{"security_headers_enabled":false}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var stored models.ProxyHost
	require.NoError(t, db.First(&stored, host.ID).Error)
	assert.False(t, stored.SecurityHeadersEnabled)
}

// TestSecurityHeadersCreate_OptionsSwitchedOffStayOff describes a profile created with options off: the row and the
// answer keep the values sent, including an empty X-Frame-Options.
func TestSecurityHeadersCreate_OptionsSwitchedOffStayOff(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityHeaderProfile{}))
	router := roleStampRouter("admin")
	router.POST("/api/v1/security/headers/profiles", NewSecurityHeadersHandler(db, nil).CreateProfile)

	w := serveJSON(t, router, http.MethodPost, "/api/v1/security/headers/profiles",
		`{"name":"off-profile","hsts_enabled":false,"xss_protection":false,"x_frame_options":""}`)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var answer struct {
		Profile models.SecurityHeaderProfile `json:"profile"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	assert.False(t, answer.Profile.HSTSEnabled)
	assert.False(t, answer.Profile.XSSProtection)
	assert.Equal(t, "", answer.Profile.XFrameOptions)

	var stored models.SecurityHeaderProfile
	require.NoError(t, db.Where("name = ?", "off-profile").First(&stored).Error)
	assert.False(t, stored.HSTSEnabled)
	assert.False(t, stored.XSSProtection)
	assert.Equal(t, "", stored.XFrameOptions)
}

// TestSecurityHeadersCreate_OmittedOptionsGetTheModelDefaults describes a profile created without the options: each
// omitted option gets its model default.
func TestSecurityHeadersCreate_OmittedOptionsGetTheModelDefaults(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityHeaderProfile{}))
	router := roleStampRouter("admin")
	router.POST("/api/v1/security/headers/profiles", NewSecurityHeadersHandler(db, nil).CreateProfile)

	w := serveJSON(t, router, http.MethodPost, "/api/v1/security/headers/profiles", `{"name":"default-profile"}`)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var stored models.SecurityHeaderProfile
	require.NoError(t, db.Where("name = ?", "default-profile").First(&stored).Error)
	assert.True(t, stored.HSTSEnabled)
	assert.True(t, stored.XSSProtection)
	assert.Equal(t, "DENY", stored.XFrameOptions)
}

// TestUserInvite_RowStaysDisabled describes an invite: the created user is disabled in the row, although the column has
// a default of true.
func TestUserInvite_RowStaysDisabled(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	router := roleStampRouter("admin")
	router.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Next()
	})
	router.POST("/users/invite", handler.InviteUser)

	w := serveJSON(t, router, http.MethodPost, "/users/invite", `{"email":"disabled-invite@example.com","role":"user"}`)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var user models.User
	require.NoError(t, db.Where("email = ?", "disabled-invite@example.com").First(&user).Error)
	assert.False(t, user.Enabled)
}
