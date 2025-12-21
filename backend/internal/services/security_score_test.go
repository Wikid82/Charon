package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestCalculateSecurityScore_AllEnabled(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:               true,
		HSTSMaxAge:                63072000,
		HSTSIncludeSubdomains:     true,
		HSTSPreload:               true,
		CSPEnabled:                true,
		CSPDirectives:             `{"default-src":["'self'"],"script-src":["'self'"],"style-src":["'self'"]}`,
		XFrameOptions:             "DENY",
		XContentTypeOptions:       true,
		ReferrerPolicy:            "no-referrer",
		PermissionsPolicy:         `[{"feature":"camera","allowlist":[]}]`,
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		CrossOriginEmbedderPolicy: "require-corp",
	}

	result := CalculateSecurityScore(profile)

	assert.Equal(t, 100, result.TotalScore)
	assert.Equal(t, 100, result.MaxScore)
	assert.Equal(t, 25, result.Breakdown["hsts"])
	assert.Equal(t, 25, result.Breakdown["csp"])
	assert.Equal(t, 10, result.Breakdown["x_frame_options"])
	assert.Equal(t, 10, result.Breakdown["x_content_type_options"])
	assert.Equal(t, 10, result.Breakdown["referrer_policy"])
	assert.Equal(t, 10, result.Breakdown["permissions_policy"])
	assert.Equal(t, 10, result.Breakdown["cross_origin"])
	assert.Empty(t, result.Suggestions)
}

func TestCalculateSecurityScore_HSTSOnly(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:           true,
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           false,
		CSPEnabled:            false,
		XFrameOptions:         "SAMEORIGIN",
		XContentTypeOptions:   true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
	}

	result := CalculateSecurityScore(profile)

	assert.Equal(t, 20, result.Breakdown["hsts"]) // 10 + 5 + 5, missing preload
	assert.Equal(t, 0, result.Breakdown["csp"])
	assert.Equal(t, 7, result.Breakdown["x_frame_options"]) // SAMEORIGIN = 7 points
	assert.Equal(t, 10, result.Breakdown["x_content_type_options"])
	assert.Equal(t, 10, result.Breakdown["referrer_policy"])
	assert.Equal(t, 0, result.Breakdown["permissions_policy"])
	assert.Equal(t, 0, result.Breakdown["cross_origin"])
	assert.Contains(t, result.Suggestions, "Consider HSTS preload for browser preload lists")
	assert.Contains(t, result.Suggestions, "Enable Content-Security-Policy")
}

func TestCalculateSecurityScore_NoHeaders(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:         false,
		CSPEnabled:          false,
		XFrameOptions:       "",
		XContentTypeOptions: false,
		ReferrerPolicy:      "",
	}

	result := CalculateSecurityScore(profile)

	assert.Equal(t, 0, result.TotalScore)
	assert.Equal(t, 100, result.MaxScore)
	assert.Contains(t, result.Suggestions, "Enable HSTS to enforce HTTPS")
	assert.Contains(t, result.Suggestions, "Enable Content-Security-Policy")
	assert.Contains(t, result.Suggestions, "Set X-Frame-Options to DENY or SAMEORIGIN")
	assert.Contains(t, result.Suggestions, "Enable X-Content-Type-Options: nosniff")
}

func TestCalculateSecurityScore_UnsafeCSP(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:         true,
		HSTSMaxAge:          31536000,
		CSPEnabled:          true,
		CSPDirectives:       `{"default-src":["'self'"],"script-src":["'self'","'unsafe-inline'","'unsafe-eval'"]}`,
		XFrameOptions:       "DENY",
		XContentTypeOptions: true,
		ReferrerPolicy:      "strict-origin",
	}

	result := CalculateSecurityScore(profile)

	assert.Equal(t, 15, result.Breakdown["csp"]) // Base 15, no bonus for unsafe directives
	assert.Contains(t, result.Suggestions, "Avoid 'unsafe-inline' in CSP for better security")
	assert.Contains(t, result.Suggestions, "Avoid 'unsafe-eval' in CSP for better security")
}

func TestCalculateSecurityScore_PartialCrossOrigin(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:               true,
		HSTSMaxAge:                31536000,
		CSPEnabled:                false,
		XFrameOptions:             "DENY",
		XContentTypeOptions:       true,
		ReferrerPolicy:            "strict-origin",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
	}

	result := CalculateSecurityScore(profile)

	assert.Equal(t, 7, result.Breakdown["cross_origin"]) // 4 + 3, missing embedder
	assert.Contains(t, result.Suggestions, "Add Cross-Origin isolation headers")
}

func TestCalculateSecurityScore_WeakReferrerPolicy(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:         true,
		HSTSMaxAge:          31536000,
		XFrameOptions:       "DENY",
		XContentTypeOptions: true,
		ReferrerPolicy:      "origin-when-cross-origin",
	}

	result := CalculateSecurityScore(profile)

	assert.Equal(t, 7, result.Breakdown["referrer_policy"])
	assert.Contains(t, result.Suggestions, "Use a stricter Referrer-Policy")
}

func TestCalculateSecurityScore_UnknownReferrerPolicy(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:         true,
		HSTSMaxAge:          31536000,
		XFrameOptions:       "DENY",
		XContentTypeOptions: true,
		ReferrerPolicy:      "unsafe-url",
	}

	result := CalculateSecurityScore(profile)

	assert.Equal(t, 3, result.Breakdown["referrer_policy"]) // Non-empty but not strict
	assert.Contains(t, result.Suggestions, "Use a stricter Referrer-Policy")
}

func TestCalculateSecurityScore_ShortHSTSMaxAge(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:           true,
		HSTSMaxAge:            86400, // 1 day - too short
		HSTSIncludeSubdomains: false,
		XFrameOptions:         "DENY",
		XContentTypeOptions:   true,
		ReferrerPolicy:        "strict-origin",
	}

	result := CalculateSecurityScore(profile)

	assert.Equal(t, 10, result.Breakdown["hsts"]) // Only base score, no bonus
	assert.Contains(t, result.Suggestions, "Increase HSTS max-age to at least 1 year")
	assert.Contains(t, result.Suggestions, "Enable HSTS for subdomains")
}
