package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// geoipDBFixture is the GeoIP database API over a temporary data folder, bundled copy and audit database. The routes
// mirror routes.go: the admin role gate and the same-origin check sit in front of the two mutating handlers.
type geoipDBFixture struct {
	t           *testing.T
	router      *gin.Engine
	handler     *SecurityHandler
	db          *gorm.DB
	root        string
	dataDir     string
	dataPath    string
	bundledPath string
	applyCalls  int
}

type geoipDBOptions struct {
	role     string
	env      bool
	noStore  bool
	applyErr error
	// apply, when set, replaces the default apply hook.
	apply func(context.Context) error
}

func newGeoIPDBFixture(t *testing.T, opts geoipDBOptions) *geoipDBFixture {
	t.Helper()
	if opts.role == "" {
		opts.role = string(models.RoleAdmin)
	}
	root := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(root, "test.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.SecurityAudit{}, &models.AccessList{}))

	f := &geoipDBFixture{
		t:           t,
		db:          db,
		root:        root,
		dataDir:     filepath.Join(root, "data", "geoip"),
		bundledPath: filepath.Join(root, "bundled", geoip.DatabaseFileName),
	}
	f.dataPath = filepath.Join(f.dataDir, geoip.DatabaseFileName)

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)
	f.handler = h
	if !opts.noStore {
		svc := services.NewUnloadedGeoIPService(f.dataPath)
		apply := opts.apply
		if apply == nil {
			apply = func(context.Context) error {
				f.applyCalls++
				return opts.applyErr
			}
		}
		h.geoipSvc = svc
		h.geoipStore = services.NewGeoIPDatabaseStore(services.GeoIPDatabaseStoreConfig{
			Service:     svc,
			Apply:       apply,
			DataPath:    f.dataPath,
			BundledPath: f.bundledPath,
			EnvInEffect: func() bool { return opts.env },
		})
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.RoleKey, opts.role)
		c.Set(middleware.UserIDKey, uint(7))
		c.Next()
	})
	securityAdmin := r.Group("/security")
	securityAdmin.Use(middleware.RequireRole(models.RoleAdmin))
	securityAdmin.POST("/geoip/database", middleware.RequireSameOriginForBrowserAuth(), h.ReplaceGeoIPDatabase)
	securityAdmin.DELETE("/geoip/database", middleware.RequireSameOriginForBrowserAuth(), h.RemoveGeoIPDatabase)
	f.router = r
	return f
}

// upload builds a multipart request with one file part named field, carrying data.
func upload(t *testing.T, field string, data []byte) (body io.Reader, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(field, "client-chosen-name.mmdb")
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

// validUpload is a country database that resolves every address, built with buildEpoch.
func validUpload(t *testing.T, buildEpoch uint32, opts ...testutil.GeoIPCountryOption) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), geoip.DatabaseFileName)
	all := append([]testutil.GeoIPCountryOption{
		testutil.WithGeoIPDatabaseType("GeoLite2-Country"),
		testutil.WithGeoIPRecordISOCode("US"),
	}, opts...)
	testutil.WriteGeoIPCountryDB(t, path, buildEpoch, all...)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

// do sends a request through the fixture router.
func (f *geoipDBFixture) do(method string, body io.Reader, contentType string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/security/geoip/database", body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

// replace uploads data as the file part and returns the response.
func (f *geoipDBFixture) replace(data []byte) *httptest.ResponseRecorder {
	body, ctype := upload(f.t, "file", data)
	return f.do(http.MethodPost, body, ctype, nil)
}

// seedDataCopy writes and loads a data-folder copy with buildEpoch.
func (f *geoipDBFixture) seedDataCopy(buildEpoch uint32) {
	f.t.Helper()
	require.NoError(f.t, os.MkdirAll(f.dataDir, 0o755))
	testutil.WriteGeoIPCountryDB(f.t, f.dataPath, buildEpoch, testutil.WithGeoIPDatabaseType("GeoLite2-Country"))
	require.NoError(f.t, f.handler.geoipStore.Reload(context.Background()))
}

// auditRows returns the audit rows with the given action, after the audit worker has persisted them.
func (f *geoipDBFixture) auditRows(action string) []models.SecurityAudit {
	f.t.Helper()
	f.handler.svc.Flush()
	var rows []models.SecurityAudit
	require.NoError(f.t, f.db.Where("action = ?", action).Find(&rows).Error)
	return rows
}

// decode unmarshals a response body into a map.
func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
	return body
}

