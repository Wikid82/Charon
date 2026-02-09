package models

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSecurityHeaderProfileDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&SecurityHeaderProfile{})
	assert.NoError(t, err)

	return db
}

func TestSecurityHeaderProfile_Create(t *testing.T) {
	db := setupSecurityHeaderProfileDB(t)

	profile := SecurityHeaderProfile{
		UUID:                  uuid.New().String(),
		Name:                  "Test Profile",
		HSTSEnabled:           true,
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           false,
		CSPEnabled:            false,
		XFrameOptions:         "DENY",
		XContentTypeOptions:   true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		XSSProtection:         true,
		SecurityScore:         65,
		IsPreset:              false,
	}

	err := db.Create(&profile).Error
	assert.NoError(t, err)
	assert.NotZero(t, profile.ID)
}

func TestSecurityHeaderProfile_JSONSerialization(t *testing.T) {
	profile := SecurityHeaderProfile{
		ID:                    1,
		UUID:                  "test-uuid",
		Name:                  "Test Profile",
		HSTSEnabled:           true,
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		XFrameOptions:         "DENY",
		SecurityScore:         85,
	}

	data, err := json.Marshal(profile)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"hsts_enabled":true`)
	assert.Contains(t, string(data), `"hsts_max_age":31536000`)
	assert.Contains(t, string(data), `"x_frame_options":"DENY"`)

	var decoded SecurityHeaderProfile
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, profile.Name, decoded.Name)
	assert.Equal(t, profile.HSTSEnabled, decoded.HSTSEnabled)
	assert.Equal(t, profile.SecurityScore, decoded.SecurityScore)
}

func TestCSPDirective_JSONSerialization(t *testing.T) {
	directive := CSPDirective{
		Directive: "default-src",
		Values:    []string{"'self'", "https:"},
	}

	data, err := json.Marshal(directive)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"directive":"default-src"`)
	assert.Contains(t, string(data), `"values":["'self'","https:"]`)

	var decoded CSPDirective
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, directive.Directive, decoded.Directive)
	assert.Equal(t, directive.Values, decoded.Values)
}

func TestPermissionsPolicyItem_JSONSerialization(t *testing.T) {
	item := PermissionsPolicyItem{
		Feature:   "camera",
		Allowlist: []string{"self"},
	}

	data, err := json.Marshal(item)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"feature":"camera"`)
	assert.Contains(t, string(data), `"allowlist":["self"]`)

	var decoded PermissionsPolicyItem
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, item.Feature, decoded.Feature)
	assert.Equal(t, item.Allowlist, decoded.Allowlist)
}

func TestSecurityHeaderProfile_Defaults(t *testing.T) {
	db := setupSecurityHeaderProfileDB(t)

	profile := SecurityHeaderProfile{
		UUID: uuid.New().String(),
		Name: "Default Test",
	}

	err := db.Create(&profile).Error
	assert.NoError(t, err)

	// Reload to check defaults
	var reloaded SecurityHeaderProfile
	err = db.First(&reloaded, profile.ID).Error
	assert.NoError(t, err)

	assert.True(t, reloaded.HSTSEnabled)
	assert.Equal(t, 31536000, reloaded.HSTSMaxAge)
	assert.True(t, reloaded.HSTSIncludeSubdomains)
	assert.False(t, reloaded.HSTSPreload)
	assert.False(t, reloaded.CSPEnabled)
	assert.Equal(t, "DENY", reloaded.XFrameOptions)
	assert.True(t, reloaded.XContentTypeOptions)
	assert.Equal(t, "strict-origin-when-cross-origin", reloaded.ReferrerPolicy)
	assert.True(t, reloaded.XSSProtection)
	assert.False(t, reloaded.CacheControlNoStore)
	assert.Equal(t, 0, reloaded.SecurityScore)
	assert.False(t, reloaded.IsPreset)
}

func TestSecurityHeaderProfile_UniqueUUID(t *testing.T) {
	db := setupSecurityHeaderProfileDB(t)

	testUUID := uuid.New().String()

	profile1 := SecurityHeaderProfile{
		UUID: testUUID,
		Name: "Profile 1",
	}
	err := db.Create(&profile1).Error
	assert.NoError(t, err)

	profile2 := SecurityHeaderProfile{
		UUID: testUUID,
		Name: "Profile 2",
	}
	err = db.Create(&profile2).Error
	assert.Error(t, err) // Should fail due to unique constraint
}

func TestSecurityHeaderProfile_CSPDirectivesStorage(t *testing.T) {
	db := setupSecurityHeaderProfileDB(t)

	cspDirectives := map[string][]string{
		"default-src": {"'self'"},
		"script-src":  {"'self'", "'unsafe-inline'"},
		"style-src":   {"'self'", "https:"},
	}
	cspJSON, err := json.Marshal(cspDirectives)
	assert.NoError(t, err)

	profile := SecurityHeaderProfile{
		UUID:          uuid.New().String(),
		Name:          "CSP Test",
		CSPEnabled:    true,
		CSPDirectives: string(cspJSON),
	}

	err = db.Create(&profile).Error
	assert.NoError(t, err)

	// Reload and verify
	var reloaded SecurityHeaderProfile
	err = db.First(&reloaded, profile.ID).Error
	assert.NoError(t, err)

	var decoded map[string][]string
	err = json.Unmarshal([]byte(reloaded.CSPDirectives), &decoded)
	assert.NoError(t, err)
	assert.Equal(t, cspDirectives, decoded)
}

func TestSecurityHeaderProfile_PermissionsPolicyStorage(t *testing.T) {
	db := setupSecurityHeaderProfileDB(t)

	permissions := []PermissionsPolicyItem{
		{Feature: "camera", Allowlist: []string{}},
		{Feature: "microphone", Allowlist: []string{"self"}},
		{Feature: "geolocation", Allowlist: []string{"*"}},
	}
	permJSON, err := json.Marshal(permissions)
	assert.NoError(t, err)

	profile := SecurityHeaderProfile{
		UUID:              uuid.New().String(),
		Name:              "Permissions Test",
		PermissionsPolicy: string(permJSON),
	}

	err = db.Create(&profile).Error
	assert.NoError(t, err)

	// Reload and verify
	var reloaded SecurityHeaderProfile
	err = db.First(&reloaded, profile.ID).Error
	assert.NoError(t, err)

	var decoded []PermissionsPolicyItem
	err = json.Unmarshal([]byte(reloaded.PermissionsPolicy), &decoded)
	assert.NoError(t, err)
	assert.Equal(t, permissions, decoded)
}

func TestSecurityHeaderProfile_PresetFields(t *testing.T) {
	db := setupSecurityHeaderProfileDB(t)

	profile := SecurityHeaderProfile{
		UUID:        uuid.New().String(),
		Name:        "Basic Security",
		IsPreset:    true,
		PresetType:  "basic",
		Description: "Essential security headers for most websites",
	}

	err := db.Create(&profile).Error
	assert.NoError(t, err)

	// Reload
	var reloaded SecurityHeaderProfile
	err = db.First(&reloaded, profile.ID).Error
	assert.NoError(t, err)

	assert.True(t, reloaded.IsPreset)
	assert.Equal(t, "basic", reloaded.PresetType)
	assert.Equal(t, "Essential security headers for most websites", reloaded.Description)
}
