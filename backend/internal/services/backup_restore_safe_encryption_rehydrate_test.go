package services

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orderRecorder collects the names of post-restore hooks in call order.
type orderRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *orderRecorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *orderRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

// recordingEncryptionRehydrator is an EncryptionRehydrator test double.
type recordingEncryptionRehydrator struct {
	order *orderRecorder
	err   error
	calls int
}

func (r *recordingEncryptionRehydrator) Rehydrate(_ context.Context) error {
	r.calls++
	r.order.add("encryption")
	return r.err
}

// recordingCaddyReloader is a CaddyReloader test double.
type recordingCaddyReloader struct {
	order *orderRecorder
}

func (r *recordingCaddyReloader) ApplyConfig(_ context.Context) error {
	r.order.add("caddy")
	return nil
}

// TestRestoreBackupSafe_EncryptionRehydrateRunsBeforeCaddyReload proves the key
// registry is reloaded after a live rehydrate and before Caddy regenerates its
// config, because that config decrypts through the key version map.
func TestRestoreBackupSafe_EncryptionRehydrateRunsBeforeCaddyReload(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)
	order := &orderRecorder{}
	keys := &recordingEncryptionRehydrator{order: order}
	svc.SetEncryptionRehydrator(keys)
	svc.SetCaddyReloader(&recordingCaddyReloader{order: order})

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.True(t, result.LiveRehydrateApplied, "precondition: the live DB rehydrated in-process")
	assert.Equal(t, 1, keys.calls)
	assert.Equal(t, []string{"encryption", "caddy"}, order.snapshot())
	assert.Empty(t, result.Warnings)
}

// TestRestoreBackupSafe_EncryptionRehydrateFailureIsReportedNotFatal proves a key
// registry failure is reported as a warning and the restore still stands.
func TestRestoreBackupSafe_EncryptionRehydrateFailureIsReportedNotFatal(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)
	order := &orderRecorder{}
	svc.SetEncryptionRehydrator(&recordingEncryptionRehydrator{order: order, err: errors.New("registry unavailable")})

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	assert.True(t, result.LiveRehydrateApplied)
	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "encryption keys could not be re-registered")
}
