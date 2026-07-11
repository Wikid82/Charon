package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// --- respondCreateError / respondRestoreError: direct unit tests of every
// switch branch, mirroring the existing
// TestBackupHandler_RespondCreateError_ConcurrentInProgress_Returns409
// pattern already in backup_handler_test.go. Exercising these directly with
// sentinel errors is far cheaper and more deterministic than trying to
// force a real disk-full condition or manufacture every validation failure
// through the full HTTP stack.

func TestBackupHandler_RespondCreateError_InsufficientSpace_Returns500WithCode(t *testing.T) {
	h := NewBackupHandler(&services.BackupService{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/backups", http.NoBody)

	h.respondCreateError(c, services.ErrInsufficientSpace)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "backup_insufficient_space", body["error_code"])
}

func TestBackupHandler_RespondRestoreError_AllBranches(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"passphrase required", services.ErrPassphraseRequired, http.StatusBadRequest, "backup_passphrase_required"},
		{"passphrase invalid", services.ErrPassphraseInvalid, http.StatusBadRequest, "backup_passphrase_invalid"},
		{"newer backup format", services.ErrNewerBackupFormat, http.StatusBadRequest, "backup_validation_failed"},
		{"validation failed", services.ErrBackupValidationFailed, http.StatusBadRequest, "backup_validation_failed"},
		{"not found", os.ErrNotExist, http.StatusNotFound, ""},
		{"generic default", assertBoomError{}, http.StatusInternalServerError, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewBackupHandler(&services.BackupService{})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/backups/x/restore", http.NoBody)

			h.respondRestoreError(c, tt.err)
			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantCode != "" {
				var body map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, tt.wantCode, body["error_code"])
			}
		})
	}
}

type assertBoomError struct{}

func (assertBoomError) Error() string { return "boom: some unmapped internal error" }

// --- List / Delete / Upload with a real (non-nil) db, exercising the
// h.db != nil branches these three handlers otherwise never reach in the
// nil-db setupBackupTest fixture.

func setupBackupTestWithDB(t *testing.T) (*gin.Engine, *services.BackupService, *gorm.DB, string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "cpm-backup-db-test")
	require.NoError(t, err)

	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))

	dbPath := filepath.Join(dataDir, "charon.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, err = rawDB.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, email TEXT)")
	require.NoError(t, err)
	_, err = rawDB.Exec("CREATE TABLE IF NOT EXISTS proxy_hosts (id INTEGER PRIMARY KEY, domain_names TEXT)")
	require.NoError(t, err)
	require.NoError(t, rawDB.Close())

	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.BackupRecord{}, &models.BackupRemoteCopy{}, &models.RemoteStorageTarget{}, &models.Setting{}))

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewBackupService(cfg, gdb, nil)
	t.Cleanup(svc.Stop)

	h := NewBackupHandlerWithDeps(svc, nil, gdb)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	api := r.Group("/api/v1")
	backups := api.Group("/backups")
	backups.GET("", h.List)
	backups.POST("", h.Create)
	backups.POST("/upload", h.Upload)
	backups.POST("/:filename/restore", h.Restore)
	backups.POST("/:filename/validate", h.Validate)
	backups.DELETE("/:filename", h.Delete)
	backups.GET("/:filename/download", h.Download)

	return r, svc, gdb, tmpDir
}

