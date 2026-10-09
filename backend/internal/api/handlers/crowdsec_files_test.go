package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type filesFixture struct {
	t      *testing.T
	root   string // parent of the data dir, where backup dirs are created
	dir    string // the handler's DataDir
	router *gin.Engine
}

func newFilesFixture(t *testing.T) *filesFixture {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "crowdsec")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	h := newTestCrowdsecHandler(t, setupCrowdDB(t), &fakeExec{}, "/bin/false", dir)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return &filesFixture{t: t, root: root, dir: dir, router: r}
}

func (f *filesFixture) put(rel, content string) string {
	f.t.Helper()
	p := filepath.Join(f.dir, rel)
	require.NoError(f.t, os.MkdirAll(filepath.Dir(p), 0o750))
	require.NoError(f.t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func (f *filesFixture) read(rel string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/file?path="+url.QueryEscape(rel), http.NoBody)
	f.router.ServeHTTP(w, req)
	return w
}

func (f *filesFixture) list() []string {
	f.t.Helper()
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/files", http.NoBody))
	require.Equal(f.t, http.StatusOK, w.Code)
	var resp struct {
		Files []string `json:"files"`
	}
	require.NoError(f.t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp.Files
}

func (f *filesFixture) writeRaw(body []byte) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	f.router.ServeHTTP(w, req)
	return w
}

func (f *filesFixture) write(rel, content string) *httptest.ResponseRecorder {
	body, err := json.Marshal(map[string]string{"path": rel, "content": content})
	require.NoError(f.t, err)
	return f.writeRaw(body)
}

func (f *filesFixture) glob(kind string) []string {
	m, err := filepath.Glob(filepath.Join(f.root, "crowdsec."+kind+".*"))
	require.NoError(f.t, err)
	return m
}

func errorOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["error"]
}

func TestCrowdsecFiles_Predicates(t *testing.T) {
	t.Parallel()
	readable := map[string]bool{
		"config.yaml":                 true,
		"config/config.yaml":          true,
		"config/scenarios/x.yaml":     true,
		"config/acquis.d/a.conf":      true,
		"notes.md":                    true,
		"config/data/nested.yaml":     true, // only the top-level data/ is protected
		"bouncer_key":                 false,
		"config/bouncer_key":          false,
		"crowdsec.db":                 false,
		"config/crowdsec.db-wal":      false,
		"data/crowdsec.db":            false,
		"data/GeoLite2-City.mmdb":     false,
		"hub_cache/a.tgz":             false,
		"config/other.db":             false,
		"config/other.db-journal":     false,
		"config/CASE.DB":              false,
		"data":                        false,
		"hub_cache":                   false,
		"config/hub_cache/thing.yaml": true,
	}
	for rel, want := range readable {
		assert.Equal(t, want, isReadable(rel), "isReadable(%q)", rel)
	}

	writable := map[string]bool{
		"config.yaml":                         true,
		"config/config.yaml":                  true,
		"a.yml":                               true,
		"a.json":                              true,
		"a.txt":                               true,
		"a.conf":                              true,
		"A.YAML":                              true,
		"notes.md":                            false,
		"script.sh":                           false,
		"noextension":                         false,
		"config/local_api_credentials.yaml":   false,
		"config/online_api_credentials.yaml":  false,
		"bouncer_key":                         false,
		"data/x.json":                         false,
		"hub_cache/x.yaml":                    false,
		"config/hub/parsers/s01-parse/x.yaml": false,
		"hub/.index.json":                     false,
		"config/x.db":                         false,
		"config/.charon-write-123456789.yaml": false,
		"config/hub_cache/thing.yaml":         true,
		"config/data/nested.yaml":             true,
		"config/parsers/s01-parse/my-parser.yaml": true,
	}
	for rel, want := range writable {
		assert.Equal(t, want, isWritable(rel), "isWritable(%q)", rel)
		if want {
			assert.True(t, isReadable(rel), "writable implies readable: %q", rel)
		}
	}
}

func TestCrowdsecFiles_CleanRelPath(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"../x.yaml", "a/../../x.yaml", "/etc/passwd", "a\x00b.yaml", ".", "./", ".."} {
		_, ferr := cleanRelPath(raw)
		require.NotNil(t, ferr, raw)
		assert.Equal(t, http.StatusBadRequest, ferr.status, raw)
		assert.Equal(t, "invalid path", ferr.msg, raw)
	}
	_, ferr := cleanRelPath("")
	require.NotNil(t, ferr)
	assert.Equal(t, "path required", ferr.msg)

	rel, ferr := cleanRelPath("a/./b/../c.yaml")
	require.Nil(t, ferr)
	assert.Equal(t, filepath.Join("a", "c.yaml"), rel)
}