func TestGeoIPDatabase_ViewerIsForbiddenAndNothingChanges(t *testing.T) {
	f := newGeoIPDBFixture(t, geoipDBOptions{role: string(models.RoleUser)})
	f.seedDataCopy(1_750_000_000)
	before, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)

	replaced := f.replace(validUpload(t, 1_800_000_000))
	removed := f.do(http.MethodDelete, nil, "", nil)

	assert.Equal(t, http.StatusForbidden, replaced.Code)
	assert.Equal(t, http.StatusForbidden, removed.Code)
	after, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestGeoIPDatabase_MissingStoreIsUnavailable(t *testing.T) {
	f := newGeoIPDBFixture(t, geoipDBOptions{noStore: true})

	replaced := f.replace(validUpload(t, 1_700_000_000))
	removed := f.do(http.MethodDelete, nil, "", nil)

	for _, w := range []*httptest.ResponseRecorder{replaced, removed} {
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Equal(t, "geoip_service_unavailable", decode(t, w)["error_code"])
	}
}

func TestReplaceGeoIPDatabase_InstallsUploadAndAudits(t *testing.T) {
	f := newGeoIPDBFixture(t, geoipDBOptions{})
	data := validUpload(t, 1_700_000_000)

	w := f.replace(data)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decode(t, w)
	assert.Equal(t, false, body["replaced_previous"])
	status, ok := body["status"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "GeoLite2-Country", status["database_type"])
	assert.Equal(t, "2023-11-14T22:13:20Z", status["build_date"])

	rows := f.auditRows("replace_geoip_database")
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0].Details, "type=GeoLite2-Country build=2023-11-14T22:13:20Z size=")
	assert.Contains(t, rows[0].Details, fmt.Sprintf("size=%d", len(data)))
	assert.Regexp(t, `sha256=[0-9a-f]{64}$`, rows[0].Details)
	assert.NotContains(t, rows[0].Details, f.root, "audit rows never carry a path")
}

func TestReplaceGeoIPDatabase_ReplacesExistingCopy(t *testing.T) {
	f := newGeoIPDBFixture(t, geoipDBOptions{})
	f.seedDataCopy(1_700_000_000)

	w := f.replace(validUpload(t, 1_750_000_000))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, true, decode(t, w)["replaced_previous"])
}

