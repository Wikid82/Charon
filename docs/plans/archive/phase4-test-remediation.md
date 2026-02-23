# Phase 4 Settings E2E Test Remediation Plan

**Created**: $(date +%Y-%m-%d)
**Status**: In Progress
**Tests Affected**: 137 total, ~87 failing (~63% failure rate)

## Executive Summary

Analysis of Phase 4 Settings E2E tests reveals systematic selector mismatches between test expectations and actual frontend implementations. The primary causes are:

1. **Missing `data-testid` attributes** in several components
2. **Different element structure** (e.g., table column headers vs. expected patterns)
3. **Missing route** (`/encryption` page exists but uses `PageShell` layout)
4. **Workflow differences** in modal interactions

---

## Test Status Overview

| Test Suite | Passing | Failing | Pass Rate | Priority |
|------------|---------|---------|-----------|----------|
| system-settings.spec.ts | ~27 | ~2 | 93% | P3 (Quick Wins) |
| smtp-settings.spec.ts | ~17 | ~1 | 94% | P3 (Quick Wins) |
| account-settings.spec.ts | ~8 | ~13 | 38% | P2 (Moderate) |
| encryption-management.spec.ts | 0 | 9 | 0% | P1 (Critical) |
| notifications.spec.ts | ~2 | ~28 | 7% | P1 (Critical) |
| user-management.spec.ts | ~5 | ~23 | 18% | P1 (Critical) |

---

## Priority 1: Critical Fixes (Complete Failures)

### 1.1 Encryption Management (0/9 passing)

**Root Cause**: Tests navigate to `/encryption` but the page uses `PageShell` component with different structure than expected.

**File**: [tests/settings/encryption-management.spec.ts](../../tests/settings/encryption-management.spec.ts)
**Component**: [frontend/src/pages/EncryptionManagement.tsx](../../frontend/src/pages/EncryptionManagement.tsx)

#### Selector Mismatches

| Test Expectation | Actual Implementation | Fix Required |
|------------------|----------------------|--------------|
| `page.getByText(/current version/i)` | Card with `t('encryption.currentVersion')` title | ✅ Works (translation may differ) |
| `page.getByText(/providers updated/i)` | Card with `t('encryption.providersUpdated')` title | ✅ Works |
| `page.getByText(/providers outdated/i)` | Card with `t('encryption.providersOutdated')` title | ✅ Works |
| `page.getByText(/next key/i)` | Card with `t('encryption.nextKey')` title | ✅ Works |
| `getByRole('button', { name: /rotate/i })` | Button with `RefreshCw` icon, text from translation | ✅ Works |
| Dialog confirmation | Uses `Dialog` component from ui | ✅ Should work |

**Likely Issue**: The page loads but may have API errors. Check:
1. `/api/encryption/status` endpoint availability
2. Loading state blocking tests
3. Translation keys loading

**Action Items**:
- [ ] Verify `/encryption` route is registered in router
- [ ] Add `data-testid` attributes to key cards for reliable selection
- [ ] Ensure API endpoints are mocked properly in tests

#### Recommended Component Changes

```tsx
// Add to EncryptionManagement.tsx status cards
<Card data-testid="encryption-current-version">
<Card data-testid="encryption-providers-updated">
<Card data-testid="encryption-providers-outdated">
<Card data-testid="encryption-next-key">
<Button data-testid="rotate-key-btn" ...>
<Button data-testid="validate-config-btn" ...>
```

---

### 1.2 Notifications (2/30 passing)

**Root Cause**: Component uses `data-testid` attributes correctly, but tests may have timing issues or the form structure differs.

**File**: [tests/settings/notifications.spec.ts](../../tests/settings/notifications.spec.ts)
**Component**: [frontend/src/pages/Notifications.tsx](../../frontend/src/pages/Notifications.tsx)

#### Selector Verification ✅ (Matching)

| Test Selector | Component Implementation | Status |
|---------------|-------------------------|--------|
| `getByTestId('provider-name')` | `data-testid="provider-name"` | ✅ Present |
| `getByTestId('provider-type')` | `data-testid="provider-type"` | ✅ Present |
| `getByTestId('provider-url')` | `data-testid="provider-url"` | ✅ Present |
| `getByTestId('provider-config')` | `data-testid="provider-config"` | ✅ Present |
| `getByTestId('provider-save-btn')` | `data-testid="provider-save-btn"` | ✅ Present |
| `getByTestId('provider-test-btn')` | `data-testid="provider-test-btn"` | ✅ Present |
| `getByTestId('notify-proxy-hosts')` | `data-testid="notify-proxy-hosts"` | ✅ Present |
| `getByTestId('notify-remote-servers')` | `data-testid="notify-remote-servers"` | ✅ Present |
| `getByTestId('notify-domains')` | `data-testid="notify-domains"` | ✅ Present |
| `getByTestId('notify-certs')` | `data-testid="notify-certs"` | ✅ Present |
| `getByTestId('notify-uptime')` | `data-testid="notify-uptime"` | ✅ Present |
| `getByTestId('template-name')` | `data-testid="template-name"` | ✅ Present |
| `getByTestId('template-save-btn')` | `data-testid="template-save-btn"` | ✅ Present |

