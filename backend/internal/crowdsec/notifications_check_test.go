package crowdsec

import (
	"archive/tar"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	plainConfig = "api:\n  server:\n    listen_uri: 127.0.0.1:8080\n"
	httpNotif   = "type: http\nname: %s\nurl: https://example.invalid/\nmethod: POST\n"
)

// profilesNaming returns a profiles stream with one document per entry; an entry is the raw text of
// the notifications value ("" omits the key).
func profilesNaming(values ...string) string {
	docs := make([]string, len(values))
	for i, v := range values {
		docs[i] = fmt.Sprintf("name: p%d\nfilters:\n - Alert.Remediation == true\ndecisions:\n - type: ban\n   duration: 4h\n%s", i, v)
	}
	return strings.Join(docs, "---\n")
}

func list(names ...string) string {
	return "notifications:\n - " + strings.Join(names, "\n - ") + "\n"
}

func notif(name string) string { return fmt.Sprintf(httpNotif, name) }

func writeArchive(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg.tar.gz")
	require.NoError(t, os.WriteFile(p, makeTarGz(t, files), 0o600))
	return p
}

// TestCheckProfileNotificationsMatrix covers the behaviour verified against CrowdSec v1.8.1 with
// `crowdsec -t`: matching by the name field, every document of a file, .yaml and .yml only,
// no subdirectories, and the guards that skip the check instead of refusing.
func TestCheckProfileNotificationsMatrix(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("# padding\n", 110_000) + notif("big")
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"defined by name field", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("x")), "config/notifications/x.yaml": notif("x")}, nil},
		{"root layout", map[string]string{"config.yaml": plainConfig, "profiles.yaml": profilesNaming(list("x")), "notifications/x.yaml": notif("x")}, nil},
		{"root layout missing", map[string]string{"config.yaml": plainConfig, "profiles.yaml": profilesNaming(list("x"))}, []string{"x"}},
		{"config layout ignores root files", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("x")), "notifications/x.yaml": notif("x")}, []string{"x"}},
		{"profiles in root with config layout is not used", map[string]string{"config/config.yaml": plainConfig, "profiles.yaml": profilesNaming(list("x"))}, nil},
		{"missing sorted", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("zeta", "alpha", "mid"))}, []string{"alpha", "mid", "zeta"}},
		{"file name does not match", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("a")), "config/notifications/a.yaml": notif("b")}, []string{"a"}},
		{"name field satisfies other name", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("b")), "config/notifications/a.yaml": notif("b")}, nil},
		{"multi document file defines both", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("m1", "m2")), "config/notifications/m.yaml": notif("m1") + "---\n" + notif("m2")}, nil},
		{"multi document file second name only", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("m2", "m3")), "config/notifications/m.yaml": notif("m1") + "---\n" + notif("m2")}, []string{"m3"}},
		{"yml loaded", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("y")), "config/notifications/y.yml": notif("y")}, nil},
		{"upper case extension not loaded", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("y")), "config/notifications/y.YAML": notif("y")}, []string{"y"}},
		{"other extension not loaded", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("y")), "config/notifications/y.yaml.bak": notif("y"), "config/notifications/y.txt": notif("y")}, []string{"y"}},
		{"extension only file not loaded", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("y")), "config/notifications/.yaml": notif("y")}, []string{"y"}},
		{"subdirectory not loaded", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("s")), "config/notifications/sub/s.yaml": notif("s")}, []string{"s"}},
		{"duplicate names across files", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("d")), "config/notifications/a.yaml": notif("d"), "config/notifications/b.yaml": notif("d")}, nil},
		{"duplicate reference counted once", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("x", "x"), list("x"))}, []string{"x"}},
		{"second profile document", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming("", list("zz"))}, []string{"zz"}},
		{"empty list", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming("notifications: []\n")}, nil},
		{"null value", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming("notifications:\n")}, nil},
		{"no notifications key", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming("")}, nil},
		{"non-list value skips", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming("notifications: x\n")}, nil},
		{"non-string item skips", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming("notifications:\n - 12\n")}, nil},
		{"nested item skips", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming("notifications:\n - [a]\n")}, nil},
		{"empty name skips", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming("notifications:\n - \"\"\n")}, nil},
		{"no profiles file", map[string]string{"config/config.yaml": plainConfig, "config/notifications/x.yaml": notif("x")}, nil},
		{"malformed profiles skips", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": "name: [unclosed\n"}, nil},
		{"malformed notification skips", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("x")), "config/notifications/bad.yaml": "name: x\n\ttype: [\n"}, nil},
		{"empty notification file", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("x")), "config/notifications/e.yaml": ""}, []string{"x"}},
		{"notification without name", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("x")), "config/notifications/n.yaml": "type: http\n"}, []string{"x"}},
		{"alias in profiles", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming("x-list: &l [a]\nnotifications: *l\n"), "config/notifications/a.yaml": notif("a")}, nil},
		{"redirected profiles path skips", map[string]string{"config/config.yaml": "api:\n  server:\n    profiles_path: /elsewhere/profiles.yaml\n", "config/profiles.yaml": profilesNaming(list("x"))}, nil},
		{"redirected notification dir skips", map[string]string{"config/config.yaml": "config_paths:\n  notification_dir: /elsewhere/\n", "config/profiles.yaml": profilesNaming(list("x"))}, nil},
		{"redirected config dir skips", map[string]string{"config/config.yaml": "config_paths:\n  config_dir: /elsewhere/\n", "config/profiles.yaml": profilesNaming(list("x"))}, nil},
		{"non-scalar path skips", map[string]string{"config/config.yaml": "config_paths:\n  notification_dir: [a]\n", "config/profiles.yaml": profilesNaming(list("x"))}, nil},
		{"default paths still checked", map[string]string{"config/config.yaml": "config_paths:\n  config_dir: /etc/crowdsec/\n  notification_dir: /etc/crowdsec/notifications/\napi:\n  server:\n    profiles_path: /etc/crowdsec/profiles.yaml\n", "config/profiles.yaml": profilesNaming(list("x"))}, []string{"x"}},
		{"scalar section does not crash", map[string]string{"config/config.yaml": "config_paths: none\n", "config/profiles.yaml": profilesNaming(list("x"))}, nil},
		{"unparsable config skips", map[string]string{"config/config.yaml": "api: [unclosed\n", "config/profiles.yaml": profilesNaming(list("x"))}, nil},
		{"empty config", map[string]string{"config/config.yaml": "", "config/profiles.yaml": profilesNaming(list("x"))}, []string{"x"}},
		{"oversized notification skips", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("x")), "config/notifications/big.yaml": big}, nil},
		{"oversized profiles skips", map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": big + profilesNaming(list("x"))}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			info, err := InspectArchive(writeArchive(t, tc.files))
			require.NoError(t, err)
			require.Equal(t, tc.want, CheckProfileNotifications(info))
		})
	}
}

