package handlers

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSettingsWave3DB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}, &models.SecurityAudit{}))
	return db
}

func TestSettingsHandler_EnsureSecurityConfigEnabledWithDB_Branches(t *testing.T) {
	db := setupSettingsWave3DB(t)
	h := &SettingsHandler{DB: db}

	// Record missing -> create enabled
	require.NoError(t, h.ensureSecurityConfigEnabledWithDB(db))
	var cfg models.SecurityConfig
	require.NoError(t, db.Where("name = ?", "default").First(&cfg).Error)
	require.True(t, cfg.Enabled)

	// Record exists enabled=false -> update to true
	require.NoError(t, db.Model(&cfg).Update("enabled", false).Error)
	require.NoError(t, h.ensureSecurityConfigEnabledWithDB(db))
	require.NoError(t, db.Where("name = ?", "default").First(&cfg).Error)
	require.True(t, cfg.Enabled)

	// Record exists enabled=true -> no-op success
	require.NoError(t, h.ensureSecurityConfigEnabledWithDB(db))
}

func TestFlattenConfig_MixedTypes(t *testing.T) {
	result := map[string]string{}
	input := map[string]interface{}{
		"security": map[string]interface{}{
			"acl": map[string]interface{}{
				"enabled": true,
			},
			"rate_limit": map[string]interface{}{
				"requests": 100,
			},
		},
		"name": "charon",
	}

	flattenConfig(input, "", result)

	require.Equal(t, "true", result["security.acl.enabled"])
	require.Equal(t, "100", result["security.rate_limit.requests"])
	require.Equal(t, "charon", result["name"])
}

func TestValidateAdminWhitelist_Strictness(t *testing.T) {
	require.NoError(t, validateAdminWhitelist(""))
	require.NoError(t, validateAdminWhitelist("192.0.2.0/24, 198.51.100.10/32"))
	require.Error(t, validateAdminWhitelist("192.0.2.1"))
}
