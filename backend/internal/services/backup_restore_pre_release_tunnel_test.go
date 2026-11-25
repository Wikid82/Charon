package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// preReleaseTunnelConfig is tunnel_configs as the previous release created it:
// the same columns as models.TunnelConfig, without key_version.
type preReleaseTunnelConfig struct {
	ID                   uint   `gorm:"primaryKey"`
	UUID                 string `gorm:"uniqueIndex;not null"`
	Name                 string `gorm:"not null;index"`
	Provider             string `gorm:"not null;index"`
	EncryptedCredentials string `gorm:"type:text;not null"`
	Configuration        string `gorm:"type:text"`
	IsActive             bool   `gorm:"default:false;index"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (preReleaseTunnelConfig) TableName() string { return "tunnel_configs" }

// TestRehydrateLiveDatabase_PreReleaseTunnelTableFailsWithoutTouchingLiveData (E29):
// a backup whose tunnel_configs has one column fewer cannot be copied positionally.
// The copy fails inside the rehydrate transaction, so the live tables are left as
// they were and the caller takes the restart-required path.
func TestRehydrateLiveDatabase_PreReleaseTunnelTableFailsWithoutTouchingLiveData(t *testing.T) {
	dataDir := t.TempDir()
	livePath := filepath.Join(dataDir, "charon.db")
	live, err := gorm.Open(sqlite.Open(livePath), &gorm.Config{})
	require.NoError(t, err)
	liveSQL, err := live.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = liveSQL.Close() })
	require.NoError(t, live.AutoMigrate(&models.TunnelConfig{}, &models.User{}))

	liveTunnel := models.TunnelConfig{UUID: uuid.NewString(), Name: "live", Provider: models.ProviderCloudflare, KeyVersion: 2}
	require.NoError(t, live.Create(&liveTunnel).Error)

	// The pre-release backup: old tunnel_configs shape with one row.
	backupPath := filepath.Join(t.TempDir(), "pre-release.db")
	backup, err := gorm.Open(sqlite.Open(backupPath), &gorm.Config{})
	require.NoError(t, err)
	backupSQL, err := backup.DB()
	require.NoError(t, err)
	require.NoError(t, backup.Migrator().CreateTable(&preReleaseTunnelConfig{}))
	require.NoError(t, backup.Create(&preReleaseTunnelConfig{UUID: uuid.NewString(), Name: "from backup", Provider: "cloudflare"}).Error)
	require.NoError(t, backupSQL.Close())

	svc := NewBackupService(&config.Config{DatabasePath: livePath}, nil, nil)
	t.Cleanup(svc.Stop)
	svc.restoreDBPath = backupPath

	err = svc.RehydrateLiveDatabase(live)
	require.Error(t, err, "a positional copy of the old shape must fail, so the caller falls back to restart-required")

	var names []string
	require.NoError(t, live.Model(&models.TunnelConfig{}).Pluck("name", &names).Error)
	assert.Equal(t, []string{"live"}, names, "the failed rehydrate must roll back and leave live rows unchanged")

	var keyVersion int
	require.NoError(t, live.Model(&models.TunnelConfig{}).Select("key_version").Where("id = ?", liveTunnel.ID).Scan(&keyVersion).Error)
	assert.Equal(t, 2, keyVersion, "the live key version column is intact")
}
