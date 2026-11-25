package services

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
)

// encryptSecret seals plaintext with the injected cipher and returns the
// ciphertext with the key version it was written under. A nil cipher means no
// encryption key is configured; it is reported as an error, never a panic.
func encryptSecret(cipher crypto.VersionedCipher, plaintext []byte) (ciphertext string, keyVersion int, err error) {
	if cipher == nil {
		return "", 0, fmt.Errorf("%w: %w", ErrEncryptionFailed, ErrEncryptionKeyMissing)
	}
	ciphertext, keyVersion, err = cipher.EncryptWithCurrentKey(plaintext)
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

// Encrypted stores written by the services below, looked up once from the registry.
var (
	dnsProviderStore    = crypto.StoreNamed("dns_providers")
	dnsCredentialStore  = crypto.StoreNamed("dns_provider_credentials")
	remoteTargetStore   = crypto.StoreNamed("remote_storage_targets")
	certificateKeyStore = crypto.StoreNamed("certificate_keys")
	tunnelStore         = crypto.StoreNamed("tunnels")
)

// saveRow saves every column of row except the store's secret columns, unless
// the write produced new ciphertext in the same call (reencrypted). A write that
// only changes other fields therefore cannot put back a ciphertext or key version
// that was loaded before a rotation.
func saveRow(db *gorm.DB, store crypto.EncryptedStore, row any, reencrypted bool) error {
	q := db
	if !reencrypted {
		q = q.Omit(store.Columns()...)
	}
	return q.Save(row).Error
}

// saveFields writes only the named columns of row, for writers that change
// their secret columns together with a few fields.
func saveFields(db *gorm.DB, row any, columns ...string) error {
	return db.Select(columns).Updates(row).Error
}

// oauthWriteColumns lists the columns an OAuth status write sets. The secret
// columns are included only when the same call produced new ciphertext.
func oauthWriteColumns(reencrypted bool) []string {
	columns := []string{"OAuthStatus", "OAuthConnectedAt"}
	if reencrypted {
		columns = append(columns, remoteTargetStore.Columns()...)
	}
	return columns
}

// writeColumns returns the columns a re-encrypting write sets: the fields the
// request can change, plus the store's cipher and version columns when the write
// produced new ciphertext. Without reencrypted those two columns are never named,
// so a stale row cannot bring back an old ciphertext.
func writeColumns(store crypto.EncryptedStore, reencrypted bool, fields ...string) []string {
	columns := append([]string{"UpdatedAt"}, fields...)
	if reencrypted {
		columns = append(columns, store.Columns()...)
	}
	return columns
}