func TestBackupHandler_List_WithDBRecord_PopulatesFieldsAndRemoteCopies(t *testing.T) {
	router, svc, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename, err := svc.CreateBackup()
	require.NoError(t, err)

	target := models.RemoteStorageTarget{Name: "NAS", Type: "sftp", ConfigJSON: "{}"}
	require.NoError(t, db.Create(&target).Error)

	// svc.CreateBackup() already persisted a BackupRecord row for filename
	// (CreateBackupWithOptions writes one whenever a db is attached) — fetch
	// and update it in place rather than inserting a second, colliding row.
	var record models.BackupRecord
	require.NoError(t, db.Where("filename = ?", filename).First(&record).Error)
	require.NoError(t, db.Model(&record).Updates(map[string]any{
		"type":           "manual",
		"status":         "completed",
		"format_version": 2,
		"sha256":         "deadbeef",
		"app_version":    "1.2.3",
	}).Error)

	remoteCopy := models.BackupRemoteCopy{BackupRecordID: record.ID, RemoteTargetID: target.ID, Status: "uploaded"}
	require.NoError(t, db.Create(&remoteCopy).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var entries []map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &entries))
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, filename, entry["filename"])
	assert.Equal(t, "manual", entry["type"])
	assert.Equal(t, float64(2), entry["format_version"])
	assert.Equal(t, "deadbeef", entry["sha256"])
	assert.Equal(t, "completed", entry["status"])
	assert.Equal(t, "1.2.3", entry["app_version"])

	remoteCopies, ok := entry["remote_copies"].([]any)
	require.True(t, ok, "remote_copies must be present when a BackupRecord has RemoteCopies")
	require.Len(t, remoteCopies, 1)
	firstCopy := remoteCopies[0].(map[string]any)
	assert.Equal(t, "uploaded", firstCopy["status"])
	assert.Equal(t, "NAS", firstCopy["target_name"])
}

func TestBackupHandler_List_RecordWithMissingDiskFile_MarkedDeleted(t *testing.T) {
	router, _, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// A record whose file never existed on disk (e.g. deleted out-of-band).
	record := models.BackupRecord{Filename: "backup_ghost.zip", Type: "manual", Status: "completed"}
	require.NoError(t, db.Create(&record).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var reloaded models.BackupRecord
	require.NoError(t, db.First(&reloaded, record.ID).Error)
	assert.Equal(t, "deleted", reloaded.Status, "a record whose file is missing from disk must be marked deleted")
}

func TestBackupHandler_Delete_WithDBRecord_CleansUpRelatedRows(t *testing.T) {
	router, svc, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename, err := svc.CreateBackup()
	require.NoError(t, err)

	target := models.RemoteStorageTarget{Name: "NAS", Type: "sftp", ConfigJSON: "{}"}
	require.NoError(t, db.Create(&target).Error)

	// svc.CreateBackup() already persisted a BackupRecord row for filename;
	// reuse it rather than inserting a colliding duplicate.
	var record models.BackupRecord
	require.NoError(t, db.Where("filename = ?", filename).First(&record).Error)

	remoteCopy := models.BackupRemoteCopy{BackupRecordID: record.ID, RemoteTargetID: target.ID, Status: "uploaded"}
	require.NoError(t, db.Create(&remoteCopy).Error)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/backups/"+filename, http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var recordCount, copyCount int64
	db.Model(&models.BackupRecord{}).Where("filename = ?", filename).Count(&recordCount)
	db.Model(&models.BackupRemoteCopy{}).Where("backup_record_id = ?", record.ID).Count(&copyCount)
	assert.Equal(t, int64(0), recordCount, "the BackupRecord row must be deleted alongside the file")
	assert.Equal(t, int64(0), copyCount, "BackupRemoteCopy rows must be deleted alongside their BackupRecord")
}

func TestBackupHandler_Upload_WithDB_PersistsBackupRecord(t *testing.T) {
	router, svc, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dbPath := filepath.Join(t.TempDir(), "upload.db")
	createValidSQLiteDBWithCharonTables(t, dbPath)
	content, err := os.ReadFile(dbPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	body, contentType := buildMultipartUpload(t, "upload.db", content, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	filename, _ := respBody["filename"].(string)
	require.NotEmpty(t, filename)

	var record models.BackupRecord
	require.NoError(t, db.Where("filename = ?", filename).First(&record).Error)
	assert.Equal(t, "uploaded", record.Type)
	assert.NotEmpty(t, record.SHA256)
	_ = svc
}

// --- Download requireAdmin guard ---

func TestBackupHandler_Download_RequiresAdmin(t *testing.T) {
	router, _, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	nonAdminRouter := gin.New()
	nonAdminRouter.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("userID", uint(1))
		c.Next()
	})
	svc := services.NewBackupService(&config.Config{DatabasePath: filepath.Join(t.TempDir(), "data", "charon.db")}, nil, nil)
	defer svc.Stop()
	h := NewBackupHandler(svc)
	nonAdminRouter.GET("/api/v1/backups/:filename/download", h.Download)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/whatever.zip/download", http.NoBody)
	resp := httptest.NewRecorder()
	nonAdminRouter.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)
	_ = router
}

