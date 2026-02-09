# Security Configuration Priority System

## Overview

The Charon security configuration system uses a three-tier priority chain to determine the effective security settings. This allows for flexible configuration management across different deployment scenarios.

## Priority Chain

1. **Settings Table** (Highest Priority)
   - Runtime overrides stored in the `settings` database table
   - Used for feature flags and quick toggles
   - Can enable/disable individual security modules without full config changes
   - Takes precedence over all other sources

2. **SecurityConfig Database Record** (Middle Priority)
   - Persistent configuration stored in the `security_configs` table
   - Contains comprehensive security settings including admin whitelists, rate limits, etc.
   - Overrides static configuration file settings
   - Used for user-managed security configuration

3. **Static Configuration File** (Lowest Priority)
   - Default values from `config/config.yaml` or environment variables
   - Fallback when no database overrides exist
   - Used for initial setup and defaults

## How It Works

When the `/api/v1/security/status` endpoint is called, the system:

1. Starts with static config values
2. Checks for SecurityConfig DB record and overrides static values if present
3. Checks for Settings table entries and overrides both static and DB values if present
4. Computes effective enabled state based on final values

## Supported Settings Table Keys

### Cerberus (Master Switch)

- `feature.cerberus.enabled` - "true"/"false" - Enables/disables all security features

### WAF (Web Application Firewall)

- `security.waf.enabled` - "true"/"false" - Overrides WAF mode

### Rate Limiting

- `security.rate_limit.enabled` - "true"/"false" - Overrides rate limit mode

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
    WAFMode: "enabled",  // Tries to enable WAF
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

The priority logic is implemented in [security_handler.go](backend/internal/api/handlers/security_handler.go#L55-L170):

```go
// GetStatus returns the current status of all security services.
// Priority chain:
// 1. Settings table (highest - runtime overrides)
// 2. SecurityConfig DB record (middle - user configuration)
// 3. Static config (lowest - defaults)
func (h *SecurityHandler) GetStatus(c *gin.Context) {
    // Start with static config defaults
    enabled := h.cfg.CerberusEnabled
    wafMode := h.cfg.WAFMode
    // ... other fields

    // Override with database SecurityConfig if present (priority 2)
    if h.db != nil {
        var sc models.SecurityConfig
        if err := h.db.Where("name = ?", "default").First(&sc).Error; err == nil {
            enabled = sc.Enabled
            if sc.WAFMode != "" {
                wafMode = sc.WAFMode
            }
            // ... other overrides
        }

        // Check runtime setting overrides from settings table (priority 1 - highest)
        var setting struct{ Value string }
        if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.waf.enabled").Scan(&setting).Error; err == nil && setting.Value != "" {
            if strings.EqualFold(setting.Value, "true") {
                wafMode = "enabled"
            } else {
                wafMode = "disabled"
            }
        }
        // ... other setting checks
    }
    // ... compute effective state and return
}
```

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
