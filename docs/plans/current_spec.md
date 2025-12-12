# Implementation Plan: Rename WAF Card to Coraza on Cerberus Dashboard

## Overview

Modify the WAF card on the Cerberus Dashboard to:

1. Rename "WAF" to "Coraza" (consistent with how CrowdSec is named - the card uses the product name)
2. Remove the Mode and Rule Set dropdowns from the card (these are handled on the config page)

## Reference: CrowdSec Card Pattern

The CrowdSec card naming convention shows that we use the **product name** (CrowdSec) rather than generic terms. Similarly, WAF should become "Coraza" since Coraza is the underlying product.

**CrowdSec Card Structure:**

- Title: "CrowdSec" (not "IPS" or "Intrusion Prevention System")
- Simple enabled/disabled status
- Config button navigates to `/security/crowdsec` for detailed configuration

**Current WAF Card Issues:**

- Title: "WAF (Coraza)" - should just be "Coraza"
- Contains Mode and Rule Set dropdowns (should be on config page only)
- Inconsistent with CrowdSec card simplicity

## Files to Modify

### 1. Frontend: Security Dashboard Page

**File:** `frontend/src/pages/Security.tsx`

**Changes Required:**

| Line(s) | Current Text/Code | New Text/Code | Notes |
|---------|------------------|---------------|-------|
| 171 | `Cerberus powers CrowdSec, WAF, ACLs, and Rate Limiting.` | `Cerberus powers CrowdSec, Coraza, ACLs, and Rate Limiting.` | Info banner text |
| 317 | `{/* WAF - Layer 3: Request Inspection */}` | `{/* Coraza - Layer 3: Request Inspection */}` | Comment update |
| 321 | `<h3 className="text-sm font-medium text-white">WAF (Coraza)</h3>` | `<h3 className="text-sm font-medium text-white">Coraza</h3>` | Card title |
| 337-340 | Protection text using "WAF" terminology | Keep as-is | Still valid for "Coraza" |
| **341-379** | **WAF Mode and Rule Set dropdowns (entire block)** | **REMOVE** | Delete the entire conditional block that renders dropdowns when WAF is enabled |
| 383 | `onClick={() => navigate('/security/waf')}` | Keep same path | Route stays the same, just config page |
| 385 | `{status.waf.enabled ? 'Manage Rule Sets' : 'Configure'}` | `{status.waf.enabled ? 'Configure' : 'Configure'}` or just `Configure` | Simplify button text |

**Specific Block to Remove (Lines ~341-379):**

```tsx
{status.waf.enabled && (
  <div className="mt-3 space-y-3">
    <div>
      <label className="text-xs text-gray-400 block mb-1">WAF Mode</label>
      <select
        value={securityConfig?.config?.waf_mode || 'block'}
        onChange={(e) => updateSecurityConfigMutation.mutate({ name: 'default', waf_mode: e.target.value })}
        className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-white"
        data-testid="waf-mode-select"
      >
        <option value="block">Block (deny malicious requests)</option>
        <option value="monitor">Monitor (log only, don't block)</option>
      </select>
    </div>
    <div>
      <label className="text-xs text-gray-400 block mb-1">Active Rule Set</label>
      <select
        value={securityConfig?.config?.waf_rules_source || ''}
        onChange={(e) => updateSecurityConfigMutation.mutate({ name: 'default', waf_rules_source: e.target.value || undefined })}
        className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-white"
        data-testid="waf-ruleset-select"
      >
        <option value="">None (all rule sets)</option>
        {ruleSetsData?.rulesets?.map((rs) => (
          <option key={rs.id} value={rs.name}>
            {rs.name} ({rs.mode === 'blocking' ? 'blocking' : 'detection'})
          </option>
        ))}
      </select>
      {(!ruleSetsData?.rulesets || ruleSetsData.rulesets.length === 0) && (
        <p className="text-xs text-yellow-500 mt-1">
          No rule sets configured. Add one below.
        </p>
      )}
    </div>
  </div>
)}
```

### 2. Frontend: Layout Navigation

**File:** `frontend/src/components/Layout.tsx`

**Changes Required:**

| Line | Current Text | New Text |
|------|-------------|----------|
| 70 | `{ name: 'WAF (Coraza)', path: '/security/waf', icon: '🛡️' }` | `{ name: 'Coraza', path: '/security/waf', icon: '🛡️' }` |

### 3. Test Files to Update

#### 3.1 Security Page Tests

**File:** `frontend/src/pages/__tests__/Security.spec.tsx`

**Changes Required:**

| Test Name | Change Description |
|-----------|-------------------|
| `shows WAF mode selector when WAF is enabled` | **DELETE entire test** - Mode selector no longer on dashboard |
| `shows WAF ruleset selector with available rulesets` | **DELETE entire test** - Ruleset selector no longer on dashboard |
| `calls updateSecurityConfig when WAF mode is changed` | **DELETE entire test** - No mode selector on dashboard |
| `calls updateSecurityConfig when WAF ruleset is changed` | **DELETE entire test** - No ruleset selector on dashboard |
| `shows warning when no rulesets are configured` | **DELETE entire test** - Warning no longer on dashboard |
| `displays correct WAF threat protection summary when enabled` | Keep but update any "WAF" string references if needed |
| `does not show WAF controls when WAF is disabled` | **DELETE entire test** - Controls never shown on dashboard now |

