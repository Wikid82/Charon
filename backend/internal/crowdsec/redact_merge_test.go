package crowdsec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Wikid82/charon/backend/internal/logger"
)

const (
	liveMarker  = "LIVE-SECRET-1234"
	headerLine  = "# " + redactHeaderComment
	mergeConfig = "api:\n  server:\n    listen_uri: 0.0.0.0:8080\n"
)

// secretKeys is the non-target key each plugin type uses to carry the value that gets merged.
var secretKeys = map[string]string{
	"http": "headers:\n  Authorization", "slack": "icon_url", "splunk": "token", "email": "smtp_password",
	"sentinel": "shared_key", "mystery": "secret",
}

// mergeDoc builds one notification document of the given type with extra lines and a secret line.
func mergeDoc(typ, secret string, lines ...string) string {
	body := "type: " + typ + "\nname: hook\n"
	for _, l := range lines {
		body += l + "\n"
	}
	return body + secretKeys[typ] + ": " + secret + "\n"
}

func blankedFile(typ string, lines ...string) string {
	return headerLine + "\n" + mergeDoc(typ, RedactedPlaceholder, lines...)
}

// mergeTrees returns a snapshot and a data folder holding the given files.
func mergeTrees(t *testing.T, live, archive map[string]string) (snap, data string) {
	t.Helper()
	return writeTree(t, live), writeTree(t, archive)
}

func relPath(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

func TestMergeKeepsCurrentValuesForBlankedFile(t *testing.T) {
	t.Parallel()
	snap, data := mergeTrees(t,
		map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker, "url: https://hooks.example.invalid/a")},
		map[string]string{"notifications/hook.yaml": blankedFile("http", "url: https://hooks.example.invalid/a")})

	require.NoError(t, MergeRedactedNotifications(snap, data))

	got := readFile(t, relPath(data, "notifications/hook.yaml"))
	require.Contains(t, got, liveMarker)
	require.NotContains(t, got, RedactedPlaceholder)
	require.NotContains(t, got, redactHeaderComment)
}

