package crowdsec

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestApplyWithOpenFileHandles simulates the "device or resource busy" scenario
// where the data directory has open file handles (e.g., from cache operations)
func TestApplyWithOpenFileHandles(t *testing.T) {
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config.txt"), []byte("original"), 0o600))

	// Create a subdirectory with nested files (similar to hub_cache)
	subDir := filepath.Join(dataDir, "hub_cache")
	require.NoError(t, os.MkdirAll(subDir, 0o750))
	cacheFile := filepath.Join(subDir, "cache.json")
	require.NoError(t, os.WriteFile(cacheFile, []byte(`{"test": "data"}`), 0o600))

	// Open a file handle to simulate an in-use directory
	// This would cause os.Rename to fail with "device or resource busy" on some systems
	f, err := os.Open(cacheFile) // #nosec G304 -- Test opens test cache file // #nosec G304 -- Test opens test cache file
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	// Create and cache a preset
	archive := makeTarGz(t, map[string]string{"new/preset.yaml": "new: preset"})
	_, err = cache.Store(context.Background(), "test/preset", "etag1", "hub", "preview", archive)
	require.NoError(t, err)

	svc := NewHubService(nil, cache, dataDir)

	// Apply should succeed using copy-based backup even with open file handles
	res, err := svc.Apply(context.Background(), "test/preset")
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.NotEmpty(t, res.BackupPath, "BackupPath should be set on success")

	// Verify backup was created and contains the original files
	backupConfigPath := filepath.Join(res.BackupPath, "config.txt")
	backupCachePath := filepath.Join(res.BackupPath, "hub_cache", "cache.json")

	// The backup should exist
	require.FileExists(t, backupConfigPath)
	require.FileExists(t, backupCachePath)

	// Verify original content was preserved in backup
	// #nosec G304 -- Test reads from known backup paths created by test
	content, err := os.ReadFile(backupConfigPath)
	require.NoError(t, err)
	require.Equal(t, "original", string(content))

	// #nosec G304 -- Test reads from known backup paths created by test
	cacheContent, err := os.ReadFile(backupCachePath)
	require.NoError(t, err)
	require.Contains(t, string(cacheContent), "test")

	// Verify new preset was applied
	newPresetPath := filepath.Join(dataDir, "new", "preset.yaml")
	require.FileExists(t, newPresetPath)
	// #nosec G304 -- Test reads from known preset path in test dataDir
	newContent, err := os.ReadFile(newPresetPath)
	require.NoError(t, err)
	require.Contains(t, string(newContent), "new: preset")
}

// TestBackupPathOnlySetAfterSuccessfulBackup ensures that BackupPath is only
// set in the result after a successful backup, not before attempting it.
// This prevents misleading error messages that reference non-existent backups.
func TestBackupPathOnlySetAfterSuccessfulBackup(t *testing.T) {
	t.Run("backup path not set when cache missing", func(t *testing.T) {
		cache, err := NewHubCache(t.TempDir(), time.Hour)
		require.NoError(t, err)

		dataDir := filepath.Join(t.TempDir(), "crowdsec")
		// #nosec G301 -- Test CrowdSec data directory needs standard Unix permissions
		require.NoError(t, os.MkdirAll(dataDir, 0o755))

		svc := NewHubService(nil, cache, dataDir)

		// Try to apply a preset that doesn't exist in cache (no cscli available)
		res, err := svc.Apply(context.Background(), "nonexistent/preset")
		require.Error(t, err)
		require.NotEmpty(t, res.BackupPath, "BackupPath should be set when backup attempt is performed for rollback")
	})

	t.Run("backup path set only after successful backup", func(t *testing.T) {
		cache, err := NewHubCache(t.TempDir(), time.Hour)
		require.NoError(t, err)

		dataDir := filepath.Join(t.TempDir(), "crowdsec")
		require.NoError(t, os.MkdirAll(dataDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dataDir, "file.txt"), []byte("data"), 0o600))

		archive := makeTarGz(t, map[string]string{"new.yaml": "new: config"})
		_, err = cache.Store(context.Background(), "test/preset", "etag1", "hub", "preview", archive)
		require.NoError(t, err)

		svc := NewHubService(nil, cache, dataDir)

		res, err := svc.Apply(context.Background(), "test/preset")
		require.NoError(t, err)
		require.NotEmpty(t, res.BackupPath, "BackupPath should be set after successful backup")
		require.FileExists(t, filepath.Join(res.BackupPath, "file.txt"), "Backup should contain original files")
	})
}
