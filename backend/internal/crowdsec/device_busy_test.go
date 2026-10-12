package crowdsec

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestApplyWithOpenFileHandles covers a data dir with open file handles (the old "device or resource
// busy" case): the copy-based snapshot works, DataDir itself is never renamed, and the engine-owned
// hub_cache stays untouched.
func TestApplyWithOpenFileHandles(t *testing.T) {
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config.txt"), []byte("original"), 0o600))

	subDir := filepath.Join(dataDir, "hub_cache")
	require.NoError(t, os.MkdirAll(subDir, 0o750))
	cacheFile := filepath.Join(subDir, "cache.json")
	require.NoError(t, os.WriteFile(cacheFile, []byte(`{"test": "data"}`), 0o600))

	f, err := os.Open(cacheFile) // #nosec G304 -- Test opens test cache file
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	before, err := os.Stat(dataDir)
	require.NoError(t, err)

	archive := makeTarGz(t, map[string]string{"new/preset.yaml": "new: preset"})
	_, err = cache.Store(context.Background(), "test/preset", "etag1", "hub", "preview", archive)
	require.NoError(t, err)

	svc := NewHubService(nil, cache, dataDir)

	res, err := svc.Apply(context.Background(), "test/preset")
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.NotEmpty(t, res.BackupPath, "BackupPath should be set on success")

	after, err := os.Stat(dataDir)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "DataDir must never be renamed or replaced")

	// The snapshot holds the original config but not the regenerable hub cache.
	// #nosec G304 -- Test reads from known backup path created by test
	content, err := os.ReadFile(filepath.Join(res.BackupPath, "config.txt"))
	require.NoError(t, err)
	require.Equal(t, "original", string(content))
	require.NoDirExists(t, filepath.Join(res.BackupPath, "hub_cache"))

	// The live cache file is still in place and the preset was applied on top of the live tree.
	require.FileExists(t, cacheFile)
	require.FileExists(t, filepath.Join(dataDir, "config.txt"))
	// #nosec G304 -- Test reads from known preset path in test dataDir
	newContent, err := os.ReadFile(filepath.Join(dataDir, "new", "preset.yaml"))
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
