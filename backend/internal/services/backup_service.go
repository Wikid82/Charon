package services

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/Wikid82/charon/backend/internal/version"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	_ "github.com/mattn/go-sqlite3"
)

// Sentinel errors surfaced by CreateBackupWithOptions / RestoreBackupSafe so
// handlers can map them to specific HTTP status codes / error_codes (spec
// §3.3, §3.10) via errors.Is rather than fragile string matching.
var (
	// ErrInsufficientSpace is returned when GetAvailableSpace() is below 2x
	// the current database size (spec §3.10 disk-full precheck).
	ErrInsufficientSpace = errors.New("insufficient disk space for backup")
	// ErrBackupInProgress is returned when a create/restore is already
	// running and BackupService.mu could not be acquired immediately (spec
	// §3.10 concurrency guard).
	ErrBackupInProgress = errors.New("another backup or restore is in progress")
	// ErrPassphraseRequired is returned by RestoreBackupSafe when the
	// archive is age-encrypted but no passphrase was supplied.
	ErrPassphraseRequired = errors.New("passphrase is required to restore this backup")
	// ErrPassphraseInvalid is returned when age decryption fails, most
	// commonly because the supplied passphrase is wrong.
	ErrPassphraseInvalid = errors.New("passphrase is invalid for this backup")
	// ErrBackupValidationFailed covers manifest/checksum/format validation
	// failures (spec §3.5 step V) — nothing is mutated when this is
	// returned.
	ErrBackupValidationFailed = errors.New("backup validation failed")
	// ErrNewerBackupFormat is returned when a manifest declares a
	// format_version greater than the version this build understands.
	ErrNewerBackupFormat = errors.New("backup created by a newer Charon version")
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

	// CrowdSecDir is the source directory backed up into the archive's
	// crowdsec/** entries (spec §3.2). Wiring into CreateBackup happens in
	// Commit 3; this commit only threads the field through the constructor.
	CrowdSecDir string

	// db and encryption are threaded through so Commit 3's manifest
	// (EncryptionKeyRequired), settings-driven reschedule, and
	// remote-upload/BackupRecord persistence have what they need without a
	// second constructor-signature change. encryption is nilable — backups
	// are unencrypted by default and when CHARON_ENCRYPTION_KEY is unset.
	db         *gorm.DB
	encryption *crypto.EncryptionService

	// scheduleEntry identifies the cron.Cron entry created at construction so
	// a future Reschedule (Commit 3) can cron.Remove it before re-adding.
	scheduleEntry cron.EntryID

	// mu serializes create/restore operations (spec §3.10 concurrency guard).
	mu sync.Mutex

	// uploadCtx/uploadCancel/uploadWG track remote-upload goroutines so
	// Stop() can cancel in-flight uploads and wait for them rather than
	// orphaning them at shutdown (spec §3.7).
	uploadCtx    context.Context
	uploadCancel context.CancelFunc
	uploadWG     sync.WaitGroup

	// remoteUploadHook is invoked (in a tracked goroutine) after a backup is
	// successfully created, so enabled remote targets receive a copy. Wired
	// by routes.go via SetRemoteUploadHook; nil is a no-op (many unit tests
	// construct a BackupService with no remote storage wiring at all).
	remoteUploadHook RemoteUploadHook

	// caddyReloader triggers a Caddy config reload after a restore (spec
	// §3.5 step R1). Nilable — many unit tests construct a BackupService
	// without a Caddy manager, in which case restore simply skips R1 and
	// reports caddy_reloaded=false.
	caddyReloader CaddyReloader
}

// RemoteUploadHook is invoked after CreateBackupWithOptions successfully
// persists a BackupRecord, so enabled remote storage targets can receive a
// copy of the new archive (spec §3.7). Implementations must respect ctx
// cancellation (BackupService.Stop cancels uploadCtx at shutdown).
type RemoteUploadHook func(ctx context.Context, record *models.BackupRecord)

