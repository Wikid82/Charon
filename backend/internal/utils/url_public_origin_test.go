package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestNormalizeConfiguredPublicURL_CanonicalOrigin proves the configured address is reduced to the origin a browser
// sends: lowercase host and no default port for the scheme.
func TestNormalizeConfiguredPublicURL_CanonicalOrigin(t *testing.T) {
	cases := map[string]string{
		"https://example.com":         "https://example.com",
		"https://Example.COM":         "https://example.com",
		"HTTPS://Example.com/":        "https://example.com",
		"https://example.com:443":     "https://example.com",
		"https://Example.com:443/":    "https://example.com",
		"http://example.com:80":       "http://example.com",
		"http://EXAMPLE.com:80/":      "http://example.com",
		"http://example.com:443":      "http://example.com:443",
		"https://example.com:80":      "https://example.com:80",
		"https://example.com:8443":    "https://example.com:8443",
		"http://localhost:8080":       "http://localhost:8080",
		"http://[::1]:80":             "http://[::1]",
		"https://[::1]:443":           "https://[::1]",
		"https://[2001:DB8::1]:8443":  "https://[2001:db8::1]:8443",
		"https://[2001:DB8::1]":       "https://[2001:db8::1]",
		"  https://example.com:443  ": "https://example.com",
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			got, err := normalizeConfiguredPublicURL(raw)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

// TestNormalizeConfiguredPublicURL_InvalidValuesAreRejected proves every rejected value returns an error, so the
// origin source trusts nothing for it.
func TestNormalizeConfiguredPublicURL_InvalidValuesAreRejected(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"ftp://example.com",
		"javascript:alert(1)",
		"example.com",
		"https://",
		"https://user:pass@example.com",
		"https://example.com/app",
		"https://example.com?x=1",
		"https://example.com#frag",
		"https://example.com:abc",
		"https://exa mple.com",
		"https://example.com\r\nX-Injected: 1",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			got, err := normalizeConfiguredPublicURL(raw)
			assert.Error(t, err)
			assert.Empty(t, got)
		})
	}
}

// TestGetConfiguredPublicURL_InvalidStoredValueTrustsNothing proves a stored value that fails normalisation is reported
// as not configured, so no origin is derived from it.
func TestGetConfiguredPublicURL_InvalidStoredValueTrustsNothing(t *testing.T) {
	cases := []string{"ftp://example.com", "https://example.com/app", "https://user@example.com", "https://example.com:abc"}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			db := setupTestDB(t)
			require.NoError(t, db.Create(&models.Setting{Key: PublicURLSettingKey, Value: raw}).Error)

			got, ok := GetConfiguredPublicURL(db)
			assert.False(t, ok)
			assert.Empty(t, got)
		})
	}
}

// TestGetConfiguredPublicURL_CanonicalStoredOrigin proves the stored value is returned in canonical origin form.
func TestGetConfiguredPublicURL_CanonicalStoredOrigin(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: PublicURLSettingKey, Value: "https://Charon.Example.com:443/"}).Error)

	got, ok := GetConfiguredPublicURL(db)
	require.True(t, ok)
	assert.Equal(t, "https://charon.example.com", got)
}
