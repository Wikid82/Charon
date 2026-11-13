package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// Uptime monitor changes probe targets from the server, so they are
// administrator-only. Reads stay available to standard users. Checked per
// route: role=user is refused on writes and reaches reads; role=admin reaches
// the handler on writes.
func TestRegister_UptimeMonitorWritesAreAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dbPath := filepath.Join(t.TempDir(), "charon.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	cfg := config.Config{JWTSecret: "test-secret", DatabasePath: dbPath, DBCompactOnStart: config.DBCompactAuto}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	authSvc := services.NewAuthService(db, cfg)
	tokenFor := func(role models.UserRole) string {
		acct := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: string(role) + "-uptime-roles@example.com", Role: role, Enabled: true}
		require.NoError(t, db.Create(acct).Error)
		token, tokenErr := authSvc.GenerateToken(acct)
		require.NoError(t, tokenErr)
		return token
	}
	userToken, adminToken := tokenFor(models.RoleUser), tokenFor(models.RoleAdmin)

	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	const monitorPath = "/api/v1/uptime/monitors/00000000-0000-4000-8000-000000000001"

	writes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/uptime/monitors", `{"name":"x","url":"http://127.0.0.1:1","type":"http"}`},
		{http.MethodPut, monitorPath, `{"name":"x"}`},
		{http.MethodDelete, monitorPath, ""},
		{http.MethodPost, monitorPath + "/check", ""},
		{http.MethodPost, "/api/v1/uptime/sync", ""},
	}
	for _, tc := range writes {
		t.Run("write "+tc.method+" "+tc.path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, do(tc.method, tc.path, "", tc.body).Code, "no token")
			assert.Equal(t, http.StatusForbidden, do(tc.method, tc.path, userToken, tc.body).Code, "role=user")
			assert.NotEqual(t, http.StatusForbidden, do(tc.method, tc.path, adminToken, tc.body).Code, "role=admin reaches handler")
		})
	}

	reads := []string{
		"/api/v1/uptime/monitors",
		"/api/v1/uptime/monitors/summary",
		monitorPath + "/history",
		"/api/v1/uptime/health",
	}
	for _, path := range reads {
		t.Run("read "+path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, do(http.MethodGet, path, "", "").Code, "no token")
			assert.NotEqual(t, http.StatusForbidden, do(http.MethodGet, path, userToken, "").Code, "role=user may read")
		})
	}
}
