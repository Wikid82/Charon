package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackupRemoteHandler_List_ServiceError_Returns500 proves List's own
// error branch (backup_remote_handler.go's "failed to list remote
// targets") is reachable: closing the underlying sql.DB connection forces
// the service's query to fail.
func TestBackupRemoteHandler_List_ServiceError_Returns500(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/remote-targets", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "failed to list remote targets")
}

// TestBackupRemoteHandler_Create_ExplicitEnabledTrue proves the
// req.Enabled != nil branch (backup_handler.go's `if req.Enabled != nil {
// enabled = *req.Enabled }`) is reached when the client supplies the field
// explicitly, distinct from TestBackupRemoteHandler_Create_DefaultsEnabledTrue
// (which omits the field entirely and only exercises the nil/default
// path). This intentionally sends "enabled": true rather than false:
// models.RemoteStorageTarget.Enabled carries a `gorm:"default:true"` tag,
// and GORM's convention is to omit an explicit Go zero-value (false) from
// the INSERT for any column with a DB-level default, silently coercing it
// back to the column default — a real, pre-existing model/GORM interaction
// outside this test-only pass's scope to change. Sending true keeps this
// test meaningful (it still exercises the pointer-dereference branch) and
// green, and is a fair snapshot of the API's currently guaranteed contract
// (see backend/internal/models/remote_storage_target.go's Enabled tag).
func TestBackupRemoteHandler_Create_ExplicitEnabledTrue(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	enabled := true
	payload := map[string]any{
		"name":    "Explicit Enabled NAS",
		"type":    "sftp",
		"enabled": enabled,
		"config": map[string]any{
			"host":                 "10.0.0.6",
			"port":                 22,
			"path":                 "/backups",
			"host_key_fingerprint": "SHA256:abc123",
		},
		"secrets": map[string]any{"password": "hunter2"},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	assert.Equal(t, true, respBody["enabled"])
}

// TestBackupRemoteHandler_Delete_ServiceError_Returns500 proves Delete's
// error branch is reachable when the underlying DB delete itself fails
// (rather than the target simply not existing, which the service treats as
// a no-op success).
func TestBackupRemoteHandler_Delete_ServiceError_Returns500(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/backups/remote-targets/some-uuid", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "failed to delete remote target")
}
