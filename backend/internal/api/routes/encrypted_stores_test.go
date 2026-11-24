package routes

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
)

// encryptedColumnExclusions are columns whose names look encrypted but use a
// different key or are not a secret: crowdsec console enrollment (its own key,
// never read back) and the backup record's boolean flag.
var encryptedColumnExclusions = map[string]bool{
	"crowdsec_console_enrollments.encrypted_enroll_key": true,
	"backup_records.encrypted":                          true,
}

// newMigratedDB opens a temporary SQLite database and runs the full schema migration.
func newMigratedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "schema.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(db))
	return db
}

// tableNames lists every user table in the database.
func tableNames(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var names []string
	require.NoError(t, db.Raw("SELECT name FROM sqlite_master WHERE type = ? AND name NOT LIKE ?", "table", "sqlite_%").Scan(&names).Error)
	return names
}

// columnNames lists the columns of table in declaration order.
func columnNames(t *testing.T, db *gorm.DB, table string) []string {
	t.Helper()
	var names []string
	require.NoError(t, db.Raw("SELECT name FROM pragma_table_info(?) ORDER BY cid", table).Scan(&names).Error)
	return names
}

// TestEncryptedStores_EveryEncryptedColumnIsRegistered walks the migrated schema.
// Any column whose name marks it as ciphertext must be a registered cipher column
// or appear in the explicit exclusion list, so a future encrypted column cannot be
// left out of rotation.
func TestEncryptedStores_EveryEncryptedColumnIsRegistered(t *testing.T) {
	db := newMigratedDB(t)

	registered := map[string]bool{}
	for _, s := range crypto.EncryptedStores {
		registered[s.Table+"."+s.CipherColumn] = true
	}

	var missing []string
	for _, table := range tableNames(t, db) {
		for _, column := range columnNames(t, db, table) {
			if !strings.Contains(column, "_enc") && !strings.Contains(column, "encrypted") {
				continue
			}
			key := table + "." + column
			if registered[key] || encryptedColumnExclusions[key] {
				continue
			}
			missing = append(missing, key)
		}
	}
	assert.Empty(t, missing, "encrypted columns missing from crypto.EncryptedStores or the exclusion list")
}

// TestEncryptedStores_SettingKeysMatchServiceConstants parses the non-test services
// sources. Every string constant that names an encrypted settings row must be in
// crypto.EncryptedSettingKeys.
func TestEncryptedStores_SettingKeysMatchServiceConstants(t *testing.T) {
	dir := filepath.Join("..", "..", "services")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	fset := token.NewFileSet()
	found := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		require.NoError(t, parseErr)
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(lit.Value)
			if unquoteErr == nil && strings.HasSuffix(value, "_enc") {
				found[value] = true
			}
			return true
		})
	}

	registered := map[string]bool{}
	for _, key := range crypto.EncryptedSettingKeys {
		registered[key] = true
	}
	for key := range found {
		assert.True(t, registered[key], "settings key %q is encrypted but not in crypto.EncryptedSettingKeys", key)
	}
	assert.NotEmpty(t, found, "the services sources should name at least one encrypted settings key")
}

// oldTunnelConfig is the tunnel_configs table as the previous release created it,
// before KeyVersion existed.
type oldTunnelConfig struct {
	ID                   uint   `gorm:"primaryKey"`
	UUID                 string `gorm:"uniqueIndex;not null"`
	Name                 string `gorm:"not null;index"`
	Provider             string `gorm:"not null;index"`
	EncryptedCredentials string `gorm:"type:text;not null"`
	Configuration        string `gorm:"type:text"`
	IsActive             bool   `gorm:"default:false;index"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (oldTunnelConfig) TableName() string { return "tunnel_configs" }

// TestTunnelConfigColumnOrderMatchesAfterUpgrade pins the positional-copy contract:
// a fresh database and one upgraded from the previous release must list the
// tunnel_configs columns in the same order (live restore copies positionally).
func TestTunnelConfigColumnOrderMatchesAfterUpgrade(t *testing.T) {
	fresh := newMigratedDB(t)
	freshColumns := columnNames(t, fresh, "tunnel_configs")
	require.Contains(t, freshColumns, "key_version")
	assert.Equal(t, "key_version", freshColumns[len(freshColumns)-1], "key_version must be the last column")

	upgraded, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "upgraded.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, upgraded.Migrator().CreateTable(&oldTunnelConfig{}))
	require.NotContains(t, columnNames(t, upgraded, "tunnel_configs"), "key_version")
	require.NoError(t, MigrateSchema(upgraded))

	assert.Equal(t, freshColumns, columnNames(t, upgraded, "tunnel_configs"))
	assert.Contains(t, tableNames(t, upgraded), "encryption_key_versions")
}