func verifyArchiveNotifications(t *testing.T, path string) error {
	t.Helper()
	info, err := InspectArchive(path)
	require.NoError(t, err)
	return VerifyInspectedNotifications(info)
}

func TestVerifyInspectedNotifications(t *testing.T) {
	t.Parallel()
	files := map[string]string{"config/config.yaml": plainConfig, "config/profiles.yaml": profilesNaming(list("b", "a"))}
	err := verifyArchiveNotifications(t, writeArchive(t, files))
	var undefined *UndefinedNotificationsError
	require.ErrorAs(t, err, &undefined)
	require.Equal(t, []string{"a", "b"}, undefined.Names)
	require.EqualError(t, err, "profiles.yaml names notifications that have no matching file: a, b")

	files["config/notifications/n.yaml"] = notif("a") + "---\n" + notif("b")
	require.NoError(t, verifyArchiveNotifications(t, writeArchive(t, files)))
}

func TestUndefinedNotificationsErrorMessage(t *testing.T) {
	t.Parallel()
	names := make([]string, 13)
	for i := range names {
		names[i] = fmt.Sprintf("n%02d", i)
	}
	require.Equal(t,
		"profiles.yaml names notifications that have no matching file: n00, n01, n02, n03, n04, n05, n06, n07, n08, n09, and 3 more",
		(&UndefinedNotificationsError{Names: names}).Error())
	require.Equal(t,
		"profiles.yaml names notifications that have no matching file: n00, n01, n02, n03, n04, n05, n06, n07, n08, n09",
		(&UndefinedNotificationsError{Names: names[:10]}).Error())

	hostile := (&UndefinedNotificationsError{Names: []string{"a\nb\x00c", strings.Repeat("x", 100)}}).Error()
	require.NotContains(t, hostile, "\n")
	require.NotContains(t, hostile, "\x00")
	require.Contains(t, hostile, strings.Repeat("x", 64)+"...")
	require.NotContains(t, hostile, strings.Repeat("x", 65))
}

