package crypto

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Column and table names used by the store registry. Queries are built from
// these constants with GORM clause builders; no SQL text is concatenated.
const (
	settingsTable      = "settings"
	settingKeyColumn   = "key"
	settingValueColumn = "value"
	rowIDColumn        = "id"
	settingStorePrefix = "setting:"
)

// EncryptedStore describes one place where a secret is stored encrypted with
// the key service: a table column, or one settings row.
type EncryptedStore struct {
	// Name is the stable identifier used in the API and in logs.
	Name string
	// Table is the database table that holds the cipher column.
	Table string
	// CipherColumn holds the ciphertext.
	CipherColumn string
	// VersionColumn holds the key version tag; empty when the store has none.
	VersionColumn string
	// Scope adds an extra filter to the store's queries, for example one settings key.
	Scope func(tx *gorm.DB) *gorm.DB
}

// hasVersion reports whether the store records a key version per row.
func (s EncryptedStore) hasVersion() bool {
	return s.VersionColumn != ""
}

// Columns returns the columns a write that does not produce new ciphertext must
// never touch: the cipher column and, when present, the version column.
func (s EncryptedStore) Columns() []string {
	if !s.hasVersion() {
		return []string{s.CipherColumn}
	}
	return []string{s.CipherColumn, s.VersionColumn}
}

// EncryptedSettingKeys lists every settings row whose value is encrypted with
// the key service. Each key becomes one store in EncryptedStores.
//
// The backup passphrase key is duplicated from services.SettingKeyBackupEncryptionPassphraseEnc
// because crypto cannot import services; a services test asserts both values match.
var EncryptedSettingKeys = []string{
	"backup.encryption_passphrase_enc",
}

// EncryptedStores is the single list of encrypted secrets. Rotation, status,
// the backup key requirement and the completeness test all read this list.
var EncryptedStores = buildEncryptedStores()

func buildEncryptedStores() []EncryptedStore {
	stores := []EncryptedStore{
		{Name: "dns_providers", Table: "dns_providers", CipherColumn: "credentials_encrypted", VersionColumn: "key_version"},
		{Name: "dns_provider_credentials", Table: "dns_provider_credentials", CipherColumn: "credentials_encrypted", VersionColumn: "key_version"},
		{Name: "remote_storage_targets", Table: "remote_storage_targets", CipherColumn: "secrets_encrypted", VersionColumn: "key_version"},
		{Name: "certificate_keys", Table: "ssl_certificates", CipherColumn: "private_key_enc", VersionColumn: "key_version"},
		{Name: "tunnels", Table: "tunnel_configs", CipherColumn: "encrypted_credentials", VersionColumn: "key_version"},
	}
	for _, key := range EncryptedSettingKeys {
		stores = append(stores, settingStore(key))
	}
	return stores
}

// StoreNamed returns the registry entry with the given name. It panics on an
// unknown name, so a mistyped store name fails at start-up, not at write time.
func StoreNamed(name string) EncryptedStore {
	for _, s := range EncryptedStores {
		if s.Name == name {
			return s
		}
	}
	panic("crypto: unknown encrypted store " + name)
}

// settingStore returns the store for one encrypted settings row. Settings values
// carry no version column.
func settingStore(key string) EncryptedStore {
	return EncryptedStore{
		Name:         settingStorePrefix + key,
		Table:        settingsTable,
		CipherColumn: settingValueColumn,
		Scope: func(tx *gorm.DB) *gorm.DB {
			return tx.Where(clause.Eq{Column: clause.Column{Name: settingKeyColumn}, Value: key})
		},
	}
}

// storedRow is one non-empty ciphertext read from a store.
type storedRow struct {
	ID      uint
	Cipher  string
	Version int // 0 when the store has no version column or the tag is NULL
}

// selectColumns returns the columns read for a store: id, cipher and, when the
// store has one, the version column.
func (s EncryptedStore) selectColumns() []string {
	columns := []string{rowIDColumn, s.CipherColumn}
	if s.hasVersion() {
		columns = append(columns, s.VersionColumn)
	}
	return columns
}

// scoped restricts tx to the store's table and its optional scope.
func (s EncryptedStore) scoped(tx *gorm.DB) *gorm.DB {
	q := tx.Table(s.Table)
	if s.Scope != nil {
		q = s.Scope(q)
	}
	return q
}

// rowsQuery builds the query for every non-empty ciphertext in the store, on tx.
func (s EncryptedStore) rowsQuery(tx *gorm.DB) *gorm.DB {
	return s.nonEmptyCipher(s.scoped(tx).Select(s.selectColumns()))
}

// nonEmptyCipher limits q to rows that hold a ciphertext.
func (s EncryptedStore) nonEmptyCipher(q *gorm.DB) *gorm.DB {
	return q.Where(clause.Neq{Column: clause.Column{Name: s.CipherColumn}, Value: ""})
}

// HasRows reports whether the store is in use. For table stores any row counts,
// empty ciphertext or not, because a row without a secret still records that the
// store is in use. A settings row keeps its key after the value is cleared, so
// only a stored value counts there. The table must exist.
func (s EncryptedStore) HasRows(tx *gorm.DB) (bool, error) {
	q := s.scoped(tx)
	if s.Table == settingsTable {
		q = s.nonEmptyCipher(q)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, fmt.Errorf("count %s: %w", s.Name, err)
	}
	return count > 0, nil
}

// loadRows reads every non-empty ciphertext in the store. The store's table must
// exist (callers check with tableExists).
func (s EncryptedStore) loadRows(tx *gorm.DB) ([]storedRow, error) {
	rows, err := s.rowsQuery(tx).Rows()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []storedRow
	for rows.Next() {
		var row storedRow
		var version *int
		if !s.hasVersion() {
			if err := rows.Scan(&row.ID, &row.Cipher); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&row.ID, &row.Cipher, &version); err != nil {
				return nil, err
			}
			if version != nil {
				row.Version = *version
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// updateRow writes a re-encrypted cipher and, when the store has one, its version
// tag. It uses UpdateColumns, so no hooks run and updated_at is untouched: the
// content did not change, only its encryption.
func (s EncryptedStore) updateRow(tx *gorm.DB, id uint, cipher string, version int) error {
	values := map[string]any{s.CipherColumn: cipher}
	if s.hasVersion() {
		values[s.VersionColumn] = version
	}
	return tx.Table(s.Table).
		Where(clause.Eq{Column: clause.Column{Name: rowIDColumn}, Value: id}).
		UpdateColumns(values).Error
}

// updateVersion writes only the version tag of a row whose ciphertext already
// opens with the target key.
func (s EncryptedStore) updateVersion(tx *gorm.DB, id uint, version int) error {
	return tx.Table(s.Table).
		Where(clause.Eq{Column: clause.Column{Name: rowIDColumn}, Value: id}).
		UpdateColumns(map[string]any{s.VersionColumn: version}).Error
}
