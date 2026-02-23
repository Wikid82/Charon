# Phase 3 Multi-Credential Integration - Quick Reference

**Full Plan:** [phase3_caddy_integration_completion.md](./phase3_caddy_integration_completion.md)

## 3-Step Implementation

### 1. Add Fields to DNSProviderConfig (manager.go:38-44)

```go
type DNSProviderConfig struct {
    ID                 uint
    ProviderType       string
    PropagationTimeout int
    Credentials        map[string]string              // Single-cred mode
    UseMultiCredentials bool                          // NEW
    ZoneCredentials     map[string]map[string]string  // NEW: map[baseDomain]credentials
}
```

### 2. Add Credential Resolution Loop (manager.go:~125)

Insert after line 125 (after `dnsProviderConfigs` built):

```go
// Phase 2: Resolve zone-specific credentials for multi-credential providers
for i := range dnsProviderConfigs {
    cfg := &dnsProviderConfigs[i]

    // Find provider and check UseMultiCredentials flag
    var provider *models.DNSProvider
    for j := range dnsProviders {
        if dnsProviders[j].ID == cfg.ID {
            provider = &dnsProviders[j]
            break
        }
    }

    if provider == nil || !provider.UseMultiCredentials {
        continue  // Skip single-credential providers
    }

    // Enable multi-credential mode
    cfg.UseMultiCredentials = true
    cfg.ZoneCredentials = make(map[string]map[string]string)

    // Preload credentials
    if err := m.db.Preload("Credentials").First(provider, provider.ID).Error; err != nil {
        logger.Log().WithError(err).WithField("provider_id", provider.ID).Warn("failed to preload credentials")
        continue
    }

    // Resolve credentials for each host's domain
    for _, host := range hosts {
        if !host.Enabled || host.DNSProviderID == nil || *host.DNSProviderID != provider.ID {
            continue
        }

        baseDomain := extractBaseDomain(host.DomainNames)
        if baseDomain == "" || cfg.ZoneCredentials[baseDomain] != nil {
            continue  // Already resolved
        }

        // Resolve credential for this domain
        credentials, err := m.getCredentialForDomain(provider.ID, baseDomain, provider)
        if err != nil {
            logger.Log().WithError(err).WithField("domain", baseDomain).Warn("credential resolution failed")
            continue
        }

        cfg.ZoneCredentials[baseDomain] = credentials
        logger.Log().WithField("domain", baseDomain).Debug("resolved credential")
    }

    logger.Log().WithField("domains_resolved", len(cfg.ZoneCredentials)).Info("multi-credential resolution complete")
}
```

### 3. Update Config Generation (config.go:~190-198)

Replace credential assembly logic in DNS challenge policy creation:

```go
// Find DNS provider config
dnsConfig, ok := dnsProviderMap[providerID]
if !ok {
    logger.Log().WithField("provider_id", providerID).Warn("DNS provider not found")
    continue
}

// MULTI-CREDENTIAL MODE: Create separate policy per domain
if dnsConfig.UseMultiCredentials && len(dnsConfig.ZoneCredentials) > 0 {
    for baseDomain, credentials := range dnsConfig.ZoneCredentials {
        // Find domains matching this base domain
        var matchingDomains []string
        for _, domain := range domains {
            if extractBaseDomain(domain) == baseDomain {
                matchingDomains = append(matchingDomains, domain)
            }
        }
        if len(matchingDomains) == 0 {
            continue
        }

        // Build provider config with zone-specific credentials
        providerConfig := map[string]any{"name": dnsConfig.ProviderType}
        for key, value := range credentials {
            providerConfig[key] = value
        }

        // Build issuer with DNS challenge (same as original, but with zone-specific credentials)
        var issuers []any
        // ... (same issuer creation logic as original, using providerConfig)

        // Create TLS automation policy for this domain
        tlsPolicies = append(tlsPolicies, &AutomationPolicy{
            Subjects:   dedupeDomains(matchingDomains),
            IssuersRaw: issuers,
        })

        logger.Log().WithField("base_domain", baseDomain).Debug("created DNS challenge policy")
    }
    continue  // Skip single-credential logic below
}

// SINGLE-CREDENTIAL MODE: Original logic (backward compatible)
providerConfig := map[string]any{"name": dnsConfig.ProviderType}
for key, value := range dnsConfig.Credentials {
    providerConfig[key] = value
}
// ... (rest of original logic)
```

## Testing Checklist

- [ ] Run `go test ./internal/caddy -run TestExtractBaseDomain` (should pass)
- [ ] Run `go test ./internal/caddy -run TestMatchesZoneFilter` (should pass)
- [ ] Run `go test ./internal/caddy -run TestManager_GetCredentialForDomain` (should pass)
- [ ] Run `go test ./internal/caddy/...` (all tests should pass after changes)
- [ ] Create integration test for multi-credential provider
- [ ] Manual test: Create provider with 2+ credentials, verify separate TLS policies

## Validation Commands

```bash
# Test helpers
go test -v ./internal/caddy -run TestExtractBaseDomain
go test -v ./internal/caddy -run TestMatchesZoneFilter

# Test integration
go test -v ./internal/caddy/... -count=1

# Check logs for credential resolution
docker logs charon-app 2>&1 | grep "multi-credential"
docker logs charon-app 2>&1 | grep "resolved credential"

# Verify generated Caddy config
curl -s http://localhost:2019/config/ | jq '.apps.tls.automation.policies[] | select(.subjects[] | contains("example"))'
```

## Success Criteria

✅ All existing tests pass
✅ Helper function tests pass
✅ Integration tests pass (once added)
✅ Manual testing with 2+ domains works
✅ Backward compatibility validated
✅ Logs show credential selection

## Rollback

If issues occur:

1. Set `UseMultiCredentials=false` on all providers via API
2. Restart Charon
3. Investigate logs for credential resolution errors

## Files Modified

- `backend/internal/caddy/manager.go` - Add fields, add resolution loop
- `backend/internal/caddy/config.go` - Update DNS challenge policy generation
- `backend/internal/caddy/manager_multicred_integration_test.go` - Add integration tests (new file)

## Estimated Time

- Implementation: 2-3 hours
- Testing: 1-2 hours
- Documentation: 1 hour
- **Total: 4-6 hours**
