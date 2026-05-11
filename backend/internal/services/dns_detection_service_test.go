package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates a test database for DNS detection tests
func setupTestDetectionDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate models
	err = db.AutoMigrate(&models.DNSProvider{})
	require.NoError(t, err)

	return db
}

// seedTestProviders seeds the database with test DNS providers
func seedTestProviders(t *testing.T, db *gorm.DB) {
	providers := []models.DNSProvider{
		{
			UUID:               "test-cloudflare-uuid",
			Name:               "Production Cloudflare",
			ProviderType:       "cloudflare",
			Enabled:            true,
			IsDefault:          true,
			PropagationTimeout: 120,
			PollingInterval:    5,
			KeyVersion:         1,
		},
		{
			UUID:               "test-route53-uuid",
			Name:               "AWS Route53",
			ProviderType:       "route53",
			Enabled:            true,
			IsDefault:          false,
			PropagationTimeout: 120,
			PollingInterval:    5,
			KeyVersion:         1,
		},
	}

	for _, p := range providers {
		require.NoError(t, db.Create(&p).Error)
	}

	// Create disabled provider separately and explicitly disable it
	disabledProvider := models.DNSProvider{
		UUID:               "test-digitalocean-uuid",
		Name:               "DigitalOcean DNS",
		ProviderType:       "digitalocean",
		Enabled:            true, // Create as enabled first
		IsDefault:          false,
		PropagationTimeout: 120,
		PollingInterval:    5,
		KeyVersion:         1,
	}
	require.NoError(t, db.Create(&disabledProvider).Error)
	// Now explicitly disable it with an update
	require.NoError(t, db.Model(&disabledProvider).Update("enabled", false).Error)
}

func TestNewDNSDetectionService(t *testing.T) {
	db := setupTestDetectionDB(t)

	service := NewDNSDetectionService(db)
	assert.NotNil(t, service)

	// Verify it implements the interface by using it
	// NewDNSDetectionService returns DNSDetectionService, so type is guaranteed
	patterns := service.GetNameserverPatterns()
	assert.NotNil(t, patterns)
}

func TestGetNameserverPatterns(t *testing.T) {
	db := setupTestDetectionDB(t)
	service := NewDNSDetectionService(db)

	patterns := service.GetNameserverPatterns()

	// Verify we get expected providers
	assert.NotEmpty(t, patterns)
	assert.Contains(t, patterns, "cloudflare.com")
	assert.Equal(t, "cloudflare", patterns["cloudflare.com"])
	assert.Contains(t, patterns, "awsdns")
	assert.Equal(t, "route53", patterns["awsdns"])
	assert.Contains(t, patterns, "digitalocean.com")
	assert.Equal(t, "digitalocean", patterns["digitalocean.com"])

	// Verify at least 10 providers
	assert.GreaterOrEqual(t, len(patterns), 10)
}

func TestMatchNameservers(t *testing.T) {
	db := setupTestDetectionDB(t)
	svc := NewDNSDetectionService(db).(*dnsDetectionService)

	tests := []struct {
		name         string
		nameservers  []string
		expectedType string
		expectedConf string
	}{
		{
			name: "Cloudflare - high confidence",
			nameservers: []string{
				"ns1.cloudflare.com",
				"ns2.cloudflare.com",
			},
			expectedType: "cloudflare",
			expectedConf: "high",
		},
		{
			name: "Route53 - high confidence",
			nameservers: []string{
				"ns-123.awsdns-45.com",
				"ns-456.awsdns-78.net",
			},
			expectedType: "route53",
			expectedConf: "high",
		},
		{
			name: "DigitalOcean - high confidence",
			nameservers: []string{
				"ns1.digitalocean.com",
				"ns2.digitalocean.com",
				"ns3.digitalocean.com",
			},
			expectedType: "digitalocean",
			expectedConf: "high",
		},
		{
			name: "Hetzner - high confidence",
			nameservers: []string{
				"hydrogen.ns.hetzner.com",
				"oxygen.ns.hetzner.com",
				"helium.ns.hetzner.de",
			},
			expectedType: "hetzner",
			expectedConf: "high",
		},
		{
			name: "Mixed nameservers - medium confidence",
			nameservers: []string{
				"ns1.cloudflare.com",
				"ns1.unknown-provider.com",
			},
			expectedType: "cloudflare",
			expectedConf: "medium",
		},
		{
			name: "Single match - low confidence",
			nameservers: []string{
				"ns1.cloudflare.com",
				"ns1.unknown1.com",
				"ns2.unknown2.com",
			},
			expectedType: "cloudflare",
			expectedConf: "low",
		},
		{
			name: "No match",
			nameservers: []string{
				"ns1.custom-provider.com",
				"ns2.custom-provider.com",
			},
			expectedType: "",
			expectedConf: "none",
		},
		{
			name:         "Empty nameservers",
			nameservers:  []string{},
			expectedType: "",
			expectedConf: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerType, confidence := svc.matchNameservers(tt.nameservers)
			assert.Equal(t, tt.expectedType, providerType, "Provider type mismatch")
			assert.Equal(t, tt.expectedConf, confidence, "Confidence mismatch")
		})
	}
}

