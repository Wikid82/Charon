package utils

import (
	"testing"

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
