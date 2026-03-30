package models_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func newSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	return db
}

func TestSeedDefaultSecurityConfig_EmptyDB(t *testing.T) {
	db := newSeedTestDB(t)

	rec, err := models.SeedDefaultSecurityConfig(db)
	require.NoError(t, err)
	require.NotNil(t, rec)

	assert.Equal(t, "default", rec.Name)
	assert.False(t, rec.Enabled)
	assert.Equal(t, "disabled", rec.CrowdSecMode)
	assert.Equal(t, "http://127.0.0.1:8085", rec.CrowdSecAPIURL)
	assert.Equal(t, "disabled", rec.WAFMode)
	assert.Equal(t, "disabled", rec.RateLimitMode)
	assert.NotEmpty(t, rec.UUID)

	var count int64
	db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestSeedDefaultSecurityConfig_Idempotent(t *testing.T) {
	db := newSeedTestDB(t)

	// First call — creates the row.
	rec1, err := models.SeedDefaultSecurityConfig(db)
	require.NoError(t, err)
	require.NotNil(t, rec1)

	// Second call — must not error and must not duplicate.
	rec2, err := models.SeedDefaultSecurityConfig(db)
	require.NoError(t, err)
	require.NotNil(t, rec2)

	assert.Equal(t, rec1.ID, rec2.ID, "ID must be identical on subsequent calls")

	var count int64
	db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Count(&count)
	assert.Equal(t, int64(1), count, "exactly one row should exist after two seed calls")
}

func TestSeedDefaultSecurityConfig_DBError(t *testing.T) {
	db := newSeedTestDB(t)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	rec, err := models.SeedDefaultSecurityConfig(db)
	assert.Error(t, err)
	assert.Nil(t, rec)
}

func TestSeedDefaultSecurityConfig_DoesNotOverwriteExisting(t *testing.T) {
	db := newSeedTestDB(t)

	// Pre-seed a customised row.
	existing := models.SecurityConfig{
		UUID:           "pre-existing-uuid",
		Name:           "default",
		Enabled:        true,
		CrowdSecMode:   "local",
		CrowdSecAPIURL: "http://192.168.1.5:8085",
		WAFMode:        "block",
		RateLimitMode:  "enabled",
	}
	require.NoError(t, db.Create(&existing).Error)

	// Seed should find the existing row and return it unchanged.
	rec, err := models.SeedDefaultSecurityConfig(db)
	require.NoError(t, err)
	require.NotNil(t, rec)

	assert.True(t, rec.Enabled, "existing Enabled flag must not be overwritten")
	assert.Equal(t, "local", rec.CrowdSecMode, "existing CrowdSecMode must not be overwritten")
	assert.Equal(t, "http://192.168.1.5:8085", rec.CrowdSecAPIURL)
	assert.Equal(t, "block", rec.WAFMode)

	var count int64
	db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Count(&count)
	assert.Equal(t, int64(1), count)
}
