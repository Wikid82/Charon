package services

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/Wikid82/charon/backend/internal/version"
)

// RestoreResult is the outcome of RestoreBackupSafe, serialized directly by
// the handler as the response body (spec §3.3.1).
type RestoreResult struct {
	Message string `json:"message"`
	// RestartRequired is true whenever live rehydrate could not be applied
	// (whether or not a durable pending-restore file was written).
	RestartRequired bool `json:"restart_required"`
	// DatabaseSwapPending is true only when a durable .pending-restore file
	// was written for the next process boot to consume (spec §3.5 F3) — it
	// always accompanies RestartRequired.
	DatabaseSwapPending  bool   `json:"database_swap_pending"`
	LiveRehydrateApplied bool   `json:"live_rehydrate_applied"`
	CaddyReloaded        bool   `json:"caddy_reloaded"`
	PreRestoreBackup     string `json:"pre_restore_backup,omitempty"`
	LegacyFormat         bool   `json:"legacy_format"`
}

// BackupValidation is the response of the dry-run validate endpoint (spec
// §3.3.2).
type BackupValidation struct {
	Valid                 bool      `json:"valid"`
	FormatVersion         int       `json:"format_version"`
	LegacyFormat          bool      `json:"legacy_format"`
	AppVersion            string    `json:"app_version,omitempty"`
	CreatedAt             time.Time `json:"created_at,omitempty"`
	DatabaseIntegrity     string    `json:"database_integrity"`
	EncryptionKeyRequired bool      `json:"encryption_key_required"`
	Warnings              []string  `json:"warnings,omitempty"`
}

// validatedArchive carries the intermediate state produced by validating a
// backup archive (spec §3.5 steps V1-V6), shared by RestoreBackupSafe (which
// continues on to S/A/R) and ValidateBackup (a pure dry run).
type validatedArchive struct {
	cleanName     string
	archivePath   string // decrypted temp file when the source is .age, else the original path
	cleanupPaths  []string
	manifest      *BackupManifest
	legacyFormat  bool
	sizes         map[string]int64
	restoreDBPath string
}

func (v *validatedArchive) cleanup() {
	for _, p := range v.cleanupPaths {
		if p == "" {
			continue
		}
		_ = os.Remove(p)
	}
}

func (v *validatedArchive) dropFromCleanup(path string) {
	filtered := v.cleanupPaths[:0]
	for _, p := range v.cleanupPaths {
		if p != path {
			filtered = append(filtered, p)
		}
	}
	v.cleanupPaths = filtered
}

