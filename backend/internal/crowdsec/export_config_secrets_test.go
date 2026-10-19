package crowdsec

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Fixtures: testdata/config (the shipped dist config and variants with FAKE secrets only).

func configFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "config", name)) //nolint:gosec // G304: fixture path built from constants
	require.NoError(t, err)
	return string(raw)
}

// configLocations are the places an engine config can live: nested or flat layout, main file or local override.
var configLocations = []string{"config/config.yaml", "config.yaml", "config/config.yaml.local", "config.yaml.local"}

// exportTree exports a tree of files (plus the main config for nested layouts) and returns the entries and the
// slash paths the export left out.
func exportTree(t *testing.T, files map[string]string) (entries map[string]string, omitted []string, archive []byte) {
	t.Helper()
	dir := t.TempDir()
	writeExportTree(t, dir, files)
	var buf bytes.Buffer
	_, omitted, err := WriteExportArchive(context.Background(), dir, &buf)
	require.NoError(t, err)
	return readEntries(t, buf.Bytes()), omitted, buf.Bytes()
}

func yamlMap(t *testing.T, text string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(text), &m), text)
	return m
}

func nested(t *testing.T, m map[string]any, path ...string) any {
	t.Helper()
	var cur any = m
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		require.True(t, ok, "%v is not a mapping at %q", path, p)
		cur = mm[p]
	}
	return cur
}

// requireNoFakeSecret fails when any decompressed byte of the archive holds a FAKE secret of the fixtures.
func requireNoFakeSecret(t *testing.T, archive []byte) {
	t.Helper()
	raw := decompress(t, archive)
	require.NotContains(t, raw, "FAKE-", "a fixture secret reached the archive")
}

func decompress(t *testing.T, archive []byte) string {
	t.Helper()
	var sb strings.Builder
	for name, body := range readEntries(t, archive) {
		sb.WriteString(name)
		sb.WriteString("\n")
		sb.WriteString(body)
	}
	return sb.String()
}

// The main config and its local override are exported as written when they hold no secret value.
func TestExportConfigWithoutSecretsIsByteIdentical(t *testing.T) {
	t.Parallel()
	contents := map[string]string{ //nolint:gosec // G101: fixture text with empty or commented-out values
		"shipped dist config":   configFixture(t, "dist.yaml"),
		"empty secret values":   "# comment kept\ndb_config:\n  type: mysql\n  user: \"\"\n  password: \"\"   # none set\napi:\n  server:\n    auto_registration:\n      token: \"\"\n",
		"placeholders only":     "db_config:\n  type: mysql\n  user: " + RedactedPlaceholder + "\n  password: " + RedactedPlaceholder + "\n",
		"odd formatting":        "common:   {log_level: info,   log_media: file}\n\n\ndb_config:\n    type:    sqlite\n    db_path:   /var/lib/x.db\n",
		"commented-out secrets": "db_config:\n  type: sqlite\n  #user: someone\n  #password: hunter2\n",
	}
	for name, content := range contents {
		for _, loc := range configLocations {
			t.Run(name+"/"+loc, func(t *testing.T) {
				t.Parallel()
				entries, omitted, _ := exportTree(t, map[string]string{loc: content})

				require.Equal(t, content, entries[loc], "no secret value: the file is exported untouched")
				require.NotContains(t, omitted, loc)
			})
		}
	}
}

// A database password and user, the registration token and URL passwords never reach the archive.
func TestExportConfigBlanksDatabaseCredentials(t *testing.T) {
	t.Parallel()
	for _, fixture := range []string{"mysql.yaml", "postgres.yaml"} {
		for _, loc := range configLocations {
			t.Run(fixture+"/"+loc, func(t *testing.T) {
				t.Parallel()
				original := configFixture(t, fixture)
				entries, omitted, archive := exportTree(t, map[string]string{loc: original})

				requireNoFakeSecret(t, archive)
				require.NotContains(t, omitted, loc)
				got := yamlMap(t, entries[loc])
				require.Equal(t, RedactedPlaceholder, nested(t, got, "db_config", "user"))
				require.Equal(t, RedactedPlaceholder, nested(t, got, "db_config", "password"))

				// Everything else is the original, value for value.
				want := yamlMap(t, original)
				db := nested(t, want, "db_config").(map[string]any)
				db["user"], db["password"] = RedactedPlaceholder, RedactedPlaceholder
				require.Equal(t, want, got)
			})
		}
	}
}

