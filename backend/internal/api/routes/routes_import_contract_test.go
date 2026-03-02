package routes_test

import (
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/config"
)

func TestRegisterImportHandler_StrictRouteMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestImportDB(t)
	tempDir := t.TempDir()
	importCaddyfilePath := filepath.Join(tempDir, "import", "Caddyfile")

	router := gin.New()
	routes.RegisterImportHandler(router, db, config.Config{JWTSecret: "test-secret"}, "echo", tempDir, importCaddyfilePath)

	assertStrictMethodPathMatrix(t, router.Routes(), backendImportRouteMatrix(), "import")
}
