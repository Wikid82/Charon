package services

import (
	"errors"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSettingsTestService(t *testing.T, enc *crypto.EncryptionService) *BackupService {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	tmpDir := t.TempDir()
	cfg := &config.Config{DatabasePath: tmpDir + "/charon.db"}
	svc := NewBackupService(cfg, db, enc)
	t.Cleanup(svc.Stop)
	return svc
}

func TestBackupSettings_GetSettings_Defaults(t *testing.T) {
	svc := newSettingsTestService(t, nil)

	settings := svc.GetSettings()
	assert.True(t, settings.ScheduleEnabled)
	assert.Equal(t, defaultScheduleCron, settings.ScheduleCron)
	assert.Equal(t, DefaultBackupRetention, settings.RetentionCount)
	assert.Equal(t, defaultRemoteRetentionCount, settings.RemoteRetentionCount)
	assert.False(t, settings.EncryptionEnabled)
	assert.False(t, settings.EncryptionPassphraseSet)
}

func TestBackupSettings_UpdateSettings_PersistsAndReschedules(t *testing.T) {
	svc := newSettingsTestService(t, nil)

	newCron := "*/15 * * * *"
	retention := 3
	remoteRetention := 5
	require.NoError(t, svc.UpdateSettings(BackupSettingsUpdate{
		ScheduleCron:         &newCron,
		RetentionCount:       &retention,
		RemoteRetentionCount: &remoteRetention,
	}))

	settings := svc.GetSettings()
	assert.Equal(t, newCron, settings.ScheduleCron)
	assert.Equal(t, retention, settings.RetentionCount)
	assert.Equal(t, remoteRetention, settings.RemoteRetentionCount)
}

func TestBackupSettings_UpdateSettings_InvalidCronRejected(t *testing.T) {
	svc := newSettingsTestService(t, nil)

	bad := "not a cron expression"
	err := svc.UpdateSettings(BackupSettingsUpdate{ScheduleCron: &bad})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidCronSchedule))
}

func TestBackupSettings_UpdateSettings_InvalidRetentionRejected(t *testing.T) {
	svc := newSettingsTestService(t, nil)

	zero := 0
	err := svc.UpdateSettings(BackupSettingsUpdate{RetentionCount: &zero})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRetentionCount))

	negative := -1
	err = svc.UpdateSettings(BackupSettingsUpdate{RemoteRetentionCount: &negative})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRetentionCount))
}

// TestBackupSettings_EnableEncryption_WithoutKey_Returns503Equivalent proves
// R9: enabling scheduled encryption without CHARON_ENCRYPTION_KEY (and
// without setting a passphrase in the same request) fails with
// ErrEncryptionKeyMissingForOps, which the handler maps to 503
// encryption_key_missing.
func TestBackupSettings_EnableEncryption_WithoutKey_Returns503Equivalent(t *testing.T) {
	svc := newSettingsTestService(t, nil) // nil encryption == CHARON_ENCRYPTION_KEY unset

	enabled := true
	err := svc.UpdateSettings(BackupSettingsUpdate{EncryptionEnabled: &enabled})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEncryptionKeyMissing))
}

func TestBackupSettings_SetPassphrase_WithoutKey_Rejected(t *testing.T) {
	svc := newSettingsTestService(t, nil)

	passphrase := "hunter2"
	err := svc.UpdateSettings(BackupSettingsUpdate{EncryptionPassphrase: &passphrase})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEncryptionKeyMissing))
}

func TestBackupSettings_SetPassphrase_WithKey_Succeeds(t *testing.T) {
	enc, err := crypto.NewEncryptionService(testEncryptionKeyBase64(t))
	require.NoError(t, err)

	svc := newSettingsTestService(t, enc)

	passphrase := "hunter2hunter2"
	require.NoError(t, svc.UpdateSettings(BackupSettingsUpdate{EncryptionPassphrase: &passphrase}))

	settings := svc.GetSettings()
	assert.True(t, settings.EncryptionPassphraseSet)

	// The stored value must be ciphertext, never the plaintext passphrase.
	stored, ok := readBackupSettingString(svc.db, SettingKeyBackupEncryptionPassphraseEnc)
	require.True(t, ok)
	assert.NotEqual(t, passphrase, stored)
}

func TestBackupSettings_ClearPassphrase_EmptyString(t *testing.T) {
	enc, err := crypto.NewEncryptionService(testEncryptionKeyBase64(t))
	require.NoError(t, err)
	svc := newSettingsTestService(t, enc)

	passphrase := "hunter2hunter2"
	require.NoError(t, svc.UpdateSettings(BackupSettingsUpdate{EncryptionPassphrase: &passphrase}))
	assert.True(t, svc.GetSettings().EncryptionPassphraseSet)

	empty := ""
	require.NoError(t, svc.UpdateSettings(BackupSettingsUpdate{EncryptionPassphrase: &empty}))
	assert.False(t, svc.GetSettings().EncryptionPassphraseSet)
}

func TestBackupSettings_DisableSchedule_RemovesCronEntry(t *testing.T) {
	svc := newSettingsTestService(t, nil)

	disabled := false
	require.NoError(t, svc.UpdateSettings(BackupSettingsUpdate{ScheduleEnabled: &disabled}))
	assert.False(t, svc.GetSettings().ScheduleEnabled)
	assert.Equal(t, cron.EntryID(0), svc.scheduleEntry)
}

// testEncryptionKeyBase64 returns a syntactically valid base64-encoded
// 32-byte key for crypto.NewEncryptionService.
func testEncryptionKeyBase64(t *testing.T) string {
	t.Helper()
	return "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=" // base64("01234567890123456789012345678901")
}