func TestDetectProvider_WithMockedDNS(t *testing.T) {
	db := setupTestDetectionDB(t)
	service := NewDNSDetectionService(db).(*dnsDetectionService)

	// Override resolver with mock that returns controlled results
	// In a real scenario, we'd use a mock resolver, but for this test
	// we'll test the logic without actual DNS lookups

	t.Run("handles wildcard domain", func(t *testing.T) {
		// We can't easily test actual DNS lookups in unit tests
		// But we can verify wildcard prefix removal
		domain := "*.example.com"
		result, err := service.DetectProvider(domain)
		require.NoError(t, err)
		assert.Equal(t, "example.com", result.Domain, "Wildcard prefix should be removed")
	})

	t.Run("handles empty domain", func(t *testing.T) {
		result, err := service.DetectProvider("")
		require.NoError(t, err)
		assert.False(t, result.Detected)
		assert.Equal(t, "none", result.Confidence)
		assert.Equal(t, "invalid domain", result.Error)
	})

	t.Run("normalizes domain", func(t *testing.T) {
		result, err := service.DetectProvider("  EXAMPLE.COM  ")
		require.NoError(t, err)
		assert.Equal(t, "example.com", result.Domain, "Domain should be normalized")
	})
}

func TestCaching(t *testing.T) {
	db := setupTestDetectionDB(t)
	service := NewDNSDetectionService(db).(*dnsDetectionService)

	// Manually cache a result
	testDomain := "test-cache.example.com"
	cachedResult := &DetectionResult{
		Domain:       testDomain,
		Detected:     true,
		ProviderType: "cloudflare",
		Nameservers:  []string{"ns1.cloudflare.com"},
		Confidence:   "high",
	}

	service.cacheResult(testDomain, cachedResult, 1*time.Hour)

	t.Run("retrieves cached result", func(t *testing.T) {
		result := service.getCachedResult(testDomain)
		require.NotNil(t, result)
		assert.Equal(t, testDomain, result.Domain)
		assert.True(t, result.Detected)
		assert.Equal(t, "cloudflare", result.ProviderType)
	})

	t.Run("returns nil for non-existent cache", func(t *testing.T) {
		result := service.getCachedResult("non-existent-domain.com")
		assert.Nil(t, result)
	})

	t.Run("expires old cache entries", func(t *testing.T) {
		expiredDomain := "expired.example.com"
		expiredResult := &DetectionResult{
			Domain:   expiredDomain,
			Detected: false,
		}

		// Cache with very short TTL
		service.cacheResult(expiredDomain, expiredResult, 1*time.Millisecond)

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		result := service.getCachedResult(expiredDomain)
		assert.Nil(t, result, "Expired cache entry should return nil")
	})
}

func TestSuggestConfiguredProvider(t *testing.T) {
	db := setupTestDetectionDB(t)
	seedTestProviders(t, db)

	service := NewDNSDetectionService(db).(*dnsDetectionService)

	// Mock a detection result by caching it
	cloudflareResult := &DetectionResult{
		Domain:       "cloudflare-example.com",
		Detected:     true,
		ProviderType: "cloudflare",
		Nameservers:  []string{"ns1.cloudflare.com"},
		Confidence:   "high",
	}
	service.cacheResult("cloudflare-example.com", cloudflareResult, 1*time.Hour)

	route53Result := &DetectionResult{
		Domain:       "route53-example.com",
		Detected:     true,
		ProviderType: "route53",
		Nameservers:  []string{"ns-123.awsdns-45.com"},
		Confidence:   "high",
	}
	service.cacheResult("route53-example.com", route53Result, 1*time.Hour)

	disabledResult := &DetectionResult{
		Domain:       "digitalocean-example.com",
		Detected:     true,
		ProviderType: "digitalocean",
		Nameservers:  []string{"ns1.digitalocean.com"},
		Confidence:   "high",
	}
	service.cacheResult("digitalocean-example.com", disabledResult, 1*time.Hour)

	unknownResult := &DetectionResult{
		Domain:       "unknown-example.com",
		Detected:     true,
		ProviderType: "unknown-provider",
		Nameservers:  []string{"ns1.unknown.com"},
		Confidence:   "high",
	}
	service.cacheResult("unknown-example.com", unknownResult, 1*time.Hour)

	ctx := context.Background()

	t.Run("suggests default cloudflare provider", func(t *testing.T) {
		provider, err := service.SuggestConfiguredProvider(ctx, "cloudflare-example.com")
		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "cloudflare", provider.ProviderType)
		assert.Equal(t, "Production Cloudflare", provider.Name)
		assert.True(t, provider.IsDefault)
	})

	t.Run("suggests route53 provider", func(t *testing.T) {
		provider, err := service.SuggestConfiguredProvider(ctx, "route53-example.com")
		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "route53", provider.ProviderType)
		assert.Equal(t, "AWS Route53", provider.Name)
	})

	t.Run("returns nil for disabled provider", func(t *testing.T) {
		provider, err := service.SuggestConfiguredProvider(ctx, "digitalocean-example.com")
		require.NoError(t, err)
		assert.Nil(t, provider, "Should not suggest disabled provider")
	})

	t.Run("returns nil for unknown provider", func(t *testing.T) {
		provider, err := service.SuggestConfiguredProvider(ctx, "unknown-example.com")
		require.NoError(t, err)
		assert.Nil(t, provider, "Should not suggest non-existent provider type")
	})
}