func TestMergeDecidesByTargetOfTheNotification(t *testing.T) {
	t.Parallel()
	const a = "url: https://hooks.example.invalid/a"
	rows := []struct {
		name         string
		typ          string
		live         []string
		archive      []string
		wantFilledIn bool
	}{
		{"http same address", "http", []string{a}, []string{a}, true},
		{"http blanked address", "http", []string{a}, []string{"url: " + RedactedPlaceholder}, true},
		{"http no target keys at all", "http", nil, nil, true},
		{"http different address", "http", []string{a}, []string{"url: https://hooks.example.invalid/b"}, false},
		{"http trailing slash differs", "http", []string{a}, []string{a + "/"}, false},
		{"http letter case differs", "http", []string{a}, []string{"url: https://HOOKS.example.invalid/a"}, false},
		{"http address only in archive", "http", nil, []string{a}, false},
		{"http address only in current", "http", []string{a}, nil, false},
		{"http tls verification differs", "http", []string{a, "skip_tls_verification: false"}, []string{a, "skip_tls_verification: true"}, false},
		{"http tls verification equal", "http", []string{a, "skip_tls_verification: true"}, []string{a, "skip_tls_verification: true"}, true},
		{"http ca path only in current", "http", []string{a, "ca_cert_path: /ca.pem"}, []string{a}, false},
		{"http ca path only in archive", "http", []string{a}, []string{a, "ca_cert_path: /ca.pem"}, false},
		{"http cert path differs", "http", []string{a, "cert_path: /a.pem"}, []string{a, "cert_path: /b.pem"}, false},
		{"http key path differs", "http", []string{a, "key_path: /a.key"}, []string{a, "key_path: /b.key"}, false},
		{"http unix socket differs", "http", []string{a, "unix_socket: /a.sock"}, []string{a, "unix_socket: /b.sock"}, false},
		{"http method is not part of the target", "http", []string{a, "method: POST"}, []string{a, "method: PUT"}, true},
		{"http address as list in archive", "http", []string{a}, []string{"url: [x]"}, false},
		{"http address as mapping in current", "http", []string{"url: {x: y}"}, []string{"url: " + RedactedPlaceholder}, false},
		{"http address as alias in current", "http", []string{"other: &u https://hooks.example.invalid/a", "url: *u"}, []string{a}, false},
		{"email same port", "email", []string{"smtp_port: 25"}, []string{"smtp_port: 25"}, true},
		{"email port written differently", "email", []string{"smtp_port: 25"}, []string{`smtp_port: "25"`}, false},
		{"email quoted port equal", "email", []string{`smtp_port: "25"`}, []string{`smtp_port: "25"`}, true},
		{"email host differs", "email", []string{"smtp_host: a.example.invalid"}, []string{"smtp_host: b.example.invalid"}, false},
		{"email encryption differs", "email", []string{"encryption_type: ssltls"}, []string{"encryption_type: none"}, false},
		{"email auth differs", "email", []string{"auth_type: plain"}, []string{"auth_type: login"}, false},
		{"email helo host differs", "email", []string{"helo_host: a.example.invalid"}, []string{"helo_host: b.example.invalid"}, false},
		{"email sender is not part of the target", "email", []string{"sender_email: a@example.invalid"}, []string{"sender_email: b@example.invalid"}, true},
		{"slack blanked webhook", "slack", []string{"webhook: https://hooks.example.invalid/a"}, []string{"webhook: " + RedactedPlaceholder}, true},
		{"slack different webhook", "slack", []string{"webhook: https://hooks.example.invalid/a"}, []string{"webhook: https://hooks.example.invalid/b"}, false},
		{"splunk different address", "splunk", []string{a}, []string{"url: https://hooks.example.invalid/b"}, false},
		{"splunk same address", "splunk", []string{a}, []string{a}, true},
		{"sentinel different workspace", "sentinel", []string{"customer_id: one"}, []string{"customer_id: two"}, false},
		{"sentinel blanked workspace", "sentinel", []string{"customer_id: one"}, []string{"customer_id: " + RedactedPlaceholder}, true},
		{"unknown type same other keys", "mystery", []string{"extra: a"}, []string{"extra: a"}, true},
		{"unknown type different other keys", "mystery", []string{"extra: a"}, []string{"extra: b"}, false},
		{"unknown type key only in current", "mystery", []string{"extra: a"}, nil, false},
		{"unknown type key only in archive", "mystery", nil, []string{"extra: a"}, false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			archive := blankedFile(row.typ, row.archive...)
			snap, data := mergeTrees(t,
				map[string]string{"notifications/hook.yaml": mergeDoc(row.typ, liveMarker, row.live...)},
				map[string]string{"notifications/hook.yaml": archive})

			require.NoError(t, MergeRedactedNotifications(snap, data))

			got := readFile(t, relPath(data, "notifications/hook.yaml"))
			if row.wantFilledIn {
				require.Contains(t, got, liveMarker)
				require.NotContains(t, got, redactHeaderComment)
				return
			}
			require.Equal(t, archive, got)
			names, err := ScanPlaceholders(data)
			require.NoError(t, err)
			require.Equal(t, []string{"notifications/hook.yaml"}, names)
		})
	}
}

func TestMergeLeavesFreshHostPlaceholdersAndReportsTheFile(t *testing.T) {
	t.Parallel()
	archive := blankedFile("http", "url: https://hooks.example.invalid/a")
	snap, data := mergeTrees(t, map[string]string{"config.yaml": mergeConfig}, map[string]string{"notifications/hook.yaml": archive})

	require.NoError(t, MergeRedactedNotifications(snap, data))

	require.Equal(t, archive, readFile(t, relPath(data, "notifications/hook.yaml")))
	names, err := ScanPlaceholders(data)
	require.NoError(t, err)
	require.Equal(t, []string{"notifications/hook.yaml"}, names)
}

func TestMergeNeedsTheSameTypeAndASingleCurrentDefinition(t *testing.T) {
	t.Parallel()
	archive := blankedFile("http", "url: https://hooks.example.invalid/a")
	cases := map[string]map[string]string{
		"other type under the same name": {"notifications/hook.yaml": mergeDoc("splunk", liveMarker, "url: https://hooks.example.invalid/a")},
		"name defined twice in one file": {"notifications/hook.yaml": mergeDoc("http", liveMarker) + "---\n" + mergeDoc("http", liveMarker)},
		"name defined in two files": {
			"notifications/a.yaml": mergeDoc("http", liveMarker),
			"notifications/b.yaml": mergeDoc("http", liveMarker),
		},
		"current definition uses a merge key": {"notifications/hook.yaml": "base: &b {url: 'https://hooks.example.invalid/a'}\ntype: http\nname: hook\n<<: *b\nheaders:\n  Authorization: " + liveMarker + "\n"},
		"other name":                          {"notifications/hook.yaml": strings.Replace(mergeDoc("http", liveMarker), "name: hook", "name: other", 1)},
	}
	for name, live := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			snap, data := mergeTrees(t, live, map[string]string{"notifications/hook.yaml": archive})

			require.NoError(t, MergeRedactedNotifications(snap, data))

			require.Equal(t, archive, readFile(t, relPath(data, "notifications/hook.yaml")))
		})
	}
}

