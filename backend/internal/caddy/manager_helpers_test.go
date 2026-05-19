package caddy

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestExtractBaseDomain_EmptyInput verifies empty input returns empty string
func TestExtractBaseDomain_EmptyInput(t *testing.T) {
	result := extractBaseDomain("")
	require.Equal(t, "", result)
}

// TestExtractBaseDomain_OnlyCommas verifies handling of comma-only input
func TestExtractBaseDomain_OnlyCommas(t *testing.T) {
	// When input is only commas, first element is empty string after split
	result := extractBaseDomain(",,,")
	require.Equal(t, "", result)
}

// TestExtractBaseDomain_SingleDomain verifies single domain extraction
func TestExtractBaseDomain_SingleDomain(t *testing.T) {
	result := extractBaseDomain("example.com")
	require.Equal(t, "example.com", result)
}

// TestExtractBaseDomain_WildcardDomain verifies wildcard stripping
func TestExtractBaseDomain_WildcardDomain(t *testing.T) {
	result := extractBaseDomain("*.example.com")
	require.Equal(t, "example.com", result)
}

// TestExtractBaseDomain_MultipleDomains verifies first domain is used
func TestExtractBaseDomain_MultipleDomains(t *testing.T) {
	result := extractBaseDomain("first.com, second.com, third.com")
	require.Equal(t, "first.com", result)
}

// TestExtractBaseDomain_MultipleDomainsWithWildcard verifies wildcard stripping with multiple domains
func TestExtractBaseDomain_MultipleDomainsWithWildcard(t *testing.T) {
	result := extractBaseDomain("*.example.com, sub.example.com")
	require.Equal(t, "example.com", result)
}

// TestExtractBaseDomain_WithWhitespace verifies whitespace trimming
func TestExtractBaseDomain_WithWhitespace(t *testing.T) {
	result := extractBaseDomain("  example.com  ")
	require.Equal(t, "example.com", result)
}

// TestExtractBaseDomain_CaseNormalization verifies lowercase normalization
func TestExtractBaseDomain_CaseNormalization(t *testing.T) {
	result := extractBaseDomain("EXAMPLE.COM")
	require.Equal(t, "example.com", result)
}

// TestExtractBaseDomain_Subdomain verifies subdomain handling
func TestExtractBaseDomain_Subdomain(t *testing.T) {
	// Note: extractBaseDomain returns the first domain as-is (after wildcard removal)
	// It does NOT extract the registrable domain (like public suffix)
	result := extractBaseDomain("sub.example.com")
	require.Equal(t, "sub.example.com", result)
}

// TestExtractBaseDomain_MultiLevelSubdomain verifies multi-level subdomain handling
func TestExtractBaseDomain_MultiLevelSubdomain(t *testing.T) {
	result := extractBaseDomain("deep.sub.example.com")
	require.Equal(t, "deep.sub.example.com", result)
}

// TestMatchesZoneFilter_EmptyFilter verifies empty filter returns false (catch-all handled separately)
func TestMatchesZoneFilter_EmptyFilter(t *testing.T) {
	result := matchesZoneFilter("", "example.com", false)
	require.False(t, result)

	result = matchesZoneFilter("   ", "example.com", false)
	require.False(t, result)
}

// TestMatchesZoneFilter_EmptyZonesInList verifies empty zones in list are skipped
func TestMatchesZoneFilter_EmptyZonesInList(t *testing.T) {
	// Empty zone entries should be skipped
	result := matchesZoneFilter(",example.com,", "example.com", false)
	require.True(t, result)

	result = matchesZoneFilter(",,,", "example.com", false)
	require.False(t, result)
}

// TestMatchesZoneFilter_ExactMatch verifies exact domain matching
func TestMatchesZoneFilter_ExactMatch(t *testing.T) {
	result := matchesZoneFilter("example.com", "example.com", false)
	require.True(t, result)

	result = matchesZoneFilter("example.com", "other.com", false)
	require.False(t, result)
}

// TestMatchesZoneFilter_ExactMatchOnly verifies exact-only mode
func TestMatchesZoneFilter_ExactMatchOnly(t *testing.T) {
	// With exactOnly=true, wildcard patterns should not match
	result := matchesZoneFilter("*.example.com", "sub.example.com", true)
	require.False(t, result)

	// But exact matches should still work
	result = matchesZoneFilter("sub.example.com", "sub.example.com", true)
	require.True(t, result)
}

