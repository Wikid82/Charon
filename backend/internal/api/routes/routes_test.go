package routes

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Use in-memory DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret: "test-secret",
	}

	err = Register(router, db, cfg)
	assert.NoError(t, err)

	// Verify some routes are registered
	routes := router.Routes()
	assert.NotEmpty(t, routes)

	foundHealth := false
	for _, r := range routes {
		if r.Path == "/api/v1/health" {
			foundHealth = true
			break
		}
	}
	assert.True(t, foundHealth, "Health route should be registered")
}

func TestRegister_WithDevelopmentEnvironment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_dev_env"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret:   "test-secret",
		Environment: "development",
	}

	err = Register(router, db, cfg)
	assert.NoError(t, err)
}

func TestRegister_WithProductionEnvironment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_prod_env"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret:   "test-secret",
		Environment: "production",
	}

	err = Register(router, db, cfg)
	assert.NoError(t, err)
}

func TestRegister_AutoMigrateFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Open a valid connection then close it to simulate migration failure
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_migrate_fail"), &gorm.Config{})
	require.NoError(t, err)

	// Close underlying SQL connection to force migration failure
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.Close()

	cfg := config.Config{
		JWTSecret: "test-secret",
	}

	err = Register(router, db, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auto migrate")
}

func TestRegisterImportHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_import"), &gorm.Config{})
	require.NoError(t, err)

	// RegisterImportHandler should not panic
	RegisterImportHandler(router, db, "/usr/bin/caddy", "/tmp/imports", "/tmp/mount")

	// Verify import routes exist
	routes := router.Routes()
	hasImportRoute := false
	for _, r := range routes {
		// Import routes are: /api/v1/import/status, /api/v1/import/preview, etc.
		if r.Path == "/api/v1/import/status" || r.Path == "/api/v1/import/preview" || r.Path == "/api/v1/import/upload" {
			hasImportRoute = true
			break
		}
	}
	assert.True(t, hasImportRoute, "Import routes should be registered")
}

func TestRegister_RoutesRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret: "test-secret",
	}

	err = Register(router, db, cfg)
	require.NoError(t, err)

	routes := router.Routes()

	// Verify key routes are registered
	expectedRoutes := []string{
		"/api/v1/health",
		"/metrics",
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/setup",
	}

	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	for _, expected := range expectedRoutes {
		assert.True(t, routeMap[expected], "Route %s should be registered", expected)
	}
}
