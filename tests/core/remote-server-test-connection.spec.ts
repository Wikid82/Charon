/**
 * Remote server (Sources) custom connection test E2E Tests
 *
 * The "Test Connection" button in the Add Source form runs the custom dial test. A refused destination shows the
 * fixed destination text, an unreachable target shows the plain connection failure text, and a reachable target
 * shows the measured latency. The form is never submitted, so no server is created and nothing needs cleanup.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const DESTINATION_REFUSED_TEXT = 'This destination is not allowed.';
const CONNECTION_FAILED_TEXT = 'Could not connect to the target.';

test.describe('Remote server custom connection test', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await page.goto('/hecate/remote-servers');
    await waitForLoadingComplete(page);
    await page.getByRole('button', { name: /add source/i }).click();
    await expect(page.getByRole('heading', { name: /add source/i })).toBeVisible();
  });

  /** Fills the direct-connection host and port of the Add Source form. */
  async function enterTarget(page: import('@playwright/test').Page, host: string, port: string): Promise<void> {
    await page.locator('#host').fill(host);
    await page.locator('#port').fill(port);
  }

  test('Refused destination shows the fixed destination text in the form', async ({ page }) => {
    await enterTarget(page, '169.254.169.254', '80');

    await page.getByRole('button', { name: /test connection/i }).click();

    await expect(page.getByText(DESTINATION_REFUSED_TEXT)).toBeVisible();
    await expect(page.getByTestId('remote-server-test-latency')).toBeHidden();
  });

  test('Unreachable target shows the plain connection failure text in the form', async ({ page }) => {
    await enterTarget(page, '127.0.0.1', '1');

    await page.getByRole('button', { name: /test connection/i }).click();

    await expect(page.getByText(CONNECTION_FAILED_TEXT)).toBeVisible();
    await expect(page.getByTestId('remote-server-test-latency')).toBeHidden();
  });

  test('Reachable target shows the measured latency next to Test Connection', async ({ page }) => {
    // The application listens on loopback port 8080 inside the test container.
    await enterTarget(page, '127.0.0.1', '8080');

    await page.getByRole('button', { name: /test connection/i }).click();

    await expect(page.getByTestId('remote-server-test-latency')).toHaveText(/Reachable in \d+ ms/);
  });
});
