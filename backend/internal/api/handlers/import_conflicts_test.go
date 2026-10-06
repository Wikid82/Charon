package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
)

// failingListProxyHostSvc simulates a proxy host service whose List() fails.
type failingListProxyHostSvc struct{}

func (failingListProxyHostSvc) Create(*models.ProxyHost) error { return nil }
func (failingListProxyHostSvc) Update(*models.ProxyHost) error { return nil }
func (failingListProxyHostSvc) List() ([]models.ProxyHost, error) {
	return nil, errors.New("simulated list failure: secret detail")
}

// setupNoProxyHostTableDB returns a DB without the proxy_hosts table so that
// proxyHostSvc.List() fails.
func setupNoProxyHostTableDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "noph.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ImportSession{}))
	return db
}

func assertGenericListFailure(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to check for existing hosts")
	assert.NotContains(t, w.Body.String(), "secret detail")
	assert.NotContains(t, w.Body.String(), "no such table")
}

func postImportJSON(r http.Handler, path string, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestDetectImportConflicts_Overlap(t *testing.T) {
	existing := []models.ProxyHost{{DomainNames: "a.example.com, B.Example.com", ForwardHost: "old"}}
	imported := []caddy.ParsedHost{
		{DomainNames: " b.example.COM ,c.example.com", ForwardHost: "new"},
		{DomainNames: "unrelated.example.com"},
	}

	conflicts, details := detectImportConflicts(existing, imported)

	assert.Equal(t, []string{" b.example.COM ,c.example.com"}, conflicts)
	require.Contains(t, details, " b.example.COM ,c.example.com")
	assert.Len(t, details, 1)
}

func TestDetectImportConflicts_NoConflicts(t *testing.T) {
	conflicts, details := detectImportConflicts(nil, []caddy.ParsedHost{{DomainNames: "x.example.com"}})
	assert.Empty(t, conflicts)
	assert.Empty(t, details)
}

func TestJSONImportHandler_Upload_OverlapReportedAsConflict(t *testing.T) {
	db := setupJSONTestDB(t)
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID: "e1", DomainNames: "one.example.com, Two.Example.com", ForwardScheme: "http", ForwardHost: "old", ForwardPort: 80, Enabled: true,
	}).Error)

	router := gin.New()
	NewJSONImportHandler(db).RegisterRoutes(router.Group("/api/v1"))

	t.Run("charon format", func(t *testing.T) {
		content, _ := json.Marshal(CharonExport{Version: "1.0.0", ProxyHosts: []CharonProxyHost{
			{UUID: "n1", DomainNames: " two.example.com ,three.example.com", ForwardScheme: "http", ForwardHost: "new", ForwardPort: 8080, Enabled: true},
		}})
		w := postImportJSON(router, "/api/v1/import/json/upload", map[string]string{"content": string(content)})
		require.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp["conflict_details"])
		assert.Contains(t, resp["preview"].(map[string]any)["conflicts"], " two.example.com ,three.example.com")
	})

	t.Run("npm format", func(t *testing.T) {
		content, _ := json.Marshal(NPMExport{ProxyHosts: []NPMProxyHost{
			{ID: 1, DomainNames: []string{"ONE.example.com", "other.example.com"}, ForwardScheme: "http", ForwardHost: "new", ForwardPort: 8080, Enabled: true},
		}})
		w := postImportJSON(router, "/api/v1/import/json/upload", map[string]string{"content": string(content)})
		require.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp["conflict_details"])
		assert.NotEmpty(t, resp["preview"].(map[string]any)["conflicts"])
	})
}

