package services

import (
	"context"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services/remotestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// fakeUploader is the required test #10 fake Uploader: an in-memory
// implementation of remotestorage.Uploader used to exercise
// BackupRemoteService's upload/retention-pruning flow without a real
// S3/SFTP endpoint.
type fakeUploader struct {
	mu        sync.Mutex
	uploaded  []string
	deleted   []string
	objects   []remotestorage.RemoteObject
	uploadErr error
	testErr   error
}

func (f *fakeUploader) Upload(_ context.Context, _, remoteKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.uploadErr != nil {
		return f.uploadErr
	}
	f.uploaded = append(f.uploaded, remoteKey)
	f.objects = append(f.objects, remotestorage.RemoteObject{Key: remoteKey, Name: path.Base(remoteKey), Size: 1, LastModified: time.Now()})
	return nil
}

func (f *fakeUploader) Delete(_ context.Context, remoteKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, remoteKey)
	filtered := f.objects[:0]
	for _, obj := range f.objects {
		if obj.Key != remoteKey {
			filtered = append(filtered, obj)
		}
	}
	f.objects = filtered
	return nil
}

func (f *fakeUploader) List(_ context.Context, _ string) ([]remotestorage.RemoteObject, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]remotestorage.RemoteObject, len(f.objects))
	copy(out, f.objects)
	return out, nil
}

func (f *fakeUploader) Test(_ context.Context) error {
	return f.testErr
}

func newRemoteServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteStorageTarget{}, &models.BackupRecord{}, &models.BackupRemoteCopy{}, &models.Setting{}))
	return db
}

// TestBackupRemoteService_UploadFailure_DoesNotFailBackup is required test
// #10 (part 1): an upload failure to one target must be recorded on that
// target's BackupRemoteCopy row only — TriggerUpload itself never returns
// an error, so the backup-creation call site can't fail because of it
// (spec §3.7).
func TestBackupRemoteService_UploadFailure_DoesNotFailBackup(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	fake := &fakeUploader{uploadErr: assertAnError}
	svc.uploaderFactory = func(*models.RemoteStorageTarget, map[string]string) (remotestorage.Uploader, error) {
		return fake, nil
	}

	target := models.RemoteStorageTarget{Name: "Broken NAS", Type: "sftp", Enabled: true, ConfigJSON: `{"host":"nas.lan"}`}
	require.NoError(t, db.Create(&target).Error)

	record := models.BackupRecord{Filename: "backup_2026-07-08_00-00-00.zip", Type: "manual", Status: "completed"}
	require.NoError(t, db.Create(&record).Error)

	require.NotPanics(t, func() {
		svc.TriggerUpload(context.Background(), &record)
	})

	var copyRow models.BackupRemoteCopy
	require.NoError(t, db.Where("backup_record_id = ?", record.ID).First(&copyRow).Error)
	assert.Equal(t, "failed", copyRow.Status)
	assert.Contains(t, copyRow.ErrorMessage, "boom")

	// The BackupRecord itself is completely unaffected by the upload failure.
	var reloaded models.BackupRecord
	require.NoError(t, db.First(&reloaded, record.ID).Error)
	assert.Equal(t, "completed", reloaded.Status)
}

// TestBackupRemoteService_UploadSuccess_RecordsUploadedCopy proves the
// happy path: a successful upload transitions the copy row through
// pending -> uploading -> uploaded with RemoteKey and UploadedAt populated.
func TestBackupRemoteService_UploadSuccess_RecordsUploadedCopy(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	fake := &fakeUploader{}
	svc.uploaderFactory = func(*models.RemoteStorageTarget, map[string]string) (remotestorage.Uploader, error) {
		return fake, nil
	}

	target := models.RemoteStorageTarget{Name: "Home NAS", Type: "sftp", Enabled: true, ConfigJSON: `{"host":"nas.lan","path_prefix":"charon"}`}
	require.NoError(t, db.Create(&target).Error)

	record := models.BackupRecord{Filename: "backup_2026-07-08_00-00-00.zip", Type: "manual", Status: "completed"}
	require.NoError(t, db.Create(&record).Error)

	svc.TriggerUpload(context.Background(), &record)

	var copyRow models.BackupRemoteCopy
	require.NoError(t, db.Where("backup_record_id = ?", record.ID).First(&copyRow).Error)
	assert.Equal(t, "uploaded", copyRow.Status)
	assert.NotNil(t, copyRow.UploadedAt)
	assert.Contains(t, fake.uploaded, copyRow.RemoteKey)
}

// TestBackupRemoteService_DisabledTarget_NeverUploaded proves only enabled
// targets are considered.
func TestBackupRemoteService_DisabledTarget_NeverUploaded(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	fake := &fakeUploader{}
	svc.uploaderFactory = func(*models.RemoteStorageTarget, map[string]string) (remotestorage.Uploader, error) {
		return fake, nil
	}

	target := models.RemoteStorageTarget{Name: "Disabled", Type: "sftp", Enabled: true, ConfigJSON: `{"host":"nas.lan"}`}
	require.NoError(t, db.Create(&target).Error)
	// GORM's `default:true` tag on Enabled means a zero-value (false) field
	// is omitted from the INSERT and the column default wins instead — set
	// it false via an explicit UPDATE, which does not have that quirk.
	require.NoError(t, db.Model(&target).Update("enabled", false).Error)

	record := models.BackupRecord{Filename: "backup_2026-07-08_00-00-00.zip", Type: "manual", Status: "completed"}
	require.NoError(t, db.Create(&record).Error)

	svc.TriggerUpload(context.Background(), &record)

	var count int64
	db.Model(&models.BackupRemoteCopy{}).Count(&count)
	assert.Equal(t, int64(0), count)
	assert.Empty(t, fake.uploaded)
}

