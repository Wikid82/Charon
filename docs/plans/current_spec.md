# Security Header Presets: Mobile App Compatibility Analysis

**Created:** 2025-12-19
**Status:** Research Complete - Implementation Ready
**Priority:** HIGH - User-impacting UX issue

---

## Executive Summary

Users report that **Strict** and **Paranoid** security header presets break mobile app connectivity (Radarr, Sonarr, Plex, Jellyfin, Home Assistant, etc.) while **Basic** (65/100 score) works. This document analyzes the root cause and proposes a solution to provide enterprise-grade security that remains compatible with mobile/API access.

---

## 1. Root Cause Analysis

### 1.1 Current Preset Definitions

**Source:** [backend/internal/services/security_headers_service.go](../../backend/internal/services/security_headers_service.go)

| Header | Basic (65/100) | Strict (85/100) | Paranoid (100/100) |
|--------|----------------|-----------------|---------------------|
| **HSTS** | ✅ 1 year | ✅ 1 year + subdomains | ✅ 2 years + subdomains + preload |
| **CSP Enabled** | ❌ | ✅ Restrictive | ✅ Very Restrictive |
| **X-Frame-Options** | `SAMEORIGIN` | `DENY` | `DENY` |
| **X-Content-Type-Options** | `nosniff` | `nosniff` | `nosniff` |
| **Referrer-Policy** | `strict-origin-when-cross-origin` | `strict-origin-when-cross-origin` | `no-referrer` |
| **COOP** (Cross-Origin-Opener) | ❌ | `same-origin` | `same-origin` |
| **CORP** (Cross-Origin-Resource) | ❌ | `same-origin` | `same-origin` |
| **COEP** (Cross-Origin-Embedder) | ❌ | ❌ | `require-corp` |
| **Permissions-Policy** | ❌ | ✅ Blocks camera/mic/geo | ✅ + Blocks payment/usb |
| **Cache-Control: no-store** | ❌ | ❌ | ✅ |
| **CSP `connect-src`** | N/A (CSP disabled) | `'self'` | `'self'` |
| **CSP `frame-src`** | N/A | `'none'` | `'none'` |
| **CSP `frame-ancestors`** | N/A | Not set | `'none'` |

### 1.2 Headers Breaking Mobile Apps

#### ❌ **Content-Security-Policy (CSP)** - MAJOR ISSUE

**Current Strict CSP:**
```json
{
  "default-src": ["'self'"],
  "script-src": ["'self'"],
  "style-src": ["'self'", "'unsafe-inline'"],
  "img-src": ["'self'", "data:", "https:"],
  "font-src": ["'self'", "data:"],
  "connect-src": ["'self'"],
  "frame-src": ["'none'"],
  "object-src": ["'none'"]
}
```

**Problem:** Mobile apps (native iOS/Android) make API calls that:
- **`connect-src: 'self'`** - Blocks API connections from mobile apps that aren't the "same origin"
- Mobile apps don't have an "origin" in the HTTP sense - they send requests with no Origin header or with app-specific origins
- WebView-based apps may be blocked by frame restrictions

**Paranoid CSP is even worse:**
```json
{
  "default-src": ["'none'"],
  "frame-ancestors": ["'none'"]
}
```

**`frame-ancestors: 'none'`** - Completely prevents embedding, breaking any app that uses WebViews.

#### ❌ **Cross-Origin-Resource-Policy (CORP): `same-origin`** - MAJOR ISSUE

**Problem:** When set to `same-origin`, the browser (and WebViews) refuses to load resources from cross-origin contexts.

- Mobile apps accessing API endpoints are considered "cross-origin"
- Images, fonts, API responses are blocked
- Apps like Radarr mobile, nzb360, LunaSea rely on cross-origin API access

#### ❌ **Cross-Origin-Opener-Policy (COOP): `same-origin`** - MODERATE ISSUE

**Problem:** Isolates the browsing context, breaking:
- OAuth flows that use popup windows
- Cross-window communication (e.g., Plex authentication)
- Some mobile apps that open authentication flows in external browsers

