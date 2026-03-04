# Implementation Plan: Add HTTP Headers to Bulk Apply Feature

## Overview

### Feature Description

Extend the existing **Bulk Apply** feature on the Proxy Hosts page to allow users to assign **Security Header Profiles** to multiple proxy hosts simultaneously. This enhancement enables administrators to efficiently apply consistent security header configurations across their infrastructure without editing each host individually.

### User Benefit

- **Time Savings**: Apply security header profiles to 10, 50, or 100+ hosts in a single operation
- **Consistency**: Ensure uniform security posture across all proxy hosts
- **Compliance**: Quickly remediate security gaps by bulk-applying strict security profiles
- **Workflow Efficiency**: Integrates seamlessly with existing Bulk Apply modal (Force SSL, HTTP/2, HSTS, etc.)

### Scope of Changes

| Area | Scope |
|------|-------|
| Frontend | Modify 4-5 files (ProxyHosts page, helpers, API, translations) |
| Backend | Modify 2 files (handler, possibly add new endpoint) |
| Database | No schema changes required (uses existing `security_header_profile_id` field) |
| Tests | Add unit tests for frontend and backend |

---

## A. Current Implementation Analysis

### Existing Bulk Apply Architecture

The Bulk Apply feature currently supports these boolean settings:

- `ssl_forced` - Force SSL
- `http2_support` - HTTP/2 Support
- `hsts_enabled` - HSTS Enabled
- `hsts_subdomains` - HSTS Subdomains
- `block_exploits` - Block Exploits
- `websocket_support` - Websockets Support
- `enable_standard_headers` - Standard Proxy Headers

**Key Files:**

| File | Purpose |
|------|---------|
| [frontend/src/pages/ProxyHosts.tsx](../../frontend/src/pages/ProxyHosts.tsx) | Main page with Bulk Apply modal (L60-67 defines `bulkApplySettings` state) |
| [frontend/src/utils/proxyHostsHelpers.ts](../../frontend/src/utils/proxyHostsHelpers.ts) | Helper functions: `formatSettingLabel()`, `settingHelpText()`, `settingKeyToField()`, `applyBulkSettingsToHosts()` |
| [frontend/src/api/proxyHosts.ts](../../frontend/src/api/proxyHosts.ts) | API client with `updateProxyHost()` for individual updates |
| [frontend/src/hooks/useProxyHosts.ts](../../frontend/src/hooks/useProxyHosts.ts) | React Query hook with `updateHost()` mutation |

### How Bulk Apply Currently Works