// TestBackupRemoteService_PruneRemoteRetention is required test #10 (part
// 2): remote objects matching backup_*.zip* beyond the retention count are
// pruned, newest by LastModified first, and only Charon-named keys are ever
// deleted.
func TestBackupRemoteService_PruneRemoteRetention(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	now := time.Now()
	fake := &fakeUploader{objects: []remotestorage.RemoteObject{
		{Key: "backup_1.zip", Name: "backup_1.zip", LastModified: now.Add(-5 * time.Hour)},
		{Key: "backup_2.zip", Name: "backup_2.zip", LastModified: now.Add(-4 * time.Hour)},
		{Key: "backup_3.zip", Name: "backup_3.zip", LastModified: now.Add(-3 * time.Hour)},
		{Key: "backup_4.zip", Name: "backup_4.zip", LastModified: now.Add(-2 * time.Hour)},
		{Key: "not-a-charon-file.txt", Name: "not-a-charon-file.txt", LastModified: now.Add(-10 * time.Hour)},
	}}

	svc.pruneRemoteRetention(context.Background(), fake, "", 2)

	// Only the 2 newest backup_*.zip objects should survive; the
	// non-Charon file must never be touched regardless of age.
	remaining, err := fake.List(context.Background(), "")
	require.NoError(t, err)

	var remainingKeys []string
	for _, obj := range remaining {
		remainingKeys = append(remainingKeys, obj.Key)
	}
	assert.ElementsMatch(t, []string{"backup_4.zip", "backup_3.zip", "not-a-charon-file.txt"}, remainingKeys)
	assert.ElementsMatch(t, []string{"backup_1.zip", "backup_2.zip"}, fake.deleted)
}

// TestBackupRemoteService_PruneRemoteRetention_KeyDiffersFromName proves the
// Locator abstraction (spec §3.2, Issue #32 Phase 2): candidate filtering
// must use Name (human-readable filename) while deletion must use Key (the
// provider-native locator) — this is the exact split Google Drive's opaque
// file IDs require in Commit 3, and this test locks in that
// pruneRemoteRetention already respects it even though every provider wired
// up so far (S3/SFTP) happens to have Key derived from Name.
func TestBackupRemoteService_PruneRemoteRetention_KeyDiffersFromName(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	now := time.Now()
	fake := &fakeUploader{objects: []remotestorage.RemoteObject{
		{Key: "opaque-id-1", Name: "backup_1.zip", LastModified: now.Add(-2 * time.Hour)},
		{Key: "opaque-id-2", Name: "backup_2.zip", LastModified: now.Add(-1 * time.Hour)},
	}}

	svc.pruneRemoteRetention(context.Background(), fake, "", 1)

	remaining, err := fake.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, "opaque-id-2", remaining[0].Key)
	assert.Equal(t, "backup_2.zip", remaining[0].Name)
	// Deletion must use the opaque Key, never Name.
	assert.Equal(t, []string{"opaque-id-1"}, fake.deleted)
}

// TestBackupRemoteService_TestConnection_RecordsOutcome proves Test()
// records success/failure and timestamps on the target row.
func TestBackupRemoteService_TestConnection_RecordsOutcome(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())

	fake := &fakeUploader{}
	svc.uploaderFactory = func(*models.RemoteStorageTarget, map[string]string) (remotestorage.Uploader, error) {
		return fake, nil
	}

	target := models.RemoteStorageTarget{Name: "Home NAS", Type: "sftp", ConfigJSON: `{"host":"nas.lan"}`}
	require.NoError(t, db.Create(&target).Error)

	require.NoError(t, svc.Test(context.Background(), target.UUID))

	var reloaded models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", target.UUID).First(&reloaded).Error)
	assert.Equal(t, "ok", reloaded.LastTestStatus)
	assert.NotNil(t, reloaded.LastTestAt)

	fake.testErr = assertAnError
	err := svc.Test(context.Background(), target.UUID)
	require.Error(t, err)

	require.NoError(t, db.Where("uuid = ?", target.UUID).First(&reloaded).Error)
	assert.Equal(t, "failed", reloaded.LastTestStatus)
	assert.Contains(t, reloaded.LastError, "boom")
}

// TestReconcileStuckUploadingCopies proves a copy row left "uploading" from
// a prior crash is transitioned to "failed" exactly once at construction
// (spec §3.7).
func TestReconcileStuckUploadingCopies(t *testing.T) {
	db := newRemoteServiceTestDB(t)

	record := models.BackupRecord{Filename: "backup_x.zip", Type: "manual", Status: "completed"}
	require.NoError(t, db.Create(&record).Error)
	target := models.RemoteStorageTarget{Name: "NAS", Type: "sftp", ConfigJSON: "{}"}
	require.NoError(t, db.Create(&target).Error)
	stuck := models.BackupRemoteCopy{BackupRecordID: record.ID, RemoteTargetID: target.ID, Status: "uploading"}
	require.NoError(t, db.Create(&stuck).Error)

	require.NoError(t, reconcileStuckUploadingCopies(db))

	var reloaded models.BackupRemoteCopy
	require.NoError(t, db.First(&reloaded, stuck.ID).Error)
	assert.Equal(t, "failed", reloaded.Status)
}

var assertAnError = &testBoomError{}

type testBoomError struct{}

func (*testBoomError) Error() string { return "boom: simulated upload failure" }
