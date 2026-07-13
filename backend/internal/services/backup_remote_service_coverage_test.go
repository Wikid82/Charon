package services

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services/remotestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRemoteServiceCoverageEncryption builds a real (non-nil)
// crypto.EncryptionService so Create/Update/decryptSecrets can be exercised
// against genuine encrypt/decrypt round trips rather than only the
// ErrEncryptionKeyMissing short-circuit.
func newRemoteServiceCoverageEncryption(t *testing.T) *crypto.EncryptionService {
	t.Helper()
	enc, err := crypto.NewEncryptionService(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)
	return enc
}

// --- validateRemoteTargetConfig: direct unit tests of every branch ---

func TestValidateRemoteTargetConfig_S3_MissingEndpoint(t *testing.T) {
	err := validateRemoteTargetConfig("s3", RemoteTargetConfig{Bucket: "b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "s3 endpoint is required")
}

func TestValidateRemoteTargetConfig_S3_MissingBucket(t *testing.T) {
	err := validateRemoteTargetConfig("s3", RemoteTargetConfig{Endpoint: "s3.example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "s3 bucket is required")
}

func TestValidateRemoteTargetConfig_S3_SSRFRejected(t *testing.T) {
	err := validateRemoteTargetConfig("s3", RemoteTargetConfig{Endpoint: "169.254.169.254:80", Bucket: "b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSRF validation")
}

func TestValidateRemoteTargetConfig_S3_ValidEndpointWithPort(t *testing.T) {
	// 203.0.113.0/24 is RFC 5737 TEST-NET-3, safe/deterministic for SSRF checks.
	err := validateRemoteTargetConfig("s3", RemoteTargetConfig{Endpoint: "203.0.113.10:9000", Bucket: "b"})
	require.NoError(t, err)
}

func TestValidateRemoteTargetConfig_SFTP_MissingHost(t *testing.T) {
	err := validateRemoteTargetConfig("sftp", RemoteTargetConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sftp host is required")
}

func TestValidateRemoteTargetConfig_SFTP_SSRFRejected(t *testing.T) {
	err := validateRemoteTargetConfig("sftp", RemoteTargetConfig{Host: "169.254.169.254"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSRF validation")
}

func TestValidateRemoteTargetConfig_UnknownType(t *testing.T) {
	err := validateRemoteTargetConfig("ftp", RemoteTargetConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown remote storage target type "ftp"`)
}

// --- RemoteTargetSecrets.isEmpty ---

func TestRemoteTargetSecrets_IsEmpty(t *testing.T) {
	assert.True(t, RemoteTargetSecrets{}.isEmpty())
	assert.False(t, RemoteTargetSecrets{Password: "x"}.isEmpty())
}

// --- reconcileStuckUploadingCopies: nil db no-op ---

func TestReconcileStuckUploadingCopies_NilDB_NoOp(t *testing.T) {
	require.NoError(t, reconcileStuckUploadingCopies(nil))
}

// --- Create ---

func TestBackupRemoteService_Create_EncryptionKeyMissing(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	target, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "x"})
	require.ErrorIs(t, err, ErrEncryptionKeyMissing)
	assert.Nil(t, target)
}

func TestBackupRemoteService_Create_InvalidConfig(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	target, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{}, RemoteTargetSecrets{Password: "x"})
	require.Error(t, err)
	assert.Nil(t, target)
	assert.Contains(t, err.Error(), "sftp host is required")
}

func TestBackupRemoteService_Create_Success(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	target, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "hunter2"})
	require.NoError(t, err)
	require.NotNil(t, target)
	assert.Equal(t, "NAS", target.Name)
	assert.Equal(t, "never", target.LastTestStatus)
	assert.NotEmpty(t, target.SecretsEncrypted)

	// Round trip: decryptSecrets against the freshly created row.
	secrets, err := svc.decryptSecrets(target)
	require.NoError(t, err)
	assert.Equal(t, "hunter2", secrets.Password)
}

// TestBackupRemoteService_Create_EnabledFalse_PersistsAsDisabled is a
// regression test for a bug where models.RemoteStorageTarget.Enabled carries
// a GORM `default:true` tag: a plain db.Create sees the Go zero value
// (false) for that column and silently omits it from the INSERT, so the row
// falls back to the DB-level default (true) even though the caller
// explicitly asked for enabled: false. The row is re-queried by UUID
// (rather than trusting the in-memory struct Create may have written back
// into) to prove what actually landed in the database.
func TestBackupRemoteService_Create_EnabledFalse_PersistsAsDisabled(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	target, err := svc.Create("NAS", "sftp", false, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "x"})
	require.NoError(t, err)
	require.NotNil(t, target)

	// The in-memory struct returned by Create.
	assert.False(t, target.Enabled, "in-memory struct after Create should reflect enabled:false")

	// The persisted row, re-queried independently from the DB.
	var persisted models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", target.UUID).First(&persisted).Error)
	assert.False(t, persisted.Enabled, "persisted row must have enabled=false, not the DB default")
}

// TestBackupRemoteService_Create_EnabledTrue_PersistsAsEnabled is the
// companion case for the above: enabled:true must still persist correctly.
func TestBackupRemoteService_Create_EnabledTrue_PersistsAsEnabled(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	target, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "x"})
	require.NoError(t, err)
	require.NotNil(t, target)
	assert.True(t, target.Enabled)

	var persisted models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", target.UUID).First(&persisted).Error)
	assert.True(t, persisted.Enabled)
}

// --- Update ---

func TestBackupRemoteService_Update_NotFound(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	name := "New Name"
	target, err := svc.Update("does-not-exist", &name, nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, target)
}

func TestBackupRemoteService_Update_InvalidConfig(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	created, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "x"})
	require.NoError(t, err)

	badConfig := RemoteTargetConfig{} // sftp host now missing
	target, err := svc.Update(created.UUID, nil, nil, &badConfig, nil)
	require.Error(t, err)
	assert.Nil(t, target)
	assert.Contains(t, err.Error(), "sftp host is required")
}

func TestBackupRemoteService_Update_SecretsWithoutEncryption(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	created, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "x"})
	require.NoError(t, err)

	// A service with no encryption configured cannot update secrets, even
	// on an already-encrypted target.
	svcNoEnc := NewBackupRemoteService(db, nil, t.TempDir())
	newSecrets := RemoteTargetSecrets{Password: "y"}
	target, err := svcNoEnc.Update(created.UUID, nil, nil, nil, &newSecrets)
	require.ErrorIs(t, err, ErrEncryptionKeyMissing)
	assert.Nil(t, target)
}

func TestBackupRemoteService_Update_Success(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	created, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "x"})
	require.NoError(t, err)

	newName := "Renamed NAS"
	disabled := false
	newConfig := RemoteTargetConfig{Host: "203.0.113.21"}
	newSecrets := RemoteTargetSecrets{Password: "new-password"}

	updated, err := svc.Update(created.UUID, &newName, &disabled, &newConfig, &newSecrets)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Renamed NAS", updated.Name)
	assert.False(t, updated.Enabled)
	assert.Contains(t, updated.ConfigJSON, "203.0.113.21")

	secrets, err := svc.decryptSecrets(updated)
	require.NoError(t, err)
	assert.Equal(t, "new-password", secrets.Password)
}

// TestBackupRemoteService_Update_EnabledFalse_PersistsAsDisabled checks
// Update for the same zero-value/GORM-default bug class as Create: Update
// goes through db.Save, which (unlike a plain db.Create) always writes every
// field regardless of the `default:true` tag, so this is expected to pass
// even before the Create fix — re-querying the row from the DB proves it.
func TestBackupRemoteService_Update_EnabledFalse_PersistsAsDisabled(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	created, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "x"})
	require.NoError(t, err)

	disabled := false
	updated, err := svc.Update(created.UUID, nil, &disabled, nil, nil)
	require.NoError(t, err)
	assert.False(t, updated.Enabled)

	var persisted models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", created.UUID).First(&persisted).Error)
	assert.False(t, persisted.Enabled, "persisted row must have enabled=false after Update, not the DB default")
}

// TestBackupRemoteService_Update_EmptySecretsKeepsExisting proves an empty
// (zero-value) RemoteTargetSecrets on Update means "keep existing secrets"
// (spec §3.3.2), never wiping them.
func TestBackupRemoteService_Update_EmptySecretsKeepsExisting(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	created, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "keep-me"})
	require.NoError(t, err)

	emptySecrets := RemoteTargetSecrets{}
	updated, err := svc.Update(created.UUID, nil, nil, nil, &emptySecrets)
	require.NoError(t, err)

	secrets, err := svc.decryptSecrets(updated)
	require.NoError(t, err)
	assert.Equal(t, "keep-me", secrets.Password)
}

// --- Delete ---

func TestBackupRemoteService_Delete_Success(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	created, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "x"})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(created.UUID))

	_, err = svc.Get(created.UUID)
	require.Error(t, err)
}

// --- decryptSecrets ---

func TestDecryptSecrets_EmptyEncryptedBlob_ReturnsZeroValue(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	secrets, err := svc.decryptSecrets(&models.RemoteStorageTarget{SecretsEncrypted: ""})
	require.NoError(t, err)
	assert.True(t, secrets.isEmpty())
}

func TestDecryptSecrets_NoEncryptionService_ReturnsError(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	_, err := svc.decryptSecrets(&models.RemoteStorageTarget{SecretsEncrypted: "some-ciphertext"})
	require.ErrorIs(t, err, ErrEncryptionKeyMissing)
}

func TestDecryptSecrets_DecryptFailure(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	_, err := svc.decryptSecrets(&models.RemoteStorageTarget{SecretsEncrypted: "not-valid-base64-ciphertext!!"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decrypt remote target secrets")
}

func TestDecryptSecrets_UnmarshalFailure(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	// Encrypt something that decrypts fine but isn't valid JSON.
	ciphertext, err := enc.Encrypt([]byte("this is not json"))
	require.NoError(t, err)

	_, err = svc.decryptSecrets(&models.RemoteStorageTarget{SecretsEncrypted: ciphertext})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse decrypted remote target secrets")
}

// --- uploaderFor ---

func TestUploaderFor_DecryptSecretsError(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	_, err := svc.uploaderFor(&models.RemoteStorageTarget{Type: "sftp", SecretsEncrypted: "x"})
	require.ErrorIs(t, err, ErrEncryptionKeyMissing)
}

func TestUploaderFor_FactoryNilFallsBackToRemoteStorageNew(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())
	svc.uploaderFactory = nil

	_, err := svc.uploaderFor(&models.RemoteStorageTarget{Type: "unknown-type"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "construct uploader for target")
}

func TestUploaderFor_FactoryError_Wrapped(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())
	svc.uploaderFactory = func(*models.RemoteStorageTarget, map[string]string, remotestorage.TokenSaver) (remotestorage.Uploader, error) {
		return nil, assertAnError
	}

	target := &models.RemoteStorageTarget{Name: "Broken", Type: "sftp"}
	_, err := svc.uploaderFor(target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `construct uploader for target "Broken"`)
	assert.Contains(t, err.Error(), "boom")
}

// --- Test() ---

func TestBackupRemoteService_Test_TargetNotFound(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	err := svc.Test(context.Background(), "does-not-exist")
	require.Error(t, err)
}

func TestBackupRemoteService_Test_UploaderConstructionFailure_RecordsOutcome(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())
	svc.uploaderFactory = func(*models.RemoteStorageTarget, map[string]string, remotestorage.TokenSaver) (remotestorage.Uploader, error) {
		return nil, assertAnError
	}

	target := models.RemoteStorageTarget{Name: "Broken", Type: "sftp", ConfigJSON: `{"host":"nas.lan"}`}
	require.NoError(t, db.Create(&target).Error)

	err := svc.Test(context.Background(), target.UUID)
	require.Error(t, err)

	var reloaded models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", target.UUID).First(&reloaded).Error)
	assert.Equal(t, "failed", reloaded.LastTestStatus)
	assert.Contains(t, reloaded.LastError, "construct uploader for target")
}

// --- TriggerUpload guard clauses ---

func TestTriggerUpload_NilDB_NoOp(t *testing.T) {
	svc := &BackupRemoteService{db: nil}
	require.NotPanics(t, func() {
		svc.TriggerUpload(context.Background(), &models.BackupRecord{Filename: "backup_x.zip"})
	})
}

func TestTriggerUpload_NilRecord_NoOp(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())
	require.NotPanics(t, func() {
		svc.TriggerUpload(context.Background(), nil)
	})
}

// TestUploadToTarget_UploaderConstructionFailure_FailsCopyRow proves the
// uploaderFor error branch inside uploadToTarget (distinct from an Upload()
// call failing) still records the copy row as failed.
func TestUploadToTarget_UploaderConstructionFailure_FailsCopyRow(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())
	svc.uploaderFactory = func(*models.RemoteStorageTarget, map[string]string, remotestorage.TokenSaver) (remotestorage.Uploader, error) {
		return nil, assertAnError
	}

	target := models.RemoteStorageTarget{Name: "Broken", Type: "sftp", Enabled: true, ConfigJSON: `{"host":"nas.lan"}`}
	require.NoError(t, db.Create(&target).Error)
	record := models.BackupRecord{Filename: "backup_2026-07-08_00-00-00.zip", Type: "manual", Status: "completed"}
	require.NoError(t, db.Create(&record).Error)

	svc.TriggerUpload(context.Background(), &record)

	var copyRow models.BackupRemoteCopy
	require.NoError(t, db.Where("backup_record_id = ?", record.ID).First(&copyRow).Error)
	assert.Equal(t, "failed", copyRow.Status)
	assert.Contains(t, copyRow.ErrorMessage, "construct uploader for target")
}

// --- pruneRemoteRetention edge cases ---

func TestPruneRemoteRetention_NonPositiveRetentionCount_UsesDefault(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	fake := &fakeUploader{}
	for i := 0; i < defaultRemoteRetentionCount+3; i++ {
		name := "backup_" + string(rune('a'+i)) + ".zip"
		fake.objects = append(fake.objects, remotestorage.RemoteObject{Key: name, Name: name})
	}

	svc.pruneRemoteRetention(context.Background(), fake, "", 0)

	remaining, err := fake.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, remaining, defaultRemoteRetentionCount, "a non-positive retentionCount must fall back to defaultRemoteRetentionCount")
}

func TestPruneRemoteRetention_ListError_LoggedNotPanicked(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	fake := &fakeUploader{}
	listErrUploader := &listErrorUploader{fakeUploader: fake, err: assertAnError}

	require.NotPanics(t, func() {
		svc.pruneRemoteRetention(context.Background(), listErrUploader, "", 2)
	})
}

func TestPruneRemoteRetention_DeleteError_LoggedNotPanicked(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	now := time.Now()
	fake := &fakeUploader{objects: []remotestorage.RemoteObject{
		{Key: "backup_1.zip", Name: "backup_1.zip", LastModified: now.Add(-time.Hour)},
		{Key: "backup_2.zip", Name: "backup_2.zip", LastModified: now},
	}}
	deleteErrUploader := &deleteErrorUploader{fakeUploader: fake, err: assertAnError}

	require.NotPanics(t, func() {
		svc.pruneRemoteRetention(context.Background(), deleteErrUploader, "", 1)
	})
}

// listErrorUploader / deleteErrorUploader wrap fakeUploader to force
// List/Delete failures without adding error-injection fields to the shared
// fakeUploader type other tests in this package depend on.

type listErrorUploader struct {
	*fakeUploader
	err error
}

func (l *listErrorUploader) List(_ context.Context, _ string) ([]remotestorage.RemoteObject, error) {
	return nil, l.err
}

type deleteErrorUploader struct {
	*fakeUploader
	err error
}

func (d *deleteErrorUploader) Delete(_ context.Context, remoteKey string) error {
	return d.err
}

// TestBackupRemoteService_List_DatabaseError proves List's error branch
// (line ~113) surfaces a wrapped error when the underlying query fails,
// forced here by closing the database connection out from under it.
func TestBackupRemoteService_List_DatabaseError(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = svc.List()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list remote storage targets")
}

// TestBackupRemoteService_Create_DatabaseWriteFails proves Create's own
// db.Create error branch (distinct from every other Create failure mode
// already covered — encryption-key-missing, invalid config): validation
// and encryption both succeed here, only the final persist fails.
func TestBackupRemoteService_Create_DatabaseWriteFails(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	target, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "hunter2"})
	require.Error(t, err)
	assert.Nil(t, target)
	assert.Contains(t, err.Error(), "create remote storage target")
}

// TestBackupRemoteService_Update_DatabaseWriteFails proves Update's own
// db.Save error branch: the target is found and every field validates
// fine, only the final persist fails. The connection is closed only after
// the Get lookup that Update itself performs internally would otherwise
// need, so a fresh single-connection DB with query_only enabled is used
// instead of a fully-closed connection, letting the SELECT succeed while
// the UPDATE fails.
func TestBackupRemoteService_Update_DatabaseWriteFails(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	target, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.20"}, RemoteTargetSecrets{Password: "hunter2"})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)

	newName := "Renamed NAS"
	updated, err := svc.Update(target.UUID, &newName, nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Contains(t, err.Error(), "update remote storage target")
}
