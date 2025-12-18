# HTTP Security Headers Implementation Plan (Issue #20)

**Created**: December 18, 2025
**Status**: Planning Complete - Ready for Implementation
**Issue**: GitHub Issue #20 - HTTP Security Headers

---

## Executive Summary

This plan implements automatic security header injection for proxy hosts in Charon. The feature enables users to configure HTTP security headers (HSTS, CSP, X-Frame-Options, etc.) through presets or custom configurations, with a security score calculator to help users understand their security posture.

**Key Goals:**

- Zero-config secure defaults for novice users
- Granular control for advanced users
- Visual security score feedback
- Per-host and global configuration options
- CSP builder that doesn't break sites

---

## Architecture Overview

### Data Flow

```text
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Frontend   │────▶│  Backend API │────▶│    Models    │────▶│    Caddy     │
│  UI/Config   │     │   Handlers   │     │  (SQLite)    │     │ JSON Config  │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
        │                    │                    │                    │
        │                    │                    │                    │
   ┌────▼────┐          ┌────▼────┐         ┌────▼────┐          ┌────▼────┐
   │ Presets │          │ CRUD    │         │ Profile │          │ Headers │
   │ Selector│          │ Profile │         │ Storage │          │ Handler │
   │ Score   │          │ Score   │         │ Host FK │          │ Inject  │
   └─────────┘          └─────────┘         └─────────┘          └─────────┘
```

### Key Design Decisions

1. **Security Header Profiles** - Reusable configurations that can be shared across hosts
2. **Per-Host Override** - Each proxy host can reference a profile OR have inline settings
3. **Presets System** - Pre-built profiles (basic, strict, paranoid) for easy setup
4. **Score Calculator** - Visual feedback showing security level (0-100)
5. **CSP Builder** - Interactive tool to build Content-Security-Policy without breaking sites

---

## Phase 1: Backend Models and Migrations

### 1.1 New Model: `SecurityHeaderProfile`

**File**: `backend/internal/models/security_header_profile.go`

```go
package models

import (
    "time"
)

// SecurityHeaderProfile stores reusable security header configurations.
// Users can create profiles and assign them to proxy hosts.
type SecurityHeaderProfile struct {
    ID        uint      `json:"id" gorm:"primaryKey"`
    UUID      string    `json:"uuid" gorm:"uniqueIndex;not null"`
    Name      string    `json:"name" gorm:"index;not null"`

    // HSTS Configuration
    HSTSEnabled           bool   `json:"hsts_enabled" gorm:"default:true"`
    HSTSMaxAge            int    `json:"hsts_max_age" gorm:"default:31536000"` // 1 year in seconds
    HSTSIncludeSubdomains bool   `json:"hsts_include_subdomains" gorm:"default:true"`
    HSTSPreload           bool   `json:"hsts_preload" gorm:"default:false"`

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
```

### 1.2 Update ProxyHost Model

**File**: `backend/internal/models/proxy_host.go`

Add the following fields to `ProxyHost`:

```go
// Security Headers Configuration
// Either reference a profile OR use inline settings
SecurityHeaderProfileID *uint                  `json:"security_header_profile_id"`
SecurityHeaderProfile   *SecurityHeaderProfile `json:"security_header_profile" gorm:"foreignKey:SecurityHeaderProfileID"`

// Inline security header settings (used when no profile is selected)
// These override profile settings if both are set
SecurityHeadersEnabled bool   `json:"security_headers_enabled" gorm:"default:true"`
SecurityHeadersCustom  string `json:"security_headers_custom" gorm:"type:text"` // JSON for custom headers
```

### 1.3 Migration Updates

**File**: `backend/internal/api/routes/routes.go`

Add to `AutoMigrate`:

```go
&models.SecurityHeaderProfile{},
```

### 1.4 Unit Tests

**File**: `backend/internal/models/security_header_profile_test.go`

Test cases:

- Profile creation with default values
- JSON serialization/deserialization of CSPDirectives
- JSON serialization/deserialization of PermissionsPolicy
- UUID generation on creation
- Security score calculation triggers

---

## Phase 2: Backend API Handlers

### 2.1 Security Headers Handler

**File**: `backend/internal/api/handlers/security_headers_handler.go`

