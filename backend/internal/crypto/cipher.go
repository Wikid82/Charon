package crypto

// VersionedCipher protects secrets stored in the database.
type VersionedCipher interface {
	// EncryptWithCurrentKey encrypts with the key new data is written with and
	// returns that key's version.
	EncryptWithCurrentKey(plaintext []byte) (ciphertext string, keyVersion int, err error)
	// DecryptWithVersion decrypts ciphertext tagged with keyVersion (0 = untagged).
	DecryptWithVersion(ciphertext string, keyVersion int) ([]byte, error)
}

// Compile-time check: the rotation service is the production cipher.
var _ VersionedCipher = (*RotationService)(nil)

// singleKeyCipher adapts one EncryptionService to VersionedCipher. It ignores
// version tags and always writes version 1, which is the behaviour of stores
// that predate key versions.
type singleKeyCipher struct {
	svc *EncryptionService
}

// SingleKey wraps svc as a VersionedCipher. A nil svc returns a nil interface
// so callers can test for "no key configured" with a plain nil check.
func SingleKey(svc *EncryptionService) VersionedCipher {
	if svc == nil {
		return nil
	}
	return singleKeyCipher{svc: svc}
}

// EncryptWithCurrentKey encrypts with the single key and reports version 1.
func (c singleKeyCipher) EncryptWithCurrentKey(plaintext []byte) (string, int, error) {
	ciphertext, err := c.svc.Encrypt(plaintext)
	if err != nil {
		return "", 0, err
	}
	return ciphertext, 1, nil
}

// DecryptWithVersion decrypts with the single key. The version tag is ignored.
func (c singleKeyCipher) DecryptWithVersion(ciphertext string, _ int) ([]byte, error) {
	return c.svc.Decrypt(ciphertext)
}
