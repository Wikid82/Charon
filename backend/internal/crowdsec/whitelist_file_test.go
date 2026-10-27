package crowdsec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsManagedPath(t *testing.T) {
	t.Parallel()
	managed := []string{
		WhitelistRelPath,
		"./" + WhitelistRelPath,
		"config/parsers/s02-enrich/../s02-enrich/charon-whitelist.yaml",
	}
	for _, rel := range managed {
		require.True(t, IsManagedPath(rel), rel)
	}

	notManaged := []string{
		"",
		".",
		"/" + WhitelistRelPath,
		"charon-whitelist.yaml",
		"config/parsers/s02-enrich/other.yaml",
		"config/parsers/s02-enrich/charon-whitelist.yaml.tmp",
		"config/parsers/s02-enrich/CHARON-WHITELIST.yaml",
	}
	for _, rel := range notManaged {
		require.False(t, IsManagedPath(rel), rel)
	}
}

func writeScanFile(t *testing.T, dataDir, rel, content string) {
	t.Helper()
	p := filepath.Join(dataDir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
}

func TestFindOtherWhitelistFiles_ListsOnlyOtherParsersWithAWhitelistSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeScanFile(t, dir, WhitelistRelPath, "name: charon-whitelist\nwhitelist:\n  reason: managed\n")
	writeScanFile(t, dir, "config/parsers/s02-enrich/extra.yaml", "name: extra\nwhitelist:\n  reason: x\n")
	writeScanFile(t, dir, "config/parsers/s02-enrich/extra.yml", "whitelist:\n  reason: y\n")
	writeScanFile(t, dir, "config/parsers/s02-enrich/plain.yaml", "name: plain\nfilter: x\n")
	writeScanFile(t, dir, "config/parsers/s02-enrich/broken.yaml", "whitelist: [\n")
	writeScanFile(t, dir, "config/parsers/s02-enrich/notes.txt", "whitelist:\n  reason: z\n")
	writeScanFile(t, dir, "config/parsers/s01-parse/nested.yaml", "whitelist:\n  reason: w\n")
	writeScanFile(t, dir, "config/config.yaml", "whitelist:\n  reason: outside the parsers\n")

	require.Equal(t, []string{
		"config/parsers/s01-parse/nested.yaml",
		"config/parsers/s02-enrich/extra.yaml",
		"config/parsers/s02-enrich/extra.yml",
	}, FindOtherWhitelistFiles(dir))
}

func TestFindOtherWhitelistFiles_MissingParserDirIsEmpty(t *testing.T) {
	t.Parallel()
	require.Empty(t, FindOtherWhitelistFiles(t.TempDir()))
}

func TestFindOtherWhitelistFiles_SkipsSymlinksAndOversizedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("whitelist:\n  reason: outside\n"), 0o600))
	const s02 = "config/parsers/s02-enrich"
	writeScanFile(t, dir, s02+"/big.yaml", "whitelist:\n  reason: "+strings.Repeat("a", maxWhitelistScanBytes)+"\n")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, s02), 0o750))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, s02, "link.yaml")))

	require.Empty(t, FindOtherWhitelistFiles(dir))
}

func TestFindOtherWhitelistFiles_StopsAfterTheFileLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 0; i <= maxWhitelistScanFiles; i++ {
		writeScanFile(t, dir, fmt.Sprintf("config/parsers/s02-enrich/f%03d.yaml", i), "whitelist:\n  reason: x\n")
	}

	found := FindOtherWhitelistFiles(dir)
	require.Len(t, found, maxWhitelistScanFiles)
	require.NotContains(t, found, fmt.Sprintf("config/parsers/s02-enrich/f%03d.yaml", maxWhitelistScanFiles))
}
