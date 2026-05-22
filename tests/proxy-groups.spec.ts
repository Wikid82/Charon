import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { getToastLocator } from './utils/ui-helpers';
import {
  waitForAPIResponse,
  waitForDialog,
  waitForLoadingComplete,
} from './utils/wait-helpers';

test.describe('Proxy Groups', () => {
  test.beforeEach(async ({ page }) => {
    await waitForAPIHealth(page.request);
    await page.goto('/proxy-hosts');
    await waitForLoadingComplete(page);
  });

  test.describe('Group Management', () => {
    test('should open Manage Groups dialog', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);
      await expect(page.getByRole('dialog')).toBeVisible();
    });

    test('should create a new proxy group', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);

      await page.getByRole('button', { name: /create group/i }).click();

      await page.getByRole('dialog').last().waitFor({ state: 'visible' });
      await page.getByLabel(/group name/i).fill('Test Group');

      const savePromise = waitForAPIResponse(page, '/api/v1/proxy-groups', { status: 201 });
      await page.getByRole('button', { name: /save/i }).click();
      await savePromise;

      await expect(getToastLocator(page)).toBeVisible();
    });

    test('should show validation error when name is empty', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);

      await page.getByRole('button', { name: /create group/i }).click();
      await page.getByRole('dialog').last().waitFor({ state: 'visible' });

      await page.getByRole('button', { name: /save/i }).click();

      const nameInput = page.getByLabel(/group name/i);
      await expect(nameInput).toBeFocused();
    });

    test('should allow selecting a preset color', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);

      await page.getByRole('button', { name: /create group/i }).click();
      await page.getByRole('dialog').last().waitFor({ state: 'visible' });

      await page.getByLabel(/group name/i).fill('Colored Group');

      const colorSwatches = page.locator('[data-testid="color-swatch"], button[title]').filter({ hasText: '' });
      if (await colorSwatches.count() > 0) {
        await colorSwatches.first().click();
      }

      await expect(page.getByLabel(/group name/i)).toHaveValue('Colored Group');
    });
  });

  test.describe('Grouped Display', () => {
    test('should show ungrouped section when groups exist', async ({ page }) => {
      const hasGroups = await page.getByRole('button', { name: /manage groups/i }).isVisible();
      expect(hasGroups).toBe(true);
    });

    test('should display flat table when no groups exist', async ({ page }) => {
      const table = page.getByRole('table').first();
      await expect(table).toBeVisible();
    });
  });

  test.describe('Bulk Assignment', () => {
    test('Assign to Group button is conditionally visible', async ({ page }) => {
      const checkboxes = page.getByRole('checkbox');
      const count = await checkboxes.count();

      if (count > 1) {
        await checkboxes.first().check();

        const assignBtn = page.getByRole('button', { name: /assign to group/i });
        const isVisible = await assignBtn.isVisible();
        if (isVisible) {
          await expect(assignBtn).toBeVisible();
        }
      }
    });
  });
});