```go
package handlers

// SecurityHeadersHandler manages security header profiles
type SecurityHeadersHandler struct {
    db           *gorm.DB
    caddyManager *caddy.Manager
}

// NewSecurityHeadersHandler creates a new handler
func NewSecurityHeadersHandler(db *gorm.DB, caddyManager *caddy.Manager) *SecurityHeadersHandler

// RegisterRoutes registers all security headers routes
func (h *SecurityHeadersHandler) RegisterRoutes(router *gin.RouterGroup)

// --- Profile CRUD ---

// ListProfiles returns all security header profiles
// GET /api/v1/security/headers/profiles
func (h *SecurityHeadersHandler) ListProfiles(c *gin.Context)

// GetProfile returns a single profile by ID or UUID
// GET /api/v1/security/headers/profiles/:id
func (h *SecurityHeadersHandler) GetProfile(c *gin.Context)

// CreateProfile creates a new security header profile
// POST /api/v1/security/headers/profiles
func (h *SecurityHeadersHandler) CreateProfile(c *gin.Context)

// UpdateProfile updates an existing profile
// PUT /api/v1/security/headers/profiles/:id
func (h *SecurityHeadersHandler) UpdateProfile(c *gin.Context)

// DeleteProfile deletes a profile (not presets)
// DELETE /api/v1/security/headers/profiles/:id
func (h *SecurityHeadersHandler) DeleteProfile(c *gin.Context)

// --- Presets ---

// GetPresets returns the list of built-in presets
// GET /api/v1/security/headers/presets
func (h *SecurityHeadersHandler) GetPresets(c *gin.Context)

// ApplyPreset applies a preset to create/update a profile
// POST /api/v1/security/headers/presets/apply
func (h *SecurityHeadersHandler) ApplyPreset(c *gin.Context)

// --- Security Score ---

// CalculateScore calculates security score for given settings
// POST /api/v1/security/headers/score
func (h *SecurityHeadersHandler) CalculateScore(c *gin.Context)

// --- CSP Builder ---

// ValidateCSP validates a CSP string
// POST /api/v1/security/headers/csp/validate
func (h *SecurityHeadersHandler) ValidateCSP(c *gin.Context)

// BuildCSP builds a CSP string from directives
// POST /api/v1/security/headers/csp/build
func (h *SecurityHeadersHandler) BuildCSP(c *gin.Context)
```

### 2.2 API Routes Registration

**File**: `backend/internal/api/routes/routes.go`

Add to the protected routes section:

```go
// Security Headers
securityHeadersHandler := handlers.NewSecurityHeadersHandler(db, caddyManager)
securityHeadersHandler.RegisterRoutes(protected)
```

### 2.3 Request/Response Types

```go
// CreateProfileRequest for creating a new profile
type CreateProfileRequest struct {
    Name                      string `json:"name" binding:"required"`
    HSTSEnabled               bool   `json:"hsts_enabled"`
    HSTSMaxAge                int    `json:"hsts_max_age"`
    HSTSIncludeSubdomains     bool   `json:"hsts_include_subdomains"`
    HSTSPreload               bool   `json:"hsts_preload"`
    CSPEnabled                bool   `json:"csp_enabled"`
    CSPDirectives             string `json:"csp_directives"`
    CSPReportOnly             bool   `json:"csp_report_only"`
    CSPReportURI              string `json:"csp_report_uri"`
    XFrameOptions             string `json:"x_frame_options"`
    XContentTypeOptions       bool   `json:"x_content_type_options"`
    ReferrerPolicy            string `json:"referrer_policy"`
    PermissionsPolicy         string `json:"permissions_policy"`
    CrossOriginOpenerPolicy   string `json:"cross_origin_opener_policy"`
    CrossOriginResourcePolicy string `json:"cross_origin_resource_policy"`
    CrossOriginEmbedderPolicy string `json:"cross_origin_embedder_policy"`
    XSSProtection             bool   `json:"xss_protection"`
    CacheControlNoStore       bool   `json:"cache_control_no_store"`
    Description               string `json:"description"`
}

// CalculateScoreRequest for scoring
type CalculateScoreRequest struct {
    HSTSEnabled               bool   `json:"hsts_enabled"`
    HSTSMaxAge                int    `json:"hsts_max_age"`
    HSTSIncludeSubdomains     bool   `json:"hsts_include_subdomains"`
    HSTSPreload               bool   `json:"hsts_preload"`
    CSPEnabled                bool   `json:"csp_enabled"`
    CSPDirectives             string `json:"csp_directives"`
    XFrameOptions             string `json:"x_frame_options"`
    XContentTypeOptions       bool   `json:"x_content_type_options"`
    ReferrerPolicy            string `json:"referrer_policy"`
    PermissionsPolicy         string `json:"permissions_policy"`
    CrossOriginOpenerPolicy   string `json:"cross_origin_opener_policy"`
    CrossOriginResourcePolicy string `json:"cross_origin_resource_policy"`
    XSSProtection             bool   `json:"xss_protection"`
}

// ScoreResponse returns the calculated score
type ScoreResponse struct {
    Score       int            `json:"score"`
    MaxScore    int            `json:"max_score"`
    Breakdown   map[string]int `json:"breakdown"`
    Suggestions []string       `json:"suggestions"`
}
```

