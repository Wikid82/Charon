package crowdsec

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsConfigFile(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"config.yaml", "config.yaml.local", "config/config.yaml", "config/config.yaml.local"} {
		require.True(t, isConfigFile(rel), rel)
	}
	for _, rel := range []string{"config/sub/config.yaml", "notifications/config.yaml", "config.yml", "profiles.yaml", "config/config.yaml.bak"} {
		require.False(t, isConfigFile(rel), rel)
	}
}

func TestRedactConfigFileBlanksCredentials(t *testing.T) {
	t.Parallel()
	res, err := RedactConfigFile([]byte(configFixture(t, "mysql.yaml")))
	require.NoError(t, err)

	out := string(res.Data)
	require.NotContains(t, out, "FAKE-")
	require.NotContains(t, out, "fakeuser")
	require.Equal(t, 2, res.Redacted)
	require.True(t, strings.HasPrefix(out, "# "+redactHeaderComment))
	got := yamlMap(t, out)
	require.Equal(t, RedactedPlaceholder, nested(t, got, "db_config", "user"))
	require.Equal(t, "192.0.2.50", nested(t, got, "db_config", "host"))
}

func TestRedactConfigFileKeepsAFileWithoutCredentialsAsWritten(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{ //nolint:gosec // G101: fake fixture values
		"empty file":        "",
		"comments only":     "# nothing here\n",
		"dist":              configFixture(t, "dist.yaml"),
		"env references":    "db_config:\n  user: ${DB_USER}\n  password: $DB_PASSWORD # from the environment\n",
		"null and empty":    "db_config:\n  user:\n  password: ~\n  token: \"\"\n",
		"empty collections": "integration:\n  secret: {}\n  token: []\n",
		"url without pass":  "url: https://user@example.invalid/path # note\n",
		"url env password":  "url: https://user:${PASSWORD}@example.invalid/\n",
		"plain url":         "url: https://example.invalid/a@b:c\n",
		"not a url":         "text: \"1x://user:pass@host\"\n",
		"placeholder":       "db_config:\n  password: " + RedactedPlaceholder + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res, err := RedactConfigFile([]byte(src))

			require.NoError(t, err)
			require.Equal(t, src, string(res.Data))
			require.Zero(t, res.Redacted)
		})
	}
}

func TestRedactConfigFileKeyRules(t *testing.T) {
	t.Parallel()
	//nolint:gosec // G101: fake fixture values
	src := `common:
  log_level: info
Db_Config:
  USER: FAKE-user-0001
  Password: FAKE-password-0002
  host: 192.0.2.50
plugin_config:
  user: nobody
api:
  server:
    Auto_Registration:
      token: FAKE-token-0003
other:
  list:
    - name: a
      api_key: FAKE-api-key-0004
    - https://u:FAKE-url-pass-0005@example.invalid/x?y=1#z
  secret:
    nested: FAKE-nested-0006
  tokens: 5
`
	res, err := RedactConfigFile([]byte(src))
	require.NoError(t, err)
	require.NotContains(t, string(res.Data), "FAKE-")

	got := yamlMap(t, string(res.Data))
	require.Equal(t, RedactedPlaceholder, nested(t, got, "Db_Config", "USER"))
	require.Equal(t, "nobody", nested(t, got, "plugin_config", "user"))
	require.Equal(t, RedactedPlaceholder, nested(t, got, "other", "secret"), "a mapping under a credential key is replaced as a whole")
	list, ok := nested(t, got, "other", "list").([]any)
	require.True(t, ok)
	require.Equal(t, RedactedPlaceholder, list[0].(map[string]any)["api_key"])
	require.Equal(t, "https://u:"+RedactedPlaceholder+"@example.invalid/x?y=1#z", list[1])
	require.EqualValues(t, 5, nested(t, got, "other", "tokens"))
}

func TestRedactConfigFileMultipleDocuments(t *testing.T) {
	t.Parallel()
	res, err := RedactConfigFile([]byte("a: 1\n---\ndb_config:\n  password: FAKE-second-doc-0001\n"))

	require.NoError(t, err)
	require.NotContains(t, string(res.Data), "FAKE-")
	require.Contains(t, string(res.Data), "a: 1")
}