// --- Upload requireAdmin guard ---

func TestBackupHandler_Upload_RequiresAdmin(t *testing.T) {
	svc := services.NewBackupService(&config.Config{DatabasePath: filepath.Join(t.TempDir(), "data", "charon.db")}, nil, nil)
	defer svc.Stop()
	h := NewBackupHandler(svc)

	nonAdminRouter := gin.New()
	nonAdminRouter.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("userID", uint(1))
		c.Next()
	})
	nonAdminRouter.POST("/api/v1/backups/upload", h.Upload)

	body, contentType := buildMultipartUpload(t, "upload.db", []byte("irrelevant"), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	nonAdminRouter.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)
}

// --- UpdateSettings requireAdmin guard + default error branch ---

func TestBackupHandler_UpdateSettings_RequiresAdmin(t *testing.T) {
	svc := services.NewBackupService(&config.Config{DatabasePath: filepath.Join(t.TempDir(), "data", "charon.db")}, nil, nil)
	defer svc.Stop()
	h := NewBackupHandler(svc)

	nonAdminRouter := gin.New()
	nonAdminRouter.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("userID", uint(1))
		c.Next()
	})
	nonAdminRouter.PUT("/api/v1/backups/settings", h.UpdateSettings)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/settings", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	nonAdminRouter.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)
}

func TestBackupHandler_UpdateSettings_GenericError_Returns500(t *testing.T) {
	// A BackupService constructed with a nil db makes UpdateSettings return
	// a generic ("backup settings require a database connection") error
	// that matches none of the three known sentinel errors, exercising the
	// handler's default 500 branch.
	svc := services.NewBackupService(&config.Config{DatabasePath: filepath.Join(t.TempDir(), "data", "charon.db")}, nil, nil)
	defer svc.Stop()
	h := NewBackupHandler(svc)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.PUT("/api/v1/backups/settings", h.UpdateSettings)

	payload := map[string]any{"retention_count": 5}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusInternalServerError, resp.Code)
}

// --- Validate: passphrase-required / passphrase-invalid / default
// validation-failed branches, exercised through real encrypted/corrupt
// archives rather than mocks.

func TestBackupHandler_Validate_PassphraseRequiredAndInvalid(t *testing.T) {
	router, _, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/backups", bytes.NewReader(mustJSON(t, map[string]any{
		"encrypt":    true,
		"passphrase": "correct-horse-battery-staple",
	})))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())

	var created map[string]string
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &created))
	filename := created["filename"]
	require.Contains(t, filename, ".zip.age")

	// No passphrase supplied.
	noPassReq := httptest.NewRequest(http.MethodPost, "/api/v1/backups/"+filename+"/validate", http.NoBody)
	noPassResp := httptest.NewRecorder()
	router.ServeHTTP(noPassResp, noPassReq)
	require.Equal(t, http.StatusBadRequest, noPassResp.Code)
	var noPassBody map[string]any
	require.NoError(t, json.Unmarshal(noPassResp.Body.Bytes(), &noPassBody))
	assert.Equal(t, "backup_passphrase_required", noPassBody["error_code"])

	// Wrong passphrase.
	wrongPassReq := httptest.NewRequest(http.MethodPost, "/api/v1/backups/"+filename+"/validate",
		bytes.NewReader(mustJSON(t, map[string]any{"passphrase": "wrong-passphrase"})))
	wrongPassReq.Header.Set("Content-Type", "application/json")
	wrongPassResp := httptest.NewRecorder()
	router.ServeHTTP(wrongPassResp, wrongPassReq)
	require.Equal(t, http.StatusBadRequest, wrongPassResp.Code)
	var wrongPassBody map[string]any
	require.NoError(t, json.Unmarshal(wrongPassResp.Body.Bytes(), &wrongPassBody))
	assert.Equal(t, "backup_passphrase_invalid", wrongPassBody["error_code"])
}

