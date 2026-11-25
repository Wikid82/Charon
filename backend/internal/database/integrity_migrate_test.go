package database

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/testutil"
)

// referenceSchemaDDL creates the parent and child tables of the references the repair rules cover, with the same
// ON DELETE clauses the models declare.
var referenceSchemaDDL = []string{
	"CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY)",
	"CREATE TABLE access_lists (id INTEGER PRIMARY KEY)",
	"CREATE TABLE dns_providers (id INTEGER PRIMARY KEY)",
	"CREATE TABLE proxy_groups (id INTEGER PRIMARY KEY)",
	"CREATE TABLE security_header_profiles (id INTEGER PRIMARY KEY)",
	"CREATE TABLE backup_records (id INTEGER PRIMARY KEY)",
	"CREATE TABLE remote_storage_targets (id INTEGER PRIMARY KEY)",
	"CREATE TABLE backup_remote_copies (id INTEGER PRIMARY KEY, backup_record_id INTEGER NOT NULL REFERENCES backup_records(id), remote_target_id INTEGER NOT NULL REFERENCES remote_storage_targets(id))",
	"CREATE TABLE dns_provider_credentials (id INTEGER PRIMARY KEY, dns_provider_id INTEGER NOT NULL REFERENCES dns_providers(id))",
	"CREATE TABLE proxy_hosts (id INTEGER PRIMARY KEY, use_dns_challenge INTEGER NOT NULL DEFAULT 0, certificate_id INTEGER REFERENCES ssl_certificates(id), proxy_group_id INTEGER REFERENCES proxy_groups(id), dns_provider_id INTEGER REFERENCES dns_providers(id), security_header_profile_id INTEGER REFERENCES security_header_profiles(id), access_list_id INTEGER REFERENCES access_lists(id))",
	"CREATE TABLE locations (id INTEGER PRIMARY KEY, proxy_host_id INTEGER NOT NULL REFERENCES proxy_hosts(id) ON DELETE CASCADE)",
	"CREATE TABLE redirection_hosts (id INTEGER PRIMARY KEY, certificate_id INTEGER REFERENCES ssl_certificates(id), dns_provider_id INTEGER REFERENCES dns_providers(id), use_dns_challenge INTEGER NOT NULL DEFAULT 0)",
	"CREATE TABLE uptime_monitors (id TEXT PRIMARY KEY, proxy_host_id INTEGER REFERENCES proxy_hosts(id))",
	"CREATE TABLE uptime_heartbeats (id INTEGER PRIMARY KEY, monitor_id TEXT)",
}

// openReferenceSchemaDB opens an enforcement-on database with the reference schema and no rows.
func openReferenceSchemaDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenMemoryDBWithForeignKeys(t)
	for _, stmt := range referenceSchemaDDL {
		require.NoError(t, db.Exec(stmt).Error, stmt)
	}
	return db
}

// foreignKeysOn reads the enforcement setting of the pool.
func foreignKeysOn(t *testing.T, db *gorm.DB) int {
	t.Helper()
	var on int
	require.NoError(t, db.Raw("PRAGMA foreign_keys").Scan(&on).Error)
	return on
}

// TestConnect_EnablesForeignKeysOnEveryConnection covers a replacement connection: the first one is discarded and the
// next one must still have reference enforcement on.
func TestConnect_EnablesForeignKeysOnEveryConnection(t *testing.T) {
	db, err := Connect(filepath.Join(t.TempDir(), "fk.db"))
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	assert.Equal(t, 1, foreignKeysOn(t, db))

	discardPooledConnection(t, sqlDB)

	assert.Equal(t, 1, foreignKeysOn(t, db))
}

// TestPauseForeignKeys_RefusedInsideTransaction covers the one fatal case: enforcement does not change inside an open
// transaction, so the pause reports that the checks could not be paused.
func TestPauseForeignKeys_RefusedInsideTransaction(t *testing.T) {
	db := openReferenceSchemaDB(t)

	err := db.Connection(func(conn *gorm.DB) error {
		require.NoError(t, conn.Exec("BEGIN").Error)
		defer func() { _ = conn.Exec("ROLLBACK").Error }()
		return PauseForeignKeys(conn)
	})

	assert.ErrorIs(t, err, errForeignKeysStillOn)
	assert.Contains(t, err.Error(), "restart Charon, and report this if it happens again")
}