func TestCrowdsecFiles_Read_ProtectedFilesRefused(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	for _, rel := range []string{"bouncer_key", "crowdsec.db", "data/crowdsec.db", "data/x.txt", "hub_cache/a.yaml", "config/x.db-wal"} {
		f.put(rel, "secret")
		w := f.read(rel)
		assert.Equal(t, http.StatusBadRequest, w.Code, rel)
		assert.NotContains(t, w.Body.String(), "secret", rel)
	}
}

func TestCrowdsecFiles_Read_Basics(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	f.put("config/config.yaml", "a: 1\n")
	w := f.read("config/config.yaml")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "a: 1")

	assert.Equal(t, http.StatusNotFound, f.read("config/missing.yaml").Code)
	assert.Equal(t, http.StatusBadRequest, f.read("../x.yaml").Code)
	assert.Equal(t, http.StatusBadRequest, f.read("/etc/passwd").Code)

	require.NoError(t, os.MkdirAll(filepath.Join(f.dir, "config", "d"), 0o750))
	assert.Equal(t, http.StatusBadRequest, f.read("config/d").Code, "directories are not files")
}

func TestCrowdsecFiles_Read_MissingDataDir(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	require.NoError(t, os.RemoveAll(f.dir))
	assert.Equal(t, http.StatusNotFound, f.read("config/config.yaml").Code)
}

func TestCrowdsecFiles_Read_SymlinkContainment(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	f.put("hub/scenarios/real.yaml", "type: leaky\n")
	require.NoError(t, os.MkdirAll(filepath.Join(f.dir, "config", "scenarios"), 0o750))
	require.NoError(t, os.Symlink(filepath.Join(f.dir, "hub", "scenarios", "real.yaml"), filepath.Join(f.dir, "config", "scenarios", "real.yaml")))

	w := f.read("config/scenarios/real.yaml")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "leaky")

	// A relative link that stays inside DataDir is also readable.
	require.NoError(t, os.Symlink("../../hub/scenarios/real.yaml", filepath.Join(f.dir, "config", "scenarios", "rel.yaml")))
	assert.Equal(t, http.StatusOK, f.read("config/scenarios/rel.yaml").Code)

	// A link resolving outside DataDir is refused.
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("outside-content"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(f.dir, "config", "scenarios", "out.yaml")))
	w = f.read("config/scenarios/out.yaml")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "outside-content")

	// A link inside DataDir that resolves to a protected file is refused too.
	f.put("bouncer_key", "secret")
	require.NoError(t, os.Symlink(filepath.Join(f.dir, "bouncer_key"), filepath.Join(f.dir, "config", "k.yaml")))
	w = f.read("config/k.yaml")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "secret")

	// A dangling link is reported as missing.
	require.NoError(t, os.Symlink(filepath.Join(f.dir, "nope"), filepath.Join(f.dir, "config", "dangling.yaml")))
	assert.Equal(t, http.StatusNotFound, f.read("config/dangling.yaml").Code)
}

func TestCrowdsecFiles_List(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	f.put("config/config.yaml", "a: 1\n")
	f.put("hub/scenarios/real.yaml", "x: 1\n")
	f.put("bouncer_key", "secret")
	f.put("data/crowdsec.db", "db")
	f.put("data/GeoLite2-City.mmdb", "geo")
	f.put("hub_cache/a.tgz", "tgz")
	f.put("config/other.db-wal", "wal")
	f.put("config/hub_cache/keep.yaml", "k: 1\n")
	require.NoError(t, os.MkdirAll(filepath.Join(f.dir, "config", "scenarios"), 0o750))
	require.NoError(t, os.Symlink(filepath.Join(f.dir, "hub", "scenarios", "real.yaml"), filepath.Join(f.dir, "config", "scenarios", "real.yaml")))
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("x"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(f.dir, "config", "scenarios", "out.yaml")))
	require.NoError(t, os.Symlink(filepath.Join(f.dir, "nope"), filepath.Join(f.dir, "config", "scenarios", "dangling.yaml")))
	require.NoError(t, os.Symlink(filepath.Join(f.dir, "hub"), filepath.Join(f.dir, "config", "dirlink")))

	files := f.list()
	assert.ElementsMatch(t, []string{
		filepath.Join("config", "config.yaml"),
		filepath.Join("hub", "scenarios", "real.yaml"),
		filepath.Join("config", "scenarios", "real.yaml"),
		filepath.Join("config", "hub_cache", "keep.yaml"),
	}, files)
}