// validateBackupArchive performs spec §3.5 steps V1-V6: path sanitation,
// age-decryption (if applicable), manifest parsing (rejecting format
// versions newer than this build understands), per-entry checksum
// verification for v2 archives, DB extraction, and a PRAGMA
// integrity_check + sanity-table check on the extracted database. Nothing
// in DataDir/the live database is touched by this function — that's the
// entire point (F1: a validation failure must never require a rollback,
// because nothing was mutated yet).
func (s *BackupService) validateBackupArchive(filename, passphrase string) (*validatedArchive, error) {
	cleanName := filepath.Base(filename)
	if filename != cleanName {
		return nil, fmt.Errorf("invalid filename: path traversal attempt detected")
	}
	srcPath := filepath.Join(s.BackupDir, cleanName)
	if !strings.HasPrefix(srcPath, filepath.Clean(s.BackupDir)) {
		return nil, fmt.Errorf("invalid filename: path traversal attempt detected")
	}
	if _, err := os.Stat(srcPath); err != nil {
		return nil, err
	}

	result := &validatedArchive{cleanName: cleanName, archivePath: srcPath}

	// V2: age-decrypt to a temp file when the archive is encrypted.
	if strings.HasSuffix(cleanName, ".age") {
		if strings.TrimSpace(passphrase) == "" {
			return nil, ErrPassphraseRequired
		}

		tmp, err := os.CreateTemp("", "charon-restore-decrypted-*.zip")
		if err != nil {
			return nil, fmt.Errorf("create temp file for decrypted archive: %w", err)
		}
		tmpPath := tmp.Name()
		if closeErr := tmp.Close(); closeErr != nil {
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("close temp file for decrypted archive: %w", closeErr)
		}

		if err := decryptArchiveWithPassphrase(srcPath, tmpPath, passphrase); err != nil {
			_ = os.Remove(tmpPath)
			return nil, err
		}

		result.archivePath = tmpPath
		result.cleanupPaths = append(result.cleanupPaths, tmpPath)
	}

	// V3: locate + parse manifest.json (absent => legacy v1 format).
	manifest, legacy, err := readBackupManifest(result.archivePath)
	if err != nil {
		result.cleanup()
		return nil, fmt.Errorf("%w: %s", ErrBackupValidationFailed, err.Error())
	}
	if manifest != nil && manifest.FormatVersion > 2 {
		result.cleanup()
		return nil, ErrNewerBackupFormat
	}
	result.manifest = manifest
	result.legacyFormat = legacy

	if manifest != nil {
		result.sizes = make(map[string]int64, len(manifest.Contents))
		for _, entry := range manifest.Contents {
			result.sizes[filepath.Clean(entry.Path)] = entry.Size
		}

		// V4: verify every declared entry's checksum, bounded by its own
		// declared size + slack (spec §3.5, §3.9).
		if checksumErr := verifyManifestChecksums(result.archivePath, manifest); checksumErr != nil {
			result.cleanup()
			return nil, fmt.Errorf("%w: %s", ErrBackupValidationFailed, checksumErr.Error())
		}
	} else {
		logger.Log().WithField("filename", util.SanitizeForLog(cleanName)).
			Warn("Restoring/validating a legacy v1 backup archive with no manifest; skipping checksum verification")
	}

	// V5: extract the database (+ -wal/-shm) to a temp file.
	restoreDBPath, err := s.extractDatabaseFromBackupWithSizes(result.archivePath, result.sizes)
	if err != nil {
		result.cleanup()
		return nil, fmt.Errorf("%w: extract database from backup: %s", ErrBackupValidationFailed, err.Error())
	}
	result.restoreDBPath = restoreDBPath
	result.cleanupPaths = append(result.cleanupPaths, restoreDBPath, restoreDBPath+"-wal", restoreDBPath+"-shm")

	// V6: PRAGMA integrity_check + sanity check that this looks like a
	// Charon database.
	if err := sanityCheckSQLiteFile(restoreDBPath); err != nil {
		result.cleanup()
		return nil, fmt.Errorf("%w: %s", ErrBackupValidationFailed, err.Error())
	}

	return result, nil
}

// ValidateBackup performs a dry-run validation of filename (spec §3.3.2
// POST /backups/:filename/validate) — steps V1-V6 only; nothing is mutated
// and all temp files are cleaned up before returning.
func (s *BackupService) ValidateBackup(filename, passphrase string) (*BackupValidation, error) {
	validated, err := s.validateBackupArchive(filename, passphrase)
	if err != nil {
		return nil, err
	}
	defer validated.cleanup()

	result := &BackupValidation{
		Valid:             true,
		LegacyFormat:      validated.legacyFormat,
		DatabaseIntegrity: "ok",
	}

	if validated.manifest != nil {
		result.FormatVersion = validated.manifest.FormatVersion
		result.AppVersion = validated.manifest.AppVersion
		result.CreatedAt = validated.manifest.CreatedAt
		result.EncryptionKeyRequired = validated.manifest.EncryptionKeyRequired

		if validated.manifest.AppVersion != "" && validated.manifest.AppVersion != version.Version {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"archive app_version (%s) differs from the running version (%s)",
				validated.manifest.AppVersion, version.Version))
		}
	} else {
		result.FormatVersion = 1
	}

	return result, nil
}