#### ❌ **Cross-Origin-Embedder-Policy (COEP): `require-corp`** - MAJOR ISSUE (Paranoid only)

**Problem:** Requires all resources to opt-in via CORP headers. When the backend service (Radarr, Plex, etc.) doesn't set CORP headers, the response is blocked entirely.

- Third-party APIs (TMDb, TheTVDB) don't set CORP headers
- Poster images, metadata requests are blocked
- Breaks entire app functionality

#### ⚠️ **X-Frame-Options: DENY** - MODERATE ISSUE

**Problem:** Prevents any framing. Some mobile apps use:
- WebView containers that technically frame the content
- Embedded players (Plex, Jellyfin)
- In-app browsers

**`SAMEORIGIN`** (Basic preset) allows same-domain framing, which works better.

#### ⚠️ **Permissions-Policy** - MINOR ISSUE

**Problem:** Blocking camera/microphone can break:
- Video calling apps (Home Assistant video doorbell)
- Media apps that use device features

---

## 2. Mobile App Requirements Research

### 2.1 Common Home Server Mobile Apps

| App | Platform | Access Pattern | Requirements |
|-----|----------|----------------|--------------|
| **Radarr/Sonarr** | iOS/Android (nzb360, LunaSea) | Native API calls | CORS-like access, no restrictive CSP |
| **Plex** | iOS/Android/Web | API + Streaming | WebSocket, flexible framing |
| **Jellyfin** | iOS/Android | API + Streaming | WebSocket, cross-origin access |
| **Home Assistant** | iOS/Android/Web | API + WebSocket | Companion app needs full API access |
| **Vaultwarden** | iOS/Android (Bitwarden) | API only | Must allow mobile app API calls |
| **Nextcloud** | iOS/Android | WebDAV + API | File sync needs unrestricted API |

### 2.2 What Mobile Apps Need

1. **No restrictive CSP on API endpoints** - Mobile apps don't execute JavaScript, CSP is irrelevant
2. **`Cross-Origin-Resource-Policy: cross-origin`** or not set at all - Allow resource loading
3. **No `Cross-Origin-Embedder-Policy`** - Breaks third-party resource loading
4. **`X-Frame-Options: SAMEORIGIN`** or not set - Allow in-app WebViews
5. **`Cross-Origin-Opener-Policy: unsafe-none`** or not set - Allow OAuth/popups

### 2.3 Why "Basic" Works

The **Basic** preset works because:
- CSP is **disabled** - No blocking of API calls
- CORP is **not set** - Default allows cross-origin
- COEP is **not set** - No resource isolation
- X-Frame-Options is **SAMEORIGIN** - WebViews work
- COOP is **not set** - OAuth flows work

---

## 3. Solution Design

### 3.1 Recommendation: Create "API-Friendly" Preset

**Why not just modify existing presets?**
- Basic is intentionally minimal - users expect low security
- Strict/Paranoid are intentionally restrictive - changing them defeats their purpose
- A new preset clearly communicates "use this for mobile/API access"

### 3.2 Proposed "API-Friendly" Preset Definition

**Design Goals:**
- Security score ~70-75/100 (between Basic and Strict)
- Maximum compatibility with mobile apps and API clients
- Strong transport security (HSTS)
- Sensible protections without breaking functionality

