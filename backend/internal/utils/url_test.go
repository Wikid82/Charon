package utils

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "failed to connect to test database")

	// Auto-migrate the Setting model
	err = db.AutoMigrate(&models.Setting{})
	require.NoError(t, err, "failed to migrate database")

	return db
}

// TestGetPublicURL_WithConfiguredURL verifies retrieval of configured public URL
func TestGetPublicURL_WithConfiguredURL(t *testing.T) {
	db := setupTestDB(t)

	// Insert a configured public URL
	setting := models.Setting{
		Key:   "app.public_url",
		Value: "https://example.com/",
	}
	err := db.Create(&setting).Error
	require.NoError(t, err)

	// Create test Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/test", http.NoBody)
	c.Request = req

	// Test GetPublicURL
	publicURL := GetPublicURL(db, c)

	// Should return configured URL with trailing slash removed
	assert.Equal(t, "https://example.com", publicURL)
}

// TestGetPublicURL_WithTrailingSlash verifies trailing slash removal
func TestGetPublicURL_WithTrailingSlash(t *testing.T) {
	db := setupTestDB(t)

	// Insert URL with multiple trailing slashes
	setting := models.Setting{
		Key:   "app.public_url",
		Value: "https://example.com///",
	}
	err := db.Create(&setting).Error
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/test", http.NoBody)
	c.Request = req

	publicURL := GetPublicURL(db, c)

	// Should remove only the trailing slash (TrimSuffix removes one slash)
	assert.Equal(t, "https://example.com//", publicURL)
}

// TestGetPublicURL_Fallback_HTTPSWithTLS verifies fallback to request URL with TLS
func TestGetPublicURL_Fallback_HTTPSWithTLS(t *testing.T) {
	db := setupTestDB(t)
	// No setting in DB - should fallback

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create request with TLS
	req := httptest.NewRequest(http.MethodGet, "https://myapp.com:8443/path", http.NoBody)
	req.TLS = &tls.ConnectionState{} // Simulate TLS connection
	c.Request = req

	publicURL := GetPublicURL(db, c)

	// Should detect TLS and use https
	assert.Equal(t, "https://myapp.com:8443", publicURL)
}

// TestGetPublicURL_Fallback_HTTP verifies fallback to HTTP when no TLS
func TestGetPublicURL_Fallback_HTTP(t *testing.T) {
	db := setupTestDB(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/test", http.NoBody)
	c.Request = req

	publicURL := GetPublicURL(db, c)

	// Should use http scheme when no TLS
	assert.Equal(t, "http://localhost:8080", publicURL)
}

// TestGetPublicURL_Fallback_XForwardedProto verifies X-Forwarded-Proto header handling
func TestGetPublicURL_Fallback_XForwardedProto(t *testing.T) {
	db := setupTestDB(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://internal-server:8080/test", http.NoBody)
	req.Header.Set("X-Forwarded-Proto", "https")
	c.Request = req

	publicURL := GetPublicURL(db, c)

	// Should respect X-Forwarded-Proto header
	assert.Equal(t, "https://internal-server:8080", publicURL)
}

// TestGetPublicURL_EmptyValue verifies behavior with empty setting value
func TestGetPublicURL_EmptyValue(t *testing.T) {
	db := setupTestDB(t)

	// Insert setting with empty value
	setting := models.Setting{
		Key:   "app.public_url",
		Value: "",
	}
	err := db.Create(&setting).Error
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://localhost:9000/test", http.NoBody)
	c.Request = req

	publicURL := GetPublicURL(db, c)

	// Should fallback to request URL when value is empty
	assert.Equal(t, "http://localhost:9000", publicURL)
}

// TestGetPublicURL_NoSettingInDB verifies behavior when setting doesn't exist
func TestGetPublicURL_NoSettingInDB(t *testing.T) {
	db := setupTestDB(t)
	// No setting created - should fallback

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://fallback-host.com/test", http.NoBody)
	c.Request = req

	publicURL := GetPublicURL(db, c)

	// Should fallback to request host
	assert.Equal(t, "http://fallback-host.com", publicURL)
}

// TestValidateURL_ValidHTTPS verifies validation of valid HTTPS URLs
func TestValidateURL_ValidHTTPS(t *testing.T) {
	testCases := []struct {
		name       string
		url        string
		normalized string
	}{
		{"HTTPS with trailing slash", "https://example.com/", "https://example.com"},
		{"HTTPS without path", "https://example.com", "https://example.com"},
		{"HTTPS with port", "https://example.com:8443", "https://example.com:8443"},
		{"HTTPS with subdomain", "https://app.example.com", "https://app.example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalized, warning, err := ValidateURL(tc.url)

			assert.NoError(t, err)
			assert.Equal(t, tc.normalized, normalized)
			assert.Empty(t, warning, "HTTPS should not produce warning")
		})
	}
}

// TestValidateURL_ValidHTTP verifies validation of HTTP URLs with warning
func TestValidateURL_ValidHTTP(t *testing.T) {
	testCases := []struct {
		name       string
		url        string
		normalized string
	}{
		{"HTTP with trailing slash", "http://example.com/", "http://example.com"},
		{"HTTP without path", "http://example.com", "http://example.com"},
		{"HTTP with port", "http://example.com:8080", "http://example.com:8080"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalized, warning, err := ValidateURL(tc.url)

			assert.NoError(t, err)
			assert.Equal(t, tc.normalized, normalized)
			assert.NotEmpty(t, warning, "HTTP should produce security warning")
			assert.Contains(t, warning, "HTTP", "warning should mention HTTP")
			assert.Contains(t, warning, "HTTPS", "warning should suggest HTTPS")
		})
	}
}

