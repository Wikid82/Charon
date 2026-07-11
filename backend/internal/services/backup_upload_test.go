package services

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawSQLiteBytes is a minimal but genuine "SQLite format 3\x00"-prefixed
// payload — WrapRawDatabaseAsBackup never parses the bytes itself (that's
// the downstream sanity check's job), it only wraps whatever the caller
// hands it, so a magic-header-prefixed blob is sufficient here.
func rawSQLiteBytes() []byte {
	payload := append([]byte{}, sqliteMagicHeaderForTest...)
	payload = append(payload, []byte("charon-test-raw-db-payload")...)
	return payload
}

var sqliteMagicHeaderForTest = []byte("SQLite format 3\x00")

// TestWrapRawDatabaseAsBackup_HappyPath proves a raw uploaded .db payload is
// wrapped into a valid format-v2 archive: manifest.json + the database entry
// under s.DatabaseName, with FormatVersion 2 and BackupType "uploaded".
func TestWrapRawDatabaseAsBackup_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	s := &BackupService{
		BackupDir:    filepath.Join(tmpDir, "backups"),
		DatabaseName: "charon.db",
	}

	raw := rawSQLiteBytes()
	filename, err := s.WrapRawDatabaseAsBackup(raw, "2026-07-10_12-00-00")
	require.NoError(t, err)
	assert.Equal(t, "uploaded_2026-07-10_12-00-00.zip", filename)

	zipPath := filepath.Join(s.BackupDir, filename)
	require.FileExists(t, zipPath)

	r, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	var manifestFile, dbFile *zip.File
	for _, f := range r.File {
		switch f.Name {
		case "manifest.json":
			manifestFile = f
		case "charon.db":
			dbFile = f
		}
	}
	require.NotNil(t, manifestFile, "wrapped archive must contain manifest.json")
	require.NotNil(t, dbFile, "wrapped archive must contain the database entry under DatabaseName")

	// The database entry must round-trip the exact raw bytes uploaded.
	rc, err := dbFile.Open()
	require.NoError(t, err)
	content, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, raw, content)

	mrc, err := manifestFile.Open()
	require.NoError(t, err)
	manifestBytes, err := io.ReadAll(mrc)
	require.NoError(t, err)
	require.NoError(t, mrc.Close())

	var manifest BackupManifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	assert.Equal(t, 2, manifest.FormatVersion)
	assert.Equal(t, "uploaded", manifest.BackupType)
	assert.Equal(t, "charon.db", manifest.DatabaseName)
	assert.False(t, manifest.CreatedAt.IsZero())
	require.Len(t, manifest.Contents, 1, "manifest must track the wrapped database entry (checksum, size)")
	assert.Equal(t, "charon.db", manifest.Contents[0].Path)
	assert.Equal(t, int64(len(raw)), manifest.Contents[0].Size)
	assert.NotEmpty(t, manifest.Contents[0].SHA256)

	// No leftover temp file for the uploaded raw database.
	entries, err := os.ReadDir(os.TempDir())
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), "charon-upload-raw-db-", "temp upload file must be cleaned up")
	}
}

// TestWrapRawDatabaseAsBackup_BackupDirCannotBeCreated proves the
// os.MkdirAll(s.BackupDir) failure path (a regular file occupying the path
// a directory needs to exist at) surfaces a wrapped error rather than
// panicking or silently succeeding.
func TestWrapRawDatabaseAsBackup_BackupDirCannotBeCreated(t *testing.T) {
	tmpDir := t.TempDir()
	// Occupy the would-be BackupDir path with a regular file so MkdirAll
	// fails with ENOTDIR.
	blockerPath := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.WriteFile(blockerPath, []byte("not a directory"), 0o600))

	s := &BackupService{
		BackupDir:    blockerPath,
		DatabaseName: "charon.db",
	}

	filename, err := s.WrapRawDatabaseAsBackup(rawSQLiteBytes(), "2026-07-10_12-00-00")
	require.Error(t, err)
	assert.Empty(t, filename)
	assert.Contains(t, err.Error(), "ensure backup directory")
}

// TestWrapRawDatabaseAsBackup_TempFileCreateFails proves a failure to
// create the temp file staging the uploaded bytes (simulated via an
// unwritable TMPDIR) is wrapped and surfaced rather than silently ignored.
func TestWrapRawDatabaseAsBackup_TempFileCreateFails(t *testing.T) {
	tmpDir := t.TempDir()
	s := &BackupService{
		BackupDir:    filepath.Join(tmpDir, "backups"),
		DatabaseName: "charon.db",
	}

	// Point TMPDIR at a path that does not exist so os.CreateTemp fails.
	t.Setenv("TMPDIR", filepath.Join(tmpDir, "no-such-tmp-dir"))

	filename, err := s.WrapRawDatabaseAsBackup(rawSQLiteBytes(), "2026-07-10_12-00-00")
	require.Error(t, err)
	assert.Empty(t, filename)
	assert.Contains(t, err.Error(), "create temp file for uploaded database")
}

// TestWrapRawDatabaseAsBackup_ArchiveCreateFails proves a failure to create
// the destination archive file (BackupDir made read-only) is wrapped and
// surfaced.
func TestWrapRawDatabaseAsBackup_ArchiveCreateFails(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0o700))

	s := &BackupService{
		BackupDir:    backupDir,
		DatabaseName: "charon.db",
	}

	require.NoError(t, os.Chmod(backupDir, 0o500)) // read+execute only, no write
	t.Cleanup(func() { _ = os.Chmod(backupDir, 0o700) })

	filename, err := s.WrapRawDatabaseAsBackup(rawSQLiteBytes(), "2026-07-10_12-00-00")
	require.Error(t, err)
	assert.Empty(t, filename)
	assert.Contains(t, err.Error(), "create wrapped archive")
}

// TestSHA256File_ExportedWrapper proves the small exported SHA256File
// wrapper (used by handlers to hash already-persisted upload files) matches
// the package-private sha256File implementation.
func TestSHA256File_ExportedWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sample.bin")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o600))

	sum, size, err := SHA256File(path)
	require.NoError(t, err)
	assert.Equal(t, int64(len("hello world")), size)
	assert.NotEmpty(t, sum)

	directSum, directSize, err := sha256File(path)
	require.NoError(t, err)
	assert.Equal(t, directSum, sum)
	assert.Equal(t, directSize, size)
}
