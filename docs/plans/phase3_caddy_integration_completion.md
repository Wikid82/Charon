# Phase 3: Caddy Manager Multi-Credential Integration - Completion Plan

**Status:** 95% Complete - Final Integration Required
**Created:** 2026-01-04
**Target Completion:** Sprint 11

## Executive Summary

The multi-credential infrastructure is complete (models, services, API, helpers, tests). The remaining 5% is integrating the credential resolution logic into the Caddy Manager's config generation flow.

## Completion Checklist

- [x] DNSProviderCredential model created
- [x] CredentialService with zone matching
- [x] API handlers (7 endpoints)
- [x] Helper functions (extractBaseDomain, matchesZoneFilter, getCredentialForDomain)
- [x] Helper function tests
- [ ] **ApplyConfig credential resolution loop** ← THIS STEP
- [ ] **buildDNSChallengeIssuer integration** ← THIS STEP
- [ ] Integration tests
- [ ] Backward compatibility validation

---

## Part 1: Understanding Current Flow

### Current Architecture (Single Credential)

**File:** `backend/internal/caddy/manager.go`
**Method:** `ApplyConfig()` (Lines 80-140)

```go
// Current flow:
1. Load proxy hosts from DB
2. Load DNS providers from DB
3. Decrypt DNS provider credentials (single set per provider)
4. Build dnsProviderConfigs []DNSProviderConfig
5. Pass to GenerateConfig()
```

**File:** `backend/internal/caddy/config.go`
**Method:** `GenerateConfig()` (Lines 18-130)
**Submethods:** DNS policy generation (Lines 131-220)

```go
// Current flow:
1. Group hosts by DNS provider
2. For each provider: Build DNS challenge issuer with provider.Credentials
3. Create TLS automation policy with DNS challenge
```

### New Architecture (Multi-Credential)

```
ApplyConfig()
  ↓
  For each proxy host with DNS challenge:
    ↓
    getCredentialForDomain(providerID, baseDomain, provider)
      ↓
      Returns zone-specific credentials (or provider default)
  ↓
  Store credentials in map[baseDomain]map[string]string
  ↓
  Pass map to GenerateConfig()
  ↓
  buildDNSChallengeIssuer() uses per-domain credentials
```

---

## Part 2: Code Changes Required

### Change 1: Add Fields to DNSProviderConfig

**File:** `backend/internal/caddy/manager.go`
**Location:** Lines 38-44 (DNSProviderConfig struct)

**Before:**

```go
// DNSProviderConfig contains a DNS provider with its decrypted credentials
// for use in Caddy DNS challenge configuration generation
type DNSProviderConfig struct {
 ID                 uint
 ProviderType       string
 PropagationTimeout int
 Credentials        map[string]string
}
```

**After:**

```go
// DNSProviderConfig contains a DNS provider with its decrypted credentials
// for use in Caddy DNS challenge configuration generation
type DNSProviderConfig struct {
 ID                 uint
 ProviderType       string
 PropagationTimeout int

 // Single-credential mode: Use these credentials for all domains
 Credentials        map[string]string

 // Multi-credential mode: Use zone-specific credentials
 UseMultiCredentials bool
 ZoneCredentials     map[string]map[string]string // map[baseDomain]credentials
}
```

**Why:**

- Backwards compatible: Existing Credentials field still works for single-cred mode
- New ZoneCredentials field stores per-domain credentials
- UseMultiCredentials flag determines which field to use

---

### Change 2: Credential Resolution in ApplyConfig

**File:** `backend/internal/caddy/manager.go`
**Method:** `ApplyConfig()`
**Location:** Lines 80-140 (between provider decryption and GenerateConfig call)

**Context (Lines 93-125):**

