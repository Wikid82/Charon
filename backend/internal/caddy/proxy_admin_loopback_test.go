package caddy

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminDisallowedPatterns are admin listeners that would expose the admin API on
// every interface. No shipped file may contain them.
var adminDisallowedPatterns = []*regexp.Regexp{
	regexp.MustCompile(regexp.QuoteMeta("0.0.0.0:2019")),
	regexp.MustCompile(`"listen"\s*:\s*":2019"`),
}

// adminURLVariablePattern matches a shipped file that sets the admin URL variable.
var adminURLVariablePattern = regexp.MustCompile(`(CHARON|CPM)_CADDY_ADMIN_API`)

// TestEntrypointBootstrapUsesAdminSocket pins the entrypoint to the admin socket:
// the bootstrap listener comes from the binary, the stale socket is cleared before
// Caddy starts, and readiness uses the binary instead of a TCP probe.
func TestEntrypointBootstrapUsesAdminSocket(t *testing.T) {
	root := repoRoot(t)
	entrypoint, err := os.ReadFile(filepath.Clean(filepath.Join(root, ".docker", "docker-entrypoint.sh")))
	require.NoError(t, err)
	script := string(entrypoint)

	assert.Contains(t, script, "caddy-admin listen", "bootstrap must take admin.listen from the binary")
	assert.Contains(t, script, "caddy-admin prepare", "stale socket must be cleared before Caddy starts")
	assert.Contains(t, script, "caddy-admin get /config/", "readiness must read the admin socket")
	assert.NotContains(t, script, "127.0.0.1:2019", "entrypoint must not reference the TCP admin listener")
	assert.NotContains(t, script, "wget", "readiness must not use a TCP probe")
	// The image and the mkdir -p above create the directory with the default mode,
	// so the entrypoint has to restrict it before Caddy creates the socket.
	assert.Contains(t, script, "chmod 700 /app/data/run", "the admin socket directory must be private to the charon user")
	assert.Less(t, strings.Index(script, "chmod 700 /app/data/run"), strings.Index(script, `"$CHARON_BIN" caddy-admin prepare`),
		"the socket directory must be restricted before Caddy starts")

	dockerfile, err := os.ReadFile(filepath.Clean(filepath.Join(root, "Dockerfile")))
	require.NoError(t, err)
	assert.Contains(t, string(dockerfile), "chmod 700 /app/data/run", "a fresh volume copies the image's directory mode")
}

// TestAdminListenerNeverBindsAllInterfaces is a repository scan. No shipped Go,
// container, compose or configs file may bind the admin API on every interface.
func TestAdminListenerNeverBindsAllInterfaces(t *testing.T) {
	root := repoRoot(t)

	var files []string
	walkErr := filepath.WalkDir(filepath.Join(root, "backend"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	require.NoError(t, walkErr)

	shipped := []string{filepath.Join(root, "Dockerfile"), filepath.Join(root, ".docker", "docker-entrypoint.sh")}
	composes, err := filepath.Glob(filepath.Join(root, ".docker", "compose", "*.yml"))
	require.NoError(t, err)
	files = append(files, shipped...)
	files = append(files, composes...)

	configsErr := filepath.WalkDir(filepath.Join(root, "configs"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, configsErr)

	var violations []string
	for _, path := range files {
		for _, pattern := range adminDisallowedPatterns {
			violations = append(violations, scanFileLines(t, path, root, pattern)...)
		}
	}
	if len(violations) > 0 {
		t.Errorf("the admin interface must not listen on every interface; found:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// TestShippedFilesDoNotSetAdminAPIVariable is a repository scan. Charon's own
// files rely on the default admin socket; none of them may set the variable.
func TestShippedFilesDoNotSetAdminAPIVariable(t *testing.T) {
	root := repoRoot(t)

	files := []string{filepath.Join(root, "Dockerfile"), filepath.Join(root, "backend", ".env.example")}
	for _, pattern := range []string{
		filepath.Join(root, ".docker", "compose", "*.yml"),
		filepath.Join(root, "scripts", "*.sh"),
	} {
		matches, err := filepath.Glob(pattern)
		require.NoError(t, err)
		files = append(files, matches...)
	}

	var violations []string
	for _, path := range files {
		violations = append(violations, scanFileLines(t, path, root, adminURLVariablePattern)...)
	}
	if len(violations) > 0 {
		t.Errorf("shipped files must not set the admin URL variable; found:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// repoRoot returns the repository root relative to this package.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(repoRootFromCaddyPkg)
	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(root, "scripts"))
	require.NoError(t, statErr, "repository root %q does not contain scripts/", root)
	return root
}
