package services

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
)

// TestBackupService_ExcludesGeoIPFolder pins that a backup never carries the GeoIP data folder: the uploaded database,
// its previous copy and any upload temp file stay on the host, and a restore brings back only what the backup holds.
func TestBackupService_ExcludesGeoIPFolder(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)
	caddyDir := filepath.Join(dataDir, "caddy")
	require.NoError(t, os.MkdirAll(caddyDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(caddyDir, "caddy.json"), []byte("{}"), 0o600))

	geoipDir := filepath.Join(dataDir, "geoip")
	require.NoError(t, os.MkdirAll(geoipDir, 0o750))
	for _, name := range []string{"GeoLite2-Country.mmdb", "GeoLite2-Country.mmdb.prev", ".GeoLite2-Country.mmdb.upload-123"} {
		require.NoError(t, os.WriteFile(filepath.Join(geoipDir, name), []byte("database bytes"), 0o600))
	}

	service := NewBackupService(&config.Config{DatabasePath: dbPath}, nil, nil)
	t.Cleanup(service.discardRestoreSnapshot)
	defer service.Stop()

	filename, err := service.CreateBackup()
	require.NoError(t, err)

	r, err := zip.OpenReader(filepath.Join(service.BackupDir, filename))
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
		assert.False(t, strings.Contains(f.Name, "geoip"), "backup entry %q must not come from the GeoIP folder", f.Name)
	}
	assert.Contains(t, names, "caddy/caddy.json", "the archive does hold the rest of the data folder")
}
