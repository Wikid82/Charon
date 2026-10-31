/**
 * CrowdSec First-Enable UX E2E Tests
 *
 * Tests the UI behavior while the CrowdSec startup mutation is pending.
 * Uses route interception to simulate the slow startup without a real CrowdSec install.
 *
 * @see /projects/Charon/docs/plans/current_spec.md PR-4
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { clickSwitch } from '../utils/ui-helpers';
import { CROWDSEC_ROUTES, crowdsecFixtures, ensureCerberusEnabled } from '../utils/crowdsec-stubs';

/** English copy of the CrowdSec key notice, read from frontend/src/locales/en/translation.json. */
const KEY_NOTICE_EN = {
  title: 'CrowdSec API Key Updated',
  description:
    'Your configured API key was rejected by CrowdSec LAPI. Charon generated a new key, saved it, and uses it now to restore protection.',
  instructions:
    'Remove the old CrowdSec key variable from docker-compose.yml. If you want to keep one, set it to this key:',
  restartNote: 'No restart is needed: Charon already uses this key.',
} as const;

/** English text of the passive message shown when CrowdSec stops while it is starting. */
const ENGINE_EXITED_MESSAGE_EN =
  'CrowdSec stopped right after it started. Check the CrowdSec log for the reason.';

test.describe('CrowdSec first-enable UX @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await ensureCerberusEnabled(page);
    await page.goto('/security');
    await waitForLoadingComplete(page);
  });

  test('CrowdSec toggle stays checked while starting', async ({ page }) => {
    // Intercept start endpoint and hold the response for 2 seconds
    await page.route('**/api/v1/admin/crowdsec/start', async (route) => {
      await new Promise<void>((resolve) => setTimeout(resolve, 2000));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ pid: 123, lapi_ready: false }),
      });
    });

    const toggle = page.getByTestId('toggle-crowdsec');
    // Switch markup is `<label><input class="sr-only peer" /><div /></label>` —
    // the visible track `<div>` sits on top of the visually-hidden `<input>`
    // and intercepts pointer events, so a raw `.click()` on the input never
    // lands and retries until the test timeout. clickSwitch() clicks the
    // parent `<label>` instead, which correctly forwards the click to the
    // input (see tests/utils/ui-helpers.ts).
    await clickSwitch(toggle);

    // Immediately after click, the toggle should remain checked (user intent)
    await expect(toggle).toBeChecked();
  });

  test('CrowdSec card shows Starting badge while starting', async ({ page }) => {
    await page.route('**/api/v1/admin/crowdsec/start', async (route) => {
      await new Promise<void>((resolve) => setTimeout(resolve, 2000));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ pid: 123, lapi_ready: false }),
      });
    });

    const toggle = page.getByTestId('toggle-crowdsec');
    await clickSwitch(toggle);

    // Badge should show "Starting..." text while mutation is pending
    await expect(page.getByText('Starting...')).toBeVisible();
  });

  test('CrowdSecKeyWarning absent while starting', async ({ page }) => {
    await page.route('**/api/v1/admin/crowdsec/start', async (route) => {
      await new Promise<void>((resolve) => setTimeout(resolve, 2000));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ pid: 123, lapi_ready: false }),
      });
    });

    // Make key-status return a rejected key
    await page.route('**/api/v1/admin/crowdsec/key-status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          env_key_rejected: true,
          key_source: 'env',
          full_key: 'key123',
          current_key_preview: 'key...',
          rejected_key_preview: 'old...',
          message: 'Key rejected',
        }),
      });
    });

    const toggle = page.getByTestId('toggle-crowdsec');
    await clickSwitch(toggle);

    // The key warning alert must not be present while mutation is pending
    await expect(page.getByRole('alert', { name: /CrowdSec API Key/i })).not.toBeVisible({ timeout: 1500 });
    const keyWarning = page.locator('[role="alert"]').filter({ hasText: /CrowdSec API Key Updated/ });
    await expect(keyWarning).not.toBeVisible({ timeout: 500 });
  });

  test('CrowdSec key notice shows the new wording and no restart instruction', async ({ page }) => {
    // Status stays stopped until the start call lands, then reports running so the notice renders.
    let started = false;
    await page.route(CROWDSEC_ROUTES.status, (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      const body = started
        ? crowdsecFixtures.runningStatus({ running: true, pid: 123, lapi_ready: true })
        : crowdsecFixtures.runningStatus({ running: false, pid: 0, lapi_ready: false });
      return route.fulfill({ status: 200, contentType: 'application/json', json: body });
    });
    await page.route('**/api/v1/admin/crowdsec/start', async (route) => {
      started = true;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ pid: 123, lapi_ready: true }),
      });
    });
    await page.route('**/api/v1/admin/crowdsec/key-status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          env_key_rejected: true,
          key_source: 'auto-generated',
          full_key: 'a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6',
          current_key_preview: 'a1b2...',
          rejected_key_preview: 'old0...',
          message: 'Charon generated a new key',
        }),
      });
    });

    await test.step('Load the page with no dismissed notice and CrowdSec stopped', async () => {
      await page.evaluate(() => localStorage.removeItem('crowdsec-key-warning-dismissed'));
      await page.reload();
      await waitForLoadingComplete(page);
    });

    await test.step('Turn CrowdSec on', async () => {
      await clickSwitch(page.getByTestId('toggle-crowdsec'));
    });

    const keyNotice = page.locator('[role="alert"]').filter({ hasText: KEY_NOTICE_EN.title });

    await test.step('Show the English notice text and copy button', async () => {
      await expect(keyNotice).toBeVisible();
      await expect(keyNotice.getByText(KEY_NOTICE_EN.description)).toBeVisible();
      await expect(keyNotice.getByText(KEY_NOTICE_EN.instructions)).toBeVisible();
      await expect(keyNotice.getByRole('button', { name: 'Copy' })).toBeVisible();
      await expect(keyNotice.getByText(KEY_NOTICE_EN.restartNote)).toBeVisible();
    });

    await test.step('Do not tell the operator to restart or edit docker-compose.yml with the new key', async () => {
      await expect(keyNotice).not.toContainText(/update your docker-compose|restart (the|charon|crowdsec|your)/i);
    });
  });

  test('Backend accepts empty value for setting', async ({ page }) => {
    // Confirm POST /settings with empty value returns 200 (not 400)
    const response = await page.request.post('/api/v1/settings', {
      data: { key: 'security.crowdsec.enabled', value: '' },
    });
    expect(response.status()).toBe(200);
  });

  test('CrowdSec start that stops during the wait shows the stop message and reverts the switch', async ({ page }) => {
    await page.route('**/api/v1/admin/crowdsec/start', (route) =>
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: ENGINE_EXITED_MESSAGE_EN, error_code: 'crowdsec_engine_exited' }),
      }),
    );

    const toggle = page.getByTestId('toggle-crowdsec');
    await clickSwitch(toggle);

    await test.step('Show the passive stop message', async () => {
      await expect(page.getByText(ENGINE_EXITED_MESSAGE_EN)).toBeVisible();
    });

    await test.step('Revert the switch to off', async () => {
      await expect(toggle).not.toBeChecked();
    });
  });
});
