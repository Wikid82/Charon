package utils

import (
	"errors"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// GetConfiguredPublicURL returns the configured, normalized public URL.
//
// Security note:
// This function intentionally never derives URLs from request data (Host/X-Forwarded-*),
// so it is safe to use for embedding external links (e.g., invite emails).
func GetConfiguredPublicURL(db *gorm.DB) (string, bool) {
	var setting models.Setting
	if err := db.Where("key = ?", "app.public_url").First(&setting).Error; err != nil {
		return "", false
	}

	normalized, err := normalizeConfiguredPublicURL(setting.Value)
	if err != nil {
		return "", false
	}

	return normalized, true
}

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

func normalizeConfiguredPublicURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("public URL is empty")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", errors.New("public URL contains invalid characters")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("public URL must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("public URL must include a host")
	}
	if parsed.User != nil {
		return "", errors.New("public URL must not include userinfo")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("public URL must not include query or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("public URL must not include a path")
	}
	if parsed.Opaque != "" {
		return "", errors.New("public URL must not be opaque")
	}

	normalized := (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()
	return normalized, nil
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
