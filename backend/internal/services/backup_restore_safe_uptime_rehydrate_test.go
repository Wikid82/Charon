package services

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingUptimeRehydrator is a test double for the UptimeRehydrator hook that
// RestoreBackupSafe invokes after a successful live database rehydrate.
type recordingUptimeRehydrator struct {
	mu    sync.Mutex
	calls int
}

func (r *recordingUptimeRehydrator) Rehydrate(_ context.Context) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
}

func (r *recordingUptimeRehydrator) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// TestRestoreBackupSafe_InvokesUptimeRehydratorOnLiveRehydrate proves the R1b
// hook fires exactly once when a wired rehydrator is present and the live DB was
// actually swapped in (rehydrated == true).
func TestRestoreBackupSafe_InvokesUptimeRehydratorOnLiveRehydrate(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)

	rh := &recordingUptimeRehydrator{}
	svc.SetUptimeRehydrator(rh)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.LiveRehydrateApplied, "precondition: the live DB rehydrated in-process")
	assert.Equal(t, 1, rh.count(), "the uptime scheduler must be rehydrated once after a live restore")
}
