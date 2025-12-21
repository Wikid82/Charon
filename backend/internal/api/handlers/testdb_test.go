package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestGetTemplateDB tests that the template database is initialized correctly
// and can be retrieved multiple times (singleton pattern).
func TestGetTemplateDB(t *testing.T) {
	// First call should initialize the template
	tmpl1, err1 := GetTemplateDB()
	require.NoError(t, err1, "first GetTemplateDB should succeed")
	require.NotNil(t, tmpl1, "template DB should not be nil")

	// Second call should return the same instance
	tmpl2, err2 := GetTemplateDB()
	require.NoError(t, err2, "second GetTemplateDB should succeed")
	require.NotNil(t, tmpl2, "template DB should not be nil on second call")

	// Both should be the same instance (singleton)
	assert.Equal(t, tmpl1, tmpl2, "GetTemplateDB should return same instance")
}

// TestGetTemplateDB_HasTables verifies the template DB has migrated tables.
func TestGetTemplateDB_HasTables(t *testing.T) {
	tmpl, err := GetTemplateDB()
	require.NoError(t, err)
	require.NotNil(t, tmpl)

	// Check that some key tables exist in the template
	var tables []string
	rows, err := tmpl.Raw("SELECT name FROM sqlite_master WHERE type='table'").Rows()
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}

	// Verify some expected tables exist
	assert.Contains(t, tables, "users", "template should have users table")
	assert.Contains(t, tables, "proxy_hosts", "template should have proxy_hosts table")
	assert.Contains(t, tables, "settings", "template should have settings table")
}

// TestOpenTestDB creates a basic test database.
func TestOpenTestDB(t *testing.T) {
	db := OpenTestDB(t)
	require.NotNil(t, db, "OpenTestDB should return non-nil DB")

	// Verify we can execute basic SQL
	var result int
	err := db.Raw("SELECT 1").Scan(&result).Error
	require.NoError(t, err)
	assert.Equal(t, 1, result)
}

// TestOpenTestDB_Uniqueness ensures each call creates a unique database.
func TestOpenTestDB_Uniqueness(t *testing.T) {
	db1 := OpenTestDB(t)
	db2 := OpenTestDB(t)

	require.NotNil(t, db1)
	require.NotNil(t, db2)

	// Create a table in db1
	err := db1.Exec("CREATE TABLE test_unique (id INTEGER PRIMARY KEY)").Error
	require.NoError(t, err)

	// Insert a row in db1
	err = db1.Exec("INSERT INTO test_unique (id) VALUES (1)").Error
	require.NoError(t, err)

	// db2 should NOT have this table (different database)
	var count int64
	err = db2.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_unique'").Scan(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "db2 should not have db1's table")
}

// TestOpenTestDBWithMigrations tests the function that creates a DB with migrations.
func TestOpenTestDBWithMigrations(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	require.NotNil(t, db, "OpenTestDBWithMigrations should return non-nil DB")

	// Verify that tables were created
	var tables []string
	rows, err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Rows()
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}

	// Should have key tables from the migration
	assert.Contains(t, tables, "users", "should have users table")
	assert.Contains(t, tables, "proxy_hosts", "should have proxy_hosts table")
	assert.Contains(t, tables, "settings", "should have settings table")
}

// TestOpenTestDBWithMigrations_CanInsertData verifies we can actually use the migrated DB.
func TestOpenTestDBWithMigrations_CanInsertData(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	require.NotNil(t, db)

	// Create a user
	user := &models.User{
		UUID:         "test-uuid-123",
		Name:         "testuser",
		Email:        "test@example.com",
		PasswordHash: "fakehash",
	}
	err := db.Create(user).Error
	require.NoError(t, err, "should be able to create user in migrated DB")
	assert.NotZero(t, user.ID, "user should have an ID after creation")

	// Query it back
	var fetched models.User
	err = db.First(&fetched, user.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "testuser", fetched.Name)
}

// TestOpenTestDBWithMigrations_MultipleModels tests various model operations.
func TestOpenTestDBWithMigrations_MultipleModels(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	require.NotNil(t, db)

	// Test ProxyHost
	host := &models.ProxyHost{
		UUID:        "test-host-uuid",
		DomainNames: "example.com",
		ForwardHost: "localhost",
		ForwardPort: 8080,
	}
	err := db.Create(host).Error
	require.NoError(t, err, "should be able to create proxy host")

	// Test Setting
	setting := &models.Setting{
		Key:   "test_key",
		Value: "test_value",
	}
	err = db.Create(setting).Error
	require.NoError(t, err, "should be able to create setting")

	// Verify counts
	var hostCount, settingCount int64
	db.Model(&models.ProxyHost{}).Count(&hostCount)
	db.Model(&models.Setting{}).Count(&settingCount)

	assert.Equal(t, int64(1), hostCount)
	assert.Equal(t, int64(1), settingCount)
}
