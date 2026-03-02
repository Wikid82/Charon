package routes_test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/config"
)

func TestRegisterImportHandler_StrictRouteMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestImportDB(t)

	router := gin.New()
	routes.RegisterImportHandler(router, db, config.Config{JWTSecret: "test-secret"}, "echo", "/tmp", "/import/Caddyfile")

	assertStrictMethodPathMatrix(t, router.Routes(), backendImportRouteMatrix(), "import")
}
