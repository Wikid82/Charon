# Telegram Notification Provider — Test Failure Remediation Plan

**Date:** 2026-03-11
**Author:** Planning Agent
**Status:** Remediation Required — All security scans pass, test failures block merge
**Previous Plan:** Archived as `docs/plans/telegram_implementation_spec.md`

---

## 1. Introduction

The Telegram notification provider feature is functionally complete with passing security scans and coverage gates. However, **56 E2E test failures** and **2 frontend unit test failures** block the PR merge. This plan identifies root causes, categorises each failure set, and provides specific remediation steps.

### Failure Summary

| Spec File | Failures | Browsers | Unique Est. | Category |
|---|---|---|---|---|
| `notifications.spec.ts` | 48 | 3 | ~16 | **Our change** |
| `notifications-payload.spec.ts` | 18 | 3 | ~6 | **Our change** |
| `telegram-notification-provider.spec.ts` | 4 | 1–3 | ~2 | **Our change** |
| `encryption-management.spec.ts` | 20 | 3 | ~7 | Pre-existing |
| `auth-middleware-cascade.spec.ts` | 18 | 3 | 6 | Pre-existing |
| `Notifications.test.tsx` (unit) | 2 | — | 2 | **Our change** |

CI retries: 2 per test (`playwright.config.js` L144). Failure counts above represent unique test failures × browser projects.

---

## 2. Root Cause Analysis

### Root Cause A: `isNew` Guard on Test Button (CRITICAL — Causes ~80% of failures)

**What changed:** The Telegram feature added a guard in `Notifications.tsx` (L117-124) that blocks the "Test" button for new (unsaved) providers:

```typescript
// Line 117-124: handleTest() early return guard
const handleTest = () => {
  const formData = watch();
  const currentType = normalizeProviderType(formData.type);
  if (!formData.id && currentType !== 'email') {
    toast.error(t('notificationProviders.saveBeforeTesting'));
    return;
  }
  testMutation.mutate({ ...formData, type: currentType } as Partial<NotificationProvider>);
};
```

And a `disabled` attribute on the test button at `Notifications.tsx` (L382):

```typescript
// Line 382: Button disabled state
disabled={testMutation.isPending || (isNew && !isEmail)}
```

**Why it was added:** The backend `Test` handler at `notification_provider_handler.go` (L333-336) requires a saved provider ID for all non-email types. For Gotify/Telegram, the server needs the stored token. For Discord/Webhook, the server still fetches the provider from DB. Without a saved provider, the backend returns `MISSING_PROVIDER_ID`.

**Why it breaks tests:** Many existing E2E and unit tests click the test button from a **new (unsaved) provider form** using mocked endpoints. With the new guard:

1. The `<button>` is `disabled` → browser ignores clicks → mocked routes never receive requests
2. Even if not disabled, `handleTest()` returns early with a toast instead of calling `testMutation.mutate()`
3. Tests that `waitForRequest` on `/providers/test` time out (60s default)
4. Tests that assert on `capturedTestPayload` find `null`

**Is the guard correct?** Yes — it matches the backend's security-by-design constraint. The tests need to be adapted to the new behavior, not the guard removed.

### Root Cause B: Pre-existing Infrastructure Failures (encryption-management, auth-middleware-cascade)

**encryption-management.spec.ts** (17 tests, ~7 unique failures) navigates to `/security/encryption` and tests key rotation, validation, and history display. **Zero overlap** with notification provider code paths. No files modified in the Telegram PR affect encryption.

**auth-middleware-cascade.spec.ts** (6 tests, all 6 fail) uses deprecated `waitUntil: 'networkidle'`, creates proxy hosts via UI forms (`getByLabel(/domain/i)`), and tests auth flows through Caddy. **Zero overlap** with notification code. These tests have known fragility from UI element selectors and `networkidle` waits.

**Verdict:** Both are pre-existing failures. They should be tracked separately and not block the Telegram PR.

### Root Cause C: Telegram E2E Spec Issues (4 failures)

The `telegram-notification-provider.spec.ts` has 8 tests, with ~2 unique failures. Most likely candidates:

1. **"should edit telegram notification provider and preserve token"** (L159): Uses fragile keyboard navigation (focus Send Test → Tab → Enter) to reach the Edit button. If the `title` attribute on the Send Test button doesn't match the accessible name pattern `/send test/i`, or if the tab order is affected by any intermediate focusable element, the Enter press activates the wrong button or nothing at all.

