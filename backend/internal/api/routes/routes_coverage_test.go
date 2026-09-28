package routes

import (
	"context"
	"errors"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRegister_NotifyOnlyProviderMigrationErrorReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{
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

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = Register(ctx, router, db, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "notify-only provider migration")
}

func TestRegister_LegacyMigrationErrorIsNonFatal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{
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

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = Register(ctx, router, db, cfg)
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

func TestRegister_UptimeFeatureFlagDefaultErrorIsNonFatal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	const cbName = "routes:test_force_settings_query_error"
	err = db.Callback().Query().Before("gorm:query").Register(cbName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "settings" {
			_ = tx.AddError(errors.New("forced settings query failure"))
		}
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(cbName)
	})

	cfg := config.Config{JWTSecret: "test-secret"}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = Register(ctx, router, db, cfg)
	require.NoError(t, err)
}

// TestCleanInvalidLetsEncryptCertAssignments_QueryErrorReturnsEarly confirms
// the generic startup sweep (called for both ProxyHost and RedirectionHost
// at Register-time, routes.go:201-202) treats a failed lookup query as a
// no-op rather than panicking or propagating the error — it is a
// best-effort cleanup, not something that should ever block startup.
func TestCleanInvalidLetsEncryptCertAssignments_QueryErrorReturnsEarly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.SSLCertificate{}))

	require.NoError(t, db.Create(&models.ProxyHost{
		UUID:        "clean-letsencrypt-query-error",
		DomainNames: "clean-letsencrypt-query-error.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}).Error)

	// Drop ssl_certificates so the LEFT JOIN in cleanInvalidLetsEncryptCertAssignments
	// fails outright (no such table), forcing its early-return error branch
	// instead of the normal zero-rows-matched path.
	require.NoError(t, db.Migrator().DropTable(&models.SSLCertificate{}))

	require.NotPanics(t, func() {
		cleanInvalidLetsEncryptCertAssignments(db, "proxy_hosts", func(h models.ProxyHost) string { return h.DomainNames })
	})

	// The ProxyHost row must be untouched — confirms the function returned
	// before reaching its update loop.
	var host models.ProxyHost
	require.NoError(t, db.Where("uuid = ?", "clean-letsencrypt-query-error").First(&host).Error)
	assert.Nil(t, host.CertificateID)
}

func TestRegister_SecurityHeaderPresetInitErrorIsNonFatal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	const cbName = "routes:test_force_security_header_profile_query_error"
	err = db.Callback().Query().Before("gorm:query").Register(cbName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "security_header_profiles" {
			_ = tx.AddError(errors.New("forced security_header_profiles query failure"))
		}
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(cbName)
	})

	cfg := config.Config{JWTSecret: "test-secret"}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = Register(ctx, router, db, cfg)
	require.NoError(t, err)
}
