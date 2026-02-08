package services

import (
	"fmt"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SecurityHeadersService manages security header profiles
type SecurityHeadersService struct {
	db *gorm.DB
}

// NewSecurityHeadersService creates a new security headers service
func NewSecurityHeadersService(db *gorm.DB) *SecurityHeadersService {
	return &SecurityHeadersService{db: db}
}

// GetPresets returns the built-in presets
func (s *SecurityHeadersService) GetPresets() []models.SecurityHeaderProfile {
	return []models.SecurityHeaderProfile{
		{
			UUID:                  "preset-basic",
			Name:                  "Basic Security",
			PresetType:            "basic",
			IsPreset:              true,
			Description:           "Essential security headers for most websites. Safe defaults that won't break functionality.",
			HSTSEnabled:           true,
			HSTSMaxAge:            31536000, // 1 year
			HSTSIncludeSubdomains: false,
			HSTSPreload:           false,
			CSPEnabled:            false, // CSP can break sites
			XFrameOptions:         "SAMEORIGIN",
			XContentTypeOptions:   true,
			ReferrerPolicy:        "strict-origin-when-cross-origin",
			XSSProtection:         true,
			SecurityScore:         65,
		},
		{
			UUID:                      "preset-api-friendly",
			Name:                      "API-Friendly",
			PresetType:                "api-friendly",
			IsPreset:                  true,
			Description:               "Optimized for mobile apps and API access (Radarr, Plex, Home Assistant). Strong transport security without breaking API compatibility.",
			HSTSEnabled:               true,
			HSTSMaxAge:                31536000, // 1 year
			HSTSIncludeSubdomains:     false,
			HSTSPreload:               false,
			CSPEnabled:                false, // APIs don't need CSP
			XFrameOptions:             "",    // Allow WebViews
			XContentTypeOptions:       true,
			ReferrerPolicy:            "strict-origin-when-cross-origin",
			PermissionsPolicy:         "",             // Allow all permissions
			CrossOriginOpenerPolicy:   "",             // Allow OAuth popups
			CrossOriginResourcePolicy: "cross-origin", // KEY: Allow cross-origin access
			CrossOriginEmbedderPolicy: "",             // Don't require CORP
			XSSProtection:             true,
			CacheControlNoStore:       false,
			SecurityScore:             70,
		},
		{
			UUID:                      "preset-strict",
			Name:                      "Strict Security",
			PresetType:                "strict",
			IsPreset:                  true,
			Description:               "Strong security for applications handling sensitive data. May require CSP adjustments.",
			HSTSEnabled:               true,
			HSTSMaxAge:                31536000,
			HSTSIncludeSubdomains:     true,
			HSTSPreload:               false,
			CSPEnabled:                true,
			CSPDirectives:             `{"default-src":["'self'"],"script-src":["'self'"],"style-src":["'self'","'unsafe-inline'"],"img-src":["'self'","data:","https:"],"font-src":["'self'","data:"],"connect-src":["'self'"],"frame-src":["'none'"],"object-src":["'none'"]}`,
			XFrameOptions:             "DENY",
			XContentTypeOptions:       true,
			ReferrerPolicy:            "strict-origin-when-cross-origin",
			PermissionsPolicy:         `[{"feature":"camera","allowlist":[]},{"feature":"microphone","allowlist":[]},{"feature":"geolocation","allowlist":[]}]`,
			XSSProtection:             true,
			CrossOriginOpenerPolicy:   "same-origin",
			CrossOriginResourcePolicy: "same-origin",
			SecurityScore:             85,
		},
		{
			UUID:                      "preset-paranoid",
			Name:                      "Paranoid Security",
			PresetType:                "paranoid",
			IsPreset:                  true,
			Description:               "Maximum security for high-risk applications. May break some functionality. Test thoroughly.",
			HSTSEnabled:               true,
			HSTSMaxAge:                63072000, // 2 years
			HSTSIncludeSubdomains:     true,
			HSTSPreload:               true,
			CSPEnabled:                true,
			CSPDirectives:             `{"default-src":["'none'"],"script-src":["'self'"],"style-src":["'self'"],"img-src":["'self'"],"font-src":["'self'"],"connect-src":["'self'"],"frame-src":["'none'"],"object-src":["'none'"],"base-uri":["'self'"],"form-action":["'self'"],"frame-ancestors":["'none'"]}`,
			XFrameOptions:             "DENY",
			XContentTypeOptions:       true,
			ReferrerPolicy:            "no-referrer",
			PermissionsPolicy:         `[{"feature":"camera","allowlist":[]},{"feature":"microphone","allowlist":[]},{"feature":"geolocation","allowlist":[]},{"feature":"payment","allowlist":[]},{"feature":"usb","allowlist":[]}]`,
			XSSProtection:             true,
			CrossOriginOpenerPolicy:   "same-origin",
			CrossOriginResourcePolicy: "same-origin",
			CrossOriginEmbedderPolicy: "require-corp",
			CacheControlNoStore:       true,
			SecurityScore:             100,
		},
	}
}

// EnsurePresetsExist creates default presets if they don't exist
func (s *SecurityHeadersService) EnsurePresetsExist() error {
	presets := s.GetPresets()

	for _, preset := range presets {
		var existing models.SecurityHeaderProfile
		err := s.db.Where("uuid = ?", preset.UUID).First(&existing).Error

		switch {
		case err == gorm.ErrRecordNotFound:
			// Create preset with a fresh UUID for the ID field
			if createErr := s.db.Create(&preset).Error; createErr != nil {
				return fmt.Errorf("failed to create preset %s: %w", preset.Name, createErr)
			}
		case err != nil:
			return fmt.Errorf("failed to check preset %s: %w", preset.Name, err)
		default:
			// Update existing preset to ensure it has latest values
			preset.ID = existing.ID // Keep the existing ID
			if saveErr := s.db.Save(&preset).Error; saveErr != nil {
				return fmt.Errorf("failed to update preset %s: %w", preset.Name, saveErr)
			}
		}
	}

	return nil
}

// ApplyPreset creates a new profile based on a preset
func (s *SecurityHeadersService) ApplyPreset(presetType, name string) (*models.SecurityHeaderProfile, error) {
	presets := s.GetPresets()

	var selectedPreset *models.SecurityHeaderProfile
	for i := range presets {
		if presets[i].PresetType == presetType {
			selectedPreset = &presets[i]
			break
		}
	}

	if selectedPreset == nil {
		return nil, fmt.Errorf("preset type %s not found", presetType)
	}

	// Create a copy with custom name and UUID
	newProfile := *selectedPreset
	newProfile.ID = 0 // Clear ID so GORM creates a new record
	newProfile.UUID = uuid.New().String()
	newProfile.Name = name
	newProfile.IsPreset = false // User-created profiles are not presets
	newProfile.PresetType = ""  // Clear preset type for custom profiles

	if err := s.db.Create(&newProfile).Error; err != nil {
		return nil, fmt.Errorf("failed to create profile from preset: %w", err)
	}

	return &newProfile, nil
}