```go
 // Decrypt DNS provider credentials for config generation
 // We need an encryption service to decrypt the credentials
 var dnsProviderConfigs []DNSProviderConfig
 if len(dnsProviders) > 0 {
  // Try to get encryption key from environment
  encryptionKey := os.Getenv("CHARON_ENCRYPTION_KEY")
  if encryptionKey == "" {
   // Try alternative env vars
   for _, key := range []string{"ENCRYPTION_KEY", "CERBERUS_ENCRYPTION_KEY"} {
    if val := os.Getenv(key); val != "" {
     encryptionKey = val
     break
    }
   }
  }

  if encryptionKey != "" {
   // Import crypto package for inline decryption
   encryptor, err := crypto.NewEncryptionService(encryptionKey)
   if err != nil {
    logger.Log().WithError(err).Warn("failed to initialize encryption service for DNS provider credentials")
   } else {
    // Decrypt each DNS provider's credentials
    for _, provider := range dnsProviders {
     if provider.CredentialsEncrypted == "" {
      continue
     }

     decryptedData, err := encryptor.Decrypt(provider.CredentialsEncrypted)
     if err != nil {
      logger.Log().WithError(err).WithField("provider_id", provider.ID).Warn("failed to decrypt DNS provider credentials")
      continue
     }

     var credentials map[string]string
     if err := json.Unmarshal(decryptedData, &credentials); err != nil {
      logger.Log().WithError(err).WithField("provider_id", provider.ID).Warn("failed to parse DNS provider credentials")
      continue
     }

     dnsProviderConfigs = append(dnsProviderConfigs, DNSProviderConfig{
      ID:                 provider.ID,
      ProviderType:       provider.ProviderType,
      PropagationTimeout: provider.PropagationTimeout,
      Credentials:        credentials,
     })
    }
   }
  } else {
   logger.Log().Warn("CHARON_ENCRYPTION_KEY not set, DNS challenge configuration will be skipped")
  }
 }
```

**Insert After Line 125 (after dnsProviderConfigs built, before acmeEmail fetch):**

```go
 // Phase 2: Resolve zone-specific credentials for multi-credential providers
 // For each provider with UseMultiCredentials=true, build a map of domain->credentials
 // by iterating through all proxy hosts that use DNS challenge
 for i := range dnsProviderConfigs {
  cfg := &dnsProviderConfigs[i]

  // Find the provider in the dnsProviders slice to check UseMultiCredentials
  var provider *models.DNSProvider
  for j := range dnsProviders {
   if dnsProviders[j].ID == cfg.ID {
    provider = &dnsProviders[j]
    break
   }
  }

  // Skip if not multi-credential mode or provider not found
  if provider == nil || !provider.UseMultiCredentials {
   continue
  }

  // Enable multi-credential mode for this provider config
  cfg.UseMultiCredentials = true
  cfg.ZoneCredentials = make(map[string]map[string]string)

  // Preload credentials for this provider (eager loading for better logging)
  if err := m.db.Preload("Credentials").First(provider, provider.ID).Error; err != nil {
   logger.Log().WithError(err).WithField("provider_id", provider.ID).Warn("failed to preload credentials for provider")
   continue
  }

  // Iterate through proxy hosts to find domains that use this provider
  for _, host := range hosts {
   if !host.Enabled || host.DNSProviderID == nil || *host.DNSProviderID != provider.ID {
    continue
   }

   // Extract base domain from host's domain names
   baseDomain := extractBaseDomain(host.DomainNames)
   if baseDomain == "" {
    continue
   }

   // Skip if we already resolved credentials for this domain
   if _, exists := cfg.ZoneCredentials[baseDomain]; exists {
    continue
   }

   // Resolve the appropriate credential for this domain
   credentials, err := m.getCredentialForDomain(provider.ID, baseDomain, provider)
   if err != nil {
    logger.Log().
     WithError(err).
     WithField("provider_id", provider.ID).
     WithField("domain", baseDomain).
     Warn("failed to resolve credential for domain, DNS challenge will be skipped for this domain")
    continue
   }

   // Store resolved credentials for this domain
   cfg.ZoneCredentials[baseDomain] = credentials

   logger.Log().WithFields(map[string]any{
    "provider_id":   provider.ID,
    "provider_type": provider.ProviderType,
    "domain":        baseDomain,
   }).Debug("resolved credential for domain")
  }

  // Log summary of credential resolution for audit trail
  logger.Log().WithFields(map[string]any{
   "provider_id":      provider.ID,
   "provider_type":    provider.ProviderType,
   "domains_resolved": len(cfg.ZoneCredentials),
  }).Info("multi-credential DNS provider resolution complete")
 }
```

**Why This Works:**

1. **Non-invasive:** Only adds logic for providers with UseMultiCredentials=true
2. **Backward compatible:** Single-cred providers skip this entire block
3. **Efficient:** Pre-resolves credentials once, before config generation
4. **Auditable:** Logs credential selection for security compliance
5. **Error-resilient:** Failed credential resolution logs warning, doesn't block entire config

---

### Change 3: Use Resolved Credentials in Config Generation

