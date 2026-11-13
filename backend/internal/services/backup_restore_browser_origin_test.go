package services

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingOriginRefresher is a test double for the BrowserOriginRefresher hook that RestoreBackupSafe invokes after a
// successful live database rehydrate.
type recordingOriginRefresher struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (r *recordingOriginRefresher) Refresh() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.err
}

func (r *recordingOriginRefresher) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// TestRestoreBackupSafe_RefreshesBrowserOriginOnLiveRehydrate proves the cross-origin check is rebuilt once after a live
// restore, so the public address restored from the backup is trusted without a restart.
func TestRestoreBackupSafe_RefreshesBrowserOriginOnLiveRehydrate(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)

	refresher := &recordingOriginRefresher{}
	svc.SetBrowserOriginRefresher(refresher)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.LiveRehydrateApplied, "precondition: the live DB rehydrated in-process")
	assert.Equal(t, 1, refresher.count(), "the origin check must be refreshed once after a live restore")
}

// TestRestoreBackupSafe_BrowserOriginRefreshErrorDoesNotFailRestore proves a refresh failure is logged and does not
// turn a restored database into a failed restore; the guard already fails closed for the value it rejected.
func TestRestoreBackupSafe_BrowserOriginRefreshErrorDoesNotFailRestore(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)

	refresher := &recordingOriginRefresher{err: assert.AnError}
	svc.SetBrowserOriginRefresher(refresher)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.True(t, result.LiveRehydrateApplied)
	assert.Equal(t, 1, refresher.count())
}

// TestRestoreBackupSafe_NoBrowserOriginRefresherIsSafe proves a service without a wired guard still restores.
func TestRestoreBackupSafe_NoBrowserOriginRefresherIsSafe(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.True(t, result.LiveRehydrateApplied)
}
