package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto/cryptotest"
	"github.com/Wikid82/charon/backend/internal/models"
)

// newSharedRemoteTestDB opens a private, shared-cache database so triggers installed by a test apply to every pooled
// connection, and migrates the tables the remote target code uses.
func newSharedRemoteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteStorageTarget{}, &models.BackupRecord{}, &models.BackupRemoteCopy{}, &models.Setting{}))
	return db
}

func TestBackupRemoteDelete_StorageFailuresAreReported(t *testing.T) {
	t.Run("lookup failure", func(t *testing.T) {
		db := newSharedRemoteTestDB(t)
		svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
		require.NoError(t, db.Migrator().DropTable(&models.RemoteStorageTarget{}))

		err := svc.Delete("any-uuid")

		assert.ErrorContains(t, err, "delete remote storage target")
		assert.ErrorContains(t, err, "find remote storage target")
	})

	t.Run("upload history delete failure", func(t *testing.T) {
		db := newSharedRemoteTestDB(t)
		svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
		target := seedWebDAVTarget(t, db, "history", `{"webdav":{"url":"https://203.0.113.10/dav/"}}`, "")
		require.NoError(t, db.Create(&models.BackupRemoteCopy{BackupRecordID: 1, RemoteTargetID: target.ID, Status: "uploaded"}).Error)
		refuseOn(t, db, "DELETE", "backup_remote_copies")

		err := svc.Delete(target.UUID)

		assert.ErrorContains(t, err, "delete upload history of remote storage target")
	})

	t.Run("target delete failure", func(t *testing.T) {
		db := newSharedRemoteTestDB(t)
		svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
		target := seedWebDAVTarget(t, db, "target", `{"webdav":{"url":"https://203.0.113.10/dav/"}}`, "")
		refuseOn(t, db, "DELETE", "remote_storage_targets")

		err := svc.Delete(target.UUID)

		assert.ErrorContains(t, err, "delete remote storage target")
		assert.NotContains(t, err.Error(), "upload history")
	})
}

func TestMarkExistingInsecureWebDAV_UpdateFailureIsReported(t *testing.T) {
	db := newSharedRemoteTestDB(t)
	svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
	seedWebDAVTarget(t, db, "legacy-update", `{"webdav":{"url":"http://203.0.113.10/dav/"}}`, "")
	refuseOn(t, db, "UPDATE", "remote_storage_targets")

	count, err := svc.MarkExistingInsecureWebDAV(context.Background())

	assert.ErrorContains(t, err, "flag webdav remote target")
	assert.Zero(t, count)
}

func TestRehydrate_BackfillFailureIsReported(t *testing.T) {
	db := newSharedRemoteTestDB(t)
	svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
	require.NoError(t, db.Migrator().DropTable(&models.RemoteStorageTarget{}))

	err := svc.Rehydrate(context.Background())

	assert.ErrorContains(t, err, "backfill webdav remote targets after restore")
}

func TestFlagInsecureWebDAVConfig_NonWebDAVConfigIsUnchanged(t *testing.T) {
	raw := `{"endpoint":"http://203.0.113.10"}`

	updated, changed, err := flagInsecureWebDAVConfig(raw)

	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, raw, updated)
}
