package services

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// BuiltInNameservers maps nameserver patterns to provider types.
// Pattern matching is case-insensitive and uses substring matching.
var BuiltInNameservers = map[string]string{
	// Cloudflare
	"cloudflare.com": "cloudflare",

	// AWS Route 53
	"awsdns": "route53",

	// DigitalOcean
	"digitalocean.com": "digitalocean",

	// Google Cloud DNS
	"googledomains.com": "googleclouddns",
	"ns-cloud":          "googleclouddns",

	// Azure DNS
	"azure-dns": "azure",

	// Namecheap
	"registrar-servers.com": "namecheap",

	// GoDaddy
	"domaincontrol.com": "godaddy",

	// Hetzner
	"hetzner.com": "hetzner",
	"hetzner.de":  "hetzner",

	// Vultr
	"vultr.com": "vultr",

	// DNSimple
	"dnsimple.com": "dnsimple",
}

// DetectionResult represents the result of DNS provider auto-detection.
type DetectionResult struct {
	Domain            string              `json:"domain"`
	Detected          bool                `json:"detected"`
	ProviderType      string              `json:"provider_type,omitempty"`
	Nameservers       []string            `json:"nameservers"`
	Confidence        string              `json:"confidence"` // "high", "medium", "low", "none"
	SuggestedProvider *models.DNSProvider `json:"suggested_provider,omitempty"`
	Error             string              `json:"error,omitempty"`
}

// cacheEntry stores a cached detection result with expiration.
type cacheEntry struct {
	result    *DetectionResult
	expiresAt time.Time
}

// DNSDetectionService provides DNS provider auto-detection capabilities.
type DNSDetectionService interface {
	// DetectProvider identifies the DNS provider for a domain
	DetectProvider(domain string) (*DetectionResult, error)

	// SuggestConfiguredProvider finds a matching configured provider
	SuggestConfiguredProvider(ctx context.Context, domain string) (*models.DNSProvider, error)

	// GetNameserverPatterns returns current pattern database
	GetNameserverPatterns() map[string]string
}

// dnsDetectionService implements DNSDetectionService.
type dnsDetectionService struct {
	db          *gorm.DB
	cache       map[string]*cacheEntry
	cacheMutex  sync.RWMutex
	cacheTTL    time.Duration
	dnsResolver *net.Resolver
}

// NewDNSDetectionService creates a new DNS detection service.
func NewDNSDetectionService(db *gorm.DB) DNSDetectionService {
	return &dnsDetectionService{
		db:       db,
		cache:    make(map[string]*cacheEntry),
		cacheTTL: 1 * time.Hour,
		dnsResolver: &net.Resolver{
			PreferGo: true,
			Dial:     nil, // Use default
		},
	}
}

// DetectProvider identifies the DNS provider for a domain based on nameserver lookups.
func (s *dnsDetectionService) DetectProvider(domain string) (*DetectionResult, error) {
	// Normalize domain - remove wildcard prefix
	baseDomain := strings.TrimPrefix(domain, "*.")
	baseDomain = strings.TrimSpace(strings.ToLower(baseDomain))

	if baseDomain == "" {
		return &DetectionResult{
			Domain:      domain,
			Detected:    false,
			Nameservers: []string{},
			Confidence:  "none",
			Error:       "invalid domain",
		}, nil
	}

	// Check cache first
	if cached := s.getCachedResult(baseDomain); cached != nil {
		return cached, nil
	}

	// Lookup NS records with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nameservers, err := s.dnsResolver.LookupNS(ctx, baseDomain)
	if err != nil {
		result := &DetectionResult{
			Domain:      baseDomain,
			Detected:    false,
			Nameservers: []string{},
			Confidence:  "none",
			Error:       fmt.Sprintf("DNS lookup failed: %v", err),
		}
		// Cache error results for shorter duration
		s.cacheResult(baseDomain, result, 5*time.Minute)
		return result, nil
	}

	// Extract nameserver hosts
	nsHosts := make([]string, len(nameservers))
	for i, ns := range nameservers {
		nsHosts[i] = strings.ToLower(strings.TrimSuffix(ns.Host, "."))
	}

	// Match against patterns
	providerType, confidence := s.matchNameservers(nsHosts)

	result := &DetectionResult{
		Domain:       baseDomain,
		Detected:     providerType != "",
		ProviderType: providerType,
		Nameservers:  nsHosts,
		Confidence:   confidence,
	}

	// Cache successful results
	s.cacheResult(baseDomain, result, s.cacheTTL)

	return result, nil
}