### 2.4 Unit Tests

**File**: `backend/internal/api/handlers/security_headers_handler_test.go`

Test cases:

- `TestListProfiles` - List all profiles
- `TestGetProfile` - Get profile by ID
- `TestGetProfileByUUID` - Get profile by UUID
- `TestCreateProfile` - Create new profile
- `TestUpdateProfile` - Update existing profile
- `TestDeleteProfile` - Delete custom profile
- `TestDeletePresetFails` - Cannot delete system presets
- `TestGetPresets` - List all presets
- `TestApplyPreset` - Apply preset to create profile
- `TestCalculateScore` - Score calculation
- `TestValidateCSP` - CSP validation
- `TestBuildCSP` - CSP building

---

## Phase 3: Caddy Integration for Header Injection

### 3.1 Header Builder Function

**File**: `backend/internal/caddy/config.go`

Add new function:

```go
// buildSecurityHeadersHandler creates a headers handler for security headers
// based on the profile configuration or host-level settings
func buildSecurityHeadersHandler(host *models.ProxyHost, profile *models.SecurityHeaderProfile) (Handler, error) {
    if profile == nil && !host.SecurityHeadersEnabled {
        return nil, nil
    }

    responseHeaders := make(map[string][]string)

    // Use profile settings or host defaults
    var cfg *models.SecurityHeaderProfile
    if profile != nil {
        cfg = profile
    } else {
        // Use host's inline settings or defaults
        cfg = getDefaultSecurityHeaderProfile()
    }

    // HSTS
    if cfg.HSTSEnabled {
        hstsValue := fmt.Sprintf("max-age=%d", cfg.HSTSMaxAge)
        if cfg.HSTSIncludeSubdomains {
            hstsValue += "; includeSubDomains"
        }
        if cfg.HSTSPreload {
            hstsValue += "; preload"
        }
        responseHeaders["Strict-Transport-Security"] = []string{hstsValue}
    }

    // CSP
    if cfg.CSPEnabled && cfg.CSPDirectives != "" {
        cspHeader := "Content-Security-Policy"
        if cfg.CSPReportOnly {
            cspHeader = "Content-Security-Policy-Report-Only"
        }
        responseHeaders[cspHeader] = []string{buildCSPString(cfg.CSPDirectives)}
    }

    // X-Frame-Options
    if cfg.XFrameOptions != "" {
        responseHeaders["X-Frame-Options"] = []string{cfg.XFrameOptions}
    }

    // X-Content-Type-Options
    if cfg.XContentTypeOptions {
        responseHeaders["X-Content-Type-Options"] = []string{"nosniff"}
    }

    // Referrer-Policy
    if cfg.ReferrerPolicy != "" {
        responseHeaders["Referrer-Policy"] = []string{cfg.ReferrerPolicy}
    }

    // Permissions-Policy
    if cfg.PermissionsPolicy != "" {
        responseHeaders["Permissions-Policy"] = []string{buildPermissionsPolicyString(cfg.PermissionsPolicy)}
    }

    // Cross-Origin headers
    if cfg.CrossOriginOpenerPolicy != "" {
        responseHeaders["Cross-Origin-Opener-Policy"] = []string{cfg.CrossOriginOpenerPolicy}
    }
    if cfg.CrossOriginResourcePolicy != "" {
        responseHeaders["Cross-Origin-Resource-Policy"] = []string{cfg.CrossOriginResourcePolicy}
    }
    if cfg.CrossOriginEmbedderPolicy != "" {
        responseHeaders["Cross-Origin-Embedder-Policy"] = []string{cfg.CrossOriginEmbedderPolicy}
    }

    // X-XSS-Protection
    if cfg.XSSProtection {
        responseHeaders["X-XSS-Protection"] = []string{"1; mode=block"}
    }

    // Cache-Control
    if cfg.CacheControlNoStore {
        responseHeaders["Cache-Control"] = []string{"no-store"}
    }

    if len(responseHeaders) == 0 {
        return nil, nil
    }

    return Handler{
        "handler": "headers",
        "response": map[string]interface{}{
            "set": responseHeaders,
        },
    }, nil
}

// buildCSPString converts JSON CSP directives to a CSP string
func buildCSPString(directivesJSON string) string

// buildPermissionsPolicyString converts JSON permissions to policy string
func buildPermissionsPolicyString(permissionsJSON string) string

// getDefaultSecurityHeaderProfile returns secure defaults
func getDefaultSecurityHeaderProfile() *models.SecurityHeaderProfile
```

