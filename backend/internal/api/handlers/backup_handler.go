package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// maxBackupUploadSize bounds the /backups/upload request body (spec §3.3.2;
// precedent: image_upload_handler.go's maxImageUploadSize).
const maxBackupUploadSize = 512 * 1024 * 1024 // 512 MB

type BackupHandler struct {
	service         *services.BackupService
	securityService *services.SecurityService
	db              *gorm.DB
}

func NewBackupHandler(service *services.BackupService) *BackupHandler {
	return NewBackupHandlerWithDeps(service, nil, nil)
}

func NewBackupHandlerWithDeps(service *services.BackupService, securityService *services.SecurityService, db *gorm.DB) *BackupHandler {
	return &BackupHandler{service: service, securityService: securityService, db: db}
}

// backupListEntry is the per-backup shape returned by List (spec §3.3.1).
// filename/size/time are unchanged from v1 so the existing frontend and
// mocked E2E helpers keep working during the transition; the rest are
// additive and only populated when a matching BackupRecord exists.
type backupListEntry struct {
	Filename      string                  `json:"filename"`
	Size          int64                   `json:"size"`
	Time          time.Time               `json:"time"`
	UUID          string                  `json:"uuid,omitempty"`
	Type          string                  `json:"type,omitempty"`
	Encrypted     bool                    `json:"encrypted,omitempty"`
	FormatVersion int                     `json:"format_version,omitempty"`
	SHA256        string                  `json:"sha256,omitempty"`
	Status        string                  `json:"status,omitempty"`
	AppVersion    string                  `json:"app_version,omitempty"`
	RemoteCopies  []backupRemoteCopyEntry `json:"remote_copies,omitempty"`
}

type backupRemoteCopyEntry struct {
	TargetUUID string     `json:"target_uuid"`
	TargetName string     `json:"target_name"`
	Status     string     `json:"status"`
	UploadedAt *time.Time `json:"uploaded_at,omitempty"`
}

// List returns the backup history, reconciling the filesystem scan with any
// persisted BackupRecord (spec §3.3.1). Files on disk without a record
// (pre-upgrade backups) are listed as legacy manual/v1 entries; records
// whose file has disappeared are marked Status "deleted" so a future list
// no longer references them.
func (h *BackupHandler) List(c *gin.Context) {
	files, err := h.service.ListBackups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list backups"})
		return
	}

	entries := make([]backupListEntry, 0, len(files))

	var recordsByFilename map[string]models.BackupRecord
	if h.db != nil {
		var records []models.BackupRecord
		if err := h.db.Preload("RemoteCopies.RemoteTarget").Find(&records).Error; err == nil {
			recordsByFilename = make(map[string]models.BackupRecord, len(records))
			onDisk := make(map[string]struct{}, len(files))
			for _, f := range files {
				onDisk[f.Filename] = struct{}{}
			}
			for _, rec := range records {
				recordsByFilename[rec.Filename] = rec
				if _, exists := onDisk[rec.Filename]; !exists && rec.Status != "deleted" {
					h.db.Model(&models.BackupRecord{}).Where("id = ?", rec.ID).Update("status", "deleted")
				}
			}
		}
	}

	for _, f := range files {
		entry := backupListEntry{Filename: f.Filename, Size: f.Size, Time: f.Time, Type: "manual", FormatVersion: 1}
		if rec, ok := recordsByFilename[f.Filename]; ok {
			entry.UUID = rec.UUID
			entry.Type = rec.Type
			entry.Encrypted = rec.Encrypted
			entry.FormatVersion = rec.FormatVersion
			entry.SHA256 = rec.SHA256
			entry.Status = rec.Status
			entry.AppVersion = rec.AppVersion
			for _, copyRow := range rec.RemoteCopies {
				remoteEntry := backupRemoteCopyEntry{Status: copyRow.Status, UploadedAt: copyRow.UploadedAt}
				if copyRow.RemoteTarget != nil {
					remoteEntry.TargetUUID = copyRow.RemoteTarget.UUID
					remoteEntry.TargetName = copyRow.RemoteTarget.Name
				}
				entry.RemoteCopies = append(entry.RemoteCopies, remoteEntry)
			}
		}
		entries = append(entries, entry)
	}

	c.JSON(http.StatusOK, entries)
}