func TestNPMImportHandler_Upload_OverlapReportedAsConflict(t *testing.T) {
	db := setupNPMTestDB(t)
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID: "e1", DomainNames: "one.example.com, Two.Example.com", ForwardScheme: "http", ForwardHost: "old", ForwardPort: 80, Enabled: true,
	}).Error)

	router := gin.New()
	NewNPMImportHandler(db).RegisterRoutes(router.Group("/api/v1"))

	content, _ := json.Marshal(NPMExport{ProxyHosts: []NPMProxyHost{
		{ID: 1, DomainNames: []string{"TWO.example.com", "extra.example.com"}, ForwardScheme: "http", ForwardHost: "new", ForwardPort: 8080, Enabled: true},
	}})
	w := postImportJSON(router, "/api/v1/import/npm/upload", map[string]string{"content": string(content)})
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["conflict_details"])
	assert.NotEmpty(t, resp["preview"].(map[string]any)["conflicts"])
}

func TestJSONAndNPMImport_ListFailureFailsClosed(t *testing.T) {
	charonContent, _ := json.Marshal(CharonExport{Version: "1.0.0", ProxyHosts: []CharonProxyHost{
		{UUID: "n1", DomainNames: "a.example.com", ForwardScheme: "http", ForwardHost: "new", ForwardPort: 8080, Enabled: true},
	}})
	npmContent, _ := json.Marshal(NPMExport{ProxyHosts: []NPMProxyHost{
		{ID: 1, DomainNames: []string{"a.example.com"}, ForwardScheme: "http", ForwardHost: "new", ForwardPort: 8080, Enabled: true},
	}})

	t.Run("json upload charon", func(t *testing.T) {
		router := gin.New()
		NewJSONImportHandler(setupNoProxyHostTableDB(t)).RegisterRoutes(router.Group("/api/v1"))
		assertGenericListFailure(t, postImportJSON(router, "/api/v1/import/json/upload", map[string]string{"content": string(charonContent)}))
	})
	t.Run("json upload npm", func(t *testing.T) {
		router := gin.New()
		NewJSONImportHandler(setupNoProxyHostTableDB(t)).RegisterRoutes(router.Group("/api/v1"))
		assertGenericListFailure(t, postImportJSON(router, "/api/v1/import/json/upload", map[string]string{"content": string(npmContent)}))
	})
	t.Run("npm upload", func(t *testing.T) {
		router := gin.New()
		NewNPMImportHandler(setupNoProxyHostTableDB(t)).RegisterRoutes(router.Group("/api/v1"))
		assertGenericListFailure(t, postImportJSON(router, "/api/v1/import/npm/upload", map[string]string{"content": string(npmContent)}))
	})
	t.Run("json commit charon", func(t *testing.T) {
		db := setupNoProxyHostTableDB(t)
		h := NewJSONImportHandler(db)
		router := gin.New()
		h.RegisterRoutes(router.Group("/api/v1"))
		jsonSID := "sid-charon"
		var export CharonExport
		require.NoError(t, json.Unmarshal(charonContent, &export))
		jsonImportSessionsMu.Lock()
		jsonImportSessions[jsonSID] = jsonImportSession{SourceType: "charon", CharonExport: &export}
		jsonImportSessionsMu.Unlock()
		assertGenericListFailure(t, postImportJSON(router, "/api/v1/import/json/commit", map[string]any{"session_uuid": jsonSID, "resolutions": map[string]string{}}))
	})
	t.Run("json commit npm", func(t *testing.T) {
		db := setupNoProxyHostTableDB(t)
		h := NewJSONImportHandler(db)
		router := gin.New()
		h.RegisterRoutes(router.Group("/api/v1"))
		jsonSID := "sid-npm"
		var export NPMExport
		require.NoError(t, json.Unmarshal(npmContent, &export))
		jsonImportSessionsMu.Lock()
		jsonImportSessions[jsonSID] = jsonImportSession{SourceType: "npm", NPMExport: &export}
		jsonImportSessionsMu.Unlock()
		assertGenericListFailure(t, postImportJSON(router, "/api/v1/import/json/commit", map[string]any{"session_uuid": jsonSID, "resolutions": map[string]string{}}))
	})
	t.Run("npm commit", func(t *testing.T) {
		db := setupNoProxyHostTableDB(t)
		router := gin.New()
		NewNPMImportHandler(db).RegisterRoutes(router.Group("/api/v1"))
		var export NPMExport
		require.NoError(t, json.Unmarshal(npmContent, &export))
		npmImportSessionsMu.Lock()
		npmImportSessions["sid-npm-commit"] = export
		npmImportSessionsMu.Unlock()
		assertGenericListFailure(t, postImportJSON(router, "/api/v1/import/npm/commit", map[string]any{"session_uuid": "sid-npm-commit", "resolutions": map[string]string{}}))
	})
}

