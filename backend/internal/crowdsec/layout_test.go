package crowdsec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectLayout(t *testing.T) {
	t.Parallel()
	write := func(t *testing.T, p string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte("x: 1\n"), 0o600))
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  Layout
	}{
		{"nothing at all", func(*testing.T, string) {}, LayoutFlat},
		{"config yaml in config folder", func(t *testing.T, d string) { write(t, filepath.Join(d, "config", "config.yaml")) }, LayoutNested},
		{"config folder with only hub", func(t *testing.T, d string) {
			require.NoError(t, os.MkdirAll(filepath.Join(d, "config", "hub"), 0o750))
		}, LayoutNested},
		{"config folder with only credentials", func(t *testing.T, d string) { write(t, filepath.Join(d, "config", "local_api_credentials.yaml")) }, LayoutNested},
		{"empty config folder", func(t *testing.T, d string) { require.NoError(t, os.MkdirAll(filepath.Join(d, "config"), 0o750)) }, LayoutFlat},
		{"config is a regular file", func(t *testing.T, d string) { write(t, filepath.Join(d, "config")) }, LayoutFlat},
		{"top level config yaml only", func(t *testing.T, d string) { write(t, filepath.Join(d, "config.yaml")) }, LayoutFlat},
		{"both layouts", func(t *testing.T, d string) {
			write(t, filepath.Join(d, "config.yaml"))
			write(t, filepath.Join(d, "config", "config.yaml"))
		}, LayoutNested},
		{"hybrid tree of a fresh container", func(t *testing.T, d string) {
			write(t, filepath.Join(d, "config.yaml"))
			write(t, filepath.Join(d, "acquis.yaml"))
			write(t, filepath.Join(d, "config", "local_api_credentials.yaml"))
		}, LayoutNested},
		{"link to a populated folder", func(t *testing.T, d string) {
			target := filepath.Join(t.TempDir(), "real")
			write(t, filepath.Join(target, "config.yaml"))
			require.NoError(t, os.Symlink(target, filepath.Join(d, "config")))
		}, LayoutNested},
		{"link to an empty folder", func(t *testing.T, d string) {
			target := filepath.Join(t.TempDir(), "real")
			require.NoError(t, os.MkdirAll(target, 0o750))
			require.NoError(t, os.Symlink(target, filepath.Join(d, "config")))
		}, LayoutFlat},
		{"dangling link", func(t *testing.T, d string) {
			require.NoError(t, os.Symlink(filepath.Join(d, "missing"), filepath.Join(d, "config")))
		}, LayoutNested},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			tc.setup(t, dir)
			require.Equal(t, tc.want, DetectLayout(dir))
		})
	}
}

func TestDetectLayoutUnreadableConfigFolderIsNested(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not restrict root")
	}
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "hub"), 0o750))
	require.NoError(t, os.Chmod(configDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(configDir, 0o750) }) //nolint:gosec // G302: a directory needs the execute bit so the test tree can be removed
	require.Equal(t, LayoutNested, DetectLayout(dir))
}

func TestCheckImportLayout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		live    Layout
		archive ArchiveInfo
		refused bool
	}{
		{"nested live, nested archive", LayoutNested, ArchiveInfo{HasNestedConfig: true}, false},
		{"nested live, flat archive", LayoutNested, ArchiveInfo{}, true},
		{"flat live, nested archive", LayoutFlat, ArchiveInfo{HasNestedConfig: true}, false},
		{"flat live, neither", LayoutFlat, ArchiveInfo{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := CheckImportLayout(tc.live, tc.archive)
			if tc.refused {
				require.ErrorIs(t, err, ErrArchiveLayout)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestInspectArchiveLayoutFlags(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("#", int(maxInspectEntryBytes)+1)
	for _, tc := range []struct {
		name   string
		files  map[string]string
		nested bool
	}{
		{"nested", map[string]string{"config/config.yaml": plainConfig}, true},
		{"flat", map[string]string{"config.yaml": plainConfig}, false},
		{"both", map[string]string{"config.yaml": plainConfig, "config/config.yaml": plainConfig}, true},
		{"normalised spellings", map[string]string{"./config//config.yaml": plainConfig, "x/../config.yaml": plainConfig}, true},
		{"deeper copies do not count", map[string]string{"parsers/config.yaml": plainConfig, "config/sub/config.yaml": plainConfig}, false},
		{"oversized entries still set the flags", map[string]string{"config/config.yaml": huge, "config.yaml": huge}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			info, err := InspectArchive(writeArchive(t, tc.files))
			require.NoError(t, err)
			require.Equal(t, tc.nested, info.HasNestedConfig)
		})
	}
}

func TestDetectLayoutUninspectableTreeIsNested(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not restrict root")
	}
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o750))
	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) }) //nolint:gosec // G302: a directory needs the execute bit so the test tree can be removed
	require.Equal(t, LayoutNested, DetectLayout(dir))
}

func TestDetectLayoutBelowAFileIsFlat(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	require.Equal(t, LayoutFlat, DetectLayout(filepath.Join(blocker, "crowdsec")))
}
