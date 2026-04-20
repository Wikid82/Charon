package routes_test

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/config"
)

func TestRegister_StrictSaveRouteMatrixUsedByImportWorkflows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_save_contract_matrix"), &gorm.Config{})
	require.NoError(t, err)

	router := gin.New()
	require.NoError(t, routes.Register(context.Background(), router, db, config.Config{JWTSecret: "test-secret"}))

	assertStrictMethodPathMatrix(t, router.Routes(), saveRouteMatrixForImportWorkflows(), "save")
}
