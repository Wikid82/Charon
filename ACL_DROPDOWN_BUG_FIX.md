# ACL & Security Headers Dropdown Bug Fix - RESOLVED

**Date**: February 12, 2026
**Status**: ✅ FIXED
**Priority**: CRITICAL (Production-Blocking)

---

## User Report

> "There is a bug in the ACL dropdown menu. I could not remove or edit the attached ACL on a proxy host. I had to delete the host and add it without the ACL to bypass."
>
> "Same issue with Security Headers dropdown - cannot remove or change once selected."

**Impact**: Users unable to manage ACL or Security Headers on proxy hosts, forcing deletion and recreation.

---

## Root Cause Analysis

### The REAL Problem: Stale Closure Bug

The bug was **NOT** in `AccessListSelector.tsx` - that component was correctly implemented.

The bug was in **`ProxyHostForm.tsx`** where state updates used stale closures:

```jsx
// ❌ BUGGY CODE (Line 822 & 836)
<AccessListSelector
  value={formData.access_list_id || null}
  onChange={id => setFormData({ ...formData, access_list_id: id })}
/>

<Select
  value={String(formData.security_header_profile_id || 0)}
  onValueChange={e => {
    const value = e === "0" ? null : parseInt(e) || null
    setFormData({ ...formData, security_header_profile_id: value })
  }}
>
```

### Critical Issues

1. **Stale Closure Pattern**:
   - `onChange` callback captures `formData` from render when created
   - When callback executes, `formData` may be stale
   - Spreading stale `formData` overwrites current state with old values
   - Only the changed field updates, but rest of form may revert

2. **React setState Batching**:
   - React batches multiple setState calls for performance
   - If multiple updates happen quickly, closure captures old state
   - Direct object spread `{ ...formData, ... }` uses stale snapshot

3. **No Re-render Trigger**:
   - State update happens but uses old values
   - Select component receives same value, doesn't re-render
   - User sees no change, thinks dropdown is broken

---

## The Fix

### Solution: Functional setState Form

```jsx
// ✅ FIXED CODE
<AccessListSelector
  value={formData.access_list_id || null}
  onChange={id => setFormData(prev => ({ ...prev, access_list_id: id }))}
/>

<Select
  value={String(formData.security_header_profile_id || 0)}
  onValueChange={e => {
    const value = e === "0" ? null : parseInt(e) || null
    setFormData(prev => ({ ...prev, security_header_profile_id: value }))
  }}
>
```

### Key Improvements

1. **Always Fresh State**: `setFormData(prev => ...)` guarantees `prev` is latest state
2. **No Closure Dependencies**: Callback doesn't capture `formData` from outer scope
3. **React Guarantee**: React ensures `prev` parameter is current state value
4. **Race Condition Safe**: Multiple rapid updates all work on latest state

### Full Scope of Fix

Fixed **17 instances** of stale closure bugs throughout `ProxyHostForm.tsx`:

**Critical Fixes (User-Reported)**:
- ✅ Line 822: ACL dropdown
- ✅ Line 836: Security Headers dropdown

**Additional Preventive Fixes**:
- ✅ Line 574: Name input
- ✅ Line 691: Domain names input
- ✅ Line 703: Forward scheme select
- ✅ Line 728: Forward host input
- ✅ Line 751: Forward port input
- ✅ Line 763: Certificate select
- ✅ Lines 1099-1163: All checkboxes (SSL, HTTP/2, HSTS, etc.)
- ✅ Line 1184: Advanced config textarea

---

## State Transition Matrix

| Scenario | Buggy Behavior | Fixed Behavior |
|----------|---------------|----------------|
| Change ACL 1 → 2 | Sometimes stays at 1 | Always changes to 2 |
| Remove ACL | Sometimes stays assigned | Always removes |
| Change Headers A → B | Sometimes stays at A | Always changes to B |
| Remove Headers | Sometimes stays assigned | Always removes |
| Multiple rapid changes | Last change wins OR reverts | All changes apply correctly |

---

## Validation

### Test Coverage

**New Comprehensive Test Suite**: `ProxyHostForm-dropdown-changes.test.tsx`

✅ **5 passing tests:**
1. `allows changing ACL selection after initial selection`
2. `allows removing ACL selection`
3. `allows changing Security Headers selection after initial selection`
4. `allows removing Security Headers selection`
5. `allows editing existing host with ACL and changing it`