func TestMergeFindsTheCurrentDefinitionInAnyFile(t *testing.T) {
	t.Parallel()
	snap, data := mergeTrees(t,
		map[string]string{"notifications/renamed.yaml": mergeDoc("http", liveMarker)},
		map[string]string{"notifications/hook.yaml": blankedFile("http")})

	require.NoError(t, MergeRedactedNotifications(snap, data))

	require.Contains(t, readFile(t, relPath(data, "notifications/hook.yaml")), liveMarker)
}

func TestMergeNeverFillsDocumentsWithAliasesOrMergeKeys(t *testing.T) {
	t.Parallel()
	live := map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker, "url: https://hooks.example.invalid/a")}
	cases := map[string]string{
		"alias": "type: http\nname: hook\nurl: https://hooks.example.invalid/a\nheaders: &h\n  Authorization: " + RedactedPlaceholder + "\nmore: *h\n",
		"merge key": "base: &b {method: POST}\ntype: http\nname: hook\nurl: https://hooks.example.invalid/a\n<<: *b\nheaders:\n  Authorization: " +
			RedactedPlaceholder + "\n",
	}
	for name, archive := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			snap, data := mergeTrees(t, live, map[string]string{"notifications/hook.yaml": archive})

			require.NoError(t, MergeRedactedNotifications(snap, data))

			require.Equal(t, archive, readFile(t, relPath(data, "notifications/hook.yaml")))
			names, err := ScanPlaceholders(data)
			require.NoError(t, err)
			require.Equal(t, []string{"notifications/hook.yaml"}, names)
		})
	}
}

func TestMergeKeepsEditedValuesAndAddsNoKeys(t *testing.T) {
	t.Parallel()
	live := mergeDoc("http", liveMarker, "url: https://hooks.example.invalid/a", "method: PUT", "log_level: debug")
	archive := headerLine + "\ntype: http\nname: hook\nurl: https://hooks.example.invalid/a\nmethod: POST\nheaders:\n  Authorization: " +
		RedactedPlaceholder + "\n  X-Edited: edited-by-hand\n"
	snap, data := mergeTrees(t, map[string]string{"notifications/hook.yaml": live}, map[string]string{"notifications/hook.yaml": archive})

	require.NoError(t, MergeRedactedNotifications(snap, data))

	got := readFile(t, relPath(data, "notifications/hook.yaml"))
	require.Contains(t, got, liveMarker)
	require.Contains(t, got, "method: POST")
	require.Contains(t, got, "X-Edited: edited-by-hand")
	require.NotContains(t, got, "log_level")
	require.NotContains(t, got, "PUT")
}

func TestMergeKeepsHeaderWhilePlaceholdersRemain(t *testing.T) {
	t.Parallel()
	live := mergeDoc("http", liveMarker, "url: https://hooks.example.invalid/a")
	archive := headerLine + "\ntype: http\nname: hook\nurl: https://hooks.example.invalid/a\nheaders:\n  Authorization: " +
		RedactedPlaceholder + "\n  X-Extra: " + RedactedPlaceholder + "\n"
	snap, data := mergeTrees(t, map[string]string{"notifications/hook.yaml": live}, map[string]string{"notifications/hook.yaml": archive})

	require.NoError(t, MergeRedactedNotifications(snap, data))

	got := readFile(t, relPath(data, "notifications/hook.yaml"))
	require.Contains(t, got, liveMarker)
	require.Equal(t, 1, strings.Count(got, redactHeaderComment))
	require.Equal(t, 1, strings.Count(got, RedactedPlaceholder))
}

func TestMergeFillsEachDocumentOfAFileOnItsOwn(t *testing.T) {
	t.Parallel()
	second := strings.Replace(blankedFile("splunk", "url: https://s.example.invalid"), headerLine+"\n", "", 1)
	second = strings.Replace(second, "name: hook", "name: second", 1)
	snap, data := mergeTrees(t,
		map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker)},
		map[string]string{"notifications/hook.yaml": blankedFile("http") + "---\n" + second})

	require.NoError(t, MergeRedactedNotifications(snap, data))

	got := readFile(t, relPath(data, "notifications/hook.yaml"))
	require.Contains(t, got, liveMarker)
	require.Equal(t, 1, strings.Count(got, RedactedPlaceholder))
	require.Equal(t, 1, strings.Count(got, redactHeaderComment))
}

