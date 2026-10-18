package crowdsec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyWithItemDigest(t *testing.T) {
	t.Parallel()
	req := func(e *applyEnv, digest string) ApplyRequest {
		return ApplyRequest{Slug: applyTestSlug, Type: "collections", CacheKey: e.meta.CacheKey, ItemSHA256: digest}
	}

	t.Run("matching digest applies", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		res, err := e.svc.ApplyWith(context.Background(), req(e, strings.ToUpper(e.meta.ItemSHA256)))
		require.NoError(t, err)
		require.Equal(t, "applied", res.Status)
	})
	t.Run("absent digest applies", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		_, err := e.svc.ApplyWith(context.Background(), req(e, ""))
		require.NoError(t, err)
	})
	t.Run("differing digest is stale and runs nothing", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		_, err := e.svc.ApplyWith(context.Background(), req(e, strings.Repeat("0", 64)))
		require.ErrorIs(t, err, ErrPresetCacheStale)
		require.Empty(t, e.commands())
		e.requireRolledBack()
	})
}

func TestValidIndexPath(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"collections/crowdsecurity/base.yaml", "parsers/s01-parse/a/b.yaml", "", "a/b..c/d.yaml"} {
		require.True(t, validIndexPath(p), p)
	}
	for _, p := range []string{
		"../etc/passwd", "a/../../b", "a/..", "//evil.example/x", "https://evil.example/x", "javascript:alert(1)",
		"a\\b", "a/\x00b", "a/\nb", "a/\x7fb",
	} {
		require.False(t, validIndexPath(p), p)
	}
}

func TestParseRawIndexDropsUnsafePaths(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"collections":{
		"crowdsecurity/good":{"path":"collections/crowdsecurity/good.yaml","version":"1"},
		"crowdsecurity/dots":{"path":"../../x.yaml","version":"1"},
		"crowdsecurity/scheme":{"path":"https://evil.example/x.yaml","version":"1"},
		"crowdsecurity/slashes":{"path":"//evil.example/x.yaml","version":"1"}}}`)

	idx, err := parseRawIndex(raw, "https://hub.example/.index.json")
	require.NoError(t, err)
	require.Len(t, idx.Items, 1)
	require.Equal(t, "crowdsecurity/good", idx.Items[0].Name)
	require.Equal(t, "https://hub.example/collections/crowdsecurity/good.yaml", idx.Items[0].DownloadURL)
}

func TestParseRawIndexAllUnsafeIsAnEmptyIndex(t *testing.T) {
	t.Parallel()
	_, err := parseRawIndex([]byte(`{"collections":{"crowdsecurity/dots":{"path":"../x.yaml","version":"1"}}}`), "https://hub.example")
	require.Error(t, err)
}

func TestCopyFileCopiesARegularFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0o600))
	require.NoError(t, os.WriteFile(dst, []byte("old old old"), 0o600))

	require.NoError(t, copyFile(src, dst))
	got, err := os.ReadFile(dst) // #nosec G304 -- test temp dir
	require.NoError(t, err)
	require.Equal(t, "data", string(got))
}

func TestCopyFileRefusesNonRegularSources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("secret"), 0o600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))

	for name, src := range map[string]string{"symlink": link, "directory": dir, "missing": filepath.Join(dir, "none")} {
		for _, copyFn := range []func(string, string) error{copyFile, copyFileExclusive} {
			dst := filepath.Join(t.TempDir(), name)
			require.Error(t, copyFn(src, dst), name)
			require.NoFileExists(t, dst, name)
		}
	}
	require.ErrorIs(t, copyFile(dir, filepath.Join(t.TempDir(), "d")), errNotRegularFile)
}

func TestCopyFileExclusiveRefusesAnExistingDestination(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	require.NoError(t, os.WriteFile(src, []byte("new"), 0o600))

	existing := filepath.Join(dir, "existing")
	require.NoError(t, os.WriteFile(existing, []byte("old"), 0o600))
	require.Error(t, copyFileExclusive(src, existing))
	got, err := os.ReadFile(existing) // #nosec G304 -- test temp dir
	require.NoError(t, err)
	require.Equal(t, "old", string(got))

	outside := filepath.Join(dir, "outside")
	danglingLink := filepath.Join(dir, "dangling")
	require.NoError(t, os.Symlink(outside, danglingLink))
	require.Error(t, copyFileExclusive(src, danglingLink))
	require.NoFileExists(t, outside)

	fresh := filepath.Join(dir, "fresh")
	require.NoError(t, copyFileExclusive(src, fresh))
	require.FileExists(t, fresh)
}
