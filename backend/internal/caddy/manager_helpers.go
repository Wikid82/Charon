package caddy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// extractBaseDomain extracts the base domain from a domain name.
// Handles wildcard domains (*.example.com -> example.com)
func extractBaseDomain(domainNames string) string {
	if domainNames == "" {
		return ""
	}

	// Split by comma and take first domain
	domains := strings.Split(domainNames, ",")
	if len(domains) == 0 {
		return ""
	}

	domain := strings.TrimSpace(domains[0])
	// Strip wildcard prefix if present
	domain = strings.TrimPrefix(domain, "*.")

	return strings.ToLower(domain)
}

// matchesZoneFilter checks if a domain matches a zone filter pattern.
// exactOnly=true means only check for exact matches, false allows wildcards.
func matchesZoneFilter(zoneFilter, domain string, exactOnly bool) bool {
	if strings.TrimSpace(zoneFilter) == "" {
		return false // Empty filter is catch-all, handled separately
	}

	// Parse comma-separated zones
	zones := strings.Split(zoneFilter, ",")
	for _, zone := range zones {
		zone = strings.ToLower(strings.TrimSpace(zone))
		if zone == "" {
			continue
		}

		// Exact match
		if zone == domain {
			return true
		}

		// Wildcard match (only if not exact-only)
		if !exactOnly && strings.HasPrefix(zone, "*.") {
			suffix := zone[2:] // Remove "*."
			if strings.HasSuffix(domain, "."+suffix) || domain == suffix {
				return true
			}
		}
	}

	return false
}

// getCredentialForDomain resolves the appropriate credential for a domain.
// For multi-credential providers, it selects zone-specific credentials.
// For single-credential providers, it returns the default credentials.
func (m *Manager) getCredentialForDomain(providerID uint, domain string, provider *models.DNSProvider) (map[string]string, error) {
	// If not using multi-credentials, use provider's main credentials
	if !provider.UseMultiCredentials {
		var decryptedData []byte
		var err error

		// Try to get encryption key from environment
		encryptionKey := ""
		for _, key := range []string{"CHARON_ENCRYPTION_KEY", "ENCRYPTION_KEY", "CERBERUS_ENCRYPTION_KEY"} {
			if val := os.Getenv(key); val != "" {
				encryptionKey = val
				break
			}
		}

		if encryptionKey == "" {
			return nil, fmt.Errorf("no encryption key available")
		}

		// Create encryptor inline
		encryptor, err := crypto.NewEncryptionService(encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create encryptor: %w", err)
		}

		decryptedData, err = encryptor.Decrypt(provider.CredentialsEncrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
		}

		var credentials map[string]string
		if err := json.Unmarshal(decryptedData, &credentials); err != nil {
			return nil, fmt.Errorf("failed to parse credentials: %w", err)
		}

		return credentials, nil
	}

	// Multi-credential mode: find the best matching credential
	var bestMatch *models.DNSProviderCredential
	normalizedDomain := strings.ToLower(strings.TrimSpace(domain))

	// Priority 1: Exact match
	for i := range provider.Credentials {
		if !provider.Credentials[i].Enabled {
			continue
		}
		if matchesZoneFilter(provider.Credentials[i].ZoneFilter, normalizedDomain, true) {
			bestMatch = &provider.Credentials[i]
			break
		}
	}

	// Priority 2: Wildcard match
	if bestMatch == nil {
		for i := range provider.Credentials {
			if !provider.Credentials[i].Enabled {
				continue
			}
			if matchesZoneFilter(provider.Credentials[i].ZoneFilter, normalizedDomain, false) {
				bestMatch = &provider.Credentials[i]
				break
			}
		}
	}

	// Priority 3: Catch-all (empty zone_filter)
	if bestMatch == nil {
		for i := range provider.Credentials {
			if !provider.Credentials[i].Enabled {
				continue
			}
			if strings.TrimSpace(provider.Credentials[i].ZoneFilter) == "" {
				bestMatch = &provider.Credentials[i]
				break
			}
		}
	}

	if bestMatch == nil {
		return nil, fmt.Errorf("no matching credential found for domain %s", domain)
	}

	// Decrypt the matched credential
	encryptionKey := ""
	for _, key := range []string{"CHARON_ENCRYPTION_KEY", "ENCRYPTION_KEY", "CERBERUS_ENCRYPTION_KEY"} {
		if val := os.Getenv(key); val != "" {
			encryptionKey = val
			break
		}
	}

	if encryptionKey == "" {
		return nil, fmt.Errorf("no encryption key available")
	}

	encryptor, err := crypto.NewEncryptionService(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	decryptedData, err := encryptor.Decrypt(bestMatch.CredentialsEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credential %s: %w", bestMatch.UUID, err)
	}

	var credentials map[string]string
	if err := json.Unmarshal(decryptedData, &credentials); err != nil {
		return nil, fmt.Errorf("failed to parse credential %s: %w", bestMatch.UUID, err)
	}

	// Log credential selection for audit trail
	logger.Log().WithFields(map[string]any{
		"provider_id":      providerID,
		"domain":           domain,
		"credential_uuid":  bestMatch.UUID,
		"credential_label": bestMatch.Label,
		"zone_filter":      bestMatch.ZoneFilter,
	}).Info("selected credential for domain")

	return credentials, nil
}