**File:** `backend/internal/caddy/config.go`
**Method:** `GenerateConfig()`
**Location:** Lines 131-220 (DNS challenge policy generation)

**Context (Lines 131-140):**

```go
 // Group hosts by DNS provider for TLS automation policies
 // We need separate policies for:
 // 1. Wildcard domains with DNS challenge (per DNS provider)
 // 2. Regular domains with HTTP challenge (default policy)
 var tlsPolicies []*AutomationPolicy

 // Build a map of DNS provider ID to DNS provider config for quick lookup
 dnsProviderMap := make(map[uint]DNSProviderConfig)
 for _, cfg := range dnsProviderConfigs {
  dnsProviderMap[cfg.ID] = cfg
 }
```

**Find the section that builds DNS challenge issuer (Lines 180-230):**

```go
  // Create DNS challenge policies for each DNS provider
  for providerID, domains := range dnsProviderDomains {
   // Find the DNS provider config
   dnsConfig, ok := dnsProviderMap[providerID]
   if !ok {
    logger.Log().WithField("provider_id", providerID).Warn("DNS provider not found in decrypted configs")
    continue
   }

   // Build provider config for Caddy with decrypted credentials
   providerConfig := map[string]any{
    "name": dnsConfig.ProviderType,
   }

   // Add all credential fields to the provider config
   for key, value := range dnsConfig.Credentials {
    providerConfig[key] = value
   }
```

**Replace Lines 190-198 (credential assembly) with multi-credential logic:**

```go
  // Create DNS challenge policies for each DNS provider
  for providerID, domains := range dnsProviderDomains {
   // Find the DNS provider config
   dnsConfig, ok := dnsProviderMap[providerID]
   if !ok {
    logger.Log().WithField("provider_id", providerID).Warn("DNS provider not found in decrypted configs")
    continue
   }

   // **CHANGED: Multi-credential support**
   // If provider uses multi-credentials, create separate policies per domain
   if dnsConfig.UseMultiCredentials && len(dnsConfig.ZoneCredentials) > 0 {
    // Create a separate TLS automation policy for each domain with its own credentials
    for baseDomain, credentials := range dnsConfig.ZoneCredentials {
     // Find all domains that match this base domain
     var matchingDomains []string
     for _, domain := range domains {
      if extractBaseDomain(domain) == baseDomain {
       matchingDomains = append(matchingDomains, domain)
      }
     }

     if len(matchingDomains) == 0 {
      continue // No domains for this credential
     }

     // Build provider config with zone-specific credentials
     providerConfig := map[string]any{
      "name": dnsConfig.ProviderType,
     }
     for key, value := range credentials {
      providerConfig[key] = value
     }

     // Build issuer config with these credentials
     var issuers []any
     switch sslProvider {
     case "letsencrypt":
      acmeIssuer := map[string]any{
       "module": "acme",
       "email":  acmeEmail,
       "challenges": map[string]any{
        "dns": map[string]any{
         "provider":            providerConfig,
         "propagation_timeout": int64(dnsConfig.PropagationTimeout) * 1_000_000_000,
        },
       },
      }
      if acmeStaging {
       acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
      }
      issuers = append(issuers, acmeIssuer)
     case "zerossl":
      issuers = append(issuers, map[string]any{
       "module": "zerossl",
       "challenges": map[string]any{
        "dns": map[string]any{
         "provider":            providerConfig,
         "propagation_timeout": int64(dnsConfig.PropagationTimeout) * 1_000_000_000,
        },
       },
      })
     default: // "both" or empty
      acmeIssuer := map[string]any{
       "module": "acme",
       "email":  acmeEmail,
       "challenges": map[string]any{
        "dns": map[string]any{
         "provider":            providerConfig,
         "propagation_timeout": int64(dnsConfig.PropagationTimeout) * 1_000_000_000,
        },
       },
      }
      if acmeStaging {
       acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
      }
      issuers = append(issuers, acmeIssuer)
      issuers = append(issuers, map[string]any{
       "module": "zerossl",
       "challenges": map[string]any{
        "dns": map[string]any{
         "provider":            providerConfig,
         "propagation_timeout": int64(dnsConfig.PropagationTimeout) * 1_000_000_000,
        },
       },
      })
     }

     // Create TLS automation policy for this domain with zone-specific credentials
     tlsPolicies = append(tlsPolicies, &AutomationPolicy{
      Subjects:   dedupeDomains(matchingDomains),
      IssuersRaw: issuers,
     })

     logger.Log().WithFields(map[string]any{
      "provider_id":    providerID,
      "base_domain":    baseDomain,
      "domain_count":   len(matchingDomains),
      "credential_used": true,
     }).Debug("created DNS challenge policy with zone-specific credential")
    }

    // Skip the original single-credential logic below
    continue
   }

   // **ORIGINAL: Single-credential mode (backward compatible)**
   // Build provider config for Caddy with decrypted credentials
   providerConfig := map[string]any{
    "name": dnsConfig.ProviderType,
   }

   // Add all credential fields to the provider config
   for key, value := range dnsConfig.Credentials {
    providerConfig[key] = value
   }

   // [KEEP EXISTING CODE FROM HERE - Lines 201-235 for single-credential issuer creation]
```