func newCaddyfileImportHandlerWithFailingList(t *testing.T, mountPath string) (*ImportHandler, *MockImporterService) {
	t.Helper()
	db := setupNoProxyHostTableDB(t)
	mockSvc := new(MockImporterService)
	h := NewImportHandlerWithService(db, failingListProxyHostSvc{}, "caddy", t.TempDir(), mountPath, nil)
	h.importerservice = mockSvc
	return h, mockSvc
}

func importableResult() *caddy.ImportResult {
	return &caddy.ImportResult{Hosts: []caddy.ParsedHost{{DomainNames: "a.example.com", ForwardHost: "localhost", ForwardPort: 8080}}}
}

func adminRouter(register func(r *gin.Engine)) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	register(r)
	return r
}

func TestImportHandler_ListFailureFailsClosed(t *testing.T) {
	t.Run("upload", func(t *testing.T) {
		h, mockSvc := newCaddyfileImportHandlerWithFailingList(t, "")
		mockSvc.On("NormalizeCaddyfile", mock.Anything).Return("a.example.com { reverse_proxy localhost:8080 }", nil)
		mockSvc.On("ImportFile", mock.Anything).Return(importableResult(), nil)
		r := adminRouter(func(r *gin.Engine) { r.POST("/upload", h.Upload) })
		assertGenericListFailure(t, postImportJSON(r, "/upload", map[string]string{"filename": "Caddyfile", "content": "x"}))
	})

	t.Run("upload multi", func(t *testing.T) {
		h, mockSvc := newCaddyfileImportHandlerWithFailingList(t, "")
		mockSvc.On("ImportFile", mock.Anything).Return(importableResult(), nil)
		r := adminRouter(func(r *gin.Engine) { r.POST("/upload-multi", h.UploadMulti) })
		w := postImportJSON(r, "/upload-multi", map[string]any{"files": []map[string]string{{"filename": "Caddyfile", "content": "a.example.com { reverse_proxy localhost:8080 }"}}})
		assertGenericListFailure(t, w)
	})

	t.Run("preview mount", func(t *testing.T) {
		mount := filepath.Join(t.TempDir(), "Caddyfile")
		require.NoError(t, os.WriteFile(mount, []byte("a.example.com { reverse_proxy localhost:8080 }"), 0o600))
		h, mockSvc := newCaddyfileImportHandlerWithFailingList(t, mount)
		mockSvc.On("ImportFile", mock.Anything).Return(importableResult(), nil)
		r := adminRouter(func(r *gin.Engine) { r.GET("/preview", h.GetPreview) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preview", http.NoBody))
		assertGenericListFailure(t, w)
	})

	t.Run("commit", func(t *testing.T) {
		h, mockSvc := newCaddyfileImportHandlerWithFailingList(t, "")
		mockSvc.On("ImportFile", mock.Anything).Return(importableResult(), nil)
		uploads := filepath.Join(h.importDir, "uploads")
		require.NoError(t, os.MkdirAll(uploads, 0o750))
		sid := "list-fail-commit"
		require.NoError(t, os.WriteFile(filepath.Join(uploads, sid+".caddyfile"), []byte("a.example.com { reverse_proxy localhost:8080 }"), 0o600))
		r := adminRouter(func(r *gin.Engine) { r.POST("/commit", h.Commit) })
		assertGenericListFailure(t, postImportJSON(r, "/commit", map[string]any{"session_uuid": sid, "resolutions": map[string]string{}}))
	})
}
