package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestCleanupOldBackups_ExcludesPreRestoreRecordsFromRetention proves M4:
// with a real, wired s.db, CleanupOldBackups must never delete a
// pre_restore-type backup regardless of its age or position in the
// retention ordering, and pre_restore entries must be excluded from the
// retention-count denominator entirely (not merely spared deletion) —
// backup_service.go:400-409's `prunable` slice is built without them before
// the keep/delete split is computed.
func TestCleanupOldBackups_ExcludesPreRestoreRecordsFromRetention(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	backupDir := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	require.NoError(t, os.MkdirAll(backupDir, 0o700))

	dbPath := filepath.Join(dataDir, "charon.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.BackupRecord{}))

	svc := &BackupService{DataDir: dataDir, BackupDir: backupDir, db: db}

	// Seed 2 pre_restore records with old timestamps (so age alone would
	// otherwise make them the first candidates for pruning).
	preRestoreFilenames := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		filename := fmt.Sprintf("backup_pre_restore_%d.zip", i)
		writeEmptyBackupFile(t, backupDir, filename, time.Date(2020, 1, i+1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, db.Create(&models.BackupRecord{
			UUID: uuid.NewString(), Filename: filename, Type: "pre_restore", Status: "completed",
		}).Error)
		preRestoreFilenames = append(preRestoreFilenames, filename)
	}

	// Seed 5 manual records, newer than the pre_restore ones, so age-based
	// ordering alone would never explain the pre_restore survival below.
	manualFilenames := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		filename := fmt.Sprintf("backup_manual_%d.zip", i)
		writeEmptyBackupFile(t, backupDir, filename, time.Date(2025, 1, i+1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, db.Create(&models.BackupRecord{
			UUID: uuid.NewString(), Filename: filename, Type: "manual", Status: "completed",
		}).Error)
		manualFilenames = append(manualFilenames, filename)
	}

	const keep = 2
	deleted, err := svc.CleanupOldBackups(keep)
	require.NoError(t, err)

	// 5 manual backups, keep=2 => 3 deleted. The pre_restore records must be
	// excluded from the retention-count denominator entirely: if they were
	// merely "spared deletion" but still counted toward `keep`, the deleted
	// count would differ from this.
	assert.Equal(t, 5-keep, deleted, "pre_restore records must not count toward the retention denominator")

	for _, filename := range preRestoreFilenames {
		assert.FileExists(t, filepath.Join(backupDir, filename), "pre_restore backups must survive cleanup regardless of age")
	}

	remaining, err := svc.ListBackups()
	require.NoError(t, err)
	assert.Len(t, remaining, len(preRestoreFilenames)+keep, "both pre_restore files plus the 2 newest manual files should remain")

	// manualFilenames was seeded in oldest-to-newest order (index 0 = Jan 1,
	// index 4 = Jan 5), so ListBackups' newest-first ordering keeps the
	// LAST `keep` entries of this slice and deletes the rest.
	for i, filename := range manualFilenames {
		_, statErr := os.Stat(filepath.Join(backupDir, filename))
		if i >= len(manualFilenames)-keep {
			assert.NoError(t, statErr, "the %d newest manual backups should be kept", keep)
		} else {
			assert.True(t, os.IsNotExist(statErr), "manual backup %q beyond the retention count should be deleted", filename)
		}
	}
}

// writeEmptyBackupFile creates an empty file at backupDir/filename and sets
// its mtime, mirroring the existing CleanupOldBackups test fixture pattern
// (backup_service_test.go:262) so ListBackups' newest-first ordering is
// deterministic.
func writeEmptyBackupFile(t *testing.T, backupDir, filename string, modTime time.Time) {
	t.Helper()

	zipPath := filepath.Join(backupDir, filename)
	f, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, os.Chtimes(zipPath, modTime, modTime))
}
