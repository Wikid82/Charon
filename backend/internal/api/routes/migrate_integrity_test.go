package routes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/models"
)

const previousReleaseWithoutCertificateFK = "testdata/schema_previous_release_no_cert_fk.sql"

// loadFixture runs every statement of a schema dump on db. Statements end with ";" at the end of a line.
func loadFixture(t *testing.T, db *gorm.DB, path string) {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	for _, stmt := range strings.Split(string(fixture), ";\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		require.NoErrorf(t, db.Exec(stmt).Error, "load fixture statement %q", stmt)
	}
}

// openConnectedPreviousRelease opens a database with the production connection settings and the previous release
// schema. Enforcement is switched off for the setup only, so broken rows can be written.
func openConnectedPreviousRelease(t *testing.T, fixture string) *gorm.DB {
	t.Helper()
	db, err := database.Connect(filepath.Join(t.TempDir(), "charon.db"))
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	loadFixture(t, db, fixture)
	return db
}

// insertBrokenRows writes one row for every declared reference that is broken, plus one intact host that must survive
// the repair unchanged. Enforcement is off while they are written, as a damaged database has it.
func insertBrokenRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	stmts := []string{
		"PRAGMA foreign_keys = OFF",
		"INSERT INTO proxy_hosts (id, uuid, domain_names, forward_host, forward_port, certificate_id, access_list_id, security_header_profile_id, dns_provider_id, use_dns_challenge, proxy_group_id) VALUES (1, 'broken', 'broken.example.com', 'h', 80, 5, 6, 7, 8, 1, 9)",
		"INSERT INTO proxy_hosts (id, uuid, domain_names, forward_host, forward_port, certificate_id, use_dns_challenge) VALUES (2, 'intact', 'intact.example.com', 'h', 80, NULL, 0)",
		"INSERT INTO locations (uuid, proxy_host_id, path, forward_host, forward_port) VALUES ('loc-missing', 99, '/', 'h', 80)",
		"INSERT INTO locations (uuid, proxy_host_id, path, forward_host, forward_port) VALUES ('loc-kept', 2, '/', 'h', 80)",
		"INSERT INTO dns_provider_credentials (uuid, dns_provider_id, label, credentials_encrypted) VALUES ('cred-missing', 8, 'gone', 'x')",
		"INSERT INTO backup_remote_copies (backup_record_id, remote_target_id) VALUES (42, 43)",
		"INSERT INTO uptime_monitors (id, proxy_host_id) VALUES ('mon-missing', 99)",
		"INSERT INTO uptime_heartbeats (monitor_id, status) VALUES ('mon-missing', 'up')",
		"INSERT INTO redirection_hosts (uuid, domain_names, target_url, certificate_id, dns_provider_id, use_dns_challenge) VALUES ('redir', 'redir.example.com', 'https://t.example.com', 5, 8, 1)",
		"PRAGMA foreign_keys = ON",
	}
	for _, stmt := range stmts {
		require.NoErrorf(t, db.Exec(stmt).Error, "insert %q", stmt)
	}
}

// TestMigrateSchema_RepairsReferencesOnPreviousReleaseDatabase covers a previous-release database with broken rows for
// the declared references: each one is cleared or removed as its rule says, the intact host is unchanged, and
// enforcement is on afterwards.
func TestMigrateSchema_RepairsReferencesOnPreviousReleaseDatabase(t *testing.T) {
	db := openConnectedPreviousRelease(t, previousReleaseSchemaFixture)
	insertBrokenRows(t, db)

	require.NoError(t, MigrateSchema(db))

	var broken struct {
		CertificateID           *int64
		AccessListID            *int64
		SecurityHeaderProfileID *int64
		DNSProviderID           *int64
		ProxyGroupID            *int64
		UseDNSChallenge         bool
	}
	require.NoError(t, db.Raw(`SELECT certificate_id, access_list_id, security_header_profile_id, dns_provider_id,
		proxy_group_id, use_dns_challenge FROM proxy_hosts WHERE id = 1`).Scan(&broken).Error)
	assert.Nil(t, broken.CertificateID)
	assert.Nil(t, broken.AccessListID)
	assert.Nil(t, broken.SecurityHeaderProfileID)
	assert.Nil(t, broken.DNSProviderID)
	assert.Nil(t, broken.ProxyGroupID)
	assert.False(t, broken.UseDNSChallenge, "clearing the DNS provider turns the DNS challenge off")

	var intactDomain string
	require.NoError(t, db.Raw("SELECT domain_names FROM proxy_hosts WHERE id = 2").Scan(&intactDomain).Error)
	assert.Equal(t, "intact.example.com", intactDomain)

	assert.Equal(t, int64(1), countRows(t, db, "SELECT COUNT(*) FROM locations WHERE uuid = 'loc-kept'"))
	assert.Zero(t, countRows(t, db, "SELECT COUNT(*) FROM locations WHERE uuid = 'loc-missing'"))
	assert.Zero(t, countRows(t, db, "SELECT COUNT(*) FROM dns_provider_credentials"))
	assert.Zero(t, countRows(t, db, "SELECT COUNT(*) FROM backup_remote_copies"))
	assert.Zero(t, countRows(t, db, "SELECT COUNT(*) FROM uptime_monitors"))
	assert.Zero(t, countRows(t, db, "SELECT COUNT(*) FROM uptime_heartbeats"))

	var redirect struct {
		CertificateID   *int64
		DNSProviderID   *int64
		UseDNSChallenge bool
	}
	require.NoError(t, db.Raw("SELECT certificate_id, dns_provider_id, use_dns_challenge FROM redirection_hosts").Scan(&redirect).Error)
	assert.Nil(t, redirect.CertificateID)
	assert.Nil(t, redirect.DNSProviderID)
	assert.False(t, redirect.UseDNSChallenge)

	violations, err := database.CheckReferences(db)
	require.NoError(t, err)
	assert.Empty(t, violations)
	var enforced int
	require.NoError(t, db.Raw("PRAGMA foreign_keys").Scan(&enforced).Error)
	assert.Equal(t, 1, enforced)
}

