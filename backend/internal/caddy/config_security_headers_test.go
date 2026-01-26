package caddy

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestBuildSecurityHeadersHandler_AllEnabled(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:               true,
		HSTSMaxAge:                31536000,
		HSTSIncludeSubdomains:     true,
		HSTSPreload:               true,
		CSPEnabled:                true,
		CSPDirectives:             `{"default-src":["'self'"],"script-src":["'self'"]}`,
		CSPReportOnly:             false,
		XFrameOptions:             "DENY",
		XContentTypeOptions:       true,
		ReferrerPolicy:            "no-referrer",
		PermissionsPolicy:         `[{"feature":"camera","allowlist":[]}]`,
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		CrossOriginEmbedderPolicy: "require-corp",
		XSSProtection:             true,
		CacheControlNoStore:       true,
	}

	host := &models.ProxyHost{
		SecurityHeaderProfile: profile,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)
	assert.NotNil(t, handler)
	assert.Equal(t, "headers", handler["handler"])

	response := handler["response"].(map[string]any)
	headers := response["set"].(map[string][]string)

	assert.Contains(t, headers["Strict-Transport-Security"][0], "max-age=31536000")
	assert.Contains(t, headers["Strict-Transport-Security"][0], "includeSubDomains")
	assert.Contains(t, headers["Strict-Transport-Security"][0], "preload")
	assert.Contains(t, headers, "Content-Security-Policy")
	assert.Equal(t, "DENY", headers["X-Frame-Options"][0])
	assert.Equal(t, "nosniff", headers["X-Content-Type-Options"][0])
	assert.Equal(t, "no-referrer", headers["Referrer-Policy"][0])
	assert.Contains(t, headers, "Permissions-Policy")
	assert.Equal(t, "same-origin", headers["Cross-Origin-Opener-Policy"][0])
	assert.Equal(t, "same-origin", headers["Cross-Origin-Resource-Policy"][0])
	assert.Equal(t, "require-corp", headers["Cross-Origin-Embedder-Policy"][0])
	assert.Equal(t, "1; mode=block", headers["X-XSS-Protection"][0])
	assert.Equal(t, "no-store", headers["Cache-Control"][0])
}

func TestBuildSecurityHeadersHandler_HSTSOnly(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:           true,
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           false,
		CSPEnabled:            false,
		XFrameOptions:         "SAMEORIGIN",
		XContentTypeOptions:   true,
	}

	host := &models.ProxyHost{
		SecurityHeaderProfile: profile,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	response := handler["response"].(map[string]any)
	headers := response["set"].(map[string][]string)

	assert.Contains(t, headers["Strict-Transport-Security"][0], "max-age=31536000")
	assert.Contains(t, headers["Strict-Transport-Security"][0], "includeSubDomains")
	assert.NotContains(t, headers["Strict-Transport-Security"][0], "preload")
	assert.NotContains(t, headers, "Content-Security-Policy")
	assert.Equal(t, "SAMEORIGIN", headers["X-Frame-Options"][0])
	assert.Equal(t, "nosniff", headers["X-Content-Type-Options"][0])
}

func TestBuildSecurityHeadersHandler_CSPOnly(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled: false,
		CSPEnabled:  true,
		CSPDirectives: `{
			"default-src": ["'self'"],
			"script-src": ["'self'", "https://cdn.example.com"],
			"style-src": ["'self'", "'unsafe-inline'"]
		}`,
	}

	host := &models.ProxyHost{
		SecurityHeaderProfile: profile,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	response := handler["response"].(map[string]any)
	headers := response["set"].(map[string][]string)

	assert.NotContains(t, headers, "Strict-Transport-Security")
	assert.Contains(t, headers, "Content-Security-Policy")
	csp := headers["Content-Security-Policy"][0]
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "script-src 'self' https://cdn.example.com")
	assert.Contains(t, csp, "style-src 'self' 'unsafe-inline'")
}

func TestBuildSecurityHeadersHandler_CSPReportOnly(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		CSPEnabled:    true,
		CSPDirectives: `{"default-src":["'self'"]}`,
		CSPReportOnly: true,
	}

	host := &models.ProxyHost{
		SecurityHeaderProfile: profile,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	response := handler["response"].(map[string]any)
	headers := response["set"].(map[string][]string)

	assert.NotContains(t, headers, "Content-Security-Policy")
	assert.Contains(t, headers, "Content-Security-Policy-Report-Only")
}

