package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupTestImportDB(t *testing.T) *gorm.DB {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	_ = db.AutoMigrate(&models.ImportSession{}, &models.ProxyHost{})
	return db
}

func TestRegisterImportHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestImportDB(t)

	router := gin.New()
	routes.RegisterImportHandler(router, db, config.Config{JWTSecret: "test-secret"}, "echo", "/tmp", "/import/Caddyfile")

	// Verify routes are registered by checking the routes list
	routeInfo := router.Routes()

	expectedRoutes := map[string]bool{
		"GET /api/v1/import/status":          false,
		"GET /api/v1/import/preview":         false,
		"POST /api/v1/import/upload":         false,
		"POST /api/v1/import/upload-multi":   false,
		"POST /api/v1/import/detect-imports": false,
		"POST /api/v1/import/commit":         false,
		"DELETE /api/v1/import/cancel":       false,
	}

	for _, route := range routeInfo {
		key := route.Method + " " + route.Path
		if _, exists := expectedRoutes[key]; exists {
			expectedRoutes[key] = true
		}
	}

	for route, found := range expectedRoutes {
		assert.True(t, found, "route %s should be registered", route)
	}
}

func TestRegisterImportHandler_AuthzGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestImportDB(t)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	cfg := config.Config{JWTSecret: "test-secret"}
	router := gin.New()
	routes.RegisterImportHandler(router, db, cfg, "echo", "/tmp", "/import/Caddyfile")

	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/import/status", http.NoBody)
	unauthW := httptest.NewRecorder()
	router.ServeHTTP(unauthW, unauthReq)
	assert.Equal(t, http.StatusUnauthorized, unauthW.Code)

	nonAdmin := &models.User{Email: "user@example.com", Role: models.RoleUser, Enabled: true}
	require.NoError(t, db.Create(nonAdmin).Error)
	authSvc := services.NewAuthService(db, cfg)
	token, err := authSvc.GenerateToken(nonAdmin)
	require.NoError(t, err)

	nonAdminReq := httptest.NewRequest(http.MethodGet, "/api/v1/import/preview", http.NoBody)
	nonAdminReq.Header.Set("Authorization", "Bearer "+token)
	nonAdminW := httptest.NewRecorder()
	router.ServeHTTP(nonAdminW, nonAdminReq)
	assert.Equal(t, http.StatusForbidden, nonAdminW.Code)
}