**Tests to delete (Lines ~189-344):**

- `it('shows WAF mode selector when WAF is enabled', ...)`
- `it('shows WAF ruleset selector with available rulesets', ...)`
- `it('calls updateSecurityConfig when WAF mode is changed', ...)`
- `it('calls updateSecurityConfig when WAF ruleset is changed', ...)`
- `it('shows warning when no rulesets are configured', ...)`
- `it('does not show WAF controls when WAF is disabled', ...)`

Keep:

- `it('displays correct WAF threat protection summary when enabled', ...)` - This tests the protection description text, not the dropdowns

#### 3.2 Security Audit Tests

**File:** `frontend/src/pages/__tests__/Security.audit.test.tsx`

**Changes Required:**

| Line | Current Text | New Text |
|------|-------------|----------|
| 287 | `expect(screen.getByText('WAF (Coraza)')).toBeInTheDocument()` | `expect(screen.getByText('Coraza')).toBeInTheDocument()` |
| 306-315 | `it('WAF controls have proper test IDs when enabled', ...)` | **DELETE entire test** - WAF controls no longer on dashboard |
| 344 | `expect(cardNames).toEqual(['CrowdSec', 'Access Control', 'WAF (Coraza)', 'Rate Limiting', 'Live Security Logs'])` | `expect(cardNames).toEqual(['CrowdSec', 'Access Control', 'Coraza', 'Rate Limiting', 'Live Security Logs'])` |

### 4. WAF Config Page (NO CHANGES NEEDED)

**File:** `frontend/src/pages/WafConfig.tsx`

The WAF config page already properly uses "WAF" terminology in its title ("WAF Configuration") and references "Coraza" where appropriate. Since this is the configuration page, the Mode and Rule Set selections should remain here. **No changes required.**

### 5. WAF Config Tests (NO CHANGES NEEDED)

**File:** `frontend/src/pages/__tests__/WafConfig.spec.tsx`

These tests test the WafConfig page which is unaffected. **No changes required.**

## Summary of Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `frontend/src/pages/Security.tsx` | Modify | Rename "WAF" → "Coraza", remove Mode/RuleSet dropdowns |
| `frontend/src/components/Layout.tsx` | Modify | Rename nav item "WAF (Coraza)" → "Coraza" |
| `frontend/src/pages/__tests__/Security.spec.tsx` | Delete tests | Remove 6 tests for WAF dropdown controls |
| `frontend/src/pages/__tests__/Security.audit.test.tsx` | Modify | Update card name assertions, remove dropdown test |

## API Changes

**None required.** This is purely a frontend UI change. The backend API endpoints, types, and data structures remain unchanged.

## Type Definition Changes

**None required.** The SecurityStatus type and related interfaces don't need modification.

## Text/Label Changes Summary

| Location | From | To |
|----------|------|-----|
| Card title (Security.tsx) | `WAF (Coraza)` | `Coraza` |
| Nav sidebar (Layout.tsx) | `WAF (Coraza)` | `Coraza` |
| Info banner (Security.tsx) | `CrowdSec, WAF, ACLs` | `CrowdSec, Coraza, ACLs` |
| Comment (Security.tsx) | `/* WAF - Layer 3 */` | `/* Coraza - Layer 3 */` |
| Test assertions | `WAF (Coraza)` | `Coraza` |

## Button Simplification

The button text on the Coraza card should be simplified:

- **Current:** `{status.waf.enabled ? 'Manage Rule Sets' : 'Configure'}`
- **New:** `Configure` (always)

This matches the CrowdSec card pattern which just shows "Config" regardless of enabled state.

## Implementation Order

1. Update `Security.tsx`:
   - Change card title from "WAF (Coraza)" to "Coraza"
   - Update banner text
   - Update comment
   - Remove the dropdown controls block
   - Simplify button text

2. Update `Layout.tsx`:
   - Change nav item name

3. Update test files:
   - `Security.spec.tsx`: Remove obsolete tests
   - `Security.audit.test.tsx`: Update assertions, remove dropdown test

4. Run tests to verify: `cd frontend && npm test`

5. Run type check: `cd frontend && npm run type-check`

6. Run pre-commit checks

## Verification Checklist

- [ ] Card title shows "Coraza" (not "WAF (Coraza)")
- [ ] Nav sidebar shows "Coraza"
- [ ] No Mode dropdown on dashboard card
- [ ] No Rule Set dropdown on dashboard card
- [ ] "Configure" button navigates to `/security/waf` config page
- [ ] Mode/Rule Set controls still available on `/security/waf` config page
- [ ] All tests pass
- [ ] TypeScript compiles without errors
- [ ] Pre-commit hooks pass