```go
{
    UUID:                      "preset-api-friendly",
    Name:                      "API-Friendly",
    PresetType:                "api-friendly",
    IsPreset:                  true,
    Description:               "Optimized for mobile apps and API access (Radarr, Plex, Home Assistant). Strong transport security without breaking API compatibility.",

    // Transport Security - STRONG
    HSTSEnabled:               true,
    HSTSMaxAge:                31536000, // 1 year
    HSTSIncludeSubdomains:     false,    // Don't break subdomains
    HSTSPreload:               false,

    // Content Security - DISABLED for API compatibility
    CSPEnabled:                false,    // APIs don't need CSP

    // Framing - PERMISSIVE for WebViews
    XFrameOptions:             "",       // Not set - allow framing (mobile WebViews)

    // MIME Sniffing - ENABLED (safe)
    XContentTypeOptions:       true,     // nosniff is safe

    // Referrer - BALANCED
    ReferrerPolicy:            "strict-origin-when-cross-origin", // Safe default

    // Permissions - NOT SET (allow all)
    PermissionsPolicy:         "",

    // Cross-Origin - PERMISSIVE
    CrossOriginOpenerPolicy:   "",       // Not set - allow OAuth popups
    CrossOriginResourcePolicy: "cross-origin", // Explicitly allow cross-origin
    CrossOriginEmbedderPolicy: "",       // Not set - don't require CORP

    // Legacy XSS - ENABLED
    XSSProtection:             true,

    // Caching - DEFAULT
    CacheControlNoStore:       false,

    SecurityScore:             70,
}
```

### 3.3 Preset Comparison After Change

| Header | Basic (65) | **API-Friendly (70)** | Strict (85) | Paranoid (100) |
|--------|------------|----------------------|-------------|----------------|
| HSTS | ✅ 1yr | ✅ 1yr | ✅ 1yr+sub | ✅ 2yr+sub+preload |
| CSP | ❌ | ❌ | ✅ Restrictive | ✅ Very Restrictive |
| X-Frame-Options | SAMEORIGIN | **Not Set** | DENY | DENY |
| CORP | Not Set | **cross-origin** | same-origin | same-origin |
| COEP | Not Set | **Not Set** | Not Set | require-corp |
| COOP | Not Set | **Not Set** | same-origin | same-origin |
| Permissions-Policy | Not Set | **Not Set** | Restrictive | Very Restrictive |

---

## 4. Tooltip Text Design

### 4.1 Tooltip Content for Each Preset

#### Basic Security (65/100)
```
Minimal security headers for maximum compatibility.

✓ Best for: Testing, development, simple websites
✓ Enables: HSTS, X-Content-Type-Options, basic XSS protection
⚠ Note: Does not include CSP or cross-origin restrictions

Compatible with: All applications and mobile apps
```

#### API-Friendly (70/100) - NEW
```
Optimized for mobile apps, API clients, and media servers.

✓ Best for: Radarr, Sonarr, Plex, Jellyfin, Home Assistant, Vaultwarden
✓ Enables: Strong transport security (HSTS), MIME protection
✓ Allows: Cross-origin API access, WebView embedding, OAuth flows

Recommended for services accessed by mobile apps (nzb360, LunaSea,
Infuse, official companion apps).

⚠ Note: Less restrictive than Strict - prioritizes compatibility
```

#### Strict Security (85/100)
```
Strong security for web applications handling sensitive data.

✓ Best for: Web-only applications, admin panels, dashboards
✓ Enables: Full CSP, cross-origin isolation, frame blocking
✓ Blocks: Inline scripts, cross-origin embedding, external frames

⚠ Warning: May break mobile apps and API clients
⚠ Not recommended for: Radarr, Sonarr, Plex, Jellyfin, or services
  accessed by mobile companion apps

Test thoroughly before using in production.
```

#### Paranoid Security (100/100)
```
Maximum security for high-risk applications. Use with caution.

✓ Best for: Banking, healthcare, or compliance-critical applications
✓ Enables: Strictest CSP, COEP isolation, HSTS preload, no-referrer
✓ Blocks: All cross-origin access, all embedding, all external resources

⚠ WILL BREAK: Mobile apps, API clients, OAuth flows, media streaming
⚠ WILL BREAK: Third-party integrations, CDN resources, analytics
⚠ Requires extensive testing and manual CSP adjustments

Only use if you understand every header and can customize exceptions.
```

---

## 5. Implementation Plan

### 5.1 Files to Modify

#### Backend Changes

1. **[backend/internal/services/security_headers_service.go](../../backend/internal/services/security_headers_service.go)**
   - Add new "API-Friendly" preset to `GetPresets()` function
   - Position it between Basic (65) and Strict (85) in score order

