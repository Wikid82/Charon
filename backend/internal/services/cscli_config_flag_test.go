package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/testutil/cscliguard"
)

// TestCSCLIStartsUseConfigFlag keeps every cscli start in this package on crowdsec.RunCSCLI, which passes the
// engine configuration file as -c. A direct start with a literal "cscli" command would drop that flag.
func TestCSCLIStartsUseConfigFlag(t *testing.T) {
	t.Parallel()

	offenders, err := cscliguard.Offenders(".", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, offenders)
}

// TestEnsureBouncerRegistration_StartupCommandsCarryConfigFlag checks that the lapi status poll, the bouncer
// delete and the bouncer add all pass -c with the configuration file found in the data directory, and no -c
// when there is none.
func TestEnsureBouncerRegistration_StartupCommandsCarryConfigFlag(t *testing.T) {
	cases := []struct {
		name   string
		layout string // configuration file path relative to the data directory; empty means no file
	}{
		{name: "nested config directory", layout: filepath.Join("config", "config.yaml")},
		{name: "flat config file", layout: "config.yaml"},
		{name: "no config file"},
	}
	wantCommands := [][]string{
		{"lapi", "status"},
		{"bouncers", "delete", "caddy-bouncer"},
		{"bouncers", "add", "caddy-bouncer", "-o", "raw"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newBouncerHarness(t)
			var wantConfig string
			if tc.layout != "" {
				wantConfig = filepath.Join(h.dataDir, tc.layout)
				require.NoError(t, os.MkdirAll(filepath.Dir(wantConfig), 0o750))
				require.NoError(t, os.WriteFile(wantConfig, []byte("common:\n"), 0o600))
			}
			exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey)}

			require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

			require.Len(t, exec.calls, len(wantCommands))
			for i, want := range wantCommands {
				call := exec.calls[i]
				require.Equal(t, "cscli", call[0])
				args := call[1:]
				if wantConfig == "" {
					assert.NotEqual(t, "-c", args[0], "call %d must not pass -c without a config file", i)
				} else {
					require.GreaterOrEqual(t, len(args), 2, "call %d", i)
					assert.Equal(t, []string{"-c", wantConfig}, args[:2], "call %d", i)
					args = args[2:]
				}
				assert.Equal(t, want, args, "call %d", i)
			}
		})
	}
}