func TestReplaceGeoIPDatabase_RefusedUploadsReturnStableCodes(t *testing.T) {
	cases := []struct {
		name       string
		env        bool
		prepare    func(f *geoipDBFixture)
		body       func(t *testing.T) (io.Reader, string)
		wantStatus int
		wantCode   string
		wantReason string // non-empty when a reject audit row is expected
	}{
		{
			name: "not multipart",
			body: func(t *testing.T) (io.Reader, string) {
				return strings.NewReader("plain"), "text/plain"
			},
			wantStatus: http.StatusBadRequest, wantCode: "geoip_file_required",
		},
		{
			name: "wrong part name",
			body: func(t *testing.T) (io.Reader, string) {
				return upload(t, "other", validUpload(t, 1_700_000_000))
			},
			wantStatus: http.StatusBadRequest, wantCode: "geoip_file_required",
		},
		{
			name: "a second part after the file",
			body: func(t *testing.T) (io.Reader, string) {
				var buf bytes.Buffer
				mw := multipart.NewWriter(&buf)
				part, err := mw.CreateFormFile("file", "a.mmdb")
				require.NoError(t, err)
				_, err = part.Write(validUpload(t, 1_700_000_000))
				require.NoError(t, err)
				extra, err := mw.CreateFormField("note")
				require.NoError(t, err)
				_, err = extra.Write([]byte("x"))
				require.NoError(t, err)
				require.NoError(t, mw.Close())
				return &buf, mw.FormDataContentType()
			},
			wantStatus: http.StatusBadRequest, wantCode: "geoip_file_required", wantReason: "geoip_file_required",
		},
		{
			name: "a malformed trailer after the file",
			body: func(t *testing.T) (io.Reader, string) {
				var buf bytes.Buffer
				mw := multipart.NewWriter(&buf)
				part, err := mw.CreateFormFile("file", "a.mmdb")
				require.NoError(t, err)
				_, err = part.Write(validUpload(t, 1_700_000_000))
				require.NoError(t, err)
				buf.WriteString("--" + mw.Boundary() + "\r\nmalformed header line\r\n\r\n")
				return &buf, mw.FormDataContentType()
			},
			wantStatus: http.StatusBadRequest, wantCode: "geoip_file_required", wantReason: "geoip_file_required",
		},
		{
			name: "not a MaxMind database",
			body: func(t *testing.T) (io.Reader, string) {
				return upload(t, "file", []byte("plain text"))
			},
			wantStatus: http.StatusBadRequest, wantCode: "geoip_not_mmdb", wantReason: "geoip_not_mmdb",
		},
		{
			name: "not a country database",
			body: func(t *testing.T) (io.Reader, string) {
				return upload(t, "file", validUpload(t, 1_700_000_000, testutil.WithGeoIPDatabaseType("GeoLite2-ASN")))
			},
			wantStatus: http.StatusBadRequest, wantCode: "geoip_wrong_type", wantReason: "geoip_wrong_type",
		},
		{
			name: "no build date",
			body: func(t *testing.T) (io.Reader, string) {
				return upload(t, "file", validUpload(t, 0))
			},
			wantStatus: http.StatusBadRequest, wantCode: "geoip_corrupt", wantReason: "geoip_corrupt",
		},
		{
			name: "build date in the future",
			body: func(t *testing.T) (io.Reader, string) {
				future := uint32(time.Now().Add(48 * time.Hour).Unix())
				return upload(t, "file", validUpload(t, future))
			},
			wantStatus: http.StatusBadRequest, wantCode: "geoip_build_date_in_future", wantReason: "geoip_build_date_in_future",
		},
		{
			name:       "custom path is set",
			env:        true,
			body:       func(t *testing.T) (io.Reader, string) { return upload(t, "file", validUpload(t, 1_700_000_000)) },
			wantStatus: http.StatusConflict, wantCode: "geoip_custom_path_set",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newGeoIPDBFixture(t, geoipDBOptions{env: tc.env})
			if tc.prepare != nil {
				tc.prepare(f)
			}
			body, ctype := tc.body(t)

			w := f.do(http.MethodPost, body, ctype, nil)

			require.Equal(t, tc.wantStatus, w.Code, w.Body.String())
			respBody := decode(t, w)
			assert.Equal(t, tc.wantCode, respBody["error_code"])
			assert.NotContains(t, w.Body.String(), f.root, "error bodies carry no paths")
			assert.NoFileExists(t, f.dataPath)
			assert.Empty(t, leftoverUploadFiles(t, f.dataDir))

			rows := f.auditRows("reject_geoip_database")
			if tc.wantReason == "" {
				assert.Empty(t, rows)
				return
			}
			require.Len(t, rows, 1)
			assert.Contains(t, rows[0].Details, "reason="+tc.wantReason)
			assert.Contains(t, rows[0].Details, "size=")
		})
	}
}

// leftoverUploadFiles lists upload temp files in dir.
func leftoverUploadFiles(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".GeoLite2-Country.mmdb.upload-*"))
	require.NoError(t, err)
	return matches
}

func TestReplaceGeoIPDatabase_OlderThanCurrentReportsBothDates(t *testing.T) {
	f := newGeoIPDBFixture(t, geoipDBOptions{})
	f.seedDataCopy(1_750_000_000)

	w := f.replace(validUpload(t, 1_700_000_000))

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	body := decode(t, w)
	assert.Equal(t, "geoip_older_than_current", body["error_code"])
	assert.Equal(t, "2023-11-14T22:13:20Z", body["uploaded_build_date"])
	assert.Equal(t, "2025-06-15T15:06:40Z", body["current_build_date"])
}

func TestReplaceGeoIPDatabase_OversizedUploadsAreTooLarge(t *testing.T) {
	const limit = 32 << 20
	t.Run("a file one byte over the limit", func(t *testing.T) {
		f := newGeoIPDBFixture(t, geoipDBOptions{})

		w := f.replace(make([]byte, limit+1))

		require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Equal(t, "geoip_too_large", decode(t, w)["error_code"])
		rows := f.auditRows("reject_geoip_database")
		require.Len(t, rows, 1)
		assert.Contains(t, rows[0].Details, "reason=geoip_too_large")
	})
	t.Run("headers that exhaust the request limit while the file streams", func(t *testing.T) {
		f := newGeoIPDBFixture(t, geoipDBOptions{})
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		header := textproto.MIMEHeader{
			"Content-Disposition": {`form-data; name="file"; filename="a.mmdb"`},
			"X-Padding":           {strings.Repeat("p", 100<<10)},
		}
		part, err := mw.CreatePart(header)
		require.NoError(t, err)
		_, err = part.Write(make([]byte, limit))
		require.NoError(t, err)
		require.NoError(t, mw.Close())

		w := f.do(http.MethodPost, &buf, mw.FormDataContentType(), nil)

		require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Equal(t, "geoip_too_large", decode(t, w)["error_code"])
	})
}