// TestValidateURL_InvalidScheme verifies rejection of non-HTTP/HTTPS schemes
func TestValidateURL_InvalidScheme(t *testing.T) {
	testCases := []string{
		"ftp://example.com",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"ssh://user@host",
	}

	for _, url := range testCases {
		t.Run(url, func(t *testing.T) {
			_, _, err := ValidateURL(url)

			assert.Error(t, err, "non-HTTP(S) scheme should be rejected")
		})
	}
}

// TestValidateURL_WithPath verifies rejection of URLs with paths
func TestValidateURL_WithPath(t *testing.T) {
	testCases := []string{
		"https://example.com/api/v1",
		"https://example.com/admin",
		"http://example.com/path/to/resource",
		"https://example.com/index.html",
	}

	for _, url := range testCases {
		t.Run(url, func(t *testing.T) {
			_, _, err := ValidateURL(url)

			assert.Error(t, err, "URL with path should be rejected")
		})
	}
}

// TestValidateURL_RootPathAllowed verifies "/" path is allowed
func TestValidateURL_RootPathAllowed(t *testing.T) {
	testCases := []string{
		"https://example.com/",
		"http://example.com/",
	}

	for _, url := range testCases {
		t.Run(url, func(t *testing.T) {
			normalized, _, err := ValidateURL(url)

			assert.NoError(t, err, "root path '/' should be allowed")
			// Trailing slash should be removed
			assert.NotContains(t, normalized[len(normalized)-1:], "/", "normalized URL should not end with slash")
		})
	}
}

