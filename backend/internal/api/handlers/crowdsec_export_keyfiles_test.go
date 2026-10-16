package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const keyFilesHeader = "X-Charon-Omitted-Files"

func exportWithKeyFiles(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"config/config.yaml": importableConfig,
		"config/notifications/http_default.yaml": "type: http\nname: http_default\nurl: https://hooks.example.invalid/n\n" +
			"cert_path: /etc/crowdsec/ssl/client.crt\nkey_path: /etc/crowdsec/ssl/client.pem\n",
		"config/ssl/client.crt": "certificate\n",
		"config/ssl/client.pem": "opaque private material\n",
		"config/ssl/other.key":  "opaque\n",
	})
	_, r := newImportRouter(t, dir)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)
	return w
}

func TestExportConfigListsTheFilesItLeftOut(t *testing.T) {
	t.Parallel()

	w := exportWithKeyFiles(t)

	var omitted []string
	require.NoError(t, json.Unmarshal([]byte(w.Header().Get(keyFilesHeader)), &omitted))
	require.Equal(t, []string{"config/ssl/client.pem", "config/ssl/other.key"}, omitted)
	require.NotContains(t, archiveNames(t, w.Body.Bytes()), "config/ssl/client.pem")
	require.Contains(t, archiveNames(t, w.Body.Bytes()), "config/ssl/client.crt")
}

func TestExportConfigWithoutKeyFilesSendsNoList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	_, r := newImportRouter(t, dir)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody))

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Header().Values(keyFilesHeader))
}

func TestOmittedFilesHeader(t *testing.T) {
	t.Parallel()
	require.Empty(t, omittedFilesHeader(nil))
	require.Equal(t, `["a b","c\"d"]`, omittedFilesHeader([]string{"a b", `c"d`}))

	many := make([]string, maxOmittedFilesListed+10)
	for i := range many {
		many[i] = "f"
	}
	var listed []string
	require.NoError(t, json.Unmarshal([]byte(omittedFilesHeader(many)), &listed))
	require.Len(t, listed, maxOmittedFilesListed)
}
