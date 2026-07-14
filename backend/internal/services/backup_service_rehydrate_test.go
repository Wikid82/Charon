package services

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateSQLiteSnapshot_InvalidDBPath(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "missing-parent", "missing.db")
	_, _, err := createSQLiteSnapshot(badPath)
	require.Error(t, err)
}

func TestCheckpointSQLiteDatabase_InvalidDBPath(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "missing-parent", "missing.db")
	err := checkpointSQLiteDatabase(badPath)
	require.Error(t, err)
}

func TestBackupService_RehydrateLiveDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	dbPath := filepath.Join(dataDir, "charon.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)
	require.NoError(t, db.Exec("PRAGMA wal_autocheckpoint=0").Error)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	seedUser := models.User{
		UUID:    uuid.NewString(),
		Email:   "restore-user@example.com",
		Name:    "Restore User",
		Role:    models.RoleUser,
		Enabled: true,
		APIKey:  uuid.NewString(),
	}
	require.NoError(t, db.Create(&seedUser).Error)

	svc := NewBackupService(&config.Config{DatabasePath: dbPath}, nil, nil)
	defer svc.Stop()

	backupFile, err := svc.CreateBackup()
	require.NoError(t, err)

	require.NoError(t, db.Where("1 = 1").Delete(&models.User{}).Error)
	var countAfterDelete int64
	require.NoError(t, db.Model(&models.User{}).Count(&countAfterDelete).Error)
	require.Equal(t, int64(0), countAfterDelete)

	require.NoError(t, svc.RestoreBackup(backupFile))
	require.NoError(t, svc.RehydrateLiveDatabase(db))

	var restoredUsers []models.User
	require.NoError(t, db.Find(&restoredUsers).Error)
	require.Len(t, restoredUsers, 1)
	assert.Equal(t, "restore-user@example.com", restoredUsers[0].Email)
}

