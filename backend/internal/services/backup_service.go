package services

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	_ "github.com/mattn/go-sqlite3"
)

func quoteSQLiteIdentifier(identifier string) (string, error) {
	if identifier == "" {
		return "", fmt.Errorf("sqlite identifier is empty")
	}

	for _, character := range identifier {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' {
			continue
		}
		return "", fmt.Errorf("sqlite identifier contains invalid characters: %s", identifier)
	}

	return `"` + identifier + `"`, nil
}

// SafeJoinPath sanitizes and validates file paths to prevent directory traversal attacks.
// It ensures the resulting path is within the base directory.
func SafeJoinPath(baseDir, userPath string) (string, error) {
	// Clean the user-provided path
	cleanPath := filepath.Clean(userPath)

	// Reject absolute paths
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("absolute paths not allowed: %s", cleanPath)
	}

	// Reject parent directory references
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("parent directory traversal not allowed: %s", cleanPath)
	}

	// Join with base directory
	fullPath := filepath.Join(baseDir, cleanPath)

	// Verify the resolved path is still within base directory
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Ensure path is within base directory (handles symlinks)
	if !strings.HasPrefix(absPath+string(filepath.Separator), absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path escape attempt detected: %s", userPath)
	}

	return fullPath, nil
}

type BackupService struct {
	DataDir       string
	BackupDir     string
	DatabaseName  string
	Cron          *cron.Cron
	restoreDBPath string
	createBackup  func() (string, error)
	cleanupOld    func(int) (int, error)
}

func checkpointSQLiteDatabase(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite database for checkpoint: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("checkpoint sqlite wal: %w", err)
	}

	return nil
}

func createSQLiteSnapshot(dbPath string) (snapshotPath string, cleanup func(), err error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return "", nil, fmt.Errorf("open sqlite database for snapshot: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tmpFile, err := os.CreateTemp("", "charon-backup-snapshot-*.db")
	if err != nil {
		return "", nil, fmt.Errorf("create sqlite snapshot file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if closeErr := tmpFile.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", nil, fmt.Errorf("close sqlite snapshot file: %w", closeErr)
	}

	if _, err := db.Exec("VACUUM INTO ?", tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", nil, fmt.Errorf("vacuum into sqlite snapshot: %w", err)
	}

	cleanup = func() {
		_ = os.Remove(tmpPath)
	}

	return tmpPath, cleanup, nil
}

type BackupFile struct {
	Filename string    `json:"filename"`
	Size     int64     `json:"size"`
	Time     time.Time `json:"time"`
}

func NewBackupService(cfg *config.Config) *BackupService {
	// Ensure backup directory exists
	backupDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "backups")
	// Use 0700 for backup directory (contains complete database dumps with sensitive data)
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		logger.Log().WithError(err).Error("Failed to create backup directory")
	}

	s := &BackupService{
		DataDir:      filepath.Dir(cfg.DatabasePath), // e.g. /app/data
		BackupDir:    backupDir,
		DatabaseName: filepath.Base(cfg.DatabasePath),
		Cron:         cron.New(),
	}
	s.createBackup = s.CreateBackup
	s.cleanupOld = s.CleanupOldBackups

	// Schedule daily backup at 3 AM
	_, err := s.Cron.AddFunc("0 3 * * *", s.RunScheduledBackup)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to schedule backup")
	}
	// Note: Cron scheduler must be explicitly started via Start() method

	return s
}

// DefaultBackupRetention is the number of backups to keep during cleanup.
const DefaultBackupRetention = 7

// Start starts the cron scheduler for automatic backups.
// Must be called after NewBackupService() to enable scheduled backups.
func (s *BackupService) Start() {
	s.Cron.Start()
	logger.Log().Info("Backup service cron scheduler started")
}

// Stop gracefully shuts down the cron scheduler.
// Waits for any running backup jobs to complete.
func (s *BackupService) Stop() {
	ctx := s.Cron.Stop()
	<-ctx.Done()
	logger.Log().Info("Backup service cron scheduler stopped")
}

func (s *BackupService) RunScheduledBackup() {
	logger.Log().Info("Starting scheduled backup")
	createBackup := s.CreateBackup
	if s.createBackup != nil {
		createBackup = s.createBackup
	}

	cleanupOld := s.CleanupOldBackups
	if s.cleanupOld != nil {
		cleanupOld = s.cleanupOld
	}

	if name, err := createBackup(); err != nil {
		logger.Log().WithError(err).Error("Scheduled backup failed")
	} else {
		logger.Log().WithField("backup", name).Info("Scheduled backup created")

		// Clean up old backups after successful creation
		if deleted, err := cleanupOld(DefaultBackupRetention); err != nil {
			logger.Log().WithError(err).Warn("Failed to cleanup old backups")
		} else if deleted > 0 {
			logger.Log().WithField("deleted_count", deleted).Info("Cleaned up old backups")
		}
	}
}

