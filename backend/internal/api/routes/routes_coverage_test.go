package routes

import (
	"errors"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRegister_NotifyOnlyProviderMigrationErrorReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_migration_errors"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	const cbName = "routes:test_force_notify_only_migration_query_error"
	err = db.Callback().Query().Before("gorm:query").Register(cbName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "notification_providers" {
			_ = tx.AddError(errors.New("forced notification_providers query failure"))
		}
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(cbName)
	})

	cfg := config.Config{JWTSecret: "test-secret"}

	err = Register(router, db, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "notify-only provider migration")
}

func TestRegister_LegacyMigrationErrorIsNonFatal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_legacy_migration_warn"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	const cbName = "routes:test_force_legacy_migration_query_error"
	err = db.Callback().Query().Before("gorm:query").Register(cbName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "notification_configs" {
			_ = tx.AddError(errors.New("forced notification_configs query failure"))
		}
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(cbName)
	})

	cfg := config.Config{JWTSecret: "test-secret"}

	err = Register(router, db, cfg)
	require.NoError(t, err)

	hasHealth := false
	for _, r := range router.Routes() {
		if r.Path == "/api/v1/health" {
			hasHealth = true
			break
		}
	}
	require.True(t, hasHealth)
}
