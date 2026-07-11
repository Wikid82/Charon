package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// TestBackupHandler_Delete_DBWritesFail_StillReturns200WithWarnings proves
// Delete's two best-effort DB cleanup branches (backup_handler.go's
// "failed to delete backup remote copy rows" / "failed to delete backup
// record" warnings): a query_only SQLite connection lets the earlier
// SELECT (finding the record) succeed while every subsequent write fails,
// so both warn branches are hit in the same request, and the endpoint must
// still report success since the on-disk file was already deleted and a
// best-effort DB label mismatch is not a client-facing failure.
func TestBackupHandler_Delete_DBWritesFail_StillReturns200WithWarnings(t *testing.T) {
	router, svc, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename, err := svc.CreateBackup()
	require.NoError(t, err)

	target := models.RemoteStorageTarget{Name: "NAS", Type: "sftp", ConfigJSON: "{}"}
	require.NoError(t, db.Create(&target).Error)

	var record models.BackupRecord
	require.NoError(t, db.Where("filename = ?", filename).First(&record).Error)
	require.NoError(t, db.Create(&models.BackupRemoteCopy{BackupRecordID: record.ID, RemoteTargetID: target.ID, Status: "uploaded"}).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/backups/"+filename, http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code, "a best-effort DB cleanup failure must not fail the request: %s", resp.Body.String())
}

// TestBackupHandler_Upload_RecordPersistFails_StillReturns201 proves
// Upload's own best-effort DB persist branch ("failed to persist uploaded
// backup record"): the archive is still written to disk and validated
// successfully, so the upload itself must still be reported as created
// even though the read-only DB connection can't record it.
func TestBackupHandler_Upload_RecordPersistFails_StillReturns201(t *testing.T) {
	router, svc, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	_ = svc

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)

	dbPath := os.TempDir() + "/charon-upload-record-persist-fails.db"
	createValidSQLiteDBWithCharonTables(t, dbPath)
	content, err := os.ReadFile(dbPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(dbPath) })

	body, contentType := buildMultipartUpload(t, "upload.db", content, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code, "a best-effort backup-record persist failure must not fail the upload response: %s", resp.Body.String())
}

// --- Upload: the passphrase-required / passphrase-invalid switch branches,
// distinct from TestBackupHandler_Upload_ZipKind_ValidationFailure_DeletesFile
// which only exercises the switch's default case. ---

func TestBackupHandler_Upload_AgeKind_MissingPassphrase_ReturnsPassphraseRequired(t *testing.T) {
	router, svc, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	record, err := svc.CreateBackupWithOptions(services.BackupOptions{Type: "manual", Encrypt: true, Passphrase: "correct-passphrase"})
	require.NoError(t, err)
	content, err := os.ReadFile(svc.BackupDir + "/" + record.Filename) // #nosec G304 -- test-controlled path
	require.NoError(t, err)

	body, contentType := buildMultipartUpload(t, "whatever.zip.age", content, nil) // no passphrase field
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "backup_passphrase_required")
}

func TestBackupHandler_Upload_AgeKind_WrongPassphrase_ReturnsPassphraseInvalid(t *testing.T) {
	router, svc, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	record, err := svc.CreateBackupWithOptions(services.BackupOptions{Type: "manual", Encrypt: true, Passphrase: "correct-passphrase"})
	require.NoError(t, err)
	content, err := os.ReadFile(svc.BackupDir + "/" + record.Filename) // #nosec G304 -- test-controlled path
	require.NoError(t, err)

	body, contentType := buildMultipartUpload(t, "whatever.zip.age", content, map[string]string{"passphrase": "wrong-passphrase"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "backup_passphrase_invalid")
}