func TestCrowdsecFiles_List_MissingDir(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	require.NoError(t, os.RemoveAll(f.dir))
	assert.Empty(t, f.list())
}

func TestCrowdsecFiles_List_SkipsInaccessibleDirs(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	t.Parallel()
	f := newFilesFixture(t)
	f.put("config/ok.yaml", "a: 1\n")
	locked := filepath.Join(f.dir, "lost+found")
	require.NoError(t, os.MkdirAll(locked, 0o750))
	require.NoError(t, os.Chmod(locked, 0o000))       // #nosec G302 -- test fixture
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) }) // #nosec G302 -- test fixture
	assert.Equal(t, []string{filepath.Join("config", "ok.yaml")}, f.list())
}

// Every file the write endpoint accepts must also be listed (and readable).
func TestCrowdsecFiles_EveryWritableFileIsListed(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	candidates := []string{
		"config.yaml", "config/config.yaml", "config/a.yml", "config/a.json", "config/a.txt", "config/a.conf",
		"config/parsers/s01/x.yaml", "config/hub_cache/n.yaml", "config/data/n.yaml", "notes.md", "bouncer_key",
		"data/x.json", "hub_cache/x.yaml", "config/local_api_credentials.yaml", "config/x.db", "hub/x.yaml",
	}
	accepted := 0
	for _, rel := range candidates {
		w := f.write(rel, "k: v\n")
		if w.Code != http.StatusOK {
			continue
		}
		accepted++
		assert.Contains(t, f.list(), filepath.FromSlash(rel), "writable but not listed: %s", rel)
		assert.Equal(t, http.StatusOK, f.read(rel).Code, "writable but not readable: %s", rel)
	}
	assert.Greater(t, accepted, 5)
}