func TestReplaceGeoIPDatabase_BusyWhileAnotherChangeRuns(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once bool
	f := newGeoIPDBFixture(t, geoipDBOptions{apply: func(context.Context) error {
		if !once {
			once = true
			close(started)
			<-release
		}
		return nil
	}})

	first := make(chan *httptest.ResponseRecorder, 1)
	go func() { first <- f.replace(validUpload(t, 1_700_000_000)) }()
	<-started

	second := f.replace(validUpload(t, 1_750_000_000))
	assert.Equal(t, http.StatusConflict, second.Code)
	assert.Equal(t, "geoip_busy", decode(t, second)["error_code"])

	close(release)
	assert.Equal(t, http.StatusOK, (<-first).Code)
}

func TestReplaceGeoIPDatabase_ApplyFailureIsRolledBack(t *testing.T) {
	f := newGeoIPDBFixture(t, geoipDBOptions{applyErr: errors.New("proxy rejected config")})

	w := f.replace(validUpload(t, 1_700_000_000))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "proxy_apply_failed", decode(t, w)["error_code"])
	assert.NoFileExists(t, f.dataPath, "a failed apply removes the upload when there was no previous copy")
	assert.NotContains(t, w.Body.String(), "proxy rejected config")
}

func TestReplaceGeoIPDatabase_ReadOnlyFolderIsConflict(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	f := newGeoIPDBFixture(t, geoipDBOptions{})
	require.NoError(t, os.MkdirAll(f.dataDir, 0o755))
	require.NoError(t, os.Chmod(f.dataDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(f.dataDir, 0o755) })

	w := f.replace(validUpload(t, 1_700_000_000))

	require.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "data_folder_read_only", decode(t, w)["error_code"])
}

func TestRemoveGeoIPDatabase_RestoresBundledCopyAndAudits(t *testing.T) {
	f := newGeoIPDBFixture(t, geoipDBOptions{})
	require.NoError(t, os.MkdirAll(filepath.Dir(f.bundledPath), 0o755))
	testutil.WriteGeoIPCountryDB(t, f.bundledPath, 1_700_000_000)
	f.seedDataCopy(1_750_000_000)

	w := f.do(http.MethodDelete, nil, "", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	status, ok := decode(t, w)["status"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2023-11-14T22:13:20Z", status["build_date"], "the bundled copy is in use")
	assert.NoFileExists(t, f.dataPath)

	rows := f.auditRows("remove_geoip_database")
	require.Len(t, rows, 1)
	assert.Equal(t, "build=2025-06-15T15:06:40Z", rows[0].Details)
	assert.NotContains(t, rows[0].Details, f.root)
}

func TestRemoveGeoIPDatabase_RefusedCases(t *testing.T) {
	t.Run("no data-folder copy is not found", func(t *testing.T) {
		f := newGeoIPDBFixture(t, geoipDBOptions{})

		w := f.do(http.MethodDelete, nil, "", nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "geoip_no_data_copy", decode(t, w)["error_code"])
	})
	t.Run("custom path is a conflict", func(t *testing.T) {
		f := newGeoIPDBFixture(t, geoipDBOptions{env: true})
		f.seedDataCopy(1_750_000_000)

		w := f.do(http.MethodDelete, nil, "", nil)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "geoip_custom_path_set", decode(t, w)["error_code"])
		assert.FileExists(t, f.dataPath)
	})
	t.Run("busy is a conflict", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		f := newGeoIPDBFixture(t, geoipDBOptions{})
		f.seedDataCopy(1_750_000_000)
		var once bool
		f.handler.geoipStore = services.NewGeoIPDatabaseStore(services.GeoIPDatabaseStoreConfig{
			Service:     f.handler.geoipSvc,
			DataPath:    f.dataPath,
			BundledPath: f.bundledPath,
			Apply: func(context.Context) error {
				if !once {
					once = true
					close(started)
					<-release
				}
				return nil
			},
		})

		done := make(chan *httptest.ResponseRecorder, 1)
		go func() { done <- f.replace(validUpload(t, 1_760_000_000)) }()
		<-started
		w := f.do(http.MethodDelete, nil, "", nil)
		close(release)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "geoip_busy", decode(t, w)["error_code"])
		<-done
	})
	t.Run("apply failure restores the copy", func(t *testing.T) {
		f := newGeoIPDBFixture(t, geoipDBOptions{})
		require.NoError(t, os.MkdirAll(filepath.Dir(f.bundledPath), 0o755))
		testutil.WriteGeoIPCountryDB(t, f.bundledPath, 1_700_000_000)
		f.seedDataCopy(1_750_000_000)
		f.handler.geoipStore = services.NewGeoIPDatabaseStore(services.GeoIPDatabaseStoreConfig{
			Service:     f.handler.geoipSvc,
			DataPath:    f.dataPath,
			BundledPath: f.bundledPath,
			Apply:       func(context.Context) error { return errors.New("proxy rejected config") },
		})

		w := f.do(http.MethodDelete, nil, "", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "proxy_apply_failed", decode(t, w)["error_code"])
		assert.FileExists(t, f.dataPath)
	})
}