2. **[backend/internal/models/security_header_profile.go](../../backend/internal/models/security_header_profile.go)**
   - No changes needed (model supports all fields)

3. **[backend/internal/caddy/config.go](../../backend/internal/caddy/config.go)**
   - No changes needed (`buildSecurityHeadersHandler` handles all fields)

#### Frontend Changes

1. **[frontend/src/pages/SecurityHeaders.tsx](../../frontend/src/pages/SecurityHeaders.tsx)**
   - Add tooltip component for preset cards
   - Display description with warnings for Strict/Paranoid

2. **[frontend/src/components/ProxyHostForm.tsx](../../frontend/src/components/ProxyHostForm.tsx)**
   - Add tooltip to Security Headers dropdown
   - Show compatibility warnings when Strict/Paranoid selected

3. **[frontend/src/api/securityHeaders.ts](../../frontend/src/api/securityHeaders.ts)**
   - No changes needed (types already support preset_type)

4. **[frontend/src/types/securityHeaders.ts](../../frontend/src/types/securityHeaders.ts)** (if exists)
   - Add `api-friendly` to PresetType union type

### 5.2 Backend Implementation

**File:** `backend/internal/services/security_headers_service.go`

Add after Basic preset and before Strict preset in `GetPresets()`:

```go
{
    UUID:                      "preset-api-friendly",
    Name:                      "API-Friendly",
    PresetType:                "api-friendly",
    IsPreset:                  true,
    Description:               "Optimized for mobile apps and API access (Radarr, Plex, Home Assistant). Strong transport security without breaking API compatibility.",
    HSTSEnabled:               true,
    HSTSMaxAge:                31536000,
    HSTSIncludeSubdomains:     false,
    HSTSPreload:               false,
    CSPEnabled:                false,
    XFrameOptions:             "",
    XContentTypeOptions:       true,
    ReferrerPolicy:            "strict-origin-when-cross-origin",
    PermissionsPolicy:         "",
    CrossOriginOpenerPolicy:   "",
    CrossOriginResourcePolicy: "cross-origin",
    CrossOriginEmbedderPolicy: "",
    XSSProtection:             true,
    CacheControlNoStore:       false,
    SecurityScore:             70,
},
```

### 5.3 Frontend Implementation

**File:** `frontend/src/pages/SecurityHeaders.tsx`

Add tooltip component to preset cards:

```tsx
// Add import
import { Tooltip, TooltipContent, TooltipTrigger } from '../components/ui/Tooltip';
import { Info } from 'lucide-react';

// In the preset card rendering:
<Card key={profile.id} className="p-4">
  <div className="flex items-start justify-between mb-3">
    <div className="flex-1 flex items-center gap-2">
      <h3 className="font-semibold text-gray-900 dark:text-white">
        {profile.name}
      </h3>
      <Tooltip>
        <TooltipTrigger>
          <Info className="w-4 h-4 text-gray-400 hover:text-gray-600" />
        </TooltipTrigger>
        <TooltipContent className="max-w-sm">
          {getPresetTooltip(profile.preset_type)}
        </TooltipContent>
      </Tooltip>
    </div>
    {/* ... rest of card */}
  </div>
</Card>

// Add helper function:
function getPresetTooltip(presetType: string | undefined): React.ReactNode {
  switch (presetType) {
    case 'basic':
      return (
        <div className="space-y-1">
          <p className="font-medium">Minimal security headers</p>
          <p className="text-xs">✓ Best for: Testing, development</p>
          <p className="text-xs">✓ Compatible with all apps</p>
        </div>
      );
    case 'api-friendly':
      return (
        <div className="space-y-1">
          <p className="font-medium">Optimized for mobile apps & APIs</p>
          <p className="text-xs text-green-400">✓ Works with: Radarr, Plex, Jellyfin, Home Assistant</p>
          <p className="text-xs">✓ Strong HTTPS, allows cross-origin</p>
        </div>
      );
    case 'strict':
      return (
        <div className="space-y-1">
          <p className="font-medium">Strong web application security</p>
          <p className="text-xs text-yellow-400">⚠ May break mobile apps</p>
          <p className="text-xs">✓ Best for: Web-only dashboards</p>
        </div>
      );
    case 'paranoid':
      return (
        <div className="space-y-1">
          <p className="font-medium">Maximum security</p>
          <p className="text-xs text-red-400">⚠ WILL break mobile apps</p>
          <p className="text-xs text-red-400">⚠ WILL break API clients</p>
          <p className="text-xs">Only for high-risk applications</p>
        </div>
      );
    default:
      return null;
  }
}
```