### 3.2 Update GenerateConfig

**File**: `backend/internal/caddy/config.go`

In the `GenerateConfig` function, add security headers handler to the handler chain:

```go
// After building securityHandlers and before the main reverse proxy handler:

// Add Security Headers handler
if secHeadersHandler, err := buildSecurityHeadersHandler(&host, host.SecurityHeaderProfile); err == nil && secHeadersHandler != nil {
    handlers = append(handlers, secHeadersHandler)
}
```

### 3.3 Update Manager to Load Profiles

**File**: `backend/internal/caddy/manager.go`

Update `ApplyConfig` to preload security header profiles:

```go
// Preload security header profiles for hosts
var profiles []models.SecurityHeaderProfile
if err := m.db.Find(&profiles).Error; err != nil {
    return fmt.Errorf("failed to load security header profiles: %w", err)
}
profileMap := make(map[uint]*models.SecurityHeaderProfile)
for i := range profiles {
    profileMap[profiles[i].ID] = &profiles[i]
}

// Attach profiles to hosts
for i := range hosts {
    if hosts[i].SecurityHeaderProfileID != nil {
        hosts[i].SecurityHeaderProfile = profileMap[*hosts[i].SecurityHeaderProfileID]
    }
}
```

### 3.4 Unit Tests

**File**: `backend/internal/caddy/config_security_headers_test.go`

Test cases:

- `TestBuildSecurityHeadersHandler_AllEnabled` - All headers enabled
- `TestBuildSecurityHeadersHandler_HSTSOnly` - Only HSTS
- `TestBuildSecurityHeadersHandler_CSPOnly` - Only CSP
- `TestBuildSecurityHeadersHandler_CSPReportOnly` - CSP in report-only mode
- `TestBuildSecurityHeadersHandler_NoProfile` - Default behavior
- `TestBuildSecurityHeadersHandler_Disabled` - Headers disabled
- `TestBuildCSPString` - CSP JSON to string conversion
- `TestBuildPermissionsPolicyString` - Permissions JSON to string
- `TestGenerateConfig_WithSecurityHeaders` - Full integration

---

## Phase 4: Frontend UI Components

### 4.1 API Client

**File**: `frontend/src/api/securityHeaders.ts`

```typescript
import client from './client'

// Types
export interface SecurityHeaderProfile {
  id: number
  uuid: string
  name: string
  hsts_enabled: boolean
  hsts_max_age: number
  hsts_include_subdomains: boolean
  hsts_preload: boolean
  csp_enabled: boolean
  csp_directives: string
  csp_report_only: boolean
  csp_report_uri: string
  x_frame_options: string
  x_content_type_options: boolean
  referrer_policy: string
  permissions_policy: string
  cross_origin_opener_policy: string
  cross_origin_resource_policy: string
  cross_origin_embedder_policy: string
  xss_protection: boolean
  cache_control_no_store: boolean
  security_score: number
  is_preset: boolean
  preset_type: string
  description: string
  created_at: string
  updated_at: string
}

export interface SecurityHeaderPreset {
  type: 'basic' | 'strict' | 'paranoid'
  name: string
  description: string
  score: number
  config: Partial<SecurityHeaderProfile>
}

export interface ScoreBreakdown {
  score: number
  max_score: number
  breakdown: Record<string, number>
  suggestions: string[]
}

export interface CSPDirective {
  directive: string
  values: string[]
}

// API Functions
export const listProfiles = async (): Promise<{ profiles: SecurityHeaderProfile[] }> => {
  const response = await client.get('/security/headers/profiles')
  return response.data
}

export const getProfile = async (id: number | string): Promise<{ profile: SecurityHeaderProfile }> => {
  const response = await client.get(`/security/headers/profiles/${id}`)
  return response.data
}

export const createProfile = async (data: Partial<SecurityHeaderProfile>): Promise<{ profile: SecurityHeaderProfile }> => {
  const response = await client.post('/security/headers/profiles', data)
  return response.data
}

export const updateProfile = async (id: number, data: Partial<SecurityHeaderProfile>): Promise<{ profile: SecurityHeaderProfile }> => {
  const response = await client.put(`/security/headers/profiles/${id}`, data)
  return response.data
}

export const deleteProfile = async (id: number): Promise<{ deleted: boolean }> => {
  const response = await client.delete(`/security/headers/profiles/${id}`)
  return response.data
}

export const getPresets = async (): Promise<{ presets: SecurityHeaderPreset[] }> => {
  const response = await client.get('/security/headers/presets')
  return response.data
}

export const applyPreset = async (presetType: string, name: string): Promise<{ profile: SecurityHeaderProfile }> => {
  const response = await client.post('/security/headers/presets/apply', { preset_type: presetType, name })
  return response.data
}

export const calculateScore = async (config: Partial<SecurityHeaderProfile>): Promise<ScoreBreakdown> => {
  const response = await client.post('/security/headers/score', config)
  return response.data
}

export const validateCSP = async (csp: string): Promise<{ valid: boolean; errors: string[] }> => {
  const response = await client.post('/security/headers/csp/validate', { csp })
  return response.data
}

export const buildCSP = async (directives: CSPDirective[]): Promise<{ csp: string }> => {
  const response = await client.post('/security/headers/csp/build', { directives })
  return response.data
}
```

