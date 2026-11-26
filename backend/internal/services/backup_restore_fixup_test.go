package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// recordingFixup returns a restore fix-up that records its name in order and returns err.
func recordingFixup(order *orderRecorder, name string, err error) func(context.Context, *gorm.DB) error {
	return func(context.Context, *gorm.DB) error {
		order.add(name)
		return err
	}
}

// TestRestoreBackupSafe_FixupsRunAfterTheKeysAndBeforeCaddy proves the fix-ups run in registration order between
// the encryption rehydrate and the Caddy reload.
func TestRestoreBackupSafe_FixupsRunAfterTheKeysAndBeforeCaddy(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)
	order := &orderRecorder{}
	svc.SetEncryptionRehydrator(&recordingEncryptionRehydrator{order: order})
	svc.SetCaddyReloader(&recordingCaddyReloader{order: order})
	svc.AddRestoreFixup("first", recordingFixup(order, "fixup-first", nil))
	svc.AddRestoreFixup("second", recordingFixup(order, "fixup-second", nil))

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.True(t, result.LiveRehydrateApplied, "precondition: the live DB rehydrated in-process")
	assert.Equal(t, []string{"encryption", "fixup-first", "fixup-second", "caddy"}, order.snapshot())
	assert.Empty(t, result.Warnings)
}

// TestRestoreBackupSafe_FixupFailureIsAWarningAndTheRestoreStands proves a failing fix-up is reported, the
// remaining fix-ups still run and the restore is not failed.
func TestRestoreBackupSafe_FixupFailureIsAWarningAndTheRestoreStands(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)
	order := &orderRecorder{}
	svc.AddRestoreFixup("broken", recordingFixup(order, "fixup-broken", errors.New("fix-up down")))
	svc.AddRestoreFixup("later", recordingFixup(order, "fixup-later", nil))

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	assert.True(t, result.LiveRehydrateApplied)
	assert.Equal(t, []string{"fixup-broken", "fixup-later"}, order.snapshot())
	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "a data fix-up could not run after restore")
}

// TestRestoreBackupSafe_ClearsStoredApplyTextFromAnOlderBackup proves a backup taken before the one-time clearing
// has its stored apply messages cleared when it is restored live.
func TestRestoreBackupSafe_ClearsStoredApplyTextFromAnOlderBackup(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)
	require.NoError(t, svc.db.AutoMigrate(&models.CaddyConfig{}, &models.Setting{}))
	require.NoError(t, svc.db.Create(&models.CaddyConfig{AppliedAt: time.Now(), ErrorMsg: "stored refusal text"}).Error)
	svc.AddRestoreFixup("caddy_apply_error_text", func(ctx context.Context, db *gorm.DB) error {
		_, err := ClearStoredApplyErrorText(ctx, db)
		return err
	})

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)
	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.True(t, result.LiveRehydrateApplied, "precondition: the live DB rehydrated in-process")

	var rows []models.CaddyConfig
	require.NoError(t, svc.db.Find(&rows).Error)
	require.NotEmpty(t, rows)
	for _, row := range rows {
		assert.Empty(t, row.ErrorMsg)
	}
	assert.Empty(t, result.Warnings)
}
