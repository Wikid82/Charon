package services

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openWave4ZipInTempDir(t *testing.T, tempDir, zipPath string) *os.File {
	t.Helper()

	absTempDir, err := filepath.Abs(tempDir)
	require.NoError(t, err)
	absZipPath, err := filepath.Abs(zipPath)
	require.NoError(t, err)

	relPath, err := filepath.Rel(absTempDir, absZipPath)
	require.NoError(t, err)
	require.False(t, relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)))

	// #nosec G304 -- absZipPath is constrained to test TempDir via Abs+Rel checks above.
	zipFile, err := os.OpenFile(absZipPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	require.NoError(t, err)

	return zipFile
}

func registerBackupRawErrorHook(t *testing.T, db *gorm.DB, name string, shouldFail func(*gorm.DB) bool) {
	t.Helper()
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(name, func(tx *gorm.DB) {
		if shouldFail(tx) {
			_ = tx.AddError(fmt.Errorf("forced raw failure"))
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Raw().Remove(name)
	})
}

func backupSQLContains(tx *gorm.DB, fragment string) bool {
	if tx == nil || tx.Statement == nil {
		return false
	}
	return strings.Contains(strings.ToLower(tx.Statement.SQL.String()), strings.ToLower(fragment))
}

func setupRehydrateDBPair(t *testing.T) (db *gorm.DB, activeDataDir, restoreDBPath string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDBPath := filepath.Join(tmpDir, "active.db")
	activeDB, err := gorm.Open(sqlite.Open(activeDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`).Error)

	restoreDBPath = filepath.Join(tmpDir, "restore.db")
	restoreDB, err := gorm.Open(sqlite.Open(restoreDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, restoreDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`).Error)
	require.NoError(t, restoreDB.Exec(`INSERT INTO users (name) VALUES ('alice')`).Error)

	return activeDB, dataDir, restoreDBPath
}

func TestBackupServiceWave4_Rehydrate_CheckpointWarningPath(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDBPath := filepath.Join(tmpDir, "active.db")
	activeDB, err := gorm.Open(sqlite.Open(activeDBPath), &gorm.Config{})
	require.NoError(t, err)

	// Place an invalid database file at DataDir/DatabaseName so checkpointSQLiteDatabase fails
	restoredDBPath := filepath.Join(dataDir, "charon.db")
	require.NoError(t, os.WriteFile(restoredDBPath, []byte("not-sqlite"), 0o600))

	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db"}
	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
}

func TestBackupServiceWave4_Rehydrate_CreateTempFailure(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	activeDB, err := gorm.Open(sqlite.Open(filepath.Join(tmpDir, "active.db")), &gorm.Config{})
	require.NoError(t, err)

	t.Setenv("TMPDIR", filepath.Join(tmpDir, "missing-temp-dir"))
	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db"}
	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create temporary restore database copy")
}

func TestBackupServiceWave4_Rehydrate_CopyErrorFromDirectorySource(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDB, err := gorm.Open(sqlite.Open(filepath.Join(tmpDir, "active.db")), &gorm.Config{})
	require.NoError(t, err)

	// Use a directory as restore source path so io.Copy fails deterministically.
	badSourceDir := filepath.Join(tmpDir, "restore-source-dir")
	require.NoError(t, os.MkdirAll(badSourceDir, 0o700))

	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db", restoreDBPath: badSourceDir}
	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "copy restored database to temporary file")
}

func TestBackupServiceWave4_Rehydrate_CopyTableErrorOnSchemaMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDBPath := filepath.Join(tmpDir, "active.db")
	activeDB, err := gorm.Open(sqlite.Open(activeDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`).Error)

	restoreDBPath := filepath.Join(tmpDir, "restore.db")
	restoreDB, err := gorm.Open(sqlite.Open(restoreDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, restoreDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, extra TEXT)`).Error)
	require.NoError(t, restoreDB.Exec(`INSERT INTO users (name, extra) VALUES ('alice', 'x')`).Error)

	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db", restoreDBPath: restoreDBPath}
	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "copy table users")
}

