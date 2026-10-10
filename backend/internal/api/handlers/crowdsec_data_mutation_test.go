package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const importableConfig = "api:\n  server:\n    listen_uri: 0.0.0.0:8080\n"

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
}

func readTreeFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // G304: test fixture path
	require.NoError(t, err)
	return string(b)
}

func importRequest(t *testing.T, files map[string]string) *http.Request {
	t.Helper()
	archivePath := createTestArchive(t, "tar.gz", files, true)
	data, err := os.ReadFile(archivePath) //nolint:gosec // G304: archive in the test temp dir
	require.NoError(t, err)
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

func newImportRouter(t *testing.T, dataDir string) (*CrowdsecHandler, *gin.Engine) {
	t.Helper()
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dataDir)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return h, r
}

func seedEngineState(t *testing.T, dir string) {
	t.Helper()
	writeTree(t, dir, map[string]string{
		"config.yaml":              "old: true\n",
		"old-only.yaml":            "old",
		"crowdsec.db":              "live-db",
		"crowdsec.db-wal":          "wal",
		"data/crowdsec.db":         "lapi-db",
		"data/GeoLite2-City.mmdb":  "mmdb",
		"hub_cache/x/bundle.tgz":   "archive",
		"nested/data/keep.yaml":    "nested",
		"nested/hub/collection.md": "doc",
	})
}

func assertEngineStateIntact(t *testing.T, dir string) {
	t.Helper()
	require.Equal(t, "live-db", readTreeFile(t, filepath.Join(dir, "crowdsec.db")))
	require.Equal(t, "wal", readTreeFile(t, filepath.Join(dir, "crowdsec.db-wal")))
	require.Equal(t, "lapi-db", readTreeFile(t, filepath.Join(dir, "data", "crowdsec.db")))
	require.Equal(t, "mmdb", readTreeFile(t, filepath.Join(dir, "data", "GeoLite2-City.mmdb")))
	require.Equal(t, "archive", readTreeFile(t, filepath.Join(dir, "hub_cache", "x", "bundle.tgz")))
}

func TestImportConfigReplacesConfigButPreservesEngineStateAndDataDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	before, err := os.Stat(dir)
	require.NoError(t, err)
	_, r := newImportRouter(t, dir)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{
		"config.yaml":      importableConfig,
		"new.yaml":         "new: true",
		"crowdsec.db":      "evil-db",
		"data/x":           "evil",
		"data/crowdsec.db": "evil-lapi",
		"hub_cache/y":      "evil",
	}))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	backupName, _ := resp["backup"].(string)
	require.True(t, strings.HasPrefix(backupName, filepath.Base(dir)+".backup."), backupName)
	require.Equal(t, filepath.Base(backupName), backupName, "response carries a name, not a path")
	backup := filepath.Join(filepath.Dir(dir), backupName)

	after, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "DataDir must never be renamed")

	require.Equal(t, importableConfig, readTreeFile(t, filepath.Join(dir, "config.yaml")))
	require.Equal(t, "new: true", readTreeFile(t, filepath.Join(dir, "new.yaml")))
	require.NoFileExists(t, filepath.Join(dir, "old-only.yaml"))
	require.NoDirExists(t, filepath.Join(dir, "nested"))

	// Uploaded engine-owned entries were not extracted; live state is untouched.
	assertEngineStateIntact(t, dir)
	require.NoFileExists(t, filepath.Join(dir, "data", "x"))
	require.NoFileExists(t, filepath.Join(dir, "hub_cache", "y"))

	// The snapshot holds the previous config, including a nested dir named data, but no engine state.
	require.Equal(t, "old: true\n", readTreeFile(t, filepath.Join(backup, "config.yaml")))
	require.Equal(t, "nested", readTreeFile(t, filepath.Join(backup, "nested", "data", "keep.yaml")))
	require.NoFileExists(t, filepath.Join(backup, "crowdsec.db"))
	require.NoDirExists(t, filepath.Join(backup, "data"))
	require.NoDirExists(t, filepath.Join(backup, "hub_cache"))
}

func TestImportConfigValidationFailureRestoresPreviousConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	before, err := os.Stat(dir)
	require.NoError(t, err)
	_, r := newImportRouter(t, dir)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{"config.yaml": "{{ not: [valid yaml", "extra.yaml": "x"}))
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())

	after, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after))
	require.Equal(t, "old: true\n", readTreeFile(t, filepath.Join(dir, "config.yaml")))
	require.Equal(t, "old", readTreeFile(t, filepath.Join(dir, "old-only.yaml")))
	require.Equal(t, "nested", readTreeFile(t, filepath.Join(dir, "nested", "data", "keep.yaml")))
	require.NoFileExists(t, filepath.Join(dir, "extra.yaml"))
	assertEngineStateIntact(t, dir)
}

func TestImportConfigExtractionFailureRestoresPreviousConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	_, r := newImportRouter(t, dir)

	// "a" is both a file and a directory prefix, so extraction fails part-way.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{"config.yaml": importableConfig, "a": "file", "a/b": "nested"}))
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

	require.Equal(t, "old: true\n", readTreeFile(t, filepath.Join(dir, "config.yaml")))
	require.Equal(t, "old", readTreeFile(t, filepath.Join(dir, "old-only.yaml")))
	require.NoFileExists(t, filepath.Join(dir, "a"))
	assertEngineStateIntact(t, dir)
}