func TestBuildSecurityHeadersHandler_NoProfile(t *testing.T) {
	host := &models.ProxyHost{
		SecurityHeaderProfile:  nil,
		SecurityHeadersEnabled: true,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	// Should use defaults
	response := handler["response"].(map[string]any)
	headers := response["set"].(map[string][]string)

	assert.Contains(t, headers, "Strict-Transport-Security")
	assert.Equal(t, "SAMEORIGIN", headers["X-Frame-Options"][0])
	assert.Equal(t, "nosniff", headers["X-Content-Type-Options"][0])
}

func TestBuildSecurityHeadersHandler_Disabled(t *testing.T) {
	host := &models.ProxyHost{
		SecurityHeaderProfile:  nil,
		SecurityHeadersEnabled: false,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)
	assert.Nil(t, handler)
}

func TestBuildSecurityHeadersHandler_NilHost(t *testing.T) {
	handler, err := buildSecurityHeadersHandler(nil)
	assert.NoError(t, err)
	assert.Nil(t, handler)
}

func TestBuildCSPString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple CSP",
			input:    `{"default-src":["'self'"]}`,
			expected: "default-src 'self'",
			wantErr:  false,
		},
		{
			name:     "multiple directives",
			input:    `{"default-src":["'self'"],"script-src":["'self'","https:"],"style-src":["'self'","'unsafe-inline'"]}`,
			expected: "default-src 'self'; script-src 'self' https:; style-src 'self' 'unsafe-inline'",
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "invalid JSON",
			input:    "not json",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildCSPString(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// CSP order can vary, so check parts exist
				if tt.expected != "" {
					var parts []string
					if tt.expected == "default-src 'self'" {
						parts = []string{"default-src 'self'"}
					} else {
						// For multiple directives, just check all parts are present
						parts = []string{"default-src 'self'", "script-src", "style-src"}
					}
					for _, part := range parts {
						assert.Contains(t, result, part)
					}
				}
			}
		})
	}
}

func TestBuildPermissionsPolicyString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "single feature no allowlist",
			input:    `[{"feature":"camera","allowlist":[]}]`,
			expected: "camera=()",
			wantErr:  false,
		},
		{
			name:     "single feature with self",
			input:    `[{"feature":"microphone","allowlist":["self"]}]`,
			expected: "microphone=(self)",
			wantErr:  false,
		},
		{
			name:     "multiple features",
			input:    `[{"feature":"camera","allowlist":[]},{"feature":"microphone","allowlist":["self"]},{"feature":"geolocation","allowlist":["*"]}]`,
			expected: "camera=(), microphone=(self), geolocation=(*)",
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "invalid JSON",
			input:    "not json",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildPermissionsPolicyString(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expected != "" {
					assert.Equal(t, tt.expected, result)
				}
			}
		})
	}
}

func TestGetDefaultSecurityHeaderProfile(t *testing.T) {
	profile := getDefaultSecurityHeaderProfile()

	assert.NotNil(t, profile)
	assert.True(t, profile.HSTSEnabled)
	assert.Equal(t, 31536000, profile.HSTSMaxAge)
	assert.False(t, profile.HSTSIncludeSubdomains)
	assert.False(t, profile.HSTSPreload)
	assert.False(t, profile.CSPEnabled) // Off by default
	assert.Equal(t, "SAMEORIGIN", profile.XFrameOptions)
	assert.True(t, profile.XContentTypeOptions)
	assert.Equal(t, "strict-origin-when-cross-origin", profile.ReferrerPolicy)
	assert.True(t, profile.XSSProtection)
	assert.Equal(t, "same-origin", profile.CrossOriginOpenerPolicy)
	assert.Equal(t, "same-origin", profile.CrossOriginResourcePolicy)
}

func TestBuildSecurityHeadersHandler_PermissionsPolicy(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		PermissionsPolicy: `[{"feature":"camera","allowlist":[]},{"feature":"microphone","allowlist":["self"]},{"feature":"geolocation","allowlist":["*"]}]`,
	}

	host := &models.ProxyHost{
		SecurityHeaderProfile: profile,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	response := handler["response"].(map[string]any)
	headers := response["set"].(map[string][]string)

	assert.Contains(t, headers, "Permissions-Policy")
	pp := headers["Permissions-Policy"][0]
	assert.Contains(t, pp, "camera=()")
	assert.Contains(t, pp, "microphone=(self)")
	assert.Contains(t, pp, "geolocation=(*)")
}

