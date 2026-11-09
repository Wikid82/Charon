package crowdsec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/testutil/cscliguard"
)

// shellScriptsRunningCSCLI lists the shell scripts that run cscli in the image, relative to the repository root.
var shellScriptsRunningCSCLI = []string{
	".docker/docker-entrypoint.sh",
	".docker/cscli-hub-mirror.sh",
	"configs/crowdsec/install_hub_items.sh",
}

// repoRoot returns the repository root: the parent of the directory that holds backend/go.mod. It walks up from the
// package directory, so the test finds the scripts from any working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "backend", "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "no backend/go.mod above the package directory")
		dir = parent
	}
}

// The shell scripts must name the live configuration file on every cscli start, as the Go calls do. The guard
// reads the scripts as they are, so a new bare call fails here without a list change.
func TestShellScriptsRunCSCLIOnlyWithTheConfigFile(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	for _, rel := range shellScriptsRunningCSCLI {
		offenders, err := cscliguard.ShellOffenders(filepath.Join(root, filepath.FromSlash(rel)))
		require.NoError(t, err, "scan %s", rel)
		assert.Empty(t, offenders, "these lines start cscli without -c in %s", rel)
	}
}

// The check above proves nothing unless the guard flags a bare start, so the bare fixture must offend.
func TestShellGuardFlagsTheBareFixture(t *testing.T) {
	t.Parallel()
	fixture := filepath.Join(repoRoot(t), "backend", "internal", "testutil", "cscliguard", "testdata", "shell", "bare.sh.txt")
	offenders, err := cscliguard.ShellOffenders(fixture)
	require.NoError(t, err)
	assert.NotEmpty(t, offenders)
}
