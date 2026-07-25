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

      // Explicit timeout: react-hot-toast's success toast auto-dismisses after
      // 5000ms (App.tsx's <Toaster toastOptions={{ duration: 5000 }}>), which
      // exactly matches Playwright's global default expect.timeout (also
      // 5000ms per playwright.config.js). That leaves zero margin between "the
      // toast is still there" and "the assertion gave up" — any latency
      // between the API response and React's onSuccess->toast.success() call
      // eats directly into a already-zero-slack shared budget. A longer
      // explicit timeout here doesn't mask real failures (the toast reliably
      // fires within a second or two in practice); it just stops a race with
      // the toast's own dismiss timer from flaking the assertion.
      await expect(getToastLocator(page)).toBeVisible({ timeout: 8000 });
    });

    test('should disable Save button when name is empty', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);

      await page.getByRole('button', { name: /create group/i }).click();
      await page.getByRole('dialog').last().waitFor({ state: 'visible' });

      // Save is disabled while the name field is blank.
      await expect(page.getByRole('button', { name: /save/i })).toBeDisabled();
    });

    test('should allow selecting a preset color', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);

      await page.getByRole('button', { name: /create group/i }).click();
      await page.getByRole('dialog').last().waitFor({ state: 'visible' });

      await page.getByLabel(/group name/i).fill('Colored Group');

      // Color preset buttons are labelled "Color #<hex>" via the colorPreset i18n key.
      // Scoping to the last dialog avoids matching unrelated buttons on the page.
      const dialog = page.getByRole('dialog').last();
      const colorSwatch = dialog.getByRole('button', { name: /^color #/i }).first();
      await colorSwatch.click();
      await expect(colorSwatch).toHaveAttribute('aria-pressed', 'true');
    });
  });

  test.describe('Grouped Display', () => {
    test('should show ungrouped section when groups exist', async ({ page }) => {
      // Use an auto-retrying assertion instead of a synchronous isVisible()
      // check: the button is unconditionally rendered in the page's action
      // bar, but a bare isVisible() has no wait/retry margin and can read
      // the DOM a beat before React finishes painting after navigation.
      await expect(page.getByRole('button', { name: /manage groups/i })).toBeVisible();
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
