package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

// The database maintenance endpoints expose sizes and set a server flag, so
// they are admin-only: no token is 401, role=user is 403, role=admin reaches
// the handler (GH #1422).
func TestRegister_DatabaseMaintenanceRoutesAreAdminOnly(t *testing.T) {
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
		acct := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: string(role) + "-dbmaint@example.com", Role: role, Enabled: true}
		require.NoError(t, db.Create(acct).Error)
		token, tokenErr := authSvc.GenerateToken(acct)
		require.NoError(t, tokenErr)
		return token
	}
	userToken, adminToken := tokenFor(models.RoleUser), tokenFor(models.RoleAdmin)

	do := func(method, path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, http.NoBody)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	for _, tc := range []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/api/v1/system/database", http.StatusOK},
		// A scratch database is far below the reclaim floor: the handler answers
		// with its own 409, which proves the request reached it.
		{http.MethodPost, "/api/v1/system/database/optimize-on-restart", http.StatusConflict},
		{http.MethodDelete, "/api/v1/system/database/optimize-on-restart", http.StatusOK},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, do(tc.method, tc.path, "").Code, "no token")
			assert.Equal(t, http.StatusForbidden, do(tc.method, tc.path, userToken).Code, "role=user")
			assert.Equal(t, tc.want, do(tc.method, tc.path, adminToken).Code, "role=admin")
		})
	}
}
