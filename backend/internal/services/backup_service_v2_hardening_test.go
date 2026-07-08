package services

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createCharonLikeTestDB creates a sqlite database with the users and
// proxy_hosts tables the V6 sanity check (spec §3.5) requires, plus an
// optional padding row so the resulting file can be grown arbitrarily large
// for the >100MB round-trip test.
func createCharonLikeTestDB(t *testing.T, dbPath string, paddingBytes int) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (email) VALUES ('admin@example.com')`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE proxy_hosts (id INTEGER PRIMARY KEY, domain_names TEXT)`)
	require.NoError(t, err)

	if paddingBytes > 0 {
		_, err = db.Exec(`CREATE TABLE padding (id INTEGER PRIMARY KEY, data BLOB)`)
		require.NoError(t, err)
		blob := bytes.Repeat([]byte("x"), paddingBytes)
		_, err = db.Exec(`INSERT INTO padding (data) VALUES (?)`, blob)
		require.NoError(t, err)
	}
}

func newHardeningTestService(t *testing.T) *BackupService {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	dbPath := filepath.Join(dataDir, "charon.db")
	createCharonLikeTestDB(t, dbPath, 0)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := NewBackupService(cfg, nil, nil)
	t.Cleanup(svc.Stop)
	return svc
}

// TestRestoreBackupSafe_TamperedChecksum_Rejected is required test #1: a v2
// archive whose entry bytes are corrupted after the manifest was written
// must be rejected by checksum verification (V4) before any live mutation —
// no pre_restore safety backup should be created, proving S1/A1 never ran.
func TestRestoreBackupSafe_TamperedChecksum_Rejected(t *testing.T) {
	svc := newHardeningTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	tamperZipEntry(t, filepath.Join(svc.BackupDir, record.Filename), "charon.db")

	before, err := svc.ListBackups()
	require.NoError(t, err)
	require.Len(t, before, 1, "only the original archive should exist before the failed restore attempt")

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.Error(t, err)
	require.Nil(t, result)
	assert.True(t, errors.Is(err, ErrBackupValidationFailed), "got: %v", err)

	after, err := svc.ListBackups()
	require.NoError(t, err)
	assert.Len(t, after, 1, "a failed validation must not create a pre_restore safety backup or otherwise mutate BackupDir")
}

// tamperZipEntry rewrites the zip at zipPath, flipping one byte in the
// entry named targetEntry while preserving its length (so the corruption is
// caught by checksum mismatch specifically, not a size mismatch) and every
// other entry (including manifest.json) byte-for-byte.
func tamperZipEntry(t *testing.T, zipPath, targetEntry string) {
	t.Helper()

	r, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	tmpPath := zipPath + ".tmp"
	out, err := os.Create(tmpPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)

	w := zip.NewWriter(out)
	for _, f := range r.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		_ = rc.Close()

		if filepath.Clean(f.Name) == targetEntry && len(data) > 0 {
			data[len(data)/2] ^= 0xFF
		}

		entryWriter, err := w.Create(f.Name)
		require.NoError(t, err)
		_, err = entryWriter.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	require.NoError(t, os.Rename(tmpPath, zipPath))
}

// TestRestoreBackupSafe_WrongPassphrase_Rejected is required test #2: an
// age-encrypted archive restored with the wrong passphrase must be rejected
// with no side effects (no pre_restore safety backup created).
func TestRestoreBackupSafe_WrongPassphrase_Rejected(t *testing.T) {
	svc := newHardeningTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual", Encrypt: true, Passphrase: "correct-horse-battery-staple"})
	require.NoError(t, err)
	require.True(t, record.Encrypted)

	result, err := svc.RestoreBackupSafe(record.Filename, "definitely-the-wrong-passphrase")
	require.Error(t, err)
	require.Nil(t, result)
	assert.True(t, errors.Is(err, ErrPassphraseInvalid), "got: %v", err)

	backups, err := svc.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 1, "a wrong-passphrase rejection must not create a pre_restore safety backup or otherwise mutate BackupDir")
}

// TestRestoreBackupSafe_MissingPassphrase_Rejected proves an encrypted
// archive restored with no passphrase at all fails fast with
// ErrPassphraseRequired rather than attempting decryption.
func TestRestoreBackupSafe_MissingPassphrase_Rejected(t *testing.T) {
	svc := newHardeningTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual", Encrypt: true, Passphrase: "hunter2hunter2"})
	require.NoError(t, err)

	_, err = svc.RestoreBackupSafe(record.Filename, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPassphraseRequired), "got: %v", err)
}

