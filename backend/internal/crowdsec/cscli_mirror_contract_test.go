package crowdsec

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCSCLIContractAcceptsMirrorConfig runs the cscli on the PATH with a derived mirror configuration. It makes no hub
// request. It skips when cscli is not installed, as the other cscli checks do on hosts without the binary.
func TestCSCLIContractAcceptsMirrorConfig(t *testing.T) {
	if _, err := exec.LookPath("cscli"); err != nil {
		t.Skip("cscli is not installed")
	}
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n")
	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.NoError(t, err)
	defer cleanup()

	// The binary is fixed; the only variable argument is a configuration file this test created.
	out, err := exec.CommandContext(t.Context(), "cscli", "-c", derived, "version").CombinedOutput() //nolint:gosec // G204: fixed binary, test-owned config path
	require.NoError(t, err, string(out))
	require.NotContains(t, strings.ToLower(string(out)), "yaml", "cscli must parse the derived configuration")
}
