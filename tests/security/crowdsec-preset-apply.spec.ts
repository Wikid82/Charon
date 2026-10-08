/**
 * CrowdSec Curated Preset Apply E2E Tests
 *
 * Verifies that applying a curated CrowdSec preset reports the real outcome:
 * - Success toast only when the backend returns status 'applied'
 * - Errors (500/503/504) are surfaced and never shown as success
 * - Curated presets never fall back to a local file write
 *
 * The apply endpoint is stubbed so the tests are deterministic and do not
 * require cscli in the E2E container.
 *
 * Enabled by the curated-preset apply fix (docs/plans/current_spec.md).
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const APPLY_ROUTE = '**/api/v1/admin/crowdsec/presets/apply';
const FILE_WRITE_ROUTE = '**/api/v1/admin/crowdsec/file';
const CURATED_PRESET_TITLE = 'Honeypot Friendly Defaults';

/** Records POSTs to the generic file-write endpoint (the local fallback path). */
function trackFileWrites(page: Page): { count: () => number } {
  let writes = 0;
  void page.route(FILE_WRITE_ROUTE, async (route) => {
    if (route.request().method() === 'POST') {
      writes += 1;
    }
    await route.continue();
  });
  return { count: () => writes };
}

async function selectCuratedPresetAndApply(page: Page): Promise<void> {
  await test.step('Select the curated preset', async () => {
    await page.getByRole('button', { name: new RegExp(CURATED_PRESET_TITLE, 'i') }).click();
  });
  await test.step('Click Apply Preset', async () => {
    await page.getByRole('button', { name: /apply preset/i }).click();
  });
}

test.describe('CrowdSec Curated Preset Apply @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/security/crowdsec');
    await waitForLoadingComplete(page);
  });

  test('should show success toast with reload note when backend reports applied', async ({ page }) => {
    const fileWrites = trackFileWrites(page);
    await page.route(APPLY_ROUTE, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        json: {
          status: 'applied',
          used_cscli: true,
          backup: '/app/data/backups/crowdsec-preset-test.tar.gz',
          reload_hint: true,
        },
      });
    });

    await selectCuratedPresetAndApply(page);

    await test.step('Verify success toast mentions reload', async () => {
      await expect(page.getByText(/preset applied.*reload required/i)).toBeVisible();
    });
    await test.step('Verify no local file write happened', async () => {
      expect(fileWrites.count()).toBe(0);
    });
  });

  for (const status of [500, 503, 504]) {
    test(`should show an error and no success toast when apply returns ${status}`, async ({ page }) => {
      const fileWrites = trackFileWrites(page);
      const message = 'CrowdSec CLI is not available; curated preset could not be applied';
      await page.route(APPLY_ROUTE, async (route) => {
        await route.fulfill({
          status,
          contentType: 'application/json',
          json: { error: message },
        });
      });

      await selectCuratedPresetAndApply(page);

      await test.step('Verify the error is surfaced', async () => {
        await expect(page.getByText(message).first()).toBeVisible();
      });
      await test.step('Verify no success toast is shown', async () => {
        await expect(page.getByText(/preset applied/i)).toHaveCount(0);
      });
      await test.step('Verify no local fallback write', async () => {
        expect(fileWrites.count()).toBe(0);
      });
    });
  }

  test('should not fall back to a local write when apply returns 501 for a curated preset', async ({ page }) => {
    const fileWrites = trackFileWrites(page);
    await page.route(APPLY_ROUTE, async (route) => {
      await route.fulfill({
        status: 501,
        contentType: 'application/json',
        json: { error: 'preset apply not implemented' },
      });
    });

    await selectCuratedPresetAndApply(page);

    await test.step('Verify error shown and no success toast', async () => {
      await expect(page.getByText(/preset apply not implemented/i).first()).toBeVisible();
      await expect(page.getByText(/preset applied/i)).toHaveCount(0);
    });
    await test.step('Verify no local fallback write', async () => {
      expect(fileWrites.count()).toBe(0);
    });
  });

  test('should not show success toast when a 200 response has a non-applied status', async ({ page }) => {
    const fileWrites = trackFileWrites(page);
    await page.route(APPLY_ROUTE, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        json: { status: 'failed', used_cscli: false, reload_hint: false },
      });
    });

    await selectCuratedPresetAndApply(page);

    await test.step('Verify the incomplete status is surfaced and no success toast is shown', async () => {
      await expect(page.getByText(/preset apply did not complete \(status: failed\)/i).first()).toBeVisible();
      await expect(page.getByText(/preset applied/i)).toHaveCount(0);
    });
    await test.step('Verify no local fallback write', async () => {
      expect(fileWrites.count()).toBe(0);
    });
  });
});