type createBackupRequest struct {
	Encrypt    bool   `json:"encrypt"`
	Passphrase string `json:"passphrase"`
}

// Create handles POST /api/v1/backups (admin). Body is optional (spec
// §3.3.1); when encrypt is requested without a passphrase, both settings
// defaults and an explicit 400 are reasonable — we require the caller to
// supply one explicitly rather than silently falling back to a stored
// scheduled passphrase, since manual encryption is a per-request secret
// (spec §3.6).
func (h *BackupHandler) Create(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req createBackupRequest
	_ = c.ShouldBindJSON(&req) // optional body

	record, err := h.service.CreateBackupWithOptions(services.BackupOptions{
		Type:       "manual",
		Encrypt:    req.Encrypt,
		Passphrase: req.Passphrase,
	})
	if err != nil {
		h.respondCreateError(c, err)
		return
	}

	middleware.GetRequestLogger(c).WithField("action", "create_backup").
		WithField("filename", util.SanitizeForLog(record.Filename)).Info("Backup created successfully")
	c.JSON(http.StatusCreated, gin.H{"filename": record.Filename, "uuid": record.UUID, "message": "Backup created successfully"})
}

func (h *BackupHandler) respondCreateError(c *gin.Context, err error) {
	middleware.GetRequestLogger(c).WithField("action", "create_backup").WithError(err).Error("Failed to create backup")

	switch {
	case errors.Is(err, services.ErrBackupInProgress):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrInsufficientSpace):
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "error_code": "backup_insufficient_space"})
	default:
		if respondPermissionError(c, h.securityService, "backup_create_failed", err, h.service.BackupDir) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create backup: " + err.Error()})
	}
}

// Delete handles DELETE /api/v1/backups/:filename (admin). Also removes the
// BackupRecord and any BackupRemoteCopy rows (best-effort, logged only).
func (h *BackupHandler) Delete(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	filename := c.Param("filename")
	if err := h.service.DeleteBackup(filename); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
			return
		}
		if respondPermissionError(c, h.securityService, "backup_delete_failed", err, h.service.BackupDir) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete backup"})
		return
	}

	if h.db != nil {
		var record models.BackupRecord
		if err := h.db.Where("filename = ?", filename).First(&record).Error; err == nil {
			if err := h.db.Where("backup_record_id = ?", record.ID).Delete(&models.BackupRemoteCopy{}).Error; err != nil {
				logger.Log().WithError(err).Warn("failed to delete backup remote copy rows")
			}
			if err := h.db.Delete(&record).Error; err != nil {
				logger.Log().WithError(err).Warn("failed to delete backup record")
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Backup deleted"})
}

// Download handles GET /api/v1/backups/:filename/download. Admin-gated
// (changed from management in v1 — a backup contains the full database
// including password hashes and cert private keys, spec §3.9).
func (h *BackupHandler) Download(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	filename := c.Param("filename")
	path, err := h.service.GetBackupPath(filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.File(path)
}

type restoreRequest struct {
	Passphrase string `json:"passphrase"`
}

// Restore handles POST /api/v1/backups/:filename/restore (admin), running
// the full V->S->A->R->F safe-restore pipeline (spec §3.5).
func (h *BackupHandler) Restore(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	filename := c.Param("filename")

	var req restoreRequest
	_ = c.ShouldBindJSON(&req) // optional body; passphrase required only for .age archives

	result, err := h.service.RestoreBackupSafe(filename, req.Passphrase)
	if err != nil {
		// codeql[go/log-injection] Safe: user input sanitized via util.SanitizeForLog()
		// which removes control characters (0x00-0x1F, 0x7F) including CRLF
		middleware.GetRequestLogger(c).WithField("action", "restore_backup").
			WithField("filename", util.SanitizeForLog(filepath.Base(filename))).
			WithField("error", util.SanitizeForLog(err.Error())).Error("Failed to restore backup")
		h.respondRestoreError(c, err)
		return
	}

	middleware.GetRequestLogger(c).WithField("action", "restore_backup").
		WithField("filename", util.SanitizeForLog(filepath.Base(filename))).Info("Backup restored successfully")
	c.JSON(http.StatusOK, result)
}

func (h *BackupHandler) respondRestoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrBackupInProgress):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrPassphraseRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_passphrase_required"})
	case errors.Is(err, services.ErrPassphraseInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_passphrase_invalid"})
	case errors.Is(err, services.ErrNewerBackupFormat), errors.Is(err, services.ErrBackupValidationFailed):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_validation_failed"})
	case os.IsNotExist(err):
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
	case errors.Is(err, services.ErrRestoreUnrecoverable):
		// C1: rehydrate (A2) and the durable pending-restore fallback (F3)
		// both failed — an internal, non-client-actionable failure (like
		// default's 500), but with an explicit error_code so operators/the
		// frontend/log-scrapers can key off it instead of string-matching
		// err.Error().
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "error_code": "backup_restore_unrecoverable"})
	default:
		if respondPermissionError(c, h.securityService, "backup_restore_failed", err, h.service.BackupDir) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore backup: " + err.Error()})
	}
}

