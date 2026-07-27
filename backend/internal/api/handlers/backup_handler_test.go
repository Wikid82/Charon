package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// createJobResponse mirrors the 202 body Create/Restore now return (spec
// §3.2.1/§3.2.2 — Async Backup/Restore Jobs).
type createJobResponse struct {
	JobID  string `json:"job_id"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// createBackupViaRouter POSTs to /api/v1/backups through router, waits for
// the resulting job to finish via svc.WaitForJobs() (test determinism, spec
// §3.3.1), and returns the completed job's filename. Fails the test if the
// job does not reach status "completed".
func createBackupViaRouter(t *testing.T, router *gin.Engine, svc *services.BackupService) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusAccepted, resp.Code, resp.Body.String())

	var started createJobResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &started))
	require.NotEmpty(t, started.JobID)
	require.Equal(t, "create", started.Type)
	require.Equal(t, "pending", started.Status)

	svc.WaitForJobs()

	job, err := svc.GetBackupJob(started.JobID)
	require.NoError(t, err)
	require.Equal(t, "completed", job.Status, "error_message=%q error_code=%q", job.ErrorMessage, job.ErrorCode)
	require.NotEmpty(t, job.Filename)
	return job.Filename
}

// restoreBackupViaRouter POSTs to /api/v1/backups/:filename/restore through
// router, waits for the job to finish, and returns the decoded
// services.RestoreResult (as a generic map, matching how the frontend
// consumes GetJob's polymorphic "result" field) once status is "completed".
func restoreBackupViaRouter(t *testing.T, router *gin.Engine, svc *services.BackupService, filename string) map[string]any {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/"+filename+"/restore", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusAccepted, resp.Code, resp.Body.String())

	var started createJobResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &started))
	require.Equal(t, "restore", started.Type)

	svc.WaitForJobs()

	job, err := svc.GetBackupJob(started.JobID)
	require.NoError(t, err)
	require.Equal(t, "completed", job.Status, "error_message=%q error_code=%q", job.ErrorMessage, job.ErrorCode)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(job.ResultJSON), &result))
	return result
}

// TestBackupHandler_RespondStartJobError_* replace the old
// TestBackupHandler_RespondCreateError_*/RespondRestoreError_* tests —
// respondCreateError/respondRestoreError no longer exist, replaced by the
// single respondStartJobError (spec §3.5) that handles only what
// StartCreateBackupJob/StartRestoreJob can still return synchronously.
func TestBackupHandler_RespondStartJobError_ConcurrentInProgress_Returns409(t *testing.T) {
	h := NewBackupHandler(&services.BackupService{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/backups", http.NoBody)

	h.respondStartJobError(c, services.ErrBackupInProgress)
	require.Equal(t, http.StatusConflict, w.Code)
}

func TestBackupHandler_RespondStartJobError_NotFound_Returns404(t *testing.T) {
	h := NewBackupHandler(&services.BackupService{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/backups/x/restore", http.NoBody)

	h.respondStartJobError(c, services.ErrBackupNotFound)
	require.Equal(t, http.StatusNotFound, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "Backup not found", body["error"])
}

func TestBackupHandler_RespondStartJobError_Default_Returns500(t *testing.T) {
	h := NewBackupHandler(&services.BackupService{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/backups", http.NoBody)

	h.respondStartJobError(c, errors.New("boom: some unmapped internal error"))
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestIsSQLiteTransientRehydrateError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "database is locked", err: errors.New("database is locked"), want: true},
		{name: "database is busy", err: errors.New("database is busy"), want: true},
		{name: "database table is locked", err: errors.New("database table is locked"), want: true},
		{name: "table is locked", err: errors.New("table is locked"), want: true},
		{name: "resource busy", err: errors.New("resource busy"), want: true},
		{name: "mixed-case transient message", err: errors.New("Database Is Locked"), want: true},
		{name: "non-transient error", err: errors.New("constraint failed"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isSQLiteTransientRehydrateError(tt.err))
		})
	}
}

// setupBackupTest builds a full BackupHandler stack backed by a REAL
// database (required since commit 7's async Create/Restore both call
// StartCreateBackupJob/StartRestoreJob, which return an error synchronously
// when s.db == nil — job tracking requires persistence, spec §3.3.1).
func setupBackupTest(t *testing.T) (*gin.Engine, *services.BackupService, string) {
	t.Helper()

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "cpm-backup-test")
	require.NoError(t, err)

	// Structure: tmpDir/data/charon.db
	// BackupService expects DatabasePath to be .../data/charon.db
	// It sets DataDir to filepath.Dir(DatabasePath) -> .../data
	// It sets BackupDir to .../data/backups.

	dataDir := filepath.Join(tmpDir, "data")
	err = os.MkdirAll(dataDir, 0o750)
	require.NoError(t, err)

	dbPath := filepath.Join(dataDir, "charon.db")

	// RestoreBackupSafe's V6 sanity check (spec §3.5) requires the
	// extracted database to look like a Charon database, i.e. contain
	// "users" and "proxy_hosts" tables — created via a raw connection to
	// the same file before it's opened via GORM below.
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.Exec("CREATE TABLE IF NOT EXISTS healthcheck (id INTEGER PRIMARY KEY, value TEXT)").Error)
	require.NoError(t, gdb.Exec("INSERT INTO healthcheck (value) VALUES (?)", "ok").Error)
	require.NoError(t, gdb.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, email TEXT)").Error)
	require.NoError(t, gdb.Exec("CREATE TABLE IF NOT EXISTS proxy_hosts (id INTEGER PRIMARY KEY, domain_names TEXT)").Error)
	// Async job tracking (this plan's §3.1/§3.3.1) requires BackupJob;
	// BackupRecord/BackupRemoteCopy/RemoteStorageTarget are needed for
	// CreateBackupWithOptions's own persistence and List's Preload.
	require.NoError(t, gdb.AutoMigrate(&models.BackupJob{}, &models.BackupRecord{}, &models.BackupRemoteCopy{}, &models.RemoteStorageTarget{}))
	t.Cleanup(func() {
		if sqlDB, dbErr := gdb.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	cfg := &config.Config{
		DatabasePath: dbPath,
	}

	svc := services.NewBackupService(cfg, gdb, nil)
	t.Cleanup(svc.Stop)
	h := NewBackupHandler(svc)

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
	backups.GET("/jobs/:job_id", h.GetJob)
	backups.POST("/:filename/restore", h.Restore)
	backups.DELETE("/:filename", h.Delete)
	backups.GET("/:filename/download", h.Download)

	return r, svc, tmpDir
}

func TestBackupLifecycle(t *testing.T) {
	router, svc, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 1. List backups (should be empty)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// 2. Create backup (async job, spec §3.2.1)
	filename := createBackupViaRouter(t, router, svc)
	require.NotEmpty(t, filename)

	// 3. List backups (should have 1)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// 4. Restore backup (async job, spec §3.2.2)
	restoreResult := restoreBackupViaRouter(t, router, svc, filename)
	require.Contains(t, restoreResult, "restart_required")
	require.Contains(t, restoreResult, "live_rehydrate_applied")
	// RestoreBackupSafe always creates a "pre_restore" safety snapshot (S1)
	// before applying the restore, so it must be cleaned up alongside the
	// original backup below or the list-should-be-empty assertion in step 7
	// would depend on it coincidentally sharing the same second-granularity
	// timestamp as the original backup (see randomFilenameSuffix in
	// backup_service.go, which now makes that collision impossible).
	preRestoreBackup, _ := restoreResult["pre_restore_backup"].(string)
	require.NotEmpty(t, preRestoreBackup)

	// 5. Download backup
	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups/"+filename+"/download", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// 6. Delete backup (and the pre_restore safety snapshot it left behind)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/backups/"+filename, http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/backups/"+preRestoreBackup, http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// 7. List backups (should be empty again)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	var list []any
	_ = json.Unmarshal(resp.Body.Bytes(), &list)
	require.Empty(t, list)

	// 8. Delete non-existent backup
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/backups/missing.zip", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// 9. Restore non-existent backup — stays a synchronous 404 (spec
	// §3.2.2/§3.3.1's ErrBackupNotFound fix), no job created.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/backups/missing.zip/restore", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// 10. Download non-existent backup
	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups/missing.zip/download", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestBackupHandler_Errors(t *testing.T) {
	router, svc, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 1. List Error (remove backup dir to cause ReadDir error)
	// Note: Service now handles missing dir gracefully by returning empty list
	_ = os.RemoveAll(svc.BackupDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	var list []any
	_ = json.Unmarshal(resp.Body.Bytes(), &list)
	require.Empty(t, list)

	// 4. Delete Error (Not Found)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/backups/missing.zip", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestBackupHandler_List_Success(t *testing.T) {
	router, svc, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a backup first
	createBackupViaRouter(t, router, svc)

	// Now list should return it
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var backups []services.BackupFile
	err := json.Unmarshal(resp.Body.Bytes(), &backups)
	require.NoError(t, err)
	require.Len(t, backups, 1)
	require.Contains(t, backups[0].Filename, "backup_")
}

func TestBackupHandler_Create_Success(t *testing.T) {
	router, svc, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename := createBackupViaRouter(t, router, svc)
	require.Contains(t, filename, "backup_")
}

func TestBackupHandler_Create_ReturnsAcceptedImmediately(t *testing.T) {
	router, _, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusAccepted, resp.Code, resp.Body.String())

	var body createJobResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.NotEmpty(t, body.JobID)
	require.Equal(t, "create", body.Type)
	require.Equal(t, "pending", body.Status)
}

func TestBackupHandler_Download_Success(t *testing.T) {
	router, svc, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename := createBackupViaRouter(t, router, svc)

	// Download it
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/"+filename+"/download", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "application")
}

func TestBackupHandler_PathTraversal(t *testing.T) {
	router, _, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Try path traversal in Delete
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/backups/../../../etc/passwd", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// Try path traversal in Download
	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups/../../../etc/passwd/download", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, resp.Code)

	// Try path traversal in Restore
	req = httptest.NewRequest(http.MethodPost, "/api/v1/backups/../../../etc/passwd/restore", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestBackupHandler_Download_InvalidPath(t *testing.T) {
	router, _, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Request with path traversal attempt
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/../invalid/download", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	// Should be BadRequest due to path validation failure
	require.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, resp.Code)
}

func TestBackupHandler_Create_ServiceError(t *testing.T) {
	router, svc, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Remove write permissions on backup dir to force create error
	// #nosec G302 -- Test intentionally uses restrictive perms to simulate error
	_ = os.Chmod(svc.BackupDir, 0o444)
	defer func() {
		// #nosec G302 -- Cleanup restores directory permissions
		_ = os.Chmod(svc.BackupDir, 0o755)
	}()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	// The 202 still fires immediately (the job row was created before the
	// permission error occurs, deeper inside the goroutine) — this is the
	// entire point of the async-job architecture (spec §3.2.1).
	require.Equal(t, http.StatusAccepted, resp.Code, resp.Body.String())

	var body createJobResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))

	svc.WaitForJobs()
	job, err := svc.GetBackupJob(body.JobID)
	require.NoError(t, err)
	require.Equal(t, "failed", job.Status)
}

// TestBackupHandler_Create_UploadToRemoteFalse_DecodesAndSkipsUpload proves
// {"upload_to_remote": false} decodes into a *bool pointing at false (not a
// nil-vs-false ambiguity) and is threaded all the way through
// StartCreateBackupJob/runCreateBackupJob to gate the remote-upload hook
// (spec §12.2.4's handler-test note).
func TestBackupHandler_Create_UploadToRemoteFalse_DecodesAndSkipsUpload(t *testing.T) {
	router, svc, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var called atomic.Bool
	svc.SetRemoteUploadHook(func(ctx context.Context, record *models.BackupRecord) {
		called.Store(true)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups", bytes.NewBufferString(`{"upload_to_remote": false}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusAccepted, resp.Code, resp.Body.String())

	var body createJobResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))

	svc.WaitForJobs()
	job, err := svc.GetBackupJob(body.JobID)
	require.NoError(t, err)
	require.Equal(t, "completed", job.Status, "error_message=%q error_code=%q", job.ErrorMessage, job.ErrorCode)

	assert.False(t, called.Load(), "upload_to_remote:false in the request body must skip the remote-upload hook")
}