func TestDetectionResult_Validation(t *testing.T) {
	t.Run("result with all fields", func(t *testing.T) {
		result := &DetectionResult{
			Domain:       "example.com",
			Detected:     true,
			ProviderType: "cloudflare",
			Nameservers:  []string{"ns1.cloudflare.com", "ns2.cloudflare.com"},
			Confidence:   "high",
		}

		assert.Equal(t, "example.com", result.Domain)
		assert.True(t, result.Detected)
		assert.Equal(t, "cloudflare", result.ProviderType)
		assert.Len(t, result.Nameservers, 2)
		assert.Equal(t, "high", result.Confidence)
		assert.Empty(t, result.Error)
	})

	t.Run("result with error", func(t *testing.T) {
		result := &DetectionResult{
			Detected:   false,
			Confidence: "none",
			Error:      "DNS lookup failed: no such host",
		}

		assert.False(t, result.Detected)
		assert.Equal(t, "none", result.Confidence)
		assert.NotEmpty(t, result.Error)
	})
}

func TestBuiltInNameserversCompleteness(t *testing.T) {
	// Verify that built-in nameservers cover expected providers
	expectedProviders := []string{
		"cloudflare",
		"route53",
		"digitalocean",
		"googleclouddns",
		"azure",
		"namecheap",
		"godaddy",
		"hetzner",
		"vultr",
		"dnsimple",
	}

	foundProviders := make(map[string]bool)
	for _, providerType := range BuiltInNameservers {
		foundProviders[providerType] = true
	}

	for _, expected := range expectedProviders {
		assert.True(t, foundProviders[expected], "Missing provider: %s", expected)
	}

	// Verify at least 10 unique providers
	assert.GreaterOrEqual(t, len(foundProviders), 10, "Should have at least 10 providers")
}

func TestCaseInsensitiveMatching(t *testing.T) {
	db := setupTestDetectionDB(t)
	svc := NewDNSDetectionService(db).(*dnsDetectionService)

	tests := []struct {
		name        string
		nameservers []string
		expected    string
	}{
		{
			name:        "lowercase",
			nameservers: []string{"ns1.cloudflare.com"},
			expected:    "cloudflare",
		},
		{
			name:        "uppercase",
			nameservers: []string{"NS1.CLOUDFLARE.COM"},
			expected:    "cloudflare",
		},
		{
			name:        "mixed case",
			nameservers: []string{"Ns1.CloudFlare.Com"},
			expected:    "cloudflare",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerType, _ := svc.matchNameservers(tt.nameservers)
			assert.Equal(t, tt.expected, providerType)
		})
	}
}

func TestConcurrentCacheAccess(t *testing.T) {
	db := setupTestDetectionDB(t)
	service := NewDNSDetectionService(db).(*dnsDetectionService)

	// Test concurrent cache writes and reads
	done := make(chan bool)
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			domain := strings.ReplaceAll("test-DOMAIN-ID.com", "ID", string(rune(id))) //nolint:gosec // G115: test ID values fit safely in rune range
			result := &DetectionResult{
				Domain:   domain,
				Detected: true,
			}

			// Write
			service.cacheResult(domain, result, 1*time.Hour)

			// Read
			cached := service.getCachedResult(domain)
			assert.NotNil(t, cached)

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}
}

// TestDatabaseError verifies error handling when database is unavailable
func TestDatabaseError(t *testing.T) {
	// Create a database and immediately close it
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	service := NewDNSDetectionService(db)

	// Cache a successful detection
	svc := service.(*dnsDetectionService)
	testResult := &DetectionResult{
		Domain:       "test.com",
		Detected:     true,
		ProviderType: "cloudflare",
		Nameservers:  []string{"ns1.cloudflare.com"},
		Confidence:   "high",
	}
	svc.cacheResult("test.com", testResult, 1*time.Hour)

	// This should fail because database is closed
	provider, err := service.SuggestConfiguredProvider(context.Background(), "test.com")

	// Should get error due to closed database
	assert.Error(t, err)
	assert.Nil(t, provider)
	// Check for database closed error (exact message varies by SQLite driver)
	assert.Contains(t, err.Error(), "database is closed")
}
