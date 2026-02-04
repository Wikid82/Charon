# Modal Z-Index Fix Implementation Plan

**Date**: 2026-02-04  
**Issue**: Modal overlay z-index conflicts preventing dropdown interactions  
**Scope**: 7 P0 critical modal components with native `<select>` elements  
**Estimated Time**: 6-8 hours total implementation + testing  

---

## REQUIREMENTS (EARS Format)

**WHEN** a user opens any modal with dropdown elements, **THE SYSTEM SHALL** allow dropdown interaction without z-index conflicts

**WHEN** a user clicks on native select elements in modals, **THE SYSTEM SHALL** display dropdown options and allow selection  

**WHEN** a user clicks outside modal content, **THE SYSTEM SHALL** close the modal (preserve existing behavior)

**WHEN** a user presses the Escape key, **THE SYSTEM SHALL** close the modal (preserve existing behavior)

---

## TECHNICAL DESIGN

### Root Cause Analysis

All affected modals use the same problematic pattern:
```tsx
<div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
  <div className="bg-dark-card ...">
    <form>
      <select> {/* BROKEN: Dropdown can't render above z-50 overlay */} </select>
    </form>
  </div>
</div>
```

### Solution: 3-Layer Modal Architecture

Replace single-layer pattern with 3 distinct layers:

```tsx
{/* FIXED: 3-layer architecture */}
<>
  {/* Layer 1: Background overlay (z-40) - Click to close */}
  <div className="fixed inset-0 bg-black/50 z-40" onClick={onCancel} />
  
  {/* Layer 2: Container (z-50, pointer-events-none) - Centers content */}
  <div className="fixed inset-0 flex items-center justify-center p-4 pointer-events-none z-50">
    
    {/* Layer 3: Content (pointer-events-auto) - Interactive form */}
    <div className="bg-dark-card ... pointer-events-auto">
      <form className="pointer-events-auto">
        <select> {/* WORKS: Dropdown renders above all layers */} </select>
      </form>
    </div>
  </div>
</>
```

**Key CSS Classes**:
- `pointer-events-none` on container: Passes clicks through to overlay
- `pointer-events-auto` on content: Re-enables form interactions
- Background overlay at `z-40`: Lower than form container at `z-50`
3. **Native Dropdown Conflict**: Native HTML `<select>` elements spawn dropdown menus outside the normal DOM flow
4. **Z-Index Stacking Problem**: The dropdown menu may render at a z-index below the parent overlay's z-50, making it invisible or unclickable
5. **Pointer Events**: The overlay's `fixed inset-0` may block pointer events from reaching child dropdowns depending on browser implementation

**Evidence from Related Components**:

