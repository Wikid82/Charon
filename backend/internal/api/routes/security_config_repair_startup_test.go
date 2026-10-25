package routes

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/dbmaint"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/server"
	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

// TestRegisterWithDeps_RepairFailureDoesNotStopStartup pins the repair error path before the seed: a
// failed repair is logged at Warn, the default row is still seeded, and the server still registers.
func TestRegisterWithDeps_RepairFailureDoesNotStopStartup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logs := &bytes.Buffer{}
	logger.Init(false, logs)
	t.Cleanup(func() { logger.Init(false, io.Discard) })

	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	const legacyName = "Default Security Config"
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: legacyName}).Error)
	gormfault.Inject(t, db, gormfault.Query, "security_configs", gormfault.VarIs(legacyName))

	frontend := t.TempDir()
	cfg := config.Config{JWTSecret: "test-secret", FrontendDir: frontend}
	router := server.NewRouter(frontend, "", nil)
	caddyManager := caddy.NewManager(caddy.NewClient(cfg.CaddyAdminAPI), db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)
	cerb := cerberus.New(cfg.Security, db)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, err = RegisterWithDeps(ctx, router, db, cfg, caddyManager, cerb, dbmaint.NewGate())

	require.NoError(t, err, "a repair failure must not stop registration")
	assert.Contains(t, logs.String(), "Failed to repair default SecurityConfig name")

	var defaults int64
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Count(&defaults).Error)
	assert.Equal(t, int64(1), defaults, "the seed still creates the default row when the repair fails")
}
