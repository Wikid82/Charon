package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupService_CreateAndList(t *testing.T) {
	// Setup temp dirs
	tmpDir, err := os.MkdirTemp("", "cpm-backup-service-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	err = os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	// Create dummy DB
	dbPath := filepath.Join(dataDir, "cpm.db")
	err = os.WriteFile(dbPath, []byte("dummy db"), 0644)
	require.NoError(t, err)

	// Create dummy caddy dir
	caddyDir := filepath.Join(dataDir, "caddy")
	err = os.MkdirAll(caddyDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(caddyDir, "caddy.json"), []byte("{}"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)

	// Test Create
	filename, err := service.CreateBackup()
	require.NoError(t, err)
	assert.NotEmpty(t, filename)
	assert.FileExists(t, filepath.Join(service.BackupDir, filename))

	// Test List
	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Equal(t, filename, backups[0].Filename)
	assert.True(t, backups[0].Size > 0)

	// Test Restore (Basic check that it unzips)
	// Modify the "current" file to verify restore overwrites/restores it
	err = os.WriteFile(dbPath, []byte("modified db"), 0644)
	require.NoError(t, err)

	err = service.RestoreBackup(filename)
	require.NoError(t, err)

	// Verify content restored
	content, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	assert.Equal(t, "dummy db", string(content))
}

func TestBackupService_Cron(t *testing.T) {
	// Just verify cron is running/scheduled
	tmpDir, err := os.MkdirTemp("", "cpm-backup-cron-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0755)
	cfg := &config.Config{DatabasePath: filepath.Join(dataDir, "cpm.db")}

	service := NewBackupService(cfg)
	entries := service.Cron.Entries()
	assert.Len(t, entries, 1)
}
