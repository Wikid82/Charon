package services

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/database"
)

// rehydrateReferenceDDL is the part of the schema the live restore repairs: a host with optional references and its
// locations, which must go when the host does.
var rehydrateReferenceDDL = []string{
	"CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY)",
	"CREATE TABLE proxy_hosts (id INTEGER PRIMARY KEY, uuid TEXT, certificate_id INTEGER REFERENCES ssl_certificates(id))",
	"CREATE TABLE locations (id INTEGER PRIMARY KEY, proxy_host_id INTEGER NOT NULL REFERENCES proxy_hosts(id) ON DELETE CASCADE)",
}

// openRehydrateReferenceDB opens a file database with the production connection settings and the reference schema.
func openRehydrateReferenceDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := database.Connect(path)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, stmt := range rehydrateReferenceDDL {
		require.NoError(t, db.Exec(stmt).Error, stmt)
	}
	return db
}

// newRehydrateService builds a backup service whose restore source is restorePath.
func newRehydrateService(t *testing.T, livePath, restorePath string) *BackupService {
	t.Helper()
	svc := NewBackupService(&config.Config{DatabasePath: livePath}, nil, nil)
	t.Cleanup(svc.discardRestoreSnapshot)
	t.Cleanup(svc.Stop)
	svc.restoreDBPath = restorePath
	return svc
}

// TestRehydrateLiveDatabase_RepairsReferences covers a restore whose archive holds a host with a missing certificate:
// the host is restored with the reference cleared, its location is kept, and enforcement is on afterwards.
func TestRehydrateLiveDatabase_RepairsReferences(t *testing.T) {
	dir := t.TempDir()
	live := openRehydrateReferenceDB(t, filepath.Join(dir, "live.db"))
	require.NoError(t, live.Exec("INSERT INTO proxy_hosts (id, uuid) VALUES (5, 'old')").Error)

	restorePath := filepath.Join(dir, "restore.db")
	archive := openRehydrateReferenceDB(t, restorePath)
	require.NoError(t, archive.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, archive.Exec("INSERT INTO proxy_hosts (id, uuid, certificate_id) VALUES (1, 'restored', 77)").Error)
	require.NoError(t, archive.Exec("INSERT INTO locations (id, proxy_host_id) VALUES (1, 1)").Error)
	require.NoError(t, archive.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error) // the restore copies the main file only
	sqlArchive, err := archive.DB()
	require.NoError(t, err)
	require.NoError(t, sqlArchive.Close())

	svc := newRehydrateService(t, filepath.Join(dir, "live.db"), restorePath)
	require.NoError(t, svc.RehydrateLiveDatabase(live))

	var certificateID *int64
	require.NoError(t, live.Raw("SELECT certificate_id FROM proxy_hosts WHERE uuid = 'restored'").Scan(&certificateID).Error)
	assert.Nil(t, certificateID, "the missing certificate is cleared")
	var locations int64
	require.NoError(t, live.Raw("SELECT COUNT(*) FROM locations WHERE proxy_host_id = 1").Scan(&locations).Error)
	assert.Equal(t, int64(1), locations, "the restored location stays with its host")
	var enforced int
	require.NoError(t, live.Raw("PRAGMA foreign_keys").Scan(&enforced).Error)
	assert.Equal(t, 1, enforced)
}

// TestRehydrateLiveDatabase_RepairFailureRollsBack covers a repair that fails during a restore: the live database keeps
// its rows from before the restore and the restore reports the error.
func TestRehydrateLiveDatabase_RepairFailureRollsBack(t *testing.T) {
	dir := t.TempDir()
	live := openRehydrateReferenceDB(t, filepath.Join(dir, "live.db"))
	require.NoError(t, live.Exec("INSERT INTO proxy_hosts (id, uuid) VALUES (5, 'before')").Error)
	require.NoError(t, live.Exec("CREATE TRIGGER refuse_repair BEFORE UPDATE ON proxy_hosts BEGIN SELECT RAISE(ABORT, 'refused by test'); END;").Error)

	restorePath := filepath.Join(dir, "restore.db")
	archive := openRehydrateReferenceDB(t, restorePath)
	require.NoError(t, archive.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, archive.Exec("INSERT INTO proxy_hosts (id, uuid, certificate_id) VALUES (1, 'restored', 77)").Error)
	require.NoError(t, archive.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error) // the restore copies the main file only
	sqlArchive, err := archive.DB()
	require.NoError(t, err)
	require.NoError(t, sqlArchive.Close())

	svc := newRehydrateService(t, filepath.Join(dir, "live.db"), restorePath)
	err = svc.RehydrateLiveDatabase(live)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repair references after restore")
	var uuids []string
	require.NoError(t, live.Raw("SELECT uuid FROM proxy_hosts").Scan(&uuids).Error)
	assert.Equal(t, []string{"before"}, uuids, "the failed restore leaves the live rows as they were")
}
