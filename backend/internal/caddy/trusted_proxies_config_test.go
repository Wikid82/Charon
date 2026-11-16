package caddy

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
)

func TestTrustedProxiesFor_LoopbackWhenUnconfigured(t *testing.T) {
	got := trustedProxiesFor(nil)
	assert.Equal(t, "static", got.Source)
	assert.Equal(t, security.AlwaysTrustedProxies(), got.Ranges)
}

func TestTrustedProxiesFor_ConfiguredRangesAreUsedAsGiven(t *testing.T) {
	configured := []string{"127.0.0.1/32", "::1/128", "172.18.0.5/32"}
	got := trustedProxiesFor(configured)
	assert.Equal(t, "static", got.Source)
	assert.Equal(t, configured, got.Ranges)
}

func TestGenerateConfig_WithTrustedProxiesEmitsOperatorRanges(t *testing.T) {
	cfg, err := GenerateConfig(nil, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil, nil,
		WithTrustedProxies([]string{"127.0.0.1/32", "::1/128", "172.18.0.5/32"}))
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.NotNil(t, server.TrustedProxies)
	assert.Equal(t, []string{"127.0.0.1/32", "::1/128", "172.18.0.5/32"}, server.TrustedProxies.Ranges)
}

// TestGeneratedServerTrustsConfiguredProxiesOnly: the server reads a forwarded client address only from the configured
// peers, using the strict walk over X-Forwarded-For.
func TestGeneratedServerTrustsConfiguredProxiesOnly(t *testing.T) {
	cfg, err := GenerateConfig(nil, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil, nil,
		WithTrustedProxies([]string{"127.0.0.1/32", "::1/128", "172.18.0.5/32"}))
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	assert.Equal(t, 1, server.TrustedProxiesStrict)
	assert.Equal(t, []string{"X-Forwarded-For"}, server.ClientIPHeaders)
	assert.Equal(t, []string{"127.0.0.1/32", "::1/128", "172.18.0.5/32"}, server.TrustedProxies.Ranges)
}

func TestWithTrustedProxies_CopiesInput(t *testing.T) {
	ranges := []string{"172.18.0.5/32"}
	resolved := resolveGenerateConfigOptions([]GenerateConfigOption{WithTrustedProxies(ranges)})
	ranges[0] = "changed"
	assert.Equal(t, []string{"172.18.0.5/32"}, resolved.trustedProxies)
}

func trustedProxyManagerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/manager.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

func TestManagerTrustedProxyRanges(t *testing.T) {
	db := trustedProxyManagerDB(t)
	env := []string{"10.9.9.9", "0.0.0.0/0"}
	m := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{TrustedProxies: env})

	t.Run("environment list is clamped when nothing is saved", func(t *testing.T) {
		assert.Equal(t, []string{"127.0.0.1/32", "::1/128", "10.9.9.9/32"}, m.trustedProxyRanges())
	})

	t.Run("saved list replaces the environment list", func(t *testing.T) {
		saved := "172.18.0.5/32"
		require.NoError(t, models.WriteTrustedProxiesSetting(db, &saved))
		assert.Equal(t, []string{"127.0.0.1/32", "::1/128", "172.18.0.5/32"}, m.trustedProxyRanges())
	})

	t.Run("invalid saved list trusts loopback only", func(t *testing.T) {
		invalid := "0.0.0.0/0"
		require.NoError(t, models.WriteTrustedProxiesSetting(db, &invalid))
		assert.Equal(t, security.AlwaysTrustedProxies(), m.trustedProxyRanges())
	})

	t.Run("removing the saved list restores the environment list", func(t *testing.T) {
		require.NoError(t, models.WriteTrustedProxiesSetting(db, nil))
		assert.Equal(t, []string{"127.0.0.1/32", "::1/128", "10.9.9.9/32"}, m.trustedProxyRanges())
	})
}

func TestManagerTrustedProxyRanges_ReadErrorTrustsLoopbackOnly(t *testing.T) {
	db := trustedProxyManagerDB(t)
	m := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{TrustedProxies: []string{"10.9.9.9"}})
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	assert.Equal(t, security.AlwaysTrustedProxies(), m.trustedProxyRanges(), "a failed read must not widen trust")
}

func TestWarnDroppedEnvTrustedProxies(t *testing.T) {
	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })

	warnDroppedEnvTrustedProxies([]string{"10.9.9.9", "0.0.0.0/0", "bogus"})
	assert.Contains(t, buf.String(), "refused by the proxy layer")
	assert.Contains(t, buf.String(), "0.0.0.0/0")

	buf.Reset()
	warnDroppedEnvTrustedProxies([]string{"10.9.9.9"})
	assert.Empty(t, buf.String(), "no warning when every entry is accepted")
}
