package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- readBackupSettingString / Bool / Int: direct unit tests of every
// fallback branch, not just the happy path exercised via GetSettings. ---

// TestReadBackupSettingString_NilDB proves the nil-db guard returns
// ("", false) rather than panicking — many unit tests across this package
// construct a bare BackupService{} with no database.
func TestReadBackupSettingString_NilDB(t *testing.T) {
	value, ok := readBackupSettingString(nil, SettingKeyBackupScheduleCron)
	assert.Empty(t, value)
	assert.False(t, ok)
}

// TestReadBackupSettingBool_UnparsableValue_FallsBackToDefault proves a
// corrupt/non-bool Setting.Value (e.g. hand-edited in the DB) falls back to
// the caller-supplied default instead of surfacing a parse error.
func TestReadBackupSettingBool_UnparsableValue_FallsBackToDefault(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{
		Key: SettingKeyBackupEncryptionEnabled, Value: "not-a-bool", Type: "bool", Category: backupSettingsCategory,
	}).Error)

	assert.True(t, readBackupSettingBool(db, SettingKeyBackupEncryptionEnabled, true))
	assert.False(t, readBackupSettingBool(db, SettingKeyBackupEncryptionEnabled, false))
}

// TestReadBackupSettingInt_UnparsableValue_FallsBackToDefault mirrors the
// bool case for the int reader.
func TestReadBackupSettingInt_UnparsableValue_FallsBackToDefault(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{
		Key: SettingKeyBackupRetentionCount, Value: "not-a-number", Type: "int", Category: backupSettingsCategory,
	}).Error)

	assert.Equal(t, 42, readBackupSettingInt(db, SettingKeyBackupRetentionCount, 42))
}

// --- upsertBackupSetting: direct unit tests of every error branch, forced
// via SQLite's "PRAGMA query_only = ON" so SELECT (the existence check)
// keeps working while INSERT/UPDATE fail — a faithful stand-in for a
// read-only database file/filesystem in production. ---

// newReadOnlyPragmaDB returns a single-connection in-memory DB with
// query_only enabled, so any subsequent write through it fails while reads
// still succeed. SetMaxOpenConns(1) is required so the same underlying
// SQLite connection (and therefore the same session-scoped PRAGMA) backs
// every query made through db, including inside db.Transaction closures.
func newReadOnlyPragmaDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)
	return db
}

// TestUpsertBackupSetting_QueryFails proves the "default" branch (the
// existence-check SELECT itself failing, distinct from ErrRecordNotFound)
// is wrapped and surfaced. Closing the connection entirely forces every
// query — including the initial SELECT — to fail.
func TestUpsertBackupSetting_QueryFails(t *testing.T) {
	db := newTestSettingsDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = upsertBackupSetting(db, "some.key", "v", "string")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query backup setting some.key")
}

// TestUpsertBackupSetting_CreateFails proves the tx.Create failure path
// (key not found, so an insert is attempted) is wrapped and surfaced.
func TestUpsertBackupSetting_CreateFails(t *testing.T) {
	db := newReadOnlyPragmaDB(t)

	err := upsertBackupSetting(db, "brand.new.key", "v", "string")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create backup setting brand.new.key")
}

// TestUpsertBackupSetting_SaveFails proves the tx.Save failure path (key
// already exists, so an update is attempted) is wrapped and surfaced.
func TestUpsertBackupSetting_SaveFails(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{
		Key: "existing.key", Value: "orig", Type: "string", Category: backupSettingsCategory,
	}).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)

	err = upsertBackupSetting(db, "existing.key", "new-value", "string")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update backup setting existing.key")
}

// --- migrateLegacyBackupSettings: the two upsertBackupSetting call sites
// (interval->schedule_cron, retention->retention_count) each propagate a
// failed upsert as their own wrapped/unwrapped return, and a failed
// tx.Delete of the now-migrated legacy row is wrapped separately. ---

