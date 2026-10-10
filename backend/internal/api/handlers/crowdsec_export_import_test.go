package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// seedRealLayout builds the on-disk layout of a real install (config/ holds the configuration).
func seedRealLayout(t *testing.T, dir string) {
	t.Helper()
	writeTree(t, dir, map[string]string{
		"config/config.yaml":                    importableConfig,
		"config/acquis.yaml":                    "filenames: [x]\n",
		"config/local_api_credentials.yaml":     "live-lapi",
		"config/online_api_credentials.yaml":    "live-capi",
		"config/hub/index.json":                 "{}",
		"config/notes.yaml.bak":                 "user copy",
		"bouncer_key":                           "live-key",
		"bouncer_key.bak":                       "near-miss",
		"caddy.yaml":                            "misc",
		"crowdsec.db":                           "live-db",
		"data/crowdsec.db":                      "lapi-db",
		"data/GeoLite2-City.mmdb":               "mmdb",
		"hub_cache/x/bundle.tgz":                "archive",
		"nested/data/keep.yaml":                 "nested",
		"nested/crowdsec.db-wal":                "nested-wal",
		"config/collections/placeholder.yaml":   "real file",
		"config/sub/Local_API_Credentials.YAML": "mixed-case",
	})
	require.NoError(t, os.Symlink("../hub/index.json", filepath.Join(dir, "config", "collections", "linked.yaml")))
}

func exportBytes(t *testing.T, r *gin.Engine) []byte {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)
	return w.Body.Bytes()
}

func archiveNames(t *testing.T, data []byte) []string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	tr := tar.NewReader(gr)
	var names []string
	for {
		hdr, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		require.NoError(t, nextErr)
		names = append(names, filepath.ToSlash(hdr.Name))
	}
	sort.Strings(names)
	return names
}

func importBytesRequest(t *testing.T, data []byte) *http.Request {
	t.Helper()
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	fw, err := mw.CreateFormFile("file", "cfg.tar.gz")
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/import", buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func assertLiveSecretsIntact(t *testing.T, dir string) {
	t.Helper()
	require.Equal(t, "live-lapi", readTreeFile(t, filepath.Join(dir, "config", "local_api_credentials.yaml")))
	require.Equal(t, "live-capi", readTreeFile(t, filepath.Join(dir, "config", "online_api_credentials.yaml")))
	require.Equal(t, "live-key", readTreeFile(t, filepath.Join(dir, "bouncer_key")))
}

func TestExportConfigRealLayoutEntryList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	_, r := newImportRouter(t, dir)

	require.Equal(t, []string{
		"bouncer_key.bak",
		"caddy.yaml",
		"config/acquis.yaml",
		"config/collections/placeholder.yaml",
		"config/config.yaml",
		"config/hub/index.json",
		"config/notes.yaml.bak",
		"nested/data/keep.yaml",
	}, archiveNames(t, exportBytes(t, r)))
}

func TestExportConfigSkippedSecretKeepsSiblings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"a.yaml":                            "a",
		"bouncer_key":                       "k",
		"z.yaml":                            "z",
		"config/aaa.yaml":                   "a",
		"config/local_api_credentials.yaml": "s",
		"config/zzz.yaml":                   "z",
		"crowdsec.db":                       "db",
		"zz.yaml":                           "zz",
	})
	_, r := newImportRouter(t, dir)

	require.Equal(t, []string{"a.yaml", "config/aaa.yaml", "config/zzz.yaml", "z.yaml", "zz.yaml"},
		archiveNames(t, exportBytes(t, r)))
}

