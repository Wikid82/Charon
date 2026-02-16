package handlers

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestFlattenConfig_NestedAndScalars(t *testing.T) {
	result := map[string]string{}
	input := map[string]interface{}{
		"security": map[string]interface{}{
			"acl": map[string]interface{}{
				"enabled": true,
			},
			"admin_whitelist": "192.0.2.0/24",
		},
		"port": 8080,
	}

	flattenConfig(input, "", result)

	require.Equal(t, "true", result["security.acl.enabled"])
	require.Equal(t, "192.0.2.0/24", result["security.admin_whitelist"])
	require.Equal(t, "8080", result["port"])
}

func TestValidateAdminWhitelist(t *testing.T) {
	tests := []struct {
		name      string
		whitelist string
		wantErr   bool
	}{
		{name: "empty valid", whitelist: "", wantErr: false},
		{name: "single valid cidr", whitelist: "192.0.2.0/24", wantErr: false},
		{name: "multiple with spaces", whitelist: "192.0.2.0/24, 203.0.113.1/32", wantErr: false},
		{name: "blank entries ignored", whitelist: "192.0.2.0/24, ,", wantErr: false},
		{name: "invalid no slash", whitelist: "192.0.2.1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdminWhitelist(tt.whitelist)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSettingsHandler_EnsureSecurityConfigEnabledWithDB(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	h := NewSettingsHandler(db)

	require.NoError(t, h.ensureSecurityConfigEnabledWithDB(db))

	var cfg models.SecurityConfig
	require.NoError(t, db.Where("name = ?", "default").First(&cfg).Error)
	require.True(t, cfg.Enabled)

	cfg.Enabled = false
	require.NoError(t, db.Save(&cfg).Error)
	require.NoError(t, h.ensureSecurityConfigEnabledWithDB(db))
	require.NoError(t, db.Where("name = ?", "default").First(&cfg).Error)
	require.True(t, cfg.Enabled)
}

func TestSettingsHandler_SyncAdminWhitelistWithDB(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	h := NewSettingsHandler(db)

	require.NoError(t, h.syncAdminWhitelistWithDB(db, "198.51.100.0/24"))

	var cfg models.SecurityConfig
	require.NoError(t, db.Where("name = ?", "default").First(&cfg).Error)
	require.Equal(t, "198.51.100.0/24", cfg.AdminWhitelist)

	require.NoError(t, h.syncAdminWhitelistWithDB(db, "203.0.113.0/24"))
	require.NoError(t, db.Where("name = ?", "default").First(&cfg).Error)
	require.Equal(t, "203.0.113.0/24", cfg.AdminWhitelist)
}
