package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// retentionHandler is an import handler over a migrated database whose importer is the test double.
func retentionHandler(t *testing.T) (*ImportHandler, *mockImporter) {
	t.Helper()
	handler, _, mockImport := setupTestHandler(t, setupImportTestDB(t))
	handler.importerservice = &mockImporterAdapter{mockImport}
	return handler, mockImport
}

func importableHost() *caddy.ImportResult {
	return &caddy.ImportResult{Hosts: []caddy.ParsedHost{{DomainNames: "upload.example.com", ForwardHost: "localhost", ForwardPort: 8080, ForwardScheme: "http"}}}
}

// writeUpload writes an upload file under importDir/uploads and returns its path.
func writeUpload(t *testing.T, importDir, name string) string {
	t.Helper()
	dir := filepath.Join(importDir, "uploads")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("upload.example.com { reverse_proxy localhost:8080 }"), 0o600))
	return path
}

func uploadEntries(t *testing.T, importDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(importDir, "uploads"))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestUploadArtifactOfOnlyNamesEntriesUnderUploads(t *testing.T) {
	t.Parallel()
	importDir := filepath.Join(t.TempDir(), "import")
	cases := []struct {
		name, source string
		want         string
		ok           bool
	}{
		{"single upload", filepath.Join(importDir, "uploads", "s1.caddyfile"), filepath.Join(importDir, "uploads", "s1.caddyfile"), true},
		{"multi-file upload", filepath.Join(importDir, "uploads", "s2", "sites", "a.caddyfile"), filepath.Join(importDir, "uploads", "s2"), true},
		{"mounted file", "/etc/caddy/Caddyfile", "", false},
		{"empty", "", "", false},
		{"uploads itself", filepath.Join(importDir, "uploads"), "", false},
		{"escape", filepath.Join(importDir, "uploads", "..", "x.caddyfile"), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := uploadArtifactOf(importDir, tc.source)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRemoveUploadArtifactLeavesFilesOutsideUploads(t *testing.T) {
	t.Parallel()
	importDir := t.TempDir()
	outside := filepath.Join(importDir, "keep.caddyfile")
	require.NoError(t, os.WriteFile(outside, []byte("x"), 0o600))

	require.NoError(t, removeUploadArtifact(importDir, filepath.Join(importDir, "uploads", "..", "keep.caddyfile")))
	require.NoError(t, removeUploadArtifact(importDir, "/etc/caddy/Caddyfile"))

	_, err := os.Stat(outside)
	assert.NoError(t, err)
}

// The sweep expires stale uploads and their files, removes orphan entries, deletes old closed rows, and never touches
// the mounted Caddyfile's session or committed rows.
func TestSweepImportUploadsAppliesRetention(t *testing.T) {
	t.Parallel()
	db := setupImportTestDB(t)
	importDir := t.TempDir()
	mountPath := filepath.Join(importDir, "mounted.caddyfile")
	require.NoError(t, os.WriteFile(mountPath, []byte("x"), 0o600))
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

	staleFile := writeUpload(t, importDir, "stale.caddyfile")
	freshFile := writeUpload(t, importDir, "fresh.caddyfile")
	orphan := writeUpload(t, importDir, "orphan.caddyfile")
	require.NoError(t, os.Chtimes(orphan, now.Add(-25*time.Hour), now.Add(-25*time.Hour)))

	rows := []models.ImportSession{
		{UUID: "stale", SourceFile: staleFile, Status: "pending"},
		{UUID: "fresh", SourceFile: freshFile, Status: "pending"},
		{UUID: "mounted", SourceFile: mountPath, Status: "pending"},
		{UUID: "closed-old", SourceFile: "", Status: "rejected"},
		{UUID: "committed-old", SourceFile: "", Status: "committed"},
	}
	for i := range rows {
		require.NoError(t, db.Create(&rows[i]).Error)
	}
	setCreated := func(uuid string, at time.Time) {
		require.NoError(t, db.Model(&models.ImportSession{}).Where("uuid = ?", uuid).UpdateColumn("created_at", at).Error)
	}
	setUpdated := func(uuid string, at time.Time) {
		require.NoError(t, db.Model(&models.ImportSession{}).Where("uuid = ?", uuid).UpdateColumn("updated_at", at).Error)
	}
	setCreated("stale", now.Add(-25*time.Hour))
	setCreated("mounted", now.Add(-25*time.Hour))
	setCreated("fresh", now)
	setUpdated("closed-old", now.Add(-31*24*time.Hour))
	setUpdated("committed-old", now.Add(-40*24*time.Hour))

	require.NoError(t, sweepImportUploads(db, importDir, mountPath, now))

	var stale, fresh, mounted models.ImportSession
	require.NoError(t, db.Where("uuid = ?", "stale").First(&stale).Error)
	assert.Equal(t, "expired", stale.Status)
	_, err := os.Stat(staleFile)
	assert.True(t, os.IsNotExist(err), "an expired upload's file is removed")

	require.NoError(t, db.Where("uuid = ?", "fresh").First(&fresh).Error)
	assert.Equal(t, "pending", fresh.Status)
	_, err = os.Stat(freshFile)
	assert.NoError(t, err, "a fresh upload's file is kept")

	require.NoError(t, db.Where("uuid = ?", "mounted").First(&mounted).Error)
	assert.Equal(t, "pending", mounted.Status, "the mounted session is not expired")
	_, err = os.Stat(mountPath)
	assert.NoError(t, err)

	_, err = os.Stat(orphan)
	assert.True(t, os.IsNotExist(err), "an orphan entry older than the TTL is removed")

	var closed, committed int64
	require.NoError(t, db.Model(&models.ImportSession{}).Where("uuid = ?", "closed-old").Count(&closed).Error)
	require.NoError(t, db.Model(&models.ImportSession{}).Where("uuid = ?", "committed-old").Count(&committed).Error)
	assert.Zero(t, closed, "a rejected row older than the retention is deleted")
	assert.Equal(t, int64(1), committed, "committed rows are kept")
}

func TestSupersedeUploadSessionsExpiresOlderUploadsOnly(t *testing.T) {
	t.Parallel()
	db := setupImportTestDB(t)
	importDir := t.TempDir()
	oldFile := writeUpload(t, importDir, "old.caddyfile")
	newFile := writeUpload(t, importDir, "new.caddyfile")
	require.NoError(t, db.Create(&models.ImportSession{UUID: "old", SourceFile: oldFile, Status: "pending"}).Error)
	require.NoError(t, db.Create(&models.ImportSession{UUID: "new", SourceFile: newFile, Status: "pending"}).Error)

	require.NoError(t, supersedeUploadSessions(db, importDir, "new"))

	var old, keep models.ImportSession
	require.NoError(t, db.Where("uuid = ?", "old").First(&old).Error)
	require.NoError(t, db.Where("uuid = ?", "new").First(&keep).Error)
	assert.Equal(t, "expired", old.Status)
	assert.Equal(t, "pending", keep.Status)
	_, err := os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(newFile)
	assert.NoError(t, err)
}

// Every failure after the file is written leaves no file behind, and the answer is the existing one.
func TestUploadFailuresLeaveNoFile(t *testing.T) {
	t.Parallel()
	t.Run("parse error", func(t *testing.T) {
		t.Parallel()
		handler, mockImport := retentionHandler(t)
		mockImport.importErr = fmt.Errorf("bad directive")
		w := postUploadJSON(t, handler, "/upload", map[string]any{"content": "bad", "filename": "x"})
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, uploadEntries(t, handler.importDir))
	})
	t.Run("no hosts", func(t *testing.T) {
		t.Parallel()
		handler, mockImport := retentionHandler(t)
		mockImport.importResult = &caddy.ImportResult{}
		w := postUploadJSON(t, handler, "/upload", map[string]any{"content": "x", "filename": "x"})
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, uploadEntries(t, handler.importDir))
	})
	t.Run("session not stored", func(t *testing.T) {
		t.Parallel()
		db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.ProxyHost{}))
		handler, _, mockImport := setupTestHandler(t, db)
		handler.importerservice = &mockImporterAdapter{mockImport}
		mockImport.importResult = importableHost()
		w := postUploadJSON(t, handler, "/upload", map[string]any{"content": "x", "filename": "x"})
		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assert.Equal(t, msgImportSessionNotStored, decodeBody(t, w)["error"])
		assert.Empty(t, uploadEntries(t, handler.importDir))
	})
}

