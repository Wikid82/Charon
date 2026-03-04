# Phase 6: User Management UI Implementation

> **Status**: Planning Complete
> **Created**: 2026-01-24
> **Estimated Effort**: L (Large) - Initially estimated 40-60 hours, **revised to 16-22 hours**
> **Priority**: P2 - Feature Completeness
> **Tests Targeted**: 19 skipped tests in `tests/settings/user-management.spec.ts`
> **Dependencies**: Phase 5 (TestDataManager Auth Fix) - Infrastructure complete, blocked by environment config

---

## Executive Summary

### Goals

Complete the User Management frontend to enable 19 currently-skipped Playwright E2E tests. This phase implements missing UI components including status badges with proper color classes, role badges, resend invite action, email validation, enhanced modal accessibility, and fixes a React anti-pattern bug.

### Key Finding

**Most UI components already exist.** After thorough analysis, the work is primarily:
1. Verifying existing functionality (toast test IDs already exist)
2. Implementing resend invite action (backend endpoint missing - needs implementation)
3. Adding email format validation with visible error
4. Fixing React anti-pattern in PermissionsModal
5. Verification and unskipping tests

**Revised Effort**: 16-22 hours (pending backend resend endpoint scope).

**Solution**: Add missing test selectors, implement resend invite, add email validation UI, fix React bugs, and systematically unskip tests as they pass.

### Test Count Reconciliation

The original plan stated 22 tests, but verification shows **19 skipped test declarations**. The discrepancy came from counting 4 conditional `test.skip()` calls inside test bodies (not actual test declarations). See Section 2 for the complete inventory.

---

## 1. Current State Analysis

### What EXISTS (in `UsersPage.tsx`)

The Users page at `frontend/src/pages/UsersPage.tsx` already contains substantial functionality:

| Component | Status | Notes |
|-----------|--------|-------|
| User list table | ✅ Complete | Columns: User, Role, Status, Permissions, Enabled, Actions |
| InviteModal | ✅ Complete | Email, role, permission mode, host selection, URL preview |
| PermissionsModal | ✅ Complete | Edit user permissions, host toggle |
| Role badges | ✅ Complete | Purple for admin, blue for user, rounded styling |
| Status indicators | ✅ Complete | Active (green), Pending (yellow), Expired (red) with icons |
| Enable/Disable toggle | ✅ Complete | Switch component per user |
| Delete button | ✅ Complete | Trash2 icon with confirmation |
| Settings/Permissions button | ✅ Complete | For non-admin users |
| React Query mutations | ✅ Complete | All CRUD operations |
| Copy invite link | ✅ Complete | With clipboard API |
| URL preview for invites | ✅ Complete | Shows invite URL before sending |

### What is PARTIALLY IMPLEMENTED

| Item | Issue | Fix Required |
|------|-------|--------------|
| Status badges | Class names may not match test expectations | Add explicit color classes |
| Modal keyboard nav | Escape key handling may be missing | Add keyboard event handler |
| PermissionsModal state init | **React anti-pattern: useState used like useEffect** | Fix to use useEffect (see Section 3.6) |

### What is MISSING

| Item | Description | Effort |
|------|-------------|--------|
| Email validation UI | Client-side format validation with visible error | 2 hours |
| Resend invite action | Button + API for pending users | 6-10 hours (backend missing) |
| Backend resend endpoint | `POST /api/v1/users/{id}/resend-invite` | See Phase 6.4 |

---

## 2. Test Analysis

### Summary: 19 Skipped Tests

**File**: `tests/settings/user-management.spec.ts`