// CleanupOldBackups removes backups exceeding the retention count.
// Keeps the most recent 'keep' backups, deletes the rest.
// Returns the number of deleted backups.
func (s *BackupService) CleanupOldBackups(keep int) (int, error) {
	if keep < 1 {
		keep = 1 // Always keep at least one backup
	}

	backups, err := s.ListBackups()
	if err != nil {
		return 0, fmt.Errorf("list backups for cleanup: %w", err)
	}

	// ListBackups returns sorted newest first, so skip the first 'keep' entries
	if len(backups) <= keep {
		return 0, nil
	}

	deleted := 0
	toDelete := backups[keep:]

	for _, backup := range toDelete {
		if err := s.DeleteBackup(backup.Filename); err != nil {
			logger.Log().WithError(err).WithField("filename", util.SanitizeForLog(backup.Filename)).Warn("Failed to delete old backup")
			continue
		}
		deleted++
		logger.Log().WithField("filename", util.SanitizeForLog(backup.Filename)).Debug("Deleted old backup")
	}

	return deleted, nil
}

// GetLastBackupTime returns the timestamp of the most recent backup, or zero if none exist.
func (s *BackupService) GetLastBackupTime() (time.Time, error) {
	backups, err := s.ListBackups()
	if err != nil {
		return time.Time{}, err
	}

	if len(backups) == 0 {
		return time.Time{}, nil
	}

	// ListBackups returns sorted newest first
	return backups[0].Time, nil
}

// ListBackups returns all backup files sorted by time (newest first)
func (s *BackupService) ListBackups() ([]BackupFile, error) {
	entries, err := os.ReadDir(s.BackupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupFile{}, nil
		}
		return nil, err
	}

	var backups []BackupFile
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			backups = append(backups, BackupFile{
				Filename: entry.Name(),
				Size:     info.Size(),
				Time:     info.ModTime(),
			})
		}
	}

	// Sort newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Time.After(backups[j].Time)
	})

	return backups, nil
}

// CreateBackup creates a zip archive of the database and caddy data
func (s *BackupService) CreateBackup() (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("backup_%s.zip", timestamp)
	zipPath := filepath.Join(s.BackupDir, filename)

	outFile, err := os.Create(zipPath) // #nosec G304 -- Backup zip path controlled by app
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("failed to close backup file")
		}
	}()

	w := zip.NewWriter(outFile)

	// Files/Dirs to backup
	// 1. Database
	dbPath := filepath.Join(s.DataDir, s.DatabaseName)
	// Ensure DB exists before backing up
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return "", fmt.Errorf("database file not found: %s", dbPath)
	}
	backupSourcePath, cleanupBackupSource, err := createSQLiteSnapshot(dbPath)
	if err != nil {
		return "", fmt.Errorf("create sqlite snapshot before backup: %w", err)
	}
	defer cleanupBackupSource()

	if err := s.addToZip(w, backupSourcePath, s.DatabaseName); err != nil {
		return "", fmt.Errorf("backup db: %w", err)
	}

	// 2. Caddy Data (Certificates, etc)
	// We walk the 'caddy' subdirectory
	caddyDir := filepath.Join(s.DataDir, "caddy")
	if err := s.addDirToZip(w, caddyDir, "caddy"); err != nil {
		// It's possible caddy dir doesn't exist yet, which is fine
		logger.Log().WithError(err).Warn("Warning: could not backup caddy dir")
	}

	// Close zip writer and check for errors (important for zip integrity)
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize backup: %w", err)
	}

	return filename, nil
}

func (s *BackupService) addToZip(w *zip.Writer, srcPath, zipPath string) error {
	file, err := os.Open(srcPath) // #nosec G304 -- Source path controlled by app
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("failed to close file after adding to zip")
		}
	}()

	f, err := w.Create(zipPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, file)
	return err
}

func (s *BackupService) addDirToZip(w *zip.Writer, srcDir, zipBase string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		zipPath := filepath.Join(zipBase, relPath)
		return s.addToZip(w, path, zipPath)
	})
}

