package testutil

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// dsnGuardPackages lists the package directories (relative to the backend
// module root) whose tests are repeatable in one process (-count=N, -shuffle).
// Their tests must obtain in-memory databases from MemoryDSN/OpenMemoryDB and
// never spell a shared-cache DSN themselves: a hand-built name derived from
// t.Name() collides across -count runs. Add a package here once it is clean.
var dsnGuardPackages = []string{
	"internal/crowdsec",
	"internal/api/handlers",
}

// sharedCacheMarker is split so this file does not contain the literal itself.
const sharedCacheMarker = "cache=" + "shared"

// backendRoot locates the backend module root (the directory holding go.mod)
// by walking up from this source file.
func backendRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "cannot locate source file")
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "go.mod not found above %s", file)
		dir = parent
	}
}

// findSharedCacheLiterals returns "path:line" for every line of every
// *_test.go file under dir (recursively) containing the shared-cache marker.
func findSharedCacheLiterals(t *testing.T, root, dir string) []string {
	t.Helper()
	var hits []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path) // #nosec G304 G122 -- test-only walk of a fixed in-repo directory
		if err != nil {
			return err
		}
		if !bytes.Contains(data, []byte(sharedCacheMarker)) {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, sharedCacheMarker) {
				hits = append(hits, rel+":"+strconv.Itoa(i+1))
			}
		}
		return nil
	})
	require.NoError(t, err)
	return hits
}

func TestNoSharedCacheLiteralsInRepeatablePackages(t *testing.T) {
	root := backendRoot(t)
	for _, pkg := range dsnGuardPackages {
		dir := filepath.Join(root, filepath.FromSlash(pkg))
		_, err := os.Stat(dir)
		require.NoError(t, err, "guarded package directory %s is missing; update dsnGuardPackages", pkg)
		if hits := findSharedCacheLiterals(t, root, dir); len(hits) > 0 {
			t.Errorf("%s: tests must use testutil.MemoryDSN/OpenMemoryDB instead of a hand-written %q DSN "+
				"(shared names collide under -count/-shuffle); offending locations:\n  %s",
				pkg, sharedCacheMarker, strings.Join(hits, "\n  "))
		}
	}
}

func TestFindSharedCacheLiterals_ReportsFileAndLine(t *testing.T) {
	root := t.TempDir()
	body := "package x\n\nvar a = 1\nvar dsn = \"file:x?mode=memory&" + sharedCacheMarker + "\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "bad_test.go"), []byte(body), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "clean_test.go"), []byte("package x\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notest.go"), []byte(body), 0o600))
	require.Equal(t, []string{"bad_test.go:4"}, findSharedCacheLiterals(t, root, root))
}
