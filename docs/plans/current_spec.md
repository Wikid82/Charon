# Security Headers UX Fix - Complete Specification

**Created:** 2025-12-18
**Status:** Ready for Implementation

---

## Executive Summary

This plan addresses two critical UX issues with Security Headers:

1. **"Apply" button creates copies instead of assigning presets** - Users expect presets to be assignable profiles, not templates that clone
2. **No UI to assign profiles to proxy hosts** - Users cannot activate security headers for their hosts

### Current State Analysis

#### ✅ What Works
- `EnsurePresetsExist()` **IS called on startup** ([routes.go:272](../backend/internal/api/routes/routes.go#L272))
- Basic, Strict, Paranoid presets **ARE created in the database** with `is_preset=true`
- Backend model **HAS** `SecurityHeaderProfileID` field ([proxy_host.go:41](../backend/internal/models/proxy_host.go#L41))
- Backend relationships are properly configured with GORM foreign keys

#### ❌ What's Broken
- **Frontend UI** - ProxyHostForm has NO security headers section
- **Frontend types** - ProxyHost interface missing `security_header_profile_id`
- **Backend handler** - Update handler does NOT process `security_header_profile_id` field
- **UX confusion** - "Apply" button suggests copying instead of assigning

---

## Part A: Fix Preset System (Make Presets "Real")

### Problem Statement

Current flow (CONFUSING):
```
User clicks "Apply" on Basic preset
  ↓
Creates NEW profile "Basic Security Profile"
  ↓
User cannot assign this to hosts (no UI)
  ↓
User is confused
```

Desired flow (CLEAR):
```
System creates Basic, Strict, Paranoid on startup
  ↓
User goes to Proxy Host form
  ↓
Selects "Basic Security" from dropdown
  ↓
Host uses preset directly (no copying)
```

### Solution: Remove "Apply" Button, Keep Presets as Assignable Profiles

#### Changes to SecurityHeaders.tsx

**Before:**
```tsx
<Button onClick={() => handleApplyPreset(profile.preset_type)}>
  <Play className="h-4 w-4 mr-1" /> Apply
</Button>
```

**After:**
```tsx
<Button onClick={() => setEditingProfile(profile)}>
  <Eye className="h-4 w-4 mr-1" /> View Details
</Button>
```

**Rationale:**
- Remove confusing "Apply" action
- Users can view preset settings (read-only modal)
- Users can clone if they want to customize
- Assignment happens in Proxy Host form (Part B)

#### Remove ApplyPreset Mutation

Since we're not copying presets anymore, remove:
- Frontend: `useApplySecurityHeaderPreset` hook
- Frontend: `applyPresetMutation` state
- Frontend: `handleApplyPreset` function
- Backend: Keep `ApplyPreset` service method (might be useful for future features)
- Backend: Keep `/presets/apply` endpoint (might be useful for API users)

---

## Part B: Add Profile Selector to Proxy Host Form

### Backend Changes

#### 1. Update Proxy Host Handler - Add security_header_profile_id Support

**File:** `backend/internal/api/handlers/proxy_host_handler.go`

**Location:** In `Update()` method, after the `access_list_id` handling block (around line 265)

**Add this code:**
```go
if v, ok := payload["security_header_profile_id"]; ok {
    if v == nil {
        host.SecurityHeaderProfileID = nil
    } else {
        switch t := v.(type) {
        case float64:
            if id, ok := safeFloat64ToUint(t); ok {
                host.SecurityHeaderProfileID = &id
            }
        case int:
            if id, ok := safeIntToUint(t); ok {
                host.SecurityHeaderProfileID = &id
            }
        case string:
            if n, err := strconv.ParseUint(t, 10, 32); err == nil {
                id := uint(n)
                host.SecurityHeaderProfileID = &id
            }
        }
    }
}
```

**Why:** Backend needs to accept and persist the security header profile assignment

#### 2. Preload Security Header Profile in Queries

**File:** `backend/internal/services/proxy_host_service.go`

**Method:** `GetByUUID()` and `List()`

**Change:**
```go
// Before
db.Preload("Certificate").Preload("AccessList").Preload("Locations")

// After
db.Preload("Certificate").Preload("AccessList").Preload("Locations").Preload("SecurityHeaderProfile")
```

**Why:** Frontend needs to display which profile is assigned to each host

---

### Frontend Changes

#### 1. Update ProxyHost Interface

**File:** `frontend/src/api/proxyHosts.ts`

**Add field:**
```typescript
export interface ProxyHost {
  // ... existing fields ...
  access_list_id?: number | null;
  security_header_profile_id?: number | null;  // ADD THIS
  security_header_profile?: {                   // ADD THIS
    id: number;
    uuid: string;
    name: string;
    description: string;
    security_score: number;
    is_preset: boolean;
  } | null;
  created_at: string;
  updated_at: string;
}
```

**Why:** TypeScript needs to know about the new field

#### 2. Add Security Headers Section to ProxyHostForm

**File:** `frontend/src/components/ProxyHostForm.tsx`

**Location:** After the "Access Control List" section (after line 750), before "Application Preset"

**Add imports:**
```typescript
import { useSecurityHeaderProfiles } from '../hooks/useSecurityHeaders'
import { SecurityScoreDisplay } from './SecurityScoreDisplay'
```

**Add hook:**
```typescript
const { data: securityProfiles } = useSecurityHeaderProfiles()
```

**Add to formData state:**
```typescript
const [formData, setFormData] = useState<ProxyHostFormState>({
  // ... existing fields ...
  access_list_id: host?.access_list_id,
  security_header_profile_id: host?.security_header_profile_id,  // ADD THIS
})
```

**Add UI section:**
```tsx
{/* Security Headers Profile */}
<div>
  <label className="block text-sm font-medium text-gray-300 mb-2">
    Security Headers
    <span className="text-gray-500 font-normal ml-2">(Optional)</span>
  </label>

  <select
    value={formData.security_header_profile_id || 0}
    onChange={e => {
      const value = parseInt(e.target.value) || null
      setFormData({ ...formData, security_header_profile_id: value })
    }}
    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
  >
    <option value={0}>None (No Security Headers)</option>
    <optgroup label="Quick Presets">
      {securityProfiles
        ?.filter(p => p.is_preset)
        .sort((a, b) => a.security_score - b.security_score)
        .map(profile => (
          <option key={profile.id} value={profile.id}>
            {profile.name} (Score: {profile.security_score}/100)
          </option>
        ))}
    </optgroup>
    {securityProfiles?.filter(p => !p.is_preset).length > 0 && (
      <optgroup label="Custom Profiles">
        {securityProfiles
          .filter(p => !p.is_preset)
          .map(profile => (
            <option key={profile.id} value={profile.id}>
              {profile.name} (Score: {profile.security_score}/100)
            </option>
          ))}
      </optgroup>
    )}
  </select>

  {formData.security_header_profile_id && (
    <div className="mt-2 flex items-center gap-2">
      {(() => {
        const selected = securityProfiles?.find(p => p.id === formData.security_header_profile_id)
        if (!selected) return null

        return (
          <>
            <SecurityScoreDisplay
              score={selected.security_score}
              size="sm"
              showDetails={false}
            />
            <span className="text-xs text-gray-400">
              {selected.description}
            </span>
          </>
        )
      })()}
    </div>
  )}

  <p className="text-xs text-gray-500 mt-1">
    Apply HTTP security headers to protect against common web vulnerabilities.{' '}
    <a
      href="/security-headers"
      target="_blank"
      className="text-blue-400 hover:text-blue-300"
    >
      Manage Profiles →
    </a>
  </p>
</div>
```

**Why:**
- Users need a clear, discoverable way to assign security headers
- Grouped by Presets vs Custom for easy scanning
- Shows security score inline for quick decision-making
- Link to Security Headers page for advanced users

---

## Part C: Update SecurityHeaders Page UI

### Changes to SecurityHeaders.tsx

**File:** `frontend/src/pages/SecurityHeaders.tsx`

#### 1. Remove "Apply" Button from Preset Cards

**Before (lines 143-149):**
```tsx
<Button
  variant="outline"
  size="sm"
  onClick={() => handleApplyPreset(profile.preset_type)}
  disabled={applyPresetMutation.isPending}
>
  <Play className="h-4 w-4 mr-1" /> Apply
</Button>
```

**After:**
```tsx
<Button
  variant="outline"
  size="sm"
  onClick={() => setEditingProfile(profile)}
>
  <Eye className="h-4 w-4 mr-1" /> View
</Button>
```

#### 2. Remove Unused Imports

**Remove:**
```typescript
import { useApplySecurityHeaderPreset } from '../hooks/useSecurityHeaders'
import { Play } from 'lucide-react'
```

**Keep:**
```typescript
import { Eye } from 'lucide-react'
```

#### 3. Remove Mutation and Handler

**Remove:**
```typescript
const applyPresetMutation = useApplySecurityHeaderPreset()

const handleApplyPreset = (presetType: string) => {
  const name = `${presetType.charAt(0).toUpperCase() + presetType.slice(1)} Security Profile`
  applyPresetMutation.mutate({ preset_type: presetType, name })
}
```

#### 4. Update Quick Presets Section Title and Description

**Before:**
```tsx
<h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Quick Presets</h2>
```

**After:**
```tsx
<h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
  System Profiles (Read-Only)
</h2>
<p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
  Pre-configured security profiles you can assign to proxy hosts. Clone to customize.
</p>
```

#### 5. Update Preset Modal to Show Read-Only Badge

**File:** `frontend/src/components/SecurityHeaderProfileForm.tsx`

**Add visual indicator when viewing presets:**
```tsx
{initialData?.is_preset && (
  <Alert variant="info" className="mb-4">
    <Shield className="w-4 h-4" />
    <div>
      <p className="font-semibold">System Profile (Read-Only)</p>
      <p className="text-sm mt-1">
        This is a built-in security profile. You cannot edit it, but you can clone it to create a custom version.
      </p>
    </div>
  </Alert>
)}
```

---

## UI Mockups (Text-Based)

### Proxy Host Form - Security Headers Section

```
┌────────────────────────────────────────────────────────────────┐
│ Security Headers (Optional)                                    │
├────────────────────────────────────────────────────────────────┤
│ ┌────────────────────────────────────────────────────────────┐ │
│ │ [Dropdown]                                                 │ │
│ │  None (No Security Headers)                                │ │
│ │  ─────────────────────────                                 │ │
│ │  Quick Presets                                             │ │
│ │    Basic Security (Score: 65/100)           ◄──── PRESET  │ │
│ │    Strict Security (Score: 85/100)          ◄──── PRESET  │ │
│ │    Paranoid Security (Score: 100/100)       ◄──── PRESET  │ │
│ │  ─────────────────────────                                 │ │
│ │  Custom Profiles                                           │ │
│ │    My API Profile (Score: 72/100)                          │ │
│ │    Production Profile (Score: 90/100)                      │ │
│ └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│ [🛡️ 85] Strong security for sensitive data                     │
│                                                                 │
│ Apply HTTP security headers to protect against common          │
│ web vulnerabilities. Manage Profiles →                         │
└────────────────────────────────────────────────────────────────┘
```

### Security Headers Page - Presets Section

```
┌────────────────────────────────────────────────────────────────┐
│ System Profiles (Read-Only)                                    │
│ Pre-configured security profiles you can assign to proxy hosts.│
│ Clone to customize.                                            │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ ┌──────────────────┐  ┌──────────────────┐  ┌────────────────┐│
│ │ Basic Security  │  │ Strict Security │  │ Paranoid Sec.  ││
│ │ [🛡️ 65/100]     │  │ [🛡️ 85/100]     │  │ [🛡️ 100/100]  ││
│ │                  │  │                  │  │                 ││
│ │ Essential        │  │ Strong security  │  │ Maximum sec.   ││
│ │ security for     │  │ for sensitive    │  │ for high-risk  ││
│ │ most websites    │  │ data             │  │ applications   ││
│ │                  │  │                  │  │                 ││
│ │ [View] [Clone]   │  │ [View] [Clone]   │  │ [View] [Clone] ││
│ └──────────────────┘  └──────────────────┘  └────────────────┘│
│                                                                 │
│ Custom Profiles                                                 │
│ ┌──────────────────┐  ┌──────────────────┐                    │
│ │ My Profile       │  │ API Profile      │                    │
│ │ [🛡️ 72/100]     │  │ [🛡️ 90/100]     │                    │
│ │ [Edit] [Clone]   │  │ [Edit] [Clone]   │                    │
│ └──────────────────┘  └──────────────────┘                    │
└────────────────────────────────────────────────────────────────┘
```

---

## Implementation Steps

### Phase 1: Backend Support (30 min)

1. ✅ **Verify Presets Creation**
   - Check logs on startup to confirm `EnsurePresetsExist()` runs
   - Query database: `SELECT * FROM security_header_profiles WHERE is_preset = true;`
   - Should see 3 rows: Basic, Strict, Paranoid

2. **Update Proxy Host Handler**
   - File: `backend/internal/api/handlers/proxy_host_handler.go`
   - Add `security_header_profile_id` handling in Update method
   - Test with curl:
     ```bash
     curl -X PUT http://localhost:8080/api/v1/proxy-hosts/{uuid} \
       -H "Content-Type: application/json" \
       -d '{"security_header_profile_id": 1}'
     ```

3. **Update Service Layer**
   - File: `backend/internal/services/proxy_host_service.go`
   - Add `.Preload("SecurityHeaderProfile")` to List and GetByUUID
   - Verify profile loads with GET request

### Phase 2: Frontend Types (10 min)

4. **Update TypeScript Interfaces**
   - File: `frontend/src/api/proxyHosts.ts`
   - Add `security_header_profile_id` and `security_header_profile` fields
   - Run `npm run type-check` to verify

### Phase 3: Frontend UI (45 min)

5. **Update ProxyHostForm**
   - File: `frontend/src/components/ProxyHostForm.tsx`
   - Add security headers section (see Part B)
   - Import `useSecurityHeaderProfiles` hook
   - Add dropdown with presets + custom profiles
   - Add score display and description
   - Test creating/editing hosts with profiles

6. **Update SecurityHeaders Page**
   - File: `frontend/src/pages/SecurityHeaders.tsx`
   - Remove "Apply" button from presets
   - Change to "View" button
   - Remove `useApplySecurityHeaderPreset` hook
   - Remove `handleApplyPreset` function
   - Update section titles and descriptions
   - Test viewing/cloning presets

### Phase 4: Testing (45 min)

7. **Backend Tests**
   - File: `backend/internal/api/handlers/proxy_host_handler_test.go`
   - Add test: `TestUpdateProxyHost_SecurityHeaderProfile`
   - Verify profile assignment works
   - Verify profile can be cleared (set to null)

8. **Integration Testing**
   - Create new proxy host
   - Assign "Basic Security" preset
   - Save and verify in database
   - Edit host, change to "Strict Security"
   - Edit host, remove profile (set to None)
   - Verify Caddy config includes security headers when profile assigned

9. **UX Testing**
   - User flow: Create host → Assign preset → Save
   - User flow: Edit host → Change profile → Save
   - User flow: View preset details (read-only modal)
   - User flow: Clone preset → Customize → Assign to host
   - Verify no confusing "Apply" button remains
   - Verify dropdown shows presets grouped separately

### Phase 5: Documentation (15 min)

10. **Update Feature Docs**
    - File: `docs/features.md`
    - Document new security header assignment flow
    - Add screenshots (if possible)
    - Explain preset vs custom profiles

11. **Update PR Template**
    - Mention UX improvement in PR description
    - Link to this spec document

---

## Testing Requirements

### Unit Tests

**Backend:**
```go
// File: backend/internal/api/handlers/proxy_host_handler_test.go

func TestUpdateProxyHost_SecurityHeaderProfile(t *testing.T) {
    // Test assigning profile
    // Test changing profile
    // Test removing profile (null)
    // Test invalid profile ID (should fail gracefully)
}

func TestCreateProxyHost_WithSecurityHeaderProfile(t *testing.T) {
    // Test creating host with profile assigned
}
```

**Frontend:**
```typescript
// File: frontend/src/components/ProxyHostForm.test.tsx

describe('ProxyHostForm - Security Headers', () => {
  it('shows security header dropdown', () => {})
  it('displays preset profiles grouped', () => {})
  it('displays custom profiles grouped', () => {})
  it('shows selected profile score', () => {})
  it('includes security_header_profile_id in submission', () => {})
  it('allows clearing profile selection', () => {})
})
```

### Integration Tests

1. **Preset Creation on Startup**
   - Start fresh Charon instance
   - Verify 3 presets exist in database
   - Verify UUIDs are correct: `preset-basic`, `preset-strict`, `preset-paranoid`

2. **Profile Assignment Flow**
   - Create proxy host with Basic Security preset
   - Verify `security_header_profile_id` is set in database
   - Verify profile relationship loads correctly
   - Verify Caddy config includes security headers

3. **Profile Update Flow**
   - Edit proxy host, change from Basic to Strict
   - Verify `security_header_profile_id` updates
   - Verify Caddy config updates with new headers

4. **Profile Removal Flow**
   - Edit proxy host, set profile to None
   - Verify `security_header_profile_id` is NULL
   - Verify Caddy config removes security headers

### Manual QA Checklist

- [ ] Presets visible on Security Headers page
- [ ] "Apply" button removed from presets
- [ ] "View" button opens read-only modal
- [ ] Clone button creates editable copy
- [ ] Proxy Host form shows Security Headers dropdown
- [ ] Dropdown groups Presets vs Custom
- [ ] Selected profile shows score inline
- [ ] "Manage Profiles" link works
- [ ] Creating host with profile saves correctly
- [ ] Editing host can change profile
- [ ] Removing profile (set to None) works
- [ ] Caddy config includes headers when profile assigned
- [ ] No errors in browser console
- [ ] TypeScript compiles without errors

---

## Edge Cases & Considerations

### 1. Deleting a Profile That's In Use

**Current Behavior:**
Backend checks if profile is in use before deletion ([security_headers_handler.go:202](../backend/internal/api/handlers/security_headers_handler.go#L202))

**Expected Behavior:**
- User tries to delete profile
- Backend returns error: "Cannot delete profile, it is assigned to N hosts"
- Frontend shows error toast
- User must reassign hosts first

**No Changes Needed** - Already handled correctly

### 2. Editing a Preset (Should Be Read-Only)

**Current Behavior:**
`SecurityHeaderProfileForm` checks `initialData?.is_preset` and disables fields

**Expected Behavior:**
- User clicks "View" on preset
- Modal opens in read-only mode
- All fields disabled
- "Save" button hidden
- "Clone" button available

**Implementation:**
Already handled in `SecurityHeaderProfileForm.tsx` - verify it works

### 3. Preset Updates (When Charon Updates)

**Scenario:**
New Charon version updates Strict preset from Score 85 → 87

**Current Behavior:**
`EnsurePresetsExist()` updates existing presets on startup

**Expected Behavior:**
- User updates Charon
- Startup runs `EnsurePresetsExist()`
- Presets update to new values
- Hosts using presets automatically get new headers on next Caddy reload

**No Changes Needed** - Already handled correctly

### 4. Cloning a Preset

**Expected Behavior:**
- User clicks "Clone" on "Basic Security"
- Creates new profile "Basic Security (Copy)"
- `is_preset = false`
- `preset_type = ""`
- New UUID generated
- User can now edit it

**Implementation:**
Already implemented via `handleCloneProfile()` - verify it works

### 5. Deleting All Custom Profiles

**Expected Behavior:**
- User deletes all custom profiles
- Custom Profiles section shows empty state
- Presets section still visible
- No UI breakage

**Implementation:**
Already handled - conditional rendering based on `customProfiles.length === 0`

---

## Rollback Plan

If critical issues found after deployment:

1. **Frontend Only Rollback**
   - Revert ProxyHostForm changes
   - Users lose ability to assign profiles (but data safe)
   - Existing assignments remain in database

2. **Full Rollback**
   - Revert all changes
   - Data: Keep `security_header_profile_id` values in database
   - No data loss - just UI reverts

3. **Database Migration (if needed)**
   - Not required - field already exists
   - No schema changes in this update

---

## Success Metrics

### User Experience
- ✅ No more confusing "Apply" button
- ✅ Clear visual hierarchy: System Profiles vs Custom
- ✅ Easy discovery of security header assignment
- ✅ Inline security score helps decision-making

### Technical
- ✅ Zero breaking changes to API
- ✅ Backward compatible with existing data
- ✅ No new database migrations needed
- ✅ Type-safe TypeScript interfaces

### Validation
- [ ] Run pre-commit checks (all pass)
- [ ] Backend unit tests (coverage ≥85%)
- [ ] Frontend unit tests (coverage ≥85%)
- [ ] Manual QA checklist (all items checked)
- [ ] TypeScript compiles without errors

---

## Files to Modify

### Backend (2 files)
1. `backend/internal/api/handlers/proxy_host_handler.go` - Add security_header_profile_id support
2. `backend/internal/services/proxy_host_service.go` - Add Preload("SecurityHeaderProfile")

### Frontend (3 files)
1. `frontend/src/api/proxyHosts.ts` - Add security_header_profile_id to interface
2. `frontend/src/components/ProxyHostForm.tsx` - Add Security Headers section
3. `frontend/src/pages/SecurityHeaders.tsx` - Remove Apply button, update UI

### Tests (2+ files)
1. `backend/internal/api/handlers/proxy_host_handler_test.go` - Add security profile tests
2. `frontend/src/components/ProxyHostForm.test.tsx` - Add security headers tests

### Docs (1 file)
1. `docs/features.md` - Update security headers documentation

---

## Dependencies & Prerequisites

### Already Satisfied ✅
- Backend model has `SecurityHeaderProfileID` field
- Backend relationships configured (GORM foreign keys)
- `EnsurePresetsExist()` runs on startup
- Presets service methods exist
- Security header profiles API endpoints exist
- Frontend hooks for security headers exist

### New Dependencies ❌
- None - All required functionality already exists

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing hosts | Low | High | Backward compatible - field already exists, nullable |
| TypeScript errors | Low | Medium | Run type-check before commit |
| Caddy config errors | Low | High | Test with various profile types |
| UI confusion | Low | Low | Clear labels, grouped options |
| Performance impact | Very Low | Low | Single extra Preload, negligible |

---

## Timeline

- **Phase 1-2 (Backend + Types):** 40 minutes
- **Phase 3 (Frontend UI):** 45 minutes
- **Phase 4 (Testing):** 45 minutes
- **Phase 5 (Documentation):** 15 minutes

**Total:** ~2.5 hours for complete implementation and testing

---

## Appendix A: Current Code Analysis

### Startup Flow

```
main()
  ↓
server.Run()
  ↓
routes.RegisterRoutes()
  ↓
secHeadersSvc.EnsurePresetsExist()  ← RUNS HERE
  ↓
Creates/Updates 3 presets in database
```

**Verification Query:**
```sql
SELECT id, uuid, name, is_preset, preset_type, security_score
FROM security_header_profiles
WHERE is_preset = true;
```

**Expected Result:**
```
id | uuid            | name                | is_preset | preset_type | security_score
---+-----------------+---------------------+-----------+-------------+---------------
1  | preset-basic    | Basic Security      | true      | basic       | 65
2  | preset-strict   | Strict Security     | true      | strict      | 85
3  | preset-paranoid | Paranoid Security   | true      | paranoid    | 100
```

### Current ProxyHost Data Flow

```
Frontend (ProxyHostForm)
  ↓ POST /api/v1/proxy-hosts
Backend Handler (Create)
  ↓ Validates JSON
ProxyHostService.Create()
  ↓ GORM Insert
Database (proxy_hosts table)
  ↓ Includes certificate_id, access_list_id
  ↓ SHOULD include security_header_profile_id (but form doesn't send it)
```

### Missing Piece

**Form doesn't include:**
```typescript
security_header_profile_id: formData.security_header_profile_id
```

**Handler doesn't read:**
```go
if v, ok := payload["security_header_profile_id"]; ok {
    // ... handle it
}
```

---

## Appendix B: Preset Definitions

### Basic Security (Score: 65)
- HSTS: 1 year, no subdomains
- X-Frame-Options: SAMEORIGIN
- X-Content-Type-Options: nosniff
- Referrer-Policy: strict-origin-when-cross-origin
- XSS-Protection: enabled
- **No CSP** (to avoid breaking sites)

### Strict Security (Score: 85)
- HSTS: 1 year, includes subdomains
- CSP: Restrictive defaults
- X-Frame-Options: DENY
- Permissions-Policy: Block camera/mic/geolocation
- CORS policies: same-origin
- XSS-Protection: enabled

### Paranoid Security (Score: 100)
- HSTS: 2 years, preload enabled
- CSP: Maximum restrictions
- X-Frame-Options: DENY
- Permissions-Policy: Block all dangerous features
- CORS: Strict same-origin
- Cache-Control: no-store
- Cross-Origin-Embedder-Policy: require-corp

---

## Notes for Implementation

1. **Start with Backend** - Easier to test with curl before UI work
2. **Use VS Code Tasks** - Run "Lint: TypeScript Check" after frontend changes
3. **Test Incrementally** - Don't wait until everything is done
4. **Check Caddy Config** - Verify headers appear in generated config
5. **Mobile Responsive** - Test dropdown on mobile viewport
6. **Accessibility** - Ensure dropdown has proper labels
7. **Dark Mode** - Verify colors work in both themes

---

## Questions Resolved During Investigation

**Q: Are presets created in the database?**
**A:** Yes, `EnsurePresetsExist()` runs on startup and creates/updates them.

**Q: Does the backend model support profile assignment?**
**A:** Yes, `SecurityHeaderProfileID` field exists with proper GORM relationships.

**Q: Do the handlers support the field?**
**A:** No - Update handler needs to be modified to accept `security_header_profile_id`.

**Q: Is there UI to assign profiles?**
**A:** No - ProxyHostForm has no security headers section. This is the main missing piece.

**Q: What does "Apply" button do?**
**A:** It calls `ApplyPreset()` which COPIES the preset to create a new custom profile. Confusing!

---

**End of Specification**

This document is ready for implementation. Follow the phases in order, run tests after each phase, and verify the UX improvements work as expected.