func TestCrowdsecFiles_Write_Success(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	w := f.write("config/new/dir/a.yaml", "key: value\n")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "written", resp["status"])
	assert.Empty(t, resp["backup"], "nothing to back up for a new file")

	b, err := os.ReadFile(filepath.Join(f.dir, "config", "new", "dir", "a.yaml")) // #nosec G304 -- test path
	require.NoError(t, err)
	assert.Equal(t, "key: value\n", string(b))
	info, err := os.Stat(filepath.Join(f.dir, "config", "new", "dir", "a.yaml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestCrowdsecFiles_Write_EmptyYAMLAndMultiDocument(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	assert.Equal(t, http.StatusOK, f.write("a.yaml", "").Code)
	assert.Equal(t, http.StatusOK, f.write("b.yaml", "a: 1\n---\nb: 2\n").Code)
	assert.Equal(t, http.StatusOK, f.write("c.json", "not validated as json").Code)
}

func TestCrowdsecFiles_Write_Validation(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	f.put("sentinel.yaml", "keep: me\n")

	cases := []struct {
		name, path, content string
		status              int
	}{
		{"missing path", "", "x", http.StatusBadRequest},
		{"traversal", "../escape.yaml", "a: 1", http.StatusBadRequest},
		{"nested traversal", "config/../../escape.yaml", "a: 1", http.StatusBadRequest},
		{"absolute", "/tmp/escape.yaml", "a: 1", http.StatusBadRequest},
		{"disallowed type", "run.sh", "echo hi", http.StatusBadRequest},
		{"no extension", "noext", "x", http.StatusBadRequest},
		{"invalid yaml", "bad.yaml", "a: [unclosed\n", http.StatusBadRequest},
		{"invalid yml", "bad.yml", "a: b: c: d\n", http.StatusBadRequest},
		{"protected secret", "bouncer_key", "x", http.StatusBadRequest},
		{"protected db", "crowdsec.db", "x", http.StatusBadRequest},
		{"protected data tree", "data/x.json", "{}", http.StatusBadRequest},
		{"protected cache tree", "hub_cache/x.yaml", "a: 1", http.StatusBadRequest},
		{"protected credentials", "config/local_api_credentials.yaml", "a: 1", http.StatusBadRequest},
		{"too large", "big.txt", strings.Repeat("a", maxCrowdsecFileBytes+1), http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		w := f.write(tc.path, tc.content)
		assert.Equal(t, tc.status, w.Code, "%s: %s", tc.name, w.Body.String())
		assert.NotEmpty(t, errorOf(t, w), tc.name)
	}

	// Exactly the limit is accepted.
	assert.Equal(t, http.StatusOK, f.write("limit.txt", strings.Repeat("a", maxCrowdsecFileBytes)).Code)

	// Nothing leaked outside DataDir and nothing was created by the rejected requests.
	_, err := os.Stat(filepath.Join(f.root, "escape.yaml"))
	assert.True(t, os.IsNotExist(err))
	assert.Empty(t, f.glob("filebackup"))
	b, err := os.ReadFile(filepath.Join(f.dir, "sentinel.yaml")) // #nosec G304 -- test path
	require.NoError(t, err)
	assert.Equal(t, "keep: me\n", string(b))
}

func TestCrowdsecFiles_Write_OversizedBodyIs413(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	body := []byte(`{"path":"a.txt","content":"` + strings.Repeat("a", maxCrowdsecWriteBodyBytes+1) + `"}`)
	w := f.writeRaw(body)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Equal(t, "file too large", errorOf(t, w))
}

func TestCrowdsecFiles_Write_InvalidPayload(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	w := f.writeRaw([]byte("not json"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "invalid payload", errorOf(t, w))
}

func TestCrowdsecFiles_Write_SymlinkPolicy(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "outside.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("orig: outside\n"), 0o600))
	hubItem := f.put("hub/scenarios/real.yaml", "orig: hub\n")
	require.NoError(t, os.MkdirAll(filepath.Join(f.dir, "config", "scenarios"), 0o750))
	inLink := filepath.Join(f.dir, "config", "scenarios", "real.yaml")
	require.NoError(t, os.Symlink(hubItem, inLink))
	outLink := filepath.Join(f.dir, "config", "scenarios", "out.yaml")
	require.NoError(t, os.Symlink(outside, outLink))
	dirLink := filepath.Join(f.dir, "config", "linkeddir")
	require.NoError(t, os.Symlink(outsideDir, dirLink))
	require.NoError(t, os.Symlink(filepath.Join(f.dir, "nope.yaml"), filepath.Join(f.dir, "config", "dangling.yaml")))

	for _, rel := range []string{
		"config/scenarios/real.yaml", "config/scenarios/out.yaml", "config/linkeddir/outside.yaml",
		"config/linkeddir/new.yaml", "config/dangling.yaml",
	} {
		w := f.write(rel, "changed: true\n")
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", rel, w.Body.String())
	}

	// Targets are untouched and the links are still links.
	for path, want := range map[string]string{hubItem: "orig: hub\n", outside: "orig: outside\n"} {
		b, err := os.ReadFile(path) // #nosec G304 -- test path
		require.NoError(t, err)
		assert.Equal(t, want, string(b))
	}
	for _, l := range []string{inLink, outLink, dirLink} {
		fi, err := os.Lstat(l)
		require.NoError(t, err)
		assert.NotZero(t, fi.Mode()&os.ModeSymlink, l)
	}
	_, err := os.Stat(filepath.Join(outsideDir, "new.yaml"))
	assert.True(t, os.IsNotExist(err))
	assert.Empty(t, f.glob("filebackup"))

	// A regular file in the same tree is still writable.
	assert.Equal(t, http.StatusOK, f.write("config/scenarios/regular.yaml", "ok: true\n").Code)
}

func TestCrowdsecFiles_Write_RejectsNonRegularTargets(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	require.NoError(t, os.MkdirAll(filepath.Join(f.dir, "config", "adir.yaml"), 0o750))
	f.put("config/file.yaml", "a: 1\n")
	assert.Equal(t, http.StatusBadRequest, f.write("config/adir.yaml", "a: 1").Code)
	assert.Equal(t, http.StatusBadRequest, f.write("config/file.yaml/child.yaml", "a: 1").Code, "parent is a file")
}

func TestCrowdsecFiles_Write_DoesNotDisturbDataDir(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	f.put("config/config.yaml", "old: 1\n")
	f.put("config/other.yaml", "other: 1\n")
	f.put("bouncer_key", "secret")
	f.put("data/crowdsec.db", "db")
	f.put("hub_cache/a.tgz", "tgz")
	before, err := os.Stat(f.dir)
	require.NoError(t, err)

	w := f.write("config/config.yaml", "new: 2\n")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after, err := os.Stat(f.dir)
	require.NoError(t, err)
	assert.True(t, os.SameFile(before, after), "DataDir must keep its identity")
	for rel, want := range map[string]string{
		"config/config.yaml": "new: 2\n", "config/other.yaml": "other: 1\n",
		"bouncer_key": "secret", "data/crowdsec.db": "db", "hub_cache/a.tgz": "tgz",
	} {
		b, readErr := os.ReadFile(filepath.Join(f.dir, rel)) // #nosec G304 -- test path
		require.NoError(t, readErr)
		assert.Equal(t, want, string(b), rel)
	}
	// No stray temp files next to the target.
	entries, err := os.ReadDir(filepath.Join(f.dir, "config"))
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Empty(t, f.glob("backup"), "a file edit must not create a full snapshot")
}

func TestCrowdsecFiles_Write_BackupHoldsOnlyPreviousVersionAndIsCappedIndependently(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	f.put("config/config.yaml", "v: 0\n")
	f.put("config/other.yaml", "other: 1\n")

	// Pre-existing full snapshots must never be evicted by file backups.
	snaps := make([]string, 0, 5)
	for _, ts := range []string{"20260101-000001", "20260101-000002", "20260101-000003", "20260101-000004", "20260101-000005"} {
		p := filepath.Join(f.root, "crowdsec.backup."+ts)
		require.NoError(t, os.Mkdir(p, 0o700))
		snaps = append(snaps, p)
	}

	w := f.write("config/config.yaml", "v: 1\n")
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["backup"])
	prev, err := os.ReadFile(filepath.Join(resp["backup"], "config", "config.yaml")) // #nosec G304 -- test path
	require.NoError(t, err)
	assert.Equal(t, "v: 0\n", string(prev))
	_, err = os.Stat(filepath.Join(resp["backup"], "config", "other.yaml"))
	assert.True(t, os.IsNotExist(err), "backup must hold only the replaced file")

	for i := 2; i <= 14; i++ {
		require.Equal(t, http.StatusOK, f.write("config/config.yaml", "v: "+string(rune('0'+i%10))+"\n").Code)
	}
	assert.LessOrEqual(t, len(f.glob("filebackup")), 10)
	assert.NotEmpty(t, f.glob("filebackup"))
	for _, s := range snaps {
		_, err := os.Stat(s)
		assert.NoError(t, err, "full snapshot evicted by file backups: %s", s)
	}
}

func TestCrowdsecFiles_Write_PreservesExistingMode(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	p := f.put("config/mode.yaml", "a: 1\n")
	require.NoError(t, os.Chmod(p, 0o640)) // #nosec G302 -- test fixture
	require.Equal(t, http.StatusOK, f.write("config/mode.yaml", "a: 2\n").Code)
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestCrowdsecFiles_Write_BackupFailureIs500(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	t.Parallel()
	f := newFilesFixture(t)
	p := f.put("config/a.yaml", "v: 0\n")
	require.NoError(t, os.Chmod(f.root, 0o500))       // #nosec G302 -- test fixture
	t.Cleanup(func() { _ = os.Chmod(f.root, 0o750) }) // #nosec G302 -- test fixture

	w := f.write("config/a.yaml", "v: 1\n")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	b, err := os.ReadFile(p) // #nosec G304 -- test path
	require.NoError(t, err)
	assert.Equal(t, "v: 0\n", string(b), "a failed backup must leave the file unchanged")
}

func TestAtomicWriteFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "a.yaml")
	require.NoError(t, atomicWriteFile(target, []byte("one"), 0o600))
	require.NoError(t, atomicWriteFile(target, []byte("two"), 0o600))
	b, err := os.ReadFile(target) // #nosec G304 -- test path
	require.NoError(t, err)
	assert.Equal(t, "two", string(b))

	// A failure at the final rename must remove the temp file and keep the destination intact.
	blocked := filepath.Join(dir, "blocked")
	require.NoError(t, os.MkdirAll(filepath.Join(blocked, "child"), 0o750))
	require.Error(t, atomicWriteFile(blocked, []byte("x"), 0o600))
	require.Error(t, atomicWriteFile(filepath.Join(dir, "no", "such", "dir", "a.yaml"), []byte("x"), 0o600))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{"a.yaml", "blocked"}, names)
}

