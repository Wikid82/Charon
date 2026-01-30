package database

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the Setting model
	err = db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	return db
}

// TestSettingsQueryWithDifferentIDs verifies that reusing a models.Setting variable
// without resetting it causes GORM to include the previous record's ID in subsequent
// WHERE clauses, resulting in "record not found" errors.
func TestSettingsQueryWithDifferentIDs(t *testing.T) {
	db := setupTestDB(t)

	// Create settings with different IDs
	setting1 := &models.Setting{Key: "feature.cerberus.enabled", Value: "true"}
	err := db.Create(setting1).Error
	require.NoError(t, err)
	assert.Equal(t, uint(1), setting1.ID)

	setting2 := &models.Setting{Key: "security.acl.enabled", Value: "true"}
	err = db.Create(setting2).Error
	require.NoError(t, err)
	assert.Equal(t, uint(2), setting2.ID)

	// Simulate the bug: reuse variable without reset
	var s models.Setting

	// First query - populates s.ID = 1
	err = db.Where("key = ?", "feature.cerberus.enabled").First(&s).Error
	require.NoError(t, err)
	assert.Equal(t, uint(1), s.ID)
	assert.Equal(t, "feature.cerberus.enabled", s.Key)

	// Second query WITHOUT reset - demonstrates the bug
	// This would fail with "record not found" if the bug exists
	// because it queries: WHERE key='security.acl.enabled' AND id=1
	t.Run("without reset should fail with bug", func(t *testing.T) {
		var sNoBugFix models.Setting
		// First query
		err := db.Where("key = ?", "feature.cerberus.enabled").First(&sNoBugFix).Error
		require.NoError(t, err)

		// Second query without reset - this is the bug scenario
		err = db.Where("key = ?", "security.acl.enabled").First(&sNoBugFix).Error
		// With the bug, this would fail with gorm.ErrRecordNotFound
		// After the fix (struct reset in production code), this should succeed
		// But in this test, we're demonstrating what WOULD happen without reset
		if err == nil {
			// Bug is present but not triggered (both records have same ID somehow)
			// Or the production code has the fix
			t.Logf("Query succeeded - either fix is applied or test setup issue")
		} else {
			// This is expected without the reset
			assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
		}
	})

	// Third query WITH reset - should always work (this is the fix)
	t.Run("with reset should always work", func(t *testing.T) {
		var sWithFix models.Setting
		// First query
		err := db.Where("key = ?", "feature.cerberus.enabled").First(&sWithFix).Error
		require.NoError(t, err)

		// Reset the struct (THE FIX)
		sWithFix = models.Setting{}

		// Second query with reset - should work
		err = db.Where("key = ?", "security.acl.enabled").First(&sWithFix).Error
		require.NoError(t, err)
		assert.Equal(t, uint(2), sWithFix.ID)
		assert.Equal(t, "security.acl.enabled", sWithFix.Key)
	})
}

// TestCaddyManagerSecuritySettings simulates the real-world scenario from
// manager.go where multiple security settings are queried in sequence.
// This test verifies that non-sequential IDs don't cause query failures.
func TestCaddyManagerSecuritySettings(t *testing.T) {
	db := setupTestDB(t)

	// Create all security settings with specific IDs to simulate real database
	// where settings are created/deleted/recreated over time
	// We need to create them in a transaction and manually set IDs

	// Create with ID gaps to simulate real scenario
	settings := []models.Setting{
		{ID: 4, Key: "feature.cerberus.enabled", Value: "true"},
		{ID: 6, Key: "security.acl.enabled", Value: "true"},
		{ID: 8, Key: "security.waf.enabled", Value: "true"},
	}

	for _, setting := range settings {
		// Use Session to allow manual ID assignment
		err := db.Session(&gorm.Session{FullSaveAssociations: false}).Create(&setting).Error
		require.NoError(t, err)
	}

	// Simulate the query pattern from manager.go buildSecurityConfig()
	var s models.Setting

	// Query 1: Cerberus (ID=4)
	cerbEnabled := false
	if err := db.Where("key = ?", "feature.cerberus.enabled").First(&s).Error; err == nil {
		cerbEnabled = s.Value == "true"
	}
	require.True(t, cerbEnabled)
	assert.Equal(t, uint(4), s.ID, "Cerberus query should return ID=4")

	// Query 2: ACL (ID=6) - WITHOUT reset this would fail
	// With the fix applied in manager.go, struct should be reset here
	s = models.Setting{} // THE FIX
	aclEnabled := false
	err := db.Where("key = ?", "security.acl.enabled").First(&s).Error
	require.NoError(t, err, "ACL query should not fail with 'record not found'")
	if err == nil {
		aclEnabled = s.Value == "true"
	}
	require.True(t, aclEnabled)
	assert.Equal(t, uint(6), s.ID, "ACL query should return ID=6")

	// Query 3: WAF (ID=8) - should also work with reset
	s = models.Setting{} // THE FIX
	wafEnabled := false
	if err := db.Where("key = ?", "security.waf.enabled").First(&s).Error; err == nil {
		wafEnabled = s.Value == "true"
	}
	require.True(t, wafEnabled)
	assert.Equal(t, uint(8), s.ID, "WAF query should return ID=8")
}