// TestMatchesZoneFilter_WildcardMatch verifies wildcard pattern matching
func TestMatchesZoneFilter_WildcardMatch(t *testing.T) {
	// Subdomain should match wildcard
	result := matchesZoneFilter("*.example.com", "sub.example.com", false)
	require.True(t, result)

	// Base domain should match wildcard
	result = matchesZoneFilter("*.example.com", "example.com", false)
	require.True(t, result)

	// Different domain should not match
	result = matchesZoneFilter("*.example.com", "other.com", false)
	require.False(t, result)
}

// TestMatchesZoneFilter_MultipleZones verifies comma-separated zone matching
func TestMatchesZoneFilter_MultipleZones(t *testing.T) {
	result := matchesZoneFilter("example.com, other.com", "other.com", false)
	require.True(t, result)

	result = matchesZoneFilter("example.com, other.com", "third.com", false)
	require.False(t, result)
}

// TestMatchesZoneFilter_MultipleZonesWithWildcard verifies mixed zone list
func TestMatchesZoneFilter_MultipleZonesWithWildcard(t *testing.T) {
	result := matchesZoneFilter("example.com, *.other.com", "sub.other.com", false)
	require.True(t, result)

	result2 := matchesZoneFilter("example.com, *.other.com", "example.com", false)
	require.True(t, result2)
}

// TestMatchesZoneFilter_WhitespaceTrimming verifies whitespace handling (different from manager_multicred_test)
func TestMatchesZoneFilter_WhitespaceTrimming_Detailed(t *testing.T) {
	result := matchesZoneFilter("  example.com  ,  other.com  ", "example.com", false)
	require.True(t, result)
}

// TestMatchesZoneFilter_DeepSubdomain verifies deep subdomain matching
func TestMatchesZoneFilter_DeepSubdomain(t *testing.T) {
	result := matchesZoneFilter("*.example.com", "deep.sub.example.com", false)
	require.True(t, result)
}

// TestGetCredentialForDomain_NoEncryptionKey verifies error when no encryption key
func TestGetCredentialForDomain_NoEncryptionKey(t *testing.T) {
	// Save original env vars
	origKeys := map[string]string{}
	for _, key := range []string{"CHARON_ENCRYPTION_KEY", "ENCRYPTION_KEY", "CERBERUS_ENCRYPTION_KEY"} {
		origKeys[key] = os.Getenv(key)
		_ = os.Unsetenv(key)
	}
	defer func() {
		for key, val := range origKeys {
			if val != "" {
				_ = os.Setenv(key, val)
			}
		}
	}()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{})

	provider := &models.DNSProvider{
		UseMultiCredentials: false,
	}

	_, err = manager.getCredentialForDomain(1, "example.com", provider)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no encryption key available")
}

// TestGetCredentialForDomain_MultiCredential_NoMatch verifies error when no credential matches
func TestGetCredentialForDomain_MultiCredential_NoMatch(t *testing.T) {
	// Save original env vars
	origKey := os.Getenv("CHARON_ENCRYPTION_KEY")
	_ = os.Setenv("CHARON_ENCRYPTION_KEY", "test-key-32-characters-long!!!!!")
	defer func() {
		if origKey != "" {
			_ = os.Setenv("CHARON_ENCRYPTION_KEY", origKey)
		} else {
			_ = os.Unsetenv("CHARON_ENCRYPTION_KEY")
		}
	}()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{})

	provider := &models.DNSProvider{
		UseMultiCredentials: true,
		Credentials: []models.DNSProviderCredential{
			{
				UUID:       "cred-1",
				Label:      "Zone1",
				ZoneFilter: "zone1.com",
				Enabled:    true,
			},
			{
				UUID:       "cred-2",
				Label:      "Zone2",
				ZoneFilter: "zone2.com",
				Enabled:    true,
			},
		},
	}

	_, err = manager.getCredentialForDomain(1, "unmatched.com", provider)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no matching credential found")
}

// TestGetCredentialForDomain_MultiCredential_DisabledSkipped verifies disabled credentials are skipped
func TestGetCredentialForDomain_MultiCredential_DisabledSkipped(t *testing.T) {
	// Save original env vars
	origKey := os.Getenv("CHARON_ENCRYPTION_KEY")
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", "test-key-32-characters-long!!!!!"))
	defer func() {
		if origKey != "" {
			require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", origKey))
		} else {
			require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY"))
		}
	}()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{})

	provider := &models.DNSProvider{
		UseMultiCredentials: true,
		Credentials: []models.DNSProviderCredential{
			{
				UUID:       "cred-1",
				Label:      "Disabled Zone",
				ZoneFilter: "example.com",
				Enabled:    false, // Disabled - should be skipped
			},
		},
	}

	// Should fail because the only matching credential is disabled
	_, err = manager.getCredentialForDomain(1, "example.com", provider)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no matching credential found")
}