| # | Test Name | Line | Category | Skip Reason | Status |
|---|-----------|------|----------|-------------|--------|
| 1 | should show user status badges | 70 | User List | Status badges styling | ✅ Verify |
| 2 | should display role badges | 110 | User List | Role badges selectors | ✅ Verify |
| 3 | should show pending invite status | 164 | User List | Complex timing | ⚠️ Complex |
| 4 | should open invite user modal | 217 | Invite | Outdated skip comment | ✅ Verify |
| 5 | should validate email format | 283 | Invite | No client validation | 🔧 Implement |
| 6 | should copy invite link | 442 | Invite | Toast verification | ✅ Verify |
| 7 | should open permissions modal | 494 | Permissions | Settings icon | 🔒 Auth blocked |
| 8 | should update permission mode | 538 | Permissions | Base URL auth | 🔒 Auth blocked |
| 9 | should add permitted hosts | 612 | Permissions | Settings icon | 🔒 Auth blocked |
| 10 | should remove permitted hosts | 669 | Permissions | Settings icon | 🔒 Auth blocked |
| 11 | should save permission changes | 725 | Permissions | Settings icon | 🔒 Auth blocked |
| 12 | should enable/disable user | 781 | Actions | TestDataManager | 🔒 Auth blocked |
| 13 | should change user role | 828 | Actions | Not implemented | ❌ Future |
| 14 | should delete user with confirmation | 848 | Actions | Delete button | 🔒 Auth blocked |
| 15 | should resend invite for pending user | 956 | Actions | Not implemented | 🔧 Implement |
| 16 | should be keyboard navigable | 1014 | A11y | Known flaky | ⚠️ Flaky |
| 17 | should require admin role for access | 1091 | Security | Routing design | ℹ️ By design |
| 18 | should show error for regular user access | 1125 | Security | Routing design | ℹ️ By design |
| 19 | should have proper ARIA labels | 1157 | A11y | ARIA incomplete | ✅ Verify |

### Legend

- ✅ Verify: Likely already works, just needs verification
- 🔧 Fix/Implement: Requires small code change
- 🔒 Auth blocked: Blocked by Phase 5 (TestDataManager)
- ⚠️ Complex/Flaky: Timing or complexity issues
- ℹ️ By design: Intentional skip (routing behavior)
- ❌ Future: Feature not prioritized

### Tests Addressable in Phase 6

**Without Auth Fix** (can implement now): 6 tests
- Test 1: Status badges styling (verify only)
- Test 2: Role badges (verify only)
- Test 4: Open invite modal (verify only - button IS implemented)
- Test 5: Email validation
- Test 6: Copy invite link (verify only - toast test IDs already exist)
- Test 19: ARIA labels (verify only)

**With Resend Invite**: 1 test
- Test 15: Resend invite

**After Phase 5 Auth Fix**: 6 tests
- Tests 7-12, 14: Permission/Action tests

### Detailed Test Requirements

#### Test 1: should show user status badges (Line 70)

**Test code**:
```typescript
const statusCell = page.locator('td').filter({
  has: page.locator('span').filter({
    hasText: /active|pending.*invite|invite.*expired/i,
  }),
});

const activeStatus = page.locator('span').filter({ hasText: /^active$/i });

// Expects class to include 'green', 'text-green-400', or 'success'
const hasGreenColor = await activeStatus.first().evaluate((el) => {
  return el.className.includes('green') ||
         el.className.includes('text-green-400') ||
         el.className.includes('success');
});
```

**Current code** (UsersPage.tsx line ~459):
```tsx
<span className="inline-flex items-center gap-1 text-green-400 text-xs">
  <Check className="h-3 w-3" />
  {t('common.active')}
</span>
```

**Analysis**: Current code already includes `text-green-400` class.

**Action**: ✅ **Verify only** - unskip and run test.

#### Test 2: should display role badges (Line 110)

**Test code**:
```typescript
const adminBadge = page.locator('span').filter({ hasText: /^admin$/i });

// Expects 'purple', 'blue', or 'rounded' in class
const hasDistinctColor = await adminBadge.evaluate((el) => {
  return el.className.includes('purple') ||
         el.className.includes('blue') ||
         el.className.includes('rounded');
});
```

