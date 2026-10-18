package crowdsec

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPruneBackupsProtectingKeepsProtectedSnapshots(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	base := time.Now()
	var paths []string
	for i := range 4 {
		p := dataDir + "." + BackupKindSnapshot + "." + base.Add(time.Duration(i)*time.Second).Format(backupTimeFormat)
		require.NoError(t, os.MkdirAll(p, 0o750))
		paths = append(paths, p)
	}
	removed, err := PruneBackupsProtecting(dataDir, BackupKindSnapshot, 1, []string{paths[0]})
	require.NoError(t, err)
	require.ElementsMatch(t, paths[1:3], removed)
	require.DirExists(t, paths[0], "the protected, oldest snapshot survives")
	require.DirExists(t, paths[3], "the newest snapshot survives")
}