func TestPathWithin(t *testing.T) {
	t.Parallel()
	assert.True(t, pathWithin("/a/b", "/a/b"))
	assert.True(t, pathWithin("/a/b", "/a/b/c/d"))
	assert.False(t, pathWithin("/a/b", "/a/bc"))
	assert.False(t, pathWithin("/a/b", "/a"))
	assert.False(t, pathWithin("/a/b", "/x/y"))
	assert.False(t, pathWithin("/a/b", "relative"))
}

func TestCrowdsecFiles_Read_UnreadableFileIs500(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	t.Parallel()
	f := newFilesFixture(t)
	p := f.put("config/locked.yaml", "a: 1\n")
	require.NoError(t, os.Chmod(p, 0o000))       // #nosec G302 -- test fixture
	t.Cleanup(func() { _ = os.Chmod(p, 0o600) }) // #nosec G302 -- test fixture
	assert.Equal(t, http.StatusInternalServerError, f.read("config/locked.yaml").Code)
}

func TestFileErrorLog_StripsControlCharacters(t *testing.T) {
	t.Parallel()
	err := errors.New("open config/a\r\nb\nc.yaml: denied")
	entry := fileErrorLog(err)
	got, ok := entry.Data["error"].(string)
	require.True(t, ok)
	assert.NotContains(t, got, "\n")
	assert.NotContains(t, got, "\r")
	assert.Contains(t, got, "denied")
}