**Likely Issues**:
1. **Form visibility**: The form only appears after clicking "Add Provider" button
2. **Loading states**: API calls may not complete before assertions
3. **Template section visibility**: Templates are hidden until `managingTemplates` state is true

**Action Items**:
- [ ] Ensure tests click "Add Provider" button before looking for form elements
- [ ] Add proper `waitForLoadingComplete` before interacting with forms
- [ ] Check translation keys match expected text patterns
- [ ] Verify API mocking for `/api/notifications/providers`

---

### 1.3 User Management (5/28 passing)

**Root Cause**: Tests expect specific table column headers and modal structures that differ from implementation.

**File**: [tests/settings/user-management.spec.ts](../../tests/settings/user-management.spec.ts)
**Component**: [frontend/src/pages/UsersPage.tsx](../../frontend/src/pages/UsersPage.tsx)

#### Selector Mismatches

| Test Expectation | Actual Implementation | Fix Required |
|------------------|----------------------|--------------|
| `getByRole('columnheader', { name: /user/i })` | `<th>{t('users.columnUser')}</th>` | ⚠️ Translation match |
| `getByRole('columnheader', { name: /role/i })` | `<th>{t('users.columnRole')}</th>` | ⚠️ Translation match |
| `getByRole('columnheader', { name: /status/i })` | `<th>{t('common.status')}</th>` | ⚠️ Translation match |
| `getByRole('columnheader', { name: /actions/i })` | `<th>{t('common.actions')}</th>` | ⚠️ Translation match |
| Invite modal email input via `getByLabel(/email/i)` | `<Input label={t('users.emailAddress')} ...>` | ⚠️ Need to verify label association |
| Permission modal via Settings icon | `<button title={t('users.editPermissions')}>` uses `<Settings>` icon | ✅ Works with title |

**Additional Issues**:
1. Table has 6 columns: User, Role, Status, Permissions, Enabled, Actions
2. Tests may expect only 4 columns (user, role, status, actions)
3. Switch component for enabled state
4. Modal uses custom div overlay, not a `dialog` role

**Action Items**:
- [ ] Update tests to expect 6 column headers instead of 4
- [ ] Verify Input component properly associates label with input via `htmlFor`/`id`
- [ ] Add `role="dialog"` to modal overlays for accessibility and testability
- [ ] Add `aria-label` to icon-only buttons

#### Recommended Component Changes

```tsx
// UsersPage.tsx - Add proper dialog role to modals
<div
  className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
  role="dialog"
  aria-modal="true"
  aria-labelledby="invite-modal-title"
>
  <div className="bg-dark-card ...">
    <h3 id="invite-modal-title" ...>

// Add aria-label to icon buttons
<button
  onClick={() => openPermissions(user)}
  aria-label={t('users.editPermissions')}
  title={t('users.editPermissions')}
>
  <Settings className="h-4 w-4" />
</button>
```

---

## Priority 2: Moderate Fixes

### 2.1 Account Settings (8/21 passing)

**File**: [tests/settings/account-settings.spec.ts](../../tests/settings/account-settings.spec.ts)
**Component**: [frontend/src/pages/Account.tsx](../../frontend/src/pages/Account.tsx)

#### Selector Verification

| Test Selector | Component Implementation | Status |
|---------------|-------------------------|--------|
| `#profile-name` | `id="profile-name"` | ✅ Present |
| `#profile-email` | `id="profile-email"` | ✅ Present |
| `#useUserEmail` | `id="useUserEmail"` | ✅ Present |
| `#cert-email` | `id="cert-email"` | ✅ Present |
| `#current-password` | `id="current-password"` | ✅ Present |
| `#new-password` | `id="new-password"` | ✅ Present |
| `#confirm-password` | `id="confirm-password"` | ✅ Present |
| `#confirm-current-password` | `id="confirm-current-password"` | ✅ Present |

**Likely Issues**:
1. **Conditional rendering**: `#cert-email` only visible when `!useUserEmail`
2. **Password confirmation modal**: Only appears when changing email
3. **API key section**: Requires profile data to load

