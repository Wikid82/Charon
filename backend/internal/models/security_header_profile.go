package models

import (
	"time"
)

// SecurityHeaderProfile stores reusable security header configurations.
// Users can create profiles and assign them to proxy hosts.
type SecurityHeaderProfile struct {
	ID   uint   `json:"-" gorm:"primaryKey"`
	UUID string `json:"uuid" gorm:"uniqueIndex;not null"`
	Name string `json:"name" gorm:"index;not null"`

	// HSTS Configuration
	HSTSEnabled           bool `json:"hsts_enabled" gorm:"default:true"`
	HSTSMaxAge            int  `json:"hsts_max_age" gorm:"default:31536000"` // 1 year in seconds
	HSTSIncludeSubdomains bool `json:"hsts_include_subdomains" gorm:"default:true"`
	HSTSPreload           bool `json:"hsts_preload" gorm:"default:false"`

	// Content-Security-Policy
	CSPEnabled    bool   `json:"csp_enabled" gorm:"default:false"`
	CSPDirectives string `json:"csp_directives" gorm:"type:text"` // JSON object of CSP directives
	CSPReportOnly bool   `json:"csp_report_only" gorm:"default:false"`
	CSPReportURI  string `json:"csp_report_uri"`

	// X-Frame-Options
	XFrameOptions string `json:"x_frame_options" gorm:"default:DENY"` // DENY, SAMEORIGIN, or empty

	// X-Content-Type-Options
	XContentTypeOptions bool `json:"x_content_type_options" gorm:"default:true"` // nosniff

	// Referrer-Policy
	ReferrerPolicy string `json:"referrer_policy" gorm:"default:strict-origin-when-cross-origin"`

	// Permissions-Policy (formerly Feature-Policy)
	PermissionsPolicy string `json:"permissions_policy" gorm:"type:text"` // JSON array of policies

	// Cross-Origin Headers
	CrossOriginOpenerPolicy   string `json:"cross_origin_opener_policy" gorm:"default:same-origin"`
	CrossOriginResourcePolicy string `json:"cross_origin_resource_policy" gorm:"default:same-origin"`
	CrossOriginEmbedderPolicy string `json:"cross_origin_embedder_policy"` // require-corp or empty

	// X-XSS-Protection (legacy but still useful)
	XSSProtection bool `json:"xss_protection" gorm:"default:true"`

	// Cache-Control for security
	CacheControlNoStore bool `json:"cache_control_no_store" gorm:"default:false"`

	// Computed Security Score (0-100)
	SecurityScore int `json:"security_score" gorm:"default:0"`

	// Metadata
	IsPreset    bool   `json:"is_preset" gorm:"default:false"` // System presets can't be deleted
	PresetType  string `json:"preset_type"`                    // "basic", "strict", "paranoid", or empty for custom
	Description string `json:"description" gorm:"type:text"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CSPDirective represents a single CSP directive for the builder
type CSPDirective struct {
	Directive string   `json:"directive"` // e.g., "default-src", "script-src"
	Values    []string `json:"values"`    // e.g., ["'self'", "https:"]
}

// PermissionsPolicyItem represents a single Permissions-Policy entry
type PermissionsPolicyItem struct {
	Feature   string   `json:"feature"`   // e.g., "camera", "microphone"
	Allowlist []string `json:"allowlist"` // e.g., ["self"], ["*"], []
}
