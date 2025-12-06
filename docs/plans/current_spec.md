# SSL Provider Selection Feature Plan

## Overview
This plan details the implementation of a user-configurable SSL Certificate Provider setting in the System Settings page. The goal is to allow users to choose between "Auto (Recommended)", "Let's Encrypt (Staging)", "Let's Encrypt (Prod)", and "ZeroSSL".

## 1. Backend Changes

### Database Schema
No schema changes are required. The setting will be stored in the existing `settings` table under the key `caddy.ssl_provider`.

### Logic Updates

#### `backend/internal/caddy/manager.go`

**Function**: `ApplyConfig`

**Current Logic**:
- Fetches `caddy.ssl_provider` setting.
- Uses `m.acmeStaging` (initialized from config/env) for staging status.

**New Logic**:
- Fetch `caddy.ssl_provider` setting.
- Parse the value to determine the effective `sslProvider` string and `acmeStaging` boolean.
- **Mapping**:
    - `auto` (or empty/missing):
        - `sslProvider` = `""` (defaults to "both" in `GenerateConfig`)
        - `acmeStaging` = `false` (Recommended default)
    - `letsencrypt-staging`:
        - `sslProvider` = `"letsencrypt"`
        - `acmeStaging` = `true`
    - `letsencrypt-prod`:
        - `sslProvider` = `"letsencrypt"`
        - `acmeStaging` = `false`
    - `zerossl`:
        - `sslProvider` = `"zerossl"`
        - `acmeStaging` = `false`
- Pass these derived values to `generateConfigFunc`.

**Code Snippet (Conceptual)**:
```go
// Fetch SSL Provider setting
var sslProviderSetting models.Setting
var sslProviderVal string
if err := m.db.Where("key = ?", "caddy.ssl_provider").First(&sslProviderSetting).Error; err == nil {
    sslProviderVal = sslProviderSetting.Value
}

    // Determine effective provider and staging flag
    effectiveProvider := ""
    effectiveStaging := false // Default to prod

    switch sslProviderVal {
    case "letsencrypt-staging":
        effectiveProvider = "letsencrypt"
        effectiveStaging = true
    case "letsencrypt-prod":
        effectiveProvider = "letsencrypt"
        effectiveStaging = false
    case "zerossl":
        effectiveProvider = "zerossl"
        effectiveStaging = false
    case "auto":
        effectiveProvider = "" // "both"
        effectiveStaging = false
    default:
        // Fallback to existing behavior or default to auto
        effectiveProvider = ""
        effectiveStaging = m.acmeStaging // Respect env var if setting is unset? Or just default to false?
        // Better to default to false for stability, or respect env var if "auto" isn't explicitly set.
        if sslProviderVal == "" {
             effectiveStaging = m.acmeStaging
        }
    }

    // ...
    config, err := generateConfigFunc(..., effectiveProvider, effectiveStaging, ...)
```

## 2. Frontend Changes

### UI Updates

#### `frontend/src/pages/SystemSettings.tsx`

**Component**: `SystemSettings`

**Changes**:
- Update the `sslProvider` state initialization to handle the new values.
- Update the `<select>` element for "SSL Provider" to include the new options.

**New Options**:
- **Label**: `Auto (Recommended)` | **Value**: `auto`
- **Label**: `Let's Encrypt (Staging)` | **Value**: `letsencrypt-staging`
- **Label**: `Let's Encrypt (Prod)` | **Value**: `letsencrypt-prod`
- **Label**: `ZeroSSL` | **Value**: `zerossl`

**Code Snippet**:
```tsx
            <div className="w-full">
            <label className="block text-sm font-medium text-gray-300 mb-1.5">
              SSL Provider
            </label>
            <select
              value={sslProvider}
              onChange={(e) => setSslProvider(e.target.value)}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
            >
              <option value="auto">Auto (Recommended)</option>
              <option value="letsencrypt-prod">Let's Encrypt (Prod)</option>
              <option value="letsencrypt-staging">Let's Encrypt (Staging)</option>
              <option value="zerossl">ZeroSSL</option>
            </select>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              Choose the Certificate Authority. 'Auto' uses Let's Encrypt with ZeroSSL fallback. Staging is for testing.
            </p>
          </div>
```

### State Management
- Ensure `sslProvider` defaults to `auto` if the API returns an empty value or a value not in the list (for backward compatibility).
- The `saveSettingsMutation` will send the selected string value (`auto`, `letsencrypt-staging`, etc.) to the backend.

## 3. Verification Plan
1.  **Frontend**:
    -   Verify the dropdown shows all 4 options.
    -   Verify selecting an option and saving persists the value (reload page).
2.  **Backend**:
    -   Verify the `settings` table updates with the correct key-value pair.
    -   **Critical**: Verify the generated Caddy config (via logs or `backend/data/caddy/config-*.json` snapshots) reflects the choice:
        -   `auto`: Should show multiple issuers (ACME + ZeroSSL).
        -   `letsencrypt-staging`: Should show ACME issuer with staging CA URL.
        -   `letsencrypt-prod`: Should show ACME issuer without staging CA URL.
        -   `zerossl`: Should show only ZeroSSL issuer.
