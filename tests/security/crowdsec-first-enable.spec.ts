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

test.describe('CrowdSec first-enable UX @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
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
    await toggle.click();

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
    await toggle.click();

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
    await toggle.click();

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