// RestoreBackupSafe implements the full V->S->A->R->F pipeline (spec §3.5).
// It returns ErrBackupInProgress (mapped to 409 by the handler) if another
// create/restore is already running, rather than blocking indefinitely —
// restores and creates are mutually exclusive but a caller should not hang
// on a possibly long-running concurrent operation.
func (s *BackupService) RestoreBackupSafe(filename, passphrase string) (*RestoreResult, error) {
	if !s.mu.TryLock() {
		return nil, ErrBackupInProgress
	}
	defer s.mu.Unlock()

	// V1-V6.
	validated, err := s.validateBackupArchive(filename, passphrase)
	if err != nil {
		return nil, err // F1: nothing was touched.
	}

	// The extracted DB temp file must survive past this function (it's
	// consumed by RehydrateLiveDatabase / handed to the pending-restore
	// file), so pull it out of validatedArchive's own cleanup list.
	validated.dropFromCleanup(validated.restoreDBPath)
	defer validated.cleanup()

	if s.restoreDBPath != "" && s.restoreDBPath != validated.restoreDBPath {
		_ = os.Remove(s.restoreDBPath)
	}
	s.restoreDBPath = validated.restoreDBPath

	result := &RestoreResult{LegacyFormat: validated.legacyFormat}

	// S1: safety backup of the CURRENT state before anything is mutated.
	preRestoreRecord, err := s.createBackupLocked(BackupOptions{Type: "pre_restore"})
	if err != nil {
		return nil, fmt.Errorf("create pre-restore safety backup: %w", err)
	}
	result.PreRestoreBackup = preRestoreRecord.Filename

	// A1: apply everything except the DB entries into DataDir.
	skipEntries := map[string]struct{}{
		s.DatabaseName:          {},
		s.DatabaseName + "-wal": {},
		s.DatabaseName + "-shm": {},
	}
	if err := s.unzipWithSkipManifest(validated.archivePath, s.DataDir, skipEntries, validated.sizes); err != nil {
		// F2: re-apply the known-good pre_restore archive, then fail loudly.
		preRestorePath := filepath.Join(s.BackupDir, preRestoreRecord.Filename)
		if rerunErr := s.unzipWithSkip(preRestorePath, s.DataDir, skipEntries); rerunErr != nil {
			logger.Log().WithError(rerunErr).Error("failed to restore previous state after apply failure")
		}
		return nil, fmt.Errorf("restore failed, previous state restored: %w", err)
	}

	// A2: live rehydrate, mirroring the retry policy the handler used to
	// own directly (now centralized here so RestoreBackupSafe is the single
	// place that decides restart_required/database_swap_pending).
	rehydrated := false
	if s.db != nil {
		var rehydrateErr error
		for attempt := 0; attempt < 5; attempt++ {
			rehydrateErr = s.RehydrateLiveDatabase(s.db)
			if rehydrateErr == nil {
				rehydrated = true
				break
			}
			if !isSQLiteTransientRestoreError(rehydrateErr) || attempt == 4 {
				break
			}
			time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
		}
		if !rehydrated && rehydrateErr != nil {
			logger.Log().WithError(rehydrateErr).Warn("Backup restored but live database rehydrate failed")
		}
	}
	result.LiveRehydrateApplied = rehydrated
	result.RestartRequired = !rehydrated

	if !rehydrated {
		// F3: persist a durable pending-restore file for the next boot,
		// since the in-memory temp file alone does not survive a restart.
		if pendingErr := s.writePendingRestoreFile(validated.restoreDBPath); pendingErr != nil {
			logger.Log().WithError(pendingErr).Error("failed to persist pending-restore file")
		} else {
			result.DatabaseSwapPending = true
			if s.db != nil {
				if updErr := s.db.Model(&models.BackupRecord{}).
					Where("filename = ?", validated.cleanName).
					Update("status", "restore_pending").Error; updErr != nil {
					logger.Log().WithError(updErr).Warn("failed to mark backup record as restore_pending")
				}
			}
		}
	}

	// R1: reload Caddy — it has its own snapshot/rollback, so this runs
	// regardless of A2's outcome (A1 already wrote the new Caddy/CrowdSec
	// files to DataDir by this point).
	if s.caddyReloader != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if reloadErr := s.caddyReloader.ApplyConfig(ctx); reloadErr != nil {
			// F4: log and report caddy_reloaded=false; Caddy's own manager
			// has already rolled itself back.
			logger.Log().WithError(reloadErr).Warn("failed to reload caddy config after restore")
		} else {
			result.CaddyReloaded = true
		}
		cancel()
	}

	// R2.
	if result.DatabaseSwapPending {
		result.Message = "Backup restored; the database will finish restoring on the next process restart"
	} else {
		result.Message = "Backup restored successfully"
	}

	return result, nil
}

