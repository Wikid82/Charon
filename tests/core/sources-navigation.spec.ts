/**
 * Sources navigation E2E Tests
 *
 * The "Remote Servers" page is presented as "Sources" in the UI while the
 * route stays /hecate/remote-servers, and the legacy /remote-servers path
 * keeps redirecting.
 *
 * Written as `test.fixme` in C1; enabled once the rename lands (C4).
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('Sources navigation', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await page.goto('/');
    await waitForLoadingComplete(page);
  });

  test.fixme('sidebar "Sources" link navigates to /hecate/remote-servers with a Sources heading', async ({ page }) => {
    await test.step('Expand the Hecate group', async () => {
      const hecateGroup = page
        .getByRole('button', { name: /hecate/i })
        .or(page.getByRole('link', { name: /hecate/i }))
        .first();

      if ((await hecateGroup.getAttribute('aria-expanded')) !== 'true') {
        await hecateGroup.click();
      }
    });

    await test.step('Click the Sources link', async () => {
      await page.getByRole('link', { name: /^sources$/i }).first().click();
      await waitForLoadingComplete(page);
    });

    await test.step('Verify URL and heading', async () => {
      await expect(page).toHaveURL(/\/hecate\/remote-servers/i);
      await expect(page.getByRole('heading', { name: /^sources$/i, level: 1 })).toBeVisible();
    });
  });

  test.fixme('legacy /remote-servers still redirects to /hecate/remote-servers', async ({ page }) => {
    await page.goto('/remote-servers');
    await waitForLoadingComplete(page);
    await expect(page).toHaveURL(/\/hecate\/remote-servers/i);
    await expect(page.getByRole('heading', { name: /^sources$/i, level: 1 })).toBeVisible();
  });
});