// DeleteBackup removes a backup file
func (s *BackupService) DeleteBackup(filename string) error {
	cleanName := filepath.Base(filename)
	if filename != cleanName {
		return fmt.Errorf("invalid filename: path traversal attempt detected")
	}
	path := filepath.Join(s.BackupDir, cleanName)
	if !strings.HasPrefix(path, filepath.Clean(s.BackupDir)) {
		return fmt.Errorf("invalid filename: path traversal attempt detected")
	}
	return os.Remove(path)
}

// GetBackupPath returns the full path to a backup file (for downloading)
func (s *BackupService) GetBackupPath(filename string) (string, error) {
	cleanName := filepath.Base(filename)
	if filename != cleanName {
		return "", fmt.Errorf("invalid filename: path traversal attempt detected")
	}
	path := filepath.Join(s.BackupDir, cleanName)
	if !strings.HasPrefix(path, filepath.Clean(s.BackupDir)) {
		return "", fmt.Errorf("invalid filename: path traversal attempt detected")
	}
	return path, nil
}

// RestoreBackup restores the database and caddy data from a zip archive
func (s *BackupService) RestoreBackup(filename string) error {
	cleanName := filepath.Base(filename)
	if filename != cleanName {
		return fmt.Errorf("invalid filename: path traversal attempt detected")
	}
	// 1. Verify backup exists
	srcPath := filepath.Join(s.BackupDir, cleanName)
	if !strings.HasPrefix(srcPath, filepath.Clean(s.BackupDir)) {
		return fmt.Errorf("invalid filename: path traversal attempt detected")
	}
	if _, err := os.Stat(srcPath); err != nil {
		return err
	}

	if restoreDBPath, err := s.extractDatabaseFromBackup(srcPath); err != nil {
		return fmt.Errorf("extract database from backup: %w", err)
	} else {
		if s.restoreDBPath != "" && s.restoreDBPath != restoreDBPath {
			_ = os.Remove(s.restoreDBPath)
		}
		s.restoreDBPath = restoreDBPath
	}

	// 2. Unzip to DataDir while skipping database files.
	// Database data is applied through controlled live rehydrate to avoid corrupting the active SQLite file.
	skipEntries := map[string]struct{}{
		s.DatabaseName:          {},
		s.DatabaseName + "-wal": {},
		s.DatabaseName + "-shm": {},
	}
	return s.unzipWithSkip(srcPath, s.DataDir, skipEntries)
}