// Validate handles POST /api/v1/backups/:filename/validate (admin) — a
// dry-run of the V1-V6 validation steps (spec §3.3.2, §3.5). Nothing is
// mutated.
func (h *BackupHandler) Validate(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	filename := c.Param("filename")

	var req restoreRequest
	_ = c.ShouldBindJSON(&req)

	result, err := h.service.ValidateBackup(filename, req.Passphrase)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPassphraseRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_passphrase_required"})
		case errors.Is(err, services.ErrPassphraseInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_passphrase_invalid"})
		case os.IsNotExist(err):
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_validation_failed", "valid": false})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

// Upload handles POST /api/v1/backups/upload (admin, multipart) — accepts
// a v2 (.zip/.zip.age), legacy v1 (.zip), or raw v0 SQLite (.db) file,
// validates it, and persists it under a server-generated filename (spec
// §3.3.2, §3.10). Client-supplied filenames are discarded entirely.
func (h *BackupHandler) Upload(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBackupUploadSize)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required: " + err.Error()})
		return
	}

	passphrase := c.PostForm("passphrase")

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to open uploaded file"})
		return
	}
	defer func() {
		_ = src.Close()
	}()

	// Magic-byte detection (spec §3.3.2, §3.10) — never trust the client
	// filename/extension alone. 32 bytes comfortably covers the zip
	// ("PK\x03\x04"), age ("age-encryption.org/"), and sqlite
	// ("SQLite format 3\x00") magic headers.
	header := make([]byte, 32)
	n, _ := io.ReadFull(src, header)
	header = header[:n]

	kind, err := detectBackupUploadKind(header)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rest, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read uploaded file"})
		return
	}
	fullContent := make([]byte, 0, len(header)+len(rest))
	fullContent = append(fullContent, header...)
	fullContent = append(fullContent, rest...)

	timestamp := time.Now().Format("2006-01-02_15-04-05")

	var filename string
	var legacyFormat bool

	switch kind {
	case uploadKindSQLite:
		wrapped, wrapErr := h.service.WrapRawDatabaseAsBackup(fullContent, timestamp)
		if wrapErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to wrap uploaded database: " + wrapErr.Error()})
			return
		}
		filename = wrapped
	case uploadKindZip, uploadKindAge:
		ext := ".zip"
		if kind == uploadKindAge {
			ext = ".zip.age"
		}
		filename = fmt.Sprintf("uploaded_%s%s", timestamp, ext)
		destPath := filepath.Join(h.service.BackupDir, filename)
		if writeErr := os.WriteFile(destPath, fullContent, 0o600); writeErr != nil { // #nosec G306 -- backup archive, 0600 is intentional
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store uploaded file"})
			return
		}
	}

	validation, err := h.service.ValidateBackup(filename, passphrase)
	if err != nil {
		_ = h.service.DeleteBackup(filename)
		switch {
		case errors.Is(err, services.ErrPassphraseRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_passphrase_required"})
		case errors.Is(err, services.ErrPassphraseInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_passphrase_invalid"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "uploaded backup failed validation: " + err.Error(), "error_code": "backup_validation_failed"})
		}
		return
	}
	legacyFormat = validation.LegacyFormat

	sum, size, err := services.SHA256File(filepath.Join(h.service.BackupDir, filename))
	record := &models.BackupRecord{
		Filename:      filename,
		Type:          "uploaded",
		Status:        "completed",
		FormatVersion: validation.FormatVersion,
		Encrypted:     strings.HasSuffix(filename, ".age"),
	}
	if err == nil {
		record.SHA256 = sum
		record.Size = size
	}
	if h.db != nil {
		if createErr := h.db.Create(record).Error; createErr != nil {
			logger.Log().WithError(createErr).Warn("failed to persist uploaded backup record")
		}
	}

	middleware.GetRequestLogger(c).WithField("action", "upload_backup").
		WithField("filename", util.SanitizeForLog(filename)).Info("Backup uploaded and validated")

	c.JSON(http.StatusCreated, gin.H{
		"filename":      filename,
		"uuid":          record.UUID,
		"legacy_format": legacyFormat,
		"message":       "Backup uploaded and validated",
	})
}

// GetSettings handles GET /api/v1/backups/settings (management, spec
// §3.3.2).
func (h *BackupHandler) GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.service.GetSettings())
}