func TestMergeFillsABlankedValueThatCarriesAnAnchor(t *testing.T) {
	t.Parallel()
	archive := "type: http\nname: hook\nheaders:\n  Authorization: &a " + RedactedPlaceholder + "\n"
	snap, data := mergeTrees(t,
		map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker)},
		map[string]string{"notifications/hook.yaml": archive})

	require.NoError(t, MergeRedactedNotifications(snap, data))

	require.Contains(t, readFile(t, relPath(data, "notifications/hook.yaml")), liveMarker)
}

func TestMergeKeepsFilesWithoutPlaceholdersByteIdentical(t *testing.T) {
	t.Parallel()
	odd := "type:   http   # untouched\nname: hook\nurl:    https://hooks.example.invalid/a\n"
	snap, data := mergeTrees(t,
		map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker)},
		map[string]string{"notifications/hook.yaml": odd, "notifications/notes.txt": RedactedPlaceholder})
	before, err := os.Stat(relPath(data, "notifications/hook.yaml"))
	require.NoError(t, err)

	require.NoError(t, MergeRedactedNotifications(snap, data))

	require.Equal(t, odd, readFile(t, relPath(data, "notifications/hook.yaml")))
	after, err := os.Stat(relPath(data, "notifications/hook.yaml"))
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "an untouched file must not be rewritten")
	require.Equal(t, RedactedPlaceholder, readFile(t, relPath(data, "notifications/notes.txt")))
}

func TestMergeDoesNotFillFromCurrentPlaceholders(t *testing.T) {
	t.Parallel()
	archive := blankedFile("http", "url: https://hooks.example.invalid/a")
	snap, data := mergeTrees(t,
		map[string]string{"notifications/hook.yaml": mergeDoc("http", RedactedPlaceholder, "url: https://hooks.example.invalid/a")},
		map[string]string{"notifications/hook.yaml": archive})

	require.NoError(t, MergeRedactedNotifications(snap, data))

	require.Equal(t, archive, readFile(t, relPath(data, "notifications/hook.yaml")))
}

func TestMergeSkipsCurrentFilesThatDoNotParse(t *testing.T) {
	t.Parallel()
	archive := blankedFile("http")
	snap, data := mergeTrees(t,
		map[string]string{
			"notifications/broken.yaml": "type: http\nname: hook\n  bad: [\n",
			"notifications/hook.yaml":   mergeDoc("http", liveMarker),
		},
		map[string]string{"notifications/hook.yaml": archive})

	require.NoError(t, MergeRedactedNotifications(snap, data))

	require.Contains(t, readFile(t, relPath(data, "notifications/hook.yaml")), liveMarker)
}

func TestMergeWritesTheCurrentFilesModeCappedToOwnerOnly(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ live, archive, want os.FileMode }{
		"archive entry is world readable":  {0o600, 0o644, 0o600},
		"current file is group readable":   {0o640, 0o644, 0o600},
		"current file is read only":        {0o400, 0o666, 0o400},
		"current file is world accessible": {0o666, 0o644, 0o600},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			snap, data := mergeTrees(t,
				map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker)},
				map[string]string{"notifications/hook.yaml": blankedFile("http")})
			require.NoError(t, os.Chmod(relPath(snap, "notifications/hook.yaml"), c.live))
			require.NoError(t, os.Chmod(relPath(data, "notifications/hook.yaml"), c.archive))

			require.NoError(t, MergeRedactedNotifications(snap, data))

			info, err := os.Stat(relPath(data, "notifications/hook.yaml"))
			require.NoError(t, err)
			require.Equal(t, c.want, info.Mode().Perm())
		})
	}
}

