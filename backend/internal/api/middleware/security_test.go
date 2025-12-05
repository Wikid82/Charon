package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		isDevelopment bool
		checkHeaders  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:          "production mode sets HSTS",
			isDevelopment: false,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				hsts := resp.Header().Get("Strict-Transport-Security")
				assert.Contains(t, hsts, "max-age=31536000")
				assert.Contains(t, hsts, "includeSubDomains")
				assert.Contains(t, hsts, "preload")
			},
		},
		{
			name:          "development mode skips HSTS",
			isDevelopment: true,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				hsts := resp.Header().Get("Strict-Transport-Security")
				assert.Empty(t, hsts)
			},
		},
		{
			name:          "sets X-Frame-Options",
			isDevelopment: false,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, "DENY", resp.Header().Get("X-Frame-Options"))
			},
		},
		{
			name:          "sets X-Content-Type-Options",
			isDevelopment: false,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, "nosniff", resp.Header().Get("X-Content-Type-Options"))
			},
		},
		{
			name:          "sets X-XSS-Protection",
			isDevelopment: false,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, "1; mode=block", resp.Header().Get("X-XSS-Protection"))
			},
		},
		{
			name:          "sets Referrer-Policy",
			isDevelopment: false,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, "strict-origin-when-cross-origin", resp.Header().Get("Referrer-Policy"))
			},
		},
		{
			name:          "sets Content-Security-Policy",
			isDevelopment: false,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				csp := resp.Header().Get("Content-Security-Policy")
				assert.NotEmpty(t, csp)
				assert.Contains(t, csp, "default-src")
			},
		},
		{
			name:          "development mode CSP allows unsafe-eval",
			isDevelopment: true,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				csp := resp.Header().Get("Content-Security-Policy")
				assert.Contains(t, csp, "unsafe-eval")
			},
		},
		{
			name:          "sets Permissions-Policy",
			isDevelopment: false,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				pp := resp.Header().Get("Permissions-Policy")
				assert.NotEmpty(t, pp)
				assert.Contains(t, pp, "camera=()")
				assert.Contains(t, pp, "microphone=()")
			},
		},
		{
			name:          "sets Cross-Origin-Opener-Policy",
			isDevelopment: false,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, "same-origin", resp.Header().Get("Cross-Origin-Opener-Policy"))
			},
		},
		{
			name:          "sets Cross-Origin-Resource-Policy",
			isDevelopment: false,
			checkHeaders: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, "same-origin", resp.Header().Get("Cross-Origin-Resource-Policy"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(SecurityHeaders(SecurityHeadersConfig{
				IsDevelopment: tt.isDevelopment,
			}))
			router.GET("/test", func(c *gin.Context) {
				c.String(http.StatusOK, "OK")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
			tt.checkHeaders(t, resp)
		})
	}
}

func TestSecurityHeadersCustomCSP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders(SecurityHeadersConfig{
		IsDevelopment: false,
		CustomCSPDirectives: map[string]string{
			"frame-src": "'self' https://trusted.com",
		},
	}))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	csp := resp.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "frame-src 'self' https://trusted.com")
}

func TestDefaultSecurityHeadersConfig(t *testing.T) {
	cfg := DefaultSecurityHeadersConfig()
	assert.False(t, cfg.IsDevelopment)
	assert.Nil(t, cfg.CustomCSPDirectives)
}

func TestBuildCSP(t *testing.T) {
	t.Run("production CSP", func(t *testing.T) {
		csp := buildCSP(SecurityHeadersConfig{IsDevelopment: false})
		assert.Contains(t, csp, "default-src 'self'")
		assert.Contains(t, csp, "script-src 'self'")
		assert.NotContains(t, csp, "unsafe-eval")
	})

	t.Run("development CSP", func(t *testing.T) {
		csp := buildCSP(SecurityHeadersConfig{IsDevelopment: true})
		assert.Contains(t, csp, "unsafe-eval")
		assert.Contains(t, csp, "ws:")
	})
}

func TestBuildPermissionsPolicy(t *testing.T) {
	pp := buildPermissionsPolicy()

	// Check that dangerous features are disabled
	disabledFeatures := []string{"camera", "microphone", "geolocation", "payment"}
	for _, feature := range disabledFeatures {
		assert.True(t, strings.Contains(pp, feature+"=()"),
			"Expected %s to be disabled in permissions policy", feature)
	}
}
