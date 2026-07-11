package services

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyWarningMessage must match backup_restore_safe.go's readBackupManifest
// warning verbatim (spec §5 "legacy v1 zip restores with warning") — kept as
// its own constant so a future wording change breaks this test loudly rather
// than the assertion silently losing its meaning.
const legacyWarningMessage = "Restoring/validating a legacy v1 backup archive with no manifest; skipping checksum verification"

// buildLegacyV1ZipArchive writes a genuine pre-format-v2 backup archive at
// zipPath: just charon.db + caddy/** entries, with NO manifest.json entry at
// all (mimicking an actual v1-era backup rather than a synthetic fixture
// that merely omits a field). readBackupManifest must classify this as
// legacyFormat=true.
func buildLegacyV1ZipArchive(t *testing.T, zipPath string, dbContent []byte) {
	t.Helper()

	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	defer func() { _ = out.Close() }()

	w := zip.NewWriter(out)

	dbWriter, err := w.Create("charon.db")
	require.NoError(t, err)
	_, err = dbWriter.Write(dbContent)
	require.NoError(t, err)

	caddyWriter, err := w.Create("caddy/caddy.json")
	require.NoError(t, err)
	_, err = caddyWriter.Write([]byte(`{"legacy":true}`))
	require.NoError(t, err)

	require.NoError(t, w.Close())
}

// TestValidateBackup_LegacyV1NoManifest_FlagsLegacyFormatWithWarning is
// required test #2a from the Issue #32 gap-closing QA report: a genuine
// no-manifest zip (the only shape a real pre-format-v2 backup ever had) must
// validate successfully, report LegacyFormat true, skip checksum
// verification without error, and log the documented warning — proven
// against real backend code, not a mocked API response.
func TestValidateBackup_LegacyV1NoManifest_FlagsLegacyFormatWithWarning(t *testing.T) {
	svc := newHardeningTestService(t)

	dbContent, err := os.ReadFile(filepath.Join(svc.DataDir, svc.DatabaseName))
	require.NoError(t, err)

	filename := "legacy_v1_backup.zip"
	buildLegacyV1ZipArchive(t, filepath.Join(svc.BackupDir, filename), dbContent)

	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })

	result, err := svc.ValidateBackup(filename, "")
	require.NoError(t, err, "a legacy no-manifest archive must validate successfully, not be rejected")
	require.NotNil(t, result)

	assert.True(t, result.Valid)
	assert.True(t, result.LegacyFormat, "no-manifest archives must be flagged legacy_format")
	assert.Equal(t, 1, result.FormatVersion, "legacy archives report format_version 1")
	assert.Equal(t, "ok", result.DatabaseIntegrity)
	assert.Contains(t, buf.String(), legacyWarningMessage,
		"skipping checksum verification for a legacy archive must be logged as a warning")
}

// TestRestoreBackupSafe_LegacyV1NoManifest_RestoresWithWarning is the
// restore-pipeline half of required test #2a: RestoreBackupSafe must run the
// full V->S->A->R pipeline against a genuine no-manifest archive end-to-end
// with no checksum-verification error, reporting LegacyFormat true in the
// result the handler serializes back to the caller.
func TestRestoreBackupSafe_LegacyV1NoManifest_RestoresWithWarning(t *testing.T) {
	svc := newHardeningTestService(t)

	dbContent, err := os.ReadFile(filepath.Join(svc.DataDir, svc.DatabaseName))
	require.NoError(t, err)

	filename := "legacy_v1_backup.zip"
	buildLegacyV1ZipArchive(t, filepath.Join(svc.BackupDir, filename), dbContent)

	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })

	result, err := svc.RestoreBackupSafe(filename, "")
	require.NoError(t, err, "a legacy no-manifest archive must restore successfully, not be rejected")
	require.NotNil(t, result)

	assert.True(t, result.LegacyFormat, "restore result must surface legacy_format so the frontend can warn the user")
	assert.NotEmpty(t, result.PreRestoreBackup, "S1 safety backup must still run for a legacy restore")
	assert.Contains(t, buf.String(), legacyWarningMessage,
		"skipping checksum verification for a legacy archive must be logged as a warning")
}