// CaddyReloader is the minimal interface RestoreBackupSafe needs from the
// Caddy manager (spec §3.5 step R1) — satisfied by *caddy.Manager's
// ApplyConfig without importing the caddy package here (avoids an import
// cycle: caddy already imports services in some builds).
type CaddyReloader interface {
	ApplyConfig(ctx context.Context) error
}

// SetRemoteUploadHook wires the remote-storage upload trigger. Called once
// from routes.go after both BackupService and the remote storage
// orchestration are constructed.
func (s *BackupService) SetRemoteUploadHook(hook RemoteUploadHook) {
	s.remoteUploadHook = hook
}

// SetCaddyReloader wires the post-restore Caddy reload hook (spec §3.5 R1).
func (s *BackupService) SetCaddyReloader(reloader CaddyReloader) {
	s.caddyReloader = reloader
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

// NewBackupService constructs a BackupService. db and enc are nilable: many
// unit tests and standalone tooling construct a BackupService without a live
// database or without CHARON_ENCRYPTION_KEY configured, and those remain
// supported — behavior identical to v1 in that case. When db is non-nil, any
// legacy backup.interval/backup.retention Setting rows are translated to the
// canonical backup.* keys and removed (see migrateLegacyBackupSettings); this
// is best-effort and logged, never fatal to construction.
func NewBackupService(cfg *config.Config, db *gorm.DB, enc *crypto.EncryptionService) *BackupService {
	// Ensure backup directory exists
	backupDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "backups")
	// Use 0700 for backup directory (contains complete database dumps with sensitive data)
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		logger.Log().WithError(err).Error("Failed to create backup directory")
	}

	uploadCtx, uploadCancel := context.WithCancel(context.Background())

	s := &BackupService{
		DataDir:      filepath.Dir(cfg.DatabasePath), // e.g. /app/data
		BackupDir:    backupDir,
		DatabaseName: filepath.Base(cfg.DatabasePath),
		CrowdSecDir:  cfg.Security.CrowdSecConfigDir,
		Cron:         cron.New(),
		db:           db,
		encryption:   enc,
		uploadCtx:    uploadCtx,
		uploadCancel: uploadCancel,
	}
	s.createBackup = s.CreateBackup
	s.cleanupOld = s.CleanupOldBackups

	// defaultScheduleCron is used until/unless a persisted backup.schedule_cron
	// setting is found below (spec §3.4.4).
	scheduleCron := defaultScheduleCron

	if db != nil {
		if migrateErr := migrateLegacyBackupSettings(db); migrateErr != nil {
			logger.Log().WithError(migrateErr).Warn("Failed to migrate legacy backup settings")
		}

		if persisted, ok := readBackupSettingString(db, SettingKeyBackupScheduleCron); ok && strings.TrimSpace(persisted) != "" {
			if _, parseErr := cron.ParseStandard(persisted); parseErr == nil {
				scheduleCron = persisted
			} else {
				logger.Log().WithError(parseErr).WithField("cron", util.SanitizeForLog(persisted)).
					Warn("Ignoring invalid persisted backup.schedule_cron, using default")
			}
		}

		if reconcileErr := reconcileStuckUploadingCopies(db); reconcileErr != nil {
			logger.Log().WithError(reconcileErr).Warn("Failed to reconcile stuck remote upload copies")
		}
	}

	entryID, err := s.Cron.AddFunc(scheduleCron, s.RunScheduledBackup)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to schedule backup")
	}
	s.scheduleEntry = entryID
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

	if s.uploadCancel != nil {
		s.uploadCancel()
	}
	s.uploadWG.Wait()

	logger.Log().Info("Backup service cron scheduler stopped")
}