// writePendingRestoreFile persists the already-validated replacement
// database at sourcePath to DataDir/<DatabaseName>.pending-restore (0600,
// fsync'd) for backend/internal/database.ApplyPendingRestore to consume on
// the next process start (spec §3.5 F3).
func (s *BackupService) writePendingRestoreFile(sourcePath string) error {
	pendingPath := filepath.Join(s.DataDir, s.DatabaseName+".pending-restore")

	src, err := os.Open(sourcePath) // #nosec G304 -- sourcePath is a server-controlled, already-validated temp file
	if err != nil {
		return fmt.Errorf("open validated restore database: %w", err)
	}
	defer func() {
		_ = src.Close()
	}()

	dst, err := os.OpenFile(pendingPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- pendingPath is derived from server-controlled DataDir/DatabaseName
	if err != nil {
		return fmt.Errorf("create pending-restore file: %w", err)
	}
	defer func() {
		_ = dst.Close()
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("write pending-restore file: %w", err)
	}

	return dst.Sync()
}

// readBackupManifest looks for manifest.json inside the zip at archivePath.
// A nil manifest with legacyFormat=true means the archive is a v1 (or
// unrecognized) archive with no manifest.
func readBackupManifest(archivePath string) (manifest *BackupManifest, legacyFormat bool, err error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, false, fmt.Errorf("open archive: %w", err)
	}
	defer func() {
		_ = r.Close()
	}()

	for _, f := range r.File {
		if filepath.Clean(f.Name) != "manifest.json" {
			continue
		}

		rc, openErr := f.Open()
		if openErr != nil {
			return nil, false, fmt.Errorf("open manifest entry: %w", openErr)
		}
		defer func() {
			_ = rc.Close()
		}()

		var parsed BackupManifest
		if decodeErr := json.NewDecoder(rc).Decode(&parsed); decodeErr != nil {
			return nil, false, fmt.Errorf("parse manifest.json: %w", decodeErr)
		}
		return &parsed, false, nil
	}

	return nil, true, nil
}

// verifyManifestChecksums streams every manifest-declared entry from the
// archive at archivePath, bounding each entry's read by its own declared
// Size + manifestEntrySizeSlack (spec §3.5 V4, §3.9), and rejects on the
// first size or checksum mismatch.
func verifyManifestChecksums(archivePath string, manifest *BackupManifest) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() {
		_ = r.Close()
	}()

	byPath := make(map[string]*zip.File, len(r.File))
	for _, f := range r.File {
		byPath[filepath.Clean(f.Name)] = f
	}

	for _, entry := range manifest.Contents {
		cleanPath := filepath.Clean(entry.Path)
		f, ok := byPath[cleanPath]
		if !ok {
			return fmt.Errorf("manifest entry %q is missing from the archive", entry.Path)
		}

		rc, openErr := f.Open()
		if openErr != nil {
			return fmt.Errorf("open archive entry %q: %w", entry.Path, openErr)
		}

		h := sha256.New()
		lr := &io.LimitedReader{R: rc, N: entry.Size + manifestEntrySizeSlack + 1}
		written, copyErr := io.Copy(h, lr)
		closeErr := rc.Close()

		if copyErr != nil {
			return fmt.Errorf("read archive entry %q: %w", entry.Path, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close archive entry %q: %w", entry.Path, closeErr)
		}
		if lr.N == 0 {
			return fmt.Errorf("archive entry %q exceeds its declared size plus slack", entry.Path)
		}
		if written != entry.Size {
			return fmt.Errorf("archive entry %q size mismatch: manifest declares %d bytes, archive has %d", entry.Path, entry.Size, written)
		}

		sum := hex.EncodeToString(h.Sum(nil))
		if sum != entry.SHA256 {
			return fmt.Errorf("archive entry %q failed checksum verification", entry.Path)
		}
	}

	return nil
}

// sanityCheckSQLiteFile runs spec §3.5 V6: PRAGMA integrity_check plus a
// sanity query that sqlite_master contains the "users" and "proxy_hosts"
// tables, so a restore is rejected before any live mutation if the
// extracted file isn't actually a Charon database.
func sanityCheckSQLiteFile(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open extracted database: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("run integrity check: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(integrity), "ok") {
		return fmt.Errorf("database integrity check failed: %s", integrity)
	}

	for _, table := range []string{"users", "proxy_hosts"} {
		var count int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table,
		).Scan(&count); err != nil {
			return fmt.Errorf("check for table %q: %w", table, err)
		}
		if count == 0 {
			return fmt.Errorf("extracted database is missing the %q table; this does not look like a Charon database", table)
		}
	}

	return nil
}

// isSQLiteTransientRestoreError mirrors
// handlers.isSQLiteTransientRehydrateError (kept as a small, intentional
// duplication rather than an inter-package dependency, since the handler's
// own copy is independently unit-tested and this package must not import
// the handlers package).
func isSQLiteTransientRestoreError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database is busy") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "table is locked") ||
		strings.Contains(message, "resource busy")
}
