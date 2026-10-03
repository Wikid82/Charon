package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func forwardAuthRequest(t *testing.T, handler *AuthHandler, token string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.GET("/verify", handler.Verify)
	req := httptest.NewRequest(http.MethodGet, "/verify", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func seedForwardAuthUser(t *testing.T, handler *AuthHandler, db *gorm.DB, mode models.PermissionMode, permitted ...*models.ProxyHost) string {
	t.Helper()
	user := &models.User{
		UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: uuid.NewString() + "@example.com", Name: "U",
		Role: models.RoleUser, Enabled: true, PermissionMode: mode,
	}
	require.NoError(t, user.SetPassword("password123"))
	for _, h := range permitted {
		user.PermittedHosts = append(user.PermittedHosts, *h)
	}
	require.NoError(t, db.Create(user).Error)
	token, err := handler.authService.GenerateToken(user)
	require.NoError(t, err)
	return token
}

func seedForwardAuthHost(t *testing.T, db *gorm.DB, domains string, forwardAuth bool) *models.ProxyHost {
	t.Helper()
	h := &models.ProxyHost{UUID: uuid.NewString(), Name: domains, DomainNames: domains, ForwardAuthEnabled: forwardAuth, Enabled: true}
	require.NoError(t, db.Create(h).Error)
	return h
}

func TestForwardAuth_ExactHostMatch(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.ProxyHost{}))

	protected := seedForwardAuthHost(t, db, "example.com", true)
	seedForwardAuthHost(t, db, "open.test", false)
	seedForwardAuthHost(t, db, "*.wild.test", true)
	seedForwardAuthHost(t, db, "multi.test, second.test:8443", false)

	// A user that is denied by default and only permitted on nothing: every
	// forward-auth-enabled host must refuse them.
	denied := seedForwardAuthUser(t, handler, db, models.PermissionModeDenyAll)
	// A user permitted on the protected host.
	allowed := seedForwardAuthUser(t, handler, db, models.PermissionModeDenyAll, protected)

	tests := []struct {
		name    string
		token   string
		headers map[string]string
		want    int
	}{
		{"permitted user, exact host", allowed, map[string]string{"X-Forwarded-Host": "example.com"}, http.StatusOK},
		{"denied user, exact host", denied, map[string]string{"X-Forwarded-Host": "example.com"}, http.StatusForbidden},
		{"denied user, host with port and case", denied, map[string]string{"X-Forwarded-Host": "EXAMPLE.com:443"}, http.StatusForbidden},
		{"near miss prefix is a different host", denied, map[string]string{"X-Forwarded-Host": "notexample.com"}, http.StatusForbidden},
		{"near miss subdomain is a different host", denied, map[string]string{"X-Forwarded-Host": "a.example.com"}, http.StatusForbidden},
		{"percent is rejected", allowed, map[string]string{"X-Forwarded-Host": "%"}, http.StatusForbidden},
		{"percent pattern is rejected", allowed, map[string]string{"X-Forwarded-Host": "%example%"}, http.StatusForbidden},
		{"underscore matches nothing", allowed, map[string]string{"X-Forwarded-Host": "_"}, http.StatusForbidden},
		{"wildcard character is rejected", allowed, map[string]string{"X-Forwarded-Host": "*.wild.test"}, http.StatusForbidden},
		{"no host header", allowed, nil, http.StatusForbidden},
		{"unknown host", allowed, map[string]string{"X-Forwarded-Host": "unknown.test"}, http.StatusForbidden},
		{"host without forward auth allows any signed-in user", denied, map[string]string{"X-Forwarded-Host": "open.test"}, http.StatusOK},
		{"stored list, first entry", denied, map[string]string{"X-Forwarded-Host": "multi.test"}, http.StatusOK},
		{"stored list, entry with port", denied, map[string]string{"X-Forwarded-Host": "second.test"}, http.StatusOK},
		{"wildcard covers one label and enforces", denied, map[string]string{"X-Forwarded-Host": "a.wild.test"}, http.StatusForbidden},
		{"wildcard does not cover two labels", denied, map[string]string{"X-Forwarded-Host": "a.b.wild.test"}, http.StatusForbidden},
		{"first forwarded entry is used", denied, map[string]string{"X-Forwarded-Host": "open.test, example.com"}, http.StatusOK},
		{"original host used only when forwarded host is absent", denied, map[string]string{"X-Original-Host": "open.test"}, http.StatusOK},
		{"forwarded host wins over original host", denied, map[string]string{"X-Forwarded-Host": "example.com", "X-Original-Host": "open.test"}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := forwardAuthRequest(t, handler, tt.token, tt.headers)
			assert.Equal(t, tt.want, w.Code)
			if tt.want == http.StatusOK {
				assert.NotEmpty(t, w.Header().Get("X-Forwarded-User"))
			} else {
				assert.Empty(t, w.Header().Get("X-Forwarded-User"))
			}
		})
	}
}

func TestForwardAuth_LookupFailureDenies(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)
	seedForwardAuthHost(t, db, "open.test", false)
	token := seedForwardAuthUser(t, handler, db, models.PermissionModeAllowAll)

	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))

	w := forwardAuthRequest(t, handler, token, map[string]string{"X-Forwarded-Host": "open.test"})
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Empty(t, w.Header().Get("X-Forwarded-User"))
}

func TestForwardAuth_NoDatabaseDenies(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)
	token := seedForwardAuthUser(t, handler, db, models.PermissionModeAllowAll)
	handler.db = nil

	w := forwardAuthRequest(t, handler, token, map[string]string{"X-Forwarded-Host": "open.test"})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestForwardAuth_PermissionLoadFailureDenies(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)
	seedForwardAuthHost(t, db, "app.test", true)
	token := seedForwardAuthUser(t, handler, db, models.PermissionModeAllowAll)

	// Token validation reads the user row first; removing the join table makes
	// only the permitted-host preload fail.
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS user_permitted_hosts").Error)

	w := forwardAuthRequest(t, handler, token, map[string]string{"X-Forwarded-Host": "app.test"})
	assert.Equal(t, http.StatusForbidden, w.Code)
}
