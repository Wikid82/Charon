# Skipped Tests Remediation Plan

**Status:** Active
**Created:** 2026-02-12
**Owner:** Playwright Dev
**Priority:** High (Blocking PRs)

## Executive Summary

This plan addresses 4 skipped E2E tests across 2 test files. Analysis reveals 1 test requires code fix (in progress), 2 tests have incorrect locators (test bugs), and 2 tests require accessibility enhancements (future backlog).

**Impact:**
- **1 test** depends on route guard fix (Frontend Dev working)
- **2 tests** can be fixed immediately (wrong test locators)
- **2 tests** require feature implementation (accessibility backlog)

---

## Test Inventory

### Category A: Bug in Code (Requires Code Fix)

#### A.1: Session Expiration Route Guard

**Location:** `tests/core/authentication.spec.ts:323`

**Test:**
```typescript
test.fixme('should redirect to login when session expires')
```

**Issue:** Route guards not blocking access to protected routes after session expiration.

**Evidence of Existing Fix:**
The route guard has been updated with defense-in-depth validation:

```typescript
// frontend/src/components/RequireAuth.tsx
const hasToken = localStorage.getItem('charon_auth_token');
const hasUser = user !== null;

if (!isAuthenticated || !hasToken || !hasUser) {
  return <Navigate to="/login" state={{ from: location }} replace />;
}
```

**Root Cause:**
Test simulates session expiration by clearing cookies/localStorage, then reloads the page. The `AuthContext.tsx` uses `checkAuth()` on mount to validate the session:

```typescript
// frontend/src/context/AuthContext.tsx
useEffect(() => {
  const checkAuth = async () => {
    const storedToken = localStorage.getItem('charon_auth_token');
    if (!storedToken) {
      setIsLoading(false);
      return;
    }
    setAuthToken(storedToken);
    try {
      const { data } = await client.get<User>('/auth/me');
      setUser(data);
    } catch {
      setUser(null);
      setAuthToken(null);
      localStorage.removeItem('charon_auth_token');
    } finally {
      setIsLoading(false);
    }
  };
  checkAuth();
}, []);
```

**Current Status:** ✅ Code fix merged (2026-01-30)

**Validation Task:**
```yaml
- task: Verify route guard fix
  owner: Playwright Dev
  priority: High
  steps:
    - Re-enable test by changing test.fixme() to test()
    - Run: npx playwright test tests/core/authentication.spec.ts:323 --project=firefox
    - Verify test passes
    - If passes: Remove .fixme() marker
    - If fails: Document failure mode and escalate to Frontend Dev
```

**Acceptance Criteria:**
- [ ] Test passes consistently (3/3 runs)
- [ ] Page redirects to `/login` within 10s after clearing auth state
- [ ] No console errors during redirect
- [ ] Test uses proper auth fixture isolation

**Estimated Effort:** 1 hour (validation + documentation)

---

### Category B: Bug in Test (Requires Test Fix)

#### B.1: Emergency Token Generation - Wrong Locator (Line 137)

**Location:** `tests/core/admin-onboarding.spec.ts:137`

**Current Test Code:**
```typescript
await test.step('Find emergency token section', async () => {
  const emergencySection = page.getByText(/admin whitelist|emergency|break.?glass|recovery token/i);
  const isVisible = await emergencySection.isVisible().catch(() => false);
  if (!isVisible) {
    test.skip(true, 'Emergency token feature not available in this deployment');
  }
  await expect(emergencySection).toBeVisible();
});
```

**Issue:** Test searches for text patterns that don't exist on the page.

**Evidence:**
- Feature EXISTS: `/settings/security` page with "Generate Token" button
- API exists: `POST /security/breakglass/generate`
- Hook exists: `useGenerateBreakGlassToken()`
- Button text: "Generate Token" (from `t('security.generateToken')`)
- Tooltip: "Generate a break-glass token for emergency access"
- Test searches: `/admin whitelist|emergency|break.?glass|recovery token/i`

**Problem:** Button text is generic ("Generate Token"), and the test doesn't search tooltips/aria-labels.

**Root Cause:** Test assumes button contains "emergency" or "break-glass" in visible text.

**Resolution:**
Use role-based locator with flexible name matching:

```typescript
await test.step('Find emergency token section', async () => {
  // Navigate to security settings first
  await page.goto('/settings/security', { waitUntil: 'domcontentloaded' });
  await waitForLoadingComplete(page);

  // Look for the generate token button - it may have different text labels
  // but should be identifiable by role and contain "token" or "generate"
  const generateButton = page.getByRole('button', { name: /generate.*token|token.*generate/i });
  const isVisible = await generateButton.isVisible().catch(() => false);

  if (!isVisible) {
    test.skip(true, 'Break-glass token feature not available in this deployment');
  }

  await expect(generateButton).toBeVisible();
});
```

