# Cerberus/Security Feature Issues - Diagnostic Plan

**Date:** December 12, 2025
**Status:** Analysis Complete

---

## Issue Summary

| # | Issue | Severity |
|---|-------|----------|
| 1 | Cerberus shows ON by default on first load (should be OFF) | High |
| 2 | Cerberus dashboard header shows "disabled" even when enabled | Medium |
| 3 | CrowdSec toggle auto-enables when Cerberus is enabled | Medium |
| 4 | CrowdSec toggle unresponsive + Config button grayed out | High |

---

## Root Cause Analysis

### Issue 1: Cerberus Shows ON by Default

**Root Cause:** The `feature_flags_handler.go` has a default value of `true` for all feature flags including `feature.cerberus.enabled`.

**File:** [backend/internal/api/handlers/feature_flags_handler.go#L39-L42](../../backend/internal/api/handlers/feature_flags_handler.go#L39-L42)

```go
// Line 39-42
for _, key := range defaultFlags {
    defaultVal := true  // <-- THIS IS THE BUG
    if v, ok := defaultFlagValues[key]; ok {
        defaultVal = v
    }
```

**Problem:** The code sets `defaultVal := true` for all flags, then only overrides it if the key exists in `defaultFlagValues`. However, `feature.cerberus.enabled` is NOT in `defaultFlagValues`:

```go
// Line 29-31
var defaultFlagValues = map[string]bool{
    "feature.crowdsec.console_enrollment": false,
}
```

**Result:** On first load with an empty database, `feature.cerberus.enabled` defaults to `true` instead of `false`.

**Additional Context:**
- The [backend/internal/config/config.go#L60](../../backend/internal/config/config.go#L60) correctly defaults `CerberusEnabled` to `false`:
  ```go
  CerberusEnabled: getEnvAny("false", "CERBERUS_SECURITY_CERBERUS_ENABLED", ...) == "true"
  ```
- However, the feature flags handler ignores this config and uses its own default.

---

### Issue 2: Dashboard Header Shows "Disabled" Even When Enabled

**Root Cause:** The header banner logic in `Security.tsx` checks `status.cerberus?.enabled` which comes from the security status API, but there's a **data source mismatch**.

**Files:**
- [frontend/src/pages/Security.tsx#L141-L153](../../frontend/src/pages/Security.tsx#L141-L153) - Header banner logic
- [backend/internal/api/handlers/security_handler.go#L35-L49](../../backend/internal/api/handlers/security_handler.go#L35-L49) - Security status API

**Problem Flow:**

1. **Security.tsx** checks `status.cerberus?.enabled` from `/api/v1/security/status`
2. **security_handler.go** reads from config AND settings table:
   ```go
   // Line 36-48
   enabled := h.cfg.CerberusEnabled
   var settingKey = "security.cerberus.enabled"  // <-- WRONG KEY!
   if h.db != nil {
       var setting struct{ Value string }
       if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", settingKey).Scan(&setting).Error; ...
   ```
3. **SystemSettings.tsx** toggles `feature.cerberus.enabled` (via feature flags API)

**The Mismatch:**

| Component | Key Used |
|-----------|----------|
| SystemSettings toggle | `feature.cerberus.enabled` |
| Security status API | `security.cerberus.enabled` |

The toggle writes to `feature.cerberus.enabled` but the security status reads from `security.cerberus.enabled` - **two different keys!**

---

### Issue 3: CrowdSec Auto-Enables When Cerberus is Enabled

**Root Cause:** The `docker-compose.override.yml` and `docker-compose.local.yml` both set `CHARON_SECURITY_CROWDSEC_MODE=local`:

**File:** [docker-compose.override.yml#L21](../../docker-compose.override.yml#L21)
```yaml
- CHARON_SECURITY_CROWDSEC_MODE=local
```

**Problem:** When the container starts:
1. Config loads with `CrowdSecMode: "local"` from env var
2. Security status API returns `crowdsec.enabled: true` because mode is "local"
3. Frontend shows CrowdSec as enabled

**File:** [backend/internal/api/handlers/security_handler.go#L59-L62](../../backend/internal/api/handlers/security_handler.go#L59-L62)
```go
// Allow runtime override for CrowdSec enabled flag via settings table
crowdsecEnabled := mode == "local"  // <-- Auto-true if mode is "local"
```

---

### Issue 4: CrowdSec Toggle Unresponsive + Config Button Grayed Out

**Root Cause:** Multiple issues combine to break the toggle:

**A. Toggle Disabled Logic:**

**File:** [frontend/src/pages/Security.tsx#L127](../../frontend/src/pages/Security.tsx#L127)
```tsx
const crowdsecToggleDisabled = cerberusDisabled || crowdsecPowerMutation.isPending
```

**File:** [frontend/src/pages/Security.tsx#L126](../../frontend/src/pages/Security.tsx#L126)
```tsx
const cerberusDisabled = !status.cerberus?.enabled
```

Since `status.cerberus?.enabled` is `false` due to Issue 2 (wrong settings key), `cerberusDisabled` is `true`, making the toggle disabled.

**B. Config Button Disabled:**

**File:** [frontend/src/pages/Security.tsx#L128](../../frontend/src/pages/Security.tsx#L128)
```tsx
const crowdsecControlsDisabled = cerberusDisabled || crowdsecPowerMutation.isPending
```

Same logic - the controls are disabled because Cerberus appears disabled.

**C. Switch Component Event Handling:**

**File:** [frontend/src/components/ui/Switch.tsx#L17-L20](../../frontend/src/components/ui/Switch.tsx#L17-L20)

The Switch component passes `disabled` to the native checkbox input, which prevents click events. This is correct behavior - the issue is the `disabled` prop is incorrectly `true`.

---

## Recommended Fixes

### Fix 1: Update Feature Flag Defaults

**File:** `backend/internal/api/handlers/feature_flags_handler.go`

```go
// Change defaultFlagValues to include cerberus.enabled as false
var defaultFlagValues = map[string]bool{
    "feature.cerberus.enabled":            false, // ADD THIS
    "feature.crowdsec.console_enrollment": false,
    "feature.uptime.enabled":              true,  // Uptime can default ON
}
```

### Fix 2: Align Settings Keys

**Option A (Recommended):** Update security_handler.go to read from feature flags key

**File:** `backend/internal/api/handlers/security_handler.go`

```go
// Line 37: Change from
var settingKey = "security.cerberus.enabled"
// To
var settingKey = "feature.cerberus.enabled"
```

**Option B:** Create a sync mechanism between feature flags and security settings

### Fix 3: Remove CrowdSec Mode Override from Docker Compose

**Files:**
- `docker-compose.override.yml`
- `docker-compose.local.yml`

```yaml
# Remove or comment out:
# - CHARON_SECURITY_CROWDSEC_MODE=local
# Or change to:
- CHARON_SECURITY_CROWDSEC_MODE=disabled
```

### Fix 4: No Additional Fix Needed

Issue 4 is a symptom of Issues 1-2. Once those are fixed:
- `cerberusDisabled` will be `false` when Cerberus is enabled
- `crowdsecToggleDisabled` will be `false`
- `crowdsecControlsDisabled` will be `false`
- Toggle and Config button will be interactive

---

## Test Scenarios

### Test 1: Fresh Install Default State
```
Given: Clean database, no env vars set
When: User loads the Settings > System page
Then: Cerberus toggle should be OFF
And: /api/v1/feature-flags returns { "feature.cerberus.enabled": false }
```

### Test 2: Cerberus Toggle Sync
```
Given: User is on Settings > System page
When: User enables Cerberus toggle
Then: /api/v1/security/status returns { "cerberus": { "enabled": true } }
And: Security dashboard header banner is NOT displayed
```

### Test 3: CrowdSec Toggle Interaction
```
Given: Cerberus is enabled
And: User is on Security dashboard
When: User clicks CrowdSec toggle
Then: Toggle should respond to click
And: CrowdSec enabled state should change
And: Toast notification should appear
```

### Test 4: CrowdSec Config Button
```
Given: Cerberus is enabled
And: User is on Security dashboard
When: User clicks CrowdSec "Config" button
Then: User should navigate to /security/crowdsec
And: Button should NOT be grayed out
```

### Test 5: Environment Variable Override
```
Given: CERBERUS_SECURITY_CERBERUS_ENABLED=true set
When: User loads Settings > System (fresh DB)
Then: Cerberus toggle should be ON (env override)
```

---

## Implementation Priority

| Priority | Fix | Effort | Impact |
|----------|-----|--------|--------|
| P0 | Fix 2 (Key alignment) | Low | High - Fixes Issues 2, 4 |
| P1 | Fix 1 (Default values) | Low | High - Fixes Issue 1 |
| P2 | Fix 3 (Docker compose) | Low | Medium - Fixes Issue 3 |

---

## Files to Modify

1. **backend/internal/api/handlers/feature_flags_handler.go** - Add default value for cerberus
2. **backend/internal/api/handlers/security_handler.go** - Change settings key to `feature.cerberus.enabled`
3. **docker-compose.override.yml** - Remove or change CrowdSec mode
4. **docker-compose.local.yml** - Remove or change CrowdSec mode

---

## Additional Observations

1. **Dual Control Systems:** There are two overlapping control systems:
   - Feature flags (`feature.cerberus.enabled`) - toggled in SystemSettings.tsx
   - Security config (`SecurityConfig.Enabled` in DB) - used by Enable/Disable endpoints

   Consider consolidating to one source of truth.

2. **Config vs Settings:** The `config.SecurityConfig` struct loaded from env vars is separate from DB-backed `SecurityConfig` model. This creates confusion about which takes precedence.

3. **No Migration:** When updating default values, existing users may need a migration or reset to see the new defaults.

---

## Code Reference Summary

| File | Line | Purpose |
|------|------|---------|
| `feature_flags_handler.go` | L29-31 | Missing cerberus default |
| `feature_flags_handler.go` | L39 | `defaultVal := true` bug |
| `security_handler.go` | L37 | Wrong settings key |
| `Security.tsx` | L126-128 | Disabled state logic |
| `SystemSettings.tsx` | L99-105 | Feature toggle UI |
| `docker-compose.override.yml` | L21 | CrowdSec mode env var |
| `config.go` | L60 | Correct cerberus default |