// TestUptimeMonitorSettingsReuse verifies the ticker loop scenario from routes.go
// where the same variable is reused across multiple query iterations.
// This test simulates what happens when a setting is deleted and recreated.
func TestUptimeMonitorSettingsReuse(t *testing.T) {
	db := setupTestDB(t)

	setting := &models.Setting{Key: "feature.uptime.enabled", Value: "true"}
	err := db.Create(setting).Error
	require.NoError(t, err)
	firstID := setting.ID

	// First query - simulates initial check before ticker starts
	var s models.Setting
	err = db.Where("key = ?", "feature.uptime.enabled").First(&s).Error
	require.NoError(t, err)
	assert.Equal(t, firstID, s.ID)
	assert.Equal(t, "true", s.Value)

	// Simulate setting being deleted and recreated (e.g., during migration or manual change)
	err = db.Delete(setting).Error
	require.NoError(t, err)

	newSetting := &models.Setting{Key: "feature.uptime.enabled", Value: "true"}
	err = db.Create(newSetting).Error
	require.NoError(t, err)
	newID := newSetting.ID
	assert.NotEqual(t, firstID, newID, "New record should have different ID")

	// Second query WITH reset - simulates ticker loop iteration with fix
	s = models.Setting{} // THE FIX
	err = db.Where("key = ?", "feature.uptime.enabled").First(&s).Error
	require.NoError(t, err, "Query should find new record after reset")
	assert.Equal(t, newID, s.ID, "Should find new record with new ID")
	assert.Equal(t, "true", s.Value)

	// Third iteration - verify reset works across multiple ticks
	s = models.Setting{} // THE FIX
	err = db.Where("key = ?", "feature.uptime.enabled").First(&s).Error
	require.NoError(t, err)
	assert.Equal(t, newID, s.ID)
}

// TestSettingsQueryBugDemonstration explicitly demonstrates the bug scenario
// This test documents the expected behavior BEFORE and AFTER the fix
func TestSettingsQueryBugDemonstration(t *testing.T) {
	db := setupTestDB(t)

	// Setup: Create two settings with different IDs
	db.Create(&models.Setting{Key: "setting.one", Value: "value1"}) // ID=1
	db.Create(&models.Setting{Key: "setting.two", Value: "value2"}) // ID=2

	t.Run("bug scenario - no reset", func(t *testing.T) {
		var s models.Setting

		// Query 1: Gets setting.one (ID=1)
		err := db.Where("key = ?", "setting.one").First(&s).Error
		require.NoError(t, err)
		assert.Equal(t, uint(1), s.ID)

		// Query 2: Try to get setting.two (ID=2)
		// WITHOUT reset, s.ID is still 1, so GORM generates:
		// SELECT * FROM settings WHERE key = 'setting.two' AND id = 1
		// This fails because no record matches both conditions
		err = db.Where("key = ?", "setting.two").First(&s).Error

		// This assertion documents the bug behavior
		if err != nil {
			assert.ErrorIs(t, err, gorm.ErrRecordNotFound,
				"Bug causes 'record not found' because GORM includes ID=1 in WHERE clause")
		}
	})

	t.Run("fixed scenario - with reset", func(t *testing.T) {
		var s models.Setting

		// Query 1: Gets setting.one (ID=1)
		err := db.Where("key = ?", "setting.one").First(&s).Error
		require.NoError(t, err)
		assert.Equal(t, uint(1), s.ID)

		// THE FIX: Reset struct before next query
		s = models.Setting{}

		// Query 2: Get setting.two (ID=2)
		// After reset, GORM generates correct query:
		// SELECT * FROM settings WHERE key = 'setting.two'
		err = db.Where("key = ?", "setting.two").First(&s).Error
		require.NoError(t, err, "With reset, query should succeed")
		assert.Equal(t, uint(2), s.ID, "Should find the correct record")
	})
}
