package routes_test

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/models"
)

func setupTestImportDB(t *testing.T) *gorm.DB {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	db.AutoMigrate(&models.ImportSession{}, &models.ProxyHost{})
	return db
}

func TestRegisterImportHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestImportDB(t)

	router := gin.New()
	routes.RegisterImportHandler(router, db, "echo", "/tmp", "/import/Caddyfile")

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
