package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func importStatus(t *testing.T, r http.Handler, files map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, files))
	return w
}

func requireNoSnapshots(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(dir + ".backup.*")
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestImportConfigRefusesFlatArchiveForNestedInstall(t *testing.T) {
	t.Parallel()
	flatArchives := map[string]map[string]string{
		"flat config":                   {"config.yaml": importableConfig, "other.yaml": "o"},
		"flat config, spelled with ./":  {"./config.yaml": importableConfig},
		"flat config with config files": {"config.yaml": importableConfig, "config/acquis.yaml": "a"},
	}
	for name, files := range flatArchives {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			seedRealLayout(t, dir)
			_, r := newImportRouter(t, dir)
			before := treeState(t, dir)

			w := importStatus(t, r, files)

			require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
			require.JSONEq(t, `{"error":"`+msgImportFlatArchive+`"}`, w.Body.String())
			require.Equal(t, before, treeState(t, dir))
			requireNoSnapshots(t, dir)
		})
	}
}

func TestImportConfigLayoutMatrix(t *testing.T) {
	t.Parallel()
	nested := map[string]string{"config/config.yaml": importableConfig}
	flat := map[string]string{"config.yaml": importableConfig}
	both := map[string]string{"config.yaml": importableConfig, "config/config.yaml": importableConfig}
	cases := []struct {
		name    string
		seed    func(t *testing.T, dir string)
		files   map[string]string
		want    int
		cfgFile string
	}{
		{"nested live, nested archive", func(t *testing.T, d string) { seedRealLayout(t, d) }, nested, http.StatusOK, "config/config.yaml"},
		{"nested live, both layouts", func(t *testing.T, d string) { seedRealLayout(t, d) }, both, http.StatusOK, "config/config.yaml"},
		{"fresh tree, flat archive", func(*testing.T, string) {}, flat, http.StatusOK, "config.yaml"},
		{"fresh tree, nested archive", func(*testing.T, string) {}, nested, http.StatusOK, "config/config.yaml"},
		{"flat live, flat archive", func(t *testing.T, d string) { writeTree(t, d, map[string]string{"config.yaml": "old: true\n"}) }, flat, http.StatusOK, "config.yaml"},
		{"empty config folder, flat archive", func(t *testing.T, d string) { require.NoError(t, os.MkdirAll(filepath.Join(d, "config"), 0o750)) }, flat, http.StatusOK, "config.yaml"},
		{"hybrid tree, flat archive", func(t *testing.T, d string) {
			writeTree(t, d, map[string]string{"config.yaml": "old: true\n", "config/local_api_credentials.yaml": "live"})
		}, flat, http.StatusUnprocessableEntity, ""},
		{"hybrid tree, nested archive", func(t *testing.T, d string) {
			writeTree(t, d, map[string]string{"config.yaml": "old: true\n", "config/local_api_credentials.yaml": "live"})
		}, nested, http.StatusOK, "config/config.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			tc.seed(t, dir)
			_, r := newImportRouter(t, dir)

			w := importStatus(t, r, tc.files)

			require.Equal(t, tc.want, w.Code, w.Body.String())
			if tc.cfgFile != "" {
				require.Equal(t, importableConfig, readTreeFile(t, filepath.Join(dir, filepath.FromSlash(tc.cfgFile))))
			}
		})
	}
}

// TestImportConfigNestedGuardAfterExtraction covers the check that backs up the archive inspection: a
// config folder that is a link is kept across the import, so the archive's config/config.yaml is never
// written and the install would be left without the file it reads.
func TestImportConfigNestedGuardAfterExtraction(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "etc-crowdsec")
	writeTree(t, target, map[string]string{"acquis.yaml": "filenames: [x]\n"})
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "config")))
	_, r := newImportRouter(t, dir)
	before := treeState(t, target)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	require.JSONEq(t, `{"error":"`+msgImportFlatArchive+`"}`, w.Body.String())
	require.Equal(t, before, treeState(t, target))
	require.NoFileExists(t, filepath.Join(target, "config.yaml"))
}

func TestRequireArchiveFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		entries []string
		want    string
	}{
		{"top level", []string{"config.yaml"}, ""},
		{"nested", []string{"config/config.yaml"}, ""},
		{"dot slash", []string{"./config.yaml"}, ""},
		{"dot slash nested", []string{"./config/config.yaml"}, ""},
		{"double slash", []string{"config//config.yaml"}, ""},
		{"dot segment", []string{"config/./config.yaml"}, ""},
		{"parent segment", []string{"x/../config.yaml"}, ""},
		{"other folder", []string{"parsers/config.yaml"}, "required file missing: config.yaml"},
		{"deeper in config", []string{"config/sub/config.yaml"}, "required file missing: config.yaml"},
		{"other case", []string{"Config.yaml"}, "required file missing: config.yaml"},
		{"similar name", []string{"myconfig.yaml", "config.yaml.bak"}, "required file missing: config.yaml"},
		{"no entries", nil, "required file missing: config.yaml"},
		{"leaves the destination", []string{"../config.yaml"}, "invalid file path: ../config.yaml"},
		{"leaves the destination but the file is there", []string{"../config.yaml", "config.yaml"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := requireArchiveFile(tc.entries, "config.yaml")
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.want)
		})
	}
}