// TestMigrateLegacyBackupSettings_IntervalUpsertFails_PropagatesError forces
// the ScheduleCron upsert (triggered by a legacy backup.interval row) to
// fail by making the destination key already exist under a read-only
// connection, so the Save path inside upsertBackupSetting fails and
// migrateLegacyBackupSettings's own error return (not just
// upsertBackupSetting's) is exercised.
func TestMigrateLegacyBackupSettings_IntervalUpsertFails_PropagatesError(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: legacySettingKeyBackupInterval, Value: "3", Type: "int", Category: "system"}).Error)
	// Pre-create the destination key so upsertBackupSetting takes the Save
	// (update) branch once query_only is enabled below.
	require.NoError(t, db.Create(&models.Setting{Key: SettingKeyBackupScheduleCron, Value: "0 3 * * *", Type: "string", Category: backupSettingsCategory}).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)

	err = migrateLegacyBackupSettings(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update backup setting")
}

// TestMigrateLegacyBackupSettings_RetentionUpsertFails_PropagatesError
// mirrors the above for the backup.retention -> retention_count call site.
func TestMigrateLegacyBackupSettings_RetentionUpsertFails_PropagatesError(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: legacySettingKeyBackupRetention, Value: "5", Type: "int", Category: "system"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: SettingKeyBackupRetentionCount, Value: "7", Type: "int", Category: backupSettingsCategory}).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)

	err = migrateLegacyBackupSettings(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update backup setting")
}

// TestMigrateLegacyBackupSettings_DeleteFails_WrapsError proves that once
// the translated setting is written successfully, a failure to delete the
// now-migrated legacy row is wrapped with the offending key rather than
// silently dropped (which would otherwise re-migrate the same row forever).
//
// This is forced with a BeforeDelete GORM hook registered on a throwaway
// model instance via db.Callback(), since query_only would also block the
// Create/Save that must succeed first in the same transaction.
func TestMigrateLegacyBackupSettings_DeleteFails_WrapsError(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: legacySettingKeyBackupRetention, Value: "5", Type: "int", Category: "system"}).Error)

	// assertAnError is the package's shared sentinel test error (see
	// backup_remote_service_test.go); injecting it via a GORM "before
	// delete" callback forces tx.Delete's own Error field to be non-nil
	// without disturbing the Create that must succeed earlier in the same
	// transaction.
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").
		Register("test:force_delete_failure", func(tx *gorm.DB) {
			_ = tx.AddError(assertAnError)
		}))

	err := migrateLegacyBackupSettings(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete legacy backup setting backup.retention")
}

// --- UpdateSettings: the s.db == nil guard, every per-field upsert error
// branch, the encryption-encrypt error branch, the outer transaction error
// return, and the reschedule error branch. ---