**Action Items**:
- [ ] Ensure tests toggle `useUserEmail` checkbox before looking for `#cert-email`
- [ ] Add `waitForLoadingComplete` after page navigation
- [ ] Mock profile API to return consistent test data
- [ ] Verify password strength meter component doesn't block interactions

---

## Priority 3: Quick Wins

### 3.1 System Settings (~2 failing)

**File**: [tests/settings/system-settings.spec.ts](../../tests/settings/system-settings.spec.ts)
**Component**: [frontend/src/pages/SystemSettings.tsx](../../frontend/src/pages/SystemSettings.tsx)

#### Selector Verification ✅ (All Present)

| Test Selector | Component Implementation | Status |
|---------------|-------------------------|--------|
| `#caddy-api` | `id="caddy-api"` | ✅ Present |
| `#ssl-provider` | `id="ssl-provider"` (on SelectTrigger) | ✅ Present |
| `#domain-behavior` | `id="domain-behavior"` (on SelectTrigger) | ✅ Present |
| `#public-url` | `id="public-url"` | ✅ Present |
| `getByRole('switch', { name: /cerberus.*toggle/i })` | `aria-label="{label} toggle"` | ✅ Present |
| `getByRole('switch', { name: /crowdsec.*toggle/i })` | `aria-label="{label} toggle"` | ✅ Present |
| `getByRole('switch', { name: /uptime.*toggle/i })` | `aria-label="{label} toggle"` | ✅ Present |

**Remaining Issues**:
- Select component behavior (opening/selecting values)
- Feature flag API responses

**Action Items**:
- [ ] Verify Select component opens dropdown on click
- [ ] Mock feature flags API consistently

---

### 3.2 SMTP Settings (~1 failing)

**File**: [tests/settings/smtp-settings.spec.ts](../../tests/settings/smtp-settings.spec.ts)
**Component**: [frontend/src/pages/SMTPSettings.tsx](../../frontend/src/pages/SMTPSettings.tsx)

#### Selector Verification ✅ (All Present)

| Test Selector | Component Implementation | Status |
|---------------|-------------------------|--------|
| `#smtp-host` | `id="smtp-host"` | ✅ Present |
| `#smtp-port` | `id="smtp-port"` | ✅ Present |
| `#smtp-username` | `id="smtp-username"` | ✅ Present |
| `#smtp-password` | `id="smtp-password"` | ✅ Present |
| `#smtp-from` | `id="smtp-from"` | ✅ Present |
| `#smtp-encryption` | `id="smtp-encryption"` (on SelectTrigger) | ✅ Present |

**Remaining Issues**:
- Test email section only visible when `smtpConfig?.configured` is true

**Action Items**:
- [ ] Ensure SMTP config API returns `configured: true` for tests requiring test email section
- [ ] Verify status indicator updates after save

---

## Implementation Checklist

### Phase 1: Component Fixes (Estimated: 2-3 hours)

- [ ] **EncryptionManagement.tsx**: Add `data-testid` to status cards and action buttons
- [ ] **UsersPage.tsx**: Add `role="dialog"` and `aria-labelledby` to modals
- [ ] **UsersPage.tsx**: Add `aria-label` to icon-only buttons
- [ ] **Notifications.tsx**: Verify form visibility states in tests

### Phase 2: Test Fixes (Estimated: 4-6 hours)

- [ ] **user-management.spec.ts**: Update column header expectations (6 columns)
- [ ] **user-management.spec.ts**: Fix modal selectors to use `role="dialog"`
- [ ] **notifications.spec.ts**: Add "Add Provider" click before form interactions
- [ ] **encryption-management.spec.ts**: Add API mocking for encryption status
- [ ] **account-settings.spec.ts**: Fix conditional element tests (cert-email toggle)

### Phase 3: Validation (Estimated: 1-2 hours)

- [ ] Run full E2E suite with `npx playwright test --project=chromium`
- [ ] Document remaining failures
- [ ] Create follow-up issues for complex fixes

---

## Appendix A: Common Test Utility Patterns

### Wait for Loading
```typescript
await waitForLoadingComplete(page);
```

### Wait for Toast
```typescript
await waitForToast(page, 'Success message');
```

### Wait for Modal
```typescript
await waitForModal(page, 'Modal Title');
```

### Wait for API Response
```typescript
await waitForAPIResponse(page, '/api/endpoint', 'POST');
```

---

## Appendix B: Translation Key Reference

When tests use regex patterns like `/current version/i`, they need to match translation output. Key files:

- `frontend/src/locales/en/translation.json`
- Translation keys used in components

Ensure test patterns match translated text, or use `data-testid` for language-independent selection.

---

## Revision History

| Date | Author | Changes |
|------|--------|---------|
| 2024-XX-XX | Agent | Initial analysis and remediation plan |