func TestMergeSelectsFilesLikeTheExport(t *testing.T) {
	t.Parallel()
	redirect := "config_paths:\n  notification_dir: %s\n"
	live := map[string]string{
		"config/config.yaml":                  mergeConfig,
		"config/notifications/sub/hook.yaml":  mergeDoc("http", liveMarker),
		"config/custom_notify/hook2.yaml":     strings.Replace(mergeDoc("http", liveMarker), "name: hook", "name: hook2", 1),
		"config/notifications.bak/hook3.yaml": strings.Replace(mergeDoc("http", liveMarker), "name: hook", "name: hook3", 1),
	}
	data := writeTree(t, map[string]string{"config/config.yaml": "x: y\n"})
	archive := map[string]string{
		"config/config.yaml":                  fmt.Sprintf(redirect, filepath.Join(data, "config", "custom_notify")),
		"config/notifications/sub/hook.yaml":  blankedFile("http"),
		"config/custom_notify/hook2.yaml":     strings.Replace(blankedFile("http"), "name: hook", "name: hook2", 1),
		"config/notifications.bak/hook3.yaml": strings.Replace(blankedFile("http"), "name: hook", "name: hook3", 1),
		"config/parsers/other.yaml":           "type: mystery\nname: hook\nsecret: " + RedactedPlaceholder + "\n",
	}
	for rel, body := range archive {
		p := relPath(data, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	}
	snap := writeTree(t, live)

	require.NoError(t, MergeRedactedNotifications(snap, data))

	for _, rel := range []string{"config/notifications/sub/hook.yaml", "config/custom_notify/hook2.yaml", "config/notifications.bak/hook3.yaml"} {
		require.Contains(t, readFile(t, relPath(data, rel)), liveMarker, rel)
	}
	require.Contains(t, readFile(t, relPath(data, "config/parsers/other.yaml")), RedactedPlaceholder)
}

func TestScanAndMergeResolveTheSameFolders(t *testing.T) {
	t.Parallel()
	redirect := func(v string) string { return "config_paths:\n  notification_dir: " + v + "\n" }
	rows := []struct {
		name   string
		config func(data string) string
		rel    string
		picked bool
	}{
		{"redirected folder inside the tree", func(d string) string { return redirect(filepath.Join(d, "alerts")) }, "alerts/odd.yaml", true},
		{"rejected redirect falls back to the defaults", func(string) string { return redirect("${HOME}/alerts") }, "alerts/odd.yaml", false},
		{"redirect at a content folder is rejected", func(d string) string { return redirect(filepath.Join(d, "parsers")) }, "parsers/odd.yaml", false},
		{"no config file", func(string) string { return "" }, "alerts/odd.yaml", false},
		{"default folder without a config file", func(string) string { return "" }, "notifications/odd.yaml", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			// A type that is not a known plugin, so only the folder rules can select the file.
			body := "type: mystery\nname: hook\nsecret: %s\n"
			data := t.TempDir()
			snap := t.TempDir()
			if cfg := row.config(data); cfg != "" {
				writeFile(t, filepath.Join(data, "config.yaml"), cfg)
				writeFile(t, filepath.Join(snap, "config.yaml"), cfg)
			}
			writeFile(t, relPath(data, row.rel), fmt.Sprintf(body, RedactedPlaceholder))
			writeFile(t, relPath(snap, row.rel), fmt.Sprintf(body, liveMarker))
			if strings.HasPrefix(row.rel, "alerts/") && row.config(snap) != "" {
				writeFile(t, filepath.Join(snap, "config.yaml"), row.config(snap))
			}

			names, err := ScanPlaceholders(data)
			require.NoError(t, err)
			require.NoError(t, MergeRedactedNotifications(snap, data))

			merged := strings.Contains(readFile(t, relPath(data, row.rel)), liveMarker)
			require.Equal(t, row.picked, merged)
			if row.picked {
				require.Equal(t, []string{row.rel}, names)
			} else {
				require.Empty(t, names)
			}
		})
	}
}

func TestScanReportsFinalFilesByTheirPathBelowTheDataFolder(t *testing.T) {
	t.Parallel()
	ph := "type: http\nname: n\nurl: " + RedactedPlaceholder + "\n"
	cases := map[string]struct {
		files map[string]string
		want  []string
	}{
		"flat layout": {
			map[string]string{"notifications/a.yaml": ph, "notifications/b.yaml": strings.ReplaceAll(ph, RedactedPlaceholder, "x")},
			[]string{"notifications/a.yaml"},
		},
		"nested layout drops the config folder": {
			map[string]string{"config/config.yaml": mergeConfig, "config/notifications/sub/a.yaml": ph, "config/notifications.bak/z.yaml": ph},
			[]string{"notifications.bak/z.yaml", "notifications/sub/a.yaml"},
		},
		"nothing to report": {map[string]string{"config/config.yaml": mergeConfig}, []string{}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			names, err := ScanPlaceholders(writeTree(t, c.files))
			require.NoError(t, err)
			require.Equal(t, c.want, append([]string{}, names...))
		})
	}
}

func TestScanCapsTheReportAtFiftyNames(t *testing.T) {
	t.Parallel()
	files := map[string]string{}
	for i := range 60 {
		files[fmt.Sprintf("notifications/n%02d.yaml", i)] = "type: http\nname: n" + fmt.Sprint(i) + "\nurl: " + RedactedPlaceholder + "\n"
	}

	names, err := ScanPlaceholders(writeTree(t, files))

	require.NoError(t, err)
	require.Len(t, names, maxNeedsValues)
	require.Equal(t, "notifications/n00.yaml", names[0])
}

