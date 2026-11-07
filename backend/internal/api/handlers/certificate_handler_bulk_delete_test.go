package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

const bulkDeletePath = "/api/v1/certificates/bulk-delete"

// bulkTestUUID returns a deterministic canonical UUID for a readable test label.
func bulkTestUUID(label string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("charon-bulk-delete-test:"+label)).String()
}

// setupBulkDeleteRouter mounts BulkDelete behind the same admin gate the production route uses.
func setupBulkDeleteRouter(db *gorm.DB, backup BackupServiceInterface, role string) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Set("userID", uint(1))
		c.Next()
	})
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, backup, nil)
	r.POST(bulkDeletePath, middleware.RequireRole(models.RoleAdmin), h.BulkDelete)
	return r
}

func postBulkDelete(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, bulkDeletePath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func bulkDeleteBody(t *testing.T, uuids ...string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"uuids": uuids})
	require.NoError(t, err)
	return string(b)
}

func createBulkTestCert(t *testing.T, db *gorm.DB, certUUID string) models.SSLCertificate {
	t.Helper()
	cert := models.SSLCertificate{UUID: certUUID, Name: "bulk-" + certUUID, Provider: "custom", Domains: certUUID + ".example.com"}
	require.NoError(t, db.Create(&cert).Error)
	return cert
}

func certExists(t *testing.T, db *gorm.DB, certUUID string) bool {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.SSLCertificate{}).Where("uuid = ?", certUUID).Count(&count).Error)
	return count == 1
}

// countingBackup returns a backup mock that records CreateBackup calls and succeeds.
func countingBackup(calls *int) *mockBackupService {
	return &mockBackupService{
		createFunc: func() (string, error) {
			*calls++
			return "bulk-backup.tar.gz", nil
		},
	}
}

func newBulkDeleteTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := openCertHandlerTestDB(t, name)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))
	return db
}

func TestBulkDeleteCertificates_SuccessTakesSingleBackup(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_success.db")
	uuids := []string{bulkTestUUID("a"), bulkTestUUID("b"), bulkTestUUID("c")}
	for _, u := range uuids {
		createBulkTestCert(t, db, u)
	}

	backupCalls := 0
	r := setupBulkDeleteRouter(db, countingBackup(&backupCalls), string(models.RoleAdmin))
	w := postBulkDelete(r, bulkDeleteBody(t, uuids...))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, backupCalls, "exactly one backup must be taken for the whole batch")
	assert.Contains(t, w.Body.String(), `"failed":[]`, "failed must be an empty array, not null")

	var resp bulkDeleteCertificatesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.ElementsMatch(t, uuids, resp.Deleted)
	assert.Empty(t, resp.Failed)
	for _, u := range uuids {
		assert.False(t, certExists(t, db, u), "certificate %s should be deleted", u)
	}
}

func TestBulkDeleteCertificates_BackupFailureDeletesNothing(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_backup_fail.db")
	a, b := bulkTestUUID("fail-a"), bulkTestUUID("fail-b")
	createBulkTestCert(t, db, a)
	createBulkTestCert(t, db, b)

	backup := &mockBackupService{
		createFunc: func() (string, error) { return "", fmt.Errorf("disk exploded") },
	}
	r := setupBulkDeleteRouter(db, backup, string(models.RoleAdmin))
	w := postBulkDelete(r, bulkDeleteBody(t, a, b))

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.JSONEq(t, `{"error":"failed to create backup before deletion"}`, w.Body.String())
	assert.True(t, certExists(t, db, a))
	assert.True(t, certExists(t, db, b))
}

func TestBulkDeleteCertificates_LowDiskSpaceReturns507(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_low_disk.db")
	a := bulkTestUUID("disk-a")
	createBulkTestCert(t, db, a)

	backupCalls := 0
	backup := countingBackup(&backupCalls)
	backup.availableSpaceFunc = func() (int64, error) { return 50 * 1024 * 1024, nil }

	r := setupBulkDeleteRouter(db, backup, string(models.RoleAdmin))
	w := postBulkDelete(r, bulkDeleteBody(t, a))

	require.Equal(t, http.StatusInsufficientStorage, w.Code, w.Body.String())
	assert.JSONEq(t, `{"error":"insufficient disk space for backup"}`, w.Body.String())
	assert.Equal(t, 0, backupCalls)
	assert.True(t, certExists(t, db, a))
}

func TestBulkDeleteCertificates_MixedOutcomes(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_mixed.db")
	okUUID, inUseUUID, missingUUID := bulkTestUUID("ok"), bulkTestUUID("in-use"), bulkTestUUID("missing")
	createBulkTestCert(t, db, okUUID)
	inUse := createBulkTestCert(t, db, inUseUUID)
	ph := models.ProxyHost{UUID: "ph-bulk-mixed", Name: "ph", DomainNames: "mixed.example.com", ForwardHost: "localhost", ForwardPort: 8080, CertificateID: &inUse.ID}
	require.NoError(t, db.Create(&ph).Error)

	backupCalls := 0
	r := setupBulkDeleteRouter(db, countingBackup(&backupCalls), string(models.RoleAdmin))
	w := postBulkDelete(r, bulkDeleteBody(t, okUUID, inUseUUID, missingUUID))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, backupCalls)

	var resp bulkDeleteCertificatesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []string{okUUID}, resp.Deleted)
	assert.ElementsMatch(t, []bulkDeleteCertificateFailure{
		{UUID: inUseUUID, Reason: "in_use"},
		{UUID: missingUUID, Reason: "not_found"},
	}, resp.Failed)

	assert.False(t, certExists(t, db, okUUID))
	assert.True(t, certExists(t, db, inUseUUID), "in-use certificate must be kept")
}

