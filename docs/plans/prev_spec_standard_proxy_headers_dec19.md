# Implementation Plan: Standard Proxy Headers on ALL Proxy Hosts

**Date:** December 19, 2025
**Status:** Revised (Supervisor Approved with Critical Gaps Addressed)
**Priority:** High
**Estimated Effort:** 5-6 hours

---

## Executive Summary

Currently, X-Forwarded-* headers are ONLY added when WebSocket support is enabled (`enableWS=true`). This creates a critical gap: applications that don't use WebSockets but still need to know they're behind a proxy (for logging, security, rate limiting, etc.) receive no proxy awareness headers.

**This implementation adds 4 explicit standard proxy headers to ALL reverse proxy configurations, regardless of WebSocket or application type, while leveraging Caddy's native X-Forwarded-For handling.**

### Supervisor Review Status

✅ **Approved with Critical Gaps Addressed**

**Critical Gaps Fixed:**

1. ✅ X-Forwarded-For duplication prevented (rely on Caddy's native behavior)
2. ✅ Backward compatibility via feature flag + database migration
3. ✅ Trusted proxies configuration verified and tested

---

## Problem Statement

### Current Behavior

```go
// In ReverseProxyHandler:
if enableWS {
    setHeaders["X-Forwarded-Proto"] = []string{"{http.request.scheme}"}
    setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
    setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
}
```

**Issues:**

1. ❌ Generic proxy hosts (`application="none"`, `enableWS=false`) get NO proxy headers
2. ❌ Applications that don't use WebSockets lose client IP information
3. ❌ Backend applications can't detect they're behind a proxy
4. ❌ Security features (rate limiting, IP-based ACLs) break
5. ❌ Logging shows proxy IP instead of real client IP

### Real-World Impact Examples

**Scenario 1: Custom API Behind Proxy**

- User creates proxy host for `api.example.com` → `localhost:3000`
- No WebSocket, no specific application type
- Backend API tries to log client IP: Gets proxy IP (127.0.0.1)
- Rate limiting by IP: All requests appear from same IP (broken)

**Scenario 2: Web Application with CSRF Protection**

- Application checks `X-Forwarded-Proto` to enforce HTTPS in redirect URLs
- Without header: App generates `http://` URLs even when accessed via `https://`
- Result: Mixed content warnings, security issues

**Scenario 3: Multi-Proxy Chain**

- External Cloudflare → Charon → Backend
- Without `X-Forwarded-For`: Backend only sees Charon's IP
- Can't trace original client through proxy chain

---

## Solution Design

### Standard Proxy Headers Strategy

**We explicitly set 4 headers and rely on Caddy's native behavior for X-Forwarded-For:**

1. **`X-Real-IP`**: Single IP of the immediate client
   - Value: `{http.request.remote.host}`
   - Most applications check this first for client IP
   - **Explicitly set by us**

2. **`X-Forwarded-For`**: Comma-separated list of client + proxies
   - Format: `client, proxy1, proxy2`
   - **Handled natively by Caddy's `reverse_proxy` directive**
   - **NOT explicitly set** (prevents duplication)
   - Caddy automatically appends to existing header

3. **`X-Forwarded-Proto`**: Original protocol (http/https)
   - Value: `{http.request.scheme}`
   - Critical for HTTPS enforcement and redirect generation
   - **Explicitly set by us**

4. **`X-Forwarded-Host`**: Original Host header
   - Value: `{http.request.host}`
   - Needed for virtual host routing and URL generation
   - **Explicitly set by us**

5. **`X-Forwarded-Port`**: Original port
   - Value: `{http.request.port}`
   - Important for non-standard ports (e.g., 8443)
   - **Explicitly set by us**

### Why Not Explicitly Set X-Forwarded-For?

**Evidence from codebase:** Code comment in `types.go` states:

```go
// Caddy already sets X-Forwarded-For and X-Forwarded-Proto by default
```

**Problem:** If we explicitly set `X-Forwarded-For`, we'll create duplicates:

- Caddy's native: `X-Forwarded-For: 203.0.113.1`
- Our explicit: `X-Forwarded-For: 203.0.113.1`
- Result: Caddy appends both → `X-Forwarded-For: 203.0.113.1, 203.0.113.1`

**Solution:** Trust Caddy's native handling. We explicitly set 4 headers; Caddy handles the 5th.

### Architecture Decision

**Layered approach with feature flag for backward compatibility:**

```
Feature Flag Check (EnableStandardHeaders)
  ↓
Standard Proxy Headers (if enabled: 4 explicit headers)
  ↓
WebSocket Headers (if enableWS)
  ↓
Application-Specific Headers (can override)
```

This ensures:

- ✅ Backward compatibility: Existing proxy hosts default to old behavior
- ✅ Opt-in for new feature: New hosts get standard headers by default
- ✅ All proxy hosts CAN get proxy awareness (if flag enabled)
- ✅ WebSocket support only adds `Upgrade`/`Connection` (not proxy headers)
- ✅ Applications can still override headers if needed
- ✅ Consistent behavior across all proxy types when enabled

### Backward Compatibility Strategy

**Critical Gap #2 Addressed: Feature Flag System**

To prevent breaking existing proxy configurations, we implement a feature flag:

```go
type ProxyHost struct {
    // ... existing fields ...

    // EnableStandardHeaders controls whether standard proxy headers are added
    // Default: true for NEW hosts, false for EXISTING hosts (via migration)
    // When true: Adds X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, X-Forwarded-Port
    // When false: Old behavior (headers only with WebSocket)
    EnableStandardHeaders *bool `json:"enable_standard_headers" gorm:"default:true"`
}
```

**Migration Strategy:**

1. **Existing hosts**: Migration sets `enable_standard_headers = false` (preserve old behavior)
2. **New hosts**: Default `enable_standard_headers = true` (get new behavior)
3. **User opt-in**: Users can enable for existing hosts via API

**Rollback Path:**

- Set `enable_standard_headers = false` via API to revert to old behavior
- No data loss, fully reversible

### Trusted Proxies Security

**Critical Gap #3 Addressed: Security Configuration Verification**

When using X-Forwarded-* headers, we MUST configure `trusted_proxies` in Caddy to prevent IP spoofing attacks.

**Security Risk Without `trusted_proxies`:**

- Attacker sets `X-Forwarded-For: 1.2.3.4` in request
- Backend trusts the forged IP
- Bypasses IP-based rate limiting, ACLs, etc.

**Mitigation:**

1. **Always set `trusted_proxies`** in generated Caddy config
2. **Default value:** `private_ranges` (RFC 1918 + loopback)
3. **Configurable:** Users can override via advanced_config

**Implementation:**

```json
{
  "handle": [{
    "handler": "reverse_proxy",
    "upstreams": [...],
    "headers": {
      "request": {
        "set": { "X-Real-IP": [...] }
      }
    },
    "trusted_proxies": {
      "source": "static",
      "ranges": ["private_ranges"]
    }
  }]
}
```

**Test Requirement:**

- Verify `trusted_proxies` present in ALL generated reverse_proxy configs
- Verify users can override via `advanced_config`

---

## Implementation

### File 1: `backend/internal/models/proxy_host.go`

**Add feature flag field:**

```go
type ProxyHost struct {
    // ... existing fields ...

    // EnableStandardHeaders controls whether standard proxy headers are added
    // Default: true for NEW hosts, false for EXISTING hosts (via migration)
    EnableStandardHeaders *bool `json:"enable_standard_headers" gorm:"default:true"`
}
```

### File 2: `backend/internal/database/migrations/YYYYMMDDHHMMSS_add_enable_standard_headers.go`

**Create migration:**

```go
package migrations

import (
    "gorm.io/gorm"
)

func init() {
    Migrations = append(Migrations, Migration{
        ID: "20251219000001",
        Migrate: func(db *gorm.DB) error {
            // Add column with default true
            if err := db.Exec(`
                ALTER TABLE proxy_hosts
                ADD COLUMN enable_standard_headers BOOLEAN DEFAULT true
            `).Error; err != nil {
                return err
            }

            // Set false for EXISTING hosts (backward compatibility)
            if err := db.Exec(`
                UPDATE proxy_hosts
                SET enable_standard_headers = false
                WHERE id IS NOT NULL
            `).Error; err != nil {
                return err
            }

            return nil
        },
        Rollback: func(db *gorm.DB) error {
            return db.Exec(`
                ALTER TABLE proxy_hosts
                DROP COLUMN enable_standard_headers
            `).Error
        },
    })
}
```

### File 3: `backend/internal/caddy/types.go`

**Key Changes:**

1. Check feature flag before adding standard headers
2. Explicitly set 4 headers (NOT X-Forwarded-For)
3. Move standard headers to TOP (before WebSocket/application logic)
4. Add `trusted_proxies` configuration
5. Remove duplicate header assignments
6. Add comprehensive comments explaining rationale

**Pseudo-code:**

```go
func (ph *ProxyHost) ReverseProxyHandler() map[string]interface{} {
    handler := map[string]interface{}{
        "handler": "reverse_proxy",
        "upstreams": [...],
    }

    setHeaders := make(map[string][]string)

    // STEP 1: Standard proxy headers (if feature enabled)
    if ph.EnableStandardHeaders == nil || *ph.EnableStandardHeaders {
        // Explicitly set 4 headers
        setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
        setHeaders["X-Forwarded-Proto"] = []string{"{http.request.scheme}"}
        setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
        setHeaders["X-Forwarded-Port"] = []string{"{http.request.port}"}

        // NOTE: X-Forwarded-For is handled natively by Caddy's reverse_proxy
        // Do NOT set it explicitly to avoid duplication
    }

    // STEP 2: WebSocket headers (if enabled)
    if enableWS {
        setHeaders["Upgrade"] = []string{"{http.request.header.Upgrade}"}
        setHeaders["Connection"] = []string{"{http.request.header.Connection}"}
    }

    // STEP 3: Application-specific headers
    switch application {
    case "plex":
        // Plex-specific headers (X-Real-IP already set above)
    case "jellyfin":
        // Jellyfin-specific headers
    }

    // STEP 4: Always set trusted_proxies for security
    handler["trusted_proxies"] = map[string]interface{}{
        "source": "static",
        "ranges": []string{"private_ranges"},
    }

    if len(setHeaders) > 0 {
        handler["headers"] = map[string]interface{}{
            "request": map[string]interface{}{
                "set": setHeaders,
            },
        }
    }

    return handler
}
```

---

## Frontend Changes Required

### Overview

Since `EnableStandardHeaders` has a database-level default (`true` for new rows, `false` for existing via migration), users need a way to:

1. **Understand** what the setting does
2. **Opt-in** existing proxy hosts to the new behavior
3. **(Optional)** Bulk-enable for all proxy hosts at once

### File 1: `frontend/src/api/proxyHosts.ts`

**Update TypeScript Interface:**

```typescript
export interface ProxyHost {
  uuid: string;
  name: string;
  // ... existing fields ...
  websocket_support: boolean;
  enable_standard_headers?: boolean;  // NEW: Optional (defaults true for new, false for existing)
  application: ApplicationPreset;
  // ... rest of fields ...
}
```

**Reasoning:** Optional because existing hosts won't have this field set (null in DB means "use default").

### File 2: `frontend/src/components/ProxyHostForm.tsx`

**Location in Form:** Add in the "SSL & Security Options" section, right after `websocket_support` checkbox.

**Add to formData state:**

```typescript
const [formData, setFormData] = useState<ProxyHostFormState>({
  // ... existing fields ...
  websocket_support: host?.websocket_support ?? true,
  enable_standard_headers: host?.enable_standard_headers ?? true,  // NEW
  application: (host?.application || 'none') as ApplicationPreset,
  // ... rest of fields ...
})
```

**Add UI Control (after WebSocket checkbox):**

```tsx
{/* Around line 892, after websocket_support checkbox */}
<label className="flex items-center gap-3">
  <input
    type="checkbox"
    checked={formData.enable_standard_headers ?? true}
    onChange={e => setFormData({ ...formData, enable_standard_headers: e.target.checked })}
    className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
  />
  <span className="text-sm text-gray-300">Enable Standard Proxy Headers</span>
  <div
    title="Adds X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, and X-Forwarded-Port headers to help backend applications detect client IPs, enforce HTTPS, and generate correct URLs. Recommended for all proxy hosts. Existing hosts: disabled by default for backward compatibility."
    className="text-gray-500 hover:text-gray-300 cursor-help"
  >
    <CircleHelp size={14} />
  </div>
</label>

{/* Optional: Show info banner when disabled on edit */}
{host && (formData.enable_standard_headers === false) && (
  <div className="bg-yellow-900/20 border border-yellow-600 rounded-lg p-3 mt-2">
    <div className="flex items-start gap-2">
      <Info className="w-5 h-5 text-yellow-500 flex-shrink-0 mt-0.5" />
      <div className="text-sm">
        <p className="font-medium text-yellow-400">Standard Proxy Headers Disabled</p>
        <p className="text-yellow-300/80 mt-1">
          This proxy host is using the legacy behavior (headers only with WebSocket support).
          Enable this option to ensure backend applications receive client IP and protocol information.
        </p>
      </div>
    </div>
  </div>
)}
```

**Visual Placement:**

```
☑ Block Exploits           ⓘ
☑ Websockets Support       ⓘ
☑ Enable Standard Proxy Headers  ⓘ  <-- NEW (right after WebSocket)
```

**Help Text:** "Adds X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, and X-Forwarded-Port headers to help backend applications detect client IPs, enforce HTTPS, and generate correct URLs. Recommended for all proxy hosts."

**Default Value:**

- **New hosts:** `true` (checkbox checked)
- **Existing hosts (edit mode):** Uses value from `host?.enable_standard_headers` (likely `false` for legacy hosts)

### File 3: `frontend/src/pages/ProxyHosts.tsx`

**Option A: Bulk Apply Integration (Recommended)**

Add `enable_standard_headers` to the existing "Bulk Apply Settings" modal (around line 64):

```typescript
const [bulkApplySettings, setBulkApplySettings] = useState<Record<string, { apply: boolean; value: boolean }>>({
  ssl_forced: { apply: false, value: true },
  http2_support: { apply: false, value: true },
  hsts_enabled: { apply: false, value: true },
  hsts_subdomains: { apply: false, value: true },
  block_exploits: { apply: false, value: true },
  websocket_support: { apply: false, value: true },
  enable_standard_headers: { apply: false, value: true },  // NEW
})
```

**Update modal to include new setting** (around line 734):

```tsx
{/* In Bulk Apply Settings modal, after websocket_support */}
<label className="flex items-center justify-between p-3 rounded-lg border border-border bg-surface-subtle hover:bg-surface-muted cursor-pointer">
  <div className="flex items-center gap-3">
    <Checkbox
      checked={bulkApplySettings.enable_standard_headers?.apply ?? false}
      onCheckedChange={(checked) => {
        setBulkApplySettings(prev => ({
          ...prev,
          enable_standard_headers: { ...prev.enable_standard_headers, apply: !!checked }
        }))
      }}
    />
    <span className="text-content-primary">Standard Proxy Headers</span>
  </div>
  {bulkApplySettings.enable_standard_headers?.apply && (
    <Switch
      checked={bulkApplySettings.enable_standard_headers?.value ?? true}
      onCheckedChange={(checked) => {
        setBulkApplySettings(prev => ({
          ...prev,
          enable_standard_headers: { ...prev.enable_standard_headers, value: checked }
        }))
      }}
    />
  )}
</label>
```

**Reasoning:** Users can enable standard headers for multiple existing hosts at once using the existing "Bulk Apply" feature.

**Option B: Dedicated "Enable Standard Headers for All" Button (Alternative)**

If you prefer a more explicit approach for this specific migration:

```tsx
{/* Add near bulk action buttons (around line 595) */}
<Button
  variant="secondary"
  onClick={async () => {
    const confirmed = confirm(
      `Enable standard proxy headers for all ${hosts.length} proxy hosts?\n\n` +
      'This will add X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, and X-Forwarded-Port headers to ALL proxy configurations.'
    )
    if (!confirmed) return

    toast.loading('Updating proxy hosts...')

    for (const host of hosts) {
      if (host.enable_standard_headers === false) {
        await updateHost(host.uuid, { enable_standard_headers: true })
      }
    }

    toast.success('Standard headers enabled for all proxy hosts')
  }}
  disabled={isUpdating || hosts.every(h => h.enable_standard_headers !== false)}
>
  <Globe className="w-4 h-4" />
  Enable Standard Headers (All)
</Button>
```

**Recommendation:** Use **Option A (Bulk Apply Integration)** because:

- ✅ Consistent with existing UI patterns
- ✅ Users already familiar with Bulk Apply workflow
- ✅ Allows selective application (choose which hosts)
- ✅ Less UI clutter (no new top-level button)

### File 4: `frontend/src/testUtils/createMockProxyHost.ts`

**Update mock to include new field:**

```typescript
export const createMockProxyHost = (overrides?: Partial<ProxyHost>): ProxyHost => ({
  uuid: 'test-uuid',
  name: 'Test Host',
  // ... existing fields ...
  websocket_support: false,
  enable_standard_headers: true,  // NEW: Default true for new hosts
  application: 'none',
  // ... rest of fields ...
})
```

### File 5: `frontend/src/utils/proxyHostsHelpers.ts`

**Update helper functions:**

```typescript
// Add to formatSettingLabel function (around line 15)
export const formatSettingLabel = (key: string): string => {
  const labels: Record<string, string> = {
    ssl_forced: 'Force SSL',
    http2_support: 'HTTP/2 Support',
    hsts_enabled: 'HSTS Enabled',
    hsts_subdomains: 'HSTS Subdomains',
    block_exploits: 'Block Exploits',
    websocket_support: 'Websockets Support',
    enable_standard_headers: 'Standard Proxy Headers',  // NEW
  }
  return labels[key] || key
}

// Add to settingHelpText function
export const settingHelpText = (key: string): string => {
  const helpTexts: Record<string, string> = {
    ssl_forced: 'Redirects HTTP to HTTPS',
    // ... existing entries ...
    websocket_support: 'Required for real-time apps',
    enable_standard_headers: 'Adds X-Real-IP and X-Forwarded-* headers for client IP detection',  // NEW
  }
  return helpTexts[key] || ''
}

// Update applyBulkSettingsToHosts to include new field
export const applyBulkSettingsToHosts = (
  hosts: ProxyHost[],
  settings: Record<string, { apply: boolean; value: boolean }>
): Partial<ProxyHost>[] => {
  return hosts.map(host => {
    const updates: Partial<ProxyHost> = { uuid: host.uuid }

    // Apply each selected setting
    Object.entries(settings).forEach(([key, { apply, value }]) => {
      if (apply) {
        updates[key as keyof ProxyHost] = value
      }
    })

    return updates
  })
}
```

### UI/UX Considerations

**Visual Design:**

- ✅ **Placement:** Right after "Websockets Support" checkbox (logical grouping)
- ✅ **Icon:** CircleHelp icon for tooltip (consistent with other options)
- ✅ **Default State:** Checked for new hosts, unchecked for existing hosts (reflects backend default)
- ✅ **Help Text:** Clear, concise explanation in tooltip

**User Journey:**

**Scenario 1: Creating New Proxy Host**

1. User clicks "Add Proxy Host"
2. Fills in domain, forward host/port
3. Sees "Enable Standard Proxy Headers" **checked by default** ✅
4. Hovers tooltip: Understands it adds proxy headers
5. Clicks Save → Backend receives `enable_standard_headers: true`

**Scenario 2: Editing Existing Proxy Host (Legacy)**

1. User edits existing proxy host (created before migration)
2. Sees "Enable Standard Proxy Headers" **unchecked** (legacy behavior)
3. Sees yellow info banner: "This proxy host is using the legacy behavior..."
4. User checks the box → Backend receives `enable_standard_headers: true`
5. Saves → Headers now added to this proxy host

**Scenario 3: Bulk Update (Recommended for Migration)**

1. User selects multiple proxy hosts (existing hosts without standard headers)
2. Clicks "Bulk Apply" button
3. Checks "Standard Proxy Headers" in modal
4. Toggles switch to `ON`
5. Clicks "Apply" → All selected hosts updated

**Error Handling:**

- If API returns error when updating `enable_standard_headers`, show toast error
- Validation: None needed (boolean field, can't be invalid)
- Rollback: User can uncheck and save again

### API Handler Changes (Backend)

**File:** `backend/internal/api/handlers/proxy_host_handler.go`

**Add to updateHost handler** (around line 212):

```go
// Around line 212, after websocket_support handling
if v, ok := payload["enable_standard_headers"].(bool); ok {
    host.EnableStandardHeaders = &v
}
```

**Add to createHost handler** (ensure default is respected):

```go
// In createHost function, no explicit handling needed
// GORM default will set enable_standard_headers=true for new records
```

**Reasoning:** The API already handles arbitrary boolean fields via type assertion. Just add one more case.

### Testing Requirements

**Frontend Unit Tests:**

1. **File:** `frontend/src/components/__tests__/ProxyHostForm.test.tsx`

```typescript
it('renders enable_standard_headers checkbox for new hosts', () => {
  render(<ProxyHostForm onSubmit={vi.fn()} onCancel={vi.fn()} />)

  const checkbox = screen.getByLabelText(/Enable Standard Proxy Headers/i)
  expect(checkbox).toBeInTheDocument()
  expect(checkbox).toBeChecked()  // Default true for new hosts
})

it('renders enable_standard_headers unchecked for legacy hosts', () => {
  const legacyHost = createMockProxyHost({ enable_standard_headers: false })
  render(<ProxyHostForm host={legacyHost} onSubmit={vi.fn()} onCancel={vi.fn()} />)

  const checkbox = screen.getByLabelText(/Enable Standard Proxy Headers/i)
  expect(checkbox).not.toBeChecked()
})

it('shows info banner when standard headers disabled on edit', () => {
  const legacyHost = createMockProxyHost({ enable_standard_headers: false })
  render(<ProxyHostForm host={legacyHost} onSubmit={vi.fn()} onCancel={vi.fn()} />)

  expect(screen.getByText(/Standard Proxy Headers Disabled/i)).toBeInTheDocument()
  expect(screen.getByText(/legacy behavior/i)).toBeInTheDocument()
})
```

1. **File:** `frontend/src/pages/__tests__/ProxyHosts-bulk-apply-all-settings.test.tsx`

```typescript
it('includes enable_standard_headers in bulk apply settings', async () => {
  // ... setup ...

  await userEvent.click(screen.getByText('Bulk Apply'))
  await waitFor(() => expect(screen.getByText('Bulk Apply Settings')).toBeTruthy())

  // Verify new setting is present
  expect(screen.getByText('Standard Proxy Headers')).toBeInTheDocument()

  // Toggle it on
  const checkbox = screen.getByLabelText(/Standard Proxy Headers/i)
  await userEvent.click(checkbox)

  // Verify toggle appears
  const toggle = screen.getByRole('switch', { name: /Standard Proxy Headers/i })
  expect(toggle).toBeInTheDocument()
})
```

**Integration Tests:**

1. **Manual Test:** Create new proxy host via UI → Verify API payload includes `enable_standard_headers: true`
2. **Manual Test:** Edit existing proxy host, enable checkbox → Verify API payload includes `enable_standard_headers: true`
3. **Manual Test:** Bulk apply to 5 hosts → Verify all updated via API

### Documentation Updates

**File:** `docs/API.md`

Add to ProxyHost model section:

```markdown
### ProxyHost Model

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| ... | ... | ... | ... | ... |
| `websocket_support` | boolean | No | `false` | Enable WebSocket protocol support |
| `enable_standard_headers` | boolean | No | `true` (new), `false` (existing) | Enable standard proxy headers (X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, X-Forwarded-Port) |
| `application` | string | No | `"none"` | Application preset configuration |
| ... | ... | ... | ... | ... |

**Note:** The `enable_standard_headers` field was added in v1.X.X. Existing proxy hosts default to `false` for backward compatibility. New proxy hosts default to `true`.
```

**File:** `README.md` or `docs/UPGRADE.md`

Add migration guide:

```markdown
## Upgrading to v1.X.X

### Standard Proxy Headers Feature

This release adds standard proxy headers to reverse proxy configurations:
- `X-Real-IP`: Client IP address
- `X-Forwarded-Proto`: Original protocol (http/https)
- `X-Forwarded-Host`: Original host header
- `X-Forwarded-Port`: Original port
- `X-Forwarded-For`: Handled natively by Caddy

**Existing Hosts:** Disabled by default (backward compatibility)
**New Hosts:** Enabled by default

**To enable for existing hosts:**
1. Go to Proxy Hosts page
2. Select hosts to update
3. Click "Bulk Apply"
4. Check "Standard Proxy Headers"
5. Toggle to ON
6. Click "Apply"

**Or enable per-host:**
1. Edit proxy host
2. Check "Enable Standard Proxy Headers"
3. Save
```

---

## Test Updates

### File: `backend/internal/caddy/types_extra_test.go`

#### Test 1: Rename and Update Existing Test

**Rename:** `TestReverseProxyHandler_NoWebSocketNoForwardedHeaders` → `TestReverseProxyHandler_StandardProxyHeadersAlwaysSet`

**New assertions:**

- Verify headers map EXISTS (was checking it DOESN'T exist)
- Verify 4 explicit standard proxy headers present (X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, X-Forwarded-Port)
- Verify X-Forwarded-For NOT in setHeaders (Caddy handles it natively)
- Verify WebSocket headers NOT present when `enableWS=false`
- Verify `trusted_proxies` configuration present

#### Test 2: Update WebSocket Test

**Update:** `TestReverseProxyHandler_WebSocketHeaders`

**New assertions:**

- Add check for `X-Forwarded-Port`
- Verify X-Forwarded-For NOT explicitly set
- Total 6 headers expected (4 standard + 2 WebSocket, X-Forwarded-For handled by Caddy)
- Verify `trusted_proxies` configuration present

#### Test 3: New Test - Feature Flag Disabled

**Add:** `TestReverseProxyHandler_FeatureFlagDisabled`

**Purpose:**

- Test backward compatibility
- Set `EnableStandardHeaders = false`
- Verify NO standard headers added (old behavior)
- Verify `trusted_proxies` NOT added when feature disabled

#### Test 4: New Test - X-Forwarded-For Not Duplicated

**Add:** `TestReverseProxyHandler_XForwardedForNotDuplicated`

**Purpose:**

- Verify X-Forwarded-For NOT in setHeaders map
- Document that Caddy handles it natively
- Prevent regression (ensure no one adds it back)

#### Test 5: New Test - Trusted Proxies Always Present

**Add:** `TestReverseProxyHandler_TrustedProxiesConfiguration`

**Purpose:**

- Verify `trusted_proxies` present when standard headers enabled
- Verify default value is `private_ranges`
- Test security requirement

#### Test 6: New Test - Application Headers Don't Duplicate

**Add:** `TestReverseProxyHandler_ApplicationHeadersDoNotDuplicate`

**Purpose:**

- Verify Plex/Jellyfin don't duplicate X-Real-IP
- Verify 4 standard headers present for applications
- Ensure map keys are unique

#### Test 7: New Test - WebSocket + Application Combined

**Add:** `TestReverseProxyHandler_WebSocketWithApplication`

**Purpose:**

- Test most complex scenario (WebSocket + Jellyfin + standard headers)
- Verify at least 6 headers present
- Ensure layered approach works correctly

#### Test 8: New Test - Advanced Config Override

**Add:** `TestReverseProxyHandler_AdvancedConfigOverridesTrustedProxies`

**Purpose:**

- Verify users can override `trusted_proxies` via `advanced_config`
- Test that advanced_config has higher priority

---

## Test Execution Plan

### Step 1: Run Tests Before Changes

```bash
cd backend && go test -v ./internal/caddy -run TestReverseProxyHandler
```

**Expected:** 3 tests pass

### Step 2: Apply Code Changes

- Add `EnableStandardHeaders` field to ProxyHost model
- Create database migration
- Modify `types.go` per specification
- Update ReverseProxyHandler logic

### Step 3: Update Tests

- Rename and update existing test
- Add 5 new tests (feature flag, X-Forwarded-For, trusted_proxies, advanced_config, combined)
- Update WebSocket test

### Step 4: Run Migration

```bash
cd backend && go run cmd/migrate/main.go
```

**Expected:** Migration applies successfully

### Step 5: Run Tests After Changes

```bash
cd backend && go test -v ./internal/caddy -run TestReverseProxyHandler
```

**Expected:** 8 tests pass

### Step 6: Full Test Suite

```bash
cd backend && go test ./...
```

**Expected:** All tests pass

### Step 7: Coverage

```bash
scripts/go-test-coverage.sh
```

**Expected:** Coverage maintained or increased (target: ≥85%)

### Step 8: Manual Testing with curl

**Test 1: Generic Proxy (New Host)**

```bash
# Create new proxy host via API (EnableStandardHeaders defaults to true)
curl -X POST http://localhost:8080/api/proxy-hosts \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"domain":"test.local","forward_host":"localhost","forward_port":3000}'

# Verify 4 headers sent to backend
curl -v http://test.local 2>&1 | grep -E 'X-(Real-IP|Forwarded)'
```

**Expected:** See X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, X-Forwarded-Port

**Test 2: Verify X-Forwarded-For Handled by Caddy**

```bash
# Check backend receives X-Forwarded-For (from Caddy, not our code)
curl -H "X-Forwarded-For: 203.0.113.1" http://test.local
# Backend should see: X-Forwarded-For: 203.0.113.1, <client-ip>
```

**Expected:** X-Forwarded-For present with proper chain

**Test 3: Existing Host (Backward Compatibility)**

```bash
# Existing host should have EnableStandardHeaders=false (from migration)
curl http://existing-host.local 2>&1 | grep -E 'X-(Real-IP|Forwarded)'
```

**Expected:** NO standard headers (old behavior preserved)

**Test 4: Enable Feature for Existing Host**

```bash
# Update existing host to enable standard headers
curl -X PATCH http://localhost:8080/api/proxy-hosts/1 \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"enable_standard_headers":true}'

curl http://existing-host.local 2>&1 | grep -E 'X-(Real-IP|Forwarded)'
```

**Expected:** NOW see 4 standard headers

**Test 5: CrowdSec Integration Still Works**

```bash
# Verify CrowdSec can still read client IP
scripts/crowdsec_integration.sh
```

**Expected:** All CrowdSec tests pass

---

## Definition of Done

### Backend Code Changes

- [ ] `proxy_host.go`: Added `EnableStandardHeaders *bool` field
- [ ] Migration: Created migration to add column with backward compatibility logic
- [ ] `types.go`: Modified `ReverseProxyHandler` to check feature flag
- [ ] `types.go`: Set 4 explicit headers (NOT X-Forwarded-For)
- [ ] `types.go`: Moved standard headers before WebSocket/application logic
- [ ] `types.go`: Added `trusted_proxies` configuration
- [ ] `types.go`: Removed duplicate header assignments
- [ ] `types.go`: Added comprehensive comments
- [ ] `proxy_host_handler.go`: Added handling for `enable_standard_headers` field in API

### Frontend Code Changes

- [ ] `proxyHosts.ts`: Added `enable_standard_headers?: boolean` to ProxyHost interface
- [ ] `ProxyHostForm.tsx`: Added checkbox for "Enable Standard Proxy Headers"
- [ ] `ProxyHostForm.tsx`: Added info banner when feature disabled on existing host
- [ ] `ProxyHostForm.tsx`: Set default `enable_standard_headers: true` for new hosts
- [ ] `ProxyHosts.tsx`: Added `enable_standard_headers` to bulkApplySettings state
- [ ] `ProxyHosts.tsx`: Added UI control in Bulk Apply modal
- [ ] `proxyHostsHelpers.ts`: Added label and help text for new setting
- [ ] `createMockProxyHost.ts`: Updated mock to include `enable_standard_headers: true`

### Backend Test Changes

- [ ] Renamed test to `TestReverseProxyHandler_StandardProxyHeadersAlwaysSet`
- [ ] Updated test to expect 4 headers (NOT 5, X-Forwarded-For excluded)
- [ ] Updated `TestReverseProxyHandler_WebSocketHeaders` to verify 6 headers
- [ ] Added `TestReverseProxyHandler_FeatureFlagDisabled`
- [ ] Added `TestReverseProxyHandler_XForwardedForNotDuplicated`
- [ ] Added `TestReverseProxyHandler_TrustedProxiesConfiguration`
- [ ] Added `TestReverseProxyHandler_ApplicationHeadersDoNotDuplicate`
- [ ] Added `TestReverseProxyHandler_WebSocketWithApplication`
- [ ] Added `TestReverseProxyHandler_AdvancedConfigOverridesTrustedProxies`

### Frontend Test Changes

- [ ] `ProxyHostForm.test.tsx`: Added test for checkbox rendering (new host)
- [ ] `ProxyHostForm.test.tsx`: Added test for unchecked state (legacy host)
- [ ] `ProxyHostForm.test.tsx`: Added test for info banner visibility
- [ ] `ProxyHosts-bulk-apply-all-settings.test.tsx`: Added test for bulk apply inclusion

### Backend Testing

- [ ] All unit tests pass (8 ReverseProxyHandler tests)
- [ ] Test coverage ≥85%
- [ ] Migration applies successfully
- [ ] Manual test: New generic proxy shows 4 explicit headers + X-Forwarded-For from Caddy
- [ ] Manual test: Existing host preserves old behavior (no headers)
- [ ] Manual test: Existing host can opt-in via API
- [ ] Manual test: WebSocket proxy shows 6 headers
- [ ] Manual test: X-Forwarded-For not duplicated
- [ ] Manual test: Trusted proxies configuration present
- [ ] Manual test: CrowdSec integration still works

### Frontend Testing

- [ ] All frontend unit tests pass
- [ ] Manual test: New host form shows checkbox checked by default
- [ ] Manual test: Existing host edit shows checkbox unchecked (if legacy)
- [ ] Manual test: Info banner appears for legacy hosts
- [ ] Manual test: Bulk apply includes "Standard Proxy Headers" option
- [ ] Manual test: Bulk apply updates multiple hosts correctly
- [ ] Manual test: API payload includes `enable_standard_headers` field

### Integration Testing

- [ ] Create new proxy host via UI → Verify headers in backend request
- [ ] Edit existing host, enable checkbox → Verify backend adds headers
- [ ] Bulk update 5+ hosts → Verify all configurations updated
- [ ] Verify no console errors or React warnings

### Documentation

- [ ] `CHANGELOG.md` updated with breaking change note + opt-in instructions
- [ ] `docs/API.md` updated with `EnableStandardHeaders` field documentation
- [ ] `docs/API.md` updated with proxy header information
- [ ] `README.md` or `docs/UPGRADE.md` with migration guide for users
- [ ] Code comments explain X-Forwarded-For exclusion rationale
- [ ] Code comments explain feature flag logic
- [ ] Code comments explain trusted_proxies security requirement
- [ ] Tooltip help text clear and user-friendly

### Review

- [ ] Changes reviewed by at least one developer
- [ ] Security implications reviewed (trusted_proxies requirement)
- [ ] Performance impact assessed
- [ ] Backward compatibility verified
- [ ] Migration strategy validated
- [ ] UI/UX reviewed for clarity and usability

---

## Performance & Security

### Performance Impact

- **Memory:** ~160 bytes per request (4 headers × 40 bytes avg, negligible)
- **CPU:** ~1-10 microseconds per request (feature flag check + 4 string copies, negligible)
- **Network:** ~120 bytes per request (4 headers × 30 bytes avg, 0.0012% increase)

**Note:** Original estimate of "10 nanoseconds" was incorrect. String operations and map allocations are in the microsecond range, not nanosecond. However, this is still negligible for web requests.

**Conclusion:** Negligible impact, acceptable for the security and functionality benefits.

### Security Impact

**Improvements:**

1. ✅ Better IP-based rate limiting (X-Real-IP available)
2. ✅ More accurate security logs (client IP not proxy IP)
3. ✅ IP-based ACLs work correctly
4. ✅ DDoS mitigation improved (real client IP for CrowdSec)
5. ✅ Trusted proxies configuration prevents IP spoofing

**Risks Mitigated:**

1. ✅ IP spoofing attack prevented by `trusted_proxies` configuration
2. ✅ X-Forwarded-For duplication prevented (security logs accuracy)
3. ✅ Backward compatibility prevents unintended behavior changes

**Security Review Required:**

- Verify `trusted_proxies` configuration is correct for deployment environment
- Verify CrowdSec can still read client IP correctly
- Test IP-based ACL rules still work

**Conclusion:** Security posture SIGNIFICANTLY IMPROVED with no new vulnerabilities introduced.

---

## Header Reference

| Header | Purpose | Format | Set By | Use Case |
|--------|---------|--------|--------|----------|
| X-Real-IP | Immediate client IP | `127.0.0.1` | **Us (explicit)** | Client IP detection |
| X-Forwarded-For | Full proxy chain | `client, proxy1, proxy2` | **Caddy (native)** | Multi-proxy support |
| X-Forwarded-Proto | Original protocol | `http` or `https` | **Us (explicit)** | HTTPS enforcement |
| X-Forwarded-Host | Original host | `example.com` | **Us (explicit)** | URL generation |
| X-Forwarded-Port | Original port | `80`, `443`, etc. | **Us (explicit)** | Port handling |

**Key Insight:** We explicitly set 4 headers. Caddy handles X-Forwarded-For natively to prevent duplication.

---

## CHANGELOG Entry

```markdown
## [vX.Y.Z] - 2025-12-19

### Added
- **BREAKING CHANGE:** Standard proxy headers now added to ALL reverse proxy configurations (opt-in via feature flag)
  - New field: `enable_standard_headers` (boolean) on ProxyHost model
  - When enabled, adds 4 explicit headers: `X-Real-IP`, `X-Forwarded-Proto`, `X-Forwarded-Host`, `X-Forwarded-Port`
  - `X-Forwarded-For` handled natively by Caddy (not explicitly set)
  - **Default for NEW hosts:** `true` (standard headers enabled)
  - **Default for EXISTING hosts:** `false` (backward compatibility via migration)
  - Trusted proxies configuration (`private_ranges`) always added for security

### Changed
- Proxy headers now set BEFORE WebSocket/application logic (layered approach)
- WebSocket headers no longer duplicate proxy headers
- Application-specific headers (Plex, Jellyfin) no longer duplicate standard headers

### Migration
- Existing proxy hosts automatically set `enable_standard_headers=false` to preserve old behavior
- To enable for existing hosts: `PATCH /api/proxy-hosts/:id` with `{"enable_standard_headers": true}`
- To disable for new hosts: `POST /api/proxy-hosts` with `{"enable_standard_headers": false}`

### Security
- Added `trusted_proxies` configuration to prevent IP spoofing attacks
- Improved IP-based rate limiting and ACL functionality
- More accurate security logs (client IP instead of proxy IP)

### Fixed
- Generic proxy hosts now receive proper client IP information
- Applications without WebSocket support now get proxy awareness headers
- X-Forwarded-For duplication prevented (Caddy native handling)
```

---

## Timeline

**Total Estimated Time:** 8-10 hours (revised to include frontend work)

### Breakdown

**Phase 1: Database & Model Changes (1 hour)**

- Add `EnableStandardHeaders` field to ProxyHost model (backend)
- Create database migration with backward compatibility logic
- Test migration on dev database

**Phase 2: Backend Core Implementation (2 hours)**

- Modify `types.go` ReverseProxyHandler logic
- Add feature flag checks
- Implement 4 explicit headers + trusted_proxies
- Remove duplicate header logic
- Add comprehensive comments
- Update API handler to accept `enable_standard_headers` field

**Phase 3: Backend Test Implementation (1.5 hours)**

- Rename and update existing tests
- Create 5 new tests (feature flag, X-Forwarded-For, trusted_proxies, advanced_config, combined)
- Run full test suite
- Verify coverage ≥85%

**Phase 4: Frontend Implementation (2 hours)**

- Update TypeScript interface in `proxyHosts.ts`
- Add checkbox to `ProxyHostForm.tsx`
- Add info banner for legacy hosts
- Integrate with Bulk Apply modal in `ProxyHosts.tsx`
- Update helper functions in `proxyHostsHelpers.ts`
- Update mock data for tests

**Phase 5: Frontend Test Implementation (1 hour)**

- Add unit tests for ProxyHostForm checkbox
- Add unit tests for Bulk Apply integration
- Run frontend test suite
- Fix any console warnings

**Phase 6: Integration & Manual Testing (1.5 hours)**

- Test backend: New proxy host (feature enabled)
- Test backend: Existing proxy host (feature disabled)
- Test backend: Opt-in for existing host
- Test backend: Verify X-Forwarded-For not duplicated
- Test backend: Verify CrowdSec integration still works
- Test frontend: Create new host via UI
- Test frontend: Edit existing host via UI
- Test frontend: Bulk apply to multiple hosts
- Test full stack: Verify headers in backend requests

**Phase 7: Documentation & Review (1 hour)**

- Update CHANGELOG.md
- Update docs/API.md with field documentation
- Add migration guide to README.md or docs/UPGRADE.md
- Code review (backend + frontend)
- Final verification

### Schedule

- **Day 1 (4 hours):** Phase 1 + Phase 2 + Phase 3 (Backend complete)
- **Day 2 (3 hours):** Phase 4 + Phase 5 (Frontend complete)
- **Day 3 (2-3 hours):** Phase 6 + Phase 7 (Testing, docs, review)
- **Day 4 (1 hour):** Final QA, merge, deploy

**Total:** 8-10 hours spread over 4 days (allows for context switching and review cycles)

---

## Risk Assessment

### High Risk

- ❌ None identified (backward compatibility via feature flag mitigates breaking change risk)

### Medium Risk

1. **Migration Failure**
   - Mitigation: Test migration on dev database first
   - Rollback: Migration includes rollback function

2. **CrowdSec Integration Break**
   - Mitigation: Explicit manual test step
   - Rollback: Set `enable_standard_headers=false` for affected hosts

### Low Risk

1. **Performance Degradation**
   - Mitigation: Negligible CPU/memory impact (1-10 microseconds)
   - Monitoring: Watch response time metrics after deploy

2. **Advanced Config Conflicts**
   - Mitigation: Test case for advanced_config override
   - Documentation: Document precedence rules

---

## Success Criteria

1. ✅ All 8 unit tests pass
2. ✅ Test coverage ≥85%
3. ✅ Migration applies successfully on dev/staging
4. ✅ New hosts get 4 explicit headers + X-Forwarded-For from Caddy (5 total)
5. ✅ Existing hosts preserve old behavior (no headers unless WebSocket)
6. ✅ Users can opt-in existing hosts via API
7. ✅ X-Forwarded-For not duplicated in any scenario
8. ✅ Trusted proxies configuration present in all cases
9. ✅ CrowdSec integration continues working
10. ✅ No performance degradation (response time <5ms increase)

---

## Frontend Implementation Summary

### Critical User Question Answered

**Q:** "If existing hosts have this disabled by default, how do users opt-in to the new behavior?"

**A:** Three methods provided:

1. **Per-Host Opt-In (Edit Form):**
   - User edits existing proxy host
   - Sees "Enable Standard Proxy Headers" checkbox (unchecked for legacy hosts)
   - Info banner explains the legacy behavior
   - User checks box → saves → headers enabled

2. **Bulk Opt-In (Recommended for Migration):**
   - User selects multiple proxy hosts
   - Clicks "Bulk Apply" → opens modal
   - Checks "Standard Proxy Headers" setting
   - Toggles switch to ON → clicks Apply
   - All selected hosts updated at once

3. **Automatic for New Hosts:**
   - New proxy hosts have checkbox checked by default
   - No action needed from user
   - Consistent with best practices

### Key Design Decisions

1. **No new top-level button:** Integrated into existing Bulk Apply modal (cleaner UI)
2. **Consistent with existing patterns:** Uses same checkbox/switch pattern as other settings
3. **Clear help text:** Tooltip explains what headers do and why they're needed
4. **Visual feedback:** Yellow info banner for legacy hosts (non-intrusive warning)
5. **Safe defaults:** Enabled for new hosts, disabled for existing (backward compatibility)

### Files Modified (5 Frontend Files)

| File | Changes | Lines Changed |
|------|---------|---------------|
| `api/proxyHosts.ts` | Added field to interface | ~2 lines |
| `ProxyHostForm.tsx` | Added checkbox + banner | ~40 lines |
| `ProxyHosts.tsx` | Added to bulk apply state/modal | ~15 lines |
| `proxyHostsHelpers.ts` | Added label/help text | ~5 lines |
| `testUtils/createMockProxyHost.ts` | Updated mock | ~1 line |

**Total:** ~63 lines of frontend code + ~50 lines of tests = ~113 lines

### User Experience Flow

```
┌─────────────────────────────────────────────────────┐
│  User Has 20 Existing Proxy Hosts (Legacy)         │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  Option 1: Edit Each Host Individually             │
│  - Tedious for many hosts                           │
│  - Clear per-host control                           │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  Option 2: Bulk Apply (RECOMMENDED)                │
│  1. Select all 20 hosts                             │
│  2. Click "Bulk Apply"                              │
│  3. Check "Standard Proxy Headers"                  │
│  4. Toggle ON → Apply                               │
│  Result: All 20 hosts updated in ~5 seconds         │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  New Hosts Created After Update:                   │
│  - Checkbox checked by default                      │
│  - Headers enabled automatically                    │
│  - No user action needed                            │
└─────────────────────────────────────────────────────┘
```

### Testing Coverage

**Frontend Unit Tests:** 4 new tests

- Checkbox renders checked for new hosts
- Checkbox renders unchecked for legacy hosts
- Info banner appears for legacy hosts
- Bulk apply includes new setting

**Integration Tests:** 3 scenarios

- Create new host → Verify API payload
- Edit existing host → Verify API payload
- Bulk apply → Verify multiple updates

### Accessibility & I18N Notes

**Accessibility:**

- ✅ Checkbox has proper label association
- ✅ Tooltip accessible via keyboard (CircleHelp icon)
- ✅ Info banner uses semantic colors (yellow for warning)

**Internationalization:**

- ⚠️ **TODO:** Add translation keys to i18n files
  - `proxyHosts.enableStandardHeaders` → "Enable Standard Proxy Headers"
  - `proxyHosts.standardHeadersHelp` → "Adds X-Real-IP and X-Forwarded-* headers..."
  - `proxyHosts.legacyHeadersBanner` → "Standard Proxy Headers Disabled..."

**Note:** Current implementation uses English strings. If i18n is required, add translation keys in Phase 4.

---