**Current code** (UsersPage.tsx line ~445):
```tsx
<span className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${
  user.role === 'admin' ? 'bg-purple-900/30 text-purple-400' : 'bg-blue-900/30 text-blue-400'
}`}>
  {user.role}
</span>
```

**Analysis**: ✅ Classes include `rounded` and `purple`/`blue`.

**Action**: ✅ **Verify only** - unskip and run test.

#### Test 4: should open invite user modal (Line 217)

**Test code**:
```typescript
const inviteButton = page.getByRole('button', { name: /invite.*user/i });
await expect(inviteButton).toBeVisible();
await inviteButton.click();
// Verify modal is visible
const modal = page.getByRole('dialog');
await expect(modal).toBeVisible();
```

**Current state**: ✅ Invite button IS implemented in UsersPage.tsx. The skip comment is outdated.

**Action**: ✅ **Verify only** - unskip and run test.

#### Test 5: should validate email format (Line 283)

**Test code**:
```typescript
const sendButton = page.getByRole('button', { name: /send.*invite/i });
const isDisabled = await sendButton.isDisabled();
// OR error message shown
const errorMessage = page.getByText(/invalid.*email|email.*invalid|valid.*email/i);
```

**Current code**: Button disabled when `!email`, but no format validation visible.

**Action**: 🔧 **Implement** - Add email regex validation with error display.

#### Test 6: should copy invite link (Line 442)

**Test code**:
```typescript
const copiedToast = page.locator('[data-testid="toast-success"]').filter({
  hasText: /copied|clipboard/i,
});
```

**Current state**: ✅ Toast component already has `data-testid={toast-${toast.type}}` at `Toast.tsx:31`.

**Action**: ✅ **Verify only** - unskip and run test. No code changes needed.

#### Test 15: should resend invite for pending user (Line 956)

**Test code**:
```typescript
const resendButton = page.getByRole('button', { name: /resend/i });
await resendButton.first().click();
await waitForToast(page, /sent|resend/i, { type: 'success' });
```

**Current state**: ❌ Resend action not implemented.

**Action**: 🔧 **Implement** - Add resend button for pending users + API call.

#### Test 19: should have proper ARIA labels (Line 1157)

**Test code**:
```typescript
const inviteButton = page.getByRole('button', { name: /invite.*user/i });
// Checks for accessible name on action buttons
const ariaLabel = await button.getAttribute('aria-label');
const title = await button.getAttribute('title');
const text = await button.textContent();
```

**Current state**:
- Invite button: text content "Invite User" ✅
- Delete button: `aria-label={t('users.deleteUser')}` ✅
- Settings button: `aria-label={t('users.editPermissions')}` ✅

**Action**: ✅ **Verify only** - unskip and run test.

---

## 3. Implementation Phases

### Phase 6.1: Verify Existing Functionality (3 hours)

**Goal**: Confirm tests 1, 2, 4, 6, 19 pass without code changes.

**Tests in Batch**:
- Test 1: should show user status badges
- Test 2: should display role badges
- Test 4: should open invite user modal
- Test 6: should copy invite link (toast test IDs already exist)
- Test 19: should have proper ARIA labels

**Tasks**:
1. Temporarily remove `test.skip` from tests 1, 2, 4, 6, 19
2. Run tests individually
3. Document results
4. Permanently unskip passing tests

**Commands**:
```bash
# Test status badges
npx playwright test tests/settings/user-management.spec.ts \
  --grep "should show user status badges" --project=chromium

# Test role badges
npx playwright test tests/settings/user-management.spec.ts \
  --grep "should display role badges" --project=chromium

# Test invite modal opens
npx playwright test tests/settings/user-management.spec.ts \
  --grep "should open invite user modal" --project=chromium

# Test copy invite link (toast test IDs already exist)
npx playwright test tests/settings/user-management.spec.ts \
  --grep "should copy invite link" --project=chromium

