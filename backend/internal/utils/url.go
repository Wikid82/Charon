package utils

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// GetPublicURL retrieves the configured public URL or falls back to request host.
// This should be used for all user-facing URLs (emails, invite links).
func GetPublicURL(db *gorm.DB, c *gin.Context) string {
	var setting models.Setting
	if err := db.Where("key = ?", "app.public_url").First(&setting).Error; err == nil {
		if setting.Value != "" {
			return strings.TrimSuffix(setting.Value, "/")
		}
	}
	// Fallback to request-derived URL
	return getBaseURL(c)
}

// getBaseURL extracts the base URL from the request.
func getBaseURL(c *gin.Context) string {
	scheme := "https"
	if c.Request.TLS == nil {
		// Check for X-Forwarded-Proto header
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + c.Request.Host
}

// ValidateURL validates that a URL is properly formatted for use as an application URL.
// Returns error message if invalid, empty string if valid.
func ValidateURL(rawURL string) (normalized, warning string, err error) {
	// Parse URL
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return "", "", parseErr
	}

	// Validate scheme
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", &url.Error{
			Op:  "parse",
			URL: rawURL,
			Err: nil,
		}
	}

	// Warn if HTTP
	if parsed.Scheme == "http" {
		warning = "Using HTTP is not recommended. Consider using HTTPS for security."
	}

	// Reject URLs with path components beyond "/"
	if parsed.Path != "" && parsed.Path != "/" {
		return "", "", &url.Error{
			Op:  "validate",
			URL: rawURL,
			Err: nil,
		}
	}

	// Normalize URL (remove trailing slash, keep scheme and host)
	normalized = strings.TrimSuffix(rawURL, "/")

	return normalized, warning, nil
}
