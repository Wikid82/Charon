package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// overComplexYAML is a mapping whose aliases expand to well over a million nodes while the text stays
// under 1 MiB.
func overComplexYAML() string {
	return "api:\n  server:\n    listen_uri: 0.0.0.0:8080\n" +
		"anchor: &a [" + strings.TrimSuffix(strings.Repeat("0,", 100), ",") + "]\n" +
		"flat: [" + strings.TrimSuffix(strings.Repeat("1,", 300_000), ",") + "]\n" +
		"refs: [" + strings.TrimSuffix(strings.Repeat("*a,", 9_000), ",") + "]\n"
}

func TestYAMLCostFailsClosedForConfigValidation(t *testing.T) {
	t.Parallel()
	t.Skip("enabled by a later commit in this series")
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(overComplexYAML()), 0o600))

	require.ErrorIs(t, validateYAMLFile(path), errConfigInvalid)
}

func TestYAMLCostFailsClosedForFileEdits(t *testing.T) {
	t.Parallel()
	t.Skip("enabled by a later commit in this series")

	require.NotNil(t, checkYAML("config/extra.yaml", overComplexYAML()))
}