# Test ARIA labels
npx playwright test tests/settings/user-management.spec.ts \
  --grep "should have proper ARIA labels" --project=chromium
```

**Expected outcome**: 4-5 tests pass immediately.

---

### Phase 6.2: Email Validation UI (2 hours)

**Goal**: Add client-side email format validation with visible error.

**File to modify**: `frontend/src/pages/UsersPage.tsx` (InviteModal)

**Implementation**:

```tsx
// Add state in InviteModal component
const [emailError, setEmailError] = useState<string | null>(null)

// Email validation function
const validateEmail = (email: string): boolean => {
  if (!email) {
    setEmailError(null)
    return false
  }
  const emailRegex = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/
  if (!emailRegex.test(email)) {
    setEmailError(t('users.invalidEmail'))
    return false
  }
  setEmailError(null)
  return true
}

// Update email input (replace existing Input)
<div>
  <Input
    label={t('users.emailAddress')}
    type="email"
    value={email}
    onChange={(e) => {
      setEmail(e.target.value)
      validateEmail(e.target.value)
    }}
    placeholder="user@example.com"
    aria-invalid={!!emailError}
    aria-describedby={emailError ? 'email-error' : undefined}
  />
  {emailError && (
    <p
      id="email-error"
      className="mt-1 text-sm text-red-400"
      role="alert"
    >
      {emailError}
    </p>
  )}
</div>

// Update button disabled logic
disabled={!email || !!emailError}
```

**Translation key to add** (to appropriate i18n file):
```json
{
  "users.invalidEmail": "Please enter a valid email address"
}
```

**Validation**:
```bash
npx playwright test tests/settings/user-management.spec.ts \
  --grep "should validate email format" --project=chromium
```

**Expected outcome**: Test 5 passes.

---

### Phase 6.3: Resend Invite Action (6-10 hours)

**Goal**: Add resend invite button for pending users.

#### Backend Verification

**REQUIRED**: Check if backend endpoint exists before proceeding:
```bash
grep -r "resend" backend/internal/api/handlers/
grep -r "ResendInvite" backend/internal/api/
```

**Result of verification**: Backend endpoint **does not exist**. Both grep commands return no results.

**Contingency**: If backend is missing (confirmed), effort increases to **8-10 hours** to implement:
- Endpoint: `POST /api/v1/users/{id}/resend-invite`
- Handler: Regenerate token, send email, return new token info
- Tests: Unit tests for the new handler

#### Frontend Implementation

**File**: `frontend/src/api/users.ts`

Add API function:
```typescript
/**
 * Resends an invitation email to a pending user.
 * @param id - The user ID to resend invite to
 * @returns Promise resolving to InviteUserResponse with new token
 */
export const resendInvite = async (id: number): Promise<InviteUserResponse> => {
  const response = await client.post<InviteUserResponse>(`/users/${id}/resend-invite`)
  return response.data
}
```

**File**: `frontend/src/pages/UsersPage.tsx`

Add mutation:
```tsx
const resendInviteMutation = useMutation({
  mutationFn: resendInvite,
  onSuccess: (data) => {
    queryClient.invalidateQueries({ queryKey: ['users'] })
    if (data.email_sent) {
      toast.success(t('users.inviteResent'))
    } else {
      toast.success(t('users.inviteCreatedNoEmail'))
    }
  },
  onError: (error: unknown) => {
    const err = error as { response?: { data?: { error?: string } } }
    toast.error(err.response?.data?.error || t('users.resendFailed'))
  },
})
```

Add button in user row actions:
```tsx
{user.invite_status === 'pending' && (
  <button
    onClick={() => resendInviteMutation.mutate(user.id)}
    className="p-1.5 text-gray-400 hover:text-blue-400 hover:bg-gray-800 rounded"
    title={t('users.resendInvite')}
    aria-label={t('users.resendInvite')}
    disabled={resendInviteMutation.isPending}
  >
    <Mail className="h-4 w-4" />
  </button>
)}
```

**Translation keys**:
```json
{
  "users.resendInvite": "Resend Invite",
  "users.inviteResent": "Invitation resent successfully",
  "users.inviteCreatedNoEmail": "New invite created. Email could not be sent.",
  "users.resendFailed": "Failed to resend invitation"
}
```

**Validation**:
```bash
npx playwright test tests/settings/user-management.spec.ts \
  --grep "should resend invite" --project=chromium