### 4.2 React Query Hooks

**File**: `frontend/src/hooks/useSecurityHeaders.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import * as api from '../api/securityHeaders'
import toast from 'react-hot-toast'

export function useSecurityHeaderProfiles() {
  return useQuery({
    queryKey: ['securityHeaderProfiles'],
    queryFn: api.listProfiles,
  })
}

export function useSecurityHeaderProfile(id: number | string) {
  return useQuery({
    queryKey: ['securityHeaderProfile', id],
    queryFn: () => api.getProfile(id),
    enabled: !!id,
  })
}

export function useCreateSecurityHeaderProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.createProfile,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityHeaderProfiles'] })
      toast.success('Profile created')
    },
    onError: (err: Error) => {
      toast.error(`Failed to create profile: ${err.message}`)
    },
  })
}

export function useUpdateSecurityHeaderProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<api.SecurityHeaderProfile> }) =>
      api.updateProfile(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityHeaderProfiles'] })
      toast.success('Profile updated')
    },
    onError: (err: Error) => {
      toast.error(`Failed to update profile: ${err.message}`)
    },
  })
}

export function useDeleteSecurityHeaderProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.deleteProfile,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityHeaderProfiles'] })
      toast.success('Profile deleted')
    },
    onError: (err: Error) => {
      toast.error(`Failed to delete profile: ${err.message}`)
    },
  })
}

export function useSecurityHeaderPresets() {
  return useQuery({
    queryKey: ['securityHeaderPresets'],
    queryFn: api.getPresets,
  })
}

export function useApplySecurityHeaderPreset() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ presetType, name }: { presetType: string; name: string }) =>
      api.applyPreset(presetType, name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityHeaderProfiles'] })
      toast.success('Preset applied')
    },
    onError: (err: Error) => {
      toast.error(`Failed to apply preset: ${err.message}`)
    },
  })
}

export function useCalculateSecurityScore() {
  return useMutation({
    mutationFn: api.calculateScore,
  })
}

export function useValidateCSP() {
  return useMutation({
    mutationFn: api.validateCSP,
  })
}

export function useBuildCSP() {
  return useMutation({
    mutationFn: api.buildCSP,
  })
}
```

### 4.3 Security Headers Page

**File**: `frontend/src/pages/SecurityHeaders.tsx`

Main page with:

- List of custom profiles
- Preset cards (Basic, Strict, Paranoid)
- Create/Edit profile modal
- Security score display
- Delete confirmation dialog

### 4.4 Security Header Profile Form

**File**: `frontend/src/components/SecurityHeaderProfileForm.tsx`

Form component with:

- Name input
- HSTS section (enable, max-age, subdomains, preload)
- CSP section with builder
- X-Frame-Options selector
- X-Content-Type-Options toggle
- Referrer-Policy selector
- Permissions-Policy builder
- Cross-Origin headers section
- Live security score preview
- Save/Cancel buttons

### 4.5 CSP Builder Component

