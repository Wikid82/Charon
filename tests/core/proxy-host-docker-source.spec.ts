/**
 * Proxy Host form - local Docker source availability
 *
 * The "Local (Docker Socket)" option in the Source select must be disabled
 * (with an Orthrus hint) only when GET /api/v1/docker/status definitively
 * reports `available: false`. It stays enabled while the status query is
 * loading and fails open when the call errors or the endpoint is missing.
 *
 * The Playwright containers mount the Docker socket, so "no socket" is
 * produced by intercepting the status endpoint.
 *
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import type { Page } from '@playwright/test';

const DOCKER_STATUS_API = '**/api/v1/docker/status*';

async function openSourceSelect(page: Page) {
  await page.goto('/proxy-hosts');
  await waitForLoadingComplete(page);
  await page.getByRole('button', { name: 'Add Proxy Host' }).first().click();
  const dialog = page.getByRole('dialog', { name: /add proxy host/i });
  await expect(dialog).toBeVisible({ timeout: 8000 });

  const sourceSelect = dialog.getByRole('combobox', { name: 'Source' });
  await expect(sourceSelect).toBeVisible({ timeout: 8000 });
  await sourceSelect.click();
  return { dialog, sourceSelect };
}

test.describe('Proxy Host form - local Docker source availability', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
  });

  test('disables the Local option with an Orthrus hint when the socket is unavailable', async ({ page }) => {
    await page.route(DOCKER_STATUS_API, (route) =>
      route.fulfill({ json: { available: false, reason: 'socket_missing' } }),
    );

    const { dialog } = await openSourceSelect(page);

    await test.step('Local option is disabled', async () => {
      await expect(page.getByRole('option', { name: /Local \(Docker Socket\)/i })).toBeDisabled();
    });

    await test.step('Orthrus hint is visible', async () => {
      await expect(dialog.getByText(/Use an Orthrus agent instead/)).toBeVisible();
    });
  });

  test('keeps the Local option enabled and selectable when the socket is available', async ({ page }) => {
    await page.route(DOCKER_STATUS_API, (route) =>
      route.fulfill({ json: { available: true } }),
    );

    const { sourceSelect } = await openSourceSelect(page);
    const localOption = page.getByRole('option', { name: /Local \(Docker Socket\)/i });

    await test.step('Local option is enabled', async () => {
      await expect(localOption).toBeEnabled();
    });

    await test.step('Local option can be selected', async () => {
      await localOption.click();
      await expect(sourceSelect).toContainText(/Local \(Docker Socket\)/i);
    });
  });

  test('fails open when the status endpoint returns 500', async ({ page }) => {
    await page.route(DOCKER_STATUS_API, (route) =>
      route.fulfill({ status: 500, json: { error: 'internal error' } }),
    );

    await openSourceSelect(page);
    await expect(page.getByRole('option', { name: /Local \(Docker Socket\)/i })).toBeEnabled();
  });

  test('fails open when the status endpoint returns 404', async ({ page }) => {
    await page.route(DOCKER_STATUS_API, (route) =>
      route.fulfill({ status: 404, json: { error: 'not found' } }),
    );

    await openSourceSelect(page);
    await expect(page.getByRole('option', { name: /Local \(Docker Socket\)/i })).toBeEnabled();
  });

  test('keeps the Local option enabled while the status response is still loading', async ({ page }) => {
    let releaseStatus: () => void = () => {};
    const gate = new Promise<void>((resolve) => {
      releaseStatus = resolve;
    });
    await page.route(DOCKER_STATUS_API, async (route) => {
      await gate;
      await route.fulfill({ json: { available: false, reason: 'socket_missing' } });
    });

    await openSourceSelect(page);
    const localOption = page.getByRole('option', { name: /Local \(Docker Socket\)/i });

    await test.step('Enabled while the status call is pending', async () => {
      await expect(localOption).toBeEnabled();
    });

    await test.step('Disabled once the definitive unavailable response arrives', async () => {
      releaseStatus();
      await expect(localOption).toBeDisabled();
    });
  });
});
