package routes

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/database"
)

// TestReferenceRules_MatchDeclaredReferences pins that every reference declared after MigrateSchema has exactly one
// repair rule, and every rule names a declared reference.
func TestReferenceRules_MatchDeclaredReferences(t *testing.T) {
	db, err := database.Connect(filepath.Join(t.TempDir(), "rules.db"))
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, MigrateSchema(db))

	declared := declaredReferences(t, db)
	ruled := make([]string, 0, len(database.ReferenceRules))
	for _, rule := range database.ReferenceRules {
		ruled = append(ruled, referenceKey(rule.Child, rule.Column, rule.Parent, rule.ParentColumn))
	}
	sort.Strings(ruled)

	assert.Equal(t, declared, ruled, "each declared reference needs exactly one rule, and each rule a declared reference")
}

// TestReferenceRules_MatchPreviousReleaseDatabase runs the same comparison on a migrated previous-release database.
func TestReferenceRules_MatchPreviousReleaseDatabase(t *testing.T) {
	db := openConnectedPreviousRelease(t, previousReleaseSchemaFixture)
	require.NoError(t, MigrateSchema(db))

	ruled := make([]string, 0, len(database.ReferenceRules))
	for _, rule := range database.ReferenceRules {
		ruled = append(ruled, referenceKey(rule.Child, rule.Column, rule.Parent, rule.ParentColumn))
	}
	sort.Strings(ruled)
	assert.Equal(t, declaredReferences(t, db), ruled)
}

// referenceKey names one reference as child.column -> parent.column.
func referenceKey(child, column, parent, parentColumn string) string {
	return child + "." + column + "->" + parent + "." + parentColumn
}

// declaredReferences lists every reference declared in the schema, as sorted keys.
func declaredReferences(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw("SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&tables).Error)
	var keys []string
	for _, table := range tables {
		for _, fk := range foreignKeysOf(t, db, table) {
			keys = append(keys, referenceKey(table, fk.From, fk.Parent, fk.To))
		}
	}
	sort.Strings(keys)
	return keys
}
