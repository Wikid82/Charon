package bouncerkey

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// clearEnv blanks every key variable so each test starts from a known environment.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, name := range EnvNames() {
		t.Setenv(name, "")
	}
}

func TestDefaultFile(t *testing.T) {
	require.Equal(t, "/app/data/crowdsec/bouncer_key", DefaultFile)
}

func TestEnvNamesOrder(t *testing.T) {
	require.Equal(t, []string{
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}, EnvNames())
}

func TestEnvNamesReturnsCopy(t *testing.T) {
	names := EnvNames()
	names[0] = "MUTATED"

	require.Equal(t, "CHARON_SECURITY_CROWDSEC_API_KEY", EnvNames()[0])
}

func TestFromEnvEachNameAlone(t *testing.T) {
	for _, name := range EnvNames() {
		t.Run(name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(name, "key-for-"+name)

			value, got := FromEnv()
			require.Equal(t, "key-for-"+name, value)
			require.Equal(t, name, got)
		})
	}
}

func TestFromEnvNoneSet(t *testing.T) {
	clearEnv(t)

	value, name := FromEnv()
	require.Empty(t, value)
	require.Empty(t, name)
}

// TestFromEnvEarlierNameWins sets every adjacent pair to different values and
// checks the earlier name in EnvNames() is returned.
func TestFromEnvEarlierNameWins(t *testing.T) {
	names := EnvNames()
	for i := 0; i+1 < len(names); i++ {
		earlier, later := names[i], names[i+1]
		t.Run(earlier+" over "+later, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(later, "later-key")
			t.Setenv(earlier, "earlier-key")

			value, name := FromEnv()
			require.Equal(t, "earlier-key", value)
			require.Equal(t, earlier, name)
		})
	}
}

// TestFromEnvSkipsEmptyValues checks that an empty earlier variable does not
// shadow a later one.
func TestFromEnvSkipsEmptyValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CROWDSEC_API_KEY", "fallback-key")

	value, name := FromEnv()
	require.Equal(t, "fallback-key", value)
	require.Equal(t, "CROWDSEC_API_KEY", name)
}

// TestFromEnvOrder pins the cross-package precedence: CHARON_SECURITY_CROWDSEC_API_KEY
// wins over CROWDSEC_API_KEY, which wins over CROWDSEC_BOUNCER_API_KEY.
func TestFromEnvOrder(t *testing.T) {
	clearEnv(t)
	t.Setenv("CROWDSEC_BOUNCER_API_KEY", "bouncer")
	t.Setenv("CROWDSEC_API_KEY", "primary")
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "charon")

	value, name := FromEnv()
	require.Equal(t, "charon", value)
	require.Equal(t, "CHARON_SECURITY_CROWDSEC_API_KEY", name)

	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "")
	value, name = FromEnv()
	require.Equal(t, "primary", value)
	require.Equal(t, "CROWDSEC_API_KEY", name)
}

func TestReadFileTrimming(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{name: "key with newline", content: "test-key-123\n", expected: "test-key-123"},
		{name: "key with extra whitespace", content: "  test-key-456  \n", expected: "test-key-456"},
		{name: "key without newline", content: "test-key-789", expected: "test-key-789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPath := filepath.Join(tmpDir, filepath.Base(tt.expected))
			require.NoError(t, os.WriteFile(keyPath, []byte(tt.content), 0o600))

			require.Equal(t, tt.expected, ReadFile(keyPath))
		})
	}
}

func TestReadFileMissingFile(t *testing.T) {
	t.Parallel()

	require.Empty(t, ReadFile(filepath.Join(t.TempDir(), "nonexistent")))
}

func TestReadFileDirectory(t *testing.T) {
	t.Parallel()

	require.Empty(t, ReadFile(t.TempDir()))
}

func TestReadFileWhitespaceOnly(t *testing.T) {
	t.Parallel()

	keyPath := filepath.Join(t.TempDir(), "bouncer_key")
	require.NoError(t, os.WriteFile(keyPath, []byte(" \n\t"), 0o600))

	require.Empty(t, ReadFile(keyPath))
}
