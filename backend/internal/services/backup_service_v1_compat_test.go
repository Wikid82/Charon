package services

import (
	"archive/zip"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestCreateBackup_V1ByteCompatible_WithNewConstructorWiring is the Commit 2
// regression test required by spec §6 / the commit's must-not-regress list:
// NewBackupService's signature now threads a *gorm.DB and an
// *crypto.EncryptionService through, but CreateBackup()'s produced archive
// must remain structurally identical to what v1 produced — still just
// charon.db + caddy/**, with no manifest.json and no crowdsec/ entries, since
// wiring the manifest and crowdsec directory into archive creation is
// Commit 3's job, not this one (no behavior change in this commit).
func TestCreateBackup_V1ByteCompatible_WithNewConstructorWiring(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	caddyDir := filepath.Join(dataDir, "caddy")
	require.NoError(t, os.MkdirAll(caddyDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(caddyDir, "caddy.json"), []byte("{}"), 0o600))

	// A crowdsec dir is present on disk to prove it is NOT yet swept into the
	// archive — including crowdsec/** is Commit 3 (spec §3.2), even though
	// CrowdSecDir is now threaded through the constructor in this commit.
	crowdsecDir := filepath.Join(dataDir, "crowdsec")
	require.NoError(t, os.MkdirAll(crowdsecDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(crowdsecDir, "profiles.yaml"), []byte("x: 1"), 0o600))

	settingsDB, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	// A real (non-nil) encryption service proves the new dependency, once
	// wired, doesn't itself alter CreateBackup's output in this commit.
	encSvc, err := crypto.NewEncryptionService(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)

	cfg := &config.Config{DatabasePath: dbPath}
	cfg.Security.CrowdSecConfigDir = crowdsecDir

	svc := NewBackupService(cfg, settingsDB, encSvc)
	defer svc.Stop()

	filename, err := svc.CreateBackup()
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(filename, ".zip"), "v1 filenames end in .zip, got %s", filename)

	zipPath := filepath.Join(svc.BackupDir, filename)
	r, err := zip.OpenReader(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}

	assert.ElementsMatch(t, []string{"charon.db", "caddy/caddy.json"}, names,
		"v2 additions (manifest.json, crowdsec/**) must not appear until Commit 3")
}