func TestGeoIPDatabase_CrossOriginRequestsAreRefused(t *testing.T) {
	crossSite := map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://app.charon.test"}

	t.Run("cookie session from a sibling origin cannot replace", func(t *testing.T) {
		f := newGeoIPDBFixture(t, geoipDBOptions{})
		body, ctype := upload(t, "file", validUpload(t, 1_700_000_000))

		w := f.do(http.MethodPost, body, ctype, crossSite)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "cross_origin_refused", decode(t, w)["error_code"])
		assert.NoFileExists(t, f.dataPath)
	})
	t.Run("cookie session from a sibling origin cannot remove", func(t *testing.T) {
		f := newGeoIPDBFixture(t, geoipDBOptions{})
		f.seedDataCopy(1_750_000_000)
		applied := f.applyCalls

		w := f.do(http.MethodDelete, nil, "", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"})

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.FileExists(t, f.dataPath, "the copy is not removed when the origin check refuses")
		assert.Equal(t, applied, f.applyCalls, "the handler does not run")
	})
	t.Run("header-authenticated client is not refused by the origin check", func(t *testing.T) {
		f := newGeoIPDBFixture(t, geoipDBOptions{})
		f.seedDataCopy(1_750_000_000)

		w := f.do(http.MethodDelete, nil, "", map[string]string{
			"Authorization":  "Bearer token",
			"Sec-Fetch-Site": "cross-site",
			"Origin":         "https://evil.example",
		})

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGeoIPErrorFailure_MapsEveryFailureToItsCode(t *testing.T) {
	older := &services.OlderThanCurrentError{Uploaded: 1, Current: 2}
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		refused    bool
	}{
		{name: "busy", err: services.ErrGeoIPBusy, wantStatus: 409, wantCode: "geoip_busy"},
		{name: "custom path", err: services.ErrGeoIPCustomPathSet, wantStatus: 409, wantCode: "geoip_custom_path_set"},
		{name: "no data copy", err: services.ErrGeoIPNoDataCopy, wantStatus: 404, wantCode: "geoip_no_data_copy"},
		{name: "too large", err: services.ErrGeoIPTooLarge, wantStatus: 413, wantCode: "geoip_too_large", refused: true},
		{
			name:       "http body limit",
			err:        fmt.Errorf("read: %w", &http.MaxBytesError{Limit: 1}),
			wantStatus: 413, wantCode: "geoip_too_large", refused: true,
		},
		{name: "upload read", err: services.ErrGeoIPUploadRead, wantStatus: 400, wantCode: "geoip_file_required", refused: true},
		{name: "not mmdb", err: geoip.ErrNotMMDB, wantStatus: 400, wantCode: "geoip_not_mmdb", refused: true},
		{name: "corrupt", err: geoip.ErrCorruptDatabase, wantStatus: 400, wantCode: "geoip_corrupt", refused: true},
		{name: "no build date", err: geoip.ErrNoBuildDate, wantStatus: 400, wantCode: "geoip_corrupt", refused: true},
		{name: "wrong type", err: geoip.ErrWrongDatabaseType, wantStatus: 400, wantCode: "geoip_wrong_type", refused: true},
		{name: "older", err: older, wantStatus: 409, wantCode: "geoip_older_than_current", refused: true},
		{name: "future", err: services.ErrGeoIPBuildDateInFuture, wantStatus: 400, wantCode: "geoip_build_date_in_future", refused: true},
		{name: "reload", err: services.ErrGeoIPReloadFailed, wantStatus: 500, wantCode: "geoip_reload_failed"},
		{name: "apply", err: services.ErrGeoIPApplyFailed, wantStatus: 500, wantCode: "proxy_apply_failed"},
		{name: "read only", err: services.ErrGeoIPDataFolderReadOnly, wantStatus: 409, wantCode: "data_folder_read_only"},
		{name: "unknown", err: errors.New("/some/path failed"), wantStatus: 500, wantCode: "geoip_write_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := geoipFailureFor(tc.err)

			assert.Equal(t, tc.wantStatus, got.status)
			assert.Equal(t, tc.wantCode, got.code)
			assert.Equal(t, tc.refused, got.refused)
			assert.NotEmpty(t, got.msg)
		})
	}
}

func TestGeoIPErrorResponse_OmitsDetailAndDatesWhenNotApplicable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", http.NoBody)

	geoipErrorResponse(c, errors.New("open /app/data/secret: permission denied"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	body := decode(t, w)
	assert.Equal(t, "geoip_write_failed", body["error_code"])
	assert.NotContains(t, w.Body.String(), "/app/data")
	assert.NotContains(t, body, "uploaded_build_date")
}

func TestFormatBuildEpoch(t *testing.T) {
	assert.Equal(t, "unknown", formatBuildEpoch(0))
	assert.Equal(t, "2023-11-14T22:13:20Z", formatBuildEpoch(1_700_000_000))
}

func TestSinglePartReader_EndsOnlyWhenNoPartFollows(t *testing.T) {
	body, ctype := upload(t, "file", []byte("content"))
	mr := multipartReaderFor(t, body, ctype)
	part, err := mr.NextPart()
	require.NoError(t, err)

	data, err := io.ReadAll(singlePartReader{part: part, mr: mr})

	require.NoError(t, err)
	assert.Equal(t, "content", string(data))
}

func TestSinglePartReader_RejectsAFollowingPart(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "a.mmdb")
	require.NoError(t, err)
	_, err = part.Write([]byte("content"))
	require.NoError(t, err)
	_, err = mw.CreateFormField("note")
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	mr := multipartReaderFor(t, &buf, mw.FormDataContentType())
	first, err := mr.NextPart()
	require.NoError(t, err)

	_, err = io.ReadAll(singlePartReader{part: first, mr: mr})

	assert.ErrorIs(t, err, errUnexpectedPart)
}

// multipartReaderFor returns the multipart reader of a request body.
func multipartReaderFor(t *testing.T, body io.Reader, contentType string) *multipart.Reader {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", contentType)
	mr, err := req.MultipartReader()
	require.NoError(t, err)
	return mr
}

// The handlers check the admin role themselves, so a request that reaches them without the route's role gate is still
// refused.
func TestGeoIPDatabaseHandlers_RefuseNonAdminWithoutRouteGate(t *testing.T) {
	f := newGeoIPDBFixture(t, geoipDBOptions{role: string(models.RoleUser)})
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.RoleKey, string(models.RoleUser))
		c.Next()
	})
	r.POST("/security/geoip/database", f.handler.ReplaceGeoIPDatabase)
	r.DELETE("/security/geoip/database", f.handler.RemoveGeoIPDatabase)

	body, ctype := upload(t, "file", validUpload(t, 1_700_000_000))
	req := httptest.NewRequest(http.MethodPost, "/security/geoip/database", body)
	req.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/security/geoip/database", http.NoBody))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.NoFileExists(t, f.dataPath)
}

func TestSinglePartReader_ReportsTruncatedTrailer(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "a.mmdb")
	require.NoError(t, err)
	_, err = part.Write([]byte("content"))
	require.NoError(t, err)
	// The body stops inside the header block of a second part.
	buf.WriteString("\r\n--" + mw.Boundary() + "\r\nX-Partial")
	mr := multipartReaderFor(t, &buf, mw.FormDataContentType())
	first, err := mr.NextPart()
	require.NoError(t, err)

	_, err = io.ReadAll(singlePartReader{part: first, mr: mr})

	assert.Error(t, err)
	assert.NotErrorIs(t, err, errUnexpectedPart, "a read failure is not reported as an extra part")
}
