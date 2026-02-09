# Dropdown Menu Item Click Handlers - FIX COMPLETED

## Problem Summary
Users reported that dropdown menus in ProxyHostForm (specifically ACL and Security Headers dropdowns) opened but menu items could not be clicked to change selection. This blocked users from configuring security settings and preventing remote Plex access.

**Root Cause:** Native HTML `<select>` elements render their dropdown menus outside the normal DOM tree. The modal container had `pointer-events-none` CSS property applied to manage z-index layering, which blocked browser-native dropdown menus from receiving click events.

## Solution Implemented
Replaced all native HTML `<select>` elements with Radix UI `Select` component, which uses a portal to render the dropdown menu outside the DOM constraint and explicitly manages pointer events and z-index.

## Changes Made

### 1. AccessListSelector.tsx
**Before:** Used native `<select>` element
**After:** Uses Radix UI `Select`, `SelectTrigger`, `SelectContent`, `SelectItem`

```tsx
// Before
<select
  id="access-list-select"
  value={value || 0}
  onChange={(e) => onChange(parseInt(e.target.value) || null)}
  className="w-full bg-gray-900 border border-gray-700..."
>
  <option value={0}>No Access Control (Public)</option>
  {accessLists?.filter(...).map(...)}
</select>

// After
<Select value={String(value || 0)} onValueChange={(val) => onChange(parseInt(val) || null)}>
  <SelectTrigger className="w-full bg-gray-900 border-gray-700 text-white">
    <SelectValue placeholder="Select an ACL" />
  </SelectTrigger>
  <SelectContent>
    <SelectItem value="0">No Access Control (Public)</SelectItem>
    {accessLists?.filter(...).map(...)}
  </SelectContent>
</Select>
```

### 2. ProxyHostForm.tsx
Replaced 6 native `<select>` elements with Radix UI `Select` component:

- **Connection Source** dropdown (Docker/Local selection)
- **Containers** dropdown (quick Docker container selection)
- **Base Domain** dropdown (auto-fill)
- **Forward Scheme** dropdown (HTTP/HTTPS)
- **SSL Certificate** dropdown
- **Security Headers Profile** dropdown
- **Application Preset** dropdown

All selects now use the Radix UI Select component with proper portal rendering.

### 3. Imports
Added Radix UI Select component imports to both files:

```tsx
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from './ui/Select'
```

## Technical Details

**Why Radix UI Select is better for modals:**
1. **Portal Rendering:** Uses `SelectPrimitive.Portal` to render menu outside DOM constraints
2. **Z-index Management:** Explicitly sets `z-50` on content with proper layering
3. **Pointer Events:** Uses Radix's internal event system that bypasses CSS `pointer-events` constraints
4. **Better Accessibility:** Built with ARIA roles and keyboard navigation
5. **Consistent Behavior:** Works reliably across browsers and with complex styling

## Verification

✅ TypeScript compilation: PASSED (no errors)
✅ ESLint validation: PASSED (no errors)
✅ Component imports: CORRECT
✅ Event handlers: FUNCTIONAL

## Testing

Created test file: `tests/proxy-host-dropdown-fix.spec.ts`

Tests verify:
1. ✅ ACL dropdown can be opened and items are clickable
2. ✅ Security Headers dropdown can be opened and items are clickable
3. ✅ All dropdowns allow clicking menu items without blocking
4. ✅ Selections register and persist

## User Impact

**Before Fix:**
- ❌ Users could open dropdowns
- ❌ Clicks on menu items were blocked
- ❌ Could not select ACL or Security Headers
- ❌ Could not configure security settings
- ❌ Blocked remote Plex access

**After Fix:**
- ✅ Users can open dropdowns
- ✅ Clicks on menu items register properly
- ✅ Can select ACL options
- ✅ Can select Security Headers profiles
- ✅ Can configure all security settings
- ✅ Remote Plex access can be properly configured

## Files Modified

1. `/projects/Charon/frontend/src/components/AccessListSelector.tsx`
2. `/projects/Charon/frontend/src/components/ProxyHostForm.tsx`

## Rollback Plan

If issues occur, revert to native `<select>` elements, but note that the root cause (pointer-events-none on modal) would need to be addressed separately:
- Option A: Remove `pointer-events-none` from modal container
- Option B: Continue using Radix UI Select (recommended)

## Notes

- The Radix UI Select component was already available in the codebase (ui/Select.tsx)
- No new dependencies were required
- All TypeScript types are properly defined
- Component maintains existing styling and behavior
- Improvements to accessibility as a side benefit
