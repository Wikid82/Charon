package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// refuseSessionUpdates makes every UPDATE of an import session row fail, the way a storage error would.
func refuseSessionUpdates(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("CREATE TRIGGER refuse_session_update BEFORE UPDATE ON import_sessions"+
		" BEGIN SELECT RAISE(ABORT, 'refused by test'); END;").Error)
}

// A stale session whose file is not under uploads/ (an older mounted path, with no mount configured now) is neither
// expired nor has anything removed.
func TestSweepImportUploadsLeavesSessionsOutsideUploads(t *testing.T) {
	t.Parallel()
	db := setupImportTestDB(t)
	importDir := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "Caddyfile")
	require.NoError(t, os.WriteFile(elsewhere, []byte("x"), 0o600))
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&models.ImportSession{UUID: "outside", SourceFile: elsewhere, Status: "pending"}).Error)
	require.NoError(t, db.Model(&models.ImportSession{}).Where("uuid = ?", "outside").
		UpdateColumn("created_at", now.Add(-25*time.Hour)).Error)

	require.NoError(t, sweepImportUploads(db, importDir, "", now))

	var row models.ImportSession
	require.NoError(t, db.Where("uuid = ?", "outside").First(&row).Error)
	assert.Equal(t, "pending", row.Status)
	_, err := os.Stat(elsewhere)
	assert.NoError(t, err)
}

func TestSweepImportUploadsReportsAFailedExpiry(t *testing.T) {
	t.Parallel()
	db := setupImportTestDB(t)
	importDir := t.TempDir()
	stale := writeUpload(t, importDir, "stale.caddyfile")
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&models.ImportSession{UUID: "stale", SourceFile: stale, Status: "pending"}).Error)
	require.NoError(t, db.Model(&models.ImportSession{}).Where("uuid = ?", "stale").
		UpdateColumn("created_at", now.Add(-25*time.Hour)).Error)
	refuseSessionUpdates(t, db)

	err := sweepImportUploads(db, importDir, "", now)

	assert.ErrorContains(t, err, "expire import session")
	_, statErr := os.Stat(stale)
	assert.NoError(t, statErr, "the file stays while its session could not be expired")
}

func TestSupersedeUploadSessionsReportsAFailedExpiry(t *testing.T) {
	t.Parallel()
	db := setupImportTestDB(t)
	importDir := t.TempDir()
	oldFile := writeUpload(t, importDir, "old.caddyfile")
	require.NoError(t, db.Create(&models.ImportSession{UUID: "old", SourceFile: oldFile, Status: "pending"}).Error)
	refuseSessionUpdates(t, db)

	err := supersedeUploadSessions(db, importDir, "new")

	assert.ErrorContains(t, err, "expire import session")
}

func TestRetentionReportsAFailedSessionLookup(t *testing.T) {
	t.Parallel()
	t.Run("orphan sweep", func(t *testing.T) {
		t.Parallel()
		db := setupImportTestDB(t)
		importDir := t.TempDir()
		writeUpload(t, importDir, "any.caddyfile")
		require.NoError(t, db.Migrator().DropTable(&models.ImportSession{}))

		err := removeOrphanUploads(db, importDir, time.Now())

		assert.ErrorContains(t, err, "list live import sessions")
	})
	t.Run("supersede", func(t *testing.T) {
		t.Parallel()
		db := setupImportTestDB(t)
		require.NoError(t, db.Migrator().DropTable(&models.ImportSession{}))

		err := supersedeUploadSessions(db, t.TempDir(), "new")

		assert.ErrorContains(t, err, "list older import sessions")
	})
}
