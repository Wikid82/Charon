package services

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/version"
)

// SHA256File computes the SHA-256 checksum and size of the file at path —
// exported so handlers can compute a BackupRecord's checksum for files they
// write directly (e.g. the upload endpoint persisting an already-uploaded
// .zip/.zip.age), without duplicating the streaming-hash logic.
func SHA256File(path string) (string, int64, error) {
	return sha256File(path)
}

// WrapRawDatabaseAsBackup wraps a v0 raw SQLite database upload (detected by
// magic header "SQLite format 3\x00", spec §3.2) into a minimal format-v2
// archive containing only the database and a manifest — no caddy/crowdsec,
// since none exists for a bare uploaded .db file. This lets a v0 upload flow
// through the exact same validate/restore pipeline as every other backup.
// Per spec, v0 archives are reachable only via the upload endpoint — never
// listed from disk directly.
func (s *BackupService) WrapRawDatabaseAsBackup(rawDB []byte, timestamp string) (string, error) {
	if err := os.MkdirAll(s.BackupDir, 0o700); err != nil {
		return "", fmt.Errorf("ensure backup directory: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "charon-upload-raw-db-*.sqlite")
	if err != nil {
		return "", fmt.Errorf("create temp file for uploaded database: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, writeErr := tmpFile.Write(rawDB); writeErr != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("write uploaded database to temp file: %w", writeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return "", fmt.Errorf("close temp file for uploaded database: %w", closeErr)
	}

	filename := fmt.Sprintf("uploaded_%s.zip", timestamp)
	zipPath := filepath.Join(s.BackupDir, filename)

	manifest := &BackupManifest{
		FormatVersion: 2,
		CreatedAt:     time.Now().UTC(),
		AppVersion:    version.Version,
		BackupType:    "uploaded",
		DatabaseName:  s.DatabaseName,
	}

	outFile, err := os.OpenFile(zipPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- zipPath is derived from server-controlled BackupDir
	if err != nil {
		return "", fmt.Errorf("create wrapped archive: %w", err)
	}
	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("failed to close wrapped upload archive")
		}
	}()

	w := zip.NewWriter(outFile)

	if zipErr := s.addToZipTracked(w, tmpPath, s.DatabaseName, &manifest.Contents); zipErr != nil {
		return "", fmt.Errorf("wrap uploaded database: %w", zipErr)
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal backup manifest: %w", err)
	}
	manifestEntry, err := w.Create("manifest.json")
	if err != nil {
		return "", fmt.Errorf("create manifest entry: %w", err)
	}
	if _, err := manifestEntry.Write(manifestBytes); err != nil {
		return "", fmt.Errorf("write manifest entry: %w", err)
	}

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize wrapped archive: %w", err)
	}

	return filename, nil
}
