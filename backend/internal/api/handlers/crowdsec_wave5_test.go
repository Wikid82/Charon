package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCrowdsecWave5_ResolveAcquisitionConfigPath_RelativeRejected(t *testing.T) {
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", "relative/acquis.yaml")
	_, err := resolveAcquisitionConfigPath()
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be absolute")
}

func TestCrowdsecWave5_ReadAcquisitionConfig_InvalidFilenameBranch(t *testing.T) {
	_, err := readAcquisitionConfig("/")
	require.Error(t, err)
	require.Contains(t, err.Error(), "filename is invalid")
}

func TestCrowdsecWave5_GetLAPIDecisions_Unauthorized(t *testing.T) {
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)


	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "default", CrowdSecAPIURL: server.URL}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/lapi", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "authentication failed")
}

func TestCrowdsecWave5_GetLAPIDecisions_NonJSONContentTypeFallsBack(t *testing.T) {
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>not-json</html>"))
	}))
	t.Cleanup(server.Close)


	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "default", CrowdSecAPIURL: server.URL}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }
	h.CmdExec = &mockCmdExecutor{output: []byte("[]"), err: nil}
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/lapi", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "decisions")
}

func TestCrowdsecWave5_GetBouncerInfo_And_GetBouncerKey_FileSource(t *testing.T) {
	t.Setenv("CROWDSEC_BOUNCER_API_KEY", "")
	t.Setenv("CERBERUS_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CPM_SECURITY_CROWDSEC_API_KEY", "")
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }
	keyPath := h.bouncerKeyPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(keyPath), 0o750))
	require.NoError(t, os.WriteFile(keyPath, []byte("abcdefghijklmnop1234567890"), 0o600))

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	wInfo := httptest.NewRecorder()
	reqInfo := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/bouncer", http.NoBody)
	r.ServeHTTP(wInfo, reqInfo)
	require.Equal(t, http.StatusOK, wInfo.Code)
	require.Contains(t, wInfo.Body.String(), "file")

	wKey := httptest.NewRecorder()
	reqKey := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/bouncer/key", http.NoBody)
	r.ServeHTTP(wKey, reqKey)
	require.Equal(t, http.StatusOK, wKey.Code)
	require.Contains(t, wKey.Body.String(), "\"source\":\"file\"")
}