func TestCollectNotificationFilesStopsAtTheFileLimit(t *testing.T) {
	t.Parallel()
	files := map[string]string{}
	for i := range maxInspectEntries + 5 {
		files[fmt.Sprintf("notifications/n%03d.yaml", i)] = "type: http\nname: x\n"
	}
	root := writeTree(t, files)

	found, err := collectNotificationFiles(root, defaultNotificationDirs)

	require.NoError(t, err)
	require.Len(t, found, maxInspectEntries)
}

func TestCollectNotificationFilesSkipsWhatCannotBeExamined(t *testing.T) {
	t.Parallel()
	many := "type: http\nname: x\nlist:\n" + strings.Repeat("- a\n", maxRedactNodes+10)
	root := writeTree(t, map[string]string{ //nolint:gosec // G101: fixture file names, no credentials
		"notifications/ok.yaml":             "type: http\nname: ok\n",
		"notifications/broken.yaml":         "type: [\n",
		"notifications/big.yaml":            "type: http\n# " + strings.Repeat("a", int(maxInspectEntryBytes)) + "\n",
		"notifications/many.yaml":           many,
		"notifications/notes.txt":           "type: http\nname: x\n",
		"config/hub/x.yaml":                 "type: http\nname: hub\n",
		"config/local_api_credentials.yaml": "type: http\nname: creds\n",
		"parsers/typeless.yaml":             "name: x\n",
		"notifications/dir.yaml/in.txt":     "x",
	})
	require.NoError(t, os.Symlink(relPath(root, "notifications/ok.yaml"), relPath(root, "notifications/link.yaml")))

	found, err := collectNotificationFiles(root, defaultNotificationDirs)

	require.NoError(t, err)
	var names []string
	for _, f := range found {
		names = append(names, f.rel)
	}
	require.Equal(t, []string{"notifications/ok.yaml"}, names)
}

func TestMergeAndScanNeverWriteOrReadThroughLinkedFolders(t *testing.T) {
	t.Parallel()
	outside := writeTree(t, map[string]string{"hook.yaml": blankedFile("http")})
	snap := writeTree(t, map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker)})
	data := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(data, "notifications")))
	before := readFile(t, filepath.Join(outside, "hook.yaml"))

	require.NoError(t, MergeRedactedNotifications(snap, data))
	names, err := ScanPlaceholders(data)

	require.NoError(t, err)
	require.Empty(t, names)
	require.Equal(t, before, readFile(t, filepath.Join(outside, "hook.yaml")))
}

func TestMergeFailsWhenTheFolderCannotBeWritten(t *testing.T) {
	t.Parallel()
	snap, data := mergeTrees(t,
		map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker)},
		map[string]string{"notifications/hook.yaml": blankedFile("http")})
	dir := relPath(data, "notifications")
	require.NoError(t, os.Chmod(dir, 0o500))       //nolint:gosec // G302: a folder needs the owner execute bit
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: a folder needs the owner execute bit

	err := MergeRedactedNotifications(snap, data)

	require.Error(t, err)
	require.NotContains(t, err.Error(), liveMarker)
	require.NotContains(t, err.Error(), data)
	require.Contains(t, readFile(t, relPath(data, "notifications/hook.yaml")), RedactedPlaceholder)
}

func TestMergeAndScanReportUnreadableFolders(t *testing.T) {
	t.Parallel()
	snap, data := mergeTrees(t,
		map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker), "locked/x.yaml": "a: b\n"},
		map[string]string{"notifications/hook.yaml": blankedFile("http"), "locked/x.yaml": "a: b\n"})
	for _, root := range []string{snap, data} {
		dir := relPath(root, "locked")
		require.NoError(t, os.Chmod(dir, 0o000))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: a folder needs the owner execute bit
	}

	require.Error(t, MergeRedactedNotifications(snap, data))
	_, err := ScanPlaceholders(data)
	require.Error(t, err)

	require.NoError(t, os.Chmod(relPath(data, "locked"), 0o700)) //nolint:gosec // G302: a folder needs the owner execute bit
	err = MergeRedactedNotifications(snap, data)
	require.Error(t, err, "the current files must be readable too")
}

