// Package cryptotest provides test doubles for the crypto package.
package cryptotest

import (
	"fmt"

	"github.com/Wikid82/charon/backend/internal/crypto"
)

// singleKeyCipher adapts one EncryptionService to crypto.VersionedCipher. It
// ignores version tags and always writes version 1, the behaviour of stores
// that predate key versions. Tests use it where a single key is enough.
type singleKeyCipher struct {
	svc *crypto.EncryptionService
}

// SingleKey wraps svc as a crypto.VersionedCipher. A nil svc returns a nil
// interface so callers can test for "no key configured" with a plain nil check.
func SingleKey(svc *crypto.EncryptionService) crypto.VersionedCipher {
	if svc == nil {
		return nil
	}
	return singleKeyCipher{svc: svc}
}

// EncryptWithCurrentKey encrypts with the single key and reports version 1.
func (c singleKeyCipher) EncryptWithCurrentKey(plaintext []byte) (ciphertext string, keyVersion int, err error) {
	ciphertext, err = c.svc.Encrypt(plaintext)
	if err != nil {
		return "", 0, fmt.Errorf("single key encrypt: %w", err)
	}
	return ciphertext, 1, nil
}

// DecryptWithVersion decrypts with the single key. The version tag is ignored.
func (c singleKeyCipher) DecryptWithVersion(ciphertext string, _ int) ([]byte, error) {
	plaintext, err := c.svc.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("single key decrypt: %w", err)
	}
	return plaintext, nil
}