func TestBackupService_RehydrateLiveDatabase_FromBackupWithWAL(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	dbPath := filepath.Join(dataDir, "charon.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)
	require.NoError(t, db.Exec("PRAGMA wal_autocheckpoint=0").Error)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	seedUser := models.User{
		UUID:    uuid.NewString(),
		Email:   "restore-from-wal@example.com",
		Name:    "Restore From WAL",
		Role:    models.RoleUser,
		Enabled: true,
		APIKey:  uuid.NewString(),
	}
	require.NoError(t, db.Create(&seedUser).Error)

	walPath := dbPath + "-wal"
	_, err = os.Stat(walPath)
	require.NoError(t, err)

	svc := NewBackupService(&config.Config{DatabasePath: dbPath}, nil, nil)
	defer svc.Stop()

	backupName := "backup_with_wal.zip"
	backupPath := filepath.Join(svc.BackupDir, backupName)
	backupFile, err := os.Create(backupPath) // #nosec G304 -- backupPath is built from service BackupDir and fixed test filename
	require.NoError(t, err)
	zipWriter := zip.NewWriter(backupFile)

	addFileToZip := func(sourcePath, zipEntryName string) {
		sourceFile, openErr := os.Open(sourcePath) // #nosec G304 -- sourcePath is provided by test with controlled db/wal paths under TempDir
		require.NoError(t, openErr)
		defer func() {
			_ = sourceFile.Close()
		}()

		zipEntry, createErr := zipWriter.Create(zipEntryName)
		require.NoError(t, createErr)
		_, copyErr := io.Copy(zipEntry, sourceFile)
		require.NoError(t, copyErr)
	}

	addFileToZip(dbPath, svc.DatabaseName)
	addFileToZip(walPath, svc.DatabaseName+"-wal")
	require.NoError(t, zipWriter.Close())
	require.NoError(t, backupFile.Close())

	require.NoError(t, db.Where("1 = 1").Delete(&models.User{}).Error)
	require.NoError(t, svc.RestoreBackup(backupName))
	require.NoError(t, svc.RehydrateLiveDatabase(db))

	var restoredUsers []models.User
	require.NoError(t, db.Find(&restoredUsers).Error)
	require.Len(t, restoredUsers, 1)
	assert.Equal(t, "restore-from-wal@example.com", restoredUsers[0].Email)
}

func TestBackupService_ExtractDatabaseFromBackup_WALCheckpointFailure(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "with-invalid-wal.zip")

	zipFile, err := os.Create(zipPath) //nolint:gosec
	require.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	dbEntry, err := writer.Create("charon.db")
	require.NoError(t, err)
	_, err = dbEntry.Write([]byte("not-a-valid-sqlite-db"))
	require.NoError(t, err)

	walEntry, err := writer.Create("charon.db-wal")
	require.NoError(t, err)
	_, err = walEntry.Write([]byte("not-a-valid-wal"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())
	require.NoError(t, zipFile.Close())

	svc := &BackupService{DatabaseName: "charon.db"}
	_, err = svc.extractDatabaseFromBackup(zipPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "checkpoint extracted sqlite wal")
}

func TestBackupService_RehydrateLiveDatabase_InvalidRestoreDB(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDBPath := filepath.Join(dataDir, "charon.db")
	activeDB, err := gorm.Open(sqlite.Open(activeDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec("CREATE TABLE IF NOT EXISTS healthcheck (id INTEGER PRIMARY KEY, value TEXT)").Error)

	invalidRestorePath := filepath.Join(tmpDir, "invalid-restore.sqlite")
	require.NoError(t, os.WriteFile(invalidRestorePath, []byte("invalid sqlite content"), 0o600))

	svc := &BackupService{
		DataDir:       dataDir,
		DatabaseName:  "charon.db",
		restoreDBPath: invalidRestorePath,
	}

	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "attach restored database")
}

func TestBackupService_RehydrateLiveDatabase_InvalidTableIdentifier(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDBPath := filepath.Join(dataDir, "charon.db")
	activeDB, err := gorm.Open(sqlite.Open(activeDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec("CREATE TABLE \"bad-name\" (id INTEGER PRIMARY KEY, value TEXT)").Error)

	restoreDBPath := filepath.Join(tmpDir, "restore.sqlite")
	restoreDB, err := gorm.Open(sqlite.Open(restoreDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, restoreDB.Exec("CREATE TABLE \"bad-name\" (id INTEGER PRIMARY KEY, value TEXT)").Error)
	require.NoError(t, restoreDB.Exec("INSERT INTO \"bad-name\" (value) VALUES (?)", "ok").Error)

	svc := &BackupService{
		DataDir:       dataDir,
		DatabaseName:  "charon.db",
		restoreDBPath: restoreDBPath,
	}

	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "quote table identifier")
}

func TestBackupService_CreateSQLiteSnapshot_TempDirInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	originalTmp := os.Getenv("TMPDIR")
	t.Setenv("TMPDIR", filepath.Join(tmpDir, "nonexistent-tmp"))
	defer func() {
		_ = os.Setenv("TMPDIR", originalTmp)
	}()

	_, _, err := createSQLiteSnapshot(dbPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create sqlite snapshot file")
}

// TestBackupService_RehydrateLiveDatabase_MidLoopFailure_RollsBackAtomically
// is required coverage for H1 (spec §3.4.2): a mid-loop INSERT failure on a
// table that comes AFTER at least one other table has already been fully
// swapped must leave every table's live data byte-for-byte/row-for-row
// identical to its pre-rehydrate state (full rollback) — not "an error was
// returned" while some tables silently kept the new data and others were
// left empty.
//
// first_table is processed before second_table (SQLite returns
// sqlite_master rows in creation order, and no ORDER BY is present in the
// production query), so by the time second_table's INSERT fails,
// first_table's DELETE+INSERT has already fully completed. Before the H1
// fix (no transaction wrap), that leaves first_table on the NEW data and
// second_table emptied (its old row deleted, the new rows rejected by its
// own UNIQUE constraint) — proven by this test failing against pre-fix code
// (see PR history/commit description). After the H1 fix (the whole loop
// wrapped in db.Transaction), the same failure rolls back atomically and
// both tables are restored to their exact pre-rehydrate contents.
func TestBackupService_RehydrateLiveDatabase_MidLoopFailure_RollsBackAtomically(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDBPath := filepath.Join(dataDir, "charon.db")
	activeDB, err := gorm.Open(sqlite.Open(activeDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec(`CREATE TABLE first_table (id INTEGER PRIMARY KEY, value TEXT)`).Error)
	require.NoError(t, activeDB.Exec(`INSERT INTO first_table (value) VALUES ('old-first')`).Error)
	require.NoError(t, activeDB.Exec(`CREATE TABLE second_table (id INTEGER PRIMARY KEY, value TEXT UNIQUE)`).Error)
	require.NoError(t, activeDB.Exec(`INSERT INTO second_table (value) VALUES ('old-second')`).Error)

	// The restore source: first_table has new, distinct data (its swap will
	// succeed); second_table has two duplicate values, which will violate
	// the LIVE table's UNIQUE(value) constraint when INSERT ... SELECT runs
	// against it, failing the statement partway through second_table's copy.
	restoreDBPath := filepath.Join(tmpDir, "restore.sqlite")
	restoreDB, err := sql.Open("sqlite3", restoreDBPath)
	require.NoError(t, err)
	_, err = restoreDB.Exec(`CREATE TABLE first_table (id INTEGER PRIMARY KEY, value TEXT)`)
	require.NoError(t, err)
	_, err = restoreDB.Exec(`INSERT INTO first_table (value) VALUES ('new-first')`)
	require.NoError(t, err)
	_, err = restoreDB.Exec(`CREATE TABLE second_table (id INTEGER PRIMARY KEY, value TEXT)`)
	require.NoError(t, err)
	_, err = restoreDB.Exec(`INSERT INTO second_table (value) VALUES ('dup')`)
	require.NoError(t, err)
	_, err = restoreDB.Exec(`INSERT INTO second_table (value) VALUES ('dup')`)
	require.NoError(t, err)
	require.NoError(t, restoreDB.Close())

	svc := &BackupService{
		DataDir:       dataDir,
		DatabaseName:  "charon.db",
		restoreDBPath: restoreDBPath,
	}

	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err, "the mid-loop UNIQUE constraint violation on second_table must still surface as an error")
	assert.Contains(t, err.Error(), "copy table second_table")

	// The real assertion (H1): every table's live data, INCLUDING
	// first_table which was already fully swapped before second_table
	// failed, must be identical to its pre-rehydrate state. A partial
	// rollback (first_table left on new data, or second_table left empty)
	// must not be possible.
	var firstValues []string
	require.NoError(t, activeDB.Raw(`SELECT value FROM first_table ORDER BY id`).Scan(&firstValues).Error)
	assert.Equal(t, []string{"old-first"}, firstValues, "first_table must be rolled back to its pre-rehydrate state even though its own swap had already succeeded")

	var secondValues []string
	require.NoError(t, activeDB.Raw(`SELECT value FROM second_table ORDER BY id`).Scan(&secondValues).Error)
	assert.Equal(t, []string{"old-second"}, secondValues, "second_table must retain its pre-rehydrate row, not be left empty")
}

func TestBackupService_RunScheduledBackup_CreateBackupAndCleanupHooks(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	cfg := &config.Config{DatabasePath: filepath.Join(dataDir, "charon.db")}
	service := NewBackupService(cfg, nil, nil)
	defer service.Stop()

	createCalls := 0
	cleanupCalls := 0
	service.createBackup = func() (string, error) {
		createCalls++
		return fmt.Sprintf("backup-%d.zip", createCalls), nil
	}
	service.cleanupOld = func(keep int) (int, error) {
		cleanupCalls++
		return 1, nil
	}

	service.RunScheduledBackup()
	require.Equal(t, 1, createCalls)
	require.Equal(t, 1, cleanupCalls)
}