**Validation:**
```bash
npx playwright test tests/core/admin-onboarding.spec.ts:130-160 --project=firefox
```

**Acceptance Criteria:**
- [ ] Test finds button using role-based locator
- [ ] Test passes on `/settings/security` page
- [ ] No false positives (doesn't match unrelated buttons)
- [ ] Clear skip message if feature missing

**Estimated Effort:** 30 minutes

---

#### B.2: Emergency Token Generation - Wrong Locator (Line 146)

**Location:** `tests/core/admin-onboarding.spec.ts:146`

**Current Test Code:**
```typescript
await test.step('Generate emergency token', async () => {
  const generateButton = page.getByRole('button', { name: /generate token/i });
  const isGenerateVisible = await generateButton.isVisible().catch(() => false);
  if (!isGenerateVisible) {
    test.skip(true, 'Generate Token button not available in this deployment');
    return;
  }

  await generateButton.click();

  // Wait for modal or confirmation
  await page.waitForSelector('[role="dialog"], [class*="modal"]', { timeout: 5000 }).catch(() => {
    return Promise.resolve();
  });
});
```

**Issue:** Same as B.1 - test needs to navigate to correct page first and use proper locator.

**Resolution:**
Merge with B.1 fix. The test should be in a single step that:
1. Navigates to `/settings/security`
2. Finds button with proper locator
3. Clicks and waits for modal/confirmation

**Combined Fix:**
```typescript
test('Emergency token can be generated', async ({ page }) => {
  await test.step('Navigate to security settings and find token generation', async () => {
    await page.goto('/settings/security', { waitUntil: 'domcontentloaded' });
    await waitForLoadingComplete(page);

    const generateButton = page.getByRole('button', { name: /generate.*token|token.*generate/i });
    const isVisible = await generateButton.isVisible().catch(() => false);

    if (!isVisible) {
      test.skip(true, 'Break-glass token feature not available in this deployment');
    }

    await expect(generateButton).toBeVisible();
    await expect(generateButton).toBeEnabled();
  });

  await test.step('Generate token and verify modal', async () => {
    const generateButton = page.getByRole('button', { name: /generate.*token|token.*generate/i });
    await generateButton.click();

    // Wait for modal or inline token display
    const modal = page.locator('[role="dialog"], [class*="modal"]');
    const hasModal = await modal.isVisible({ timeout: 5000 }).catch(() => false);

    // If no modal, token might display inline
    if (!hasModal) {
      const tokenDisplay = page.locator('[data-testid="breakglass-token"], input[readonly]');
      await expect(tokenDisplay).toBeVisible({ timeout: 5000 });
    } else {
      await expect(modal).toBeVisible();
    }
  });

  await test.step('Verify token displayed and copyable', async () => {
    // Token input or display field
    const tokenField = page.locator('input[readonly], [data-testid="breakglass-token"], [data-testid="emergency-token"], code').first();
    await expect(tokenField).toBeVisible();

    // Should have copy button near the token
    const copyButton = page.getByRole('button', { name: /copy|clipboard/i });
    const hasCopyButton = await copyButton.isVisible().catch(() => false);

    if (hasCopyButton) {
      await copyButton.click();
      // Verify copy feedback (toast, button change, etc.)
      const copiedFeedback = page.getByText(/copied/i).or(page.locator('[class*="success"]'));
      await expect(copiedFeedback).toBeVisible({ timeout: 3000 });
    }
  });
});
```

**Acceptance Criteria:**
- [ ] Test navigates to correct page
- [ ] Button found with flexible locator
- [ ] Modal or inline display detected
- [ ] Token value and copy button verified
- [ ] Test passes 3/3 times

**Estimated Effort:** 30 minutes

---

### Category C: Accessibility Enhancements (Future Features)

#### C.1: Copy Button ARIA Labels

**Location:** `tests/manual-dns-provider.spec.ts:282`

**Test:**
```typescript
test.skip('should have proper ARIA labels on copy buttons', async ({ page }) => {
  await test.step('Verify ARIA labels on copy buttons', async () => {
    const copyButtons = page.getByRole('button', { name: /copy record/i });
    const buttonCount = await copyButtons.count();
    expect(buttonCount).toBeGreaterThan(0);

    for (let i = 0; i < buttonCount; i++) {
      const button = copyButtons.nth(i);
      const ariaLabel = await button.getAttribute('aria-label');
      const textContent = await button.textContent();

      const isAccessible = ariaLabel || textContent?.trim();
      expect(isAccessible).toBeTruthy();
    }
  });
});
```

**Current Implementation:**
The copy buttons **DO** have proper ARIA labels:

```tsx
// frontend/src/components/dns-providers/ManualDNSChallenge.tsx:298-311
<Button
  variant="outline"
  size="sm"
  onClick={() => handleCopy('name', challenge.fqdn)}
  aria-label={t('dnsProvider.manual.copyRecordName')}  // ✅ HAS ARIA LABEL
  className="flex-shrink-0"
>
  {copiedField === 'name' ? (
    <Check className="h-4 w-4 text-success" aria-hidden="true" />
  ) : (
    <Copy className="h-4 w-4" aria-hidden="true" />
  )}
  <span className="sr-only">
    {copiedField === 'name' ? t('dnsProvider.manual.copied') : t('dnsProvider.manual.copy')}
  </span>
</Button>
```

**Status:** ✅ **FEATURE ALREADY IMPLEMENTED**

**Action:**
Remove `.skip()` marker and verify test passes:

```bash
npx playwright test tests/manual-dns-provider.spec.ts:282 --project=firefox
```

**Acceptance Criteria:**
- [ ] Test finds copy buttons by role
- [ ] All copy buttons have `aria-label` attributes
- [ ] Labels are descriptive and unique
- [ ] Test passes 3/3 times

**Estimated Effort:** 15 minutes (validation only)

---

#### C.2: Status Change Announcements

**Location:** `tests/manual-dns-provider.spec.ts:299`

**Test:**
```typescript
test.skip('should announce status changes to screen readers', async ({ page }) => {
  await test.step('Verify live region for status updates', async () => {
    const liveRegion = page.locator('[aria-live="polite"]').or(page.locator('[role="status"]'));
    await expect(liveRegion).toBeAttached();
  });
});
```

**Current Implementation:**
The component has a `statusAnnouncerRef` but it's **NOT** properly configured for screen readers:

```tsx
// frontend/src/components/dns-providers/ManualDNSChallenge.tsx:250-256
<div
  ref={statusAnnouncerRef}
  role="status"
  aria-live="polite"
  aria-atomic="true"
  className="sr-only"
/>
```

**Problem:**
The `<div>` exists with correct ARIA attributes, but it's **NEVER UPDATED** with text content when status changes. The ref is created but the text is not set when status changes occur.

**Evidence:**
```tsx
// Line 134: Ref created
const statusAnnouncerRef = useRef<HTMLDivElement>(null)

// Line 139-168: Status change effect exists but doesn't update announcer text
useEffect(() => {
  if (currentStatus !== previousStatusRef.current) {
    previousStatusRef.current = currentStatus
    // ❌ Missing: statusAnnouncerRef.current.textContent = statusMessage
  }
}, [currentStatus, pollData?.error_message, onComplete, t])
```

**Status:** 🔴 **FEATURE NOT IMPLEMENTED**

**Impact:**
- **Severity:** Medium
- **Users Affected:** Screen reader users
- **Workaround:** Screen reader users can query the status manually, but miss automatic updates
- **WCAG Level:** A (4.1.3 Status Messages)

**Required Implementation:**

```typescript
// In frontend/src/components/dns-providers/ManualDNSChallenge.tsx
// Update the status change effect (around line 139-168)

useEffect(() => {
  if (currentStatus !== previousStatusRef.current) {
    previousStatusRef.current = currentStatus

    // Get the status config for current status
    const statusInfo = STATUS_CONFIG[currentStatus]

    // Construct announcement text
    let announcement = t(statusInfo.labelKey)

    // Add error message if available
    if (currentStatus === 'failed' && pollData?.error_message) {
      announcement += `. ${pollData.error_message}`
    }

    // Update the screen reader announcer
    if (statusAnnouncerRef.current) {
      statusAnnouncerRef.current.textContent = announcement
    }

    // Existing completion logic...
    if (currentStatus === 'verified') {
      toast.success(t('dnsProvider.manual.verifySuccess'))
      onComplete(true)
    } else if (TERMINAL_STATES.includes(currentStatus) && currentStatus !== 'verified') {
      toast.error(pollData?.error_message || t('dnsProvider.manual.verifyFailed'))
      onComplete(false)
    }
  }
}, [currentStatus, pollData?.error_message, onComplete, t])
```

**Validation:**
1. Add manual test with screen reader (NVDA/JAWS/VoiceOver)
2. Verify status changes are announced
3. Run E2E test to verify `aria-live` region updates

**Acceptance Criteria:**
- [ ] Status announcer ref is updated on every status change
- [ ] Announcement includes status name and error message (if applicable)
- [ ] Text is cleared and replaced on each change (not appended)
- [ ] Screen reader announces changes automatically
- [ ] E2E test passes with live region text verification

**Estimated Effort:** 2 hours (implementation + testing)

**Priority:** Medium - Accessibility improvement for screen reader users

**Action Item:**
```yaml
- task: Implement status change announcements
  owner: Frontend Dev
  priority: Medium
  labels: [accessibility, enhancement, a11y]
  milestone: Q1 2026
  references:
    - WCAG 4.1.3 Status Messages
    - docs/guides/manual-dns-provider.md
```

---

## Implementation Roadmap

### Phase 1: Immediate (Block Current PR)

**Goal:** Fix tests that should already pass

| Task | Owner | Effort | Priority |
|------|-------|--------|----------|
| Verify session expiration fix (A.1) | Playwright Dev | 1h | Critical |
| Fix emergency token locators (B.1, B.2) | Playwright Dev | 1h | Critical |
| Verify copy button ARIA labels (C.1) | Playwright Dev | 15m | High |

**Total Effort:** ~2.25 hours

**Deliverables:**
- [ ] 3 tests re-enabled and passing
- [ ] Documentation updated with fix notes
- [ ] PR ready for review

**Exit Criteria:**
- All Phase 1 tests pass 3/3 times in Firefox
- No new console errors introduced
- Tests use proper fixtures and isolation

---

### Phase 2: Post-Green (Future Enhancements)

**Goal:** Implement missing accessibility features

| Task | Owner | Effort | Priority |
|------|-------|--------|----------|
| Implement status announcements (C.2) | Frontend Dev | 2h | Medium |
| Test announcements with screen readers | QA / Accessibility | 1h | Medium |
| Update E2E test to verify announcements | Playwright Dev | 30m | Medium |

**Total Effort:** ~3.5 hours

**Deliverables:**
- [ ] Status change announcer implemented
- [ ] Manual screen reader testing completed
- [ ] E2E test re-enabled and passing
- [ ] User guide updated with accessibility notes

**Exit Criteria:**
- Screen reader users receive automatic status updates
- E2E test verifies live region text content
- No WCAG 4.1.3 violations detected

---

## Risk Assessment

### High Risk
- **A.1 (Session Expiration):** If fix doesn't work, blocks route guard validation
  - *Mitigation:* Frontend Dev available for debugging
  - *Escalation:* Document exact failure mode, create new issue

### Medium Risk
- **C.2 (Status Announcements):** Requires frontend code change
  - *Mitigation:* Non-blocking, can defer to next sprint
  - *Impact:* Accessibility improvement, not critical functionality

### Low Risk
- **B.1, B.2 (Token Locators):** Simple test fix, no code changes
- **C.1 (ARIA Labels):** Feature already implemented, just needs validation

---

## Success Metrics

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| Skipped Tests | 0 | 4 | 🔴 |
| E2E Pass Rate | 100% | ~97% | 🟡 |
| Accessibility Coverage | 100% | ~95% | 🟡 |

**Post-Remediation:**
- **Skipped Tests:** 0 (all resolved or in backlog)
- **E2E Pass Rate:** 100% (all critical flows tested)
- **Accessibility Coverage:** 100% (all interactive elements accessible)

---

## Technical Debt Log

### Created During Remediation
None - all fixes are proper implementations, no shortcuts taken.

### Resolved During Remediation
1. **Vague test locators:** Emergency token tests now use role-based locators
2. **Missing navigation:** Tests now navigate to correct page before assertions
3. **Improper skip conditions:** Tests now have clear, actionable skip messages

---

## Appendix: Test Execution Reference

### Running Individual Tests

```bash
# Session expiration test
npx playwright test tests/core/authentication.spec.ts:323 --project=firefox

# Emergency token tests
npx playwright test tests/core/admin-onboarding.spec.ts:130-160 --project=firefox

# Copy button ARIA labels
npx playwright test tests/manual-dns-provider.spec.ts:282 --project=firefox

# Status announcements (after implementation)
npx playwright test tests/manual-dns-provider.spec.ts:299 --project=firefox
```

### Running Full Suites

```bash
# All authentication tests
npx playwright test tests/core/authentication.spec.ts --project=firefox

# All onboarding tests
npx playwright test tests/core/admin-onboarding.spec.ts --project=firefox

# All manual DNS tests
npx playwright test tests/manual-dns-provider.spec.ts --project=firefox
```

### Debug Mode

```bash
# Run with UI mode for visual debugging
npx playwright test <test-file> --ui

# Run with headed browser
npx playwright test <test-file> --headed --project=firefox

# Run with inspector
npx playwright test <test-file> --debug --project=firefox
```

---

## Change Log

| Date | Version | Changes |
|------|---------|---------|
| 2026-02-12 | 1.0 | Initial plan created |

---

## Approval & Sign-off

- [ ] **Technical Lead:** Reviewed and approved technical approach
- [ ] **Playwright Dev:** Agrees to Phase 1 timeline
- [ ] **Frontend Dev:** Agrees to Phase 2 timeline
- [ ] **QA Lead:** Reviewed test coverage impact

---

**Next Steps:**
1. Review this plan with team
2. Assign Phase 1 tasks to Playwright Dev
3. Create GitHub issues for Phase 2 tasks
4. Begin Phase 1 execution immediately