func TestUpdateSettings_NilDB_ReturnsError(t *testing.T) {
	svc := &BackupService{}
	err := svc.UpdateSettings(BackupSettingsUpdate{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup settings require a database connection")
}

// TestUpdateSettings_EachFieldUpsertFailure_PropagatesFromTransaction drives
// UpdateSettings once per updatable field with a read-only-pragma DB whose
// destination key already exists (so upsertBackupSetting takes the failing
// Save branch), proving each of the six per-field branches inside the
// transaction closure propagates its error out of UpdateSettings itself
// (not just out of upsertBackupSetting).
func TestUpdateSettings_EachFieldUpsertFailure_PropagatesFromTransaction(t *testing.T) {
	enc, err := crypto.NewEncryptionService(testEncryptionKeyBase64(t))
	require.NoError(t, err)

	trueVal := true
	cronVal := "*/5 * * * *"
	intVal := 2
	passphrase := "hunter2hunter2"

	seedKeys := map[string]string{
		SettingKeyBackupScheduleEnabled:         "true",
		SettingKeyBackupScheduleCron:            "0 3 * * *",
		SettingKeyBackupRetentionCount:          "7",
		SettingKeyBackupRemoteRetentionCount:    "7",
		SettingKeyBackupEncryptionEnabled:       "false",
		SettingKeyBackupEncryptionPassphraseEnc: "",
	}

	cases := []struct {
		name   string
		update BackupSettingsUpdate
	}{
		{"schedule_enabled", BackupSettingsUpdate{ScheduleEnabled: &trueVal}},
		{"schedule_cron", BackupSettingsUpdate{ScheduleCron: &cronVal}},
		{"retention_count", BackupSettingsUpdate{RetentionCount: &intVal}},
		{"remote_retention_count", BackupSettingsUpdate{RemoteRetentionCount: &intVal}},
		{"encryption_enabled", BackupSettingsUpdate{EncryptionEnabled: &trueVal}},
		{"encryption_passphrase", BackupSettingsUpdate{EncryptionPassphrase: &passphrase}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestSettingsDB(t)
			for key, value := range seedKeys {
				require.NoError(t, db.Create(&models.Setting{Key: key, Value: value, Type: "string", Category: backupSettingsCategory}).Error)
			}

			sqlDB, dbErr := db.DB()
			require.NoError(t, dbErr)
			sqlDB.SetMaxOpenConns(1)
			require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)

			svc := &BackupService{db: db, encryption: enc}
			err := svc.UpdateSettings(tc.update)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "update backup setting")
		})
	}
}

// TestUpdateSettings_TransactionError_Propagated proves UpdateSettings'
// outer `if err != nil { return err }` (immediately after
// s.db.Transaction(...)) actually surfaces the transaction's error to the
// caller, using a plain closed-connection failure on the very first write
// in the transaction (ScheduleEnabled).
func TestUpdateSettings_TransactionError_Propagated(t *testing.T) {
	db := newTestSettingsDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	svc := &BackupService{db: db}
	enabled := true
	err = svc.UpdateSettings(BackupSettingsUpdate{ScheduleEnabled: &enabled})
	require.Error(t, err)
}

// NOTE on two branches intentionally left uncovered here:
//
//   - UpdateSettings's encErr wrap (backup_settings.go ~262-264): the only
//     way crypto.EncryptionService.Encrypt returns an error is a source of
//     crypto/rand failing, which the package exposes no seam to fake (see
//     backup_encryption_test.go for that package's own direct Encrypt/Decrypt
//     error-path coverage via malformed ciphertext on the Decrypt side,
//     which *is* reachable). TestUpdateSettings_EncryptSucceeds_PersistsCiphertext
//     below instead locks down the surrounding happy-path lines.
//   - UpdateSettings's rescheduleErr wrap (backup_settings.go ~281-282):
//     Reschedule's cron spec was already validated by the identical
//     cron.ParseStandard call earlier in the same UpdateSettings invocation
//     (line ~211) and s.Cron is a concrete *cron.Cron (robfig/cron/v3) built
//     with cron.New()'s default standard 5-field parser — the same parser
//     ParseStandard uses — so a spec that reaches Reschedule has already
//     been proven to parse under the exact parser Reschedule/AddFunc will
//     use. There is no seam (Cron is a concrete struct, not an interface)
//     to fake an AddFunc failure without changing production code, and
//     doing so would violate this pass's test-only constraint.
func TestUpdateSettings_EncryptSucceeds_PersistsCiphertext(t *testing.T) {
	enc, err := crypto.NewEncryptionService(testEncryptionKeyBase64(t))
	require.NoError(t, err)
	svc := newSettingsTestService(t, enc)

	passphrase := "hunter2hunter2"
	require.NoError(t, svc.UpdateSettings(BackupSettingsUpdate{EncryptionPassphrase: &passphrase}))

	stored, ok := readBackupSettingString(svc.db, SettingKeyBackupEncryptionPassphraseEnc)
	require.True(t, ok)
	assert.NotEqual(t, passphrase, stored)
}
