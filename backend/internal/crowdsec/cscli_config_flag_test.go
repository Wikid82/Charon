package crowdsec

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/testutil/cscliguard"
)

// Hub items are verified against the engine configuration, so cscli gets `-c <file>` like the other calls.
func TestVerifyHubItemPassesTheConfigFileToCSCLI(t *testing.T) {
	t.Parallel()
	exec := &verifyExec{out: `{"installed":true,"tainted":false}`}

	require.NoError(t, VerifyHubItem(t.Context(), exec, "/data/config/config.yaml", "collections", "crowdsecurity/sshd"))

	i := slices.Index(exec.args, "-c")
	require.GreaterOrEqual(t, i, 0, "cscli was started without -c: %v", exec.args)
	require.Greater(t, len(exec.args), i+1, "-c has no file: %v", exec.args)
	assert.Equal(t, "/data/config/config.yaml", exec.args[i+1])
}

// cscliProbesWithoutConfig lists the functions that start cscli without -c on purpose.
//   - hasCSCLI only probes whether the cscli binary can run (`cscli version`). It reads no configuration, and a
//     broken or missing engine configuration must not make it report cscli as absent.
var cscliProbesWithoutConfig = map[string]bool{"hasCSCLI": true}

// Every cscli start in this package builds its arguments with withConfigFlag, so cscli gets `-c <file>`. The
// calls are found by their literal "cscli" argument, so the check needs no change when a new call is added; a
// function with several cscli starts needs a withConfigFlag call for each.
func TestCrowdSecPackageRunsCSCLIOnlyWithTheConfigFile(t *testing.T) {
	t.Parallel()
	offenders, err := cscliguard.Offenders(".", []string{"withConfigFlag"}, cscliProbesWithoutConfig)
	require.NoError(t, err)
	assert.Empty(t, offenders, "these functions start cscli without -c")
}

// stripConfigFlag drops the leading `-c <file>` pair the hub service puts in front of cscli arguments, so
// scripted fakes can match the commands without the configuration path of the test's data directory.
func stripConfigFlag(args []string) []string {
	if len(args) >= 2 && args[0] == "-c" {
		return args[2:]
	}
	return args
}
