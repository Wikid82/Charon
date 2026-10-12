package handlers

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

type archiveMember struct {
	name     string
	typeflag byte
	linkname string
	body     string
}

func writeMemberArchive(t *testing.T, members []archiveMember) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "members.tar.gz")
	f, err := os.Create(p) // #nosec G304 -- test temp path
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, m := range members {
		hdr := &tar.Header{Name: m.name, Typeflag: m.typeflag, Linkname: m.linkname, Mode: 0o640}
		if m.typeflag == tar.TypeReg {
			hdr.Size = int64(len(m.body))
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if m.typeflag == tar.TypeReg {
			_, err = tw.Write([]byte(m.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
	return p
}

// TestExtractArchiveLeavesEngineOwnedPathsAlone feeds names that address the hub cache, in case
// variants and nested forms, plus link members, and requires the cache files to stay byte-identical.
func TestExtractArchiveLeavesEngineOwnedPathsAlone(t *testing.T) {
	t.Parallel()
	const entry = "hub_cache/crowdsecurity/example"
	names := []string{
		entry + "/metadata.json",
		"HUB_CACHE/crowdsecurity/example/metadata.json",
		"Hub_Cache/crowdsecurity/example/metadata.json",
		"./" + entry + "/metadata.json",
		"a/../" + entry + "/item.yaml",
		"/" + entry + "/item.yaml",
		entry + "/../example/item.yaml",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			dir := filepath.Join(parent, "data")
			files := map[string]string{
				filepath.Join(dir, entry, "metadata.json"): `{"slug":"crowdsecurity/example"}`,
				filepath.Join(dir, entry, "item.yaml"):     "name: example\n",
			}
			for p, content := range files {
				require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
				require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
			}
			h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dir)
			arc := writeMemberArchive(t, []archiveMember{{name: name, typeflag: tar.TypeReg, body: "replaced"}})

			_ = h.extractArchive(arc, dir, crowdsec.ImportKeeper(dir)) // refusing is as good as skipping
			for p, content := range files {
				require.Equal(t, content, readTreeFile(t, p))
			}
			entries, err := os.ReadDir(parent)
			require.NoError(t, err)
			require.Len(t, entries, 1, "nothing is written outside the data dir")
		})
	}
}

func TestExtractArchiveIgnoresLinkMembers(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	dir := filepath.Join(parent, "data")
	cached := filepath.Join(dir, "hub_cache", "crowdsecurity", "example", "metadata.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(cached), 0o750))
	require.NoError(t, os.WriteFile(cached, []byte(`{"slug":"crowdsecurity/example"}`), 0o600))
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dir)

	arc := writeMemberArchive(t, []archiveMember{
		{name: "linked.yaml", typeflag: tar.TypeLink, linkname: "hub_cache/crowdsecurity/example/metadata.json"},
		{name: "hub_cache/crowdsecurity/example/other.yaml", typeflag: tar.TypeLink, linkname: "config.yaml"},
		{name: "symbolic.yaml", typeflag: tar.TypeSymlink, linkname: "/nonexistent/other.txt"},
	})
	_ = h.extractArchive(arc, dir, crowdsec.ImportKeeper(dir))

	require.Equal(t, `{"slug":"crowdsecurity/example"}`, readTreeFile(t, cached))
	require.NoFileExists(t, filepath.Join(dir, "linked.yaml"))
	require.NoFileExists(t, filepath.Join(dir, "symbolic.yaml"))
	require.NoFileExists(t, filepath.Join(dir, "hub_cache", "crowdsecurity", "example", "other.yaml"))
}