**File:** `frontend/src/components/ProxyHostForm.tsx`

Add warning when Strict/Paranoid is selected:

```tsx
{/* After the select dropdown, add warning */}
{formData.security_header_profile_id && (() => {
  const selected = securityProfiles?.find(p => p.id === formData.security_header_profile_id);
  if (!selected) return null;

  const isRestrictive = selected.preset_type === 'strict' || selected.preset_type === 'paranoid';

  return (
    <div className="mt-2 space-y-1">
      <div className="flex items-center gap-2">
        <SecurityScoreDisplay score={selected.security_score} size="sm" showDetails={false} />
        <span className="text-xs text-gray-400">{selected.description}</span>
      </div>

      {isRestrictive && (
        <div className="flex items-start gap-2 mt-2 p-2 bg-yellow-900/20 border border-yellow-700 rounded text-xs">
          <AlertCircle className="w-4 h-4 text-yellow-500 flex-shrink-0 mt-0.5" />
          <div className="text-yellow-400">
            <p className="font-medium">Mobile App Warning</p>
            <p>This profile may break mobile apps like Radarr, Plex, or Home Assistant companion apps. Consider using "API-Friendly" or "Basic" for services accessed by mobile clients.</p>
          </div>
        </div>
      )}
    </div>
  );
})()}
```

---

## 6. Test Scenarios

### 6.1 Unit Tests

**File:** `backend/internal/services/security_headers_service_test.go`

```go
func TestGetPresets_IncludesAPIFriendly(t *testing.T) {
    service := NewSecurityHeadersService(nil) // nil DB ok for GetPresets
    presets := service.GetPresets()

    var apiFriendly *models.SecurityHeaderProfile
    for _, p := range presets {
        if p.PresetType == "api-friendly" {
            apiFriendly = &p
            break
        }
    }

    require.NotNil(t, apiFriendly, "API-Friendly preset should exist")
    assert.Equal(t, "API-Friendly", apiFriendly.Name)
    assert.Equal(t, 70, apiFriendly.SecurityScore)
    assert.True(t, apiFriendly.HSTSEnabled)
    assert.False(t, apiFriendly.CSPEnabled)
    assert.Equal(t, "", apiFriendly.XFrameOptions)
    assert.Equal(t, "cross-origin", apiFriendly.CrossOriginResourcePolicy)
}

func TestGetPresets_OrderByScore(t *testing.T) {
    service := NewSecurityHeadersService(nil)
    presets := service.GetPresets()

    // Verify order: Basic (65) < API-Friendly (70) < Strict (85) < Paranoid (100)
    var scores []int
    for _, p := range presets {
        scores = append(scores, p.SecurityScore)
    }

    assert.Equal(t, []int{65, 70, 85, 100}, scores)
}
```

### 6.2 Integration Tests

**File:** `backend/internal/caddy/config_security_headers_test.go`

