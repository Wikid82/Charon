package services

import (
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

// Package note (spec §3.6): backup archive encryption uses filippo.io/age
// with an scrypt (passphrase) recipient/identity rather than a hand-rolled
// chunked-AES-GCM scheme. age's STREAM construction (64 KiB
// ChaCha20-Poly1305 chunks) streams the whole finished .zip straight to
// disk as `backup_<ts>.zip.age` without ever buffering the full archive in
// RAM — a whole-file crypto.EncryptionService.Encrypt call would do exactly
// that and is unsafe/impractical for multi-hundred-MB archives.

// encryptArchiveWithPassphrase streams the file at srcPath through an
// age/scrypt passphrase recipient into dstPath. The passphrase is used only
// in memory for the duration of this call and is never logged (CLAUDE.md /
// spec §3.9 — not even via util.SanitizeForLog, since it must never reach a
// log call at all).
func encryptArchiveWithPassphrase(srcPath, dstPath, passphrase string) error {
	if passphrase == "" {
		return fmt.Errorf("passphrase is required to encrypt backup")
	}

	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return fmt.Errorf("create age recipient: %w", err)
	}

	src, err := os.Open(srcPath) // #nosec G304 -- srcPath is a server-controlled backup archive path
	if err != nil {
		return fmt.Errorf("open archive for encryption: %w", err)
	}
	defer func() {
		_ = src.Close()
	}()

	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- dstPath is a server-controlled backup archive path
	if err != nil {
		return fmt.Errorf("create encrypted archive: %w", err)
	}
	defer func() {
		_ = dst.Close()
	}()

	w, err := age.Encrypt(dst, recipient)
	if err != nil {
		return fmt.Errorf("initialize age encryption stream: %w", err)
	}

	if _, err := io.Copy(w, src); err != nil {
		return fmt.Errorf("encrypt archive contents: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("finalize encrypted archive: %w", err)
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync encrypted archive: %w", err)
	}

	return nil
}

// decryptArchiveWithPassphrase decrypts an age-encrypted archive at srcPath
// (produced by encryptArchiveWithPassphrase) into dstPath. A wrong
// passphrase or corrupt/tampered ciphertext surfaces as ErrPassphraseInvalid
// so callers can map it to a 400 without leaking implementation details.
// dstPath is removed on any failure so a partially-written, unverified
// plaintext is never left behind for a later step to accidentally trust.
func decryptArchiveWithPassphrase(srcPath, dstPath, passphrase string) (err error) {
	if passphrase == "" {
		return ErrPassphraseRequired
	}

	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return fmt.Errorf("create age identity: %w", err)
	}

	src, err := os.Open(srcPath) // #nosec G304 -- srcPath is a server-controlled backup archive path
	if err != nil {
		return fmt.Errorf("open encrypted archive: %w", err)
	}
	defer func() {
		_ = src.Close()
	}()

	r, err := age.Decrypt(src, identity)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrPassphraseInvalid, err.Error())
	}

	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- dstPath is a server-controlled temp file path
	if err != nil {
		return fmt.Errorf("create decrypted archive: %w", err)
	}
	defer func() {
		_ = dst.Close()
		if err != nil {
			_ = os.Remove(dstPath)
		}
	}()

	if _, err = io.Copy(dst, r); err != nil {
		return fmt.Errorf("%w: %s", ErrPassphraseInvalid, err.Error())
	}

	if err = dst.Sync(); err != nil {
		return fmt.Errorf("sync decrypted archive: %w", err)
	}

	return nil
}