**Why This Works:**

1. **Conditional branching:** Checks `UseMultiCredentials` flag
2. **Per-domain policies:** Creates separate TLS automation policies per domain
3. **Credential isolation:** Each domain gets its own credential set
4. **Backward compatible:** Falls back to original logic for single-cred mode
5. **Auditable:** Logs which credential is used for each domain

---

## Part 3: Testing Strategy

### Test 1: Backward Compatibility (Single Credential)

**File:** `backend/internal/caddy/manager_test.go`

```go
func TestApplyConfig_SingleCredential_BackwardCompatibility(t *testing.T) {
 // Setup: Create provider with UseMultiCredentials=false
 provider := models.DNSProvider{
  ProviderType:        "cloudflare",
  UseMultiCredentials: false,
  CredentialsEncrypted: encryptJSON(t, map[string]string{
   "api_token": "test-token",
  }),
 }

 // Setup: Create proxy host with wildcard domain
 host := models.ProxyHost{
  DomainNames:    "*.example.com",
  DNSProviderID:  &provider.ID,
  ForwardHost:    "localhost",
  ForwardPort:    8080,
  Enabled:        true,
 }

 // Act: Apply config
 err := manager.ApplyConfig(ctx)

 // Assert: No errors
 require.NoError(t, err)

 // Assert: Generated config uses provider credentials
 config, err := manager.GetCurrentConfig(ctx)
 require.NoError(t, err)

 // Assert: TLS policy has DNS challenge with correct credentials
 assertDNSChallengePolicy(t, config, "example.com", "cloudflare", "test-token")
}
```

### Test 2: Multi-Credential Zone Matching

**File:** `backend/internal/caddy/manager_multicred_integration_test.go` (new file)

```go
func TestApplyConfig_MultiCredential_ZoneMatching(t *testing.T) {
 // Setup: Create provider with UseMultiCredentials=true
 provider := models.DNSProvider{
  ProviderType:        "cloudflare",
  UseMultiCredentials: true,
  Credentials: []models.DNSProviderCredential{
   {
    Label:      "Example.com Credential",
    ZoneFilter: "example.com",
    CredentialsEncrypted: encryptJSON(t, map[string]string{
     "api_token": "token-example-com",
    }),
    Enabled: true,
   },
   {
    Label:      "Example.org Credential",
    ZoneFilter: "example.org",
    CredentialsEncrypted: encryptJSON(t, map[string]string{
     "api_token": "token-example-org",
    }),
    Enabled: true,
   },
  },
 }

 // Setup: Create proxy hosts for different domains
 hosts := []models.ProxyHost{
  {
   DomainNames:    "*.example.com",
   DNSProviderID:  &provider.ID,
   ForwardHost:    "localhost",
   ForwardPort:    8080,
   Enabled:        true,
  },
  {
   DomainNames:    "*.example.org",
   DNSProviderID:  &provider.ID,
   ForwardHost:    "localhost",
   ForwardPort:    8081,
   Enabled:        true,
  },
 }

 // Act: Apply config
 err := manager.ApplyConfig(ctx)
 require.NoError(t, err)

 // Assert: Generated config has separate policies with correct credentials
 config, err := manager.GetCurrentConfig(ctx)
 require.NoError(t, err)

 assertDNSChallengePolicy(t, config, "example.com", "cloudflare", "token-example-com")
 assertDNSChallengePolicy(t, config, "example.org", "cloudflare", "token-example-org")
}
```

### Test 3: Wildcard and Catch-All Matching

**File:** `backend/internal/caddy/manager_multicred_integration_test.go`