func TestExportConfigBlanksTheRegistrationToken(t *testing.T) {
	t.Parallel()
	original := configFixture(t, "auto_registration_token.yaml")
	for _, loc := range configLocations {
		t.Run(loc, func(t *testing.T) {
			t.Parallel()
			entries, _, archive := exportTree(t, map[string]string{loc: original})

			requireNoFakeSecret(t, archive)
			got := yamlMap(t, entries[loc])
			require.Equal(t, RedactedPlaceholder, nested(t, got, "api", "server", "auto_registration", "token"))
			require.Equal(t, true, nested(t, got, "api", "server", "auto_registration", "enable"))
			require.Equal(t, []any{"192.0.2.0/24"}, nested(t, got, "api", "server", "auto_registration", "allowed_ranges"))
		})
	}
}

func TestExportConfigBlanksTheURLPassword(t *testing.T) {
	t.Parallel()
	original := configFixture(t, "url_userinfo.yaml")
	entries, _, archive := exportTree(t, map[string]string{"config/config.yaml": original})

	requireNoFakeSecret(t, archive)
	url, ok := nested(t, yamlMap(t, entries["config/config.yaml"]), "api", "server", "online_client", "url").(string)
	require.True(t, ok)
	require.Contains(t, url, "capi.example.invalid/v3/", "the rest of the URL stays")
	require.Contains(t, url, "https://")
}

func TestExportConfigBlanksTheGenericSecretKeys(t *testing.T) {
	t.Parallel()
	original := configFixture(t, "extra_secret_keys.yaml")
	entries, _, archive := exportTree(t, map[string]string{"config/config.yaml": original})

	requireNoFakeSecret(t, archive)
	got := yamlMap(t, entries["config/config.yaml"])
	for _, key := range []string{"api_key", "apikey", "private_key"} {
		require.Equal(t, RedactedPlaceholder, nested(t, got, "integration", key), key)
	}
	for _, key := range []string{"secret", "passwd", "token"} {
		require.Equal(t, RedactedPlaceholder, nested(t, got, "integration", "client", key), key)
	}
	require.EqualValues(t, 60, nested(t, got, "integration", "tokens_per_minute"), "a setting that merely mentions a token stays")
	require.Equal(t, "unrelated text", nested(t, got, "integration", "note"))
	require.Equal(t, "nobody", nested(t, got, "plugin_config", "user"), "only the database user is a credential")
	require.Equal(t, "info", nested(t, got, "common", "log_level"))
}

// A config whose secret keys cannot be read safely fails the export: leaving it out would give an archive that an
// import refuses, and exporting it as written would carry the credentials.
func TestExportConfigFailsWhenItCannotBeRedacted(t *testing.T) {
	t.Parallel()
	contents := map[string]string{
		"merge key at the credentials": configFixture(t, "anchors_alias.yaml"),
		"alias as the value":           configFixture(t, "anchors_alias_value.yaml"),
		"unparseable":                  "api: [unclosed\ndb_config:\n  password: FAKE-broken-0013\n",
		"too large":                    "db_config:\n  password: FAKE-large-0014\nfiller: " + strings.Repeat("x", int(maxInspectEntryBytes)) + "\n",
	}
	for name, content := range contents {
		for _, loc := range configLocations {
			t.Run(name+"/"+loc, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				writeExportTree(t, dir, map[string]string{loc: content, "config/acquis.yaml": "filenames: [x]\n"})
				var buf bytes.Buffer

				_, _, err := WriteExportArchive(context.Background(), dir, &buf)

				require.ErrorIs(t, err, ErrExportConfigUnsafe)
				require.NotContains(t, err.Error(), "FAKE-")
			})
		}
	}
}

func TestExportConfigLeavesTheLiveFileAlone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	original := configFixture(t, "mysql.yaml")
	writeExportTree(t, dir, map[string]string{"config/config.yaml": original})

	_, _, err := WriteExportArchive(context.Background(), dir, &bytes.Buffer{})

	require.NoError(t, err)
	require.Equal(t, original, readFile(t, filepath.Join(dir, "config", "config.yaml")))
}
