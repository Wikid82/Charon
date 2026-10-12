package crowdsec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func parseDocs(t *testing.T, src string) []*yaml.Node {
	t.Helper()
	docs, err := ParseYAMLDocuments(strings.NewReader(src), false)
	require.NoError(t, err)
	return docs
}

func TestIsNotificationFileClasses(t *testing.T) {
	t.Parallel()
	dirs := []string{"notifications", "config/notifications", "custom_notify"}
	httpRoot := "type: http\nname: n\n"
	cases := []struct {
		name string
		rel  string
		root string
		want NotificationFileKind
	}{
		{"default dir yaml", "notifications/http.yaml", "", InDirYAML},
		{"nested layout dir", "config/notifications/http.yml", "", InDirYAML},
		{"any depth", "config/notifications/sub/deeper/http.yaml", "", InDirYAML},
		{"case variants", "Notifications/HTTP.YAML", "", InDirYAML},
		{"notifications at any position", "alerts/Notifications/x.yaml", "", InDirYAML},
		{"hidden yaml name", "notifications/.yaml", "", InDirYAML},
		{"other file in dir", "notifications/sub/x.txt", "", InDirOther},
		{"backup file", "notifications/http.yaml.bak", "", InDirOther},
		{"no extension", "config/notifications/a/b/x", "", InDirOther},
		{"redirected dir", "custom_notify/http.yaml", "", InDirYAML},
		{"redirected dir other", "custom_notify/x.txt", "", InDirOther},
		{"redirected dir is case sensitive", "Custom_Notify/http.yaml", "", NotAFile},
		{"name merely starts with notifications", "notifications.bak/x.yaml", "", NotAFile},
		{"dash suffix", "notifications-old/x.yaml", "", NotAFile},
		{"look alike by type", "notifications.bak/x.yaml", httpRoot, LookAlikeYAML},
		{"look alike type case", "other/x.YML", "type: Slack\n", LookAlikeYAML},
		{"look alike via document node", "x.yaml", httpRoot, LookAlikeYAML},
		{"unknown type", "x.yaml", "type: webhook\n", NotAFile},
		{"typeless", "config/config.yaml", "api:\n  server: {}\n", NotAFile},
		{"double quoted type", "x.yaml", "type: \"http\"\n", LookAlikeYAML},
		{"single quoted type", "x.yaml", "type: 'Slack'\n", LookAlikeYAML},
		{"quoted type key", "x.yaml", "\"type\": http\n", LookAlikeYAML},
		{"flow style mapping", "x.yaml", "{type: http, name: n}\n", LookAlikeYAML},
		{"second document names a plugin", "x.yaml", "name: a\n---\ntype: splunk\n", LookAlikeYAML},
		{"later document after a non mapping", "x.yaml", "- a\n---\ntype: \"email\"\n", LookAlikeYAML},
		{"no document names a plugin", "x.yaml", "type: webhook\n---\ntype: parser\n", NotAFile},
		{"key case is exact", "x.yaml", "Type: http\n", NotAFile},
		{"quoted unknown type", "x.yaml", "type: \"webhook\"\n", NotAFile},
		{"explicit non string tag", "x.yaml", "type: !!binary aHR0cA==\n", NotAFile},
		{"unparsable later document", "x.yaml", "type: http\n---\na: [\n", NotAFile},
		{"aliased type", "x.yaml", "t: &t http\ntype: *t\n", NotAFile},
		{"non scalar type", "x.yaml", "type: [http]\n", NotAFile},
		{"non mapping root", "x.yaml", "- type: http\n", NotAFile},
		{"yaml look alike needs a yaml extension", "x.txt", httpRoot, NotAFile},
		{"unparsed", "x.yaml", "", NotAFile},
		{"dot", ".", "", NotAFile},
		{"escaping path", "../notifications/x.yaml", "", NotAFile},
		{"absolute path", "/notifications/x.yaml", "", NotAFile},
		{"unclean path", "./notifications//x.yaml", "", InDirYAML},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var docs []*yaml.Node
			if tc.root != "" && !strings.Contains(tc.name, "unparsable") {
				docs = parseDocs(t, tc.root)
			} else if tc.root != "" {
				require.Nil(t, LookAlikeDocuments([]byte(tc.root)))
			}
			require.Equal(t, tc.want, IsNotificationFile(tc.rel, docs, dirs))
		})
	}
}

func TestIsNotificationFileEmptyDocument(t *testing.T) {
	t.Parallel()
	require.Equal(t, NotAFile, IsNotificationFile("x.yaml", []*yaml.Node{{Kind: yaml.DocumentNode}}, nil))
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	}
	return dir
}

func cfgWithDir(v string) []byte {
	return []byte("config_paths:\n  notification_dir: " + v + "\n")
}

