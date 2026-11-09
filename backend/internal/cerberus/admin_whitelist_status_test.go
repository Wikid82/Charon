package cerberus

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

func newAdminWhitelistTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:admin_whitelist_status_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	return db
}

// TestAdminWhitelistStatus_NamedResultOrder pins the result order of adminWhitelistStatus:
// the first value means "allowed" (client IP is in the admin whitelist) and the second
// means "configured" (an admin whitelist exists and is non-empty).
func TestAdminWhitelistStatus_NamedResultOrder(t *testing.T) {
	t.Run("nil database is neither allowed nor configured", func(t *testing.T) {
		c := New(config.SecurityConfig{}, nil)
		allowed, configured := c.adminWhitelistStatus("10.0.0.5")
		require.False(t, allowed)
		require.False(t, configured)
	})

	t.Run("no security config is neither allowed nor configured", func(t *testing.T) {
		db := newAdminWhitelistTestDB(t)
		c := New(config.SecurityConfig{}, db)
		allowed, configured := c.adminWhitelistStatus("10.0.0.5")
		require.False(t, allowed)
		require.False(t, configured)
	})

	t.Run("empty admin whitelist is neither allowed nor configured", func(t *testing.T) {
		db := newAdminWhitelistTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true, AdminWhitelist: "  "}).Error)
		c := New(config.SecurityConfig{}, db)
		allowed, configured := c.adminWhitelistStatus("10.0.0.5")
		require.False(t, allowed)
		require.False(t, configured)
	})

	t.Run("configured whitelist with matching IP is allowed and configured", func(t *testing.T) {
		db := newAdminWhitelistTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true, AdminWhitelist: "10.0.0.0/8"}).Error)
		c := New(config.SecurityConfig{}, db)
		allowed, configured := c.adminWhitelistStatus("10.0.0.5")
		require.True(t, allowed)
		require.True(t, configured)
	})

	t.Run("configured whitelist without matching IP is not allowed but configured", func(t *testing.T) {
		db := newAdminWhitelistTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true, AdminWhitelist: "10.0.0.0/8"}).Error)
		c := New(config.SecurityConfig{}, db)
		allowed, configured := c.adminWhitelistStatus("203.0.113.9")
		require.False(t, allowed)
		require.True(t, configured)
	})
}
