package crowdsec

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// contractLiveConfig writes a live cscli configuration that cscli can load: a config_paths section whose
// directories exist under a temporary root. Without config_paths, cscli refuses to start with "no configuration
// paths provided".
func contractLiveConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dirs := map[string]string{
		"config_dir":       "config",
		"data_dir":         "data",
		"hub_dir":          "hub",
		"notification_dir": "notifications",
		"plugin_dir":       "plugins",
	}
	paths := make(map[string]string, len(dirs)+2)
	for key, name := range dirs {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(p, 0o750))
		paths[key] = p
	}
	paths["simulation_path"] = filepath.Join(paths["config_dir"], "simulation.yaml")
	paths["index_path"] = filepath.Join(paths["hub_dir"], ".index.json")
	raw, err := yaml.Marshal(map[string]any{
		"config_paths": paths,
		"cscli":        map[string]any{mirrorBranchKey: "master"},
	})
	require.NoError(t, err)
	return writeCSCLIConfig(t, root, string(raw))
}

// TestCSCLIContractAcceptsMirrorConfig runs the cscli on the PATH with a derived mirror configuration. It makes no hub
// request. The fixture is parsed on every host; the cscli run skips when cscli is not installed, as the other cscli
// checks do on hosts without the binary.
func TestCSCLIContractAcceptsMirrorConfig(t *testing.T) {
	live := contractLiveConfig(t)
	raw, err := os.ReadFile(live) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	var fixture map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &fixture))
	require.Contains(t, fixture, "config_paths", "the live fixture must be loadable by cscli")

	if _, lookErr := exec.LookPath("cscli"); lookErr != nil {
		t.Skip("cscli is not installed")
	}
	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.NoError(t, err)
	defer cleanup()

	info, err := os.Stat(derived)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	derivedRaw, err := os.ReadFile(derived) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	var doc struct {
		CSCLI map[string]any `yaml:"cscli"`
	}
	require.NoError(t, yaml.Unmarshal(derivedRaw, &doc))
	require.Equal(t, testMirrorTemplate, doc.CSCLI[mirrorTemplateKey], "the derived configuration carries the mirror key")

	// The binary is fixed; the only variable argument is a configuration file this test created.
	out, err := exec.CommandContext(t.Context(), "cscli", "-c", derived, "version").CombinedOutput() //nolint:gosec // G204: fixed binary, test-owned config path
	require.NoError(t, err, string(out))
	require.NotContains(t, strings.ToLower(string(out)), "yaml", "cscli must parse the derived configuration")

	cleanup()
	_, statErr := os.Stat(derived)
	require.ErrorIs(t, statErr, os.ErrNotExist, "the derived configuration is removed by cleanup")
}
