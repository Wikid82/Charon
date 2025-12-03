package cerberus_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDBForTest(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:cerberus_isenabled_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

func TestIsEnabled_ConfigTrue(t *testing.T) {
	cfg := config.SecurityConfig{CerberusEnabled: true}
	c := cerberus.New(cfg, nil)
	require.True(t, c.IsEnabled())
}

func TestIsEnabled_WAFModeEnabled(t *testing.T) {
	cfg := config.SecurityConfig{WAFMode: "block"}
	c := cerberus.New(cfg, nil)
	require.True(t, c.IsEnabled())
}

func TestIsEnabled_ACLModeEnabled(t *testing.T) {
	cfg := config.SecurityConfig{ACLMode: "enabled"}
	c := cerberus.New(cfg, nil)
	require.True(t, c.IsEnabled())
}

func TestIsEnabled_RateLimitModeEnabled(t *testing.T) {
	cfg := config.SecurityConfig{RateLimitMode: "enabled"}
	c := cerberus.New(cfg, nil)
	require.True(t, c.IsEnabled())
}

func TestIsEnabled_CrowdSecModeLocal(t *testing.T) {
	cfg := config.SecurityConfig{CrowdSecMode: "local"}
	c := cerberus.New(cfg, nil)
	require.True(t, c.IsEnabled())
}

func TestIsEnabled_DBSetting(t *testing.T) {
	db := setupDBForTest(t)
	// insert setting to database
	s := models.Setting{Key: "security.cerberus.enabled", Value: "true"}
	require.NoError(t, db.Create(&s).Error)
	cfg := config.SecurityConfig{}
	c := cerberus.New(cfg, db)
	require.True(t, c.IsEnabled())
}

func TestIsEnabled_DBSettingCaseInsensitive(t *testing.T) {
	db := setupDBForTest(t)
	s := models.Setting{Key: "security.cerberus.enabled", Value: "TrUe"}
	require.NoError(t, db.Create(&s).Error)
	cfg := config.SecurityConfig{}
	c := cerberus.New(cfg, db)
	require.True(t, c.IsEnabled())
}

func TestIsEnabled_DBSettingFalse(t *testing.T) {
	db := setupDBForTest(t)
	s := models.Setting{Key: "security.cerberus.enabled", Value: "false"}
	require.NoError(t, db.Create(&s).Error)
	cfg := config.SecurityConfig{}
	c := cerberus.New(cfg, db)
	require.False(t, c.IsEnabled())
}

func TestIsEnabled_DefaultFalse(t *testing.T) {
	cfg := config.SecurityConfig{}
	c := cerberus.New(cfg, nil)
	require.False(t, c.IsEnabled())
}