```

**Expected outcome**: Test 15 passes.

---

### Phase 6.4: Modal Keyboard Navigation (2 hours)

**Goal**: Ensure Escape key closes modals.

**File to modify**: `frontend/src/pages/UsersPage.tsx`

**Implementation** (add to InviteModal and PermissionsModal):

```tsx
// Add useEffect for keyboard handler
useEffect(() => {
  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      handleClose()
    }
  }

  if (isOpen) {
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }
}, [isOpen, handleClose])
```

**Note**: Test 16 (keyboard navigation) is marked as **known flaky** and may remain skipped.

---

### Phase 6.5: PermissionsModal useState Bug Fix (1 hour)

**Goal**: Fix React anti-pattern in PermissionsModal.

**File to modify**: `frontend/src/pages/UsersPage.tsx` (line 339)

**Bug**: `useState` is being used like a `useEffect`, which is a React anti-pattern:

```tsx
// WRONG - useState used like an effect (current code at line 339)
useState(() => {
  if (user) {
    setPermissionMode(user.permission_mode || 'allow_all')
    setSelectedHosts(user.permitted_hosts || [])
  }
})
```

**Fix**: Replace with proper `useEffect` with dependency:

```tsx
// CORRECT - useEffect with dependency
useEffect(() => {
  if (user) {
    setPermissionMode(user.permission_mode || 'allow_all')
    setSelectedHosts(user.permitted_hosts || [])
  }
}, [user])
```

**Why this matters**: The useState initializer only runs once on mount. The current code appears to work incidentally but:
1. Will not update state when `user` prop changes
2. May cause stale data bugs
3. Violates React's data flow principles

**Validation**:
```bash
# Run TypeScript check
cd frontend && npm run typecheck

# Run related permission tests (after Phase 5 auth fix)
npx playwright test tests/settings/user-management.spec.ts \
  --grep "permissions" --project=chromium
```

---

## 4. Implementation Order

```
Week 1 (10-14 hours)
├── Phase 6.1: Verify Existing (3h) → Tests 1, 2, 4, 6, 19
├── Phase 6.2: Email Validation (2h) → Test 5
├── Phase 6.3: Resend Invite (6-10h) → Test 15
│   └── Includes backend endpoint implementation
└── Phase 6.5: PermissionsModal Bug Fix (1h) → Stability

Week 2 (2-3 hours)
└── Phase 6.4: Modal Keyboard Nav (2h) → Partial for Test 16

Validation & Cleanup (3 hours)
└── Run full suite, update skip comments
```

---

## 5. Files to Modify

### Priority 1: Required for Test Enablement

| File | Changes |
|------|---------|
| `frontend/src/pages/UsersPage.tsx` | Email validation, resend invite button, keyboard nav, PermissionsModal useState→useEffect fix |
| `frontend/src/api/users.ts` | Add `resendInvite` function |
| `tests/settings/user-management.spec.ts` | Unskip verified tests |

### Priority 2: Backend (REQUIRED - endpoint missing)

| File | Changes |
|------|---------|
| `backend/internal/api/handlers/user_handler.go` | Add resend-invite endpoint |
| `backend/internal/api/routes.go` | Register new route |
| `backend/internal/api/handlers/user_handler_test.go` | Add tests for resend endpoint |

### Priority 3: Translations

| File | Keys to Add |
|------|-------------|
| `frontend/src/i18n/locales/en.json` | `invalidEmail`, `resendInvite`, `inviteResent`, etc. |

### NOT Required (Already Implemented)

| File | Status |
|------|--------|
| `frontend/src/components/Toast.tsx` | ✅ Already has `data-testid={toast-${toast.type}}` |

---

## 6. Validation Strategy

### After Each Phase

```bash
# Run specific tests
npx playwright test tests/settings/user-management.spec.ts \
  --grep "<test name>" --project=chromium
