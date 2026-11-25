package services

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingRemoteTargetRehydrator records calls made by RestoreBackupSafe after a live rehydrate.
type recordingRemoteTargetRehydrator struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (r *recordingRemoteTargetRehydrator) Rehydrate(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.err
}

func (r *recordingRemoteTargetRehydrator) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// TestRestoreBackupSafe_InvokesRemoteTargetRehydratorOnLiveRehydrate proves the remote target
// backfill runs once after a live restore that swapped the database in.
func TestRestoreBackupSafe_InvokesRemoteTargetRehydratorOnLiveRehydrate(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)
	rh := &recordingRemoteTargetRehydrator{}
	svc.SetRemoteTargetRehydrator(rh)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.True(t, result.LiveRehydrateApplied, "precondition: the live DB rehydrated in-process")
	assert.Equal(t, 1, rh.count())
}

// TestRestoreBackupSafe_RemoteTargetRehydratorFailureKeepsRestore proves a failing backfill is
// reported as a warning and does not fail a restore that already succeeded.
func TestRestoreBackupSafe_RemoteTargetRehydratorFailureKeepsRestore(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)
	rh := &recordingRemoteTargetRehydrator{err: errors.New("boom")}
	svc.SetRemoteTargetRehydrator(rh)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.True(t, result.LiveRehydrateApplied)
	assert.Equal(t, 1, rh.count())
	assert.NotEmpty(t, result.Warnings)
}