// Reschedule replaces the cron entry driving RunScheduledBackup with one
// using cronSpec, validated via cron.ParseStandard (spec §3.2/§3.4.4). It is
// safe to call whether or not the scheduler has been Start()ed yet.
func (s *BackupService) Reschedule(cronSpec string) error {
	if _, err := cron.ParseStandard(cronSpec); err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", cronSpec, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.scheduleEntry != 0 {
		s.Cron.Remove(s.scheduleEntry)
	}

	entryID, err := s.Cron.AddFunc(cronSpec, s.RunScheduledBackup)
	if err != nil {
		return fmt.Errorf("schedule backup cron %q: %w", cronSpec, err)
	}
	s.scheduleEntry = entryID
	return nil
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

	// pre_restore safety backups are excluded from scheduled retention
	// pruning (spec §3.5 S1, §3.10 Open Question 6 resolution) — they never
	// auto-expire. Without a live DB there is no way to know a filename's
	// BackupRecord.Type, so this filtering is a no-op in that case (matches
	// pre-Issue-#32 behavior for tests that construct BackupService without
	// a database).
	prunable := backups
	if s.db != nil {
		prunable = make([]BackupFile, 0, len(backups))
		for _, b := range backups {
			if s.isPreRestoreBackup(b.Filename) {
				continue
			}
			prunable = append(prunable, b)
		}
	}

	// ListBackups returns sorted newest first, so skip the first 'keep' entries
	if len(prunable) <= keep {
		return 0, nil
	}

	deleted := 0
	toDelete := prunable[keep:]

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

// isPreRestoreBackup reports whether filename's BackupRecord (if any) has
// Type "pre_restore", so CleanupOldBackups can exclude it from scheduled
// retention pruning (spec §3.5 S1). Any lookup error is treated as "no" —
// this is a best-effort exclusion, not a correctness-critical check.
func (s *BackupService) isPreRestoreBackup(filename string) bool {
	if s.db == nil {
		return false
	}
	var record models.BackupRecord
	if err := s.db.Where("filename = ?", filename).First(&record).Error; err != nil {
		return false
	}
	return record.Type == "pre_restore"
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
		if !entry.IsDir() && isBackupArchiveFilename(entry.Name()) {
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

// isBackupArchiveFilename reports whether name is a backup archive Charon
// creates/manages: unencrypted (.zip) or age-encrypted (.zip.age). Used by
// ListBackups, CleanupOldBackups (via ListBackups), and GetLastBackupTime
// (via ListBackups) so encrypted backups are listed, pruned, and reflected
// in DB-health status identically to unencrypted ones (spec §3.2 — this was
// a real gap in v1, not just a docs correction).
func isBackupArchiveFilename(name string) bool {
	return strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".zip.age")
}

// BackupOptions configures a single CreateBackupWithOptions call (spec §3.2).
type BackupOptions struct {
	// Type identifies why the backup is being created:
	// manual|scheduled|pre_restore|uploaded. Defaults to "manual" if empty.
	Type string
	// Encrypt requests age/scrypt passphrase encryption of the finished
	// archive (spec §3.6). Requires Passphrase to be non-empty.
	Encrypt bool
	// Passphrase is used only in memory for this call and is never logged
	// or persisted in plaintext (spec §3.6, §3.9).
	Passphrase string
}

// CreateBackup creates an unencrypted, manual-type format-v2 archive of the
// database, caddy/, and crowdsec/ directories, and returns just its filename
// — the original v1 signature, kept because the certificate handler's
// BackupServiceInterface (certificate_handler.go) and cron test seams depend
// on it (spec §3.2). It is now a thin wrapper over CreateBackupWithOptions.
func (s *BackupService) CreateBackup() (string, error) {
	record, err := s.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	if err != nil {
		return "", err
	}
	return record.Filename, nil
}

// CreateBackupWithOptions creates a format-v2 archive (charon.db + caddy/**
// + crowdsec/** + manifest.json written last, per spec §3.2), optionally
// encrypts it with age/scrypt (spec §3.6), persists a BackupRecord, and —
// once the local file is safely on disk — fires the remote-upload hook in a
// tracked goroutine (spec §3.7). Serialized against RestoreBackupSafe /
// RestoreBackup via mu so a create cannot race a restore touching the same
// DataDir (spec §3.10).
func (s *BackupService) CreateBackupWithOptions(opts BackupOptions) (*models.BackupRecord, error) {
	if opts.Type == "" {
		opts.Type = "manual"
	}

	if !s.mu.TryLock() {
		return nil, ErrBackupInProgress
	}
	record, err := s.createBackupLocked(opts)
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}

	if s.remoteUploadHook != nil && record != nil {
		s.uploadWG.Add(1)
		go func() {
			defer s.uploadWG.Done()
			s.remoteUploadHook(s.uploadCtx, record)
		}()
	}

	return record, nil
}

