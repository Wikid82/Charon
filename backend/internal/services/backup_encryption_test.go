package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncryptDecryptArchiveWithPassphrase_RoundTrip proves the happy path:
// an archive encrypted with a passphrase decrypts back to byte-identical
// plaintext with the same passphrase (spec §3.6).
func TestEncryptDecryptArchiveWithPassphrase_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "archive.zip")
	encPath := filepath.Join(tmpDir, "archive.zip.age")
	decPath := filepath.Join(tmpDir, "archive.decrypted.zip")

	plaintext := []byte("this is a fake zip archive's bytes for round-trip testing")
	require.NoError(t, os.WriteFile(srcPath, plaintext, 0o600))

	require.NoError(t, encryptArchiveWithPassphrase(srcPath, encPath, "correct horse battery staple"))
	require.FileExists(t, encPath)

	encBytes, err := os.ReadFile(encPath)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encBytes, "encrypted output must not equal the plaintext")

	require.NoError(t, decryptArchiveWithPassphrase(encPath, decPath, "correct horse battery staple"))
	decBytes, err := os.ReadFile(decPath)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decBytes)
}

// TestEncryptArchiveWithPassphrase_EmptyPassphrase proves encryption
// requires a non-empty passphrase.
func TestEncryptArchiveWithPassphrase_EmptyPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "archive.zip")
	require.NoError(t, os.WriteFile(srcPath, []byte("data"), 0o600))

	err := encryptArchiveWithPassphrase(srcPath, filepath.Join(tmpDir, "out.age"), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "passphrase is required")
}

// TestEncryptArchiveWithPassphrase_SourceMissing proves a missing source
// file surfaces a wrapped "open archive for encryption" error.
func TestEncryptArchiveWithPassphrase_SourceMissing(t *testing.T) {
	tmpDir := t.TempDir()
	err := encryptArchiveWithPassphrase(filepath.Join(tmpDir, "does-not-exist.zip"), filepath.Join(tmpDir, "out.age"), "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open archive for encryption")
}

// TestEncryptArchiveWithPassphrase_DestinationUnwritable proves a
// destination path that cannot be created (parent directory missing)
// surfaces a wrapped "create encrypted archive" error.
func TestEncryptArchiveWithPassphrase_DestinationUnwritable(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "archive.zip")
	require.NoError(t, os.WriteFile(srcPath, []byte("data"), 0o600))

	err := encryptArchiveWithPassphrase(srcPath, filepath.Join(tmpDir, "no-such-dir", "out.age"), "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create encrypted archive")
}

// TestEncryptArchiveWithPassphrase_StreamInitFails_DestinationOutOfSpace
// proves the "initialize age encryption stream" error branch: age.Encrypt
// writes its STREAM header to dst as part of setting up the writer, before
// any of the archive's own content is copied, so a destination that always
// rejects writes (Linux's /dev/full — a real device that returns ENOSPC on
// every write(2), the standard technique for deterministically simulating
// a full disk) fails at that header write rather than requiring a real
// disk-space exhaustion race.
func TestEncryptArchiveWithPassphrase_StreamInitFails_DestinationOutOfSpace(t *testing.T) {
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full not available on this platform")
	}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "archive.zip")
	require.NoError(t, os.WriteFile(srcPath, []byte("archive contents"), 0o600))

	err := encryptArchiveWithPassphrase(srcPath, "/dev/full", "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initialize age encryption stream")
}

// NOTE on branches intentionally left uncovered in this file:
//   - age.NewScryptRecipient/age.NewScryptIdentity's own error returns
//     (encrypt ~31-32, decrypt ~83-84): filippo.io/age's scrypt recipient
//     and identity constructors do not reject any non-empty Go string
//     passphrase — both already-empty-string branches are covered above by
//     the ErrPassphraseRequired tests, which is the only input this
//     package ever fails to validate before reaching these constructors.
//   - io.Copy/w.Close/dst.Sync failing *after* a successful stream-header
//     write (encrypt ~56-57, ~60-61, ~64-65; decrypt's dst.Sync ~115-116):
//     /dev/full fails every write including the header, so it cannot
//     isolate a failure to only the later writes/close/sync without a
//     custom capacity-limited io.Writer seam this test-only pass must not
//     add to production code.