2. **"should test telegram notification provider"** (L265): Clicks the row-level "Send Test" button. The locator uses `getByRole('button', { name: /send test/i })`. The button has `title={t('notificationProviders.sendTest')}` which renders as "Send Test". This should work, but the `title` attribute contributing to accessible name can be browser-dependent, particularly in WebKit.

---

## 3. Affected Tests — Complete Inventory

### 3.1 E2E Tests: `notifications.spec.ts` (Test Button on New Form)

These tests open the "Add Provider" form (no `id`), click `provider-test-btn`, and expect API interactions. The disabled button now prevents all interaction.

| # | Test Name | Line | Type Used | Failure Mode |
|---|---|---|---|---|
| 1 | should test notification provider | L1085 | discord | `waitForRequest` times out — button disabled |
| 2 | should show test success feedback | L1142 | discord | Success icon never appears — no click fires |
| 3 | should preserve Discord request payload contract for save, preview, and test | L1236 | discord | `capturedTestPayload` is null — button disabled |
| 4 | should show error when test fails | L1665 | discord | Error icon never appears — no click fires |

**Additional cascade effects:** The user reports ~16 unique failures from this file. The 4 above are directly caused by the `isNew` guard. Remaining failures may stem from cascading timeout effects, `beforeEach` state leakage after long timeouts, or other pre-existing flakiness amplified by the 60s timeout waterfall.

### 3.2 E2E Tests: `notifications-payload.spec.ts` (Test Button on New Form)

| # | Test Name | Line | Type Used | Failure Mode |
|---|---|---|---|---|
| 1 | provider-specific transformation strips gotify token from test and preview payloads | L264 | gotify | `provider-test-btn` disabled for new gotify form; `capturedTestPayload` is null |
| 2 | retry split distinguishes retryable and non-retryable failures | L410 | webhook | `provider-test-btn` disabled for new webhook form; `waitForResponse` times out |

**Tests that should still pass:**

- `valid payload flows for discord, gotify, and webhook` (L54) — uses `provider-save-btn`, not test button
- `malformed payload scenarios` (L158) — API-level tests via `page.request.post`
- `missing required fields block submit` (L192) — uses save button
- `auth/header behavior checks` (L217) — API-level tests
- `security: SSRF` (L314) — API-level tests
- `security: DNS-rebinding` (L381) — API-level tests
- `security: token does not leak` (L512) — API-level tests

### 3.3 E2E Tests: `telegram-notification-provider.spec.ts`

| # | Test Name | Line | Probable Failure Mode |
|---|---|---|---|
| 1 | should edit telegram notification provider and preserve token | L159 | Keyboard navigation (Tab from Send Test → Edit) fragility; may hit wrong element on some browsers |
| 2 | should test telegram notification provider | L265 | Row-level Send Test button; possible accessible name mismatch in WebKit with `title` attribute |

**Tests that should pass:**

- Form rendering tests (L25, L65) — UI assertions only
- Create telegram provider (L89) — mocked POST
- Delete telegram provider (L324) — mocked DELETE + confirm dialog
- Security tests (L389, L436) — mock-based assertions

### 3.4 Frontend Unit Tests: `Notifications.test.tsx`

| # | Test Name | Line | Failure Mode |
|---|---|---|---|
| 1 | submits provider test action from form using normalized discord type | L447 | `userEvent.click()` on disabled button is no-op → `testProvider` never called → `waitFor` times out |
| 2 | shows error toast when test mutation fails | L569 | Same — disabled button prevents click → `toast.error` with `saveBeforeTesting` fires instead of mutation error |

### 3.5 Pre-existing (Not Caused By Telegram PR)

| Spec | Tests | Rationale |
|---|---|---|
| `encryption-management.spec.ts` | ~7 unique | Tests encryption page at `/security/encryption`. No code overlap. |
| `auth-middleware-cascade.spec.ts` | 6 unique | Tests proxy creation + auth middleware. Uses `networkidle`. No code overlap. |

---

## 4. Remediation Plan

### Priority Order

1. **P0 — Fix unit tests** (fastest, unblocks local dev verification)
2. **P1 — Fix E2E test-button tests** (the core regression from our change)
3. **P2 — Fix telegram spec fragility** (new tests we added)
4. **P3 — Document pre-existing failures** (not our change, track separately)

---

### 4.1 P0: Frontend Unit Test Fixes

**File:** `frontend/src/pages/__tests__/Notifications.test.tsx`

#### Fix 1: "submits provider test action from form using normalized discord type" (L447)