// A successful upload keeps its file; a later upload expires the earlier one only once its own row is stored.
func TestSuccessfulUploadKeepsItsFileAndSupersedesOnlyAfterStore(t *testing.T) {
	t.Parallel()
	handler, mockImport := retentionHandler(t)
	mockImport.importResult = importableHost()

	first := postUploadJSON(t, handler, "/upload", map[string]any{"content": "a", "filename": "a"})
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	firstID := decodeBody(t, first)["session"].(map[string]any)["id"].(string)
	assert.Len(t, uploadEntries(t, handler.importDir), 1, "a 200 keeps its file")

	mockImport.importErr = fmt.Errorf("bad directive")
	failed := postUploadJSON(t, handler, "/upload", map[string]any{"content": "b", "filename": "b"})
	require.Equal(t, http.StatusBadRequest, failed.Code)
	assert.Equal(t, 1, countStatus(t, handler.db, "pending"), "a failed upload does not expire the first")

	mockImport.importErr = nil
	second := postUploadJSON(t, handler, "/upload", map[string]any{"content": "c", "filename": "c"})
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())

	var first0 models.ImportSession
	require.NoError(t, handler.db.Where("uuid = ?", firstID).First(&first0).Error)
	assert.Equal(t, "expired", first0.Status)
	assert.Len(t, uploadEntries(t, handler.importDir), 1, "the superseded upload's file is removed")
}