func TestBulkDeleteCertificates_UsageCheckErrorReportsGenericError(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_usage_error.db")
	a, b := bulkTestUUID("err-a"), bulkTestUUID("err-b")
	createBulkTestCert(t, db, a)
	createBulkTestCert(t, db, b)
	// Dropping the linkage table makes every usage check fail after the cert lookup succeeds.
	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))

	backupCalls := 0
	r := setupBulkDeleteRouter(db, countingBackup(&backupCalls), string(models.RoleAdmin))
	w := postBulkDelete(r, bulkDeleteBody(t, a, b))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, backupCalls)
	assert.NotContains(t, w.Body.String(), "no such table", "raw error text must not leak")

	var resp bulkDeleteCertificatesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Deleted)
	assert.ElementsMatch(t, []bulkDeleteCertificateFailure{
		{UUID: a, Reason: "error"},
		{UUID: b, Reason: "error"},
	}, resp.Failed)
}

func TestBulkDeleteCertificates_InvalidRequestsReturn400(t *testing.T) {
	tooMany := make([]string, maxBulkDeleteCertificates+1)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}

	tests := []struct {
		name     string
		body     string
		wantBody string
	}{
		{name: "malformed JSON", body: `{"uuids":`, wantBody: `{"error":"invalid request"}`},
		{name: "wrong type", body: `{"uuids":"not-a-list"}`, wantBody: `{"error":"invalid request"}`},
		{name: "empty list", body: `{"uuids":[]}`, wantBody: `{"error":"uuids must contain between 1 and 200 entries"}`},
		{name: "missing field", body: `{}`, wantBody: `{"error":"uuids must contain between 1 and 200 entries"}`},
		{name: "too many", body: bulkDeleteBody(t, tooMany...), wantBody: `{"error":"uuids must contain between 1 and 200 entries"}`},
		{name: "invalid uuid", body: `{"uuids":["not-a-uuid"]}`, wantBody: `{"error":"uuids must contain valid UUIDs"}`},
		{name: "numeric id rejected", body: `{"uuids":["12"]}`, wantBody: `{"error":"uuids must contain valid UUIDs"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newBulkDeleteTestDB(t, "bulk_invalid_"+strings.ReplaceAll(tt.name, " ", "_")+".db")
			backupCalls := 0
			r := setupBulkDeleteRouter(db, countingBackup(&backupCalls), string(models.RoleAdmin))
			w := postBulkDelete(r, tt.body)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.JSONEq(t, tt.wantBody, w.Body.String())
			assert.Equal(t, 0, backupCalls)
		})
	}
}

func TestBulkDeleteCertificates_DedupesCanonicalUUIDs(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_dedupe.db")
	canonical := uuid.NewString()
	createBulkTestCert(t, db, canonical)

	backupCalls := 0
	r := setupBulkDeleteRouter(db, countingBackup(&backupCalls), string(models.RoleAdmin))
	// The same UUID in uppercase and repeated forms must be processed once, reported canonically.
	w := postBulkDelete(r, bulkDeleteBody(t, canonical, strings.ToUpper(canonical), canonical))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, backupCalls)

	var resp bulkDeleteCertificatesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []string{canonical}, resp.Deleted)
	assert.Empty(t, resp.Failed)
}

func TestBulkDeleteCertificates_NilBackupServiceSkipsBackup(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_nil_backup.db")
	a := bulkTestUUID("nil-a")
	createBulkTestCert(t, db, a)

	r := setupBulkDeleteRouter(db, nil, string(models.RoleAdmin))
	w := postBulkDelete(r, bulkDeleteBody(t, a))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.False(t, certExists(t, db, a))
}

func TestBulkDeleteCertificates_RoleGating(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_role.db")
	a := bulkTestUUID("role-a")
	createBulkTestCert(t, db, a)

	backupCalls := 0
	r := setupBulkDeleteRouter(db, countingBackup(&backupCalls), string(models.RoleUser))
	w := postBulkDelete(r, bulkDeleteBody(t, a))

	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, 0, backupCalls)
	assert.True(t, certExists(t, db, a))
}

func TestBulkDeleteCertificates_BindErrorReturnsGenericMessage(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_bind_leak.db")
	r := setupBulkDeleteRouter(db, nil, string(models.RoleAdmin))
	w := postBulkDelete(r, "xxxx")

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"invalid request"}`, w.Body.String())
}

func TestBulkDeleteCertificates_OversizedBodyReturnsGenericBadRequest(t *testing.T) {
	db := newBulkDeleteTestDB(t, "bulk_oversized.db")
	a := bulkTestUUID("oversized-a")
	createBulkTestCert(t, db, a)

	// A body past the 64 KiB cap is cut off by MaxBytesReader and must fail binding with the generic message.
	oversized := `{"uuids":["` + strings.Repeat("a", maxBulkDeleteBodyBytes) + `"]}`
	backupCalls := 0
	r := setupBulkDeleteRouter(db, countingBackup(&backupCalls), string(models.RoleAdmin))
	w := postBulkDelete(r, oversized)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.JSONEq(t, `{"error":"invalid request"}`, w.Body.String())
	assert.Equal(t, 0, backupCalls)
	assert.True(t, certExists(t, db, a))
}