func TestBackupHandler_Delete_InternalError(t *testing.T) {
	router, svc, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename := createBackupViaRouter(t, router, svc)

	// Make backup dir read-only to cause delete error (not NotExist)
	// #nosec G302 -- Test intentionally sets restrictive permissions to verify error handling
	_ = os.Chmod(svc.BackupDir, 0o444)
	// #nosec G302 -- Test cleanup restores directory permissions
	defer func() { _ = os.Chmod(svc.BackupDir, 0o755) }()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/backups/"+filename, http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	// Should fail with 500 due to permission error (not 404)
	require.Contains(t, []int{http.StatusInternalServerError, http.StatusOK}, resp.Code)
}

func TestBackupHandler_Restore_InternalError(t *testing.T) {
	router, svc, tmpDir := setupBackupTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename := createBackupViaRouter(t, router, svc)

	// Make data dir read-only to cause restore error
	// #nosec G302 -- Test intentionally sets restrictive permissions to verify error handling
	_ = os.Chmod(svc.DataDir, 0o444)
	// #nosec G302 -- Test cleanup restores directory permissions
	defer func() { _ = os.Chmod(svc.DataDir, 0o755) }()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/"+filename+"/restore", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	// In this fixture, charon.db lives directly under DataDir (mirroring
	// production), so making DataDir read-only also makes the job-tracking
	// db read-only — s.db.Create(&job) itself fails synchronously (the
	// lock-leak-fix path, spec §3.3.1 — independently regression-tested at
	// the service level by
	// TestStartRestoreJob_LockNotLeakedOnPersistenceFailure), a 500 rather
	// than a 202.
	require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
}
