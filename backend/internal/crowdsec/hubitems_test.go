package crowdsec

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// link creates a symlink at dir/rel pointing to target, creating parent directories.
func link(t *testing.T, dir, rel, target string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
	require.NoError(t, os.Symlink(target, p))
}

func TestInstalledHubItemsLinkTargets(t *testing.T) {
	t.Parallel()
	const hub = "/etc/crowdsec/hub/"
	cases := []struct {
		name   string
		rel    string
		target string
		want   *HubItem
	}{
		{"collection", "config/collections/caddy.yaml", hub + "collections/crowdsecurity/caddy.yaml", &HubItem{"collections", "crowdsecurity/caddy"}},
		{"collection shipped as yml", "config/collections/caddy.yml", hub + "collections/crowdsecurity/caddy.yml", &HubItem{"collections", "crowdsecurity/caddy"}},
		{"yml link to yaml target", "config/collections/caddy.yml", hub + "collections/crowdsecurity/caddy.yaml", &HubItem{"collections", "crowdsecurity/caddy"}},
		{"context", "config/contexts/http_base.yaml", hub + "contexts/crowdsecurity/http_base.yaml", &HubItem{"contexts", "crowdsecurity/http_base"}},
		{"scenario", "config/scenarios/CVE-2017-9841.yaml", hub + "scenarios/crowdsecurity/CVE-2017-9841.yaml", &HubItem{"scenarios", "crowdsecurity/CVE-2017-9841"}},
		{"scenario shipped as yml", "config/scenarios/sigmahq-proc.yml", hub + "scenarios/sigmahq/proc.yml", &HubItem{"scenarios", "sigmahq/proc"}},
		{"parser with stage", "config/parsers/s01-parse/caddy-logs.yaml", hub + "parsers/s01-parse/crowdsecurity/caddy-logs.yaml", &HubItem{"parsers", "crowdsecurity/caddy-logs"}},
		{"parser with stage shipped as yml", "config/parsers/s01-parse/caddy-logs.yml", hub + "parsers/s01-parse/crowdsecurity/caddy-logs.yml", &HubItem{"parsers", "crowdsecurity/caddy-logs"}},
		{"postoverflow with stage", "config/postoverflows/s01-whitelist/rdns.yaml", hub + "postoverflows/s01-whitelist/crowdsecurity/rdns.yaml", &HubItem{"postoverflows", "crowdsecurity/rdns"}},
		{"mixed case owner", "config/collections/sftpgo.yaml", hub + "collections/Azlaroc/sftpgo.yaml", &HubItem{"collections", "Azlaroc/sftpgo"}},
		{"root layout", "collections/caddy.yaml", hub + "collections/crowdsecurity/caddy.yaml", &HubItem{"collections", "crowdsecurity/caddy"}},
		{"root layout shipped as yml", "collections/caddy.yml", hub + "collections/crowdsecurity/caddy.yml", &HubItem{"collections", "crowdsecurity/caddy"}},
		{"hub below the data dir", "config/collections/c.yaml", "/app/data/crowdsec/config/hub/collections/o/c.yaml", &HubItem{"collections", "o/c"}},
		{"unclean target", "config/collections/c.yaml", hub + "collections/./x/../o/c.yaml", &HubItem{"collections", "o/c"}},
		{"dangling target is still listed", "config/collections/c.yaml", "/nonexistent/hub/collections/o/c.yaml", &HubItem{"collections", "o/c"}},
		{"upper case yml extension", "config/collections/c.yml", hub + "collections/o/c.YML", nil},
		{"yml alone is not an item", "config/collections/c.yml", hub + "collections/o/.yml", nil},
		{"yaml suffix inside a name is not an extension", "config/collections/c.yml", hub + "collections/o/c.yml.bak", nil},
		{"relative link", "config/collections/c.yaml", "../hub/collections/o/c.yaml", nil},
		{"user link elsewhere", "config/collections/c.yaml", "/home/user/mine/c.yaml", nil},
		{"wrong type directory", "config/collections/c.yaml", hub + "scenarios/o/c.yaml", nil},
		{"stage missing for parser", "config/parsers/p.yaml", hub + "parsers/o/p.yaml", nil},
		{"parser link not in a stage directory", "config/parsers/p.yaml", hub + "parsers/s01-parse/o/p.yaml", nil},
		{"unexpected stage for collection", "config/collections/c.yaml", hub + "collections/s01/o/c.yaml", nil},
		{"link too deep", "config/collections/sub/c.yaml", hub + "collections/o/c.yaml", nil},
		{"staged link too deep", "config/parsers/s01/x/p.yaml", hub + "parsers/s01/o/p.yaml", nil},
		{"not yaml", "config/collections/c.yaml", hub + "collections/o/c.json", nil},
		{"upper case extension", "config/collections/c.yaml", hub + "collections/o/c.YAML", nil},
		{"hub is the last component", "config/collections/c.yaml", hub, nil},
		{"hub missing", "config/collections/c.yaml", "/etc/crowdsec/collections/o/c.yaml", nil},
		{"invalid name", "config/collections/c.yaml", hub + "collections/o/-c.yaml", nil},
		{"too long owner", "config/collections/c.yaml", hub + "collections/" + fmt.Sprintf("%065d", 0) + "/c.yaml", nil},
		{"too short", "config/collections/c.yaml", "/hub/collections", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			link(t, dir, tc.rel, tc.target)
			items, skipped, err := scanHubItems(dir)
			require.NoError(t, err)
			if tc.want == nil {
				require.Empty(t, items)
				require.Equal(t, 1, skipped)
				return
			}
			require.Equal(t, []HubItem{*tc.want}, items)
			require.Zero(t, skipped)
		})
	}
}