func TestRedactConfigFileRefusesWhatItCannotRewriteSafely(t *testing.T) {
	t.Parallel()
	cases := map[string]string{ //nolint:gosec // G101: fake fixture values
		"unparseable":                      "api: [unclosed\npassword: FAKE-broken-0001\n",
		"alias as the credential":          configFixture(t, "anchors_alias_value.yaml"),
		"merge key at the database block":  configFixture(t, "anchors_alias.yaml"),
		"anchored credential":              "db_config:\n  password: &pw FAKE-anchored-0001\n",
		"alias in a file with credentials": "x: &a\n  k: 1\ny: *a\ndb_config:\n  password: FAKE-with-alias-0002\n",
		"alias to a user block":            "base: &b\n  user: someone\ndb_config: *b\n",
		"alias in a list":                  "base: &b\n  user: someone\nlist:\n  - *b\n",
		"merged database block":            "base: &b\n  db_config:\n    user: someone\n<<: *b\n",
		"complex key":                      "? [a, b]\n: c\ndb_config:\n  password: FAKE-complex-key-0003\n",
		"file too large":                   "a: " + strings.Repeat("x", int(maxInspectEntryBytes)) + "\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := RedactConfigFile([]byte(src))

			require.ErrorIs(t, err, ErrRedactUnsafe)
			require.NotContains(t, err.Error(), "FAKE-")
		})
	}
}

func TestRedactConfigFileRefusesTooManyNodes(t *testing.T) {
	t.Parallel()
	_, err := RedactConfigFile([]byte("l: [" + strings.Repeat("1,", maxRedactNodes+1) + "1]\n"))

	require.ErrorIs(t, err, ErrRedactUnsafe)
}

func TestRedactConfigFileRefusesAResidualValue(t *testing.T) {
	t.Parallel()
	// The password text is also part of a kept value, so blanking cannot remove it from the file.
	_, err := RedactConfigFile([]byte("db_config:\n  password: crowdsecurity-secret\nlabel: prefix-crowdsecurity-secret-suffix\n"))

	require.ErrorIs(t, err, ErrRedactUnsafe)
}

func TestRedactURLPassword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, out, password string
		ok                bool
	}{
		{"https://user:pw-0123456@host.invalid/p", "https://user:" + RedactedPlaceholder + "@host.invalid/p", "pw-0123456", true},
		{"mysql://u:p@tcp(host:3306)/db", "mysql://u:" + RedactedPlaceholder + "@tcp(host:3306)/db", "p", true},
		{"postgres://:onlypass@host", "postgres://:" + RedactedPlaceholder + "@host", "onlypass", true},
		{"a+b.c-d://u:p@h", "a+b.c-d://u:" + RedactedPlaceholder + "@h", "p", true},
		{"https://u:p:q@h", "https://u:" + RedactedPlaceholder + "@h", "p:q", true},
		{"https://user@host/", "", "", false},
		{"https://user:@host/", "", "", false},
		{"https://host/path@x:y", "", "", false},
		{"https://u:" + RedactedPlaceholder + "@h", "", "", false},
		{"https://u:$PW@h", "", "", false},
		{"1https://u:p@h", "", "", false},
		{"://u:p@h", "", "", false},
		{"h!x://u:p@h", "", "", false},
		{"no scheme", "", "", false},
	}
	for _, tc := range cases {
		out, password, ok := redactURLPassword(tc.in)

		require.Equal(t, tc.ok, ok, tc.in)
		require.Equal(t, tc.password, password, tc.in)
		if ok {
			require.Equal(t, tc.out, out, tc.in)
		} else {
			require.Equal(t, tc.in, out, tc.in)
		}
	}
}

func TestIsBlankedURL(t *testing.T) {
	t.Parallel()
	live := "https://u:FAKE-live-pass-01@h.invalid/x" //nolint:gosec // G101: fake fixture value
	blanked := "https://u:" + RedactedPlaceholder + "@h.invalid/x"

	require.True(t, isBlankedURL(blanked, live))
	require.False(t, isBlankedURL(blanked, "https://u:FAKE-live-pass-01@other.invalid/x"), "another host")
	require.False(t, isBlankedURL(live, live), "no placeholder")
	require.False(t, isBlankedURL(blanked, "https://u@h.invalid/x"), "no password to copy")
}

// --- import merge and scan ---

func writeConfigFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o640)) //nolint:gosec // G306: test fixture
}

func blankedConfig(t *testing.T, original string) string {
	t.Helper()
	res, err := RedactConfigFile([]byte(original))
	require.NoError(t, err)
	return string(res.Data)
}

func TestMergeRedactedConfigFillsFromTheCurrentFile(t *testing.T) {
	t.Parallel()
	for _, fixture := range []string{"mysql.yaml", "auto_registration_token.yaml", "url_userinfo.yaml", "extra_secret_keys.yaml"} {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			original := configFixture(t, fixture)
			snapshot, imported := t.TempDir(), t.TempDir()
			writeConfigFile(t, snapshot, "config/config.yaml", original)
			writeConfigFile(t, imported, "config/config.yaml", blankedConfig(t, original))

			require.NoError(t, MergeRedactedConfig(snapshot, imported))

			got := readFile(t, filepath.Join(imported, "config", "config.yaml"))
			require.Equal(t, yamlMap(t, original), yamlMap(t, got))
			require.NotContains(t, got, RedactedPlaceholder)
			require.NotContains(t, got, redactHeaderComment, "the header goes once nothing is blanked")
			names, err := ScanPlaceholders(imported)
			require.NoError(t, err)
			require.Empty(t, names)
			info, err := os.Stat(filepath.Join(imported, "config", "config.yaml"))
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o640), info.Mode().Perm())
		})
	}
}