```go
func TestBuildSecurityHeadersHandler_APIFriendlyPreset(t *testing.T) {
    host := &models.ProxyHost{
        SecurityHeaderProfile: &models.SecurityHeaderProfile{
            HSTSEnabled:               true,
            HSTSMaxAge:                31536000,
            CSPEnabled:                false,
            XFrameOptions:             "",
            XContentTypeOptions:       true,
            CrossOriginResourcePolicy: "cross-origin",
        },
    }

    handler, err := buildSecurityHeadersHandler(host)
    require.NoError(t, err)
    require.NotNil(t, handler)

    response := handler["response"].(map[string]interface{})
    headers := response["set"].(map[string][]string)

    // Should have HSTS
    assert.Contains(t, headers["Strict-Transport-Security"][0], "max-age=31536000")

    // Should NOT have X-Frame-Options (empty = not set)
    _, hasXFO := headers["X-Frame-Options"]
    assert.False(t, hasXFO)

    // Should have CORP = cross-origin
    assert.Equal(t, []string{"cross-origin"}, headers["Cross-Origin-Resource-Policy"])

    // Should NOT have CSP (disabled)
    _, hasCSP := headers["Content-Security-Policy"]
    assert.False(t, hasCSP)
}
```

### 6.3 Manual Testing Scenarios

| Test Case | Steps | Expected Result |
|-----------|-------|-----------------|
| **TC-01: Radarr Mobile** | 1. Apply API-Friendly to Radarr proxy<br>2. Open nzb360/LunaSea<br>3. Test browse, search, add | All functions work |
| **TC-02: Plex Mobile** | 1. Apply API-Friendly to Plex proxy<br>2. Open Plex iOS/Android app<br>3. Test stream, sync | Streaming works |
| **TC-03: Home Assistant** | 1. Apply API-Friendly to HA proxy<br>2. Open HA Companion app<br>3. Test controls, notifications | Real-time updates work |
| **TC-04: Strict Breaks Mobile** | 1. Apply Strict to Radarr proxy<br>2. Open nzb360<br>3. Test API calls | Should fail/error |
| **TC-05: Tooltip Display** | 1. Go to Security Headers page<br>2. Hover over API-Friendly preset | Tooltip shows compatibility info |
| **TC-06: Warning Display** | 1. Edit proxy host<br>2. Select Strict profile | Warning about mobile apps appears |

---

## 7. Migration Considerations

### 7.1 Existing Users

- **No breaking changes** - Existing presets unchanged
- New preset appears automatically after backend update
- Users on Basic who want mobile compatibility can stay on Basic or upgrade to API-Friendly (slightly higher score)

### 7.2 Database Migration

- No schema changes needed
- `EnsurePresetsExist()` handles creating/updating presets
- Existing user profiles are unaffected

---

## 8. Future Enhancements

### 8.1 Per-Endpoint Security Headers (Phase 2)

Allow different security profiles for:
- `/api/*` endpoints - Relaxed for API access
- `/admin/*` endpoints - Strict for admin panels
- `/*` default - Based on user selection

### 8.2 Automatic Detection (Phase 3)

Detect application type from:
- Application preset (Plex, Radarr, etc.)
- Auto-suggest API-Friendly for known mobile-app services

---

## 9. Summary

### Problem
Strict/Paranoid security header presets break mobile apps due to restrictive CSP, CORP, COOP, COEP, and X-Frame-Options headers.

### Solution
Create a new "API-Friendly" preset that:
- Maintains strong transport security (HSTS)
- Disables CSP (unnecessary for APIs)
- Explicitly allows cross-origin resource access (CORP: cross-origin)
- Removes frame restrictions for WebView compatibility
- Achieves 70/100 security score (between Basic and Strict)

### Implementation
1. Add preset to backend service (~5 lines of code)
2. Add tooltips to frontend (~50 lines of code)
3. Add mobile warning to ProxyHostForm (~20 lines of code)
4. Add unit tests (~30 lines of code)

### Success Criteria
- [ ] API-Friendly preset appears in UI
- [ ] Radarr/Sonarr mobile apps work with API-Friendly
- [ ] Plex/Jellyfin mobile apps work with API-Friendly
- [ ] Tooltips display on all presets
- [ ] Warning displays when Strict/Paranoid selected
- [ ] All existing functionality unchanged

---

**Document Status:** Ready for Implementation
**Estimated Effort:** 2-3 hours
**Risk Level:** Low (additive change, no breaking modifications)