```go
func TestApplyConfig_MultiCredential_WildcardAndCatchAll(t *testing.T) {
 // Setup: Provider with wildcard and catch-all credentials
 provider := models.DNSProvider{
  ProviderType:        "cloudflare",
  UseMultiCredentials: true,
  Credentials: []models.DNSProviderCredential{
   {
    Label:      "Example.com Specific",
    ZoneFilter: "example.com",
    CredentialsEncrypted: encryptJSON(t, map[string]string{"api_token": "specific"}),
    Enabled: true,
   },
   {
    Label:      "Example.org Wildcard",
    ZoneFilter: "*.example.org",
    CredentialsEncrypted: encryptJSON(t, map[string]string{"api_token": "wildcard"}),
    Enabled: true,
   },
   {
    Label:      "Catch-All",
    ZoneFilter: "",
    CredentialsEncrypted: encryptJSON(t, map[string]string{"api_token": "catch-all"}),
    Enabled: true,
   },
  },
 }

 // Test exact match beats catch-all
 assertCredentialSelection(t, manager, provider.ID, "example.com", "specific")

 // Test wildcard match beats catch-all
 assertCredentialSelection(t, manager, provider.ID, "app.example.org", "wildcard")

 // Test catch-all for unmatched domain
 assertCredentialSelection(t, manager, provider.ID, "random.net", "catch-all")
}
```

### Test 4: Error Handling

**File:** `backend/internal/caddy/manager_multicred_integration_test.go`

```go
func TestApplyConfig_MultiCredential_ErrorHandling(t *testing.T) {
 tests := []struct {
  name          string
  setup         func(*models.DNSProvider)
  expectError   bool
  expectWarning string
 }{
  {
   name: "no matching credential",
   setup: func(p *models.DNSProvider) {
    p.Credentials = []models.DNSProviderCredential{
     {
      ZoneFilter: "example.com",
      Enabled:    true,
     },
    }
   },
   expectWarning: "failed to resolve credential for domain",
  },
  {
   name: "all credentials disabled",
   setup: func(p *models.DNSProvider) {
    p.Credentials = []models.DNSProviderCredential{
     {
      ZoneFilter: "example.com",
      Enabled:    false,
     },
    }
   },
   expectWarning: "no matching credential found",
  },
  {
   name: "decryption failure",
   setup: func(p *models.DNSProvider) {
    p.Credentials = []models.DNSProviderCredential{
     {
      ZoneFilter:           "example.com",
      CredentialsEncrypted: "invalid-encrypted-data",
      Enabled:              true,
     },
    }
   },
   expectWarning: "failed to decrypt credential",
  },
 }

 for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
   // Setup and run test
   // Assert warning is logged
  })
 }
}
```

---

## Part 4: Integration Sequence

To avoid breaking intermediate states, apply changes in this order:

### Step 1: Add Struct Fields

- Modify `DNSProviderConfig` struct in `manager.go`
- Add `UseMultiCredentials` and `ZoneCredentials` fields
- **Validation:** Run `go test ./internal/caddy -run TestApplyConfig` - should still pass

### Step 2: Add Credential Resolution Loop

- Insert credential resolution code in `ApplyConfig()` after provider decryption
- **Validation:** Run `go test ./internal/caddy -run TestApplyConfig` - should still pass
- **Validation:** Check logs for "multi-credential DNS provider resolution complete"

### Step 3: Update Config Generation

- Modify `GenerateConfig()` to check `UseMultiCredentials` flag
- Add per-domain policy creation logic
- Keep fallback to original logic
- **Validation:** Run `go test ./internal/caddy/...` - all tests should pass

### Step 4: Add Integration Tests

- Create `manager_multicred_integration_test.go`
- Add 4 test scenarios above
- **Validation:** All new tests pass

### Step 5: Manual Validation

- Start Charon with multi-credential provider
- Create proxy hosts for different domains
- Apply config and check generated Caddy config JSON
- Verify separate TLS automation policies per domain

---

## Part 5: Backward Compatibility Checklist