**File**: `frontend/src/components/CSPBuilder.tsx`

Interactive CSP builder:

- Directive selector dropdown
- Value input with suggestions
- Add/Remove directives
- Preview CSP string
- Validate button
- Common presets (default strict, allow scripts, etc.)

### 4.6 Security Score Display

**File**: `frontend/src/components/SecurityScoreDisplay.tsx`

Visual component showing:

- Circular progress indicator (0-100)
- Color coding (red < 50, yellow 50-75, green > 75)
- Score breakdown by category
- Suggestions for improvement

### 4.7 Permissions Policy Builder

**File**: `frontend/src/components/PermissionsPolicyBuilder.tsx`

Interactive builder for:

- Feature selector (camera, microphone, geolocation, etc.)
- Allowlist selector (none, self, all, specific origins)
- Preview policy string

### 4.8 Update ProxyHostForm

**File**: `frontend/src/components/ProxyHostForm.tsx`

Add to the form:

- "Security Headers" section
- Profile selector dropdown
- "Create New Profile" button
- Security score preview for selected profile
- Quick preset buttons

### 4.9 Unit Tests

**Files**:

- `frontend/src/hooks/__tests__/useSecurityHeaders.test.ts`
- `frontend/src/components/__tests__/SecurityHeaderProfileForm.test.tsx`
- `frontend/src/components/__tests__/CSPBuilder.test.tsx`
- `frontend/src/components/__tests__/SecurityScoreDisplay.test.tsx`
- `frontend/src/pages/__tests__/SecurityHeaders.test.tsx`

---

## Phase 5: Presets and Score Calculator

### 5.1 Built-in Presets

**File**: `backend/internal/services/security_headers_service.go`

```go
package services

// SecurityHeadersService manages security header profiles
type SecurityHeadersService struct {
    db *gorm.DB
}

// GetPresets returns the built-in presets
func (s *SecurityHeadersService) GetPresets() []models.SecurityHeaderProfile {
    return []models.SecurityHeaderProfile{
        {
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
func (s *SecurityHeadersService) EnsurePresetsExist() error
```

### 5.2 Security Score Calculator

**File**: `backend/internal/services/security_score.go`

```go
package services

import "strings"

// ScoreBreakdown represents the detailed score calculation
type ScoreBreakdown struct {
    TotalScore  int            `json:"score"`
    MaxScore    int            `json:"max_score"`
    Breakdown   map[string]int `json:"breakdown"`
    Suggestions []string       `json:"suggestions"`
}

// CalculateSecurityScore calculates the security score for a profile
func CalculateSecurityScore(profile *models.SecurityHeaderProfile) ScoreBreakdown {
    breakdown := make(map[string]int)
    suggestions := []string{}
    maxScore := 100

    // HSTS (25 points max)
    hstsScore := 0
    if profile.HSTSEnabled {
        hstsScore += 10
        if profile.HSTSMaxAge >= 31536000 {
            hstsScore += 5
        } else {
            suggestions = append(suggestions, "Increase HSTS max-age to at least 1 year")
        }
        if profile.HSTSIncludeSubdomains {
            hstsScore += 5
        } else {
            suggestions = append(suggestions, "Enable HSTS for subdomains")
        }
        if profile.HSTSPreload {
            hstsScore += 5
        } else {
            suggestions = append(suggestions, "Consider HSTS preload for browser preload lists")
        }
    } else {
        suggestions = append(suggestions, "Enable HSTS to enforce HTTPS")
    }
    breakdown["hsts"] = hstsScore

    // CSP (25 points max)
    cspScore := 0
    if profile.CSPEnabled {
        cspScore += 15
        // Additional points for strict CSP
        if !strings.Contains(profile.CSPDirectives, "'unsafe-inline'") {
            cspScore += 5
        } else {
            suggestions = append(suggestions, "Avoid 'unsafe-inline' in CSP for better security")
        }
        if !strings.Contains(profile.CSPDirectives, "'unsafe-eval'") {
            cspScore += 5
        } else {
            suggestions = append(suggestions, "Avoid 'unsafe-eval' in CSP for better security")
        }
    } else {
        suggestions = append(suggestions, "Enable Content-Security-Policy")
    }
    breakdown["csp"] = cspScore

    // X-Frame-Options (10 points)
    xfoScore := 0
    if profile.XFrameOptions == "DENY" {
        xfoScore = 10
    } else if profile.XFrameOptions == "SAMEORIGIN" {
        xfoScore = 7
    } else {
        suggestions = append(suggestions, "Set X-Frame-Options to DENY or SAMEORIGIN")
    }
    breakdown["x_frame_options"] = xfoScore

    // X-Content-Type-Options (10 points)
    xctoScore := 0
    if profile.XContentTypeOptions {
        xctoScore = 10
    } else {
        suggestions = append(suggestions, "Enable X-Content-Type-Options: nosniff")
    }
    breakdown["x_content_type_options"] = xctoScore

    // Referrer-Policy (10 points)
    rpScore := 0
    strictPolicies := []string{"no-referrer", "strict-origin", "strict-origin-when-cross-origin"}
    for _, p := range strictPolicies {
        if profile.ReferrerPolicy == p {
            rpScore = 10
            break
        }
    }
    if profile.ReferrerPolicy == "origin-when-cross-origin" {
        rpScore = 7
    }
    if rpScore == 0 && profile.ReferrerPolicy != "" {
        rpScore = 3
    }
    if rpScore < 10 {
        suggestions = append(suggestions, "Use a stricter Referrer-Policy")
    }
    breakdown["referrer_policy"] = rpScore

    // Permissions-Policy (10 points)
    ppScore := 0
    if profile.PermissionsPolicy != "" {
        ppScore = 10
    } else {
        suggestions = append(suggestions, "Add Permissions-Policy to restrict browser features")
    }
    breakdown["permissions_policy"] = ppScore

    // Cross-Origin headers (10 points)
    coScore := 0
    if profile.CrossOriginOpenerPolicy != "" {
        coScore += 4
    }
    if profile.CrossOriginResourcePolicy != "" {
        coScore += 3
    }
    if profile.CrossOriginEmbedderPolicy != "" {
        coScore += 3
    }
    if coScore < 10 {
        suggestions = append(suggestions, "Add Cross-Origin isolation headers")
    }
    breakdown["cross_origin"] = coScore

    // Calculate total
    total := hstsScore + cspScore + xfoScore + xctoScore + rpScore + ppScore + coScore

    return ScoreBreakdown{
        TotalScore:  total,
        MaxScore:    maxScore,
        Breakdown:   breakdown,
        Suggestions: suggestions,
    }
}
```