// TestMigrateWithIntegrity_MigrateErrorKeepsEnforcementOn covers a failing migration: its error is returned and
// enforcement is back on afterwards.
func TestMigrateWithIntegrity_MigrateErrorKeepsEnforcementOn(t *testing.T) {
	db := openReferenceSchemaDB(t)
	boom := errors.New("migration boom")

	err := MigrateWithIntegrity(db, func(*gorm.DB) error { return boom })

	assert.ErrorIs(t, err, boom)
	assert.Equal(t, 1, foreignKeysOn(t, db))
}

// TestMigrateWithIntegrity_RepairsAndReportsRemainingViolations covers the normal path: a broken clearable reference is
// cleared, a broken delete-row reference is removed, a violation without a repair rule stays and is reported, and
// enforcement is on afterwards.
func TestMigrateWithIntegrity_RepairsAndReportsRemainingViolations(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("CREATE TABLE tags (id INTEGER PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("CREATE TABLE tag_links (id INTEGER PRIMARY KEY, tag_id INTEGER REFERENCES tags(id))").Error)
	// Breaking the rows needs enforcement off, as a damaged database would have it.
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO proxy_hosts (id, certificate_id) VALUES (1, 77)").Error)
	require.NoError(t, db.Exec("INSERT INTO locations (id, proxy_host_id) VALUES (1, 1), (2, 88)").Error)
	require.NoError(t, db.Exec("INSERT INTO tag_links (id, tag_id) VALUES (1, 5)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	err := MigrateWithIntegrity(db, func(*gorm.DB) error { return nil })
	require.NoError(t, err)

	var host struct{ CertificateID *int64 }
	require.NoError(t, db.Raw("SELECT certificate_id FROM proxy_hosts WHERE id = 1").Scan(&host).Error)
	assert.Nil(t, host.CertificateID, "the missing certificate is cleared, the host is kept")
	var locations int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM locations").Scan(&locations).Error)
	assert.Equal(t, int64(1), locations, "the location of the missing host is removed")

	violations, err := CheckReferences(db)
	require.NoError(t, err)
	assert.Len(t, violations, 1, "the violation with no repair rule remains")
	assert.Equal(t, 1, foreignKeysOn(t, db))
}

// TestMigrateWithIntegrity_RepairFailureRollsBack covers a repair that fails: nothing of it is kept, the migration
// still returns nil, and enforcement is back on.
func TestMigrateWithIntegrity_RepairFailureRollsBack(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO proxy_hosts (id, certificate_id) VALUES (1, 77)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, db.Exec("CREATE TRIGGER refuse_locations_repair BEFORE UPDATE ON proxy_hosts BEGIN SELECT RAISE(ABORT, 'refused by test'); END;").Error)

	err := MigrateWithIntegrity(db, func(*gorm.DB) error { return nil })

	require.NoError(t, err, "a repair failure does not fail the migration")
	var certificateID *int64
	require.NoError(t, db.Raw("SELECT certificate_id FROM proxy_hosts WHERE id = 1").Scan(&certificateID).Error)
	require.NotNil(t, certificateID, "the failed repair changed nothing")
	assert.Equal(t, int64(77), *certificateID)
	assert.Equal(t, 1, foreignKeysOn(t, db))
}

// TestResumeForeignKeys_DiscardsConnectionThatCannotResume covers a connection on which enforcement cannot be turned
// on (an open transaction): the connection is removed from the pool, and the next connection has enforcement on.
func TestResumeForeignKeys_DiscardsConnectionThatCannotResume(t *testing.T) {
	db := openReferenceSchemaDB(t)

	err := db.Connection(func(conn *gorm.DB) error {
		require.NoError(t, PauseForeignKeys(conn))
		require.NoError(t, conn.Exec("BEGIN").Error)
		return ResumeForeignKeys(conn)
	})

	require.Error(t, err)
	assert.Equal(t, 1, foreignKeysOn(t, db), "the replacement connection enforces references")
}

// TestCheckReferences_ReportsBrokenRows covers the check on its own: enforcement off, one broken row.
func TestCheckReferences_ReportsBrokenRows(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO proxy_hosts (id, certificate_id) VALUES (1, 9)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	violations, err := CheckReferences(db)

	require.NoError(t, err)
	require.Len(t, violations, 1)
	assert.Equal(t, "proxy_hosts", violations[0].Table)
	assert.Equal(t, int64(1), violations[0].RowID)
}