// TestMigrateSchema_RebuildKeepsChildRows covers a migration that rebuilds proxy_hosts (the fixture variant has no
// certificate constraint, so the schema adds it): the location rows survive because enforcement is paused.
func TestMigrateSchema_RebuildKeepsChildRows(t *testing.T) {
	rec := &statementRecorder{}
	db := openConnectedPreviousRelease(t, previousReleaseWithoutCertificateFK)
	db = db.Session(&gorm.Session{Logger: rec})
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO proxy_hosts (id, uuid, domain_names, forward_host, forward_port) VALUES (1, 'host', 'host.example.com', 'h', 80)").Error)
	require.NoError(t, db.Exec("INSERT INTO locations (uuid, proxy_host_id, path, forward_host, forward_port) VALUES ('loc', 1, '/', 'h', 80)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	require.NoError(t, MigrateSchema(db))

	assert.True(t, containsStatement(rec.take(), "CREATE TABLE `proxy_hosts__temp`"), "the migration rebuilds proxy_hosts")
	assert.Equal(t, int64(1), countRows(t, db, "SELECT COUNT(*) FROM locations WHERE uuid = 'loc'"))
	assert.Equal(t, int64(1), countRows(t, db, "SELECT COUNT(*) FROM proxy_hosts WHERE uuid = 'host'"))
	assert.True(t, referenceDeclared(t, db, "proxy_hosts", "certificate_id"), "the added constraint is present afterwards")
}

// TestMigrateSchema_ControlRebuildWithChecksOnRemovesChildRows documents why the migration pauses enforcement: the
// same rebuild with enforcement on removes the location rows through the cascade.
func TestMigrateSchema_ControlRebuildWithChecksOnRemovesChildRows(t *testing.T) {
	db := openConnectedPreviousRelease(t, previousReleaseWithoutCertificateFK)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO proxy_hosts (id, uuid, domain_names, forward_host, forward_port) VALUES (1, 'host', 'host.example.com', 'h', 80)").Error)
	require.NoError(t, db.Exec("INSERT INTO locations (uuid, proxy_host_id, path, forward_host, forward_port) VALUES ('loc', 1, '/', 'h', 80)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}))

	assert.Zero(t, countRows(t, db, "SELECT COUNT(*) FROM locations WHERE uuid = 'loc'"), "the cascade removes the location")
}

// countRows runs a COUNT query and returns its value.
func countRows(t *testing.T, db *gorm.DB, query string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(query).Scan(&n).Error, query)
	return n
}

// referenceDeclared reports whether table declares a reference from column.
func referenceDeclared(t *testing.T, db *gorm.DB, table, column string) bool {
	t.Helper()
	for _, fk := range foreignKeysOf(t, db, table) {
		if fk.From == column {
			return true
		}
	}
	return false
}

// declaredForeignKey is one row of PRAGMA foreign_key_list.
type declaredForeignKey struct {
	Table, From, Parent, To string
}

// foreignKeysOf lists the references declared on table.
func foreignKeysOf(t *testing.T, db *gorm.DB, table string) []declaredForeignKey {
	t.Helper()
	rows, err := db.Raw(fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteIdent(table))).Rows()
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []declaredForeignKey
	for rows.Next() {
		var id, seq int
		var parent, from, to, onUpdate, onDelete, match string
		var toNull *string
		require.NoError(t, rows.Scan(&id, &seq, &parent, &from, &toNull, &onUpdate, &onDelete, &match))
		if toNull != nil {
			to = *toNull
		}
		out = append(out, declaredForeignKey{Table: table, From: from, Parent: parent, To: to})
	}
	require.NoError(t, rows.Err())
	return out
}

// quoteIdent quotes a table name for a PRAGMA argument. Only names from this package's own lists are passed.
func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