// TestGetCredentialForDomain_MultiCredential_CatchAllMatch verifies empty zone_filter as catch-all
func TestGetCredentialForDomain_MultiCredential_CatchAllMatch(t *testing.T) {
	// Save original env vars
	origKey := os.Getenv("CHARON_ENCRYPTION_KEY")
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", "test-key-32-characters-long!!!!!"))
	defer func() {
		if origKey != "" {
			require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", origKey))
		} else {
			require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY"))
		}
	}()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{})

	provider := &models.DNSProvider{
		UseMultiCredentials: true,
		Credentials: []models.DNSProviderCredential{
			{ //nolint:gosec // test fixture with intentionally invalid data
				UUID:                 "cred-catch-all",
				Label:                "Catch-All",
				ZoneFilter:           "", // Empty = catch-all
				Enabled:              true,
				CredentialsEncrypted: "invalid-encrypted-data", // Will fail decryption
			},
		},
	}

	// Should match catch-all but fail on decryption or encryption key processing
	_, err = manager.getCredentialForDomain(1, "any-domain.com", provider)
	require.Error(t, err)
	// The error could be from encryptor creation or decryption
	require.True(t, strings.Contains(err.Error(), "failed to decrypt") || strings.Contains(err.Error(), "failed to create encryptor"),
		"expected encryption/decryption error, got: %s", err.Error())
}

// TestComputeEffectiveFlags_DB_SecurityConfigWAFDisabled verifies WAF disabled in SecurityConfig
func TestComputeEffectiveFlags_DB_SecurityConfigWAFDisabled(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	secCfg := config.SecurityConfig{CerberusEnabled: true, WAFMode: "enabled"}
	manager := NewManager(nil, db, "", "", false, secCfg)

	// Set WAF mode to disabled in DB
	res := db.Create(&models.SecurityConfig{Name: "default", Enabled: true, WAFMode: "disabled"})
	require.NoError(t, res.Error)

	_, _, waf, _, _ := manager.computeEffectiveFlags(context.Background())
	require.False(t, waf)
}

// TestComputeEffectiveFlags_DB_RateLimitFromBooleanField verifies backward compat with boolean field
func TestComputeEffectiveFlags_DB_RateLimitFromBooleanField(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	secCfg := config.SecurityConfig{CerberusEnabled: true, RateLimitMode: ""}
	manager := NewManager(nil, db, "", "", false, secCfg)

	// Set rate limit via boolean field (backward compatibility)
	res := db.Create(&models.SecurityConfig{Name: "default", Enabled: true, RateLimitEnable: true, RateLimitMode: ""})
	require.NoError(t, res.Error)

	_, _, _, rl, _ := manager.computeEffectiveFlags(context.Background())
	require.True(t, rl)
}

// TestComputeEffectiveFlags_DB_CrowdSecModeFromSecurityConfig verifies CrowdSec mode from DB
func TestComputeEffectiveFlags_DB_CrowdSecModeFromSecurityConfig(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	secCfg := config.SecurityConfig{CerberusEnabled: true, CrowdSecMode: ""}
	manager := NewManager(nil, db, "", "", false, secCfg)

	// Set CrowdSec mode in SecurityConfig table
	res := db.Create(&models.SecurityConfig{Name: "default", Enabled: true, CrowdSecMode: "local"})
	require.NoError(t, res.Error)

	_, _, _, _, cs := manager.computeEffectiveFlags(context.Background())
	require.True(t, cs)
}

// TestComputeEffectiveFlags_DB_LegacyCerberusKey verifies legacy security.cerberus.enabled key
func TestComputeEffectiveFlags_DB_LegacyCerberusKey(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	secCfg := config.SecurityConfig{CerberusEnabled: false} // Start with false
	manager := NewManager(nil, db, "", "", false, secCfg)

	// Set via legacy key
	res := db.Create(&models.Setting{Key: "security.cerberus.enabled", Value: "true"})
	require.NoError(t, res.Error)

	cerb, _, _, _, _ := manager.computeEffectiveFlags(context.Background())
	require.True(t, cerb)
}
