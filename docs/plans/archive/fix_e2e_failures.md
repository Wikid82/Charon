# Plan: Fix E2E Test Failures

## Objective
Fix implementation bugs and test logic issues causing failures in `certificates.spec.ts`, `navigation.spec.ts`, and `proxy-acl-integration.spec.ts`.

## Analysis of Failures

### 1. Certificates Test (`tests/core/certificates.spec.ts`)
- **Failure**: Fails to assert "Domain" column header. Received `undefined`.
- **Root Cause**: Race condition. The test attempts to valid header text before the table has finished rendering (likely while in Loading or Empty state).
- **Fix**: explicit wait for the table element to be visible before asserting headers.

### 2. Navigation Test (`tests/core/navigation.spec.ts`)
- **Failure**: Sidebar expected to be hidden on mobile but is detected as visible.
- **Root Cause**: The Sidebar implementation in `Layout.tsx` uses CSS transforms (`-translate-x-full`) to hide the menu on mobile. Playwright's `.toBeVisible()` matcher considers elements with `opacity: 1` and non-zero size as "visible", even if translated off-screen.
- **Fix**: Update the assertion to check that the sidebar is hidden from the viewport OR check for the presence of the `-translate-x-full` class.

### 3. Proxy ACL Integration (`tests/integration/proxy-acl-integration.spec.ts`)
- **Failure**: Timeout waiting for `select[name="access_list_id"]`.
- **Root Cause**: The `AccessListSelector.tsx` component renders a standard `<select>` element but omits the `name` attribute. The test specifically queries by this attribute.
- **Fix**: Add `name="access_list_id"` (and `id="access_list_id"` for accessibility) to the `select` element in `AccessListSelector.tsx`.

## Tasks

### Phase 1: Fix Component Implementation
- [ ] **Task 1.1**: Update `frontend/src/components/AccessListSelector.tsx`
    - Add `name="access_list_id"` to the `<select>` element.
    - Add `id="access_list_id"` to the `<select>` element.

### Phase 2: Fix Test Logic
- [ ] **Task 2.1**: Update `tests/core/certificates.spec.ts`
    - Insert `await expect(page.getByRole('table')).toBeVisible()` before header assertions.
- [ ] **Task 2.2**: Update `tests/core/navigation.spec.ts`
    - Change `.not.toBeVisible()` to `.not.toBeInViewport()` (if available in project Playwright version) or check for class: `await expect(page.getByRole('complementary')).toHaveClass(/-translate-x-full/)`.

### Phase 3: Verification
- [ ] **Task 3.1**: Run affected tests to verify fixes.
    - `npx playwright test tests/core/certificates.spec.ts`
    - `npx playwright test tests/core/navigation.spec.ts`
    - `npx playwright test tests/integration/proxy-acl-integration.spec.ts`

## Files to Modify
- `frontend/src/components/AccessListSelector.tsx`
- `tests/core/certificates.spec.ts`
- `tests/core/navigation.spec.ts`