// TestDecryptArchiveWithPassphrase_EmptyPassphrase proves decryption
// requires a non-empty passphrase, surfacing ErrPassphraseRequired so the
// handler can map it to a specific error_code.
func TestDecryptArchiveWithPassphrase_EmptyPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	err := decryptArchiveWithPassphrase(filepath.Join(tmpDir, "in.age"), filepath.Join(tmpDir, "out.zip"), "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPassphraseRequired)
}

// TestDecryptArchiveWithPassphrase_SourceMissing proves a missing encrypted
// source file surfaces a wrapped "open encrypted archive" error rather than
// ErrPassphraseInvalid.
func TestDecryptArchiveWithPassphrase_SourceMissing(t *testing.T) {
	tmpDir := t.TempDir()
	err := decryptArchiveWithPassphrase(filepath.Join(tmpDir, "does-not-exist.age"), filepath.Join(tmpDir, "out.zip"), "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open encrypted archive")
}

// TestDecryptArchiveWithPassphrase_WrongPassphrase proves a wrong
// passphrase against a genuinely encrypted archive surfaces
// ErrPassphraseInvalid (mapped to a 400 by the handler) and leaves no
// partial plaintext file behind.
func TestDecryptArchiveWithPassphrase_WrongPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "archive.zip")
	encPath := filepath.Join(tmpDir, "archive.zip.age")
	decPath := filepath.Join(tmpDir, "archive.decrypted.zip")

	require.NoError(t, os.WriteFile(srcPath, []byte("plaintext"), 0o600))
	require.NoError(t, encryptArchiveWithPassphrase(srcPath, encPath, "correct-passphrase"))

	err := decryptArchiveWithPassphrase(encPath, decPath, "wrong-passphrase")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPassphraseInvalid)
	assert.NoFileExists(t, decPath, "a failed decryption must not leave a partial plaintext file behind")
}

// TestDecryptArchiveWithPassphrase_CorruptCiphertext proves a corrupted
// (but not simply wrong-passphrase) ciphertext also maps to
// ErrPassphraseInvalid rather than leaking an internal age error, and
// cleans up any partially-written destination file.
func TestDecryptArchiveWithPassphrase_CorruptCiphertext(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "archive.zip")
	encPath := filepath.Join(tmpDir, "archive.zip.age")
	decPath := filepath.Join(tmpDir, "archive.decrypted.zip")

	require.NoError(t, os.WriteFile(srcPath, []byte("plaintext"), 0o600))
	require.NoError(t, encryptArchiveWithPassphrase(srcPath, encPath, "correct-passphrase"))

	// Corrupt the ciphertext after the fact.
	encBytes, err := os.ReadFile(encPath)
	require.NoError(t, err)
	require.NotEmpty(t, encBytes)
	encBytes[len(encBytes)-1] ^= 0xFF
	require.NoError(t, os.WriteFile(encPath, encBytes, 0o600))

	err = decryptArchiveWithPassphrase(encPath, decPath, "correct-passphrase")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPassphraseInvalid)
	assert.NoFileExists(t, decPath)
}

// TestDecryptArchiveWithPassphrase_DestinationUnwritable proves a
// destination path that cannot be created surfaces a wrapped
// "create decrypted archive" error.
func TestDecryptArchiveWithPassphrase_DestinationUnwritable(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "archive.zip")
	encPath := filepath.Join(tmpDir, "archive.zip.age")
	require.NoError(t, os.WriteFile(srcPath, []byte("plaintext"), 0o600))
	require.NoError(t, encryptArchiveWithPassphrase(srcPath, encPath, "correct-passphrase"))

	err := decryptArchiveWithPassphrase(encPath, filepath.Join(tmpDir, "no-such-dir", "out.zip"), "correct-passphrase")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create decrypted archive")
}
