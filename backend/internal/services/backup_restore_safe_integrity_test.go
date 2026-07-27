package services

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// corruptSQLiteFileForIntegrityCheck takes a path to an already-valid,
// closed SQLite file (built via createCharonLikeTestDB or equivalent, and
// containing at least one indexed column so a "row missing from index"
// class of structural corruption is reachable) and rewrites it in place
// with a single flipped byte chosen so that:
//   - the file still opens via sql.Open + a subsequent query (no driver-
//     level "database disk image is malformed" error from Scan itself)
//   - PRAGMA integrity_check's Scan succeeds and returns a non-"ok" string
//
// This does NOT hardcode a byte offset: naive bit-flips frequently land in
// unused page space or non-structural payload bytes and are silently
// tolerated by integrity_check (verified empirically — see spec §2.3), and
// the "in database main..." style corruption reports SQLite produces are
// page-layout-dependent (sqlite3/go-sqlite3 version, page size, row count).
// Instead, it probes a bounded range of offsets against a scratch copy at
// test-run time and uses the first one that reproduces the desired
// "successful Scan, non-ok result" condition, failing the test outright
// (not skipping) if none is found within the bound — a portable fixture
// generator, not a fragile magic number.
func corruptSQLiteFileForIntegrityCheck(t *testing.T, path string) {
	t.Helper()
	orig, err := os.ReadFile(path)
	require.NoError(t, err)

	const pageSize = 4096
	for offset := pageSize; offset < len(orig); offset += 8 {
		candidate := make([]byte, len(orig))
		copy(candidate, orig)
		candidate[offset] ^= 0xFF

		probePath := path + ".probe"
		require.NoError(t, os.WriteFile(probePath, candidate, 0o600))

		db, openErr := sql.Open("sqlite3", probePath)
		if openErr != nil {
			_ = os.Remove(probePath)
			continue
		}
		var result string
		scanErr := db.QueryRow("PRAGMA integrity_check").Scan(&result)
		_ = db.Close()
		_ = os.Remove(probePath)

		if scanErr == nil && !strings.EqualFold(strings.TrimSpace(result), "ok") {
			require.NoError(t, os.WriteFile(path, candidate, 0o600))
			return
		}
	}
	t.Fatal("could not find a byte offset that reproduces a non-fatal PRAGMA integrity_check corruption result")
}

// buildIndexedCharonLikeTestDB builds on createCharonLikeTestDB by adding a
// secondary index (idx_users_email) and enough additional rows that the
// index occupies more than one page, giving
// corruptSQLiteFileForIntegrityCheck's bounded probe a realistic chance of
// finding a "row missing from index" class of corruption within its search
// bound (spec §2.3/§3.4.3).
func buildIndexedCharonLikeTestDB(t *testing.T, dbPath string) {
	t.Helper()

	createCharonLikeTestDB(t, dbPath, 0)

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE UNIQUE INDEX idx_users_email ON users(email)`)
	require.NoError(t, err)

	for i := 0; i < 50; i++ {
		_, err = db.Exec(`INSERT INTO users (email) VALUES (?)`, fmt.Sprintf("user%d@example.com", i))
		require.NoError(t, err)
	}
}

// TestSanityCheckSQLiteFile_IntegrityCheckFailure_Rejected is a direct unit
// test of the actual corruption-rejection branch (H2, backup_restore_safe.go
// ~473-475): a genuinely PRAGMA integrity_check-failing SQLite file (not
// merely "wrong tables" or "not a zip", both of which are already covered
// elsewhere) must be rejected with an error containing "database integrity
// check failed".
func TestSanityCheckSQLiteFile_IntegrityCheckFailure_Rejected(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "corrupt.db")
	buildIndexedCharonLikeTestDB(t, dbPath)

	corruptSQLiteFileForIntegrityCheck(t, dbPath)

	err := sanityCheckSQLiteFile(dbPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database integrity check failed")
}

// TestRestoreBackupSafe_And_ValidateBackup_CorruptIntegrity_Rejected is the
// end-to-end counterpart (H2): a v2 archive whose extracted database fails
// PRAGMA integrity_check must be rejected by both ValidateBackup and
// RestoreBackupSafe with ErrBackupValidationFailed, and RestoreBackupSafe
// must create zero pre-restore safety backups for it — proving V6 rejected
// the archive before S1/A1 ever ran.
func TestRestoreBackupSafe_And_ValidateBackup_CorruptIntegrity_Rejected(t *testing.T) {
	svc := newHardeningTestService(t)

	corruptDBPath := filepath.Join(t.TempDir(), "corrupt-charon.db")
	buildIndexedCharonLikeTestDB(t, corruptDBPath)
	corruptSQLiteFileForIntegrityCheck(t, corruptDBPath)

	filename := "corrupt_integrity.zip"
	zipPath := filepath.Join(svc.BackupDir, filename)
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)

	var entries []ManifestEntry
	require.NoError(t, svc.addToZipTracked(w, corruptDBPath, svc.DatabaseName, &entries))

	manifest := BackupManifest{
		FormatVersion: 2,
		DatabaseName:  svc.DatabaseName,
		Contents:      entries,
	}
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	mw, err := w.Create("manifest.json")
	require.NoError(t, err)
	_, err = mw.Write(manifestBytes)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	// ValidateBackup: a pure dry run, nothing mutated.
	_, err = svc.ValidateBackup(filename, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBackupValidationFailed)
	assert.Contains(t, err.Error(), "database integrity check failed")

	before, err := svc.ListBackups()
	require.NoError(t, err)
	beforeCount := len(before)

	// RestoreBackupSafe: must reject before S1 (pre-restore safety backup)
	// or A1 (apply) ever run.
	result, err := svc.RestoreBackupSafe(filename, "")
	require.Error(t, err)
	require.Nil(t, result)
	assert.ErrorIs(t, err, ErrBackupValidationFailed)
	assert.Contains(t, err.Error(), "database integrity check failed")

	after, err := svc.ListBackups()
	require.NoError(t, err)
	assert.Equal(t, beforeCount, len(after), "V6 rejection must create zero pre-restore safety backups")
}
