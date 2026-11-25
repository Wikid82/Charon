package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

func TestProxyHostHandler_BulkUpdateACL_LookupFailureIsInternal(t *testing.T) {
	router, db := setupTestRouter(t)
	host := &models.ProxyHost{
		UUID: uuid.NewString(), Name: "Host", DomainNames: "acl-lookup.example.com",
		ForwardScheme: "http", ForwardHost: "localhost", ForwardPort: 8011, Enabled: true,
	}
	require.NoError(t, db.Create(host).Error)
	require.NoError(t, db.Migrator().DropTable(&models.AccessList{}))

	body := fmt.Sprintf(`{"host_uuids":["%s"],"access_list_id":1}`, host.UUID)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestProxyHostHandler_BulkUpdateACL_WriteFailureIsReported(t *testing.T) {
	router, db := setupTestRouter(t)
	list := models.AccessList{UUID: uuid.NewString(), Name: "office", Enabled: true}
	require.NoError(t, db.Create(&list).Error)
	host := &models.ProxyHost{
		UUID: uuid.NewString(), Name: "Host", DomainNames: "acl-write.example.com",
		ForwardScheme: "http", ForwardHost: "localhost", ForwardPort: 8012, Enabled: true,
	}
	require.NoError(t, db.Create(host).Error)
	require.NoError(t, db.Exec("CREATE TRIGGER refuse_acl_write BEFORE UPDATE ON proxy_hosts BEGIN SELECT RAISE(ABORT, 'refused by test'); END;").Error)

	body := fmt.Sprintf(`{"host_uuids":["%s"],"access_list_id":%d}`, host.UUID, list.ID)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), `"updated":0`)
	assert.Contains(t, resp.Body.String(), "refused by test")
}

func TestDeleteProfile_StorageFailureIsInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.OpenMemoryDBWithForeignKeys(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityHeaderProfile{}, &models.ProxyHost{}, &models.RedirectionHost{}))
	profile := models.SecurityHeaderProfile{UUID: uuid.New().String(), Name: "Refused"}
	require.NoError(t, db.Create(&profile).Error)
	require.NoError(t, db.Exec("CREATE TRIGGER refuse_profile_delete BEFORE DELETE ON security_header_profiles BEGIN SELECT RAISE(ABORT, 'refused by test'); END;").Error)

	router := gin.New()
	registerSecurityHeadersRoutesForTest(router.Group("/"), NewSecurityHeadersHandler(db, nil))
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/security/headers/profiles/%d", profile.ID), http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Could not delete the profile.")
	assert.NotContains(t, w.Body.String(), "refused by test", "the database error must not reach the client")
}

func TestBackupHandler_Delete_RecordRemovalFailureStillSucceeds(t *testing.T) {
	router, svc, db, tmpDir := setupBackupTestWithDB(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename, err := svc.CreateBackup()
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TRIGGER refuse_record_delete BEFORE DELETE ON backup_records BEGIN SELECT RAISE(ABORT, 'refused by test'); END;").Error)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/backups/"+filename, http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code, "the file is already gone, so the request succeeds: %s", resp.Body.String())
	var remaining int64
	require.NoError(t, db.Model(&models.BackupRecord{}).Where("filename = ?", filename).Count(&remaining).Error)
	assert.Equal(t, int64(1), remaining, "the record survives a refused removal")
}