func TestMergeRedactedConfigLeavesWhatHasNoCounterpart(t *testing.T) {
	t.Parallel()
	original := configFixture(t, "mysql.yaml")
	cases := map[string]func(t *testing.T, snapshot string){
		"no current file":       func(*testing.T, string) {},
		"current file unparsed": func(t *testing.T, s string) { writeConfigFile(t, s, "config/config.yaml", "a: [unclosed\n") },
		"current has aliases": func(t *testing.T, s string) {
			writeConfigFile(t, s, "config/config.yaml", "x: &a 1\ny: *a\n"+original)
		},
		"another database": func(t *testing.T, s string) {
			writeConfigFile(t, s, "config/config.yaml", strings.Replace(original, "192.0.2.50", "192.0.2.99", 1))
		},
		"database removed": func(t *testing.T, s string) {
			writeConfigFile(t, s, "config/config.yaml", configFixture(t, "dist.yaml"))
		},
		"current also blanked": func(t *testing.T, s string) {
			writeConfigFile(t, s, "config/config.yaml", blankedConfig(t, original))
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			snapshot, imported := t.TempDir(), t.TempDir()
			setup(t, snapshot)
			blanked := blankedConfig(t, original)
			writeConfigFile(t, imported, "config/config.yaml", blanked)

			require.NoError(t, MergeRedactedConfig(snapshot, imported))

			require.Equal(t, blanked, readFile(t, filepath.Join(imported, "config", "config.yaml")), "untouched")
			names, err := ScanPlaceholders(imported)
			require.NoError(t, err)
			require.Equal(t, []string{"config.yaml"}, names)
		})
	}
}

func TestMergeRedactedConfigSkipsAnImportedFileWithAliases(t *testing.T) {
	t.Parallel()
	snapshot, imported := t.TempDir(), t.TempDir()
	writeConfigFile(t, snapshot, "config/config.yaml", configFixture(t, "mysql.yaml"))
	archive := "x: &a 1\ny: *a\ndb_config:\n  type: mysql\n  user: " + RedactedPlaceholder + "\n"
	writeConfigFile(t, imported, "config/config.yaml", archive)

	require.NoError(t, MergeRedactedConfig(snapshot, imported))

	require.Equal(t, archive, readFile(t, filepath.Join(imported, "config", "config.yaml")))
}

func TestMergeRedactedConfigFillsTheSecretsOfAnotherLayout(t *testing.T) {
	t.Parallel()
	original := configFixture(t, "mysql.yaml")
	for name, paths := range map[string][2]string{
		"flat live, nested archive": {"config.yaml", "config/config.yaml"},
		"nested live, flat archive": {"config/config.yaml", "config.yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			snapshot, imported := t.TempDir(), t.TempDir()
			writeConfigFile(t, snapshot, paths[0], original)
			writeConfigFile(t, imported, paths[1], blankedConfig(t, original))

			require.NoError(t, MergeRedactedConfig(snapshot, imported))

			require.Equal(t, yamlMap(t, original), yamlMap(t, readFile(t, filepath.Join(imported, filepath.FromSlash(paths[1])))))
		})
	}
}

func TestMergeRedactedConfigKeepsFilesWithoutPlaceholdersByteIdentical(t *testing.T) {
	t.Parallel()
	snapshot, imported := t.TempDir(), t.TempDir()
	dist := configFixture(t, "dist.yaml")
	writeConfigFile(t, snapshot, "config/config.yaml", configFixture(t, "mysql.yaml"))
	writeConfigFile(t, imported, "config/config.yaml", dist)

	require.NoError(t, MergeRedactedConfig(snapshot, imported))

	require.Equal(t, dist, readFile(t, filepath.Join(imported, "config", "config.yaml")))
}

func TestMergeRedactedConfigFillsOnlyTheBlankedFile(t *testing.T) {
	t.Parallel()
	local := "db_config:\n  type: mysql\n  user: fakeuser\n  password: FAKE-local-0014\n"
	snapshot, imported := t.TempDir(), t.TempDir()
	writeConfigFile(t, snapshot, "config/config.yaml", configFixture(t, "dist.yaml"))
	writeConfigFile(t, snapshot, "config/config.yaml.local", local)
	writeConfigFile(t, imported, "config/config.yaml", configFixture(t, "dist.yaml"))
	writeConfigFile(t, imported, "config/config.yaml.local", blankedConfig(t, local))

	require.NoError(t, MergeRedactedConfig(snapshot, imported))

	require.Equal(t, yamlMap(t, local), yamlMap(t, readFile(t, filepath.Join(imported, "config", "config.yaml.local"))))
}