func TestBackupHandler_Validate_DefaultValidationFailed(t *testing.T) {
	router, svc, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// A file with a valid zip magic header but garbage contents fails at
	// the manifest/extraction stage, hitting the switch's default branch.
	filename := "corrupt_backup.zip"
	require.NoError(t, os.WriteFile(filepath.Join(svc.BackupDir, filename), []byte("PK\x03\x04not a real zip"), 0o600))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/"+filename+"/validate", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "backup_validation_failed", body["error_code"])
	assert.Equal(t, false, body["valid"])
}

// --- Upload: zip and age kinds (only the raw-sqlite kind had a test
// before), plus the WrapRawDatabaseAsBackup failure branch and the
// post-write validation failure branches.

func TestBackupHandler_Upload_ZipKind_Success(t *testing.T) {
	router, svc, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename, err := svc.CreateBackup()
	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(svc.BackupDir, filename)) // #nosec G304 -- test-controlled path
	require.NoError(t, err)

	body, contentType := buildMultipartUpload(t, "whatever-name.zip", content, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	uploadedFilename, _ := respBody["filename"].(string)
	assert.Contains(t, uploadedFilename, "uploaded_")
	assert.Contains(t, uploadedFilename, ".zip")
	assert.NotContains(t, uploadedFilename, "whatever-name", "client-supplied filenames must be discarded entirely")
}

func TestBackupHandler_Upload_AgeKind_Success(t *testing.T) {
	router, svc, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	record, err := svc.CreateBackupWithOptions(services.BackupOptions{Type: "manual", Encrypt: true, Passphrase: "upload-test-passphrase"})
	require.NoError(t, err)
	require.Contains(t, record.Filename, ".zip.age")

	content, err := os.ReadFile(filepath.Join(svc.BackupDir, record.Filename)) // #nosec G304 -- test-controlled path
	require.NoError(t, err)

	body, contentType := buildMultipartUpload(t, "whatever.zip.age", content, map[string]string{"passphrase": "upload-test-passphrase"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	uploadedFilename, _ := respBody["filename"].(string)
	assert.Contains(t, uploadedFilename, ".zip.age")
	assert.Equal(t, false, respBody["legacy_format"], "a format-v2 archive with a manifest is never legacy")
}

func TestBackupHandler_Upload_WrapRawDatabaseFailure(t *testing.T) {
	router, svc, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dbPath := filepath.Join(t.TempDir(), "upload.db")
	createValidSQLiteDBWithCharonTables(t, dbPath)
	content, err := os.ReadFile(dbPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	require.NoError(t, os.Chmod(svc.BackupDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(svc.BackupDir, 0o700) })

	body, contentType := buildMultipartUpload(t, "upload.db", content, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	assert.Contains(t, respBody["error"], "failed to wrap uploaded database")
}

func TestBackupHandler_Upload_ZipKind_ValidationFailure_DeletesFile(t *testing.T) {
	router, svc, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Valid zip magic bytes but not a real/parseable archive: passes magic
	// detection, fails ValidateBackup, hitting the default branch and
	// exercising the "delete the just-written upload" cleanup call.
	body, contentType := buildMultipartUpload(t, "corrupt.zip", []byte("PK\x03\x04garbage-not-a-real-zip"), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	assert.Equal(t, "backup_validation_failed", respBody["error_code"])

	entries, err := os.ReadDir(svc.BackupDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "a backup that fails post-write validation must be deleted, not left behind")
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

// TestBackupHandler_Upload_ZipKind_WriteFileFailure proves the zip/age
// branch's os.WriteFile failure (BackupDir made read-only) surfaces a
// wrapped "failed to store uploaded file" 500, mirroring
// TestBackupHandler_Upload_WrapRawDatabaseFailure for the raw-sqlite kind.
func TestBackupHandler_Upload_ZipKind_WriteFileFailure(t *testing.T) {
	router, svc, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename, err := svc.CreateBackup()
	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(svc.BackupDir, filename)) // #nosec G304 -- test-controlled path
	require.NoError(t, err)

	require.NoError(t, os.Chmod(svc.BackupDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(svc.BackupDir, 0o700) })

	body, contentType := buildMultipartUpload(t, "whatever.zip", content, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusInternalServerError, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	assert.Contains(t, respBody["error"], "failed to store uploaded file")
}