// TestExtractZip_RejectsSymlinkEntry is required test #4: a crafted zip
// containing a symlink entry must be rejected before extraction, never
// written to disk.
func TestExtractZip_RejectsSymlinkEntry(t *testing.T) {
	tmp := t.TempDir()
	destDir := filepath.Join(tmp, "data")
	require.NoError(t, os.MkdirAll(destDir, 0o700))

	zipPath := filepath.Join(tmp, "symlink.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	header := &zip.FileHeader{Name: "evil-link", Method: zip.Deflate}
	header.SetMode(fs.ModeSymlink | 0o777)
	entryWriter, err := w.CreateHeader(header)
	require.NoError(t, err)
	_, err = entryWriter.Write([]byte("/etc/passwd"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, zipFile.Close())

	svc := &BackupService{DataDir: destDir, DatabaseName: "charon.db"}
	err = svc.unzipWithSkip(zipPath, destDir, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
	assert.NoFileExists(t, filepath.Join(destDir, "evil-link"))
}

// TestExtractZip_ClampsExtractedFileMode is required test #5: even when a
// crafted archive claims 0o777 (or setuid) permission bits, the extracted
// file must always end up 0o600 regardless.
func TestExtractZip_ClampsExtractedFileMode(t *testing.T) {
	tmp := t.TempDir()
	destDir := filepath.Join(tmp, "data")
	require.NoError(t, os.MkdirAll(destDir, 0o700))

	zipPath := filepath.Join(tmp, "setuid.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	header := &zip.FileHeader{Name: "payload.sh", Method: zip.Deflate}
	header.SetMode(0o777 | fs.ModeSetuid)
	entryWriter, err := w.CreateHeader(header)
	require.NoError(t, err)
	_, err = entryWriter.Write([]byte("#!/bin/sh\necho pwned\n"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, zipFile.Close())

	svc := &BackupService{DataDir: destDir, DatabaseName: "charon.db"}
	require.NoError(t, svc.unzipWithSkip(zipPath, destDir, nil))

	info, err := os.Stat(filepath.Join(destDir, "payload.sh"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm(), "extracted file permissions must always be clamped to 0600 regardless of archive-supplied mode bits")
}

// TestCreateBackupWithOptions_ConcurrentCreate_SecondRequestGetsInProgress
// proves the spec §3.10 concurrency guard: a second concurrent create while
// one is already running gets ErrBackupInProgress (mapped to 409 by the
// handler) rather than blocking indefinitely.
func TestCreateBackupWithOptions_ConcurrentCreate_SecondRequestGetsInProgress(t *testing.T) {
	svc := newHardeningTestService(t)

	svc.mu.Lock()
	defer svc.mu.Unlock()

	_, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBackupInProgress))
}

// TestRestoreBackupSafe_ConcurrentRestore_SecondRequestGetsInProgress mirrors
// the create case for restore.
func TestRestoreBackupSafe_ConcurrentRestore_SecondRequestGetsInProgress(t *testing.T) {
	svc := newHardeningTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	svc.mu.Lock()
	defer svc.mu.Unlock()

	_, err = svc.RestoreBackupSafe(record.Filename, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBackupInProgress))
}

// TestSetRemoteUploadHookAndCaddyReloader exercise the small setter methods
// wiring optional collaborators into BackupService.
func TestSetRemoteUploadHookAndCaddyReloader(t *testing.T) {
	svc := newHardeningTestService(t)

	var called atomic.Bool
	svc.SetRemoteUploadHook(func(ctx context.Context, record *models.BackupRecord) {
		called.Store(true)
	})

	fakeReloader := &fakeCaddyReloader{}
	svc.SetCaddyReloader(fakeReloader)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)
	require.Eventually(t, called.Load, time.Second, 10*time.Millisecond)
	_ = record
}

type fakeCaddyReloader struct{ called bool }

func (f *fakeCaddyReloader) ApplyConfig(_ context.Context) error {
	f.called = true
	return nil
}

// TestIsPreRestoreBackup_NilDB proves the nil-db fallback (no filtering
// possible without a database).
func TestIsPreRestoreBackup_NilDB(t *testing.T) {
	svc := &BackupService{}
	assert.False(t, svc.isPreRestoreBackup("backup_x.zip"))
}

// TestIsSQLiteTransientRestoreError_TableDriven exercises every branch of
// the small classifier duplicated from the handlers package.
func TestIsSQLiteTransientRestoreError_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"locked", errors.New("database is locked"), true},
		{"busy", errors.New("database is busy"), true},
		{"table locked", errors.New("table is locked"), true},
		{"resource busy", errors.New("resource busy"), true},
		{"other", errors.New("constraint failed"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isSQLiteTransientRestoreError(tt.err))
		})
	}
}

// TestRestoreBackupSafe_LargeDatabaseRoundTrip is required test #6: a
// synthetic >100MB charon.db must back up and restore round-trip without
// hitting the (now-scaled, manifest-declared-size-based) decompression
// cap — the exact gap the flat v1 100MB limit used to block.
func TestRestoreBackupSafe_LargeDatabaseRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	dbPath := filepath.Join(dataDir, "charon.db")
	const paddingBytes = 110 * 1024 * 1024 // > 100MB
	createCharonLikeTestDB(t, dbPath, paddingBytes)

	info, err := os.Stat(dbPath)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(100*1024*1024), "fixture database must exceed the old flat 100MB cap")

	cfg := &config.Config{DatabasePath: dbPath}
	svc := NewBackupService(cfg, nil, nil)
	t.Cleanup(svc.Stop)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.LegacyFormat)
}
