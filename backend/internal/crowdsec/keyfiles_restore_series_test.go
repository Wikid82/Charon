package crowdsec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// bulkFiles writes n small files below dir/config/bulk and returns nothing; the content is irrelevant.
func bulkFiles(t *testing.T, dir string, n int) {
	t.Helper()
	for i := range n {
		sub := filepath.Join(dir, "config", "bulk", fmt.Sprintf("d%03d", i/200))
		if i%200 == 0 {
			require.NoError(t, os.MkdirAll(sub, 0o750))
		}
		require.NoError(t, os.WriteFile(filepath.Join(sub, fmt.Sprintf("f%05d.txt", i)), []byte("x"), 0o600))
	}
}

// Files that exist in the new tree need no work, so they do not count toward the scan limit: a large
// configuration with one missing key file is restored.
func TestRestoreKeyFilesIgnoresPresentFilesForTheScanLimit(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/config.yaml": keyConfig, "config/ssl/missing.key": "opaque"},
		map[string]string{"config/config.yaml": keyConfig})
	bulkFiles(t, snapDir, 300)
	bulkFiles(t, liveDir, 300)

	// 300 present files exceed the scan bound of 100 but need no work
	restored, err := restoreKeyFiles(snapDir, liveDir, keyRestoreLimits{scan: 100, walk: 1000})

	require.NoError(t, err)
	require.Equal(t, []string{"config/ssl/missing.key"}, restored)
	require.Equal(t, "opaque", readFile(t, filepath.Join(liveDir, "config", "ssl", "missing.key")))
}

// A pathological tree still fails closed: the walk itself is bounded.
func TestRestoreKeyFilesFailsClosedOnAnEnormousTree(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/config.yaml": keyConfig},
		map[string]string{"config/config.yaml": keyConfig})
	bulkFiles(t, snapDir, 101)
	bulkFiles(t, liveDir, 101)

	_, err := restoreKeyFiles(snapDir, liveDir, keyRestoreLimits{scan: 100, walk: 100})

	require.ErrorIs(t, err, ErrKeyRestoreLimit)
}

// Files that do need work still count: too many absent candidates fail closed.
func TestRestoreKeyFilesStillCountsAbsentFilesTowardTheScanLimit(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/config.yaml": keyConfig},
		map[string]string{"config/config.yaml": keyConfig})
	bulkFiles(t, snapDir, 101)

	_, err := restoreKeyFiles(snapDir, liveDir, keyRestoreLimits{scan: 100, walk: 1000})

	require.ErrorIs(t, err, ErrKeyRestoreLimit)
}

// The import keeps the private keys the export left out, including one whose header straddles a window boundary.
func TestRestoreKeyFilesFindsArmorAcrossTheFirstWindowBoundary(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{
			"config/config.yaml":     keyConfig,
			"config/ssl/straddle.00": armorAt(sniffHead-10, sniffHead+4096),
			"config/ssl/late.00":     armorAt(300<<10, 600<<10),
			"config/ssl/plain.00":    "filler only\n",
		},
		map[string]string{"config/config.yaml": keyConfig})

	restored, err := RestoreKeyFiles(snapDir, liveDir)

	require.NoError(t, err)
	require.Equal(t, []string{"config/ssl/late.00", "config/ssl/straddle.00"}, restored)
}

// The production bounds are the documented ones.
func TestRestoreKeyFilesUsesTheDocumentedBounds(t *testing.T) {
	t.Parallel()
	require.Equal(t, 20000, maxKeyRestoreScan)
	require.Equal(t, 500_000, maxKeyRestoreWalk)
}

// A destination that cannot be inspected for a reason other than absence is an error, not a skip.
func TestKeyDestinationPresentReportsInspectionErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "here"), nil, 0o600))

	present, err := keyDestinationPresent(dir, "here")
	require.NoError(t, err)
	require.True(t, present)

	present, err = keyDestinationPresent(dir, "here/below")
	require.NoError(t, err)
	require.False(t, present, "a file in the parent chain counts as absent")

	_, err = keyDestinationPresent(dir, strings.Repeat("n", 300))
	require.Error(t, err)
}
