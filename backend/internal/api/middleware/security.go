package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersConfig holds configuration for the security headers middleware.
type SecurityHeadersConfig struct {
	// IsDevelopment enables less strict settings for local development
	IsDevelopment bool
	// CustomCSPDirectives allows adding extra CSP directives
	CustomCSPDirectives map[string]string
}

// DefaultSecurityHeadersConfig returns a secure default configuration.
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		IsDevelopment:       false,
		CustomCSPDirectives: nil,
	}
}

// SecurityHeaders returns middleware that sets security-related HTTP headers.
// This implements Phase 1 of the security hardening plan.
func SecurityHeaders(cfg SecurityHeadersConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Build Content-Security-Policy
		csp := buildCSP(cfg)
		c.Header("Content-Security-Policy", csp)

		// Strict-Transport-Security (HSTS)
		// max-age=31536000 = 1 year
		// includeSubDomains ensures all subdomains also use HTTPS
		// preload allows browser preload lists (requires submission to hstspreload.org)
		if !cfg.IsDevelopment {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// X-Frame-Options: Prevent clickjacking
		// DENY prevents any framing; SAMEORIGIN would allow same-origin framing
		c.Header("X-Frame-Options", "DENY")

		// X-Content-Type-Options: Prevent MIME sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// X-XSS-Protection: Enable browser XSS filtering (legacy but still useful)
		// mode=block tells browser to block the response if XSS is detected
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer-Policy: Control referrer information sent with requests
		// strict-origin-when-cross-origin sends full URL for same-origin, origin only for cross-origin
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions-Policy: Restrict browser features
		// Disable features that aren't needed for security
		c.Header("Permissions-Policy", buildPermissionsPolicy())

		// Cross-Origin-Opener-Policy: Isolate browsing context
		c.Header("Cross-Origin-Opener-Policy", "same-origin")

		// Cross-Origin-Resource-Policy: Prevent cross-origin reads
		c.Header("Cross-Origin-Resource-Policy", "same-origin")

		// Cross-Origin-Embedder-Policy: Require CORP for cross-origin resources
		// Note: This can break some external resources, use with caution
		// c.Header("Cross-Origin-Embedder-Policy", "require-corp")

		c.Next()
	}
}

// buildCSP constructs the Content-Security-Policy header value.
func buildCSP(cfg SecurityHeadersConfig) string {
	// Base CSP directives for a secure single-page application
	directives := map[string]string{
		"default-src": "'self'",
		"script-src":  "'self'",
		"style-src":   "'self' 'unsafe-inline'", // unsafe-inline needed for many CSS-in-JS solutions
		"img-src":     "'self' data: https:",    // Allow HTTPS images and data URIs
		"font-src":    "'self' data:",           // Allow self-hosted fonts and data URIs
		"connect-src": "'self'",                 // API connections
		"frame-src":   "'none'",                 // No iframes
		"object-src":  "'none'",                 // No plugins (Flash, etc.)
		"base-uri":    "'self'",                 // Restrict base tag
		"form-action": "'self'",                 // Restrict form submissions
	}

	// In development, allow more sources for hot reloading, etc.
	if cfg.IsDevelopment {
		directives["script-src"] = "'self' 'unsafe-inline' 'unsafe-eval'"
		directives["connect-src"] = "'self' ws: wss:" // WebSocket for HMR
	}

	// Apply custom directives
	for key, value := range cfg.CustomCSPDirectives {
		directives[key] = value
	}

	// Build the CSP string
	var parts []string
	for directive, value := range directives {
		parts = append(parts, fmt.Sprintf("%s %s", directive, value))
	}

	return strings.Join(parts, "; ")
}

// buildPermissionsPolicy constructs the Permissions-Policy header value.
func buildPermissionsPolicy() string {
	// Disable features we don't need
	policies := []string{
		"accelerometer=()",
		"camera=()",
		"geolocation=()",
		"gyroscope=()",
		"magnetometer=()",
		"microphone=()",
		"payment=()",
		"usb=()",
	}

	return strings.Join(policies, ", ")
}
