package models

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

// orthrusTableInfo returns PRAGMA table_info for orthrus_agents.
func orthrusTableInfo(t *testing.T, db *gorm.DB) []map[string]interface{} {
	t.Helper()
	var rows []map[string]interface{}
	require.NoError(t, db.Raw("PRAGMA table_info(orthrus_agents)").Scan(&rows).Error)
	return rows
}

func TestOrthrusAgent_AutoMigrateAddsExternalProxyBind(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "orthrus.db")), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	// Build the table the way GORM does, then remove the new column, which
	// gives the exact schema an existing installation has before this change.
	require.NoError(t, db.AutoMigrate(&OrthrusAgent{}))
	require.NoError(t, db.Exec("ALTER TABLE orthrus_agents DROP COLUMN external_proxy_bind").Error)
	require.False(t, db.Migrator().HasColumn(&OrthrusAgent{}, "external_proxy_bind"))

	require.NoError(t, db.Exec(
		`INSERT INTO orthrus_agents (uuid, name, auth_key_hash, status, external_proxy_port, write_enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"legacy-uuid", "legacy-agent", "legacy-hash", "online", 23750, true,
	).Error)

	require.NoError(t, db.AutoMigrate(&OrthrusAgent{}))
	require.True(t, db.Migrator().HasColumn(&OrthrusAgent{}, "external_proxy_bind"))

	var agent OrthrusAgent
	require.NoError(t, db.Where("uuid = ?", "legacy-uuid").First(&agent).Error)
	assert.Equal(t, "", agent.ExternalProxyBind, "existing rows must back-fill to the automatic bind")
	assert.Equal(t, "legacy-agent", agent.Name)
	assert.Equal(t, "legacy-hash", agent.AuthKeyHash)
	assert.Equal(t, OrthrusStatusOnline, agent.Status)
	assert.Equal(t, 23750, agent.ExternalProxyPort)
	assert.True(t, agent.WriteEnabled)

	schemaAfterFirst := orthrusTableInfo(t, db)
	require.NoError(t, db.AutoMigrate(&OrthrusAgent{}), "second AutoMigrate must be a no-op")
	assert.Equal(t, schemaAfterFirst, orthrusTableInfo(t, db))
}
