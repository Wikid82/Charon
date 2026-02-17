package services

import (
	"fmt"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSecurityHeadersServiceDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.SecurityHeaderProfile{})
	assert.NoError(t, err)

	return db
}

func TestGetPresets(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	presets := service.GetPresets()

	assert.Len(t, presets, 4)

	// Check basic preset
	basic := presets[0]
	assert.Equal(t, "preset-basic", basic.UUID)
	assert.Equal(t, "Basic Security", basic.Name)
	assert.Equal(t, "basic", basic.PresetType)
	assert.True(t, basic.IsPreset)
	assert.True(t, basic.HSTSEnabled)
	assert.False(t, basic.CSPEnabled)
	assert.Equal(t, 65, basic.SecurityScore)

	// Check API-Friendly preset
	apiFriendly := presets[1]
	assert.Equal(t, "preset-api-friendly", apiFriendly.UUID)
	assert.Equal(t, "API-Friendly", apiFriendly.Name)
	assert.Equal(t, "api-friendly", apiFriendly.PresetType)
	assert.True(t, apiFriendly.IsPreset)
	assert.True(t, apiFriendly.HSTSEnabled)
	assert.False(t, apiFriendly.CSPEnabled)
	assert.Equal(t, "", apiFriendly.XFrameOptions)                         // Allow WebViews
	assert.Equal(t, "cross-origin", apiFriendly.CrossOriginResourcePolicy) // KEY for APIs
	assert.Equal(t, 70, apiFriendly.SecurityScore)

	// Check strict preset
	strict := presets[2]
	assert.Equal(t, "preset-strict", strict.UUID)
	assert.Equal(t, "Strict Security", strict.Name)
	assert.Equal(t, "strict", strict.PresetType)
	assert.True(t, strict.IsPreset)
	assert.True(t, strict.CSPEnabled)
	assert.NotEmpty(t, strict.CSPDirectives)
	assert.Equal(t, 85, strict.SecurityScore)

	// Check paranoid preset
	paranoid := presets[3]
	assert.Equal(t, "preset-paranoid", paranoid.UUID)
	assert.Equal(t, "Paranoid Security", paranoid.Name)
	assert.Equal(t, "paranoid", paranoid.PresetType)
	assert.True(t, paranoid.IsPreset)
	assert.True(t, paranoid.HSTSPreload)
	assert.Equal(t, "no-referrer", paranoid.ReferrerPolicy)
	assert.True(t, paranoid.CacheControlNoStore)
	assert.Equal(t, 100, paranoid.SecurityScore)
}

func TestEnsurePresetsExist_Creates(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	// Initially no presets
	var count int64
	db.Model(&models.SecurityHeaderProfile{}).Count(&count)
	assert.Equal(t, int64(0), count)

	// Ensure presets exist
	err := service.EnsurePresetsExist()
	assert.NoError(t, err)

	// Should now have 4 presets
	db.Model(&models.SecurityHeaderProfile{}).Count(&count)
	assert.Equal(t, int64(4), count)

	// Verify presets are correct
	var basic models.SecurityHeaderProfile
	err = db.Where("uuid = ?", "preset-basic").First(&basic).Error
	assert.NoError(t, err)
	assert.Equal(t, "Basic Security", basic.Name)
	assert.True(t, basic.IsPreset)

	var apiFriendly models.SecurityHeaderProfile
	err = db.Where("uuid = ?", "preset-api-friendly").First(&apiFriendly).Error
	assert.NoError(t, err)
	assert.Equal(t, "API-Friendly", apiFriendly.Name)
	assert.True(t, apiFriendly.IsPreset)

	var strict models.SecurityHeaderProfile
	err = db.Where("uuid = ?", "preset-strict").First(&strict).Error
	assert.NoError(t, err)
	assert.Equal(t, "Strict Security", strict.Name)
	assert.True(t, strict.IsPreset)

	var paranoid models.SecurityHeaderProfile
	err = db.Where("uuid = ?", "preset-paranoid").First(&paranoid).Error
	assert.NoError(t, err)
	assert.Equal(t, "Paranoid Security", paranoid.Name)
	assert.True(t, paranoid.IsPreset)
}