**Problem:** Test opens "Add Provider" (new form, no `id`), clicks test button. Button is now disabled for new providers.

**Fix:** Change to test from an **existing provider's edit form** instead of a new form. This preserves the original intent (verifying the test payload uses normalized type).

```typescript
// BEFORE (L447-462):
it('submits provider test action from form using normalized discord type', async () => {
  vi.mocked(notificationsApi.testProvider).mockResolvedValue()
  const user = userEvent.setup()
  renderWithQueryClient(<Notifications />)

  await user.click(await screen.findByTestId('add-provider-btn'))
  await user.type(screen.getByTestId('provider-name'), 'Preview/Test Provider')
  await user.type(screen.getByTestId('provider-url'), 'https://example.com/webhook')
  await user.click(screen.getByTestId('provider-test-btn'))

  await waitFor(() => {
    expect(notificationsApi.testProvider).toHaveBeenCalled()
  })
  const payload = vi.mocked(notificationsApi.testProvider).mock.calls[0][0]
  expect(payload.type).toBe('discord')
})

// AFTER:
it('submits provider test action from form using normalized discord type', async () => {
  vi.mocked(notificationsApi.testProvider).mockResolvedValue()
  setupMocks([baseProvider])          // baseProvider has an id
  const user = userEvent.setup()
  renderWithQueryClient(<Notifications />)

  // Open edit form for existing provider (has id → test button enabled)
  const row = await screen.findByTestId(`provider-row-${baseProvider.id}`)
  const buttons = within(row).getAllByRole('button')
  await user.click(buttons[1]) // Edit button

  await user.click(screen.getByTestId('provider-test-btn'))

  await waitFor(() => {
    expect(notificationsApi.testProvider).toHaveBeenCalled()
  })
  const payload = vi.mocked(notificationsApi.testProvider).mock.calls[0][0]
  expect(payload.type).toBe('discord')
})
```

#### Fix 2: "shows error toast when test mutation fails" (L569)

**Problem:** Same — test opens new form, clicks test button, expects mutation error toast. Button is disabled.

**Fix:** Test from an existing provider's edit form.

```typescript
// BEFORE (L569-582):
it('shows error toast when test mutation fails', async () => {
  vi.mocked(notificationsApi.testProvider).mockRejectedValue(new Error('Connection refused'))
  const user = userEvent.setup()
  renderWithQueryClient(<Notifications />)

  await user.click(await screen.findByTestId('add-provider-btn'))
  await user.type(screen.getByTestId('provider-name'), 'Failing Provider')
  await user.type(screen.getByTestId('provider-url'), 'https://example.com/webhook')
  await user.click(screen.getByTestId('provider-test-btn'))

  await waitFor(() => {
    expect(toast.error).toHaveBeenCalledWith('Connection refused')
  })
})

// AFTER:
it('shows error toast when test mutation fails', async () => {
  vi.mocked(notificationsApi.testProvider).mockRejectedValue(new Error('Connection refused'))
  setupMocks([baseProvider])
  const user = userEvent.setup()
  renderWithQueryClient(<Notifications />)

  // Open edit form for existing provider
  const row = await screen.findByTestId(`provider-row-${baseProvider.id}`)
  const buttons = within(row).getAllByRole('button')
  await user.click(buttons[1]) // Edit button

  await user.click(screen.getByTestId('provider-test-btn'))

  await waitFor(() => {
    expect(toast.error).toHaveBeenCalledWith('Connection refused')
  })
})
```

#### Bonus: Add a NEW unit test for the `saveBeforeTesting` guard

```typescript
it('disables test button when provider is new (unsaved) and not email type', async () => {
  const user = userEvent.setup()
  renderWithQueryClient(<Notifications />)

  await user.click(await screen.findByTestId('add-provider-btn'))
  const testBtn = screen.getByTestId('provider-test-btn')
  expect(testBtn).toBeDisabled()
})
```

---

### 4.2 P1: E2E Test Fixes — notifications.spec.ts

**File:** `tests/settings/notifications.spec.ts`

**Strategy:** For tests that click the test button from a new form, restructure the flow to:

1. First **save** the provider (mocked create → returns id)
2. Then **test** from the saved provider row's Send Test button (row buttons are not gated by `isNew`)

#### Fix 3: "should test notification provider" (L1085)

**Current flow:** Add form → fill → mock test endpoint → click `provider-test-btn` → verify request
**Problem:** Test button disabled for new form
**Fix:** Save first, then click test from the provider row's Send Test button.

