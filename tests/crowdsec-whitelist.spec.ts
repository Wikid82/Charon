import { test, expect, request as playwrightRequest } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import {
  withSecurityEnabled,
  captureSecurityState,
  setSecurityModuleEnabled,
  getSecurityStatus,
} from './utils/security-helpers';
import { getStorageStateAuthHeaders } from './utils/api-helpers';
import { STORAGE_STATE } from './constants';

/**
 * CrowdSec IP Whitelist Management E2E Tests
 *
 * Tests the whitelist tab on the CrowdSec configuration page (/security/crowdsec).
 * The tab is conditionally rendered: it only appears when CrowdSec mode is not 'disabled'.
 *
 * Uses IPs in the 10.99.x.x range to avoid conflicts with real network addresses.
 *
 * NOTE: Uses request.newContext({ storageState }) instead of the `request` fixture because
 * the auth cookie has `secure: true` which the fixture won't send over HTTP, but
 * Playwright's APIRequestContext does send it.
 */

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:8080';
const TEST_IP_PREFIX = '10.99';

function createRequestContext(): Promise<APIRequestContext> {
  return playwrightRequest.newContext({
    baseURL: BASE_URL,
    storageState: STORAGE_STATE,
    extraHTTPHeaders: getStorageStateAuthHeaders(),
  });
}