func TestExportThenImportRealLayoutRoundTrip(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	seedRealLayout(t, src)
	_, srcRouter := newImportRouter(t, src)
	exported := exportBytes(t, srcRouter)

	// A different server with its own account details, database and a hub link.
	dst := t.TempDir()
	seedRealLayout(t, dst)
	writeTree(t, dst, map[string]string{
		"config/config.yaml":                 "api:\n  server:\n    listen_uri: 127.0.0.1:1\n",
		"config/extra.yaml":                  "gone after import",
		"config/local_api_credentials.yaml":  "live-lapi",
		"config/online_api_credentials.yaml": "live-capi",
	})
	_, dstRouter := newImportRouter(t, dst)

	w := httptest.NewRecorder()
	dstRouter.ServeHTTP(w, importBytesRequest(t, exported))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	require.Equal(t, importableConfig, readTreeFile(t, filepath.Join(dst, "config", "config.yaml")))
	require.Equal(t, "filenames: [x]\n", readTreeFile(t, filepath.Join(dst, "config", "acquis.yaml")))
	require.NoFileExists(t, filepath.Join(dst, "config", "extra.yaml"))
	assertLiveSecretsIntact(t, dst)
	require.Equal(t, "live-db", readTreeFile(t, filepath.Join(dst, "crowdsec.db")))
	require.Equal(t, "lapi-db", readTreeFile(t, filepath.Join(dst, "data", "crowdsec.db")))
	require.Equal(t, "archive", readTreeFile(t, filepath.Join(dst, "hub_cache", "x", "bundle.tgz")))

	// Known limitation: installed hub item links are not part of an export, and import removes them.
	_, err := os.Lstat(filepath.Join(dst, "config", "collections", "linked.yaml"))
	require.True(t, errors.Is(err, os.ErrNotExist), "hub item symlinks are not restored")
}

func TestImportConfigLayouts(t *testing.T) {
	t.Parallel()
	cases := map[string]map[string]string{
		"config only":    {"config/config.yaml": importableConfig},
		"root layout":    {"config.yaml": importableConfig, "other.yaml": "o"},
		"real and extra": {"config/config.yaml": importableConfig, "config/acquis.yaml": "a"},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			seedRealLayout(t, dir)
			_, r := newImportRouter(t, dir)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, importRequest(t, files))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assertLiveSecretsIntact(t, dir)
			for rel, content := range files {
				require.Equal(t, content, readTreeFile(t, filepath.Join(dir, rel)))
			}
		})
	}
}

func TestImportConfigNeverTakesProtectedEntriesFromUpload(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	_, r := newImportRouter(t, dir)

	// A legacy export carried these entries; they must not replace this server's values.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{
		"config/config.yaml":                 importableConfig,
		"config/local_api_credentials.yaml":  "uploaded",
		"config/online_api_credentials.yaml": "uploaded",
		"bouncer_key":                        "uploaded",
		"config/Online_API_Credentials.yaml": "uploaded-mixed",
		"config/bouncer_key/inner.yaml":      "uploaded-dir",
		"config/sibling.yaml":                "sibling",
	}))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assertLiveSecretsIntact(t, dir)
	require.Equal(t, "sibling", readTreeFile(t, filepath.Join(dir, "config", "sibling.yaml")))
	require.NoDirExists(t, filepath.Join(dir, "config", "bouncer_key"))
}

func TestImportConfigKeepsSymlinkedSecret(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	store := filepath.Join(t.TempDir(), "docker-secret")
	require.NoError(t, os.WriteFile(store, []byte("from-store"), 0o600))
	link := filepath.Join(dir, "config", "local_api_credentials.yaml")
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(store, link))
	_, r := newImportRouter(t, dir)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{
		"config/config.yaml":                importableConfig,
		"config/local_api_credentials.yaml": "uploaded",
	}))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	target, err := os.Readlink(link)
	require.NoError(t, err)
	require.Equal(t, store, target)
	require.Equal(t, "from-store", readTreeFile(t, store))
}

