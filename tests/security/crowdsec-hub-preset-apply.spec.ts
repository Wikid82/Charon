/**
 * CrowdSec Hub Preset Apply E2E Tests
 *
 * Verifies that applying a non-curated (hub) CrowdSec preset reports the real
 * outcome and never falls back to a client-side file write:
 * - Success toast only when the backend returns status 'applied'
 * - Errors (500/503/504) are surfaced and never shown as success
 * - HTTP 501 shows an error instead of writing the preview locally
 *
 * The preset list, pull and apply endpoints are stubbed so the tests are
 * deterministic and do not require the CrowdSec hub or cscli.
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const PRESETS_ROUTE = '**/api/v1/admin/crowdsec/presets';
const PULL_ROUTE = '**/api/v1/admin/crowdsec/presets/pull';
const APPLY_ROUTE = '**/api/v1/admin/crowdsec/presets/apply';
const FILE_WRITE_ROUTE = '**/api/v1/admin/crowdsec/file';
const HUB_PRESET_SLUG = 'crowdsecurity/e2e-hub-preset';
const HUB_PRESET_TITLE = 'E2E Hub Preset';
const CACHE_KEY = 'e2e-hub-cache-key';

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

/** Stubs the preset list and pull endpoints so a single hub preset is available. */
async function stubHubPreset(page: Page): Promise<void> {
  await page.route(PRESETS_ROUTE, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      json: {
        presets: [
          {
            slug: HUB_PRESET_SLUG,
            title: HUB_PRESET_TITLE,
            summary: 'Deterministic hub preset for E2E',
            source: 'hub',
            requires_hub: true,
            available: true,
            cached: true,
            cache_key: CACHE_KEY,
          },
        ],
      },
    });
  });
  await page.route(PULL_ROUTE, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      json: {
        status: 'pulled',
        slug: HUB_PRESET_SLUG,
        preview: 'name: e2e-hub-preset\n',
        cache_key: CACHE_KEY,
        source: 'hub',
      },
    });
  });
}

async function openPageAndApplyHubPreset(page: Page): Promise<void> {
  await test.step('Reload so the stubbed preset list is used', async () => {
    await page.goto('/security/crowdsec');
    await waitForLoadingComplete(page);
  });
  await test.step('Select the hub preset', async () => {
    await page.getByRole('button', { name: new RegExp(HUB_PRESET_TITLE, 'i') }).click();
  });
  await test.step('Click Apply Preset', async () => {
    await page.getByRole('button', { name: /apply preset/i }).click();
  });
}

test.describe('CrowdSec Hub Preset Apply @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await stubHubPreset(page);
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
          backup: 'crowdsec.backup.20260101-000000.000000',
          reload_hint: true,
          cache_key: CACHE_KEY,
        },
      });
    });

    await openPageAndApplyHubPreset(page);

    await test.step('Verify success toast mentions reload', async () => {
      await expect(page.getByText(/preset applied.*reload required/i)).toBeVisible();
    });
    await test.step('Verify no local file write happened', async () => {
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

    await openPageAndApplyHubPreset(page);

    await test.step('Verify the incomplete status is surfaced and no success toast is shown', async () => {
      await expect(page.getByText(/preset apply did not complete \(status: failed\)/i).first()).toBeVisible();
      await expect(page.getByText(/preset applied/i)).toHaveCount(0);
    });
    await test.step('Verify no local fallback write', async () => {
      expect(fileWrites.count()).toBe(0);
    });
  });

  for (const status of [500, 503, 504]) {
    test(`should show an error and no success toast when hub apply returns ${status}`, async ({ page }) => {
      const fileWrites = trackFileWrites(page);
      const message = 'hub preset could not be applied';
      await page.route(APPLY_ROUTE, async (route) => {
        await route.fulfill({
          status,
          contentType: 'application/json',
          json: { error: message },
        });
      });

      await openPageAndApplyHubPreset(page);

      await test.step('Verify an error is surfaced', async () => {
        await expect(page.getByText(message).first()).toBeVisible();
      });
      await test.step('Verify no success toast is shown', async () => {
        await expect(page.getByText(/preset applied/i)).toHaveCount(0);
      });
      await test.step('Verify zero local fallback writes', async () => {
        expect(fileWrites.count()).toBe(0);
      });
    });
  }

  test('should show an error and perform no write when hub apply returns 501', async ({ page }) => {
    const fileWrites = trackFileWrites(page);
    await page.route(APPLY_ROUTE, async (route) => {
      await route.fulfill({
        status: 501,
        contentType: 'application/json',
        json: { error: 'preset apply not implemented' },
      });
    });

    await openPageAndApplyHubPreset(page);

    await test.step('Verify error shown and no success or local-apply toast', async () => {
      await expect(page.getByText(/preset apply not implemented/i).first()).toBeVisible();
      await expect(page.getByText(/preset applied/i)).toHaveCount(0);
      await expect(page.getByText(/applying locally/i)).toHaveCount(0);
    });
    await test.step('Verify zero local fallback writes', async () => {
      expect(fileWrites.count()).toBe(0);
    });
  });
});
