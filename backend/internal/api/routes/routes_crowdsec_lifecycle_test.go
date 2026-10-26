package routes

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/dbmaint"
	"github.com/Wikid82/charon/backend/internal/server"
)

// TestRegisterWithDeps_ReturnsWiredCrowdsecLifecycle pins that the registration hands back the CrowdSec lifecycle
// that the API and the Tier 2 server share. A nil or typed-nil lifecycle would mean the switches are not wired.
func TestRegisterWithDeps_ReturnsWiredCrowdsecLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)

	frontend := t.TempDir()
	cfg := config.Config{JWTSecret: "test-secret", FrontendDir: frontend}
	router := server.NewRouter(frontend, "", nil)
	caddyManager := caddy.NewManager(caddy.NewClient(cfg.CaddyAdminAPI), db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)
	cerb := cerberus.New(cfg.Security, db)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	registered, err := RegisterWithDeps(ctx, router, db, cfg, caddyManager, cerb, dbmaint.NewGate())

	require.NoError(t, err)
	require.NotNil(t, registered.UptimeShutdown)
	require.NotNil(t, registered.CrowdSec, "production registration must wire the CrowdSec lifecycle")
	require.IsType(t, &handlers.CrowdsecHandler{}, registered.CrowdSec)
	require.Equal(t, handlers.LifecycleOrNil(registered.CrowdSec), registered.CrowdSec)
}