func TestInspectArchiveReadsOnlyNamedEntries(t *testing.T) {
	t.Parallel()
	raw := tarGzRaw(t, func(tw *tar.Writer) {
		add := func(h *tar.Header, body string) {
			h.Size = int64(len(body))
			if h.Typeflag == 0 {
				h.Typeflag = tar.TypeReg
			}
			require.NoError(t, tw.WriteHeader(h))
			_, err := tw.Write([]byte(body))
			require.NoError(t, err)
		}
		add(&tar.Header{Name: "./config/config.yaml", Mode: 0o600}, "a: 1\n")
		add(&tar.Header{Name: "x/../config/profiles.yaml", Mode: 0o600}, "first\n")
		add(&tar.Header{Name: "config/profiles.yaml", Mode: 0o600}, "second\n")
		add(&tar.Header{Name: "config/notifications/n.yml", Mode: 0o600}, "n\n")
		add(&tar.Header{Name: "config/other.yaml", Mode: 0o600}, "other\n")
		add(&tar.Header{Name: "config/notifications/link.yaml", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}, "")
		add(&tar.Header{Name: "../config/profiles.yaml", Mode: 0o600}, "escape\n")
		add(&tar.Header{Name: ManifestEntryName, Mode: 0o600}, "{}")
		add(&tar.Header{Name: "config/notifications/", Typeflag: tar.TypeDir, Mode: 0o700}, "")
	})
	p := filepath.Join(t.TempDir(), "cfg.tar.gz")
	require.NoError(t, os.WriteFile(p, raw, 0o600))
	info, err := InspectArchive(p)
	require.NoError(t, err)
	require.False(t, info.Incomplete)
	require.Equal(t, map[string][]byte{
		"config/config.yaml":         []byte("a: 1\n"),
		"config/profiles.yaml":       []byte("second\n"),
		"config/notifications/n.yml": []byte("n\n"),
	}, info.Files)
}

func TestInspectArchiveBounds(t *testing.T) {
	t.Parallel()
	t.Run("too many notification files", func(t *testing.T) {
		t.Parallel()
		files := map[string]string{}
		for i := 0; i <= maxInspectEntries; i++ {
			files[fmt.Sprintf("notifications/n%03d.yaml", i)] = "name: x\n"
		}
		info, err := InspectArchive(writeArchive(t, files))
		require.NoError(t, err)
		require.True(t, info.Incomplete)
		require.Len(t, info.Files, maxInspectEntries)
	})
	t.Run("total bytes", func(t *testing.T) {
		t.Parallel()
		files := map[string]string{}
		body := strings.Repeat("#", int(maxInspectEntryBytes)-1)
		for i := 0; i < 9; i++ {
			files[fmt.Sprintf("notifications/n%d.yaml", i)] = body
		}
		info, err := InspectArchive(writeArchive(t, files))
		require.NoError(t, err)
		require.True(t, info.Incomplete)
		require.Len(t, info.Files, 8)
	})
	t.Run("entry larger than its header claims", func(t *testing.T) {
		t.Parallel()
		info, err := InspectArchive(writeArchive(t, map[string]string{"profiles.yaml": strings.Repeat("#", int(maxInspectEntryBytes)+1)}))
		require.NoError(t, err)
		require.True(t, info.Incomplete)
		require.Empty(t, info.Files)
	})
}

func TestInspectArchiveErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := InspectArchive(filepath.Join(dir, "absent"))
	require.ErrorContains(t, err, "open archive")
	require.NotContains(t, err.Error(), dir)

	notGzip := filepath.Join(dir, "plain")
	require.NoError(t, os.WriteFile(notGzip, []byte("plain text"), 0o600))
	_, err = InspectArchive(notGzip)
	require.ErrorContains(t, err, "gzip reader")

	raw := makeTarGz(t, map[string]string{"profiles.yaml": "name: p\n"})
	truncated := filepath.Join(dir, "cut.tar.gz")
	require.NoError(t, os.WriteFile(truncated, raw[:len(raw)-12], 0o600))
	_, err = InspectArchive(truncated)
	require.Error(t, err)
}

func TestArchiveEntryRel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"config.yaml", "config.yaml", true},
		{"./config.yaml", "config.yaml", true},
		{"a/../config.yaml", "config.yaml", true},
		{"config//notifications/x.yaml", "config/notifications/x.yaml", true},
		{"dir/", "dir", true},
		{"/abs.yaml", "abs.yaml", true},
		{"../x", "", false},
		{"a/../../x", "", false},
		{".", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := ArchiveEntryRel(tc.in)
		require.Equal(t, tc.ok, ok, tc.in)
		require.Equal(t, tc.want, got, tc.in)
	}
}

func TestParseYAMLDocuments(t *testing.T) {
	t.Parallel()
	docs, err := ParseYAMLDocuments(strings.NewReader(""), false)
	require.NoError(t, err)
	require.Empty(t, docs)

	docs, err = ParseYAMLDocuments(strings.NewReader("a: 1\n---\nb: 2\n"), false)
	require.NoError(t, err)
	require.Len(t, docs, 2)

	docs, err = ParseYAMLDocuments(strings.NewReader("a: 1\n---\nb: [\n"), true)
	require.NoError(t, err)
	require.Len(t, docs, 1)

	_, err = ParseYAMLDocuments(strings.NewReader("a: 1\n---\nb: [\n"), false)
	require.ErrorContains(t, err, "decode yaml document")
	_, err = ParseYAMLDocuments(strings.NewReader("a: 1\na: 2\n"), false)
	require.ErrorContains(t, err, "decode yaml value")
}
