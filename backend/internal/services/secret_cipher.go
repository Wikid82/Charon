package services

import (
	"fmt"

	"github.com/Wikid82/charon/backend/internal/crypto"
)

// encryptSecret seals plaintext with the injected cipher and returns the
// ciphertext with the key version it was written under. A nil cipher means no
// encryption key is configured; it is reported as an error, never a panic.
func encryptSecret(cipher crypto.VersionedCipher, plaintext []byte) (string, int, error) {
	if cipher == nil {
		return "", 0, fmt.Errorf("%w: %w", ErrEncryptionFailed, ErrEncryptionKeyMissing)
	}
	ciphertext, keyVersion, err := cipher.EncryptWithCurrentKey(plaintext)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}
	return ciphertext, keyVersion, nil
}

// decryptSecret opens ciphertext tagged with keyVersion using the injected
// cipher. A nil cipher means no encryption key is configured.
func decryptSecret(cipher crypto.VersionedCipher, ciphertext string, keyVersion int) ([]byte, error) {
	if cipher == nil {
		return nil, ErrEncryptionKeyMissing
	}
	return cipher.DecryptWithVersion(ciphertext, keyVersion)
}