func TestResolveReadable_UnresolvablePaths(t *testing.T) {
	t.Parallel()
	t.Run("data dir cannot be resolved", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		loop := filepath.Join(root, "crowdsec")
		require.NoError(t, os.Symlink("crowdsec", loop)) // ELOOP is not "does not exist"
		_, ferr := resolveReadable(loop, "config/a.yaml")
		require.NotNil(t, ferr)
		assert.Equal(t, http.StatusInternalServerError, ferr.status)
	})
	t.Run("self-referencing link inside data dir is refused", func(t *testing.T) {
		t.Parallel()
		f := newFilesFixture(t)
		require.NoError(t, os.MkdirAll(filepath.Join(f.dir, "config"), 0o750))
		require.NoError(t, os.Symlink("loop.yaml", filepath.Join(f.dir, "config", "loop.yaml")))
		w := f.read("config/loop.yaml")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestResolveWritable_UninspectablePathIs500(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	t.Parallel()
	f := newFilesFixture(t)
	locked := filepath.Join(f.dir, "locked")
	require.NoError(t, os.MkdirAll(locked, 0o750))
	require.NoError(t, os.Chmod(locked, 0o000))       // #nosec G302 -- test fixture
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) }) // #nosec G302 -- test fixture

	// Lstat fails with EACCES (not ENOENT) below an unsearchable directory.
	w := f.write("locked/a.yaml", "a: 1\n")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "failed to inspect path", errorOf(t, w))
}

func TestCrowdsecFiles_Write_FilesystemFailuresAre500(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	t.Parallel()
	t.Run("new directory cannot be created", func(t *testing.T) {
		t.Parallel()
		f := newFilesFixture(t)
		require.NoError(t, os.Chmod(f.dir, 0o500))       // #nosec G302 -- test fixture
		t.Cleanup(func() { _ = os.Chmod(f.dir, 0o750) }) // #nosec G302 -- test fixture
		w := f.write("newdir/a.yaml", "a: 1\n")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "failed to prepare dir", errorOf(t, w))
	})
	t.Run("file cannot be written", func(t *testing.T) {
		t.Parallel()
		f := newFilesFixture(t)
		cfg := filepath.Join(f.dir, "config")
		require.NoError(t, os.MkdirAll(cfg, 0o750))
		require.NoError(t, os.Chmod(cfg, 0o500))       // #nosec G302 -- test fixture
		t.Cleanup(func() { _ = os.Chmod(cfg, 0o750) }) // #nosec G302 -- test fixture
		w := f.write("config/new.yaml", "a: 1\n")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "failed to write file", errorOf(t, w))
		assert.NoFileExists(t, filepath.Join(cfg, "new.yaml"))
	})
}