func TestImportConfigSnapshotFailureLeavesNothingChanged(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	_, r := newImportRouter(t, filepath.Join(blocker, "crowdsec"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{"config.yaml": importableConfig}))
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "failed to create backup")
}

func TestImportConfigPrunesSnapshotsToFive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	_, r := newImportRouter(t, dir)

	for i := 0; i < 7; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, importRequest(t, map[string]string{"config.yaml": importableConfig}))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		time.Sleep(2 * time.Millisecond) // distinct microsecond timestamps even on coarse clocks
	}
	matches, err := filepath.Glob(dir + ".backup.*")
	require.NoError(t, err)
	require.Len(t, matches, 5)
}

func TestImportConfigPrunesAfterFailureKeepingNewestSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	for i := 1; i <= 7; i++ {
		require.NoError(t, os.Mkdir(fmt.Sprintf("%s.backup.2025010%d-000000.000000", dir, i), 0o700))
	}
	_, r := newImportRouter(t, dir)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{"config.yaml": "{{ bad", "x": "y"}))
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	matches, err := filepath.Glob(dir + ".backup.*")
	require.NoError(t, err)
	require.Len(t, matches, 5)
	for _, m := range matches {
		require.NotContains(t, []string{
			dir + ".backup.20250101-000000.000000",
			dir + ".backup.20250102-000000.000000",
			dir + ".backup.20250103-000000.000000",
		}, m)
	}
}

// TestDataMutationsSerializeOnHandlerLock holds the handler lock and proves file write and
// preset apply wait for it (import is covered separately).
func TestDataMutationsSerializeOnHandlerLock(t *testing.T) {
	c := newCuratedHarness(t, nil)

	requests := map[string]func() *httptest.ResponseRecorder{
		"apply": func() *httptest.ResponseRecorder {
			body, _ := json.Marshal(map[string]string{"slug": "honeypot-friendly-defaults"})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			c.router.ServeHTTP(w, req)
			return w
		},
		"write": func() *httptest.ResponseRecorder {
			body, _ := json.Marshal(map[string]string{"path": "locked.conf", "content": "x"})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			c.router.ServeHTTP(w, req)
			return w
		},
	}

	for name, do := range requests {
		c.handler.dataMu.Lock()
		done := make(chan *httptest.ResponseRecorder, 1)
		go func() { done <- do() }()

		select {
		case <-done:
			c.handler.dataMu.Unlock()
			t.Fatalf("%s ran while the handler lock was held", name)
		case <-time.After(150 * time.Millisecond):
		}
		c.handler.dataMu.Unlock()

		select {
		case w := <-done:
			require.Equal(t, http.StatusOK, w.Code, "%s: %s", name, w.Body.String())
		case <-time.After(5 * time.Second):
			t.Fatalf("%s did not complete after the lock was released", name)
		}
	}
}

func TestImportConfigWaitsForHandlerLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	h, r := newImportRouter(t, dir)

	h.dataMu.Lock()
	done := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, importRequest(t, map[string]string{"config.yaml": importableConfig}))
		done <- w.Code
	}()
	select {
	case <-done:
		h.dataMu.Unlock()
		t.Fatal("import ran while the handler lock was held")
	case <-time.After(150 * time.Millisecond):
	}
	require.Equal(t, "old: true\n", readTreeFile(t, filepath.Join(dir, "config.yaml")), "nothing changed while locked")
	h.dataMu.Unlock()

	select {
	case code := <-done:
		require.Equal(t, http.StatusOK, code)
	case <-time.After(5 * time.Second):
		t.Fatal("import did not finish after the lock was released")
	}
}

func TestCuratedApplyPrunesSnapshotsToFive(t *testing.T) {
	c := newCuratedHarness(t, nil)
	for i := 0; i < 7; i++ {
		code, _ := c.apply(t, "honeypot-friendly-defaults")
		require.Equal(t, http.StatusOK, code)
		time.Sleep(2 * time.Millisecond)
	}
	matches, err := filepath.Glob(c.dataDir + ".backup.*")
	require.NoError(t, err)
	require.Len(t, matches, 5)
}

func TestImportConfigClearFailureRollsBackAndReportsBothFailures(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	// A read-only config directory can be snapshotted but not emptied, so both the clear and the
	// rollback that follows it fail; the snapshot is retained.
	locked := filepath.Join(dir, "locked")
	writeTree(t, locked, map[string]string{"f.yaml": "x"})
	require.NoError(t, os.Chmod(locked, 0o500))       // #nosec G302 -- test fixture
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) }) // #nosec G302 -- test fixture
	_, r := newImportRouter(t, dir)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{"config.yaml": importableConfig}))
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "failed to create config dir")

	matches, err := filepath.Glob(dir + ".backup.*")
	require.NoError(t, err)
	require.Len(t, matches, 1, "snapshot kept for manual recovery")
	require.Equal(t, "x", readTreeFile(t, filepath.Join(matches[0], "locked", "f.yaml")))
	assertEngineStateIntact(t, dir)
}

func TestPruneSnapshotsLogsAndToleratesFailure(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	h, _ := newImportRouter(t, filepath.Join(blocker, "crowdsec"))
	// The parent of DataDir is a regular file, so listing snapshots fails; pruning is best effort.
	require.NotPanics(t, h.pruneSnapshots)
}