1. User selects multiple hosts using checkboxes in the DataTable
2. User clicks "Bulk Apply" button → opens modal
3. Modal shows all available settings with checkboxes (apply/don't apply) and toggles (on/off)
4. User clicks "Apply" → `applyBulkSettingsToHosts()` iterates over selected hosts
5. For each host, it calls `updateHost(uuid, mergedData)` which triggers `PUT /api/v1/proxy-hosts/{uuid}`
6. Backend updates the host and applies Caddy config

### Existing Security Header Profile Implementation

**Frontend:**

| File | Purpose |
|------|---------|
| [frontend/src/api/securityHeaders.ts](../../frontend/src/api/securityHeaders.ts) | API client for security header profiles |
| [frontend/src/hooks/useSecurityHeaders.ts](../../frontend/src/hooks/useSecurityHeaders.ts) | React Query hooks including `useSecurityHeaderProfiles()` |
| [frontend/src/components/ProxyHostForm.tsx](../../frontend/src/components/ProxyHostForm.tsx) | Individual host form with Security Header Profile dropdown (L550-620) |

**Backend:**

| File | Purpose |
|------|---------|
| [backend/internal/models/proxy_host.go](../../backend/internal/models/proxy_host.go) | `SecurityHeaderProfileID *uint` field (L38) |
| [backend/internal/api/handlers/proxy_host_handler.go](../../backend/internal/api/handlers/proxy_host_handler.go) | `Update()` handler parses `security_header_profile_id` (L253-286) |
| [backend/internal/models/security_header_profile.go](../../backend/internal/models/security_header_profile.go) | Profile model with all header configurations |

---

## B. Frontend Changes

### B.1. Modify ProxyHosts.tsx

**File:** `frontend/src/pages/ProxyHosts.tsx`

#### B.1.1. Add Security Header Profile Selection State

**Location:** After `bulkApplySettings` state definition (around L67)

```typescript
// Existing state (L60-67)
const [bulkApplySettings, setBulkApplySettings] = useState<Record<string, { apply: boolean; value: boolean }>>({
  ssl_forced: { apply: false, value: true },
  http2_support: { apply: false, value: true },
  hsts_enabled: { apply: false, value: true },
  hsts_subdomains: { apply: false, value: true },
  block_exploits: { apply: false, value: true },
  websocket_support: { apply: false, value: true },
  enable_standard_headers: { apply: false, value: true },
})

// NEW: Add security header profile selection state
const [bulkSecurityHeaderProfile, setBulkSecurityHeaderProfile] = useState<{
  apply: boolean;
  profileId: number | null;
}>({ apply: false, profileId: null })
```

#### B.1.2. Import Security Header Profiles Hook

**Location:** At top of file with other imports

```typescript
import { useSecurityHeaderProfiles } from '../hooks/useSecurityHeaders'
```

#### B.1.3. Add Hook Usage

**Location:** After other hook calls (around L43)

```typescript
const { data: securityProfiles } = useSecurityHeaderProfiles()
```

#### B.1.4. Modify Bulk Apply Modal UI

**Location:** Inside the Bulk Apply Dialog content (around L645-690)

Add a new section after the existing toggle settings but before the progress indicator:

```tsx
{/* Security Header Profile Section - NEW */}
<div className="border-t border-border pt-3 mt-3">
  <div className="flex items-center justify-between gap-3 p-3 bg-surface-subtle rounded-lg">
    <div className="flex items-center gap-3">
      <Checkbox
        checked={bulkSecurityHeaderProfile.apply}
        onCheckedChange={(checked) => setBulkSecurityHeaderProfile(prev => ({
          ...prev,
          apply: !!checked
        }))}
      />
      <div>
        <div className="text-sm font-medium text-content-primary">
          {t('proxyHosts.bulkApplySecurityHeaders')}
        </div>
        <div className="text-xs text-content-muted">
          {t('proxyHosts.bulkApplySecurityHeadersHelp')}
        </div>
      </div>
    </div>
  </div>

  {bulkSecurityHeaderProfile.apply && (
    <div className="mt-3 p-3 bg-surface-subtle rounded-lg">
      <select
        value={bulkSecurityHeaderProfile.profileId ?? 0}
        onChange={(e) => setBulkSecurityHeaderProfile(prev => ({
          ...prev,
          profileId: e.target.value === "0" ? null : parseInt(e.target.value)
        }))}
        className="w-full bg-surface-muted border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
      >
        <option value={0}>{t('proxyHosts.noSecurityProfile')}</option>
        <optgroup label={t('securityHeaders.systemProfiles')}>
          {securityProfiles
            ?.filter(p => p.is_preset)
            .sort((a, b) => a.security_score - b.security_score)
            .map(profile => (
              <option key={profile.id} value={profile.id}>
                {profile.name} ({t('common.score')}: {profile.security_score}/100)
              </option>
            ))}
        </optgroup>
        {(securityProfiles?.filter(p => !p.is_preset) || []).length > 0 && (
          <optgroup label={t('securityHeaders.customProfiles')}>
            {securityProfiles
              ?.filter(p => !p.is_preset)
              .map(profile => (
                <option key={profile.id} value={profile.id}>
                  {profile.name} ({t('common.score')}: {profile.security_score}/100)
                </option>
              ))}
          </optgroup>
        )}
      </select>

      {bulkSecurityHeaderProfile.profileId && (() => {
        const selected = securityProfiles?.find(p => p.id === bulkSecurityHeaderProfile.profileId)
        if (!selected) return null
        return (
          <div className="mt-2 text-xs text-content-muted">
            {selected.description}
          </div>
        )
      })()}
    </div>
  )}
</div>
```

#### B.1.5. Update Apply Button Logic

**Location:** In the DialogFooter onClick handler (around L700-720)

Modify the apply handler to include security header profile:

```typescript
onClick={async () => {
  const keysToApply = Object.keys(bulkApplySettings).filter(k => bulkApplySettings[k].apply)
  const hostUUIDs = Array.from(selectedHosts)

  // Apply boolean settings
  if (keysToApply.length > 0) {
    const result = await applyBulkSettingsToHosts({
      hosts,
      hostUUIDs,
      keysToApply,
      bulkApplySettings,
      updateHost,
      setApplyProgress
    })

    if (result.errors > 0) {
      toast.error(t('notifications.updateFailed'))
    }
  }

  // Apply security header profile if selected
  if (bulkSecurityHeaderProfile.apply) {
    let profileErrors = 0
    for (const uuid of hostUUIDs) {
      try {
        await updateHost(uuid, {
          security_header_profile_id: bulkSecurityHeaderProfile.profileId
        })
      } catch {
        profileErrors++
      }
    }

    if (profileErrors > 0) {
      toast.error(t('notifications.updateFailed'))
    }
  }

  // Only show success if at least something was applied
  if (keysToApply.length > 0 || bulkSecurityHeaderProfile.apply) {
    toast.success(t('notifications.updateSuccess'))
  }

  setSelectedHosts(new Set())
  setShowBulkApplyModal(false)
  setBulkSecurityHeaderProfile({ apply: false, profileId: null })
}}
```

#### B.1.6. Update Apply Button Disabled State

**Location:** Same DialogFooter Button (around L725)

```typescript
disabled={
  applyProgress !== null ||
  (Object.values(bulkApplySettings).every(s => !s.apply) && !bulkSecurityHeaderProfile.apply)
}
```

---

### B.2. Update Translation Files

**Files to Update:**

1. `frontend/src/locales/en/translation.json`
2. `frontend/src/locales/de/translation.json`
3. `frontend/src/locales/es/translation.json`
4. `frontend/src/locales/fr/translation.json`
5. `frontend/src/locales/zh/translation.json`

#### New Translation Keys (add to `proxyHosts` section)

**English (`en/translation.json`):**

```json
{
  "proxyHosts": {
    "bulkApplySecurityHeaders": "Security Header Profile",
    "bulkApplySecurityHeadersHelp": "Apply a security header profile to all selected hosts",
    "noSecurityProfile": "None (Remove Profile)"
  }
}
```

**Also add to `common` section:**

```json
{
  "common": {
    "score": "Score"
  }
}
```

---

### B.3. Optional: Optimize with Bulk API Endpoint

For better performance with large numbers of hosts, consider adding a dedicated bulk update endpoint. This would reduce N API calls to 1.

**New API Function in `frontend/src/api/proxyHosts.ts`:**

```typescript
export interface BulkUpdateSecurityHeadersRequest {
  host_uuids: string[];
  security_header_profile_id: number | null;
}

export interface BulkUpdateSecurityHeadersResponse {
  updated: number;
  errors: { uuid: string; error: string }[];
}

export const bulkUpdateSecurityHeaders = async (
  hostUUIDs: string[],
  securityHeaderProfileId: number | null
): Promise<BulkUpdateSecurityHeadersResponse> => {
  const { data } = await client.put<BulkUpdateSecurityHeadersResponse>(
    '/proxy-hosts/bulk-update-security-headers',
    {
      host_uuids: hostUUIDs,
      security_header_profile_id: securityHeaderProfileId,
    }
  );
  return data;
};
```

---

## C. Backend Changes

### C.1. Current Update Handler Analysis

The existing `Update()` handler in [proxy_host_handler.go](../../backend/internal/api/handlers/proxy_host_handler.go) already handles `security_header_profile_id` updates (L253-286). The frontend can use individual `updateHost()` calls for each selected host.

However, for optimal performance, adding a dedicated bulk endpoint is recommended.

### C.2. Add Bulk Update Security Headers Endpoint (Recommended)

**File:** `backend/internal/api/handlers/proxy_host_handler.go`

#### C.2.1. Register New Route

**Location:** In `RegisterRoutes()` function (around L62)

```go
router.PUT("/proxy-hosts/bulk-update-security-headers", h.BulkUpdateSecurityHeaders)
```

#### C.2.2. Add Handler Function

**Location:** After `BulkUpdateACL()` function (around L540)

```go
// BulkUpdateSecurityHeadersRequest represents the request body for bulk security header updates
type BulkUpdateSecurityHeadersRequest struct {
    HostUUIDs               []string `json:"host_uuids" binding:"required"`
    SecurityHeaderProfileID *uint    `json:"security_header_profile_id"` // nil means remove profile
}

// BulkUpdateSecurityHeaders applies or removes a security header profile to multiple proxy hosts.
func (h *ProxyHostHandler) BulkUpdateSecurityHeaders(c *gin.Context) {
    var req BulkUpdateSecurityHeadersRequest

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if len(req.HostUUIDs) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "host_uuids cannot be empty"})
        return
    }

    // Validate profile exists if provided
    if req.SecurityHeaderProfileID != nil {
        var profile models.SecurityHeaderProfile
        if err := h.service.DB().First(&profile, *req.SecurityHeaderProfileID).Error; err != nil {
            if err == gorm.ErrRecordNotFound {
                c.JSON(http.StatusBadRequest, gin.H{"error": "security header profile not found"})
                return
            }
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
    }

    updated := 0
    errors := []map[string]string{}

    for _, hostUUID := range req.HostUUIDs {
        host, err := h.service.GetByUUID(hostUUID)
        if err != nil {
            errors = append(errors, map[string]string{
                "uuid":  hostUUID,
                "error": "proxy host not found",
            })
            continue
        }

        host.SecurityHeaderProfileID = req.SecurityHeaderProfileID
        if err := h.service.Update(host); err != nil {
            errors = append(errors, map[string]string{
                "uuid":  hostUUID,
                "error": err.Error(),
            })
            continue
        }

        updated++
    }

    // Apply Caddy config once for all updates
    if updated > 0 && h.caddyManager != nil {
        if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error":   "Failed to apply configuration: " + err.Error(),
                "updated": updated,
                "errors":  errors,
            })
            return
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "updated": updated,
        "errors":  errors,
    })
}
```

#### C.2.3. Update ProxyHostService (if needed)

**File:** `backend/internal/services/proxyhost_service.go`

If `h.service.DB()` is not exposed, add a getter:

```go
func (s *ProxyHostService) DB() *gorm.DB {
    return s.db
}
```

---

## D. Testing Requirements

### D.1. Frontend Unit Tests

**File to Create:** `frontend/src/pages/__tests__/ProxyHosts.bulkApplyHeaders.test.tsx`

```typescript
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
// ... test setup

describe('ProxyHosts Bulk Apply Security Headers', () => {
  it('should show security header profile option in bulk apply modal', async () => {
    // Render component with selected hosts
    // Open bulk apply modal
    // Verify security header section is visible
  })

  it('should enable profile selection when checkbox is checked', async () => {
    // Check the "Security Header Profile" checkbox
    // Verify dropdown becomes visible
  })

  it('should list all available profiles in dropdown', async () => {
    // Mock security profiles data
    // Verify preset and custom profiles are grouped
  })

  it('should apply security header profile to selected hosts', async () => {
    // Select hosts
    // Open modal
    // Enable security header option
    // Select a profile
    // Click Apply
    // Verify API calls made for each host
  })

  it('should remove security header profile when "None" selected', async () => {
    // Select hosts with existing profiles
    // Select "None" option
    // Verify null is sent to API
  })

  it('should disable Apply button when no options selected', async () => {
    // Ensure all checkboxes are unchecked
    // Verify Apply button is disabled
  })
})
```

### D.2. Backend Unit Tests

**File to Create:** `backend/internal/api/handlers/proxy_host_handler_security_headers_test.go`

```go
package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestProxyHostHandler_BulkUpdateSecurityHeaders_Success(t *testing.T) {
    // Setup test database with hosts and profiles
    // Create request with valid host UUIDs and profile ID
    // Assert 200 response
    // Assert all hosts updated
    // Assert Caddy config applied
}

func TestProxyHostHandler_BulkUpdateSecurityHeaders_RemoveProfile(t *testing.T) {
    // Create hosts with existing profiles
    // Send null security_header_profile_id
    // Assert profiles removed
}

func TestProxyHostHandler_BulkUpdateSecurityHeaders_InvalidProfile(t *testing.T) {
    // Send non-existent profile ID
    // Assert 400 error
}

func TestProxyHostHandler_BulkUpdateSecurityHeaders_EmptyUUIDs(t *testing.T) {
    // Send empty host_uuids array
    // Assert 400 error
}

func TestProxyHostHandler_BulkUpdateSecurityHeaders_PartialFailure(t *testing.T) {
    // Include some invalid UUIDs
    // Assert partial success response
    // Assert error details for failed hosts
}
```

### D.3. Integration Test Scenarios

**File:** `scripts/integration/bulk_security_headers_test.sh`

```bash
#!/bin/bash
# Test bulk apply security headers feature

# 1. Create 3 test proxy hosts
# 2. Create a security header profile
# 3. Bulk apply profile to all hosts
# 4. Verify all hosts have profile assigned
# 5. Bulk remove profile (set to null)
# 6. Verify all hosts have no profile
# 7. Cleanup test data
```

---

## E. Implementation Phases

### Phase 1: Core UI Changes (Frontend Only)

**Duration:** 2-3 hours

**Tasks:**

1. [ ] Add `bulkSecurityHeaderProfile` state to ProxyHosts.tsx
2. [ ] Import and use `useSecurityHeaderProfiles` hook
3. [ ] Add Security Header Profile section to Bulk Apply modal UI
4. [ ] Update Apply button handler to include profile updates
5. [ ] Update Apply button disabled state logic

**Dependencies:** None

**Deliverable:** Working bulk apply with security headers using individual API calls

---

### Phase 2: Translation Updates

**Duration:** 30 minutes

**Tasks:**

1. [ ] Add translation keys to `en/translation.json`
2. [ ] Add translation keys to `de/translation.json`
3. [ ] Add translation keys to `es/translation.json`
4. [ ] Add translation keys to `fr/translation.json`
5. [ ] Add translation keys to `zh/translation.json`

**Dependencies:** Phase 1

**Deliverable:** Localized UI strings

---

### Phase 3: Backend Bulk Endpoint (Optional Optimization)

**Duration:** 1-2 hours

**Tasks:**

1. [ ] Add `BulkUpdateSecurityHeaders` handler function
2. [ ] Register new route in `RegisterRoutes()`
3. [ ] Add `DB()` getter to ProxyHostService if needed
4. [ ] Update frontend to use new bulk endpoint

**Dependencies:** Phase 1

**Deliverable:** Optimized bulk update with single API call

---

### Phase 4: Testing

**Duration:** 2-3 hours

**Tasks:**

1. [ ] Write frontend unit tests
2. [ ] Write backend unit tests
3. [ ] Create integration test script
4. [ ] Manual QA testing

**Dependencies:** Phases 1-3

**Deliverable:** Full test coverage

---

### Phase 5: Documentation

**Duration:** 30 minutes

**Tasks:**

1. [ ] Update CHANGELOG.md
2. [ ] Update docs/features.md if needed
3. [ ] Add release notes

**Dependencies:** Phases 1-4

**Deliverable:** Updated documentation

---

## F. Configuration Files Review

### F.1. .gitignore

**Status:** ✅ No changes needed

Current `.gitignore` already covers all relevant patterns for new test files and build artifacts.

### F.2. codecov.yml

**Status:** ⚠️ File not found in repository

If code coverage tracking is needed, create `codecov.yml` with:

```yaml
coverage:
  status:
    project:
      default:
        target: 85%
    patch:
      default:
        target: 80%
```

### F.3. .dockerignore

**Status:** ✅ No changes needed

Current `.dockerignore` already excludes test files, coverage artifacts, and documentation.

### F.4. Dockerfile

**Status:** ✅ No changes needed

No changes to build process required for this feature.

---

## G. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Performance with many hosts | Medium | Low | Phase 3 adds bulk endpoint |
| State desync after partial failure | Low | Medium | Show clear error messages per host |
| Mobile app compatibility warnings | Low | Low | Reuse existing warning component from ProxyHostForm |
| Translation missing | Medium | Low | Fallback to English |

---

## H. Success Criteria

1. ✅ User can select Security Header Profile in Bulk Apply modal
2. ✅ Profile can be applied to multiple hosts in single operation
3. ✅ Profile can be removed (set to None) via bulk apply
4. ✅ UI shows preset and custom profiles grouped separately
5. ✅ Progress indicator shows during bulk operation
6. ✅ Error handling for partial failures
7. ✅ All translations in place
8. ✅ Unit test coverage ≥80%
9. ✅ Integration tests pass

---

## I. Files Summary

### Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/pages/ProxyHosts.tsx` | Add state, hook, modal UI, apply logic |
| `frontend/src/locales/en/translation.json` | Add 3 new keys |
| `frontend/src/locales/de/translation.json` | Add 3 new keys |
| `frontend/src/locales/es/translation.json` | Add 3 new keys |
| `frontend/src/locales/fr/translation.json` | Add 3 new keys |
| `frontend/src/locales/zh/translation.json` | Add 3 new keys |
| `backend/internal/api/handlers/proxy_host_handler.go` | Add bulk endpoint (optional) |

### Files to Create

| File | Purpose |
|------|---------|
| `frontend/src/pages/__tests__/ProxyHosts.bulkApplyHeaders.test.tsx` | Frontend tests |
| `backend/internal/api/handlers/proxy_host_handler_security_headers_test.go` | Backend tests |
| `scripts/integration/bulk_security_headers_test.sh` | Integration tests |

### Files Unchanged (No Action Needed)

| File | Reason |
|------|--------|
| `.gitignore` | Already covers new file patterns |
| `.dockerignore` | Already excludes test/docs files |
| `Dockerfile` | No build changes needed |
| `frontend/src/api/proxyHosts.ts` | Uses existing `updateProxyHost()` |
| `frontend/src/hooks/useProxyHosts.ts` | Uses existing `updateHost()` |
| `frontend/src/utils/proxyHostsHelpers.ts` | No changes needed |

---

## J. Conclusion

This implementation plan provides a complete roadmap for adding HTTP Security Headers to the Bulk Apply feature. The phased approach allows for incremental delivery:

1. **Phase 1** delivers a working feature using existing API infrastructure
2. **Phase 2** completes localization
3. **Phase 3** optimizes performance for large-scale operations
4. **Phases 4-5** ensure quality and documentation

The feature integrates naturally with the existing Bulk Apply modal pattern and reuses the Security Header Profile infrastructure already built for individual host editing.
