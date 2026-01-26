# Fix GenerateConfig Test Compilation Errors

## Issue

7 test cases in `backend/internal/caddy/config_extra_test.go` are calling `GenerateConfig` without enough arguments. The function signature was recently updated to add a new parameter for DNS provider configurations.

## Function Signature Change

**Old signature** (15 parameters):

```go
GenerateConfig(hosts []models.ProxyHost, storageDir, acmeEmail, frontendDir, sslProvider string, acmeStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled bool, adminWhitelist string, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string, decisions []models.SecurityDecision, secCfg *models.SecurityConfig)
```

**New signature** (16 parameters):

```go
GenerateConfig(hosts []models.ProxyHost, storageDir, acmeEmail, frontendDir, sslProvider string, acmeStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled bool, adminWhitelist string, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string, decisions []models.SecurityDecision, secCfg *models.SecurityConfig, dnsProviderConfigs []DNSProviderConfig)
```

**New parameter**: `dnsProviderConfigs []DNSProviderConfig` - provides DNS provider configuration for DNS challenge-based certificate issuance.

## Failing Test Cases

All 7 test cases need the same fix - append `nil` as the 16th argument:

| Line | Test Function | Current Context | Fix |
|------|--------------|----------------|-----|
| 14 | `TestGenerateConfig_CatchAllFrontend` | Testing frontend catch-all routes | Pass `nil` (no DNS providers needed) |
| 36 | `TestGenerateConfig_AdvancedInvalidJSON` | Testing invalid JSON in advanced_config | Pass `nil` (no DNS providers needed) |
| 67 | `TestGenerateConfig_AdvancedArrayHandler` | Testing array handlers in advanced_config | Pass `nil` (no DNS providers needed) |
| 81 | `TestGenerateConfig_LowercaseDomains` | Testing domain name normalization | Pass `nil` (no DNS providers needed) |
| 97 | `TestGenerateConfig_AdvancedObjectHandler` | Testing object handler in advanced_config | Pass `nil` (no DNS providers needed) |
| 114 | `TestGenerateConfig_AdvancedHeadersStringToArray` | Testing header normalization | Pass `nil` (no DNS providers needed) |
| 175 | `TestGenerateConfig_ACLWhitelistIncluded` | Testing ACL handler inclusion | Pass `nil` (no DNS providers needed) |

## Implementation

For all 7 test cases, append `, nil` as the last argument to the `GenerateConfig` call.

**Example fix** for line 14:

```go
// Before
cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil)

// After
cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
```

All 7 test cases are unit tests that don't require DNS provider configurations, so passing `nil` is appropriate.
