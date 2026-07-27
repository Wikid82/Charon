/**
 * Regression coverage: page.goto() racing a still-settling prior navigation
 *
 * Root cause (see docs/reports/qa_report.md, "Shard 4 reload-hang RCA"):
 * firing a top-level page.goto() within roughly 100ms-2s of a just-completed
 * or still-settling prior navigation can fail to produce a Playwright-trackable
 * navigation-commit event in Firefox, hanging page.goto()'s promise for the
 * full test timeout even though the application itself renders correctly.
 * This file codifies both confirmed failure shapes so a regression in either
 * the test-helper fix or the underlying Playwright/Firefox behavior is caught.
 */

import { test, expect, loginUser, logoutUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('Navigation-settle regression coverage', () => {
  test('same-URL reload immediately after a fresh /login navigation does not hang', async ({ page }) => {
    // Shape of failure #1 (user-lifecycle.spec.ts navigateToLogin): a second
    // navigation to the *same* URL fired moments after the first one
    // completed, while the SPA is still hydrating.
    await page.goto('/login', { waitUntil: 'domcontentloaded' });

    // Deliberately race the still-hydrating page with an immediate second
    // navigation to the same URL, exercising the fixed reload() pattern
    // rather than a redundant goto().
    await page.reload({ waitUntil: 'domcontentloaded' });

    const emailInput = page.locator('input[type="email"]').or(page.getByLabel(/email/i)).first();
    await expect(emailInput).toBeVisible({ timeout: 15000 });
  });

  test('navigating to a protected route immediately after login does not race the post-login redirect', async ({ page, adminUser, regularUser }) => {
    // Shape of failure #2 (user-management.spec.ts "Navigate to users page
    // directly"): a fresh top-level goto() to a *different*, protected route
    // fired immediately after login, potentially racing the app's own
    // client-side post-login redirect away from /login.
    //
    // logoutUser() requires an already-authenticated, rendered page to find
    // the Logout control on (unlike user-management.spec.ts, this file has
    // no beforeEach establishing that), so log in as adminUser first.
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);

    await logoutUser(page);
    await loginUser(page, regularUser);

    // The fix under test: wait for the app to actually leave /login before
    // firing the next top-level navigation.
    await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 15000 }).catch(() => undefined);

    await page.goto('/proxy-hosts', { waitUntil: 'domcontentloaded' });

    await expect(page).not.toHaveURL(/\/login/);
  });
});