func TestBackupServiceWave4_ExtractDatabaseFromBackup_CreateTempError(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "backup.zip")

	zf := openWave4ZipInTempDir(t, tmpDir, zipPath)
	zw := zip.NewWriter(zf)
	entry, err := zw.Create("charon.db")
	require.NoError(t, err)
	_, err = entry.Write([]byte("sqlite-header-placeholder"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, zf.Close())

	t.Setenv("TMPDIR", filepath.Join(tmpDir, "missing-temp-dir"))

	svc := &BackupService{DatabaseName: "charon.db"}
	_, err = svc.extractDatabaseFromBackup(zipPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create restore snapshot file")
}

func TestBackupServiceWave4_UnzipWithSkip_MkdirParentError(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "nested.zip")

	zf := openWave4ZipInTempDir(t, tmpDir, zipPath)
	zw := zip.NewWriter(zf)
	entry, err := zw.Create("nested/file.txt")
	require.NoError(t, err)
	_, err = entry.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, zf.Close())

	// Make destination a regular file so MkdirAll(filepath.Dir(fpath)) fails with ENOTDIR.
	destFile := filepath.Join(tmpDir, "dest-as-file")
	require.NoError(t, os.WriteFile(destFile, []byte("block"), 0o600))

	svc := &BackupService{}
	err = svc.unzipWithSkip(zipPath, destFile, nil)
	require.Error(t, err)
}

func TestBackupServiceWave4_Rehydrate_ClearSQLiteSequenceError(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDB, err := gorm.Open(sqlite.Open(filepath.Join(tmpDir, "active.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)`).Error)

	restoreDBPath := filepath.Join(tmpDir, "restore.db")
	restoreDB, err := gorm.Open(sqlite.Open(restoreDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, restoreDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)`).Error)
	require.NoError(t, restoreDB.Exec(`INSERT INTO users (name) VALUES ('alice')`).Error)

	registerBackupRawErrorHook(t, activeDB, "wave4-clear-sqlite-sequence", func(tx *gorm.DB) bool {
		return backupSQLContains(tx, "delete from sqlite_sequence")
	})

	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db", restoreDBPath: restoreDBPath}
	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "clear sqlite_sequence")
}

func TestBackupServiceWave4_Rehydrate_CopySQLiteSequenceError(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDB, err := gorm.Open(sqlite.Open(filepath.Join(tmpDir, "active.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)`).Error)

	restoreDBPath := filepath.Join(tmpDir, "restore.db")
	restoreDB, err := gorm.Open(sqlite.Open(restoreDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, restoreDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)`).Error)
	require.NoError(t, restoreDB.Exec(`INSERT INTO users (name) VALUES ('alice')`).Error)

	registerBackupRawErrorHook(t, activeDB, "wave4-copy-sqlite-sequence", func(tx *gorm.DB) bool {
		return backupSQLContains(tx, "insert into sqlite_sequence select * from restore_src.sqlite_sequence")
	})

	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db", restoreDBPath: restoreDBPath}
	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "copy sqlite_sequence")
}

func TestBackupServiceWave4_Rehydrate_DetachErrorNotBusyOrLocked(t *testing.T) {
	activeDB, dataDir, restoreDBPath := setupRehydrateDBPair(t)

	registerBackupRawErrorHook(t, activeDB, "wave4-detach-error", func(tx *gorm.DB) bool {
		return backupSQLContains(tx, "detach database restore_src")
	})

	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db", restoreDBPath: restoreDBPath}
	err := svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "detach restored database")
}

func TestBackupServiceWave4_Rehydrate_WALCheckpointErrorNotBusyOrLocked(t *testing.T) {
	activeDB, dataDir, restoreDBPath := setupRehydrateDBPair(t)

	registerBackupRawErrorHook(t, activeDB, "wave4-wal-checkpoint-error", func(tx *gorm.DB) bool {
		return backupSQLContains(tx, "pragma wal_checkpoint(truncate)")
	})

	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db", restoreDBPath: restoreDBPath}
	err := svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "checkpoint wal after rehydrate")
}