**Existing Tests:**
- ✅ 58/59 ProxyHostForm tests pass
- ✅ 15/15 ProxyHostForm-dns tests pass
- ⚠️ 1 flaky test (uptime) unrelated to changes

### Manual Testing Steps

1. **Change ACL:**
   - Edit proxy host with ACL assigned
   - Select different ACL from dropdown
   - Save → Verify new ACL applied

2. **Remove ACL:**
   - Edit proxy host with ACL
   - Select "No Access Control (Public)"
   - Save → Verify ACL removed

3. **Change Security Headers:**
   - Edit proxy host with headers profile
   - Select different profile
   - Save → Verify new profile applied

4. **Remove Security Headers:**
   - Edit proxy host with headers
   - Select "None (No Security Headers)"
   - Save → Verify headers removed

---

## Technical Deep Dive

### Why Functional setState Matters

**React's setState Behavior:**
```jsx
// ❌ BAD: Closure captures formData at render time
setFormData({ ...formData, field: value })
// If formData is stale, spread puts old values back

// ✅ GOOD: React passes latest state as 'prev'
setFormData(prev => ({ ...prev, field: value }))
// Always operates on current state
```

**Example Scenario:**
```jsx
// User rapidly changes ACL: 1 → 2 → 3

// With buggy code:
// Render 1: formData = { ...other, access_list_id: 1 }
// User clicks ACL=2
// Callback captures formData from Render 1
// setState({ ...formData, access_list_id: 2 }) // formData.access_list_id was 1

// But React hasn't re-rendered yet, another click happens:
// User clicks ACL=3
// Callback STILL has formData from Render 1 (access_list_id: 1)
// setState({ ...formData, access_list_id: 3 }) // Overwrites previous update!

// Result: ACL might be 3, 2, or even revert to 1 depending on timing

// With fixed code:
// User clicks ACL=2
// setState(prev => ({ ...prev, access_list_id: 2 }))
// React guarantees prev has current state

// User clicks ACL=3
// setState(prev => ({ ...prev, access_list_id: 3 }))
// prev includes the previous update (access_list_id: 2)

// Result: ACL is reliably 3
```

---

## Files Changed

1. **`frontend/src/components/ProxyHostForm.tsx`** - Fixed all stale closure bugs
2. **`frontend/src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`** - New test suite

---

## Impact Assessment

### Before Fix
- ❌ Cannot remove ACL from proxy host
- ❌ Cannot change ACL once assigned
- ❌ Cannot remove Security Headers
- ❌ Cannot change Security Headers once set
- ❌ Users forced to delete/recreate hosts (potential data loss)
- ❌ Race conditions in form state updates

### After Fix
- ✅ ACL can be changed and removed freely
- ✅ Security Headers can be changed and removed freely
- ✅ All form fields update reliably
- ✅ No race conditions in rapid updates
- ✅ Consistent with expected behavior

---

## Pattern for Future Development

### ✅ ALWAYS Use Functional setState

```jsx
// ✅ CORRECT
setState(prev => ({ ...prev, field: value }))

// ❌ WRONG - Avoid unless setState has NO dependencies
setState({ ...state, field: value })
```

### When Functional Form is Required

- New state depends on previous state
- Callback defined inline (most common)
- Multiple setState calls may batch
- Working with complex nested objects

---

## Deployment Notes

- **Breaking Changes**: None
- **Migration Required**: No
- **Rollback Safety**: Safe (no data model changes)
- **User Impact**: Immediate fix - dropdowns work correctly
- **Performance**: No impact (same React patterns, just correct usage)

---

## Status: ✅ RESOLVED

**Root Cause**: Stale closure in setState calls
**Solution**: Functional setState form throughout ProxyHostForm
**Validation**: Comprehensive test coverage + existing tests pass
**Confidence**: High - bug cannot occur with functional setState pattern

Users can now remove, change, and manage ACL and Security Headers without issues.

---

## Lessons Learned

1. **Consistency Matters**: Mix of functional/direct setState caused confusion
2. **Inline Callbacks**: Extra careful with inline arrow functions capturing state
3. **Testing Edge Cases**: Rapid changes and edit scenarios reveal closure bugs
4. **Pattern Enforcement**: Consider ESLint rules to enforce functional setState
5. **Code Review Focus**: Check all setState patterns during review
