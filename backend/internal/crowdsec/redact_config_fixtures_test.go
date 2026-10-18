package crowdsec

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Fixtures: testdata/config (the shipped dist config and variants with FAKE secrets only).

// redactableFixtures are the fixtures that hold credentials and no shared nodes, so they can be both blanked and
// merged back.
var redactableFixtures = []string{"mysql.yaml", "postgres.yaml", "auto_registration_token.yaml", "url_userinfo.yaml", "extra_secret_keys.yaml"}

// flattenYAML maps every leaf of a decoded document to its dotted key path; lists count as one leaf.
func flattenYAML(prefix string, node any, out map[string]any) {
	m, ok := node.(map[string]any)
	if !ok {
		out[prefix] = node
		return
	}
	for k, v := range m {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		flattenYAML(path, v, out)
	}
}

// changedLeaves returns the key paths whose value differs between the two documents, and fails when a key was
// added or removed.
func changedLeaves(t *testing.T, before, after string) []string {
	t.Helper()
	was, now := map[string]any{}, map[string]any{}
	flattenYAML("", yamlMap(t, before), was)
	flattenYAML("", yamlMap(t, after), now)
	require.Len(t, now, len(was), "redaction must not add or remove keys")
	var changed []string
	for path, value := range was {
		got, ok := now[path]
		require.True(t, ok, "key %s was removed", path)
		if !reflect.DeepEqual(got, value) {
			changed = append(changed, path)
		}
	}
	return changed
}

// Redaction changes credential values and nothing else: every changed leaf is a credential key or a URL with
// userinfo, and every changed leaf holds the placeholder or a URL without its password.
func TestRedactConfigFileChangesOnlyCredentialValues(t *testing.T) {
	t.Parallel()
	for _, fixture := range redactableFixtures {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			original := configFixture(t, fixture)
			redacted := blankedConfig(t, original)

			changed := changedLeaves(t, original, redacted)

			require.NotEmpty(t, changed)
			now := map[string]any{}
			flattenYAML("", yamlMap(t, redacted), now)
			for _, path := range changed {
				parent, name := "", path
				if i := strings.LastIndex(path, "."); i >= 0 {
					parent, name = path[:i], path[i+1:]
				}
				value, _ := now[path].(string)
				isURL := strings.Contains(value, "://")
				require.True(t, isCredentialKey(parent, name) || isURL, "%s changed but is not a credential", path)
			}
			require.NotContains(t, redacted, "FAKE")
		})
	}
}

// The shipped default and the external database variants survive a blank-and-merge round trip value for value.
func TestRedactedFixturesRoundTripThroughTheMerge(t *testing.T) {
	t.Parallel()
	for _, fixture := range append([]string{"dist.yaml"}, redactableFixtures...) {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			original := configFixture(t, fixture)
			snapshot, imported := t.TempDir(), t.TempDir()
			writeConfigFile(t, snapshot, "config/config.yaml", original)
			writeConfigFile(t, imported, "config/config.yaml", blankedConfig(t, original))

			require.NoError(t, MergeRedactedConfig(snapshot, imported))

			got := readFile(t, filepath.Join(imported, "config", "config.yaml"))
			require.Equal(t, yamlMap(t, original), yamlMap(t, got))
		})
	}
}

// Credential key names beyond the common ones (passphrase, cloud access keys, auth headers, DSNs) are blanked.
func TestRedactConfigFileBlanksLessCommonCredentialKeys(t *testing.T) {
	t.Parallel()
	for _, key := range []string{
		"passphrase", "access_key", "secret_access_key", "authorization", "pwd", "bearer", "signing_key",
		"encryption_key", "psk", "dsn", "connection_string", "db_pwd", "key_passphrase", "wifi_psk", "primary_dsn",
	} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			original := "extra:\n  " + key + ": FAKE-value-0100\n  note: kept\n"

			redacted := blankedConfig(t, original)

			require.NotContains(t, redacted, "FAKE-value-0100")
			require.Contains(t, redacted, "note: kept")
		})
	}
}

// Names that look like credentials but hold paths, limits or switches stay untouched.
func TestRedactConfigFileKeepsBenignKeyNames(t *testing.T) {
	t.Parallel()
	for _, key := range []string{
		"key_path", "ca_cert_path", "signing_key_file", "encryption_key_path", "passphrase_file", "access_key_id",
		"authorization_enabled", "dsn_file", "psk_path", "pwd_dir", "max_tokens", "keyboard", "dsn_count",
	} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			original := "extra:\n  " + key + ": plain-value-0100\n"

			require.False(t, isCredentialKey("extra", key))
			require.Equal(t, original, blankedConfig(t, original))
		})
	}
}