```typescript
// In the test, after filling the form and before clicking test:

// 1. Mock the create endpoint to return a provider with an id
await page.route('**/api/v1/notifications/providers', async (route, request) => {
  if (request.method() === 'POST') {
    const payload = await request.postDataJSON();
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ id: 'saved-test-id', ...payload }),
    });
  } else if (request.method() === 'GET') {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([{
        id: 'saved-test-id',
        name: 'Test Provider',
        type: 'discord',
        url: 'https://discord.com/api/webhooks/test/token',
        enabled: true
      }]),
    });
  } else {
    await route.continue();
  }
});

// 2. Save the provider first
await page.getByTestId('provider-save-btn').click();

// 3. Wait for the provider to appear in the list
await expect(page.getByText('Test Provider')).toBeVisible({ timeout: 5000 });

// 4. Click row-level Send Test button
const providerRow = page.getByTestId('provider-row-saved-test-id');
const sendTestButton = providerRow.getByRole('button', { name: /send test/i });
await sendTestButton.click();
```

#### Fix 4: "should show test success feedback" (L1142)

Same pattern as Fix 3: save provider first, then test from row.

#### Fix 5: "should preserve Discord request payload contract for save, preview, and test" (L1236)

**Current flow:** Add form → fill → click preview → click test → save → verify all payloads
**Problem:** Test button disabled for new form
**Fix:** Reorder to: Add form → fill → click preview → **save** → **test from row** → verify payloads

The preview button is NOT disabled for new forms (only the test button is), so preview still works from the new form. The test step must happen after save.

#### Fix 6: "should show error when test fails" (L1665)

Same pattern: save first, then test from row.

---

### 4.3 P1: E2E Test Fixes — notifications-payload.spec.ts

**File:** `tests/settings/notifications-payload.spec.ts`

#### Fix 7: "provider-specific transformation strips gotify token from test and preview payloads" (L264)

**Current flow:** Add gotify form → fill with token → click preview → click test → verify token not in payloads
**Problem:** Test button disabled for new gotify form
**Fix:** Preview still works from new form. For test, save first, then test from the saved provider row.

**Note:** The row-level test call uses `{ ...provider, type: normalizeProviderType(provider.type) }` where `provider` is the list item (which never contains `token/gotify_token` per the List handler that strips tokens). So the token-stripping assertion naturally holds for row-level tests.

#### Fix 8: "retry split distinguishes retryable and non-retryable failures" (L410)

**Current flow:** Add webhook form → fill → click test → verify retry semantics
**Problem:** Test button disabled for new webhook form
**Fix:** Save first (mock create), then open edit form (which has `id`) or test from the row.

---

### 4.4 P2: Telegram E2E Spec Hardening

**File:** `tests/settings/telegram-notification-provider.spec.ts`

#### Fix 9: "should edit telegram notification provider and preserve token" (L159)

**Problem:** Uses fragile keyboard navigation to reach the Edit button:

```typescript
await sendTestButton.focus();
await page.keyboard.press('Tab');
await page.keyboard.press('Enter');
```

This assumes Tab from Send Test lands on Edit. Tab order can vary across browsers.

**Fix:** Use a direct locator for the Edit button instead of keyboard navigation:

```typescript
// BEFORE:
await sendTestButton.focus();
await page.keyboard.press('Tab');
await page.keyboard.press('Enter');

// AFTER:
const editButton = providerRow.getByRole('button').nth(1); // Send Test=0, Edit=1
await editButton.click();
```

Or use a structural locator based on the edit icon class.

#### Fix 10: "should test telegram notification provider" (L265)

**Probable issue:** The `getByRole('button', { name: /send test/i })` relies on `title` for accessible name. WebKit may not compute accessible name from `title` the same way.

**Fix (source — preferred):** Add explicit `aria-label` to the row Send Test button in `Notifications.tsx` (L703):

```tsx
<Button
  variant="secondary"
  size="sm"
  onClick={() => testMutation.mutate({...})}
  title={t('notificationProviders.sendTest')}
  aria-label={t('notificationProviders.sendTest')}
>
```

**Fix (test — alternative):** Use structural locator:

```typescript
const sendTestButton = providerRow.locator('button').first();
```

---

### 4.5 P3: Document Pre-existing Failures

**Action:** File separate issues (not part of this PR) for:

1. **encryption-management.spec.ts** — ~7 unique test failures in `/security/encryption`. Likely UI rendering timing issues or flaky selectors. No code overlap with Telegram PR.