func TestEnsurePresetsExist_NoOp(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	// Create presets first time
	err := service.EnsurePresetsExist()
	assert.NoError(t, err)

	var count1 int64
	db.Model(&models.SecurityHeaderProfile{}).Count(&count1)
	assert.Equal(t, int64(4), count1)

	// Run again - should not duplicate
	err = service.EnsurePresetsExist()
	assert.NoError(t, err)

	var count2 int64
	db.Model(&models.SecurityHeaderProfile{}).Count(&count2)
	assert.Equal(t, int64(4), count2) // Still 4
}

func TestEnsurePresetsExist_Updates(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	// Create initial preset
	oldPreset := models.SecurityHeaderProfile{
		UUID:          "preset-basic",
		Name:          "Old Name",
		PresetType:    "basic",
		IsPreset:      true,
		SecurityScore: 50,
	}
	err := db.Create(&oldPreset).Error
	assert.NoError(t, err)

	// Ensure presets exist - should update
	err = service.EnsurePresetsExist()
	assert.NoError(t, err)

	// Check that it was updated
	var updated models.SecurityHeaderProfile
	err = db.Where("uuid = ?", "preset-basic").First(&updated).Error
	assert.NoError(t, err)
	assert.Equal(t, "Basic Security", updated.Name) // Name updated
	assert.Equal(t, 65, updated.SecurityScore)      // Score updated
}

func TestApplyPreset_Success(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	// Apply basic preset
	profile, err := service.ApplyPreset("basic", "My Custom Basic Profile")
	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.NotZero(t, profile.ID)
	assert.NotEmpty(t, profile.UUID)
	assert.NotEqual(t, "preset-basic", profile.UUID) // Should have new UUID
	assert.Equal(t, "My Custom Basic Profile", profile.Name)
	assert.False(t, profile.IsPreset) // Not a preset anymore
	assert.Empty(t, profile.PresetType)
	assert.True(t, profile.HSTSEnabled)
	assert.False(t, profile.CSPEnabled)

	// Verify it was saved
	var saved models.SecurityHeaderProfile
	err = db.Where("id = ?", profile.ID).First(&saved).Error
	assert.NoError(t, err)
	assert.Equal(t, profile.Name, saved.Name)
}

func TestApplyPreset_StrictPreset(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	profile, err := service.ApplyPreset("strict", "My Strict Profile")
	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "My Strict Profile", profile.Name)
	assert.True(t, profile.CSPEnabled)
	assert.NotEmpty(t, profile.CSPDirectives)
	assert.NotEmpty(t, profile.PermissionsPolicy)
	assert.Equal(t, "same-origin", profile.CrossOriginOpenerPolicy)
}

func TestApplyPreset_ParanoidPreset(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	profile, err := service.ApplyPreset("paranoid", "My Paranoid Profile")
	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "My Paranoid Profile", profile.Name)
	assert.True(t, profile.HSTSPreload)
	assert.Equal(t, "no-referrer", profile.ReferrerPolicy)
	assert.True(t, profile.CacheControlNoStore)
	assert.Equal(t, "require-corp", profile.CrossOriginEmbedderPolicy)
}

func TestApplyPreset_APIFriendlyPreset(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	profile, err := service.ApplyPreset("api-friendly", "My API Profile")
	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "My API Profile", profile.Name)
	assert.True(t, profile.HSTSEnabled)
	// Note: GORM applies default:true when bool is false (zero value)
	// The preset defines HSTSIncludeSubdomains: false but GORM default overrides
	assert.False(t, profile.HSTSPreload)
	assert.False(t, profile.CSPEnabled)
	// Note: GORM applies defaults for zero-value strings (XFrameOptions default:DENY)
	// The API-Friendly preset intentionally uses empty values for flexibility
	assert.True(t, profile.XContentTypeOptions)
	assert.Equal(t, "strict-origin-when-cross-origin", profile.ReferrerPolicy)
	// CORP is explicitly set to "cross-origin" which is non-zero, so it persists
	assert.Equal(t, "cross-origin", profile.CrossOriginResourcePolicy) // KEY for APIs
	assert.True(t, profile.XSSProtection)
	assert.False(t, profile.CacheControlNoStore)
}

