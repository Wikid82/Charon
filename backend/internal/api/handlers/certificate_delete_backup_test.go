package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// certDeleteRouter mounts the single-delete handler with the mock auth middleware.
func certDeleteRouter(t *testing.T, backup BackupServiceInterface, db *gorm.DB) *gin.Engine {
	t.Helper()
	svc := services.NewCertificateService(t.TempDir(), db, nil)
	h := NewCertificateHandler(svc, backup, nil)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)
	return r
}

// TestCertificateDelete_SafetyBackupUsesPreDeleteKind checks that a single
// delete takes exactly one safety backup tagged with the pre-delete kind and
// then removes the certificate.
func TestCertificateDelete_SafetyBackupUsesPreDeleteKind(t *testing.T) {
	db := newBulkDeleteTestDB(t, "single_delete_kind.db")
	certUUID := uuid.New().String()
	createBulkTestCert(t, db, certUUID)

	backupCalls := 0
	backup := countingBackup(&backupCalls)
	r := certDeleteRouter(t, backup, db)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+certUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, backupCalls)
	assert.Equal(t, "pre_delete", backup.lastKind)
	assert.False(t, certExists(t, db, certUUID))
}

// TestCertificateDelete_SafetyBackupFailureKeepsCertificate checks that when the
// safety backup fails the delete is refused and the certificate is kept.
func TestCertificateDelete_SafetyBackupFailureKeepsCertificate(t *testing.T) {
	db := newBulkDeleteTestDB(t, "single_delete_backup_fail.db")
	certUUID := uuid.New().String()
	createBulkTestCert(t, db, certUUID)

	backup := &mockBackupService{
		createFunc: func() (string, error) { return "", assert.AnError },
	}
	r := certDeleteRouter(t, backup, db)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+certUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.JSONEq(t, `{"error":"failed to create backup before deletion"}`, w.Body.String())
	assert.Equal(t, "pre_delete", backup.lastKind)
	assert.True(t, certExists(t, db, certUUID))
}

// TestBulkDeleteCertificates_SafetyBackupUsesPreDeleteKind checks that the single
// bulk safety backup carries the same pre-delete kind.
func TestBulkDeleteCertificates_SafetyBackupUsesPreDeleteKind(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_delete_kind.db")
	uuids := []string{bulkTestUUID("kind-a"), bulkTestUUID("kind-b")}
	for _, u := range uuids {
		createBulkTestCert(t, db, u)
	}

	backupCalls := 0
	backup := countingBackup(&backupCalls)
	r := setupBulkDeleteRouter(db, backup, string(models.RoleAdmin))
	w := postBulkDelete(r, bulkDeleteBody(t, uuids...))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, backupCalls)
	assert.Equal(t, "pre_delete", backup.lastKind)
}
