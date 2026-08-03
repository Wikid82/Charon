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

/**
 * Ensure the Cerberus framework feature flag is enabled before interacting
 * with the CrowdSec toggle.
 *
 * `feature.cerberus.enabled` defaults to `false` (see
 * backend/internal/api/handlers/feature_flags_handler.go). The CrowdSec
 * Switch on the Security dashboard is `disabled` whenever Cerberus itself
 * is disabled (see `crowdsecToggleDisabled` in frontend/src/pages/Security.tsx),
 * so without this precondition the toggle is unclickable and this file's
 * assertions would hang until the test timeout. This mirrors the same
 * precondition `tests/security/security-dashboard.spec.ts` already
 * establishes via `ensureSecurityDashboardPreconditions()` for its own
 * module toggles.
 */
async function ensureCerberusEnabled(page: import('@playwright/test').Page): Promise<void> {
  await expect.poll(async () => {
    const response = await page.request.post('/api/v1/settings', {
      data: { key: 'feature.cerberus.enabled', value: 'true' },
    });
    return response.ok();
  }, {
    timeout: 10000,
    message: 'Expected feature.cerberus.enabled to be set before CrowdSec toggle assertions',
  }).toBe(true);
}

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

  test('Backend accepts empty value for setting', async ({ page }) => {
    // Confirm POST /settings with empty value returns 200 (not 400)
    const response = await page.request.post('/api/v1/settings', {
      data: { key: 'security.crowdsec.enabled', value: '' },
    });
    expect(response.status()).toBe(200);
  });
});