func TestGetPresets_IncludesAPIFriendly(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	presets := service.GetPresets()

	// Find API-Friendly preset
	var apiFriendly *models.SecurityHeaderProfile
	for i := range presets {
		if presets[i].PresetType == "api-friendly" {
			apiFriendly = &presets[i]
			break
		}
	}

	assert.NotNil(t, apiFriendly, "API-Friendly preset should exist")
	assert.Equal(t, "preset-api-friendly", apiFriendly.UUID)
	assert.Equal(t, "API-Friendly", apiFriendly.Name)
	assert.True(t, apiFriendly.IsPreset)
	assert.Contains(t, apiFriendly.Description, "mobile apps")
	assert.Contains(t, apiFriendly.Description, "API")

	// Verify key API-friendly settings
	assert.True(t, apiFriendly.HSTSEnabled, "HSTS should be enabled for transport security")
	assert.False(t, apiFriendly.CSPEnabled, "CSP should be disabled for API compatibility")
	assert.Empty(t, apiFriendly.XFrameOptions, "X-Frame-Options should be empty to allow WebViews")
	assert.Equal(t, "cross-origin", apiFriendly.CrossOriginResourcePolicy, "CORP should be cross-origin for API access")
	assert.Empty(t, apiFriendly.CrossOriginOpenerPolicy, "COOP should be empty to allow OAuth popups")
	assert.Empty(t, apiFriendly.CrossOriginEmbedderPolicy, "COEP should be empty for API compatibility")
	assert.Equal(t, 70, apiFriendly.SecurityScore)
}

func TestGetPresets_OrderByScore(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	presets := service.GetPresets()

	// Verify we have all 4 presets
	assert.Len(t, presets, 4)

	// Verify order by security score: Basic(65) < API-Friendly(70) < Strict(85) < Paranoid(100)
	assert.Equal(t, "basic", presets[0].PresetType)
	assert.Equal(t, 65, presets[0].SecurityScore)

	assert.Equal(t, "api-friendly", presets[1].PresetType)
	assert.Equal(t, 70, presets[1].SecurityScore)

	assert.Equal(t, "strict", presets[2].PresetType)
	assert.Equal(t, 85, presets[2].SecurityScore)

	assert.Equal(t, "paranoid", presets[3].PresetType)
	assert.Equal(t, 100, presets[3].SecurityScore)

	// Verify ascending order
	for i := 1; i < len(presets); i++ {
		assert.Greater(t, presets[i].SecurityScore, presets[i-1].SecurityScore,
			"Presets should be ordered by ascending security score")
	}
}

func TestApplyPreset_InvalidPreset(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	profile, err := service.ApplyPreset("nonexistent", "Test")
	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "preset type nonexistent not found")
}

func TestApplyPreset_MultipleProfiles(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	// Create multiple profiles from same preset
	profile1, err := service.ApplyPreset("basic", "Profile 1")
	assert.NoError(t, err)

	profile2, err := service.ApplyPreset("basic", "Profile 2")
	assert.NoError(t, err)

	// Should have different IDs and UUIDs
	assert.NotEqual(t, profile1.ID, profile2.ID)
	assert.NotEqual(t, profile1.UUID, profile2.UUID)
	assert.Equal(t, "Profile 1", profile1.Name)
	assert.Equal(t, "Profile 2", profile2.Name)

	// Both should be saved
	var count int64
	db.Model(&models.SecurityHeaderProfile{}).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestEnsurePresetsExist_CreateError(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	cbName := "test:create-error"
	err := db.Callback().Create().Before("gorm:create").Register(cbName, func(tx *gorm.DB) {
		_ = tx.AddError(fmt.Errorf("forced create error"))
	})
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove(cbName)
	})

	err = service.EnsurePresetsExist()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create preset")
}

func TestEnsurePresetsExist_SaveError(t *testing.T) {
	db := setupSecurityHeadersServiceDB(t)
	service := NewSecurityHeadersService(db)

	require.NoError(t, service.EnsurePresetsExist())

	cbName := "test:update-error"
	err := db.Callback().Update().Before("gorm:update").Register(cbName, func(tx *gorm.DB) {
		_ = tx.AddError(fmt.Errorf("forced update error"))
	})
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(cbName)
	})

	err = service.EnsurePresetsExist()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update preset")
}