### 5.3 Initialize Presets on Startup

**File**: `backend/internal/api/routes/routes.go`

Add to the Register function:

```go
// Ensure security header presets exist
secHeadersSvc := services.NewSecurityHeadersService(db)
if err := secHeadersSvc.EnsurePresetsExist(); err != nil {
    logger.Log().WithError(err).Warn("Failed to initialize security header presets")
}
```

### 5.4 Unit Tests

**File**: `backend/internal/services/security_score_test.go`

Test cases:

- `TestCalculateScore_AllEnabled` - Full score
- `TestCalculateScore_HSTSOnly` - Partial score
- `TestCalculateScore_NoHeaders` - Zero score
- `TestCalculateScore_UnsafeCSP` - CSP with unsafe directives
- `TestCalculateScore_Suggestions` - Verify suggestions generated

**File**: `backend/internal/services/security_headers_service_test.go`

Test cases:

- `TestGetPresets` - Returns all presets
- `TestEnsurePresetsExist_Creates` - Creates presets when missing
- `TestEnsurePresetsExist_NoOp` - Doesn't duplicate presets

---

## Phase 6: Route Registration and Final Integration

### 6.1 Frontend Routes

**File**: `frontend/src/App.tsx`

Add routes:

```tsx
<Route path="/security/headers" element={<SecurityHeaders />} />
<Route path="/security/headers/:id" element={<SecurityHeadersEdit />} />
<Route path="/security/headers/new" element={<SecurityHeadersCreate />} />
```

### 6.2 Navigation Update

Add "Security Headers" to the Security section in the navigation.

### 6.3 Integration Test

**File**: `backend/integration/security_headers_test.go`

End-to-end test:

1. Create a security header profile
2. Assign to a proxy host
3. Generate Caddy config
4. Verify headers in generated config

---

## Implementation Summary

### Files to Create