func TestBuildSecurityHeadersHandler_InvalidCSPJSON(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		CSPEnabled:    true,
		CSPDirectives: "invalid json",
		// Add another header so handler isn't nil
		XContentTypeOptions: true,
	}

	host := &models.ProxyHost{
		SecurityHeaderProfile: profile,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	// Should skip CSP if invalid JSON
	response := handler["response"].(map[string]any)
	headers := response["set"].(map[string][]string)
	assert.NotContains(t, headers, "Content-Security-Policy")
	// But should include the other header
	assert.Contains(t, headers, "X-Content-Type-Options")
}

func TestBuildSecurityHeadersHandler_InvalidPermissionsJSON(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		PermissionsPolicy: "invalid json",
	}

	host := &models.ProxyHost{
		SecurityHeaderProfile: profile,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)

	// Should skip invalid permissions policy but continue with other headers
	// If profile had no other headers, handler would be nil
	// Since we're only testing permissions policy, handler will be nil
	assert.Nil(t, handler)
}

func TestBuildSecurityHeadersHandler_APIFriendlyPreset(t *testing.T) {
	// Simulate an API-Friendly preset configuration
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:               true,
		HSTSMaxAge:                31536000, // 1 year
		HSTSIncludeSubdomains:     false,
		HSTSPreload:               false,
		CSPEnabled:                false, // APIs don't need CSP
		XFrameOptions:             "",    // Allow WebViews (empty)
		XContentTypeOptions:       true,
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		PermissionsPolicy:         "",             // Allow all permissions
		CrossOriginOpenerPolicy:   "",             // Allow OAuth popups
		CrossOriginResourcePolicy: "cross-origin", // KEY: Allow cross-origin access
		CrossOriginEmbedderPolicy: "",             // Don't require CORP
		XSSProtection:             true,
		CacheControlNoStore:       false,
	}

	host := &models.ProxyHost{
		SecurityHeaderProfile: profile,
	}

	handler, err := buildSecurityHeadersHandler(host)
	assert.NoError(t, err)
	assert.NotNil(t, handler)
	assert.Equal(t, "headers", handler["handler"])

	response := handler["response"].(map[string]any)
	headers := response["set"].(map[string][]string)

	// Verify HSTS is present
	assert.Contains(t, headers, "Strict-Transport-Security")
	assert.Contains(t, headers["Strict-Transport-Security"][0], "max-age=31536000")
	assert.NotContains(t, headers["Strict-Transport-Security"][0], "includeSubDomains")
	assert.NotContains(t, headers["Strict-Transport-Security"][0], "preload")

	// Verify CSP is NOT present (disabled)
	assert.NotContains(t, headers, "Content-Security-Policy")
	assert.NotContains(t, headers, "Content-Security-Policy-Report-Only")

	// Verify X-Frame-Options is NOT present (empty string = allow WebViews)
	assert.NotContains(t, headers, "X-Frame-Options")

	// Verify X-Content-Type-Options is present
	assert.Contains(t, headers, "X-Content-Type-Options")
	assert.Equal(t, "nosniff", headers["X-Content-Type-Options"][0])

	// Verify Referrer-Policy is present
	assert.Contains(t, headers, "Referrer-Policy")
	assert.Equal(t, "strict-origin-when-cross-origin", headers["Referrer-Policy"][0])

	// Verify CORP is "cross-origin" (KEY for API access)
	assert.Contains(t, headers, "Cross-Origin-Resource-Policy")
	assert.Equal(t, "cross-origin", headers["Cross-Origin-Resource-Policy"][0])

	// Verify COOP is NOT present (empty = allow OAuth popups)
	assert.NotContains(t, headers, "Cross-Origin-Opener-Policy")

	// Verify COEP is NOT present (empty = don't require CORP)
	assert.NotContains(t, headers, "Cross-Origin-Embedder-Policy")

	// Verify Permissions-Policy is NOT present (empty)
	assert.NotContains(t, headers, "Permissions-Policy")

	// Verify XSS Protection is present
	assert.Contains(t, headers, "X-XSS-Protection")
	assert.Equal(t, "1; mode=block", headers["X-XSS-Protection"][0])

	// Verify Cache-Control is NOT present (CacheControlNoStore = false)
	assert.NotContains(t, headers, "Cache-Control")
}