// SuggestConfiguredProvider finds a matching configured provider based on detection.
func (s *dnsDetectionService) SuggestConfiguredProvider(ctx context.Context, domain string) (*models.DNSProvider, error) {
	// First detect the provider
	detection, err := s.DetectProvider(domain)
	if err != nil {
		return nil, err
	}

	// If not detected, return nil
	if !detection.Detected {
		return nil, nil
	}

	// Find enabled providers matching the detected type
	var providers []models.DNSProvider
	err = s.db.WithContext(ctx).
		Where("provider_type = ? AND enabled = ?", detection.ProviderType, true).
		Order("is_default DESC, name ASC").
		Find(&providers).Error

	if err != nil {
		return nil, err
	}

	// Return first match (prefer default)
	if len(providers) > 0 {
		return &providers[0], nil
	}

	return nil, nil
}

// GetNameserverPatterns returns the current nameserver pattern database.
func (s *dnsDetectionService) GetNameserverPatterns() map[string]string {
	// Return a copy to prevent external modification
	patterns := make(map[string]string, len(BuiltInNameservers))
	for k, v := range BuiltInNameservers {
		patterns[k] = v
	}
	return patterns
}

// matchNameservers matches nameserver hosts against known patterns.
// Returns the provider type and confidence level.
func (s *dnsDetectionService) matchNameservers(nameservers []string) (string, string) {
	if len(nameservers) == 0 {
		return "", "none"
	}

	// Track matches per provider type
	matchCounts := make(map[string]int)
	totalMatches := 0

	// Check each nameserver against all patterns
	for _, ns := range nameservers {
		nsLower := strings.ToLower(ns)
		for pattern, providerType := range BuiltInNameservers {
			patternLower := strings.ToLower(pattern)
			if strings.Contains(nsLower, patternLower) {
				matchCounts[providerType]++
				totalMatches++
				break // Count each nameserver only once per provider
			}
		}
	}

	// No matches found
	if totalMatches == 0 {
		return "", "none"
	}

	// Find provider with most matches
	var bestProvider string
	maxMatches := 0
	for provider, count := range matchCounts {
		if count > maxMatches {
			maxMatches = count
			bestProvider = provider
		}
	}

	// Calculate confidence based on match percentage
	matchPercentage := float64(maxMatches) / float64(len(nameservers))

	var confidence string
	switch {
	case matchPercentage >= 0.8: // 80%+ nameservers matched
		confidence = "high"
	case matchPercentage >= 0.5: // 50-79% matched
		confidence = "medium"
	case matchPercentage > 0: // 1-49% matched
		confidence = "low"
	default:
		confidence = "none"
	}

	return bestProvider, confidence
}

// getCachedResult retrieves a cached detection result if valid.
func (s *dnsDetectionService) getCachedResult(domain string) *DetectionResult {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	entry, exists := s.cache[domain]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		// Clean up expired entry (non-blocking)
		go func() {
			s.cacheMutex.Lock()
			delete(s.cache, domain)
			s.cacheMutex.Unlock()
		}()
		return nil
	}

	return entry.result
}

// cacheResult stores a detection result in cache.
func (s *dnsDetectionService) cacheResult(domain string, result *DetectionResult, ttl time.Duration) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.cache[domain] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(ttl),
	}
}
