package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// TestBackupHandler_GetJob_NotFound_Returns404 is the required §3.2.3
// coverage for an unknown job_id.
func TestBackupHandler_GetJob_NotFound_Returns404(t *testing.T) {
	router, _, _, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/jobs/does-not-exist", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "backup job not found", body["error"])
}

// TestBackupHandler_GetJob_FailedRestoreUnrecoverable_ReturnsErrorCode
// replaces the pre-async-job TestBackupHandler_RespondRestoreError_
// Unrecoverable_Returns500WithErrorCode: the C1 double-failure sentinel
// (services.ErrRestoreUnrecoverable) is no longer mapped to an HTTP
// response synchronously by a handler — it's classified by
// backupErrorCode onto the failed BackupJob row instead (spec §3.3.1/
// §3.7), polled via GetJob. This proves that classification survives the
// full round trip through the HTTP response body.
func TestBackupHandler_GetJob_FailedRestoreUnrecoverable_ReturnsErrorCode(t *testing.T) {
	router, _, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	job := &models.BackupJob{
		Type:         "restore",
		Status:       "failed",
		ErrorMessage: `restore could not be completed and no recovery mechanism succeeded: live database rehydrate failed (disable foreign keys: sql: database is closed) and the durable pending-restore fallback also failed (create pending-restore file: is a directory); a pre-restore safety backup "backup_2026-07-14_12-00-00.zip" was created before this attempt and can be restored manually`,
		ErrorCode:    "backup_restore_unrecoverable",
	}
	require.NoError(t, db.Create(job).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/jobs/"+job.UUID, http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body backupJobResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "failed", body.Status)
	require.NotNil(t, body.Error)
	assert.Equal(t, "backup_restore_unrecoverable", body.Error.ErrorCode)
	assert.Contains(t, body.Error.Message, "backup_2026-07-14_12-00-00.zip")
	assert.Nil(t, body.Result)
}

// TestBackupHandler_GetJob_CompletedCreate_ReturnsFilenameAndUUID proves
// the "result" shape for a completed create job mirrors today's old 201
// create response body, for frontend continuity (spec §3.2.3).
func TestBackupHandler_GetJob_CompletedCreate_ReturnsFilenameAndUUID(t *testing.T) {
	router, _, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	job := &models.BackupJob{
		Type:       "create",
		Status:     "completed",
		Filename:   "backup_2026-07-16_100230-a1b2c3d4.zip",
		ResultUUID: "11111111-1111-1111-1111-111111111111",
	}
	require.NoError(t, db.Create(job).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/jobs/"+job.UUID, http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "completed", body["status"])
	result, ok := body["result"].(map[string]any)
	require.True(t, ok, "result must be an object for a completed create job")
	assert.Equal(t, "backup_2026-07-16_100230-a1b2c3d4.zip", result["filename"])
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", result["uuid"])
}

// TestBackupHandler_GetJob_CompletedRestore_ReturnsRestoreResult proves the
// "result" shape for a completed restore job unmarshals ResultJSON into the
// same services.RestoreResult fields the old synchronous 200 body had
// (spec §3.2.3).
func TestBackupHandler_GetJob_CompletedRestore_ReturnsRestoreResult(t *testing.T) {
	router, _, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	resultJSON, err := json.Marshal(services.RestoreResult{
		Message:              "Backup restored successfully",
		RestartRequired:      false,
		DatabaseSwapPending:  false,
		LiveRehydrateApplied: true,
		CaddyReloaded:        true,
		PreRestoreBackup:     "backup_2026-07-16_095900-f00dcafe.zip",
		LegacyFormat:         false,
	})
	require.NoError(t, err)

	job := &models.BackupJob{
		Type:       "restore",
		Status:     "completed",
		Filename:   "backup_2026-07-16_090000-deadbeef.zip",
		ResultJSON: string(resultJSON),
	}
	require.NoError(t, db.Create(job).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/jobs/"+job.UUID, http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	result, ok := body["result"].(map[string]any)
	require.True(t, ok, "result must be an object for a completed restore job")
	assert.Equal(t, "Backup restored successfully", result["message"])
	assert.Equal(t, true, result["live_rehydrate_applied"])
	assert.Equal(t, "backup_2026-07-16_095900-f00dcafe.zip", result["pre_restore_backup"])
}

// TestBackupHandler_GetJob_PendingJob_OmitsResultAndError proves an
// in-flight job's response has neither "result" nor "error" populated
// (spec §3.2.3's "populated only when terminal" contract).
func TestBackupHandler_GetJob_PendingJob_OmitsResultAndError(t *testing.T) {
	router, _, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	job := &models.BackupJob{Type: "create", Status: "running", Stage: "archiving_files"}
	require.NoError(t, db.Create(job).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/jobs/"+job.UUID, http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body backupJobResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "running", body.Status)
	assert.Equal(t, "archiving_files", body.Stage)
	assert.Nil(t, body.Result)
	assert.Nil(t, body.Error)
}

// TestBackupHandler_GetJob_RequiresAdmin mirrors the requireAdmin guard
// pattern already used across this handler's other endpoints.
func TestBackupHandler_GetJob_RequiresAdmin(t *testing.T) {
	svc := services.NewBackupService(&config.Config{DatabasePath: filepath.Join(t.TempDir(), "data", "charon.db")}, nil, nil)
	defer svc.Stop()
	h := NewBackupHandler(svc)

	nonAdminRouter := gin.New()
	nonAdminRouter.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("userID", uint(1))
		c.Next()
	})
	nonAdminRouter.GET("/api/v1/backups/jobs/:job_id", h.GetJob)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/jobs/whatever", http.NoBody)
	resp := httptest.NewRecorder()
	nonAdminRouter.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)
}