type updateBackupSettingsRequest struct {
	ScheduleEnabled      *bool   `json:"schedule_enabled"`
	ScheduleCron         *string `json:"schedule_cron"`
	RetentionCount       *int    `json:"retention_count"`
	RemoteRetentionCount *int    `json:"remote_retention_count"`
	EncryptionEnabled    *bool   `json:"encryption_enabled"`
	EncryptionPassphrase *string `json:"encryption_passphrase"`
}

// UpdateSettings handles PUT /api/v1/backups/settings (admin, spec §3.3.2).
// Every field is optional (partial update); validation and the
// reschedule side effect are entirely owned by
// services.BackupService.UpdateSettings.
func (h *BackupHandler) UpdateSettings(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req updateBackupSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.UpdateSettings(services.BackupSettingsUpdate{
		ScheduleEnabled:      req.ScheduleEnabled,
		ScheduleCron:         req.ScheduleCron,
		RetentionCount:       req.RetentionCount,
		RemoteRetentionCount: req.RemoteRetentionCount,
		EncryptionEnabled:    req.EncryptionEnabled,
		EncryptionPassphrase: req.EncryptionPassphrase,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCronSchedule):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_invalid_cron"})
		case errors.Is(err, services.ErrInvalidRetentionCount):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": "backup_invalid_retention_count"})
		case errors.Is(err, services.ErrEncryptionKeyMissing):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "error_code": "encryption_key_missing"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update backup settings: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, h.service.GetSettings())
}

type uploadKind int

const (
	uploadKindZip uploadKind = iota
	uploadKindAge
	uploadKindSQLite
)

// sqliteMagicHeader is the fixed 16-byte header every SQLite database file
// begins with ("SQLite format 3\x00").
var sqliteMagicHeader = []byte("SQLite format 3\x00")

// ageMagicPrefix is the fixed literal age encrypts every file with as its
// first line.
var ageMagicPrefix = []byte("age-encryption.org/")

// detectBackupUploadKind classifies an uploaded file by magic bytes only
// (spec §3.3.2, §3.10) — never by client-supplied extension.
func detectBackupUploadKind(header []byte) (uploadKind, error) {
	if len(header) >= 4 && bytes.Equal(header[:4], []byte("PK\x03\x04")) {
		return uploadKindZip, nil
	}
	if len(header) >= len(ageMagicPrefix) && bytes.Equal(header[:len(ageMagicPrefix)], ageMagicPrefix) {
		return uploadKindAge, nil
	}
	if len(header) >= len(sqliteMagicHeader) && bytes.Equal(header[:len(sqliteMagicHeader)], sqliteMagicHeader) {
		return uploadKindSQLite, nil
	}
	return 0, fmt.Errorf("unrecognized file type: expected a zip, age-encrypted, or sqlite database file")
}

// isSQLiteTransientRehydrateError classifies sqlite errors that are worth
// retrying a live rehydrate against (busy/locked database). The retry loop
// itself now lives in services.RestoreBackupSafe (spec §3.5 A2), but this
// helper remains here — and independently unit-tested
// (TestIsSQLiteTransientRehydrateError) — as it predates that move and
// nothing about its own correctness changed.
func isSQLiteTransientRehydrateError(err error) bool {
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