func TestInstalledHubItemsIgnoresNonLinksAndSorts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const hub = "/etc/crowdsec/hub/"
	link(t, dir, "config/scenarios/z.yaml", hub+"scenarios/zeta/z.yaml")
	link(t, dir, "config/collections/b.yaml", hub+"collections/crowdsecurity/b.yaml")
	link(t, dir, "config/collections/a.yaml", hub+"collections/Azlaroc/a.yaml")
	link(t, dir, "config/parsers/s02-enrich/p.yaml", hub+"parsers/s02-enrich/crowdsecurity/p.yaml")
	link(t, dir, "config/scenarios/ymlonly.yml", hub+"scenarios/sigmahq/ymlonly.yml")
	// Duplicate through the root layout, a real file, a directory link and an unrelated tree.
	link(t, dir, "collections/b.yaml", hub+"collections/crowdsecurity/b.yaml")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config", "collections", "mine.yaml"), []byte("x"), 0o600))
	link(t, dir, "config/contexts/sub", hub+"contexts/crowdsecurity")
	link(t, dir, "config/other/o.yaml", hub+"collections/o/o.yaml")

	items, err := InstalledHubItems(dir)
	require.NoError(t, err)
	require.Equal(t, []HubItem{
		{"collections", "Azlaroc/a"},
		{"collections", "crowdsecurity/b"},
		{"parsers", "crowdsecurity/p"},
		{"scenarios", "sigmahq/ymlonly"},
		{"scenarios", "zeta/z"},
	}, items)
}

func TestInstalledHubItemsEmptyOrMissingTree(t *testing.T) {
	t.Parallel()
	items, err := InstalledHubItems(t.TempDir())
	require.NoError(t, err)
	require.Empty(t, items)
	items, err = InstalledHubItems(filepath.Join(t.TempDir(), "absent"))
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestInstalledHubItemsCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 0; i < MaxHubItems+25; i++ {
		ext := ".yaml"
		if i%2 == 1 {
			ext = ".yml" // the cap counts both spellings
		}
		link(t, dir, fmt.Sprintf("config/collections/c%04d%s", i, ext), fmt.Sprintf("/etc/crowdsec/hub/collections/o/c%04d%s", i, ext))
	}
	items, err := InstalledHubItems(dir)
	require.NoError(t, err)
	require.Len(t, items, MaxHubItems)
	require.Equal(t, HubItem{"collections", "o/c0000"}, items[0])
	require.Equal(t, HubItem{"collections", fmt.Sprintf("o/c%04d", MaxHubItems-1)}, items[MaxHubItems-1])
}

func TestInstalledHubItemsScanBound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d := filepath.Join(dir, "config", "contexts")
	require.NoError(t, os.MkdirAll(d, 0o750))
	for i := 0; i < maxHubScanEntries+5; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(d, fmt.Sprintf("f%05d", i)), nil, 0o600))
	}
	items, err := InstalledHubItems(dir)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestInstalledHubItemsUnreadableDirectories(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	dir := t.TempDir()
	link(t, dir, "config/collections/ok.yaml", "/hub/collections/o/ok.yaml")
	sub := filepath.Join(dir, "config", "parsers", "s01-parse")
	require.NoError(t, os.MkdirAll(sub, 0o750))
	require.NoError(t, os.Chmod(sub, 0o000))       //nolint:gosec // G302: test makes a directory unreadable
	t.Cleanup(func() { _ = os.Chmod(sub, 0o700) }) //nolint:gosec // G302: restore so TempDir cleanup works

	items, err := InstalledHubItems(dir)
	require.NoError(t, err, "an unreadable stage directory is skipped")
	require.Equal(t, []HubItem{{"collections", "o/ok"}}, items)

	root := filepath.Join(dir, "config", "scenarios")
	require.NoError(t, os.MkdirAll(root, 0o750))
	require.NoError(t, os.Chmod(root, 0o000))       //nolint:gosec // G302: test makes a directory unreadable
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) }) //nolint:gosec // G302: restore so TempDir cleanup works
	_, err = InstalledHubItems(dir)
	require.Error(t, err, "an unreadable type directory fails the scan")
}