func TestNotificationDirsResolver(t *testing.T) {
	t.Parallel()
	defaults := []string{"notifications", "config/notifications"}
	nested := map[string]string{"config/config.yaml": "x: 1\n"}
	cases := []struct {
		name  string
		files map[string]string
		cfg   func(root string) []byte
		want  []string
	}{
		{"no config", nested, func(string) []byte { return nil }, defaults},
		{"unparsable config", nested, func(string) []byte { return []byte("a: [\n") }, defaults},
		{"empty stream", nested, func(string) []byte { return []byte("# nothing\n") }, defaults},
		{"no setting", nested, func(string) []byte { return []byte("api: {}\n") }, defaults},
		{"non string setting", nested, func(string) []byte { return []byte("config_paths:\n  notification_dir: [a]\n") }, defaults},
		{"default value", nested, func(string) []byte { return cfgWithDir("/etc/crowdsec/notifications/") }, defaults},
		{"nested mapping of etc path", nested, func(string) []byte { return cfgWithDir("/etc/crowdsec/alerts/") }, append(append([]string{}, defaults...), "config/alerts")},
		{"flat layout does not map etc path", nil, func(string) []byte { return cfgWithDir("/etc/crowdsec/alerts") }, defaults},
		{"path inside the data folder", nested, func(r string) []byte { return cfgWithDir(r + "/custom_notify") }, append(append([]string{}, defaults...), "custom_notify")},
		{"path inside the data folder below config", nested, func(r string) []byte { return cfgWithDir(r + "/config/x/y") }, append(append([]string{}, defaults...), "config/x/y")},
		{"outside the tree", nested, func(string) []byte { return cfgWithDir("/var/lib/other") }, defaults},
		{"relative", nested, func(string) []byte { return cfgWithDir("alerts") }, defaults},
		{"relative with dotdot", nested, func(string) []byte { return cfgWithDir("../alerts") }, defaults},
		{"dotdot inside absolute", nested, func(r string) []byte { return cfgWithDir(r + "/a/../../etc") }, defaults},
		{"empty value", nested, func(string) []byte { return cfgWithDir(`""`) }, defaults},
		{"nul byte", nested, func(r string) []byte { return cfgWithDir(`"` + r + `/a\0b"`) }, defaults},
		{"env value braces", nested, func(string) []byte { return cfgWithDir(`"${DIR}/x"`) }, defaults},
		{"env value plain", nested, func(r string) []byte { return cfgWithDir(`"` + r + `/$DIR"`) }, defaults},
		{"equals data folder", nested, cfgWithDir, defaults},
		{"ancestor of the data folder", nested, func(r string) []byte { return cfgWithDir(filepath.Dir(r)) }, defaults},
		{"equals config root", nested, func(r string) []byte { return cfgWithDir(r + "/config") }, defaults},
		{"equals etc root", nested, func(string) []byte { return cfgWithDir("/etc/crowdsec") }, defaults},
		{"flat config root is the data folder", nil, func(r string) []byte { return cfgWithDir(r + "/") }, defaults},
	}
	for _, d := range []string{"parsers", "scenarios", "collections", "postoverflows", "contexts", "appsec-configs", "appsec-rules", "appsec-data", "patterns", "hub", "data", "Parsers"} {
		cases = append(cases,
			struct {
				name  string
				files map[string]string
				cfg   func(root string) []byte
				want  []string
			}{"content dir " + d, nested, func(string) []byte { return cfgWithDir("/etc/crowdsec/" + d + "/sub") }, defaults},
			struct {
				name  string
				files map[string]string
				cfg   func(root string) []byte
				want  []string
			}{"content dir at top " + d, nested, func(r string) []byte { return cfgWithDir(r + "/" + d) }, defaults},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := writeTree(t, tc.files)
			if tc.files == nil {
				require.NoError(t, os.MkdirAll(root, 0o750))
			}
			require.Equal(t, tc.want, NotificationDirs(root, tc.cfg(root)))
		})
	}
}

func TestNotificationDirsDoesNotMutateDefaults(t *testing.T) {
	t.Parallel()
	root := writeTree(t, map[string]string{"config/config.yaml": "x: 1\n"})
	_ = NotificationDirs(root, cfgWithDir(root+"/extra"))
	require.Equal(t, []string{"notifications", "config/notifications"}, NotificationDirs(root, nil))
}

func TestIsEnvReference(t *testing.T) {
	t.Parallel()
	yes := []string{"${TOKEN}", "$TOKEN", "$_x1", "Bearer ${TOKEN}", "Basic $CREDS", "token ${T}", "Token $T"}
	no := []string{
		"", "$", "${}", "${1A}", "$1A", "plain", "https://hooks.example/${X}/abc", "${A}${B}", "${A} ", " ${A}",
		"bearer ${A}", "Bearer  ${A}", "Bearer Bearer ${A}", "Bearer ", "$A\n", "${A", "$A-b",
		"Bearer " + strings.Repeat("a", 5000), "${" + strings.Repeat("A", 5000) + "}",
	}
	for _, v := range yes {
		require.True(t, IsEnvReference(v), v)
	}
	for _, v := range no {
		require.False(t, IsEnvReference(v), v)
	}
	require.False(t, IsEnvReference("$"+strings.Repeat("A", maxEnvRefBytes)))
	require.True(t, IsEnvReference("$"+strings.Repeat("A", maxEnvRefBytes-1)))
}

func TestLookAlikeDocumentsBounds(t *testing.T) {
	t.Parallel()
	require.Nil(t, LookAlikeDocuments([]byte("name: x\nurl: y\n")), "no type text: not parsed")
	require.Nil(t, LookAlikeDocuments(nil))
	require.Nil(t, LookAlikeDocuments([]byte("type: [\n")))
	require.Nil(t, LookAlikeDocuments([]byte("type: http\n"+strings.Repeat("#", int(maxInspectEntryBytes))+"\n")))
	require.Nil(t, LookAlikeDocuments([]byte("type: x\nlist: ["+strings.Repeat("1,", maxRedactNodes)+"1]\n")))
	require.Len(t, LookAlikeDocuments([]byte("a: 1\n---\ntype: http\n")), 2)
}