func TestScanPlaceholdersNamesConfigFiles(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ rel, want string }{
		"nested": {"config/config.yaml.local", "config.yaml.local"},
		"flat":   {"config.yaml", "config.yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfigFile(t, dir, tc.rel, "url: https://u:"+RedactedPlaceholder+"@h.invalid/\n")

			names, err := ScanPlaceholders(dir)

			require.NoError(t, err)
			require.Equal(t, []string{tc.want}, names)
		})
	}
}

func TestMergeRedactedConfigIgnoresEntriesThatAreNotRegularFiles(t *testing.T) {
	t.Parallel()
	for name, setup := range map[string]func(t *testing.T, dir string){
		"directory": func(t *testing.T, dir string) {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "config", "config.yaml"), 0o750))
		},
		"dangling link": func(t *testing.T, dir string) {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o750))
			require.NoError(t, os.Symlink(filepath.Join(dir, "missing.yaml"), filepath.Join(dir, "config", "config.yaml")))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			blanked := blankedConfig(t, configFixture(t, "mysql.yaml"))
			snapshot, imported, other := t.TempDir(), t.TempDir(), t.TempDir()
			setup(t, snapshot)
			writeConfigFile(t, imported, "config/config.yaml", blanked)
			setup(t, other)

			require.NoError(t, MergeRedactedConfig(snapshot, imported), "no current file to fill from")
			require.NoError(t, MergeRedactedConfig(imported, other), "nothing imported to fill")
			require.Equal(t, blanked, readFile(t, filepath.Join(imported, "config", "config.yaml")))
		})
	}
}

func TestExportLeavesOutAConfigAboveTheInspectLimit(t *testing.T) {
	t.Parallel()
	huge := "# " + strings.Repeat("x", int(maxInspectEntryBytes)) + "\na: 1\n"
	entries, omitted, _ := exportTree(t, map[string]string{"config/config.yaml": huge, "config/acquis.yaml": "filenames: [x]\n"})

	require.NotContains(t, entries, "config/config.yaml")
	require.Equal(t, []string{"config/config.yaml"}, omitted)
	require.Contains(t, entries, "config/acquis.yaml")
}

func TestExportConfigAppliesTheSizeLimitToTheRedactedLength(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"config/config.yaml": configFixture(t, "mysql.yaml")})
	redacted, err := RedactConfigFile([]byte(configFixture(t, "mysql.yaml")))
	require.NoError(t, err)

	total, _, err := writeExportArchive(context.Background(), dir, io.Discard, int64(len(redacted.Data)), exportHooks{})
	require.NoError(t, err)
	require.EqualValues(t, len(redacted.Data), total)

	_, _, err = writeExportArchive(context.Background(), dir, io.Discard, int64(len(redacted.Data))-1, exportHooks{})
	require.ErrorIs(t, err, ErrExportTooLarge)
}

func TestExportConfigChangedWhileReading(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"config/config.yaml": configFixture(t, "dist.yaml")})

	_, _, err := writeExportArchive(context.Background(), dir, io.Discard, MaxImportUncompressedBytes,
		exportHooks{beforeOpen: func(path string) { appendTo(path, "# more\n") }})

	require.ErrorIs(t, err, ErrExportFileUnreadable)
}

func TestConfigMergeAndScanFailWhenAPathCannotBeRead(t *testing.T) {
	t.Parallel()
	blanked := blankedConfig(t, configFixture(t, "mysql.yaml"))
	// A regular file named "config" makes every config/... path unreadable (not a directory).
	broken := func(t *testing.T) string {
		dir := t.TempDir()
		writeConfigFile(t, dir, "config", "not a directory")
		return dir
	}

	t.Run("imported tree", func(t *testing.T) {
		t.Parallel()
		err := MergeRedactedConfig(t.TempDir(), broken(t))

		require.ErrorIs(t, err, ErrExportFileUnreadable)
		require.Contains(t, err.Error(), "read the imported configuration")
	})
	t.Run("current tree", func(t *testing.T) {
		t.Parallel()
		imported := t.TempDir()
		writeConfigFile(t, imported, "config.yaml", blanked)

		err := MergeRedactedConfig(broken(t), imported)

		require.ErrorIs(t, err, ErrExportFileUnreadable)
		require.Contains(t, err.Error(), "read the current configuration")
	})
	t.Run("scan", func(t *testing.T) {
		t.Parallel()
		_, err := ScanPlaceholders(broken(t))

		require.ErrorIs(t, err, ErrExportFileUnreadable)
	})
}