| Phase | File Path | Description |
|-------|-----------|-------------|
| 1 | `backend/internal/models/security_header_profile.go` | New model |
| 1 | `backend/internal/models/security_header_profile_test.go` | Model tests |
| 2 | `backend/internal/api/handlers/security_headers_handler.go` | API handlers |
| 2 | `backend/internal/api/handlers/security_headers_handler_test.go` | Handler tests |
| 3 | `backend/internal/caddy/config_security_headers_test.go` | Caddy integration tests |
| 4 | `frontend/src/api/securityHeaders.ts` | API client |
| 4 | `frontend/src/hooks/useSecurityHeaders.ts` | React Query hooks |
| 4 | `frontend/src/pages/SecurityHeaders.tsx` | Main page |
| 4 | `frontend/src/components/SecurityHeaderProfileForm.tsx` | Profile form |
| 4 | `frontend/src/components/CSPBuilder.tsx` | CSP builder |
| 4 | `frontend/src/components/SecurityScoreDisplay.tsx` | Score display |
| 4 | `frontend/src/components/PermissionsPolicyBuilder.tsx` | Permissions builder |
| 5 | `backend/internal/services/security_headers_service.go` | Service layer |
| 5 | `backend/internal/services/security_score.go` | Score calculator |
| 5 | `backend/internal/services/security_score_test.go` | Score tests |
| 5 | `backend/internal/services/security_headers_service_test.go` | Service tests |

### Files to Modify

| Phase | File Path | Changes |
|-------|-----------|---------|
| 1 | `backend/internal/models/proxy_host.go` | Add security header fields |
| 1 | `backend/internal/api/routes/routes.go` | Add AutoMigrate |
| 2 | `backend/internal/api/routes/routes.go` | Register handlers |
| 3 | `backend/internal/caddy/config.go` | Add header handler builder |
| 3 | `backend/internal/caddy/manager.go` | Preload profiles |
| 4 | `frontend/src/components/ProxyHostForm.tsx` | Add profile selector |
| 4 | `frontend/src/App.tsx` | Add routes |
| 6 | Navigation component | Add menu item |

---

## Configuration File Updates

### .gitignore

No changes required - existing patterns cover new files.

### .codecov.yml

No changes required - existing patterns cover new test files.

### .dockerignore

No changes required - existing patterns cover new files.

### Dockerfile

No changes required - no new build dependencies needed.

---

## Acceptance Criteria Checklist

| Criteria | Phase | Implementation |
|----------|-------|----------------|
| ✅ Security headers automatically added | Phase 3 | `buildSecurityHeadersHandler()` |
| ✅ CSP configurable without breaking sites | Phase 4 | `CSPBuilder.tsx` with validation |
| ✅ Presets available for easy setup | Phase 5 | Basic, Strict, Paranoid presets |
| ✅ Security score shown in UI | Phase 4 | `SecurityScoreDisplay.tsx` |
| ✅ HSTS with preload support | Phase 1 | Model fields for all HSTS options |
| ✅ Content-Security-Policy builder | Phase 4 | `CSPBuilder.tsx` |
| ✅ X-Frame-Options (DENY/SAMEORIGIN) | Phase 1 | Model field with validation |
| ✅ X-Content-Type-Options (nosniff) | Phase 1 | Model boolean field |
| ✅ Referrer-Policy configuration | Phase 1 | Model field with enum values |
| ✅ Permissions-Policy headers | Phase 1/4 | Model + `PermissionsPolicyBuilder.tsx` |
| ✅ Security header presets | Phase 5 | basic, strict, paranoid |
| ✅ Security score calculator | Phase 5 | `CalculateSecurityScore()` |

---

## Testing Requirements

### Backend Unit Tests (Target: 85%+ coverage)

- Model CRUD operations
- Handler request/response validation
- Score calculation accuracy
- CSP string building
- Permissions-Policy string building
- Caddy config generation with headers

### Frontend Unit Tests (Target: 85%+ coverage)

- Component rendering
- Form validation
- API hook behavior
- Score display logic
- CSP builder interactions

### Integration Tests

- End-to-end profile creation and application
- Caddy config generation verification
- Preset application flow

---

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| CSP breaks existing sites | CSP disabled by default, builder has validation |
| HSTS preload locked forever | Warning in UI, preload disabled by default |
| Performance impact | Headers cached per-host, no per-request calculation |
| Migration issues | New table, no schema changes to existing tables |

---

## Next Steps

1. **Phase 1**: Create model and migration (1 day)
2. **Phase 2**: Implement API handlers (1-2 days)
3. **Phase 3**: Caddy integration (1 day)
4. **Phase 4**: Frontend components (2-3 days)
5. **Phase 5**: Presets and score calculator (1 day)
6. **Phase 6**: Final integration and testing (1 day)

**Total Estimated Time**: 7-9 days

---

*Plan created by Planning Agent for Issue #20*