The ConfigReloadOverlay component in [frontend/src/components/LoadingStates.tsx](frontend/src/components/LoadingStates.tsx#L251-L287) uses an identical problematic pattern:

```tsx
<div className="fixed inset-0 bg-slate-900/70 backdrop-blur-sm flex items-center justify-center z-50">
  {/* Content */}
</div>
```

This is so well-known as a problem that test helpers document workarounds for it:

**[tests/utils/ui-helpers.ts](tests/utils/ui-helpers.ts#L247-L275)**:
```typescript
// ✅ FIX P0: Wait for ConfigReloadOverlay to disappear before clicking
// The ConfigReloadOverlay component (z-50) intercepts pointer events
// during Caddy config reloads, blocking all interactions
const overlay = page.locator('[data-testid="config-reload-overlay"]');
await overlay.waitFor({ state: 'hidden', timeout: 10000 });
```

**CHANGELOG.md Reference**:
- Line 75: "E2E Tests: Added `data-testid="config-reload-overlay"` to `ConfigReloadOverlay` component"

This confirms the pattern is known to cause interaction problems.

---

### Secondary Issues

#### Issue 2: Native Select Elements vs Modal Z-Index Conflict

**Affected Components**:

**ACL Dropdown** ([frontend/src/components/AccessListSelector.tsx](frontend/src/components/AccessListSelector.tsx#L21-L32)):
```tsx
<select
  value={value || 0}
  onChange={(e) => onChange(parseInt(e.target.value) || null)}
  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
>
  <option value={0}>No Access Control (Public)</option>
  {accessLists?.filter((acl) => acl.enabled).map((acl) => (
    <option key={acl.id} value={acl.id}>
      {acl.name} ({acl.type.replace('_', ' ')})
    </option>
  ))}
</select>
```

**Security Headers Dropdown** ([frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx#L797-L830)):
```tsx
<select
  value={formData.security_header_profile_id || 0}
  onChange={e => {
    const value = e.target.value === "0" ? null : parseInt(e.target.value) || null
    setFormData({ ...formData, security_header_profile_id: value })
  }}
  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
>
  <option value={0}>None (No Security Headers)</option>
  <optgroup label="Quick Presets">
    {securityProfiles?.filter(p => p.is_preset)
      .sort((a, b) => a.security_score - b.security_score)
      .map(profile => (
        <option key={profile.id} value={profile.id}>
          {profile.name} (Score: {profile.security_score}/100)
        </option>
      ))}
  </optgroup>
  {/* ... additional profiles ... */}
</select>
```

**Problems**:
- Native `<select>` elements attempt to render dropdown menus at the browser's native overlay layer
- These native menus may be rendered at z-index below the parent modal's `z-50`
- No explicit z-index management for the dropdown menus
- No `pointer-events-auto` declaration to explicitly allow click propagation

#### Issue 3: Missing Pointer-Events Cascade

**Problem**: Form elements don't have explicit `pointer-events-auto` to override potential parent restrictions.

**Related Code** ([frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx#L525)):
```tsx
<form onSubmit={handleSubmit} className="p-6 space-y-6">
  {/* Dropdowns here - but no pointer-events-auto */}
</form>
```

---

## Component Architecture & Event Handler Analysis

### ACL Dropdown (AccessListSelector Component)

**File**: [frontend/src/components/AccessListSelector.tsx](frontend/src/components/AccessListSelector.tsx)

**Component Structure**:
```tsx
interface AccessListSelectorProps {
  value: number | null;
  onChange: (id: number | null) => void;
}

export default function AccessListSelector({ value, onChange }: AccessListSelectorProps) {
  const { data: accessLists } = useAccessLists();
  
  const selectedACL = accessLists?.find((acl) => acl.id === value);

  return (
    <div>
      <label className="block text-sm font-medium text-gray-300 mb-2">
        Access Control List
        <span className="text-gray-500 font-normal ml-2">(Optional)</span>
      </label>
      <select
        value={value || 0}
        onChange={(e) => onChange(parseInt(e.target.value) || null)}  // ✅ Handler is defined
        className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
      >
        <option value={0}>No Access Control (Public)</option>
        {accessLists?.filter((acl) => acl.enabled).map((acl) => (
          <option key={acl.id} value={acl.id}>
            {acl.name} ({acl.type.replace('_', ' ')})
          </option>
        ))}
      </select>
      {/* ... display selected ACL details ... */}
    </div>
  );
}
```

**Event Handler Analysis**:
- ✅ `onChange` handler is correctly defined
- ✅ Handler parses the selected value correctly
- ✅ Handler calls parent's `onChange` prop with the correct value
- ❌ **THE PROBLEM**: Parent modal overlay prevents click event from reaching the `<select>` element in the first place

**State Management Flow**:
```
AccessListSelector
  ↓ onChange={(e) => onChange(...)}
  ↓
ProxyHostForm (parent)
  ↓ onChange={(id) => setFormData({...formData, access_list_id: id})}
  ↓
ProxyHosts page (grandparent)
  ↓ handleSubmit() → updateHost()/createHost()
```

**State Flow Status**: ✅ Correct - No issues with state management or event handlers

---

### Security Headers Dropdown (ProxyHostForm Component)

**File**: [frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx#L797-L830)

**Component Structure**:
```tsx
export default function ProxyHostForm({ host, onSubmit, onCancel }: ProxyHostFormProps) {
  const [formData, setFormData] = useState<ProxyHostFormState>({
    security_header_profile_id: host?.security_header_profile_id || null,
    // ... other fields
  });
  
  const { data: securityProfiles } = useSecurityHeaderProfiles();

  return (
    // ... modal structure ...
    <select
      value={formData.security_header_profile_id || 0}
      onChange={e => {
        const value = e.target.value === "0" ? null : parseInt(e.target.value) || null
        setFormData({ ...formData, security_header_profile_id: value })  // ✅ Handler is defined
      }}
      className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
    >
      <option value={0}>None (No Security Headers)</option>
      <optgroup label="Quick Presets">
        {securityProfiles?.filter(p => p.is_preset)
          .sort((a, b) => a.security_score - b.security_score)
          .map(profile => (
            <option key={profile.id} value={profile.id}>
              {profile.name} (Score: {profile.security_score}/100)
            </option>
          ))}
      </optgroup>
      {/* ... more profiles ... */}
    </select>
  );
}
```

**Event Handler Analysis**:
- ✅ `onChange` handler is correctly defined
- ✅ Handler correctly parses "0" as null (unselect)
- ✅ Handler correctly parses numeric values
- ✅ Handler updates `formData` state correctly
- ❌ **THE PROBLEM**: Parent modal overlay prevents click event from reaching the `<select>` element

**State Management Flow**:
```
ProxyHostForm
  ↓ formData.security_header_profile_id (state)
  ↓ onChange: setFormData({...formData, security_header_profile_id: value})
  ↓
ProxyHosts page (parent)
  ↓ handleSubmit() → updateHost()/createHost()
```

**State Flow Status**: ✅ Correct - No issues with state management or event handlers

---

## Detailed Component Context

### ProxyHostForm Modal Wrapper

**File**: [frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx#L514-L525)

**Current Structure** (PROBLEMATIC):
```tsx
return (
  <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
    {/* ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        PROBLEM: This div covers entire viewport with pointer events at z-50.
        It contains the form, which contains the dropdowns.
        Native select menus might render below this z-50, making them unclickable.
    */}
    <div className="bg-dark-card rounded-lg border border-gray-800 max-w-2xl w-full max-h-[90vh] overflow-y-auto">
      <div className="p-6 border-b border-gray-800">
        <h2 className="text-2xl font-bold text-white">
          {host ? 'Edit Proxy Host' : 'Add Proxy Host'}
        </h2>
      </div>

      <form onSubmit={handleSubmit} className="p-6 space-y-6">
        {/* ... form fields including dropdowns ... */}
        <AccessListSelector
          value={formData.access_list_id || null}
          onChange={id => setFormData({ ...formData, access_list_id: id })}
        />
        
        <select>
          {/* Security Headers dropdown */}
        </select>
      </form>
    </div>
  </div>
)
```

**Z-Index Analysis**:
- Outer wrapper: `z-50` with `fixed inset-0` (covers viewport)
- Native `<select>` dropdown: Rendered at default z-index (likely less than 50)
- Result: Dropdown appears behind or is blocked by the overlay

---

### Parent Component: ProxyHosts Page

**File**: [frontend/src/pages/ProxyHosts.tsx](frontend/src/pages/ProxyHosts.tsx#L633-L641)

**How It Renders the Form**:
```tsx
export default function ProxyHosts() {
  const [showForm, setShowForm] = useState(false)
  const [editingHost, setEditingHost] = useState<ProxyHost | undefined>()

  const handleAdd = () => {
    setEditingHost(undefined)
    setShowForm(true)
  }

  const handleEdit = (host: ProxyHost) => {
    setEditingHost(host)
    setShowForm(true)
  }

  const handleSubmit = async (data: Partial<ProxyHost>) => {
    if (editingHost) {
      await updateHost(editingHost.uuid, data)
    } else {
      await createHost(data)
    }
    setShowForm(false)
    setEditingHost(undefined)
  }

  // In render:
  return (
    <>
      {showForm && (
        <ProxyHostForm
          host={editingHost}
          onSubmit={handleSubmit}
          onCancel={() => {
            setShowForm(false)
            setEditingHost(undefined)
          }}
        />
      )}
    </>
  )
}
```

**Parent's Role**: ✅ Correct - parent just manages state and renders form conditionally

---

## CSS and Styling Analysis

### Modal Overlay Classes Breakdown

**Problematic CSS Classes** in ProxyHostForm:

```tsx
className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50"
           ^^^^^^^                                                            ^^^^
           fixed positioning with full viewport coverage              Z-index layer
```

**Class Meanings**:
- `fixed`: Position relative to viewport
- `inset-0`: Applies top, right, bottom, left = 0 (covers entire screen)
- `bg-black/50`: Semi-transparent black background (50% opacity)
- `flex items-center justify-center`: Centers child content
- `p-4`: Padding
- `z-50`: Tailwind z-index value (z-index: 50 in actual CSS)

**CSS Cascade Issue**:
```css
/* Outer wrapper (the problem) */
.fixed.inset-0.z-50 {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 50;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Native select inside - inherits parent's stacking context */
select {
  /* No explicit z-index */
  /* Dropdown menu spawned at default z-index, likely below parent's z-50 */
}
```

**Why Native Selects Fail**:
1. Browser spawns `<select>` dropdown menu in a popup layer
2. This popup layer has default z-index (often 0 or auto)
3. Parent's z-index: 50 creates a new stacking context
4. Dropdown menu tries to render in this context
5. If dropdown z-index < parent z-index, it's hidden behind the overlay

---

## Test Evidence & References

### E2E Test File Evidence

**File**: [tests/integration/proxy-acl-integration.spec.ts](tests/integration/proxy-acl-integration.spec.ts#L130-L155)

**Test Attempting to Select ACL** (currently failing):
```typescript
await test.step('Assign IP-based whitelist ACL to proxy host', async () => {
  // Find the proxy host row and click edit
  const proxyRow = page.locator(SELECTORS.proxyRow).filter({
    hasText: createdDomain,
  });
  await expect(proxyRow).toBeVisible();

  const editButton = proxyRow.locator(SELECTORS.proxyEditBtn).first();
  await editButton.click();
  await waitForModal(page, /edit|proxy/i);

  // Try to select ACL from dropdown
  const aclDropdown = page.locator(SELECTORS.aclSelectDropdown);
  const aclCombobox = page.getByRole('combobox')
    .filter({ hasText: /No Access Control|whitelist/i });

  // Build the pattern to match the ACL name
  const aclNamePattern = aclConfig.name;

  if (await aclDropdown.isVisible()) {
    const option = aclDropdown.locator('option').filter({ hasText: aclNamePattern });
    const optionValue = await option.getAttribute('value');
    if (optionValue) {
      await aclDropdown.selectOption({ value: optionValue });
      // ❌ THIS FAILS: selectOption doesn't work because overlay blocks interaction
    }
  }
});
```

**Test Selector References** ([tests/integration/proxy-acl-integration.spec.ts](tests/integration/proxy-acl-integration.spec.ts#L52-L54)):
```typescript
const SELECTORS = {
  aclSelectDropdown: '[data-testid="acl-select"], select[name="access_list_id"]',
  // ...
}
```

---

### Known Pattern Reference: ConfigReloadOverlay

**Similar problematic pattern** in [frontend/src/components/LoadingStates.tsx](frontend/src/components/LoadingStates.tsx#L251-L287):

```tsx
export function ConfigReloadOverlay({
  message = 'Ferrying configuration...',
  submessage = 'Charon is crossing the Styx',
  type = 'charon',
}: {
  message?: string
  submessage?: string
  type?: 'charon' | 'coin' | 'cerberus'
}) {
  return (
    <div className="fixed inset-0 bg-slate-900/70 backdrop-blur-sm flex items-center justify-center z-50"
         data-testid="config-reload-overlay">
      {/* Content */}
    </div>
  )
}
```

**Test Helper Workaround** ([tests/utils/ui-helpers.ts](tests/utils/ui-helpers.ts#L247-L275)):

```typescript
/**
 * ✅ FIX P0: Wait for ConfigReloadOverlay to disappear before clicking
 * The ConfigReloadOverlay component (z-50) intercepts pointer events
 * during Caddy config reloads, blocking all interactions.
 */
export async function clickSwitch(
  locator: Locator,
  options: SwitchOptions = {}
): Promise<void> {
  // ...
  const page = locator.page();
  const overlay = page.locator('[data-testid="config-reload-overlay"]');
  
  // Wait for overlay to disappear before clicking
  await overlay.waitFor({ state: 'hidden', timeout: 10000 }).catch(() => {
    // Overlay not present - continue
  });
  // ... click the element ...
}
```

This confirms the pattern is **known to cause problems** and requires workarounds.

---

## Detailed Fix Plan

### Solution Strategy

**Overall Approach**: Restructure the modal DOM to separate the overlay from the form content, using proper z-index layering and pointer-events management.

---

### Fix 1: Restructure Modal Z-Index Hierarchy (CRITICAL)

**File to Modify**: [frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx#L514-L525)

**Current Code** (Lines 514-525):
```tsx
return (
  <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
    <div className="bg-dark-card rounded-lg border border-gray-800 max-w-2xl w-full max-h-[90vh] overflow-y-auto">
      <div className="p-6 border-b border-gray-800">
        <h2 className="text-2xl font-bold text-white">
          {host ? 'Edit Proxy Host' : 'Add Proxy Host'}
        </h2>
      </div>

      <form onSubmit={handleSubmit} className="p-6 space-y-6">
```

**Proposed Fix**:
```tsx
return (
  <>
    {/* Layer 1: Overlay (z-40) - separate from form to allow form to float above */}
    <div className="fixed inset-0 bg-black/50 z-40" onClick={onCancel} />
    
    {/* Layer 2: Form container (z-50) with pointer-events-none on wrapper */}
    <div className="fixed inset-0 flex items-center justify-center p-4 pointer-events-none z-50">
      {/* Layer 3: Form content with pointer-events-auto to re-enable interactions */}
      <div className="bg-dark-card rounded-lg border border-gray-800 max-w-2xl w-full max-h-[90vh] overflow-y-auto pointer-events-auto">
        <div className="p-6 border-b border-gray-800">
          <h2 className="text-2xl font-bold text-white">
            {host ? 'Edit Proxy Host' : 'Add Proxy Host'}
          </h2>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6 pointer-events-auto">
```

**Why This Works**:

1. **Separate Overlay Layer** (`z-40`):
   - Handles backdrop click-to-close
   - Doesn't interfere with form's z-index stacking
   - Can receive click events without blocking

2. **Form Container Layer** (`z-50` with `pointer-events-none`):
   - Higher z-index than overlay ensures form is on top visually
   - `pointer-events-none` means this div doesn't capture clicks
   - Clicks pass through to children

3. **Form Content** (`pointer-events-auto`):
   - Re-enables pointer events on actual form elements
   - Allows native `<select>` dropdowns to work properly
   - Native dropdown menus now render above both overlay and form wrapper

**Z-Index Diagram**:
```
z-50:  Form Container (pointer-events-none)
         ├─ Form Content (pointer-events-auto)
         │   ├─ Input fields
         │   └─ Select dropdowns ← DROPDOWN MENUS RENDER HERE
z-40:  Overlay (pointer-events-auto)
```

---

### Fix 2: Ensure Form Element Pointer-Events

**File to Modify**: [frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx#L525)

**Current Code**:
```tsx
<form onSubmit={handleSubmit} className="p-6 space-y-6">
```

**Updated Code**:
```tsx
<form onSubmit={handleSubmit} className="p-6 space-y-6 pointer-events-auto">
```

**Reasoning**:
- Explicitly declares that this form element and its children should receive pointer events
- Provides clear documentation of intent in the code
- Ensures no CSS inheritance quirks prevent dropdown interaction

---

### Fix 3: Add CSS Utilities for Future Select Elements (OPTIONAL ENHANCEMENT)

**File to Modify**: [frontend/src/index.css](frontend/src/index.css)

**Add CSS Rule** (at appropriate location in utilities section):
```css
/* Ensure native select dropdowns render above overlays */
@layer components {
  /* Native select dropdown positioning */
  select {
    position: relative;
    /* Allow natural z-index stacking - browser will handle dropdown placement */
  }
}
```

**Note**: This is optional but provides defensive CSS for future modals using the same pattern.

---

## Implementation Checklist

### Pre-Implementation
- [ ] Read the entire fix plan
- [ ] Understand the z-index layering issue
- [ ] Understand pointer-events CSS property
- [ ] Have access to [frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx)

### Implementation Steps

#### Step 1: Restructure Modal HTML (5-10 minutes)
- [ ] Open [frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx)
- [ ] Find the return statement (around line 514)
- [ ] Split the single overlay-form div into three layers as specified above
- [ ] Change outer div: `fixed inset-0 bg-black/50 flex ... z-50` → `fixed inset-0 bg-black/50 z-40`
- [ ] Add new container: `fixed inset-0 flex ... z-50 pointer-events-none`
- [ ] Add form wrapper: `pointer-events-auto` to content div

#### Step 2: Update Form Element (1-2 minutes)
- [ ] Add `pointer-events-auto` to `<form>` element className
- [ ] Verify className is properly formatted: `"p-6 space-y-6 pointer-events-auto"`

#### Step 3: Optional - Add CSS Utilities (2-3 minutes)
- [ ] Open [frontend/src/index.css](frontend/src/index.css)
- [ ] Add defensive CSS for select elements (optional)

### Testing (30-60 minutes)

#### Manual Testing
- [ ] Start the dev server: `npm run dev`
- [ ] Navigate to `/proxy-hosts`
- [ ] Click "Add Proxy Host" button
- [ ] Verify ACL dropdown opens and responds to clicks
- [ ] Select an ACL option
- [ ] Verify selection appears in dropdown
- [ ] Click on Security Headers dropdown
- [ ] Verify dropdown opens and responds to clicks
- [ ] Select a security profile
- [ ] Verify selection appears in dropdown
- [ ] Fill in remaining form fields
- [ ] Click Save
- [ ] Verify form submits successfully
- [ ] Repeat for "Edit Proxy Host" page

#### Browser Testing
- [ ] Test on Chrome/Chromium
- [ ] Test on Firefox
- [ ] Test on Safari (if available)
- [ ] Test on Edge (if available)

#### E2E Test Execution
- [ ] Run: `npm run test:e2e`
- [ ] Verify all tests pass
- [ ] Specifically verify proxy-acl-integration tests pass

#### Unit Test Execution
- [ ] Run: `npm run test`
- [ ] Specifically run ProxyHostForm tests:
  - `frontend/src/components/__tests__/ProxyHostForm.test.tsx`
- [ ] Verify all tests pass

#### Accessibility Verification
- [ ] Test keyboard navigation through form (Tab key)
- [ ] Verify you can navigate to dropdowns with Tab
- [ ] Verify you can open dropdowns with Enter/Space
- [ ] Verify you can navigate options with arrow keys
- [ ] Verify you can select with Enter

### Code Review Checklist
- [ ] Changes are minimal and focused
- [ ] No unintended side effects
- [ ] All dropdowns tested
- [ ] Modal close behavior still works
- [ ] No console errors
- [ ] No console warnings
- [ ] Consistent with existing code style

### Merge Checklist
- [ ] All tests passing
- [ ] All manual testing complete
- [ ] Code review approved
- [ ] Commit message references this bug report
- [ ] No merge conflicts

---

## Testing Strategy

### Unit Tests (Frontend)

**Test File**: [frontend/src/components/__tests__/ProxyHostForm.test.tsx](frontend/src/components/__tests__/ProxyHostForm.test.tsx)

**Commands**:
```bash
# Run all ProxyHostForm tests
npm run test -- ProxyHostForm.test.tsx

# Run specific test
npm run test -- ProxyHostForm.test.tsx -t "ACL dropdown"
```

**Expected Behavior**: All existing tests should pass without modification

---

### E2E Tests (Playwright)

**Test File**: [tests/integration/proxy-acl-integration.spec.ts](tests/integration/proxy-acl-integration.spec.ts#L130-L155)

**Command**:
```bash
# Run ACL integration tests
npx playwright test proxy-acl-integration.spec.ts

# Run specific test
npx playwright test proxy-acl-integration.spec.ts -g "Assign IP-based"
```

**Expected Results**:
- ✅ `selectOption()` method works on ACL dropdown
- ✅ Selection persists in form state
- ✅ Form submits with selected ACL

---

### Manual Verification Checklist

**Location**: ProxyHosts page (`/proxy-hosts`)

**Scenario 1: Add Proxy Host with ACL**
- [ ] Click "Add Proxy Host"
- [ ] Modal opens
- [ ] Fill in required fields (name, domain, forward_host, etc.)
- [ ] Click ACL dropdown
- [ ] Dropdown opens and is visible above modal
- [ ] Select an ACL from the list
- [ ] Selection appears in dropdown
- [ ] Click Save
- [ ] Form submits successfully
- [ ] New host appears in table with ACL badge

**Scenario 2: Add Proxy Host with Security Headers**
- [ ] Click "Add Proxy Host"
- [ ] Modal opens
- [ ] Fill in required fields
- [ ] Click Security Headers dropdown
- [ ] Dropdown opens and is visible above modal
- [ ] Select a security profile
- [ ] Selection appears in dropdown
- [ ] Click Save
- [ ] Form submits successfully
- [ ] Verify security headers applied (check browser DevTools)

**Scenario 3: Edit Proxy Host - Change ACL**
- [ ] Click Edit button on existing proxy host
- [ ] Modal opens with current values
- [ ] Click ACL dropdown
- [ ] Dropdown opens and shows current selection
- [ ] Select different ACL
- [ ] Selection updates in dropdown
- [ ] Click Save
- [ ] Verify API request includes new ACL
- [ ] Verify ACL badge updates in table

**Scenario 4: Modal Close Behavior**
- [ ] Open Add/Edit form
- [ ] Click overlay (dark area outside form)
- [ ] Modal closes without saving
- [ ] Click Edit again
- [ ] Open form
- [ ] Click X button in top right
- [ ] Modal closes without saving

---

## Risk Assessment

### Low-Risk Changes ✅
- ✅ Adding `pointer-events-auto` classes (non-breaking CSS)
- ✅ Restructuring div hierarchy (same functionality, different structure)
- ✅ Adding `onClick={onCancel}` to overlay (alternative way to close, same result)

### Medium-Risk Areas ⚠️
- ⚠️ Changing z-index values (could affect other components using same classes)
- ⚠️ Splitting overlay and form (must maintain visual appearance)
- ⚠️ Using Fragment (`<>`) instead of wrapper div (may affect animation/transition)

### Risk Mitigation
- Test on all browsers
- Run full E2E test suite
- Manual testing on add and edit pages
- Visual regression testing (compare before/after)
- Check for similar patterns in other components

---

## Success Criteria

### Functional Requirements ✅

**ACL Dropdown**:
- ✅ Dropdown opens on click
- ✅ Options are visible and selectable
- ✅ Selection updates form state correctly
- ✅ Selected value persists when saving proxy host
- ✅ ACL is applied to proxy host in backend

**Security Headers Dropdown**:
- ✅ Dropdown opens on click
- ✅ Options are visible and selectable
- ✅ Selection updates form state correctly
- ✅ Selected value persists when saving proxy host
- ✅ Security headers are applied to proxy host in backend

**Modal Behavior**:
- ✅ Form submits successfully with selected dropdowns
- ✅ Modal closes on successful save
- ✅ Modal can be closed by clicking overlay
- ✅ Modal can be closed by clicking Cancel button
- ✅ Modal can be closed by clicking X button

### Non-Functional Requirements ✅
- ✅ No performance regression
- ✅ No accessibility regression
- ✅ Keyboard navigation works (Tab, Arrow, Enter)
- ✅ No console errors or warnings
- ✅ Consistent with existing UI patterns

### Testing Requirements ✅
- ✅ All existing unit tests pass without modification
- ✅ All E2E tests pass (particularly proxy-acl-integration)
- ✅ Manual testing on all supported browsers
- ✅ No JavaScript errors in console

---

## File Summary Table

| File | Action | Complexity | Impact |
|------|--------|-----------|--------|
| [frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx#L514-L525) | **MUST MODIFY** - Restructure modal z-index | Medium | Fixes dropdown interaction |
| [frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx#L525) | **MUST MODIFY** - Add pointer-events-auto to form | Low | Ensures event propagation |
| [frontend/src/components/AccessListSelector.tsx](frontend/src/components/AccessListSelector.tsx) | **REVIEW ONLY** - No changes needed | N/A | Verify handler is correct |
| [frontend/src/pages/ProxyHosts.tsx](frontend/src/pages/ProxyHosts.tsx#L633-L641) | **REVIEW ONLY** - No changes needed | N/A | Verify parent rendering is correct |
| [frontend/src/index.css](frontend/src/index.css) | OPTIONAL - Add select CSS utilities | Low | Future-proofs similar issues |
| [tests/integration/proxy-acl-integration.spec.ts](tests/integration/proxy-acl-integration.spec.ts) | **VERIFY** - Run tests after fix | N/A | Confirm fix resolves blocking |

---

## Related Patterns to Monitor

### ConfigReloadOverlay Pattern

The ConfigReloadOverlay component ([frontend/src/components/LoadingStates.tsx](frontend/src/components/LoadingStates.tsx#L251-L287)) has the same problematic z-index pattern. After fixing ProxyHostForm, consider applying the same solution to ConfigReloadOverlay for consistency.

### Other Modal Forms

Scan the codebase for other modal-based forms that might have the same issue:
- Create/Edit dialogs
- Settings forms
- Configuration modals

Check if they use the same `fixed inset-0 z-50` pattern and apply fixes as needed.

---

## Timeline Estimate

| Task | Time | Status |
|------|------|--------|
| Understand problem | 30 min | ✅ Done |
| Implement fixes | 15-20 min | → Next |
| Manual testing | 45 min | → Next |
| E2E test execution | 10 min | → Next |
| Code review | 15 min | → Next |
| **Total** | **~2 hours** | → Estimated |

---

## Commit Message Template

```
fix: allow ACL and Security Headers dropdown selection in proxy host form

The ProxyHostForm modal used a full-screen overlay with z-index: 50 that
blocked pointer events to child elements. Native <select> dropdowns
couldn't open or receive clicks because the overlay intercepted events.

Changes:
- Restructured modal DOM to separate overlay (z-40) from form (z-50)
- Added pointer-events-none to form container, pointer-events-auto to
  form content to re-enable child interactions
- This allows native <select> dropdowns to render and respond to clicks

Fixes dropdown interaction on both Add and Edit Proxy Host pages.

Related: proxy-acl-integration.spec.ts tests now pass
```

---

## Appendix: Z-Index Reference

**Tailwind Z-Index Scale**:
```
z-0:   0
z-10:  10
z-20:  20
z-30:  30
z-40:  40     ← Overlay (backdrop)
z-50:  50     ← Form (content)
z-auto: auto
```

**Browser Native Elements**:
- Select dropdown menu: auto (default z-index)
- Tooltip: varies by browser
- Popup menu: varies by browser

---

**Plan Status**: ✅ Complete and Ready for Implementation  
**Estimated Time to Fix**: 2 hours (including testing)  
**Priority**: CRITICAL (blocks ACL and Security Headers assignment)  
**Severity**: HIGH (UI feature completely non-functional)

