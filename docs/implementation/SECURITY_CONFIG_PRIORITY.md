# Security Configuration Priority System

## Overview

The Charon security configuration system uses a three-tier priority chain to determine the effective security settings. This allows for flexible configuration management across different deployment scenarios.

## Priority Chain

1. **Settings Table** (Highest Priority)
   - Module switches stored in the `settings` database table (`security.waf.enabled`, `security.rate_limit.enabled`, `security.acl.enabled`)
   - A value of `"true"` or `"false"` (any case, surrounding spaces ignored) decides the module. Any other value is ignored, so the next source decides
   - Takes precedence over all other sources

2. **SecurityConfig Database Record** (Middle Priority)
   - Persistent configuration stored in the `security_configs` row named `default`
   - Holds the WAF mode and the rate limit mode (or its legacy enable flag)
   - A blank column has no opinion, so the next source decides
   - Overrides static configuration file settings

3. **Static Configuration File** (Lowest Priority)
   - Default values from `config/config.yaml` or environment variables
   - Fallback when neither a switch nor a row decides
   - Used for initial setup and defaults

## How It Works

The proxy configuration and the `/api/v1/security/status` endpoint use the same resolver (`ResolveModule` in `backend/internal/models/security_modules.go`), so both report the same state for each module. For each module the resolver:

1. Reads the settings key. A recognised `true` or `false` wins
2. Otherwise reads the default row. A set column decides
3. Otherwise uses the static configuration

The Cerberus switch is checked first. When it is off, every module reports as off.

## Who Writes the Switches

Every write to a module switch goes through one set of helpers (`SetModuleState` and `ClearModuleKey` in `backend/internal/models/security_module_state.go`), inside the caller's transaction:

- **Turning a module on** writes the key and turns on the security framework. The WAF mirror moves a missing or disabled WAF mode to `block` (an explicit `monitor` mode is kept). The rate limit mirror marks the rate limit enabled and fills each unset or zero threshold with the standard value.
- **Turning a module off** writes the key only. It never changes the default row.
- **A config save** writes a switch only when the value it sends differs from the saved value.
- **The emergency reset** writes the WAF, rate limit, ACL and CrowdSec keys as `"false"`.
- **Clearing a key** removes it, so the resolver falls back to the default row and then the static configuration.

## Supported Settings Table Keys

### Cerberus (Master Switch)

- `feature.cerberus.enabled` - "true"/"false" - Enables/disables all security features

### WAF (Web Application Firewall)

- `security.waf.enabled` - "true"/"false" - Turns the WAF on or off. The saved WAF mode is still used for rule sets that have no mode of their own

### Rate Limiting

- `security.rate_limit.enabled` - "true"/"false" - Turns the rate limit on or off. Its numbers come from the default row. Turning it on fills unset or zero numbers with the standard values; if the numbers are still unset, the proxy does not apply the limit

### CrowdSec

- `security.crowdsec.enabled` - "true"/"false" - Sets CrowdSec to local/disabled
- `security.crowdsec.mode` - "local"/"disabled" - Direct mode override

### ACL (Access Control Lists)

- `security.acl.enabled` - "true"/"false" - Overrides ACL mode

## Examples

### Example 1: Settings Override SecurityConfig

```go
// Static Config
config.SecurityConfig{
    CerberusEnabled: true,
    WAFMode: "disabled",
}

// SecurityConfig DB
SecurityConfig{
    Name: "default",
    Enabled: true,
    WAFMode: "block",  // The row would turn the WAF on
}

// Settings Table
Setting{Key: "security.waf.enabled", Value: "false"}

// Result: WAF is DISABLED (Settings table wins)
```

### Example 2: SecurityConfig Override Static

```go
// Static Config
config.SecurityConfig{
    CerberusEnabled: true,
    RateLimitMode: "disabled",
}

// SecurityConfig DB
SecurityConfig{
    Name: "default",
    Enabled: true,
    RateLimitMode: "enabled",  // Overrides static
}

// Settings Table
// (no settings for rate_limit)

// Result: Rate Limit is ENABLED (SecurityConfig DB wins)
```

### Example 3: Static Config Fallback

```go
// Static Config
config.SecurityConfig{
    CerberusEnabled: true,
    CrowdSecMode: "local",
}

// SecurityConfig DB
// (no record found)

// Settings Table
// (no settings)

// Result: CrowdSec is LOCAL (Static config wins)
```

## Important Notes

1. **Cerberus Master Switch**: All security features require Cerberus to be enabled. If Cerberus is disabled at any priority level, all features are disabled regardless of their individual settings.

2. **Mode Mapping**: Invalid CrowdSec modes are mapped to "disabled" for safety.

3. **Database Priority**: SecurityConfig DB record must have `name = "default"` to be recognized.

4. **Backward Compatibility**: The system maintains backward compatibility with the older `RateLimitEnable` boolean field by mapping it to `RateLimitMode`.

## Testing

Comprehensive unit tests verify the priority chain:

- `TestSecurityHandler_Priority_SettingsOverSecurityConfig` - Tests all three priority levels
- `TestSecurityHandler_Priority_AllModules` - Tests all security modules together
- `TestSecurityHandler_GetStatus_RespectsSettingsTable` - Tests Settings table overrides
- `TestSecurityHandler_ACL_DBOverride` - Tests ACL specific overrides
- `TestSecurityHandler_CrowdSec_Mode_DBOverride` - Tests CrowdSec mode overrides

## Implementation Details

The status handler (`backend/internal/api/handlers/security_handler.go`, `GetStatus`) reads the default row and the Cerberus switch, then calls the shared resolver for the WAF, rate limit and ACL. The proxy configuration (`backend/internal/caddy/manager.go`) calls the same resolver, so a module the status reports as on is the module the proxy applies.

## QA Verification

All previously failing tests now pass:

- ✅ `TestCertificateHandler_Delete_NotificationRateLimiting`
- ✅ `TestSecurityHandler_ACL_DBOverride`
- ✅ `TestSecurityHandler_CrowdSec_Mode_DBOverride`
- ✅ `TestSecurityHandler_GetStatus_RespectsSettingsTable` (all 6 subtests)
- ✅ `TestSecurityHandler_GetStatus_WAFModeFromSettings`
- ✅ `TestSecurityHandler_GetStatus_RateLimitModeFromSettings`

## Migration Notes

For existing deployments:

1. No database migration required - Settings table already exists
2. SecurityConfig records work as before
3. New Settings table overrides are optional
4. System remains backward compatible with all existing configurations