test.describe('CrowdSec IP Whitelist Management', () => {
  // Serial mode prevents the tab-visibility test (which disables CrowdSec) from
  // racing with the local-mode tests (which require CrowdSec enabled).
  test.describe.configure({ mode: 'serial' });

  test.describe('tab visibility', () => {
    test('whitelist tab is hidden when CrowdSec is disabled', async ({ page }) => {
      const rc = await createRequestContext();
      const originalState = await captureSecurityState(rc);
      if (originalState.crowdsec) {
        await setSecurityModuleEnabled(rc, 'crowdsec', false);
      }

      try {
        await test.step('Navigate to CrowdSec config page', async () => {
          await page.goto('/security/crowdsec');
          await page.waitForLoadState('networkidle');
        });

        await test.step('Verify whitelist tab is not present', async () => {
          await expect(page.getByRole('tab', { name: 'Whitelist' })).not.toBeVisible();
        });
      } finally {
        if (originalState.crowdsec) {
          await setSecurityModuleEnabled(rc, 'crowdsec', true);
        }
        await rc.dispose();
      }
    });
  });

  test.describe('with CrowdSec in local mode', () => {
    let rc: APIRequestContext;
    let cleanupSecurity: () => Promise<void>;

    test.beforeAll(async () => {
      rc = await createRequestContext();
      cleanupSecurity = await withSecurityEnabled(rc, { crowdsec: true, cerberus: true });

      // Wait for CrowdSec to enter local mode (may take a few seconds after enabling)
      for (let attempt = 0; attempt < 15; attempt++) {
        const statusResp = await rc.get('/api/v1/security/status');
        if (statusResp.ok()) {
          const status = await statusResp.json();
          if (status.crowdsec?.mode !== 'disabled') break;
        }
        await new Promise((resolve) => setTimeout(resolve, 2000));
      }
    });

    test.afterAll(async () => {
      // Remove any leftover test entries before restoring security state
      const resp = await rc.get('/api/v1/admin/crowdsec/whitelist');
      if (resp.ok()) {
        const data = await resp.json();
        for (const entry of (data.whitelist ?? []) as Array<{ uuid: string; ip_or_cidr: string }>) {
          if (entry.ip_or_cidr.startsWith(TEST_IP_PREFIX)) {
            await rc.delete(`/api/v1/admin/crowdsec/whitelist/${entry.uuid}`);
          }
        }
      }
      await cleanupSecurity?.();
      await rc.dispose();
    });

    test.beforeEach(async ({ page }) => {
      await test.step('Open CrowdSec Whitelist tab', async () => {
        // CrowdSec may take time to enter local mode after being enabled.
        // Retry navigation until the Whitelist tab is visible.
        const maxAttempts = 15;
        let tabFound = false;
        for (let attempt = 0; attempt < maxAttempts; attempt++) {
          await page.goto('/security/crowdsec');
          // Wait for network to settle so React Query status fetch completes
          await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
          const whitelistTab = page.getByRole('tab', { name: 'Whitelist' });
          const visible = await whitelistTab.isVisible().catch(() => false);
          if (visible) {
            await whitelistTab.click();
            await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
            tabFound = true;
            break;
          }
          if (attempt < maxAttempts - 1) {
            await new Promise((resolve) => setTimeout(resolve, 2000));
          }
        }
        if (!tabFound) {
          // Fail with a clear error message if tab never appeared
          await expect(page.getByRole('tab', { name: 'Whitelist' })).toBeVisible({
            timeout: 1000,
          });
        }
      });
    });

    test('displays empty state when no whitelist entries exist', async ({ page }) => {
      await test.step('Verify empty state message and snapshot', async () => {
        const emptyEl = page.getByTestId('whitelist-empty');
        await expect(emptyEl).toBeVisible();
        await expect(emptyEl).toHaveText('No whitelist entries');

        await expect(emptyEl).toMatchAriaSnapshot(`
          - paragraph: No whitelist entries
        `);
      });
    });

    test('adds a valid IPv4 address to the whitelist', async ({ page }) => {
      const testIP = `${TEST_IP_PREFIX}.1.10`;
      let addedUUID: string | null = null;

      try {
        await test.step('Fill IP address and reason fields', async () => {
          await page.getByTestId('whitelist-ip-input').fill(testIP);
          await page.getByTestId('whitelist-reason-input').fill('IPv4 E2E test entry');
        });

        await test.step('Submit the form and capture response', async () => {
          const responsePromise = page.waitForResponse(
            (resp) =>
              resp.url().includes('/api/v1/admin/crowdsec/whitelist') &&
              resp.request().method() === 'POST'
          );
          await page.getByTestId('whitelist-add-btn').click();
          const response = await responsePromise;
          expect(response.status()).toBe(201);
          const body = await response.json();
          addedUUID = body.uuid as string;
        });

        await test.step('Verify the entry appears in the table', async () => {
          await expect(page.getByRole('cell', { name: testIP, exact: true })).toBeVisible({ timeout: 10_000 });
          await expect(page.getByRole('cell', { name: 'IPv4 E2E test entry' })).toBeVisible({ timeout: 10_000 });
        });
      } finally {
        if (addedUUID) {
          await rc.delete(`/api/v1/admin/crowdsec/whitelist/${addedUUID}`);
        }
      }
    });

    test('adds a valid CIDR range to the whitelist', async ({ page }) => {
      const testCIDR = `${TEST_IP_PREFIX}.2.0/24`;
      let addedUUID: string | null = null;

      try {
        await test.step('Fill CIDR notation and reason', async () => {
          await page.getByTestId('whitelist-ip-input').fill(testCIDR);
          await page.getByTestId('whitelist-reason-input').fill('CIDR E2E test range');
        });

        await test.step('Submit the form and capture response', async () => {
          const responsePromise = page.waitForResponse(
            (resp) =>
              resp.url().includes('/api/v1/admin/crowdsec/whitelist') &&
              resp.request().method() === 'POST'
          );
          await page.getByTestId('whitelist-add-btn').click();
          const response = await responsePromise;
          expect(response.status()).toBe(201);
          const body = await response.json();
          addedUUID = body.uuid as string;
        });

        await test.step('Verify CIDR entry appears in the table', async () => {
          await expect(page.getByRole('cell', { name: testCIDR, exact: true })).toBeVisible({ timeout: 10_000 });
          await expect(page.getByRole('cell', { name: 'CIDR E2E test range' })).toBeVisible({ timeout: 10_000 });
        });
      } finally {
        if (addedUUID) {
          await rc.delete(`/api/v1/admin/crowdsec/whitelist/${addedUUID}`);
        }
      }
    });

    test('"Add My IP" button pre-fills the detected client IP', async ({ page }) => {
      const ipResp = await rc.get('/api/v1/system/my-ip');
      expect(ipResp.ok()).toBeTruthy();
      const { ip: detectedIP } = await ipResp.json() as { ip: string };

      await test.step('Click the "Add My IP" button', async () => {
        await page.getByTestId('whitelist-add-my-ip-btn').click();
      });

      await test.step('Verify the IP input is pre-filled with the detected IP', async () => {
        await expect(page.getByTestId('whitelist-ip-input')).toHaveValue(detectedIP);
      });
    });

    test('shows an inline validation error for an invalid IP address', async ({ page }) => {
      await test.step('Fill the IP field with an invalid value', async () => {
        await page.getByTestId('whitelist-ip-input').fill('not-an-ip');
      });

      await test.step('Submit the form', async () => {
        await page.getByTestId('whitelist-add-btn').click();
      });

      await test.step('Verify the inline error element is visible with an error message', async () => {
        const errorEl = page.getByTestId('whitelist-ip-error');
        await expect(errorEl).toBeVisible();
        await expect(errorEl).toContainText(/invalid/i);
      });
    });

    test('shows a conflict error when adding a duplicate whitelist entry', async ({ page }) => {
      const testIP = `${TEST_IP_PREFIX}.3.10`;
      let addedUUID: string | null = null;

      try {
        await test.step('Pre-seed the whitelist entry via API', async () => {
          const addResp = await rc.post('/api/v1/admin/crowdsec/whitelist', {
            data: { ip_or_cidr: testIP, reason: 'duplicate seed' },
          });
          expect(addResp.status()).toBe(201);
          const body = await addResp.json();
          addedUUID = body.uuid as string;
        });

        await test.step('Reload the whitelist tab to see the seeded entry', async () => {
          await page.goto('/security/crowdsec');
          const whitelistTab = page.getByRole('tab', { name: 'Whitelist' });
          await expect(whitelistTab).toBeVisible({ timeout: 15_000 });
          await whitelistTab.click();
          await expect(page.getByRole('cell', { name: testIP, exact: true })).toBeVisible({ timeout: 10_000 });
        });

        await test.step('Attempt to add the same IP again', async () => {
          await page.getByTestId('whitelist-ip-input').fill(testIP);
          await page.getByTestId('whitelist-add-btn').click();
        });

        await test.step('Verify the conflict error is shown inline', async () => {
          const errorEl = page.getByTestId('whitelist-ip-error');
          await expect(errorEl).toBeVisible();
          await expect(errorEl).toContainText(/already exists/i);
        });
      } finally {
        if (addedUUID) {
          await rc.delete(`/api/v1/admin/crowdsec/whitelist/${addedUUID}`);
        }
      }
    });

    test('removes a whitelist entry via the delete confirmation modal', async ({ page }) => {
      const testIP = `${TEST_IP_PREFIX}.4.10`;
      let addedUUID: string | null = null;

      try {
        await test.step('Pre-seed a whitelist entry via API', async () => {
          const addResp = await rc.post('/api/v1/admin/crowdsec/whitelist', {
            data: { ip_or_cidr: testIP, reason: 'delete modal test' },
          });
          expect(addResp.status()).toBe(201);
          const body = await addResp.json();
          addedUUID = body.uuid as string;
        });

        await test.step('Reload the whitelist tab to see the seeded entry', async () => {
          await page.goto('/security/crowdsec');
          const whitelistTab = page.getByRole('tab', { name: 'Whitelist' });
          await expect(whitelistTab).toBeVisible({ timeout: 15_000 });
          await whitelistTab.click();
          await expect(page.getByRole('cell', { name: testIP, exact: true })).toBeVisible({ timeout: 10_000 });
        });

        await test.step('Click the delete button for the entry', async () => {
          const deleteBtn = page.getByRole('button', {
            name: new RegExp(`Remove whitelist entry for ${testIP.replace(/\./g, '\\.')}`, 'i'),
          });
          await expect(deleteBtn).toBeVisible();
          await deleteBtn.click();
        });

        await test.step('Verify the confirmation modal appears', async () => {
          const modal = page.getByRole('dialog');
          await expect(modal).toBeVisible();
          await expect(modal.locator('#whitelist-delete-modal-title')).toHaveText(
            'Remove Whitelist Entry'
          );
          await expect(modal).toMatchAriaSnapshot(`
            - dialog:
              - heading "Remove Whitelist Entry" [level=2]
          `);
        });

        await test.step('Confirm deletion and verify the entry is removed', async () => {
          const deleteResponsePromise = page.waitForResponse(
            (resp) =>
              resp.url().includes('/api/v1/admin/crowdsec/whitelist/') &&
              resp.request().method() === 'DELETE'
          );
          await page.getByRole('button', { name: 'Remove', exact: true }).click();
          const deleteResponse = await deleteResponsePromise;
          expect(deleteResponse.ok()).toBeTruthy();
          addedUUID = null; // cleaned up by the UI action

          await expect(page.getByRole('cell', { name: testIP, exact: true })).not.toBeVisible();
          await expect(page.getByTestId('whitelist-empty')).toBeVisible();
        });
      } finally {
        // Fallback cleanup if the UI delete failed
        if (addedUUID) {
          await rc.delete(`/api/v1/admin/crowdsec/whitelist/${addedUUID}`);
        }
      }
    });

    test('delete confirmation modal is dismissed by the Cancel button', async ({ page }) => {
      const testIP = `${TEST_IP_PREFIX}.5.10`;
      let addedUUID: string | null = null;

      try {
        await test.step('Pre-seed a whitelist entry via API', async () => {
          const addResp = await rc.post('/api/v1/admin/crowdsec/whitelist', {
            data: { ip_or_cidr: testIP, reason: 'cancel modal test' },
          });
          expect(addResp.status()).toBe(201);
          const body = await addResp.json();
          addedUUID = body.uuid as string;
        });

        await test.step('Reload the whitelist tab', async () => {
          await page.goto('/security/crowdsec');
          const whitelistTab = page.getByRole('tab', { name: 'Whitelist' });
          await expect(whitelistTab).toBeVisible({ timeout: 15_000 });
          await whitelistTab.click();
          await expect(page.getByRole('cell', { name: testIP, exact: true })).toBeVisible({ timeout: 10_000 });
        });

        await test.step('Open the delete modal', async () => {
          const deleteBtn = page.getByRole('button', {
            name: new RegExp(`Remove whitelist entry for ${testIP.replace(/\./g, '\\.')}`, 'i'),
          });
          await deleteBtn.click();
          await expect(page.getByRole('dialog')).toBeVisible();
        });

        await test.step('Cancel and verify the entry is still present', async () => {
          await page.getByRole('button', { name: 'Cancel' }).click();
          await expect(page.getByRole('dialog')).not.toBeVisible();
          await expect(page.getByRole('cell', { name: testIP, exact: true })).toBeVisible();
        });
      } finally {
        if (addedUUID) {
          await rc.delete(`/api/v1/admin/crowdsec/whitelist/${addedUUID}`);
        }
      }
    });

    test('add button is disabled when the IP field is empty', async ({ page }) => {
      await test.step('Verify add button is disabled with empty IP field', async () => {
        const ipInput = page.getByTestId('whitelist-ip-input');
        const addBtn = page.getByTestId('whitelist-add-btn');

        await expect(ipInput).toHaveValue('');
        await expect(addBtn).toBeDisabled();
      });

      await test.step('Button becomes enabled when IP is entered', async () => {
        await page.getByTestId('whitelist-ip-input').fill('192.168.1.1');
        await expect(page.getByTestId('whitelist-add-btn')).toBeEnabled();
      });

      await test.step('Button returns to disabled state when IP is cleared', async () => {
        await page.getByTestId('whitelist-ip-input').clear();
        await expect(page.getByTestId('whitelist-add-btn')).toBeDisabled();
      });
    });
  });
});