func TestRewriteNotificationFileRefusesLinksAndKeepsTheOldFileOnFailure(t *testing.T) {
	t.Parallel()
	docs, err := ParseYAMLDocuments(strings.NewReader("a: b\n"), false)
	require.NoError(t, err)

	t.Run("link in the path", func(t *testing.T) {
		t.Parallel()
		outside := t.TempDir()
		root := t.TempDir()
		require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))

		err := rewriteNotificationFile(root, notificationFile{rel: "link/x.yaml", docs: docs}, 0o600)

		require.ErrorIs(t, err, ErrSymlinkInPath)
		require.NoFileExists(t, filepath.Join(outside, "x.yaml"))
	})
	t.Run("destination is a folder", func(t *testing.T) {
		t.Parallel()
		root := writeTree(t, map[string]string{"x.yaml/inner.txt": "keep"})

		err := rewriteNotificationFile(root, notificationFile{rel: "x.yaml", docs: docs}, 0o600)

		require.Error(t, err)
		entries, readErr := os.ReadDir(root)
		require.NoError(t, readErr)
		require.Len(t, entries, 1, "no temporary file may be left behind")
	})
	t.Run("document cannot be encoded", func(t *testing.T) {
		t.Parallel()
		bad := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.AliasNode}}}

		err := rewriteNotificationFile(t.TempDir(), notificationFile{rel: "x.yaml", docs: []*yaml.Node{bad}}, 0o600)

		require.ErrorIs(t, err, ErrRedactUnsafe)
	})
}

func TestWriteAndCloseReportsEachFailure(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(path, nil, 0o600))

	closed, err := os.Open(path) //nolint:gosec // G304: test fixture
	require.NoError(t, err)
	require.NoError(t, closed.Close())
	require.Error(t, writeAndClose(closed, []byte("x"), 0o600), "chmod on a closed file")

	readOnly, err := os.Open(path) //nolint:gosec // G304: test fixture
	require.NoError(t, err)
	require.Error(t, writeAndClose(readOnly, []byte("x"), 0o600), "write on a read-only handle")
}

func TestDropHeaderCommentOnlyRemovesCharonsLine(t *testing.T) {
	t.Parallel()
	docs, err := ParseYAMLDocuments(strings.NewReader("# my own note\na: b\n"), false)
	require.NoError(t, err)
	dropHeaderComment(docs)
	out, err := encodeDocuments(docs)
	require.NoError(t, err)
	require.Contains(t, string(out), "my own note")
	dropHeaderComment(nil)
}

func TestMergeKeepsTheDefinedNamesAndTheProfileCheckPasses(t *testing.T) {
	t.Parallel()
	profiles := "name: p\nfilters:\n - Alert.Remediation == true\ndecisions:\n - type: ban\n   duration: 4h\nnotifications:\n - hook\n"
	snap, data := mergeTrees(t,
		map[string]string{
			"config.yaml": mergeConfig, "profiles.yaml": profiles,
			"notifications/hook.yaml": mergeDoc("http", liveMarker, "url: https://hooks.example.invalid/a"),
		},
		map[string]string{"config.yaml": mergeConfig})
	before, err := InspectTree(snap)
	require.NoError(t, err)

	require.NoError(t, MergeRedactedNotifications(snap, data))
	kept, err := RestoreMissingCompanions(snap, data, "")
	require.NoError(t, err)

	require.Contains(t, kept, "notifications/hook.yaml")
	require.NoError(t, VerifyKeptProfile(data, kept))
	after, err := InspectTree(data)
	require.NoError(t, err)
	namesBefore, _ := definedNotifications(before.Files, "notifications")
	namesAfter, _ := definedNotifications(after.Files, "notifications")
	require.Equal(t, namesBefore, namesAfter)
}

func TestScanReportsRestoredFilesThatStillHoldPlaceholders(t *testing.T) {
	t.Parallel()
	profiles := "name: p\nfilters:\n - Alert.Remediation == true\ndecisions:\n - type: ban\n   duration: 4h\nnotifications:\n - hook\n"
	snap, data := mergeTrees(t,
		map[string]string{
			"config.yaml": mergeConfig, "profiles.yaml": profiles,
			"notifications/hook.yaml": blankedFile("http", "url: https://hooks.example.invalid/a"),
		},
		map[string]string{"config.yaml": mergeConfig})

	require.NoError(t, MergeRedactedNotifications(snap, data))
	kept, err := RestoreMissingCompanions(snap, data, "")
	require.NoError(t, err)
	names, err := ScanPlaceholders(data)

	require.NoError(t, err)
	require.Contains(t, kept, "notifications/hook.yaml")
	require.Equal(t, []string{"notifications/hook.yaml"}, names)
}

