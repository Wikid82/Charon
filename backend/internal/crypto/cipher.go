package crypto

// VersionedCipher protects secrets stored in the database.
type VersionedCipher interface {
	// EncryptWithCurrentKey encrypts with the key new data is written with and
	// returns that key's version.
	EncryptWithCurrentKey(plaintext []byte) (ciphertext string, keyVersion int, err error)
	// DecryptWithVersion decrypts ciphertext tagged with keyVersion (0 = untagged).
	DecryptWithVersion(ciphertext string, keyVersion int) ([]byte, error)
}
