package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAcquisitionConfigPath_Validation(t *testing.T) {
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", "")
	resolved, err := resolveAcquisitionConfigPath()
	require.NoError(t, err)
	require.Equal(t, "/etc/crowdsec/acquis.yaml", resolved)

	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", "relative/acquis.yaml")
	_, err = resolveAcquisitionConfigPath()
	require.Error(t, err)

	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", "/tmp/../etc/acquis.yaml")
	_, err = resolveAcquisitionConfigPath()
	require.Error(t, err)

	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", "/tmp/acquis.yaml")
	resolved, err = resolveAcquisitionConfigPath()
	require.NoError(t, err)
	require.Equal(t, "/tmp/acquis.yaml", resolved)
}

func TestReadAcquisitionConfig_ErrorsAndSuccess(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "acquis.yaml")
	require.NoError(t, os.WriteFile(path, []byte("source: file\n"), 0o600))

	content, err := readAcquisitionConfig(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "source: file")

	_, err = readAcquisitionConfig(filepath.Join(tmp, "missing.yaml"))
	require.Error(t, err)
}

func TestCrowdsec_AcquisitionEndpoints_InvalidConfiguredPath(t *testing.T) {
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", "relative/path.yaml")

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	wGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/acquisition", http.NoBody)
	r.ServeHTTP(wGet, reqGet)
	require.Equal(t, http.StatusInternalServerError, wGet.Code)

	wPut := httptest.NewRecorder()
	reqPut := httptest.NewRequest(http.MethodPut, "/api/v1/admin/crowdsec/acquisition", bytes.NewBufferString(`{"content":"source: file"}`))
	reqPut.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wPut, reqPut)
	require.Equal(t, http.StatusInternalServerError, wPut.Code)
}

func TestCrowdsec_GetBouncerKey_NotConfigured(t *testing.T) {
	t.Setenv("CROWDSEC_API_KEY", "")
	t.Setenv("CROWDSEC_BOUNCER_API_KEY", "")
	t.Setenv("CERBERUS_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CPM_SECURITY_CROWDSEC_API_KEY", "")

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/bouncer/key", http.NoBody)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}