func (s *BackupService) createBackupLocked(opts BackupOptions) (*models.BackupRecord, error) {
	if opts.Encrypt && strings.TrimSpace(opts.Passphrase) == "" {
		return nil, fmt.Errorf("passphrase is required to encrypt backup")
	}

	dbPath := filepath.Join(s.DataDir, s.DatabaseName)
	dbInfo, statErr := os.Stat(dbPath)
	if os.IsNotExist(statErr) {
		return nil, fmt.Errorf("database file not found: %s", dbPath)
	}

	if dbInfo != nil {
		if spaceErr := s.checkSufficientSpaceLocked(dbInfo.Size()); spaceErr != nil {
			return nil, spaceErr
		}
	}

	if err := os.MkdirAll(s.BackupDir, 0o700); err != nil {
		return nil, fmt.Errorf("ensure backup directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	baseFilename := fmt.Sprintf("backup_%s.zip", timestamp)
	zipPath := filepath.Join(s.BackupDir, baseFilename)

	manifest := &BackupManifest{
		FormatVersion:         2,
		CreatedAt:             time.Now().UTC(),
		AppVersion:            version.Version,
		BackupType:            opts.Type,
		DatabaseName:          s.DatabaseName,
		EncryptionKeyRequired: s.computeEncryptionKeyRequired(),
	}

	if err := s.writeV2Archive(zipPath, dbPath, manifest); err != nil {
		_ = os.Remove(zipPath)
		return nil, err
	}

	finalPath := zipPath
	finalFilename := baseFilename
	encrypted := false

	if opts.Encrypt {
		encPath := zipPath + ".age"
		if err := encryptArchiveWithPassphrase(zipPath, encPath, opts.Passphrase); err != nil {
			_ = os.Remove(zipPath)
			_ = os.Remove(encPath)
			return nil, fmt.Errorf("encrypt backup archive: %w", err)
		}
		_ = os.Remove(zipPath)
		finalPath = encPath
		finalFilename = baseFilename + ".age"
		encrypted = true
	}

	sum, size, err := sha256File(finalPath)
	if err != nil {
		return nil, fmt.Errorf("checksum final backup archive: %w", err)
	}

	record := &models.BackupRecord{
		Filename:      finalFilename,
		Size:          size,
		SHA256:        sum,
		Type:          opts.Type,
		FormatVersion: 2,
		Encrypted:     encrypted,
		AppVersion:    version.Version,
		Status:        "completed",
	}

	if s.db != nil {
		if createErr := s.db.Create(record).Error; createErr != nil {
			logger.Log().WithError(createErr).WithField("filename", util.SanitizeForLog(finalFilename)).
				Warn("Failed to persist backup record")
		}
	}

	return record, nil
}

// checkSufficientSpaceLocked enforces the disk-full precheck (spec §3.10):
// available space in BackupDir must be at least 2x the current database
// size. Callers must hold s.mu.
func (s *BackupService) checkSufficientSpaceLocked(currentDBSize int64) error {
	available, err := s.GetAvailableSpace()
	if err != nil {
		// Best-effort: if we can't determine free space, don't block the
		// backup on it — GetAvailableSpace already covers the common
		// failure modes (missing dir, bad statfs) with its own error.
		logger.Log().WithError(err).Warn("Could not determine available disk space before backup; proceeding")
		return nil
	}

	needed := currentDBSize * 2
	if available < needed {
		return fmt.Errorf("%w: need at least %d bytes, have %d available", ErrInsufficientSpace, needed, available)
	}
	return nil
}

// writeV2Archive builds the format-v2 zip at zipPath: the SQLite snapshot of
// dbSourcePath, caddy/**, crowdsec/**, and finally manifest.json (spec
// §3.2 — manifest MUST be written last so every preceding entry's checksum
// is already known via the single-pass checksumWriter).
func (s *BackupService) writeV2Archive(zipPath, dbSourcePath string, manifest *BackupManifest) error {
	outFile, err := os.OpenFile(zipPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- Backup zip path controlled by app
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("failed to close backup file")
		}
	}()

	w := zip.NewWriter(outFile)

	backupSourcePath, cleanupBackupSource, err := createSQLiteSnapshot(dbSourcePath)
	if err != nil {
		return fmt.Errorf("create sqlite snapshot before backup: %w", err)
	}
	defer cleanupBackupSource()

	if zipErr := s.addToZipTracked(w, backupSourcePath, s.DatabaseName, &manifest.Contents); zipErr != nil {
		return fmt.Errorf("backup db: %w", zipErr)
	}

	caddyDir := filepath.Join(s.DataDir, "caddy")
	if caddyErr := s.addDirToZipTracked(w, caddyDir, "caddy", &manifest.Contents); caddyErr != nil {
		// It's possible caddy dir doesn't exist yet, which is fine
		logger.Log().WithError(caddyErr).Warn("Warning: could not backup caddy dir")
	}

	if strings.TrimSpace(s.CrowdSecDir) != "" {
		if crowdsecErr := s.addDirToZipTracked(w, s.CrowdSecDir, "crowdsec", &manifest.Contents); crowdsecErr != nil {
			// Tolerate absence exactly like the caddy dir above (spec §3.2).
			logger.Log().WithError(crowdsecErr).Warn("Warning: could not backup crowdsec dir")
		}
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal backup manifest: %w", err)
	}

	manifestEntry, err := w.Create("manifest.json")
	if err != nil {
		return fmt.Errorf("create manifest entry: %w", err)
	}
	if _, err := manifestEntry.Write(manifestBytes); err != nil {
		return fmt.Errorf("write manifest entry: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to finalize backup: %w", err)
	}

	return nil
}

// computeEncryptionKeyRequired reports whether any rows exist in tables
// whose columns are encrypted with CHARON_ENCRYPTION_KEY, so a restore-time
// warning can tell the operator that the target host needs the same key
// (spec §3.2, §3.6). A nil db (many unit tests, and any deployment without a
// live database wired into BackupService) is treated as "no" rather than an
// error — there is nothing to check.
func (s *BackupService) computeEncryptionKeyRequired() bool {
	if s.db == nil {
		return false
	}

	for _, table := range []string{"dns_provider_credentials", "tunnel_configs", "remote_storage_targets"} {
		var count int64
		if err := s.db.Table(table).Count(&count).Error; err != nil {
			// Table may not exist yet on a fresh/partially migrated DB;
			// that's not an error condition for this best-effort check.
			continue
		}
		if count > 0 {
			return true
		}
	}
	return false
}

// sha256File computes the SHA-256 checksum and size of the file at path
// without loading it fully into memory.
func sha256File(path string) (sum string, size int64, err error) {
	f, err := os.Open(path) // #nosec G304 -- path is a server-controlled backup archive path
	if err != nil {
		return "", 0, fmt.Errorf("open file for checksum: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	h := sha256.New()
	written, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("read file for checksum: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), written, nil
}

// addToZip copies srcPath into the archive at zipPath without manifest
// checksum tracking. Kept for callers/tests that don't need a manifest
// (e.g. legacy in-place archive helpers) — CreateBackupWithOptions itself
// uses addToZipTracked so it can populate the v2 manifest in the same pass.
func (s *BackupService) addToZip(w *zip.Writer, srcPath, zipPath string) error {
	return s.addToZipTracked(w, srcPath, zipPath, nil)
}

// addToZipTracked copies srcPath into the zip archive at zipPath. If
// entries is non-nil, a ManifestEntry (streaming SHA-256 + size, via
// io.MultiWriter under the hood) is appended for this file — this is the
// single-pass checksum computation required by spec §3.2. Returns nil
// (graceful skip) when srcPath is absent, matching the historical addToZip
// behavior.
func (s *BackupService) addToZipTracked(w *zip.Writer, srcPath, zipPath string, entries *[]ManifestEntry) error {
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

	archivePath := filepath.ToSlash(zipPath)

	if entries == nil {
		if _, copyErr := io.Copy(f, file); copyErr != nil {
			return copyErr
		}
		return nil
	}

	cw := newChecksumWriter(f)
	if _, copyErr := io.Copy(cw, file); copyErr != nil {
		return copyErr
	}
	*entries = append(*entries, cw.Entry(archivePath))

	return nil
}

func (s *BackupService) addDirToZip(w *zip.Writer, srcDir, zipBase string) error {
	return s.addDirToZipTracked(w, srcDir, zipBase, nil)
}

// addDirToZipTracked walks srcDir, adding every regular file into the
// archive under zipBase, tracking manifest entries when entries is non-nil
// (see addToZipTracked).
func (s *BackupService) addDirToZipTracked(w *zip.Writer, srcDir, zipBase string, entries *[]ManifestEntry) error {
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
		return s.addToZipTracked(w, path, zipPath, entries)
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

// RestoreBackup restores the database and caddy data from a zip archive.
// Serialized against CreateBackup via mu (see CreateBackup's comment).
func (s *BackupService) RestoreBackup(filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

// Extraction hardening caps (spec §3.9/§3.10). These are package-level vars
// rather than consts so tests can shrink them to exercise the "exceeded"
// path without allocating multi-gigabyte fixtures; production code paths
// never mutate them.
var (
	// legacyPerEntryDecompressionCap is the flat per-entry decompression cap
	// used when no manifest-declared size is available for an entry (v1/
	// legacy archives, or any entry a v2 manifest doesn't mention). Raised
	// from v1's 100MB to 2GiB so a charon.db larger than 100MB (plausible
	// given RequestLog growth) is no longer permanently unrestorable.
	legacyPerEntryDecompressionCap int64 = 2 * 1024 * 1024 * 1024
	// manifestEntrySizeSlack is added on top of a v2 ManifestEntry's
	// checksum-verified declared Size to bound that entry's extraction, per
	// spec §3.5 V4 / §3.9.
	manifestEntrySizeSlack int64 = 64 * 1024
	// maxTotalExtractedSize bounds the sum of all bytes extracted from a
	// single archive, independent of any manifest, so a crafted manifest
	// cannot bypass it (spec §3.9).
	maxTotalExtractedSize int64 = 4 * 1024 * 1024 * 1024
	// maxExtractedEntryCount bounds the number of entries an archive may
	// contain, independent of any manifest (spec §3.9).
	maxExtractedEntryCount = 10000
)

// entryDecompressionCap returns the maximum number of bytes that may be
// decompressed for archive entry name. When sizes is non-nil and contains a
// checksum-verified declared size for name (v2 manifest path), the cap is
// that size plus manifestEntrySizeSlack; otherwise the flat legacy cap
// applies (v1/legacy archives, or entries a v2 manifest doesn't mention).
func entryDecompressionCap(name string, sizes map[string]int64) int64 {
	if sizes != nil {
		if declared, ok := sizes[name]; ok {
			return declared + manifestEntrySizeSlack
		}
	}
	return legacyPerEntryDecompressionCap
}

// extractDatabaseFromBackup extracts the database (+ -wal/-shm siblings, if
// present) from zipPath to a temporary file, using the flat legacy
// decompression cap for every entry (no manifest available). It is kept as
// the zero-manifest entry point for the legacy RestoreBackup path and
// existing tests; RestoreBackupSafe uses
// extractDatabaseFromBackupWithSizes for the v2, manifest-scaled path.
func (s *BackupService) extractDatabaseFromBackup(zipPath string) (string, error) {
	return s.extractDatabaseFromBackupWithSizes(zipPath, nil)
}

// extractDatabaseFromBackupWithSizes is extractDatabaseFromBackup, but
// entries named in sizes (archive-relative path -> checksum-verified
// declared size, from a v2 manifest) get a per-entry cap scaled to that
// declared size + slack instead of the flat legacy cap (spec §3.5 V4-V5,
// §3.9).
func (s *BackupService) extractDatabaseFromBackupWithSizes(zipPath string, sizes map[string]int64) (string, error) {
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

		maxDecompressedSize := entryDecompressionCap(filepath.Clean(file.Name), sizes)
		lr := &io.LimitedReader{R: rc, N: maxDecompressedSize + 1}
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

// unzipWithSkip extracts src into dest, skipping entries named in
// skipEntries, using the flat legacy per-entry decompression cap (no
// manifest available). Kept as the zero-manifest entry point for the legacy
// RestoreBackup path and existing tests; RestoreBackupSafe's apply step uses
// unzipWithSkipManifest for the v2, manifest-scaled path.
func (s *BackupService) unzipWithSkip(src, dest string, skipEntries map[string]struct{}) error {
	return s.extractZip(src, dest, skipEntries, nil)
}

// unzipWithSkipManifest is unzipWithSkip, but entries named in sizes
// (archive-relative path -> checksum-verified declared size, from a v2
// manifest) get a per-entry cap scaled to that declared size + slack
// instead of the flat legacy cap (spec §3.5 A1, §3.9).
func (s *BackupService) unzipWithSkipManifest(src, dest string, skipEntries map[string]struct{}, sizes map[string]int64) error {
	return s.extractZip(src, dest, skipEntries, sizes)
}

// extractZip is the hardened extraction core shared by unzipWithSkip and
// unzipWithSkipManifest. Hardening applied (spec §3.9, v1 gaps):
//   - symlink entries (os.ModeSymlink) are rejected before extraction
//   - archive-supplied permission bits are ignored entirely: every
//     extracted regular file is forced to 0o600, every directory to 0o700
//   - per-entry decompression is bounded by entryDecompressionCap (scaled
//     to the manifest-declared size + slack for v2, flat legacy cap
//     otherwise)
//   - total extracted bytes across the whole archive is capped at
//     maxTotalExtractedSize, and entry count at maxExtractedEntryCount,
//     both independent of any manifest so a crafted manifest cannot bypass
//     them
func (s *BackupService) extractZip(src, dest string, skipEntries map[string]struct{}, sizes map[string]int64) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close zip reader")
		}
	}()

	if len(r.File) > maxExtractedEntryCount {
		return fmt.Errorf("archive contains %d entries, exceeding the maximum of %d", len(r.File), maxExtractedEntryCount)
	}

	var totalExtracted int64

	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		if skipEntries != nil {
			if _, skip := skipEntries[cleanName]; skip {
				continue
			}
		}

		// Reject symlink entries before extraction (v1 gap fix) — a
		// crafted archive must not be able to plant a symlink that later
		// extraction steps, or the running application, would follow.
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive entry %s is a symlink, which is not allowed", f.Name)
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

		// Archive-supplied permission bits are ignored entirely (v1 gap
		// fix) — a crafted/uploaded archive could otherwise claim
		// world-writable or setuid/setgid bits via f.Mode().
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- File path from validated backup
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

		// Per-entry cap, further bounded by whatever total-size budget
		// remains, so a long run of just-under-cap entries can't blow past
		// maxTotalExtractedSize before the running total is even checked.
		remainingBudget := maxTotalExtractedSize - totalExtracted
		if remainingBudget < 0 {
			remainingBudget = 0
		}
		entryCap := entryDecompressionCap(cleanName, sizes)
		limit := entryCap
		if remainingBudget < limit {
			limit = remainingBudget
		}

		// Use limit+1 so lr.N == 0 only when a byte beyond the limit was
		// consumed, avoiding a false positive for files exactly at the cap.
		lr := &io.LimitedReader{R: rc, N: limit + 1}
		written, err := io.Copy(outFile, lr)
		totalExtracted += written

		if err == nil && lr.N == 0 {
			err = fmt.Errorf("file %s exceeded decompression limit (%d bytes), potential decompression bomb", f.Name, entryCap)
		}
		if err == nil && totalExtracted > maxTotalExtractedSize {
			err = fmt.Errorf("archive exceeds total extracted size limit (%d bytes)", maxTotalExtractedSize)
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