// RehydrateLiveDatabase reloads the currently-open SQLite database from the restored DB file
// without requiring a process restart.
func (s *BackupService) RehydrateLiveDatabase(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is required")
	}

	restoredDBPath := filepath.Join(s.DataDir, s.DatabaseName)
	rehydrateSourcePath := restoredDBPath
	if s.restoreDBPath != "" {
		if _, err := os.Stat(s.restoreDBPath); err == nil {
			rehydrateSourcePath = s.restoreDBPath
		}
	}

	if _, err := os.Stat(rehydrateSourcePath); err != nil {
		return fmt.Errorf("restored database file missing: %w", err)
	}
	if rehydrateSourcePath == restoredDBPath {
		if err := checkpointSQLiteDatabase(restoredDBPath); err != nil {
			logger.Log().WithError(err).Warn("failed to checkpoint restored sqlite wal before live rehydrate")
		}
	}

	tempRestoreFile, err := os.CreateTemp("", "charon-restore-src-*.sqlite")
	if err != nil {
		return fmt.Errorf("create temporary restore database copy: %w", err)
	}
	tempRestorePath := tempRestoreFile.Name()
	if closeErr := tempRestoreFile.Close(); closeErr != nil {
		_ = os.Remove(tempRestorePath)
		return fmt.Errorf("close temporary restore database file: %w", closeErr)
	}
	defer func() {
		_ = os.Remove(tempRestorePath)
	}()

	sourceFile, err := os.Open(rehydrateSourcePath) // #nosec G304 -- rehydrate source path is internal controlled path
	if err != nil {
		return fmt.Errorf("open restored database file: %w", err)
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	destinationFile, err := os.OpenFile(tempRestorePath, os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- tempRestorePath is created by os.CreateTemp in this function
	if err != nil {
		return fmt.Errorf("open temporary restore database file: %w", err)
	}
	defer func() {
		_ = destinationFile.Close()
	}()

	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return fmt.Errorf("copy restored database to temporary file: %w", err)
	}

	if err := destinationFile.Sync(); err != nil {
		return fmt.Errorf("sync temporary restore database file: %w", err)
	}

	if err := db.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}

	if err := db.Exec("ATTACH DATABASE ? AS restore_src", tempRestorePath).Error; err != nil {
		logger.Log().WithError(err).Warn("failed to checkpoint restored sqlite wal before live rehydrate")
		_ = db.Exec("PRAGMA foreign_keys = ON")
		return fmt.Errorf("attach restored database: %w", err)
	}

	detached := false
	defer func() {
		if !detached {
			err := db.Exec("DETACH DATABASE restore_src").Error
			if err != nil {
				errMsg := strings.ToLower(err.Error())
				if !strings.Contains(errMsg, "locked") && !strings.Contains(errMsg, "busy") {
					logger.Log().WithError(err).Warn("failed to detach restore source database")
				}
			}
		}
		_ = db.Exec("PRAGMA foreign_keys = ON")
	}()

	var currentTables []string
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&currentTables).Error; err != nil {
		return fmt.Errorf("list current tables: %w", err)
	}

	restoredTableSet := map[string]struct{}{}
	var restoredTables []string
	if err := db.Raw(`SELECT name FROM restore_src.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&restoredTables).Error; err != nil {
		return fmt.Errorf("list restored tables: %w", err)
	}
	for _, tableName := range restoredTables {
		restoredTableSet[tableName] = struct{}{}
	}

	for _, tableName := range currentTables {
		quotedTable, err := quoteSQLiteIdentifier(tableName)
		if err != nil {
			return fmt.Errorf("quote table identifier: %w", err)
		}

		if err := db.Exec("DELETE FROM " + quotedTable).Error; err != nil {
			return fmt.Errorf("clear table %s: %w", tableName, err)
		}

		if _, exists := restoredTableSet[tableName]; !exists {
			continue
		}

		if err := db.Exec("INSERT INTO " + quotedTable + " SELECT * FROM restore_src." + quotedTable).Error; err != nil {
			return fmt.Errorf("copy table %s: %w", tableName, err)
		}
	}

	hasSQLiteSequence := false
	if err := db.Raw(`SELECT COUNT(*) > 0 FROM restore_src.sqlite_master WHERE type='table' AND name='sqlite_sequence'`).Scan(&hasSQLiteSequence).Error; err != nil {
		return fmt.Errorf("check sqlite_sequence presence: %w", err)
	}

	if hasSQLiteSequence {
		if err := db.Exec("DELETE FROM sqlite_sequence").Error; err != nil {
			return fmt.Errorf("clear sqlite_sequence: %w", err)
		}
		if err := db.Exec("INSERT INTO sqlite_sequence SELECT * FROM restore_src.sqlite_sequence").Error; err != nil {
			return fmt.Errorf("copy sqlite_sequence: %w", err)
		}
	}

	if err := db.Exec("DETACH DATABASE restore_src").Error; err != nil {
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "locked") && !strings.Contains(errMsg, "busy") {
			return fmt.Errorf("detach restored database: %w", err)
		}
	} else {
		detached = true
	}

	if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "locked") && !strings.Contains(errMsg, "busy") {
			return fmt.Errorf("checkpoint wal after rehydrate: %w", err)
		}
	}

	return nil
}

func (s *BackupService) extractDatabaseFromBackup(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open backup archive: %w", err)
	}
	defer func() {
		_ = r.Close()
	}()

	var dbEntry *zip.File
	var walEntry *zip.File
	var shmEntry *zip.File
	for _, file := range r.File {
		switch filepath.Clean(file.Name) {
		case s.DatabaseName:
			dbEntry = file
		case s.DatabaseName + "-wal":
			walEntry = file
		case s.DatabaseName + "-shm":
			shmEntry = file
		}
	}

	if dbEntry == nil {
		return "", fmt.Errorf("database entry %s not found in backup archive", s.DatabaseName)
	}

	tmpFile, err := os.CreateTemp("", "charon-restore-db-*.sqlite")
	if err != nil {
		return "", fmt.Errorf("create restore snapshot file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close restore snapshot file: %w", err)
	}

	extractToPath := func(file *zip.File, destinationPath string) error {
		outFile, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- destinationPath is derived from controlled temp file paths
		if err != nil {
			return fmt.Errorf("open destination file: %w", err)
		}
		defer func() {
			_ = outFile.Close()
		}()

		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open archive entry: %w", err)
		}
		defer func() {
			_ = rc.Close()
		}()

		const maxDecompressedSize = 100 * 1024 * 1024 // 100MB
		lr := &io.LimitedReader{R: rc, N: maxDecompressedSize}
		written, err := io.Copy(outFile, lr)
		if err != nil {
			return fmt.Errorf("copy archive entry: %w", err)
		}
		_ = written
		if lr.N == 0 {
			return fmt.Errorf("archive entry %s exceeded decompression limit (%d bytes), potential decompression bomb", file.Name, maxDecompressedSize)
		}
		if err := outFile.Sync(); err != nil {
			return fmt.Errorf("sync destination file: %w", err)
		}

		return nil
	}

	if err := extractToPath(dbEntry, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("extract database entry from backup archive: %w", err)
	}

	if walEntry != nil {
		walPath := tmpPath + "-wal"
		if err := extractToPath(walEntry, walPath); err != nil {
			_ = os.Remove(tmpPath)
			_ = os.Remove(walPath)
			return "", fmt.Errorf("extract wal entry from backup archive: %w", err)
		}

		if shmEntry != nil {
			shmPath := tmpPath + "-shm"
			if err := extractToPath(shmEntry, shmPath); err != nil {
				logger.Log().Warn("failed to extract sqlite shm entry from backup archive")
			}
		}

		if err := checkpointSQLiteDatabase(tmpPath); err != nil {
			_ = os.Remove(tmpPath)
			_ = os.Remove(walPath)
			_ = os.Remove(tmpPath + "-shm")
			return "", fmt.Errorf("checkpoint extracted sqlite wal: %w", err)
		}

		_ = os.Remove(walPath)
		_ = os.Remove(tmpPath + "-shm")
	}

	return tmpPath, nil
}

func (s *BackupService) unzipWithSkip(src, dest string, skipEntries map[string]struct{}) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close zip reader")
		}
	}()

	for _, f := range r.File {
		if skipEntries != nil {
			if _, skip := skipEntries[filepath.Clean(f.Name)]; skip {
				continue
			}
		}

		// Use SafeJoinPath to prevent directory traversal attacks
		fpath, err := SafeJoinPath(dest, f.Name)
		if err != nil {
			return fmt.Errorf("invalid file path in archive: %w", err)
		}

		if f.FileInfo().IsDir() {
			// Use 0700 for extracted directories (private data workspace)
			_ = os.MkdirAll(fpath, 0o700)
			continue
		}

		// Use 0700 for parent directories
		if mkdirErr := os.MkdirAll(filepath.Dir(fpath), 0o700); mkdirErr != nil {
			return mkdirErr
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode()) // #nosec G304 -- File path from validated backup
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			if closeErr := outFile.Close(); closeErr != nil {
				logger.Log().WithError(closeErr).Warn("failed to close temporary output file after f.Open() error")
			}
			return err
		}

		// Limit decompressed size to prevent decompression bombs (100MB limit).
		// Use max+1 so lr.N == 0 only when a byte beyond the limit was consumed,
		// avoiding a false positive for files that are exactly maxDecompressedSize.
		const maxDecompressedSize = 100 * 1024 * 1024 // 100MB
		lr := &io.LimitedReader{R: rc, N: maxDecompressedSize + 1}
		_, err = io.Copy(outFile, lr)

		if err == nil && lr.N == 0 {
			err = fmt.Errorf("file %s exceeded decompression limit (%d bytes), potential decompression bomb", f.Name, maxDecompressedSize)
		}

		// Check for close errors on writable file
		if closeErr := outFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if closeErr := rc.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close reader")
		}

		if err != nil {
			return err
		}
	}
	return nil
}

// GetAvailableSpace returns the available disk space in bytes for the backup directory
func (s *BackupService) GetAvailableSpace() (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.BackupDir, &stat); err != nil {
		return 0, fmt.Errorf("failed to get disk space: %w", err)
	}

	// Safe conversion with overflow protection (gosec G115)
	bsize := stat.Bsize
	bavail := stat.Bavail

	// Check for invalid filesystem (negative block size)
	if bsize < 0 {
		return 0, fmt.Errorf("invalid block size: %d", bsize)
	}

	// Check if bavail exceeds max int64 before conversion
	if bavail > uint64(math.MaxInt64) {
		return math.MaxInt64, nil
	}

	// Safe to convert now
	availBlocks := int64(bavail)
	blockSize := int64(bsize)

	// Check for multiplication overflow
	if availBlocks > 0 && blockSize > math.MaxInt64/availBlocks {
		return math.MaxInt64, nil
	}

	return availBlocks * blockSize, nil
}
