package utils

import (
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// PublicURLSettingKey is the Setting row that holds the admin-configured public address.
const PublicURLSettingKey = "app.public_url"

// GetConfiguredPublicURL returns the configured, normalized public URL.
//
// Security note:
// This function intentionally never derives URLs from request data (Host/X-Forwarded-*),
// so it is safe to use for embedding external links (e.g., invite emails).
func GetConfiguredPublicURL(db *gorm.DB) (string, bool) {
	var setting models.Setting
	if err := db.Where("key = ?", PublicURLSettingKey).First(&setting).Error; err != nil {
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
	if err := db.Where("key = ?", PublicURLSettingKey).First(&setting).Error; err == nil {
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

	return canonicalOrigin(parsed), nil
}

// canonicalOrigin returns "scheme://host[:port]" in the form a browser sends as Origin: the host is lowercased and a
// port equal to the scheme's default (443 for https, 80 for http) is dropped. IPv6 hosts keep their brackets.
func canonicalOrigin(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == defaultPortForScheme(u.Scheme) {
		port = ""
	}

	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return u.Scheme + "://" + host
}

// defaultPortForScheme returns the port a browser omits for scheme, or "" when the scheme has none.
func defaultPortForScheme(scheme string) string {
	switch scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
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
