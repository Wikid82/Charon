# Phase 2.2: Docker Integration Investigation - Discovery Report

**Date:** 2026-02-09
**Status:** Root Cause Identified
**Severity:** High - Tests Cannot Run Due to Missing Element IDs

## Summary

Container selector appears to not render when Docker source is selected. Investigation revealed the root cause: **test locators are looking for element IDs that don't exist in the ProxyHostForm component**.

## Failing Tests

- **Test 154:** `tests/core/proxy-hosts.spec.ts:996` - "should show Docker container selector when source is selected"
- **Test 155:** `tests/core/proxy-hosts.spec.ts:1015` - "should show containers dropdown when Docker source selected"

## Root Cause Analysis

### Issue 1: Missing Element IDs
The tests use hardcoded selector IDs that are not present in the ProxyHostForm component:

**Test Code:**
```typescript
// Line 1007 in tests/core/proxy-hosts.spec.ts
const sourceSelect = page.locator('#connection-source');
await expect(sourceSelect).toBeVisible();

// Line 1024 in tests/core/proxy-hosts.spec.ts
const containersSelect = page.locator('#quick-select-docker');
await expect(containersSelect).toBeVisible();
```

**Actual Code in ProxyHostForm.tsx (lines 599-639):**
```tsx
<Select value={connectionSource} onValueChange={setConnectionSource}>
  <SelectTrigger className="w-full bg-gray-900 border-gray-700 text-white" aria-label="Source">
    <SelectValue />
  </SelectTrigger>
  <SelectContent>
    <SelectItem value="custom">Custom / Manual</SelectItem>
    <SelectItem value="local">Local (Docker Socket)</SelectItem>
    {remoteServers.map(server => ...)}
  </SelectContent>
</Select>

{/* Containers dropdown - no id */}
<Select value="" onValueChange={e => e && handleContainerSelect(e)}>
  <SelectTrigger className="w-full bg-gray-900 border-gray-700 text-white disabled:opacity-50" disabled={dockerLoading || connectionSource === 'custom'} aria-label="Containers">
    <SelectValue placeholder={connectionSource === 'custom' ? 'Select a source to view containers' : (dockerLoading ? 'Loading containers...' : 'Select a container')} />
  </SelectTrigger>
  <SelectContent>
    {dockerContainers.map(container => ...)}
  </SelectContent>
</Select>
```

**Finding:** Neither Select component has an `id` attribute. The tests cannot locate them.

### Issue 2: Test Approach Mismatch
The tests use outdated selectors:
- Looking for `<select>` HTML elements (using `selectOption()`)
- But the code uses custom shadcn/ui `<Select>` components with complex internal structure
- The selector strategy needs to align with how shadcn UI renders

## Frontend Implementation Analysis

### Current Flow (Working)
1. Source dropdown initialized to `'custom'`
2. When user selects a Docker source (local or remote server), `setConnectionSource()` updates state
3. `useDocker` hook is called with proper parameters:
   - `host='local'` if `connectionSource === 'local'`
   - `serverId=connectionSource` if it's a remote server UUID
4. Containers dropdown is disabled when `connectionSource === 'custom'`
5. When containers load, they appear in the dropdown

**Code Flow (Lines 250-254 in ProxyHostForm.tsx):**
```tsx
const { containers: dockerContainers, isLoading: dockerLoading, error: dockerError } = useDocker(
  connectionSource === 'local' ? 'local' : undefined,
  connectionSource !== 'local' && connectionSource !== 'custom' ? connectionSource : undefined
)
```

This logic is **correct**. The component is likely working in the UI, but tests can't verify it.

### Potential Runtime Issues (Secondary)
While the frontend code appears structurally sound, there could be timing/state issues:

1. **Race Condition:** `useDocker` hook might not be triggered immediately when `connectionSource` changes
   - Solution: Verify `enabled` flag in `useQuery` (currently correctly set to `Boolean(host) || Boolean(serverId)`)

2. **API Endpoint:** Tests might fail on loading containers due to missing backend endpoint
   - Need to verify: `/api/v1/docker/containers` endpoint exists and returns containers

3. **Async State Update:** Component might not re-render properly when `dockerContainers` updates
   - Current implementation looks correct, but should verify in browser

## Recommended Fixes

### CRITICAL: Add Element IDs to ProxyHostForm
Location: `frontend/src/components/ProxyHostForm.tsx`

**Fix 1: Source Select (line 599)**
```tsx
<Select value={connectionSource} onValueChange={setConnectionSource}>
  <SelectTrigger id="connection-source" className="w-full bg-gray-900 border-gray-700 text-white" aria-label="Source">
    <SelectValue />
  </SelectTrigger>
  {/* ... */}
</Select>
```

**Fix 2: Containers Select (line 623)**
```tsx
<Select value="" onValueChange={e => e && handleContainerSelect(e)}>
  <SelectTrigger id="quick-select-docker" className="w-full bg-gray-900 border-gray-700 text-white disabled:opacity-50" disabled={dockerLoading || connectionSource === 'custom'} aria-label="Containers">
    <SelectValue placeholder={...} />
  </SelectTrigger>
  {/* ... */}
</Select>
```

### IMPORTANT: Fix Test Selector Strategy
Location: `tests/core/proxy-hosts.spec.ts` lines 996-1030

Current approach (broken):
```typescript
const sourceSelect = page.locator('#connection-source');
await sourceSelect.selectOption('local');  // selectOption doesn't work with custom Select
```

Better approach (for shadcn Select):
```typescript
// For Source dropdown
const sourceButton = page.getByRole('button', { name: 'Source' }).first();
await sourceButton.click();
const localOption = page.getByRole('option', { name: /local/i });
await localOption.click();

// For Containers dropdown
const containersButton = page.getByRole('button', { name: 'Containers' }).first();
await containersButton.click();
// Wait for containers to load
await page.getByRole('option').first().waitFor({ state: 'visible' });
```

### OPTIONAL: Verify Backend Docker API
- Ensure `/api/v1/docker/containers` endpoint exists
- Returns proper container list with: `id`, `names[]`, `image`, `ports[]`
- Handles errors gracefully (503 if Docker not available)

## Testing Strategy

1. **Add IDs to components** (implements fix)
2. **Update test selectors** to use role-based approach compatible with shadcn/ui
3. **Manual verification:**
   - Open DevTools in browser
   - Navigate to proxy hosts form
   - Select "Local (Docker Socket)" from Source dropdown
   - Verify: Containers dropdown becomes enabled and loads containers
   - Verify: Container list populated and clickable
4. **Run automated tests:** Both test 154 and 155 should pass

## Files to Modify

1. **Frontend:**
   - `frontend/src/components/ProxyHostForm.tsx` - Add ids to Select triggers

2. **Tests:**
   - `tests/core/proxy-hosts.spec.ts` - Update selectors to use role-based approach (lines 996-1030)

## Success Criteria

- Tests 154 & 155 pass consistently
- No new test failures in proxy hosts test suite
- Container selector visible and functional when Docker source selected
- All container operations work (select, auto-populate form)

## Next Steps

1. Implement critical fixes (add IDs)
2. Update test selectors
3. Run proxy hosts test suite
4. Verify E2E Docker workflow manually
5. Check for additional edge cases (no docker available, permission errors, etc.)
