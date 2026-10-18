package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// The engine config (config.yaml and config.yaml.local) holds the database credentials of an external database,
// the registration token and URL passwords. The export blanks them; an import on the same host puts them back
// from the config it replaces, and on another host reports the file.

func crowdsecConfigFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "crowdsec", "testdata", "config", name)) //nolint:gosec // G304: fixed testdata directory
	require.NoError(t, err)
	return string(raw)
}

func exportTreeWith(t *testing.T, dir string, files map[string]string) (*gin.Engine, *httptest.ResponseRecorder) {
	t.Helper()
	seedRealLayout(t, dir)
	writeTree(t, dir, files)
	_, r := newImportRouter(t, dir)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody))
	return r, w
}

func parsedMap(t *testing.T, text string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(text), &m), text)
	return m
}

func omittedHeader(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	raw := w.Header().Get("X-Charon-Omitted-Files")
	if raw == "" {
		return nil
	}
	var names []string
	require.NoError(t, json.Unmarshal([]byte(raw), &names))
	return names
}

func entriesOf(t *testing.T, archive []byte) map[string]string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(archive))
	require.NoError(t, err)
	tr := tar.NewReader(gr)
	out := map[string]string{}
	for {
		hdr, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			return out
		}
		require.NoError(t, nextErr)
		body, readErr := io.ReadAll(tr)
		require.NoError(t, readErr)
		out[filepath.ToSlash(hdr.Name)] = string(body)
	}
}

func hasName(names []string, base string) bool {
	for _, n := range names {
		if n == base || strings.HasSuffix(n, "/"+base) {
			return true
		}
	}
	return false
}

func TestExportConfigRouteBlanksDatabaseCredentials(t *testing.T) {
	t.Parallel()
	for _, fixture := range []string{"mysql.yaml", "postgres.yaml"} {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			original := crowdsecConfigFixture(t, fixture)
			_, w := exportTreeWith(t, t.TempDir(), map[string]string{"config/config.yaml": original})
			require.Equal(t, http.StatusOK, w.Code)

			entries := entriesOf(t, w.Body.Bytes())

			for name, body := range entries {
				assert.NotContains(t, body, "FAKE-", name)
			}
			cfg := parsedMap(t, entries["config/config.yaml"])
			db := cfg["db_config"].(map[string]any)
			assert.Equal(t, crowdsec.RedactedPlaceholder, db["user"])
			assert.Equal(t, crowdsec.RedactedPlaceholder, db["password"])
			assert.Equal(t, "192.0.2.50", db["host"])
		})
	}
}

func TestExportConfigRouteKeepsAConfigWithoutSecretsAsWritten(t *testing.T) {
	t.Parallel()
	original := crowdsecConfigFixture(t, "dist.yaml")
	_, w := exportTreeWith(t, t.TempDir(), map[string]string{"config/config.yaml": original})
	require.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, original, entriesOf(t, w.Body.Bytes())["config/config.yaml"])
	assert.Empty(t, omittedHeader(t, w))
}

func TestExportConfigRouteRefusesAConfigItCouldNotRedact(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"anchors_alias.yaml":       crowdsecConfigFixture(t, "anchors_alias.yaml"),
		"anchors_alias_value.yaml": crowdsecConfigFixture(t, "anchors_alias_value.yaml"),
		"unparsable":               "api: [unclosed\ndb_config:\n  password: FAKE-broken-0001\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, w := exportTreeWith(t, t.TempDir(), map[string]string{"config/config.yaml": content})

			require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, "export_config_unsafe", body["error_code"])
			assert.NotEmpty(t, body["error"])
			assert.NotContains(t, w.Body.String(), "FAKE-")
			assert.Empty(t, omittedHeader(t, w))
		})
	}
}

// On the same host the import puts the blanked values back from the config it replaces.
func TestImportConfigRestoresBlankedConfigValuesOnTheSameHost(t *testing.T) {
	t.Parallel()
	cases := map[string]map[string]string{
		"database credentials": {"config/config.yaml": crowdsecConfigFixture(t, "mysql.yaml")},
		"registration token":   {"config/config.yaml": crowdsecConfigFixture(t, "auto_registration_token.yaml")},
		"url password":         {"config/config.yaml": crowdsecConfigFixture(t, "url_userinfo.yaml")},
		"local override": {
			"config/config.yaml":       crowdsecConfigFixture(t, "dist.yaml"),
			"config/config.yaml.local": "db_config:\n  type: mysql\n  user: fakeuser\n  password: FAKE-local-0014\n",
		},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			r, w := exportTreeWith(t, dir, files)
			require.Equal(t, http.StatusOK, w.Code)

			imp := httptest.NewRecorder()
			r.ServeHTTP(imp, importBytesRequest(t, w.Body.Bytes()))

			require.Equal(t, http.StatusOK, imp.Code, imp.Body.String())
			for rel, original := range files {
				assert.Equal(t, parsedMap(t, original), parsedMap(t, readTreeFile(t, filepath.Join(dir, filepath.FromSlash(rel)))), rel)
			}
			names, _ := needsValuesOf(t, imp)
			assert.False(t, hasName(names, "config.yaml") || hasName(names, "config.yaml.local"), "nothing is left to supply: %v", names)
		})
	}
}

// On another host there is nothing to restore from: the placeholders stay and the files are reported.
func TestImportConfigReportsBlankedConfigValuesOnAnotherHost(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		files map[string]string
		want  string
	}{
		"config": {map[string]string{"config/config.yaml": crowdsecConfigFixture(t, "mysql.yaml")}, "config.yaml"},
		"local override": {map[string]string{
			"config/config.yaml":       crowdsecConfigFixture(t, "dist.yaml"),
			"config/config.yaml.local": "db_config:\n  type: mysql\n  user: fakeuser\n  password: FAKE-local-0014\n",
		}, "config.yaml.local"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, exported := exportTreeWith(t, t.TempDir(), tc.files)
			require.Equal(t, http.StatusOK, exported.Code)
			target := t.TempDir()
			seedRealLayout(t, target)
			_, targetRouter := newImportRouter(t, target)

			imp := httptest.NewRecorder()
			targetRouter.ServeHTTP(imp, importBytesRequest(t, exported.Body.Bytes()))

			require.Equal(t, http.StatusOK, imp.Code, imp.Body.String())
			names, present := needsValuesOf(t, imp)
			require.True(t, present)
			assert.True(t, hasName(names, tc.want), "%q is not reported: %v", tc.want, names)
			got := readTreeFile(t, filepath.Join(target, filepath.FromSlash("config/"+tc.want)))
			assert.Contains(t, got, crowdsec.RedactedPlaceholder)
			assert.NotContains(t, got, "FAKE-")
		})
	}
}