```

### Final Validation

```bash
# Run all user management tests
npx playwright test tests/settings/user-management.spec.ts --project=chromium

# Expected:
# - ~12-14 tests passing (up from ~5)
# - ~6-8 tests still skipped (auth blocked or by design)
```

### Test Coverage

```bash
cd frontend && npm run test:coverage
# Verify UsersPage.tsx >= 85%
```

---

## 7. Expected Outcomes

### Tests to Unskip After Phase 6

| Test | Expected Outcome |
|------|------------------|
| should show user status badges | ✅ Pass |
| should display role badges | ✅ Pass |
| should open invite user modal | ✅ Pass |
| should validate email format | ✅ Pass |
| should copy invite link | ✅ Pass |
| should resend invite for pending user | ✅ Pass |
| should have proper ARIA labels | ✅ Pass |

**Total**: 7 tests enabled

### Tests Remaining Skipped

| Test | Reason |
|------|--------|
| Tests 7-12, 14 (7 tests) | 🔒 TestDataManager auth (Phase 5) |
| Test 3: pending invite status | ⚠️ Complex timing |
| Test 13: change user role | ❌ Feature not implemented |
| Test 16: keyboard navigation | ⚠️ Known flaky |
| Tests 17-18: admin access | ℹ️ Routing design (intentional) |

**Total remaining skipped**: ~12 tests (down from 22)

---

## 8. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Backend resend endpoint missing | **CONFIRMED** | High | Backend implementation included in Phase 6.3 (6-10h) |
| Tests pass locally, fail in CI | Medium | Medium | Run in both environments |
| Translation keys missing | Low | Low | Add to all locale files |
| PermissionsModal bug causes regressions | Low | Medium | Fix early in Phase 6.5 with testing |

---

## 9. Success Metrics

| Metric | Before Phase 6 | After Phase 6 | Target |
|--------|----------------|---------------|--------|
| User management tests passing | ~5 | ~12-14 | 15+ |
| User management tests skipped | 19 | 10-12 | <10 |
| Frontend coverage (UsersPage) | TBD | ≥85% | 85% |

---

## 10. Timeline

| Phase | Effort | Cumulative |
|-------|--------|------------|
| 6.1: Verify Existing (5 tests) | 3h | 3h |
| 6.2: Email Validation | 2h | 5h |
| 6.3: Resend Invite (backend included) | 6-10h | 11-15h |
| 6.4: Modal Keyboard Nav | 2h | 13-17h |
| 6.5: PermissionsModal Bug Fix | 1h | 14-18h |
| Validation & Cleanup | 3h | 17-21h |
| Buffer | 3h | **16-22h** |

**Total**: 16-22 hours (range depends on backend complexity)

---

## Change Log

| Date | Author | Change |
|------|--------|--------|
| 2026-01-24 | Planning Agent | Initial plan created with detailed test analysis |
| 2026-01-24 | Planning Agent | **REVISION**: Applied Supervisor corrections: |
| | | - Toast test IDs already exist (Phase 6.2 removed) |
| | | - Updated test line numbers to actual values (70, 110, 217, 283, 442, 956, 1014, 1157) |
| | | - Added Test 4 and Test 6 to Phase 6.1 verification batch (5 tests total) |
| | | - Added Phase 6.5: PermissionsModal useState bug fix |
| | | - Backend resend endpoint confirmed missing (grep verification) |
| | | - Corrected test count: 19 skipped tests (not 22) |
| | | - Updated effort estimates: 16-22h (was 17h) |