// TestValidateURL_MalformedURL verifies handling of malformed URLs
func TestValidateURL_MalformedURL(t *testing.T) {
	testCases := []struct {
		url        string
		shouldFail bool
	}{
		{"not a url", true},
		{"://missing-scheme", true},
		{"http://", false}, // Valid URL with empty host - Parse accepts it
		{"https://[invalid", true},
		{"", true},
	}

	for _, tc := range testCases {
		t.Run(tc.url, func(t *testing.T) {
			_, _, err := ValidateURL(tc.url)

			if tc.shouldFail {
				assert.Error(t, err, "malformed URL should be rejected")
			} else {
				// Some URLs that look malformed are actually valid per RFC
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateURL_SpecialCharacters verifies handling of special characters
func TestValidateURL_SpecialCharacters(t *testing.T) {
	testCases := []struct {
		name    string
		url     string
		isValid bool
	}{
		{"Punycode domain", "https://xn--e1afmkfd.xn--p1ai", true},
		{"Port with special chars", "https://example.com:8080", true},
		{"Query string (no path component)", "https://example.com?query=1", true}, // Query strings have empty Path
		{"Fragment (no path component)", "https://example.com#section", true},     // Fragments have empty Path
		{"Userinfo", "https://user:pass@example.com", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ValidateURL(tc.url)

			if tc.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// TestValidateURL_Normalization verifies URL normalization
func TestValidateURL_Normalization(t *testing.T) {
	testCases := []struct {
		input      string
		expected   string
		shouldFail bool
	}{
		{"https://EXAMPLE.COM", "https://EXAMPLE.COM", false},         // Case preserved
		{"https://example.com/", "https://example.com", false},        // Trailing slash removed
		{"https://example.com///", "", true},                          // Multiple slashes = path component, should fail
		{"http://example.com:80", "http://example.com:80", false},     // Port preserved
		{"https://example.com:443", "https://example.com:443", false}, // Default HTTPS port preserved
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			normalized, _, err := ValidateURL(tc.input)

			if tc.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, normalized)
			}
		})
	}
}

// TestGetBaseURL verifies base URL extraction from request
func TestGetBaseURL(t *testing.T) {
	testCases := []struct {
		name            string
		host            string
		hasTLS          bool
		xForwardedProto string
		expected        string
	}{
		{
			name:     "HTTPS with TLS",
			host:     "secure.example.com",
			hasTLS:   true,
			expected: "https://secure.example.com",
		},
		{
			name:     "HTTP without TLS",
			host:     "insecure.example.com",
			hasTLS:   false,
			expected: "http://insecure.example.com",
		},
		{
			name:            "X-Forwarded-Proto HTTPS",
			host:            "behind-proxy.com",
			hasTLS:          false,
			xForwardedProto: "https",
			expected:        "https://behind-proxy.com",
		},
		{
			name:            "X-Forwarded-Proto HTTP",
			host:            "behind-proxy.com",
			hasTLS:          false,
			xForwardedProto: "http",
			expected:        "http://behind-proxy.com",
		},
		{
			name:     "With port",
			host:     "example.com:8080",
			hasTLS:   false,
			expected: "http://example.com:8080",
		},
		{
			name:     "IPv4 host",
			host:     "192.168.1.1:8080",
			hasTLS:   false,
			expected: "http://192.168.1.1:8080",
		},
		{
			name:     "IPv6 host",
			host:     "[::1]:8080",
			hasTLS:   false,
			expected: "http://[::1]:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Build request URL
			scheme := "http"
			if tc.hasTLS {
				scheme = "https"
			}
			req := httptest.NewRequest(http.MethodGet, scheme+"://"+tc.host+"/test", http.NoBody)

			// Set TLS if needed
			if tc.hasTLS {
				req.TLS = &tls.ConnectionState{}
			}

			// Set X-Forwarded-Proto if specified
			if tc.xForwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.xForwardedProto)
			}

			c.Request = req

			baseURL := getBaseURL(c)

			assert.Equal(t, tc.expected, baseURL)
		})
	}
}

// TestGetBaseURL_PrecedenceOrder verifies header precedence
func TestGetBaseURL_PrecedenceOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Request with TLS but also X-Forwarded-Proto
	req := httptest.NewRequest(http.MethodGet, "https://example.com/test", http.NoBody)
	req.TLS = &tls.ConnectionState{}
	req.Header.Set("X-Forwarded-Proto", "http") // Should be ignored when TLS is present
	c.Request = req

	baseURL := getBaseURL(c)

	// TLS should take precedence over header
	assert.Equal(t, "https://example.com", baseURL)
}

// TestGetBaseURL_EmptyHost verifies behavior with empty host
func TestGetBaseURL_EmptyHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http:///test", http.NoBody)
	req.Host = "" // Empty host
	c.Request = req

	baseURL := getBaseURL(c)

	// Should still return valid URL with empty host
	assert.Equal(t, "http://", baseURL)
}

// ============================================
// GetConfiguredPublicURL Tests
// ============================================

