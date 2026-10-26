/**
 * Security page CrowdSec toggle while a background install is busy (#1711)
 *
 * The toggle drives the engine only: the backend persists `security.crowdsec.enabled` after its
 * busy check, so a refused start or stop must leave the setting untouched. These cases use
 * route-mocked endpoints (no cscli or LAPI in the E2E container):
 * - Start answered 409 `crowdsec_busy`: busy text shown, toggle back to off, no settings write
 * - Stop answered 409 `crowdsec_busy`: busy text shown, toggle stays on, no settings write
 *
 * Settings writes go through `POST /api/v1/settings` (the client's `updateSetting`); the guard
 * below covers every mutating method so a future PUT/PATCH is caught too.
 */

import type { Page, Route } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { clickSwitch } from '../utils/ui-helpers';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import {
  CROWDSEC_ROUTES,
  HUB_JOB_ID,
  JOB_CURRENT_ROUTE,
  crowdsecFixtures,
  ensureCerberusEnabled,
  fulfillJson,
  jobStatus,
  stubCrowdSecApi,
} from '../utils/crowdsec-stubs';

const SETTINGS_ROUTE = new RegExp(String.raw`/api/v1/settings(?:\?.*)?$`);
const MUTATING_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

/** i18n: errors.crowdsec_busy */
const BUSY_TEXT = /crowdsec is busy installing a preset/i;

/** Answers the engine start or stop with the busy refusal that the backend sends during an install. */
async function refuseWithBusy(route: Route): Promise<void> {
  await fulfillJson(route, { error: 'CrowdSec is busy', error_code: 'crowdsec_busy', job_id: HUB_JOB_ID }, 409);
}

/**
 * Records every mutating settings request and answers it without reaching the backend, so a
 * stray write can be asserted on instead of changing real state.
 */
async function recordSettingsWrites(page: Page): Promise<Array<{ method: string; url: string }>> {
  const writes: Array<{ method: string; url: string }> = [];
  await page.route(SETTINGS_ROUTE, async (route) => {
    const method = route.request().method();
    if (!MUTATING_METHODS.has(method)) {
      await route.fallback();
      return;
    }
    writes.push({ method, url: route.request().url() });
    await fulfillJson(route, {});
  });
  return writes;
}

async function loadPage(page: Page): Promise<void> {
  await page.goto('/security');
  await waitForLoadingComplete(page);
}

test.describe('Security page CrowdSec toggle during a busy install @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    // The CrowdSec switch is disabled while Cerberus is off (the security-tests baseline turns it off).
    await ensureCerberusEnabled(page);
    await page.route(JOB_CURRENT_ROUTE, async (route) => {
      await fulfillJson(route, {
        job: jobStatus({ state: 'running', phase: 'install', itemsInstalled: 11, elapsedSeconds: 70 }),
      });
    });
  });

  test('should show the busy text and keep the toggle off when start is refused', async ({ page }) => {
    await stubCrowdSecApi(page, {
      status: crowdsecFixtures.runningStatus({ running: false, pid: 0, lapi_ready: false }),
    });
    const settingsWrites = await recordSettingsWrites(page);
    let startRequests = 0;
    await page.route(CROWDSEC_ROUTES.start, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      startRequests += 1;
      await refuseWithBusy(route);
    });

    await loadPage(page);
    const toggle = page.getByTestId('toggle-crowdsec');
    await expect(toggle).toBeEnabled();
    await expect(toggle).not.toBeChecked();

    await test.step('Turn CrowdSec on while the install is busy', async () => {
      await clickSwitch(toggle);
      await expect.poll(() => startRequests).toBe(1);
    });

    await test.step('Verify the translated busy message is shown', async () => {
      await expect(page.getByText(BUSY_TEXT)).toBeVisible();
    });

    await test.step('Verify the toggle returns to off once the refusal is handled', async () => {
      await expect(toggle).toBeEnabled();
      await expect(toggle).not.toBeChecked();
    });

    await test.step('Verify no security setting was written', async () => {
      expect(settingsWrites).toEqual([]);
    });
  });

  test('should show the busy text and keep the toggle on when stop is refused', async ({ page }) => {
    await stubCrowdSecApi(page, {
      status: crowdsecFixtures.runningStatus(),
    });
    const settingsWrites = await recordSettingsWrites(page);
    let stopRequests = 0;
    await page.route(CROWDSEC_ROUTES.stop, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      stopRequests += 1;
      await refuseWithBusy(route);
    });

    await loadPage(page);
    const toggle = page.getByTestId('toggle-crowdsec');
    await expect(toggle).toBeEnabled();
    await expect(toggle).toBeChecked();

    await test.step('Turn CrowdSec off while the install is busy', async () => {
      await clickSwitch(toggle);
      await expect.poll(() => stopRequests).toBe(1);
    });

    await test.step('Verify the translated busy message is shown', async () => {
      await expect(page.getByText(BUSY_TEXT)).toBeVisible();
    });

    await test.step('Verify the toggle stays on because the engine is still running', async () => {
      await expect(toggle).toBeEnabled();
      await expect(toggle).toBeChecked();
    });

    await test.step('Verify no security setting was written', async () => {
      expect(settingsWrites).toEqual([]);
    });
  });
});
