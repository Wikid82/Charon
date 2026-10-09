package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// serverPathPattern matches absolute server path prefixes that must never reach a client.
var serverPathPattern = regexp.MustCompile(`(^|[\s"'(=:])/(app|data|etc|tmp|var|home)(/|\b)`)

// requireNoServerPaths fails when body leaks an absolute server path or any of the extra
// test-specific directories (typically t.TempDir()).
func requireNoServerPaths(t *testing.T, body string, extra ...string) {
	t.Helper()
	require.Falsef(t, serverPathPattern.MatchString(body), "response leaks a server path: %s", body)
	for _, e := range extra {
		require.NotContainsf(t, body, e, "response leaks %s: %s", e, body)
	}
}

func TestResponses_FileWriteCarriesBackupNameOnly(t *testing.T) {
	f := newFilesFixture(t)
	f.put("config/config.yaml", "v: 0\n")

	w := f.write("config/config.yaml", "v: 1\n")
	require.Equal(t, http.StatusOK, w.Code)
	requireNoServerPaths(t, w.Body.String(), f.root)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Regexp(t, `^crowdsec\.filebackup\.\d{8}-\d{6}\.\d{6}$`, resp["backup"])
}

func TestResponses_ImportSuccessAndFailuresCarryNoPaths(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	seedEngineState(t, dir)
	root := filepath.Dir(dir)
	_, r := newImportRouter(t, dir)

	cases := []struct {
		name  string
		files map[string]string
		code  int
	}{
		{"success", map[string]string{"config.yaml": importableConfig}, http.StatusOK},
		{"extraction failure", map[string]string{"config.yaml": importableConfig, "a": "file", "a/b": "nested"}, http.StatusInternalServerError},
		{"validation failure", map[string]string{"other.yaml": "x"}, http.StatusUnprocessableEntity},
		{"config validation failure", map[string]string{"config.yaml": "not: valid"}, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, importRequest(t, tc.files))
			require.Equal(t, tc.code, w.Code, w.Body.String())
			requireNoServerPaths(t, w.Body.String(), root)
			if tc.code == http.StatusInternalServerError {
				require.JSONEq(t, `{"error":"extraction failed"}`, w.Body.String())
			}
		})
	}
}

func TestResponses_CuratedApplyCarriesNoPaths(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c := newCuratedHarness(t, nil)
		code, resp := c.apply(t, "honeypot-friendly-defaults")
		require.Equal(t, http.StatusOK, code)
		body, _ := json.Marshal(resp)
		requireNoServerPaths(t, string(body), filepath.Dir(c.dataDir))
		require.Regexp(t, `\.backup\.\d{8}-\d{6}\.\d{6}$`, resp["backup"])
	})

	t.Run("rollback failure", func(t *testing.T) {
		var c *curatedHarness
		c = newCuratedHarness(t, func(_ context.Context, cmd string) ([]byte, error) {
			if strings.Contains(cmd, " install ") {
				// Make the restore fail too: DataDir becomes a regular file mid-apply.
				require.NoError(t, os.RemoveAll(c.dataDir))
				require.NoError(t, os.WriteFile(c.dataDir, []byte("not a dir"), 0o600))
				return nil, context.DeadlineExceeded
			}
			return nil, nil
		})
		code, resp := c.apply(t, "geoip-enrichment")
		require.GreaterOrEqual(t, code, http.StatusInternalServerError)
		body, _ := json.Marshal(resp)
		require.Contains(t, string(body), "see server logs")
		requireNoServerPaths(t, string(body), filepath.Dir(c.dataDir))
	})
}

func TestResponses_AcquisitionUpdateCarriesBackupNameOnly(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("old: true\n"), 0o600))
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	r.PUT("/acquisition", h.UpdateAcquisitionConfig)

	body, _ := json.Marshal(map[string]string{"content": "source: file\n"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/acquisition", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireNoServerPaths(t, w.Body.String(), dir)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Regexp(t, `^acquis\.yaml\.backup\.\d{8}-\d{6}$`, resp["backup"])
	require.FileExists(t, filepath.Join(dir, resp["backup"].(string)))
}

func TestResponses_ExportErrorCarriesNoPaths(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	dir := t.TempDir()
	secret := filepath.Join(dir, "unreadable.yaml")
	require.NoError(t, os.WriteFile(secret, []byte("x"), 0o000))
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dir)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	requireNoServerPaths(t, w.Body.String(), dir)
	require.Contains(t, w.Body.String(), "failed to export crowdsec config")
}
