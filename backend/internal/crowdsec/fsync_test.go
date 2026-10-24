package crowdsec

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeDirSyncer stands in for an open directory whose sync and close return the configured errors.
type fakeDirSyncer struct {
	syncErr  error
	closeErr error
}

func (f fakeDirSyncer) Sync() error  { return f.syncErr }
func (f fakeDirSyncer) Close() error { return f.closeErr }

func TestSyncDirSucceedsOnADirectory(t *testing.T) {
	t.Parallel()
	require.NoError(t, SyncDir(t.TempDir()))
}

func TestSyncDirFailsOnAMissingDirectory(t *testing.T) {
	t.Parallel()
	err := SyncDir(filepath.Join(t.TempDir(), "missing"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestSyncOpenDirTreatsUnsupportedSyncAsSuccess(t *testing.T) {
	t.Parallel()
	for _, err := range []error{syscall.EINVAL, syscall.ENOTSUP, errors.ErrUnsupported} {
		require.NoError(t, syncOpenDir(fakeDirSyncer{syncErr: err}), err.Error())
	}
}

func TestSyncOpenDirReportsOtherErrors(t *testing.T) {
	t.Parallel()
	err := syncOpenDir(fakeDirSyncer{syncErr: syscall.EIO})
	require.ErrorIs(t, err, syscall.EIO)
	require.ErrorContains(t, err, "sync directory")

	err = syncOpenDir(fakeDirSyncer{closeErr: syscall.EIO})
	require.ErrorIs(t, err, syscall.EIO)
}

func TestSyncOpenDirSucceedsWhenSyncAndCloseSucceed(t *testing.T) {
	t.Parallel()
	require.NoError(t, syncOpenDir(fakeDirSyncer{}))
}