func TestMergeAfterRedactionRestoresCurrentValuesExceptWhereStructureIsShared(t *testing.T) {
	t.Parallel()
	live := map[string]string{}
	archive := map[string]string{}
	for name, body := range plantedPlugins {
		live["notifications/"+name] = body
		res, err := RedactNotificationFile([]byte(body))
		require.NoError(t, err)
		archive["notifications/"+name] = string(res.Data)
	}
	snap, data := mergeTrees(t, live, archive)

	require.NoError(t, MergeRedactedNotifications(snap, data))

	names, err := ScanPlaceholders(data)
	require.NoError(t, err)
	require.Equal(t, []string{"notifications/unknown.yaml"}, names, "the nested value of an unknown plugin has no plain counterpart")
	for name, body := range live {
		if strings.HasSuffix(name, "unknown.yaml") {
			continue
		}
		require.Equal(t, decodeAll(t, body), decodeAll(t, readFile(t, relPath(data, name))), name)
	}
}

func decodeAll(t *testing.T, text string) []any {
	t.Helper()
	dec := yaml.NewDecoder(strings.NewReader(text))
	var docs []any
	for {
		var v any
		err := dec.Decode(&v)
		if err != nil {
			require.ErrorContains(t, err, "EOF")
			return docs
		}
		docs = append(docs, v)
	}
}

// Not parallel: it switches the shared logger to debug level to see every message.
func TestMergeLogsHoldNoNotificationContent(t *testing.T) {
	hook := &recordingHook{}
	logger.Log().Logger.AddHook(hook)
	previous := logger.Log().Logger.GetLevel()
	logger.Log().Logger.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() { logger.Log().Logger.SetLevel(previous) })

	snap, data := mergeTrees(t,
		map[string]string{
			"notifications/hook.yaml":   mergeDoc("http", planted+"live", "url: https://hooks.example.invalid/"+planted+"a"),
			"notifications/broken.yaml": "type: http\nurl: " + planted + "broken\n  bad: [\n",
		},
		map[string]string{"notifications/hook.yaml": blankedFile("http", "url: https://hooks.example.invalid/"+planted+"b")})
	require.NoError(t, MergeRedactedNotifications(snap, data))
	_, err := ScanPlaceholders(data)
	require.NoError(t, err)

	hook.mu.Lock()
	defer hook.mu.Unlock()
	require.NotEmpty(t, hook.messages)
	for _, m := range hook.messages {
		require.NotContains(t, m, planted)
	}
}

func TestMergeToleratesAMissingSnapshot(t *testing.T) {
	t.Parallel()
	archive := blankedFile("http")
	data := writeTree(t, map[string]string{"notifications/hook.yaml": archive})

	require.NoError(t, MergeRedactedNotifications(filepath.Join(t.TempDir(), "missing"), data))

	require.Equal(t, archive, readFile(t, relPath(data, "notifications/hook.yaml")))
}

func TestMergeFailsOnAnUnreadableNotificationFile(t *testing.T) {
	t.Parallel()
	snap, data := mergeTrees(t,
		map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker)},
		map[string]string{"notifications/hook.yaml": blankedFile("http")})
	require.NoError(t, os.Chmod(relPath(snap, "notifications/hook.yaml"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(relPath(snap, "notifications/hook.yaml"), 0o600) })

	require.Error(t, MergeRedactedNotifications(snap, data))
}

func TestMergeLeavesDocumentsWithoutAMappingOrIdentityAlone(t *testing.T) {
	t.Parallel()
	archives := map[string]string{
		"scalar document": RedactedPlaceholder + "\n",
		"no name":         "type: http\nurl: " + RedactedPlaceholder + "\n",
		"number as type":  "type: 5\nname: hook\nurl: " + RedactedPlaceholder + "\n",
	}
	for name, archive := range archives {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			snap, data := mergeTrees(t,
				map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker)},
				map[string]string{"notifications/hook.yaml": archive})

			require.NoError(t, MergeRedactedNotifications(snap, data))

			require.Equal(t, archive, readFile(t, relPath(data, "notifications/hook.yaml")))
		})
	}
}

func TestMergeFillsListsPositionByPosition(t *testing.T) {
	t.Parallel()
	live := map[string]string{"notifications/hook.yaml": mergeDoc("http", liveMarker, "extra: [one, two]", "other: {a: b}")}
	archive := map[string]string{"notifications/hook.yaml": blankedFile("http",
		"extra: ["+RedactedPlaceholder+", "+RedactedPlaceholder+", "+RedactedPlaceholder+"]", "other: ["+RedactedPlaceholder+"]")}
	snap, data := mergeTrees(t, live, archive)

	require.NoError(t, MergeRedactedNotifications(snap, data))

	got := readFile(t, relPath(data, "notifications/hook.yaml"))
	require.Contains(t, got, "one")
	require.Contains(t, got, "two")
	require.Equal(t, 2, strings.Count(got, RedactedPlaceholder), "the third item and the mismatched list stay blanked")
}