func TestGetConfiguredPublicURL_ValidURL(t *testing.T) {
	db := setupTestDB(t)

	// Insert a valid configured public URL
	setting := models.Setting{
		Key:   "app.public_url",
		Value: "https://example.com",
	}
	err := db.Create(&setting).Error
	require.NoError(t, err)

	publicURL, ok := GetConfiguredPublicURL(db)

	assert.True(t, ok, "should return true for valid URL")
	assert.Equal(t, "https://example.com", publicURL)
}

func TestGetConfiguredPublicURL_WithTrailingSlash(t *testing.T) {
	db := setupTestDB(t)

	setting := models.Setting{
		Key:   "app.public_url",
		Value: "https://example.com/",
	}
	err := db.Create(&setting).Error
	require.NoError(t, err)

	publicURL, ok := GetConfiguredPublicURL(db)

	assert.True(t, ok)
	assert.Equal(t, "https://example.com", publicURL, "should remove trailing slash")
}

func TestGetConfiguredPublicURL_NoSetting(t *testing.T) {
	db := setupTestDB(t)
	// No setting created

	publicURL, ok := GetConfiguredPublicURL(db)

	assert.False(t, ok, "should return false when setting doesn't exist")
	assert.Equal(t, "", publicURL)
}

func TestGetConfiguredPublicURL_EmptyValue(t *testing.T) {
	db := setupTestDB(t)

	setting := models.Setting{
		Key:   "app.public_url",
		Value: "",
	}
	err := db.Create(&setting).Error
	require.NoError(t, err)

	publicURL, ok := GetConfiguredPublicURL(db)

	assert.False(t, ok, "should return false for empty value")
	assert.Equal(t, "", publicURL)
}

func TestGetConfiguredPublicURL_WithPort(t *testing.T) {
	db := setupTestDB(t)

	setting := models.Setting{
		Key:   "app.public_url",
		Value: "https://example.com:8443",
	}
	err := db.Create(&setting).Error
	require.NoError(t, err)

	publicURL, ok := GetConfiguredPublicURL(db)

	assert.True(t, ok)
	assert.Equal(t, "https://example.com:8443", publicURL)
}

func TestGetConfiguredPublicURL_InvalidURL(t *testing.T) {
	db := setupTestDB(t)

	testCases := []struct {
		name  string
		value string
	}{
		{"invalid scheme", "ftp://example.com"},
		{"with path", "https://example.com/admin"},
		{"with query", "https://example.com?query=1"},
		{"with fragment", "https://example.com#section"},
		{"with userinfo", "https://user:pass@example.com"},
		{"no host", "https://"},
		{"embedded newline", "https://exam\nple.com"}, // Newline in middle (not trimmed)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean DB for each sub-test
			db.Where("1 = 1").Delete(&models.Setting{})

			setting := models.Setting{
				Key:   "app.public_url",
				Value: tc.value,
			}
			err := db.Create(&setting).Error
			require.NoError(t, err)

			publicURL, ok := GetConfiguredPublicURL(db)

			assert.False(t, ok, "should return false for invalid URL: %s", tc.value)
			assert.Equal(t, "", publicURL)
		})
	}
}

// ============================================
// Additional GetConfiguredPublicURL Edge Cases
// ============================================

func TestGetConfiguredPublicURL_WithWhitespace(t *testing.T) {
	db := setupTestDB(t)

	setting := models.Setting{
		Key:   "app.public_url",
		Value: "  https://example.com  ",
	}
	err := db.Create(&setting).Error
	require.NoError(t, err)

	publicURL, ok := GetConfiguredPublicURL(db)

	assert.True(t, ok, "should trim whitespace")
	assert.Equal(t, "https://example.com", publicURL)
}

func TestGetConfiguredPublicURL_TrailingNewline(t *testing.T) {
	db := setupTestDB(t)

	// Trailing newlines are removed by TrimSpace before validation
	setting := models.Setting{
		Key:   "app.public_url",
		Value: "https://example.com\n",
	}
	err := db.Create(&setting).Error
	require.NoError(t, err)

	publicURL, ok := GetConfiguredPublicURL(db)

	assert.True(t, ok, "trailing newline should be trimmed")
	assert.Equal(t, "https://example.com", publicURL)
}