func countStatus(t *testing.T, db *gorm.DB, status string) int {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.ImportSession{}).Where("status = ?", status).Count(&n).Error)
	return int(n)
}

func TestUploadMultiRefusesMoreThanTheFileLimitBeforeWriting(t *testing.T) {
	t.Parallel()
	handler, _ := retentionHandler(t)
	files := make([]map[string]any, 0, maxImportFiles+1)
	for i := range maxImportFiles + 1 {
		files = append(files, map[string]any{"filename": fmt.Sprintf("site%d.caddyfile", i), "content": "a.example.com { respond 200 }"})
	}
	w := postUploadJSON(t, handler, "/upload-multi", map[string]any{"files": files})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, codeImportTooManyFiles, decodeBody(t, w)["error_code"])
	assert.Empty(t, uploadEntries(t, handler.importDir))
}

// Commit and cancel remove the upload file of the session they finish.
func TestCommitAndCancelRemoveTheUploadFile(t *testing.T) {
	t.Parallel()
	t.Run("commit", func(t *testing.T) {
		t.Parallel()
		handler, mockImport := retentionHandler(t)
		mockImport.importResult = importableHost()
		path := writeUpload(t, handler.importDir, "commit-me.caddyfile")
		seedUploadSession(t, handler, "commit-me", path)

		w := postUploadJSON(t, handler, "/commit", map[string]any{"session_uuid": "commit-me", "resolutions": map[string]string{}})

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Empty(t, uploadEntries(t, handler.importDir))
	})
	t.Run("cancel", func(t *testing.T) {
		t.Parallel()
		handler, mockImport := retentionHandler(t)
		mockImport.importResult = importableHost()
		path := writeUpload(t, handler.importDir, "cancel-me.caddyfile")
		seedUploadSession(t, handler, "cancel-me", path)

		router := adminRouter(func(r *gin.Engine) { r.DELETE("/cancel", handler.Cancel) })
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/cancel?session_uuid=cancel-me", http.NoBody))

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Empty(t, uploadEntries(t, handler.importDir))
	})
}

// The import routes refuse a body over their limit with 413: 4 MiB for Caddyfile imports, 8 MiB for JSON and NPM.
func TestImportRoutesRefuseOversizedBodies(t *testing.T) {
	t.Parallel()
	db := setupImportTestDB(t)
	handler, _, _ := setupTestHandler(t, db)
	router := gin.New()
	addAdminMiddleware(router)
	handler.RegisterRoutes(router.Group("/api/v1"))
	NewJSONImportHandler(db).RegisterRoutes(router.Group("/api/v1"))
	NewNPMImportHandler(db).RegisterRoutes(router.Group("/api/v1"))

	big := func(n int) string {
		head := `{"content":"`
		tail := `"}`
		return head + strings.Repeat("a", n-len(head)-len(tail)) + tail
	}
	cases := []struct {
		path string
		size int64
	}{
		{"/api/v1/import/upload", maxImportBodyBytes},
		{"/api/v1/import/upload-multi", maxImportBodyBytes},
		{"/api/v1/import/json/upload", maxJSONImportBodyBytes},
		{"/api/v1/import/npm/upload", maxJSONImportBodyBytes},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(big(int(tc.size)+1)))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
		})
	}
}

// postUploadJSON posts a JSON body to an import route of handler and returns the answer.
func postUploadJSON(t *testing.T, handler *ImportHandler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	router := gin.New()
	addAdminMiddleware(router)
	handler.RegisterRoutes(router.Group("/api/v1"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import"+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}
