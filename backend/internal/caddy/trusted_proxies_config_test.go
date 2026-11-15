package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

func TestTrustedProxiesFor_LegacyRangesWhenUnconfigured(t *testing.T) {
	got := trustedProxiesFor(nil)
	assert.Equal(t, "static", got.Source)
	assert.Equal(t, []string{"127.0.0.1/32", "::1/128", "172.16.0.0/12", "10.0.0.0/8", "192.168.0.0/16"}, got.Ranges)
}

func TestTrustedProxiesFor_ConfiguredRangesReplaceLegacy(t *testing.T) {
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

func TestGenerateConfig_WithoutTrustedProxiesKeepsLegacyRanges(t *testing.T) {
	cfg, err := GenerateConfig(nil, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, cfg.Apps.HTTP.Servers["charon_server"].TrustedProxies.Ranges, "172.16.0.0/12")
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
	m := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{})

	ranges, ok := m.trustedProxyRanges()
	assert.False(t, ok, "nothing saved: the legacy ranges stay in force")
	assert.Nil(t, ranges)

	saved := "172.18.0.5/32"
	require.NoError(t, models.WriteTrustedProxiesSetting(db, &saved))
	ranges, ok = m.trustedProxyRanges()
	require.True(t, ok)
	assert.Equal(t, []string{"127.0.0.1/32", "::1/128", "172.18.0.5/32"}, ranges)

	invalid := "0.0.0.0/0"
	require.NoError(t, models.WriteTrustedProxiesSetting(db, &invalid))
	ranges, ok = m.trustedProxyRanges()
	require.True(t, ok)
	assert.Equal(t, []string{"127.0.0.1/32", "::1/128"}, ranges, "an invalid saved list trusts loopback only")
}

func TestManagerTrustedProxyRanges_ReadErrorKeepsLegacy(t *testing.T) {
	db := trustedProxyManagerDB(t)
	m := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{})
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	ranges, ok := m.trustedProxyRanges()
	assert.False(t, ok)
	assert.Nil(t, ranges)
}