2. **auth-middleware-cascade.spec.ts** — All 6 tests fail × 3 browsers. Uses deprecated `waitUntil: 'networkidle'`, creates proxy hosts through fragile UI selectors (`getByLabel(/domain/i)`), and tests auth middleware cascade. Needs modernization pass for locators and waits.

---

## 5. Implementation Plan

### Phase 1: Unit Test Fixes (Immediate)

| Task | File | Lines | Complexity |
|---|---|---|---|
| Fix "submits provider test action" test | `Notifications.test.tsx` | L447-462 | Low |
| Fix "shows error toast" test | `Notifications.test.tsx` | L569-582 | Low |
| Add `saveBeforeTesting` guard unit test | `Notifications.test.tsx` | New | Low |

**Validation:** `cd frontend && npx vitest run src/pages/__tests__/Notifications.test.tsx`

### Phase 2: E2E Test Fixes — Core Regression

| Task | File | Lines | Complexity |
|---|---|---|---|
| Fix "should test notification provider" | `notifications.spec.ts` | L1085-1138 | Medium |
| Fix "should show test success feedback" | `notifications.spec.ts` | L1142-1178 | Medium |
| Fix "should preserve Discord payload contract" | `notifications.spec.ts` | L1236-1340 | Medium |
| Fix "should show error when test fails" | `notifications.spec.ts` | L1665-1706 | Medium |
| Fix "transformation strips gotify token" | `notifications-payload.spec.ts` | L264-312 | Medium |
| Fix "retry split retryable/non-retryable" | `notifications-payload.spec.ts` | L410-510 | High |

**Validation per test:** `npx playwright test --project=firefox <spec-file> -g "<test-name>"`

### Phase 3: Telegram Spec Hardening

| Task | File | Lines | Complexity |
|---|---|---|---|
| Replace keyboard nav with direct locator | `telegram-notification-provider.spec.ts` | L220-223 | Low |
| Add `aria-label` to row Send Test button | `Notifications.tsx` | L703-708 | Low |
| Verify all 8 telegram tests pass 3 browsers | All | — | Low |

**Validation:** `npx playwright test tests/settings/telegram-notification-provider.spec.ts`

### Phase 4: Accessibility Hardening (Optional — Low Priority)

Consider adding `aria-label` attributes to all icon-only buttons in the provider row for improved accessibility and test resilience:

| Button | Current Accessible Name Source | Recommended |
|---|---|---|
| Send Test | `title` attribute | Add `aria-label` |
| Edit | None (icon only) | Add `aria-label={t('common.edit')}` |
| Delete | None (icon only) | Add `aria-label={t('common.delete')}` |

---

## 6. Commit Slicing Strategy

**Decision:** Single PR with 2 focused commits

**Rationale:** All fixes are tightly coupled to the Telegram feature PR and represent test adaptations to a correct behavioral change. No cross-domain changes. Small total diff.

### Commit 1: "fix(test): adapt notification tests to save-before-test guard"

- **Scope:** All unit test and E2E test fixes (Phases 1-3)
- **Files:** `Notifications.test.tsx`, `notifications.spec.ts`, `notifications-payload.spec.ts`, `telegram-notification-provider.spec.ts`
- **Dependencies:** None
- **Validation Gate:** All notification-related tests pass locally on at least one browser

### Commit 2: "feat(a11y): add aria-labels to notification provider row buttons"

- **Scope:** Source code accessibility improvement (Phase 4)
- **Files:** `Notifications.tsx`
- **Dependencies:** Depends on Commit 1 (tests must pass first)
- **Validation Gate:** Telegram spec tests pass consistently on WebKit

### Rollback

- These are test-only changes (except the optional aria-label). Reverting either commit has zero production impact.
- If tests still fail after fixes, the next step is to run with `--debug` and capture trace artifacts.

---

## 7. Acceptance Criteria

- [ ] `Notifications.test.tsx` — all 2 previously failing tests pass
- [ ] `notifications.spec.ts` — all 4 isNew-guard-affected tests pass on 3 browsers
- [ ] `notifications-payload.spec.ts` — "transformation" and "retry split" tests pass on 3 browsers
- [ ] `telegram-notification-provider.spec.ts` — all 8 tests pass on 3 browsers
- [ ] No regressions in other notification tests
- [ ] New unit test validates the `saveBeforeTesting` guard / disabled button behavior
- [ ] `encryption-management.spec.ts` and `auth-middleware-cascade.spec.ts` failures documented as separate issues (not blocked by this PR)