func TestImportConfigFailureKeepsSecretsAndRestoresConfig(t *testing.T) {
	t.Parallel()
	t.Run("invalid config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		seedRealLayout(t, dir)
		_, r := newImportRouter(t, dir)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, importRequest(t, map[string]string{
			"config/config.yaml":                "{{ not: [valid yaml",
			"config/local_api_credentials.yaml": "uploaded",
		}))
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
		require.Equal(t, importableConfig, readTreeFile(t, filepath.Join(dir, "config", "config.yaml")))
		require.Equal(t, "filenames: [x]\n", readTreeFile(t, filepath.Join(dir, "config", "acquis.yaml")))
		assertLiveSecretsIntact(t, dir)
	})
	t.Run("config only in a nested directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		seedRealLayout(t, dir)
		_, r := newImportRouter(t, dir)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, importRequest(t, map[string]string{"foo/config.yaml": importableConfig}))
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), "config.yaml was not found at the top level or in config/")
		require.Equal(t, importableConfig, readTreeFile(t, filepath.Join(dir, "config", "config.yaml")))
		assertLiveSecretsIntact(t, dir)
	})
	t.Run("extraction failure", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		seedRealLayout(t, dir)
		_, r := newImportRouter(t, dir)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, importRequest(t, map[string]string{"config/config.yaml": importableConfig, "a": "file", "a/b": "nested"}))
		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assertLiveSecretsIntact(t, dir)
		require.Equal(t, "filenames: [x]\n", readTreeFile(t, filepath.Join(dir, "config", "acquis.yaml")))
	})
}

// recordingCmdExec captures the arguments passed to the diagnostics commands.
type recordingCmdExec struct{ calls [][]string }

func (r *recordingCmdExec) Execute(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, args)
	return nil, nil
}

func TestDiagnosticsConfigLocatesConfigFile(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, files map[string]string) (map[string]any, *recordingCmdExec) {
		t.Helper()
		dir := t.TempDir()
		writeTree(t, dir, files)
		h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dir)
		rec := &recordingCmdExec{}
		h.CmdExec = rec
		r := gin.New()
		r.GET("/d", h.DiagnosticsConfig)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/d", http.NoBody))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var out map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out, rec
	}

	t.Run("config dir layout", func(t *testing.T) {
		t.Parallel()
		out, rec := run(t, map[string]string{"config/config.yaml": importableConfig})
		require.Equal(t, true, out["config_exists"])
		require.NotEmpty(t, rec.calls)
	})
	t.Run("root layout", func(t *testing.T) {
		t.Parallel()
		out, _ := run(t, map[string]string{"config.yaml": importableConfig})
		require.Equal(t, true, out["config_exists"])
	})
	t.Run("no config reports not found without running checks", func(t *testing.T) {
		t.Parallel()
		out, rec := run(t, map[string]string{"config/acquis.yaml": "x"})
		require.Equal(t, false, out["config_exists"])
		require.Contains(t, out["errors"], "config.yaml not found")
		require.Empty(t, rec.calls)
	})
}

func TestDiagnosticsConnectivityPassesLocatedConfigToCscli(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"config dir": {"config/config.yaml": importableConfig, "config/online_api_credentials.yaml": "x"},
		"root":       {"config.yaml": importableConfig, "config/online_api_credentials.yaml": "x"},
		"none":       {"config/online_api_credentials.yaml": "x"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTree(t, dir, files)
			h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{started: true}, "/bin/false", dir)
			rec := &recordingCmdExec{}
			h.CmdExec = rec
			r := gin.New()
			r.GET("/c", h.DiagnosticsConnectivity)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/c", http.NoBody))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			want := crowdsec.FindConfigFile(dir)
			require.Len(t, rec.calls, 2, "lapi status and capi status")
			for _, args := range rec.calls {
				if want == "" {
					require.NotContains(t, args, "-c")
				} else {
					require.Equal(t, []string{"-c", want}, args[:2])
				}
			}
		})
	}
}

func TestEditorHidesSecretsUsingSharedRule(t *testing.T) {
	t.Parallel()
	for rel, readable := range map[string]bool{
		"bouncer_key":                          false,
		"config/Local_API_Credentials.yaml":    false,
		"config/online_api_credentials.yaml":   false,
		"bouncer_key/inner.yaml":               false,
		"bouncer_key.bak":                      true,
		"config/my_local_api_credentials.yaml": true,
		"config/config.yaml":                   true,
	} {
		require.Equal(t, readable, isReadable(rel), rel)
	}
	require.False(t, isWritable("config/online_api_credentials.yaml"))
}
