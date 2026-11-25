package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/testutil"
)

// openReferenceParents creates the parent tables used by the proxy_hosts rules, with one row (id 1) in each.
func openReferenceParents(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenMemoryDB(t)
	for _, stmt := range []string{
		"CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY)",
		"CREATE TABLE access_lists (id INTEGER PRIMARY KEY)",
		"CREATE TABLE dns_providers (id INTEGER PRIMARY KEY)",
		"CREATE TABLE proxy_groups (id INTEGER PRIMARY KEY)",
		"CREATE TABLE security_header_profiles (id INTEGER PRIMARY KEY)",
		"INSERT INTO access_lists (id) VALUES (1)",
	} {
		require.NoError(t, db.Exec(stmt).Error, stmt)
	}
	return db
}

func uintPtr(v uint) *uint { return &v }

func TestClearMissingReferences_ClearsOnlyMissingParents(t *testing.T) {
	db := openReferenceParents(t)
	row := map[string]any{
		"certificate_id":             uintPtr(5),   // missing: cleared
		"access_list_id":             uintPtr(1),   // present: kept
		"proxy_group_id":             (*uint)(nil), // unset: not a reference
		"dns_provider_id":            uintPtr(9),   // missing: cleared and DNS challenge off
		"use_dns_challenge":          true,
		"security_header_profile_id": uint(3), // plain value, missing: cleared
	}

	cleared, err := ClearMissingReferences(db, TableProxyHosts, row)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"certificate_id", "dns_provider_id", "security_header_profile_id"}, cleared)
	assert.Nil(t, row["certificate_id"])
	assert.Equal(t, uintPtr(1), row["access_list_id"])
	assert.Nil(t, row["dns_provider_id"])
	assert.Equal(t, false, row["use_dns_challenge"])
	assert.Nil(t, row["security_header_profile_id"])
}

func TestClearMissingReferences_IgnoresOtherTables(t *testing.T) {
	db := openReferenceParents(t)
	row := map[string]any{"certificate_id": uintPtr(5)}

	cleared, err := ClearMissingReferences(db, TableLocations, row)
	require.NoError(t, err)
	assert.Empty(t, cleared)
	assert.Equal(t, uintPtr(5), row["certificate_id"])
}

func TestClearMissingReferences_ReturnsParentLookupError(t *testing.T) {
	db := openReferenceParents(t)
	require.NoError(t, db.Exec("DROP TABLE ssl_certificates").Error)

	_, err := ClearMissingReferences(db, TableProxyHosts, map[string]any{"certificate_id": uintPtr(5)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssl_certificates.id")
}

func TestReferenceRules_EveryRuleHasAShape(t *testing.T) {
	require.Len(t, ReferenceRules, 12)
	for _, rule := range ReferenceRules {
		assert.NotEmpty(t, rule.Child)
		assert.NotEmpty(t, rule.Column)
		assert.NotEmpty(t, rule.Parent)
		assert.NotEmpty(t, rule.ParentColumn)
		assert.Contains(t, []RepairAction{RepairClear, RepairDeleteRow}, rule.Action)
	}
}