- [ ] Single-credential providers (UseMultiCredentials=false) work unchanged
- [ ] Existing proxy hosts with DNS challenge still get certificates
- [ ] No breaking changes to DNSProviderConfig API (only additions)
- [ ] Existing tests still pass without modification
- [ ] New fields are optional (zero values = backward compatible behavior)
- [ ] Error handling is non-fatal (warnings logged, doesn't block config)

---

## Part 6: Performance Considerations

### Optimization 1: Lazy Loading vs Eager Loading

**Decision:** Use eager loading in credential resolution loop
**Rationale:**

- Small dataset (typically <10 credentials per provider)
- Better logging and debugging
- Simpler error handling
- Minimal performance impact

### Optimization 2: Credential Caching

**Decision:** Pre-resolve credentials once in ApplyConfig, cache in ZoneCredentials map
**Rationale:**

- Avoids repeated DB queries during config generation
- Credentials don't change during config generation
- Simpler code flow

### Optimization 3: Domain Deduplication

**Decision:** Skip already-resolved domains in credential resolution loop
**Rationale:**

- Multiple proxy hosts may use same base domain
- Avoid redundant credential resolution
- Slight performance gain

---

## Part 7: Security Considerations

### Audit Logging

- Log credential selection for each domain (provider_id, domain, credential_uuid)
- Log credential resolution summary (provider_id, domains_resolved)
- Log credential selection in debug mode for troubleshooting

### Error Handling

- Failed credential resolution logs warning, doesn't block entire config
- Decryption failures are non-fatal for individual credentials
- No credentials in error messages (use UUIDs only)

### Credential Isolation

- Each domain gets its own credential set in Caddy config
- No credential leakage between domains
- Caddy enforces per-policy credential usage

---

## Part 8: Rollback Plan

If issues arise after deployment:

1. **Immediate:** Set `UseMultiCredentials=false` on all providers via API
2. **Short-term:** Revert to previous Charon version
3. **Investigation:** Check logs for credential resolution warnings
4. **Fix:** Address specific credential matching or decryption issues

---

## Part 9: Success Criteria

- [ ] All existing tests pass
- [ ] 4 new integration tests pass
- [ ] Manual testing with 2+ domains per provider works
- [ ] Backward compatibility validated with single-credential provider
- [ ] No performance regression (config generation <2s for 100 hosts)
- [ ] Audit logs show credential selection for all domains
- [ ] Documentation updated (API docs, admin guide)

---

## Part 10: Documentation Updates Required

1. **API Documentation:** Add multi-credential endpoints to OpenAPI spec
2. **Admin Guide:** Add section on multi-credential configuration
3. **Migration Guide:** Document single→multi credential migration
4. **Troubleshooting Guide:** Add credential resolution debugging section
5. **Changelog:** Document multi-credential support in v0.3.0 release notes

---

## Appendix A: Helper Function Reference

Already implemented in `backend/internal/caddy/manager_helpers.go`:

### extractBaseDomain(domainNames string) string

- Extracts base domain from comma-separated list
- Strips wildcard prefix (*.example.com → example.com)
- Returns lowercase domain

### matchesZoneFilter(zoneFilter, domain string, exactOnly bool) bool

- Checks if domain matches zone filter pattern
- Supports exact match and wildcard match
- Returns false for empty filter (handled separately as catch-all)

### (m *Manager) getCredentialForDomain(providerID uint, domain string, provider*models.DNSProvider) (map[string]string, error)

- Resolves appropriate credential for domain
- Priority: exact match → wildcard match → catch-all
- Returns decrypted credentials map
- Logs credential selection for audit trail

---

## Appendix B: Testing Helpers

Create these in `manager_multicred_integration_test.go`:

```go
func encryptJSON(t *testing.T, data map[string]string) string {
 // Encrypt JSON for test fixtures
}

func assertDNSChallengePolicy(t *testing.T, config *Config, domain, provider, token string) {
 // Assert TLS automation policy exists with correct credentials
}

func assertCredentialSelection(t *testing.T, manager *Manager, providerID uint, domain, expectedToken string) {
 // Assert getCredentialForDomain returns expected credential
}
```

---

## Appendix C: Error Scenarios

| Scenario | Behavior | User Impact |
|----------|----------|-------------|
| No matching credential | Log warning, skip domain | Certificate not issued for that domain |
| Decryption failure | Log warning, skip credential | Fallback to catch-all or skip domain |
| Empty ZoneCredentials | Fall back to single-cred mode | Backward compatible behavior |
| Disabled credential | Skip credential | Next priority credential used |
| No encryption key | Skip DNS challenge | HTTP challenge used (if applicable) |

---

## End of Plan

**Next Action:** Implement changes in sequence (Steps 1-5)
**Review Required:** Code review after Step 3 (before integration tests)
**Deployment:** Sprint 11 release (after all success criteria met)
