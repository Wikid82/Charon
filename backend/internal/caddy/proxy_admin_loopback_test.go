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

	"github.com/Wikid82/charon/backend/internal/config"
)

// adminBootstrapPattern matches the bootstrap config the entrypoint writes before
// the bundled Caddy starts. The listen address is a literal, so the first apply
// does not rebind the admin interface.
var adminBootstrapPattern = regexp.MustCompile(`"admin":\{"listen":"127\.0\.0\.1:2019"\}`)

// adminDisallowedLiteral is the admin bind on every interface. No shipped file
// may contain it.
const adminDisallowedLiteral = "0.0.0.0:2019"

// adminURLVariablePattern matches a shipped file that sets the admin URL variable.
var adminURLVariablePattern = regexp.MustCompile(`(CHARON|CPM)_CADDY_ADMIN_API`)

// TestGenerateConfigAdminListensOnLoopbackOnly pins the generated admin bind to
// the loopback address that the entrypoint bootstrap also uses.
func TestGenerateConfigAdminListensOnLoopbackOnly(t *testing.T) {
	cfg, err := GenerateConfig(nil, "/data/caddy/data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg.Admin)
	assert.Equal(t, "127.0.0.1:2019", cfg.Admin.Listen)
	assert.Equal(t, config.CaddyAdminAddress, cfg.Admin.Listen)
}

// TestProxyAdminAddressIsLoopbackOnly is a repository scan. The entrypoint
// bootstrap must bind the loopback address, and no shipped Go, container or
// compose file may bind the admin interface on every interface.
func TestProxyAdminAddressIsLoopbackOnly(t *testing.T) {
	root := repoRoot(t)

	entrypoint, err := os.ReadFile(filepath.Join(root, ".docker", "docker-entrypoint.sh"))
	require.NoError(t, err)
	assert.Regexp(t, adminBootstrapPattern, string(entrypoint), "entrypoint bootstrap must bind the loopback address")

	var violations []string
	walkErr := filepath.WalkDir(filepath.Join(root, "backend"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		violations = append(violations, scanFileLines(t, path, root, regexp.MustCompile(regexp.QuoteMeta(adminDisallowedLiteral)))...)
		return nil
	})
	require.NoError(t, walkErr)

	shipped := []string{filepath.Join(root, "Dockerfile"), filepath.Join(root, ".docker", "docker-entrypoint.sh")}
	composes, err := filepath.Glob(filepath.Join(root, ".docker", "compose", "*.yml"))
	require.NoError(t, err)
	shipped = append(shipped, composes...)
	for _, path := range shipped {
		violations = append(violations, scanFileLines(t, path, root, regexp.MustCompile(regexp.QuoteMeta(adminDisallowedLiteral)))...)
	}

	// Every file under configs/ ships in the repository, so each one is scanned.
	configsErr := filepath.WalkDir(filepath.Join(root, "configs"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		violations = append(violations, scanFileLines(t, path, root, regexp.MustCompile(regexp.QuoteMeta(adminDisallowedLiteral)))...)
		return nil
	})
	require.NoError(t, configsErr)

	if len(violations) > 0 {
		t.Errorf("the admin interface must listen on the loopback address only; found:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// TestShippedFilesDoNotSetAdminAPIVariable is a repository scan. Charon's own
// files rely on the default admin URL; none of them may set the variable.
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
