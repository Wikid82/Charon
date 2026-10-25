package models_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestDefaultSecurityConfigName_IsDefault(t *testing.T) {
	assert.Equal(t, "default", models.DefaultSecurityConfigName)
}

func TestFindDefaultSecurityConfig_ReturnsDefaultRow(t *testing.T) {
	db := newSeedTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-other", Name: "other", Enabled: true}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "local"}).Error)

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "u-default", cfg.UUID)
	assert.Equal(t, "local", cfg.CrowdSecMode)
	assert.False(t, cfg.Enabled)
}

func TestFindDefaultSecurityConfig_IgnoresOtherRowsWithLowerID(t *testing.T) {
	db := newSeedTestDB(t)
	// Lowest ID but wrong name: the helper must not return it.
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-wrong", Name: "Default Security Config"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName}).Error)

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "u-default", cfg.UUID)
}

func TestFindDefaultSecurityConfig_MissingRowReturnsNotFound(t *testing.T) {
	db := newSeedTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-other", Name: "other"}).Error)

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	assert.Equal(t, models.SecurityConfig{}, cfg)
}

func TestFindDefaultSecurityConfig_EmptyTableReturnsNotFound(t *testing.T) {
	db := newSeedTestDB(t)

	_, err := models.FindDefaultSecurityConfig(db)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestFindDefaultSecurityConfig_DatabaseErrorIsWrapped(t *testing.T) {
	db := newSeedTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = models.FindDefaultSecurityConfig(db)
	require.Error(t, err)
	assert.False(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestFindDefaultSecurityConfig_SeededRowIsFound(t *testing.T) {
	db := newSeedTestDB(t)

	seeded, err := models.SeedDefaultSecurityConfig(db)
	require.NoError(t, err)

	found, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, seeded.ID, found.ID)
	assert.Equal(t, seeded.UUID, found.UUID)
}
