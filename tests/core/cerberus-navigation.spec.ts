/**
 * Cerberus Navigation E2E Tests
 *
 * Verifies that the Cerberus sidebar label rename is correct:
 * - The sidebar nav item is labelled "Cerberus" (not "Security")
 * - Clicking "Cerberus" navigates to a page under the /security/ path
 * - No sidebar item with accessible name exactly matching /^security$/i exists
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('Cerberus Navigation', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);

    await page.goto('/');
    await waitForLoadingComplete(page);

    if (page.url().includes('/login')) {
      await loginUser(page, adminUser);
      await waitForLoadingComplete(page);
      await page.goto('/');
      await waitForLoadingComplete(page);
    }
  });

  test('sidebar renders a nav item labelled "Cerberus"', async ({ page }) => {
    await test.step('Verify "Cerberus" nav item is visible', async () => {
      const cerberusNav = page
        .getByRole('link', { name: /^cerberus$/i })
        .or(page.getByRole('button', { name: /^cerberus$/i }))
        .first();

      await expect(cerberusNav).toBeVisible();
    });
  });

  test('clicking "Cerberus" navigates to a page under /security/', async ({ page }) => {
    await test.step('Click the Cerberus nav item', async () => {
      const cerberusNav = page
        .getByRole('link', { name: /^cerberus$/i })
        .or(page.getByRole('button', { name: /^cerberus$/i }))
        .first();

      await cerberusNav.click();
      await waitForLoadingComplete(page);
    });

    await test.step('Verify URL is under /security/', async () => {
      await expect(page).toHaveURL(/\/security/i);
    });
  });

  test('no sidebar nav item has accessible name exactly matching "security"', async ({ page }) => {
    await test.step('Confirm no nav item is labelled exactly "security"', async () => {
      const securityNavLink = page.getByRole('link', { name: /^security$/i });
      const securityNavBtn = page.getByRole('button', { name: /^security$/i });

      await expect(securityNavLink).toHaveCount(0);
      await expect(securityNavBtn).toHaveCount(0);
    });
  });
});
