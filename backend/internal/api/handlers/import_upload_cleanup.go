package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

const (
	// uploadSessionTTL is how long a pending or reviewing Caddyfile upload waits for its commit.
	uploadSessionTTL = 24 * time.Hour
	// closedSessionRetention is how long a rejected, expired or failed import session row is kept.
	closedSessionRetention = 30 * 24 * time.Hour
	// maxImportFiles bounds the files of one multi-file Caddyfile upload.
	maxImportFiles = 64
)

// uploadsSubdir is the directory under the import directory that holds Caddyfile uploads.
const uploadsSubdir = "uploads"

// uploadArtifactOf returns the entry under importDir/uploads that holds the upload sourceFile points into: the file
// of a single upload, or the directory of a multi-file upload. ok is false for any path outside uploads, such as the
// mounted Caddyfile.
func uploadArtifactOf(importDir, sourceFile string) (string, bool) {
	if sourceFile == "" {
		return "", false
	}
	uploadsDir := filepath.Join(importDir, uploadsSubdir)
	rel, err := filepath.Rel(uploadsDir, filepath.Clean(sourceFile))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	first := strings.SplitN(rel, string(filepath.Separator), 2)[0]
	if first == "" || first == "." || first == ".." {
		return "", false
	}
	return filepath.Join(uploadsDir, first), true
}

// removeUploadArtifact removes the upload entry that sourceFile points into. Paths outside importDir/uploads are
// never removed.
func removeUploadArtifact(importDir, sourceFile string) error {
	artifact, ok := uploadArtifactOf(importDir, sourceFile)
	if !ok {
		return nil
	}
	safe, err := safeJoin(importDir, filepath.Join(uploadsSubdir, filepath.Base(artifact)))
	if err != nil {
		return fmt.Errorf("resolve upload %q: %w", filepath.Base(artifact), err)
	}
	if err := os.RemoveAll(safe); err != nil {
		return fmt.Errorf("remove upload %q: %w", filepath.Base(artifact), err)
	}
	return nil
}

// sweepImportUploads applies the retention rules of Caddyfile uploads under importDir/uploads (D-35-7), at now:
//   - pending or reviewing upload sessions older than the TTL become expired and their files are removed;
//   - entries under uploads/ older than the TTL that no live session refers to are removed;
//   - rejected, expired and failed session rows older than the retention are deleted.
//
// Sessions that read the mounted Caddyfile (mountPath) are never touched.
func sweepImportUploads(db *gorm.DB, importDir, mountPath string, now time.Time) error {
	cutoff := now.Add(-uploadSessionTTL)
	var stale []models.ImportSession
	if err := db.Where("status IN ? AND created_at < ?", liveSessionStatuses, cutoff).Find(&stale).Error; err != nil {
		return fmt.Errorf("list stale import sessions: %w", err)
	}
	for _, session := range stale {
		if mountPath != "" && session.SourceFile == mountPath {
			continue
		}
		if err := expireUploadSession(db, importDir, session); err != nil {
			return err
		}
	}

	if err := removeOrphanUploads(db, importDir, cutoff); err != nil {
		return err
	}

	retentionCutoff := now.Add(-closedSessionRetention)
	query := db.Where("status IN ? AND updated_at < ?", []string{"rejected", "expired", "failed"}, retentionCutoff)
	if mountPath != "" {
		query = query.Where("source_file <> ?", mountPath)
	}
	if err := query.Delete(&models.ImportSession{}).Error; err != nil {
		return fmt.Errorf("delete closed import sessions: %w", err)
	}
	return nil
}

// liveSessionStatuses are the statuses of sessions that still wait for a commit.
var liveSessionStatuses = []string{"pending", "reviewing"}

// expireUploadSession marks an upload session expired and removes its files. Mounted sessions are left alone.
func expireUploadSession(db *gorm.DB, importDir string, session models.ImportSession) error {
	if _, ok := uploadArtifactOf(importDir, session.SourceFile); !ok {
		return nil
	}
	if err := db.Model(&models.ImportSession{}).Where("id = ?", session.ID).Update("status", "expired").Error; err != nil {
		return fmt.Errorf("expire import session %s: %w", session.UUID, err)
	}
	return removeUploadArtifact(importDir, session.SourceFile)
}

// removeOrphanUploads removes entries under uploads/ older than cutoff that no live session refers to.
func removeOrphanUploads(db *gorm.DB, importDir string, cutoff time.Time) error {
	uploadsDir := filepath.Join(importDir, uploadsSubdir)
	entries, err := os.ReadDir(uploadsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read uploads directory: %w", err)
	}
	var live []models.ImportSession
	if err := db.Where("status IN ?", liveSessionStatuses).Find(&live).Error; err != nil {
		return fmt.Errorf("list live import sessions: %w", err)
	}
	referenced := make(map[string]bool, len(live))
	for _, session := range live {
		if artifact, ok := uploadArtifactOf(importDir, session.SourceFile); ok {
			referenced[filepath.Base(artifact)] = true
		}
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) || referenced[entry.Name()] {
			continue
		}
		if err := removeUploadArtifact(importDir, filepath.Join(uploadsDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// supersedeUploadSessions expires the older pending or reviewing upload sessions once keepUUID is stored, and removes
// their files. It runs only after the new session row exists, so a failed upload never expires the previous one.
func supersedeUploadSessions(db *gorm.DB, importDir, keepUUID string) error {
	var older []models.ImportSession
	if err := db.Where("status IN ? AND uuid <> ?", liveSessionStatuses, keepUUID).Find(&older).Error; err != nil {
		return fmt.Errorf("list older import sessions: %w", err)
	}
	for _, session := range older {
		if err := expireUploadSession(db, importDir, session); err != nil {
			return err
		}
	}
	return nil
}
