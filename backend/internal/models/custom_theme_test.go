package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupCustomThemeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CustomTheme{}))
	return db
}

// TestCustomTheme_BeforeCreate_GeneratesUUID verifies UUID is set when ID is empty.
func TestCustomTheme_BeforeCreate_GeneratesUUID(t *testing.T) {
	db := setupCustomThemeTestDB(t)

	ct := &CustomTheme{
		Name:   "Test Theme",
		Colors: `{"bgBase":"15 23 42"}`,
	}
	require.NoError(t, db.Create(ct).Error)

	assert.NotEmpty(t, ct.ID, "ID should be set after create")
	assert.Len(t, ct.ID, 36, "UUID should be 36 characters")
}

// TestCustomTheme_BeforeCreate_PreservesExistingID verifies existing ID is not overwritten.
func TestCustomTheme_BeforeCreate_PreservesExistingID(t *testing.T) {
	db := setupCustomThemeTestDB(t)

	existingID := "550e8400-e29b-41d4-a716-446655440000"
	ct := &CustomTheme{
		ID:     existingID,
		Name:   "Pre-set Theme",
		Colors: `{"bgBase":"30 41 59"}`,
	}
	require.NoError(t, db.Create(ct).Error)

	assert.Equal(t, existingID, ct.ID, "pre-set ID should not be overwritten")
}

// TestCustomTheme_UniqueNameConstraint verifies duplicate names are rejected.
func TestCustomTheme_UniqueNameConstraint(t *testing.T) {
	db := setupCustomThemeTestDB(t)

	ct1 := &CustomTheme{Name: "Duplicate", Colors: `{"bgBase":"0 0 0"}`}
	require.NoError(t, db.Create(ct1).Error)

	ct2 := &CustomTheme{Name: "Duplicate", Colors: `{"bgBase":"255 255 255"}`}
	err := db.Create(ct2).Error
	assert.Error(t, err, "duplicate name should fail")
}
