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

// TestRegisterWithDeps_RetireFailureDoesNotStopStartup pins the second carry-over pass in registration: a failed
// pass is logged at Warn and the mode row stays for the next start, so registration still succeeds. The boot-time
// pass in main.go is the fatal one.
func TestRegisterWithDeps_RetireFailureDoesNotStopStartup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logs := &bytes.Buffer{}
	logger.Init(false, logs)
	t.Cleanup(func() { logger.Init(false, io.Discard) })

	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecRetiredModeSettingKey, Value: "local", Category: "security", Type: "string"}).Error)
	gormfault.Inject(t, db, gormfault.Delete, "settings", gormfault.VarIs(models.CrowdSecRetiredModeSettingKey))

	frontend := t.TempDir()
	cfg := config.Config{JWTSecret: "test-secret", FrontendDir: frontend}
	router := server.NewRouter(frontend, "", nil)
	caddyManager := caddy.NewManager(caddy.NewClient(cfg.CaddyAdminAPI), db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)
	cerb := cerberus.New(cfg.Security, db)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, err = RegisterWithDeps(ctx, router, db, cfg, caddyManager, cerb, dbmaint.NewGate())

	require.NoError(t, err, "a failed second carry-over pass must not stop registration")
	assert.Contains(t, logs.String(), "Failed to retire CrowdSec mode setting")
	assert.True(t, db.Migrator().HasTable(&models.Setting{}))
	var modeRows int64
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", models.CrowdSecRetiredModeSettingKey).Count(&modeRows).Error)
	assert.Equal(t, int64(1), modeRows, "the mode row is kept so the next start retries the carry-over")
}
