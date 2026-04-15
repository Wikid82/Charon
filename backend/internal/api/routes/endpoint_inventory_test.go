package routes_test

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/config"
)

func TestEndpointInventory_FrontendCanonicalSaveImportContractsExistInBackend(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_endpoint_inventory"), &gorm.Config{})
	require.NoError(t, err)

	router := gin.New()
	require.NoError(t, routes.Register(context.Background(), router, db, config.Config{JWTSecret: "test-secret"}))
	routes.RegisterImportHandler(router, db, config.Config{JWTSecret: "test-secret"}, "echo", "/tmp", "/import/Caddyfile")

	assertStrictMethodPathMatrix(t, router.Routes(), backendImportSaveInventoryCanonical(), "backend canonical save/import inventory")
}

func TestEndpointInventory_FrontendParityMatchesCurrentContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_endpoint_inventory_frontend_parity"), &gorm.Config{})
	require.NoError(t, err)

	router := gin.New()
	require.NoError(t, routes.Register(context.Background(), router, db, config.Config{JWTSecret: "test-secret"}))
	routes.RegisterImportHandler(router, db, config.Config{JWTSecret: "test-secret"}, "echo", "/tmp", "/import/Caddyfile")

	assertStrictMethodPathMatrix(t, router.Routes(), frontendObservedImportSaveInventory(), "frontend observed save/import inventory")
}

func TestEndpointInventory_FrontendParityDetectsActualMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_endpoint_inventory_frontend_parity_mismatch"), &gorm.Config{})
	require.NoError(t, err)

	router := gin.New()
	require.NoError(t, routes.Register(context.Background(), router, db, config.Config{JWTSecret: "test-secret"}))
	routes.RegisterImportHandler(router, db, config.Config{JWTSecret: "test-secret"}, "echo", "/tmp", "/import/Caddyfile")

	contractWithMismatch := append([]endpointInventoryEntry{}, frontendObservedImportSaveInventory()...)
	for i := range contractWithMismatch {
		if contractWithMismatch[i].Path == "/api/v1/import/cancel" {
			contractWithMismatch[i].Method = "POST"
			break
		}
	}

	drift := collectRouteMatrixDrift(router.Routes(), contractWithMismatch)

	assert.Contains(
		t,
		drift,
		"method drift for Import cancel (/api/v1/import/cancel): expected POST, registered methods=[DELETE]",
	)
}
