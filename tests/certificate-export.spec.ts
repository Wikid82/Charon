/**
 * Certificate Export E2E Tests
 *
 * Tests the certificate export dialog UX:
 * - Export button in certificate list opens export dialog
 * - Dialog displays format radio group (PEM, PFX/PKCS#12, DER)
 * - Format selection updates the active radio button
 * - Include private key checkbox shown when cert has a key
 * - Password field appears when include private key is checked
 * - PFX password field appears when PFX format is selected
 * - Cancel closes dialog without exporting
 * - Escape key closes dialog
 * - Successful PEM export triggers file download
 * - Export with include key requires password re-authentication
 * - Dialog accessibility: keyboard navigation, dialog role, labels
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import { test, expect, loginUser } from './fixtures/auth-fixtures';
import { request as playwrightRequest } from '@playwright/test';
import {
  waitForLoadingComplete,
  waitForDialog,
  waitForAPIResponse,
} from './utils/wait-helpers';
import { generateUniqueId } from './fixtures/test-data';
import { STORAGE_STATE } from './constants';
import { getStorageStateAuthHeaders } from './utils/api-helpers';
import { getTestCertificatePair } from './utils/test-certificate';

const CERTIFICATES_API = /\/api\/v1\/certificates/;

/** Self-signed certificate and key, generated once per run (see utils/test-certificate.ts). */
const { certPem: REAL_TEST_CERT, keyPem: REAL_TEST_KEY } = getTestCertificatePair();

/**
 * Create a custom certificate directly via the API.
 * Returns the numeric cert ID (from list endpoint), UUID, and name.
 */
async function createCustomCertViaAPI(
  baseURL: string,
): Promise<{ id: number; uuid: string; certName: string }> {
  const id = generateUniqueId();
  const certName = `export-cert-${id}`;

  const ctx = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
    extraHTTPHeaders: getStorageStateAuthHeaders(),
  });

  try {
    const response = await ctx.post('/api/v1/certificates', {
      multipart: {
        name: certName,
        certificate_file: {
          name: 'cert.pem',
          mimeType: 'application/x-pem-file',
          buffer: Buffer.from(REAL_TEST_CERT),
        },
        key_file: {
          name: 'key.pem',
          mimeType: 'application/x-pem-file',
          buffer: Buffer.from(REAL_TEST_KEY),
        },
      },
    });

    if (!response.ok()) {
      throw new Error(
        `Failed to create certificate: ${response.status()} ${await response.text()}`,
      );
    }

    const createResult = await response.json();
    const certUUID: string = createResult.uuid;

    const listResponse = await ctx.get('/api/v1/certificates');
    if (!listResponse.ok()) {
      throw new Error(`Failed to list certificates: ${listResponse.status()}`);
    }
    const certs: Array<{ id: number; uuid: string }> = await listResponse.json();
    const match = certs.find((c) => c.uuid === certUUID);
    if (!match) {
      throw new Error(`Certificate with UUID ${certUUID} not found in list after creation`);
    }

    return { id: match.id, uuid: certUUID, certName };
  } finally {
    await ctx.dispose();
  }
}

/**
 * Delete a certificate directly via the API for cleanup.
 */
async function deleteCertViaAPI(baseURL: string, certId: number): Promise<void> {
  const ctx = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
    extraHTTPHeaders: getStorageStateAuthHeaders(),
  });

  try {
    await ctx.delete(`/api/v1/certificates/${certId}`);
  } finally {
    await ctx.dispose();
  }
}

/**
 * Navigate to the certificates page and wait for data to load.
 */
async function navigateToCertificates(
  page: import('@playwright/test').Page,
): Promise<void> {
  const certsResponse = waitForAPIResponse(page, CERTIFICATES_API);
  await page.goto('/certificates');
  await certsResponse;
  await waitForLoadingComplete(page);
}

test.describe('Certificate Export', () => {
  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
  const createdCertIds: number[] = [];

  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.afterAll(async () => {
    for (const certId of createdCertIds) {
      await deleteCertViaAPI(baseURL, certId).catch(() => {});
    }
  });

  // ---------------------------------------------------------------------------
  // Scenario 1: Export button opens the export dialog
  // ---------------------------------------------------------------------------
  test('should open export dialog when export button is clicked', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate to certificates page', async () => {
      await navigateToCertificates(page);
    });

    await test.step('Click the export button for the seeded cert', async () => {
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });

      const exportButton = certRow.getByRole('button', { name: /export/i });
      await exportButton.click();
    });

    await test.step('Verify export dialog opens with correct title', async () => {
      const dialog = await waitForDialog(page);
      await expect(dialog).toBeVisible();

      await expect(dialog.getByText(/Export Certificate/i)).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 2: Dialog shows format radio group with PEM selected by default
  // ---------------------------------------------------------------------------
  test('should show format radio group with PEM selected by default', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      const exportButton = certRow.getByRole('button', { name: /export/i });
      await exportButton.click();
      await waitForDialog(page);
    });

    await test.step('Verify format radio group is present', async () => {
      const dialog = page.getByRole('dialog');
      const radioGroup = dialog.getByRole('radiogroup');
      await expect(radioGroup).toBeVisible();
    });

    await test.step('Verify PEM is selected by default', async () => {
      const dialog = page.getByRole('dialog');
      const pemRadio = dialog.getByRole('radio', { name: /PEM/i });
      await expect(pemRadio).toHaveAttribute('aria-checked', 'true');
    });

    await test.step('Verify all three format options are present', async () => {
      const dialog = page.getByRole('dialog');
      await expect(dialog.getByRole('radio', { name: /PEM/i })).toBeVisible();
      await expect(dialog.getByRole('radio', { name: /PFX/i })).toBeVisible();
      await expect(dialog.getByRole('radio', { name: /DER/i })).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 3: Selecting a different format updates the radio state
  // ---------------------------------------------------------------------------
  test('should update radio state when different format is selected', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Select DER format and verify state', async () => {
      const dialog = page.getByRole('dialog');
      const derRadio = dialog.getByRole('radio', { name: /DER/i });
      await derRadio.click();

      await expect(derRadio).toHaveAttribute('aria-checked', 'true');

      const pemRadio = dialog.getByRole('radio', { name: /PEM/i });
      await expect(pemRadio).toHaveAttribute('aria-checked', 'false');
    });

    await test.step('Select PFX format and verify state', async () => {
      const dialog = page.getByRole('dialog');
      const pfxRadio = dialog.getByRole('radio', { name: /PFX/i });
      await pfxRadio.click();

      await expect(pfxRadio).toHaveAttribute('aria-checked', 'true');

      const derRadio = dialog.getByRole('radio', { name: /DER/i });
      await expect(derRadio).toHaveAttribute('aria-checked', 'false');
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 4: Include private key checkbox is visible for cert with key
  // ---------------------------------------------------------------------------
  test('should show include private key checkbox for cert with key', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate with key via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Verify include private key checkbox is present', async () => {
      const dialog = page.getByRole('dialog');
      const includeKeyCheckbox = dialog.locator('#include-key');
      await expect(includeKeyCheckbox).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 5: Checking include key shows password field
  // ---------------------------------------------------------------------------
  test('should show password field when include private key is checked', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate with key via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Verify password field is hidden initially', async () => {
      const dialog = page.getByRole('dialog');
      const passwordInput = dialog.locator('#export-password');
      await expect(passwordInput).toHaveCount(0);
    });

    await test.step('Check include private key checkbox', async () => {
      const dialog = page.getByRole('dialog');
      const includeKeyCheckbox = dialog.locator('#include-key');
      await includeKeyCheckbox.check();
    });

    await test.step('Verify password field and warning appear', async () => {
      const dialog = page.getByRole('dialog');

      const passwordInput = dialog.locator('#export-password');
      await expect(passwordInput).toBeVisible();

      const warning = dialog.getByText(/re-authentication/i);
      await expect(warning).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 6: Selecting PFX format shows PFX password field
  // ---------------------------------------------------------------------------
  test('should show PFX password field when PFX format is selected', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Verify PFX password is hidden with PEM selected', async () => {
      const dialog = page.getByRole('dialog');
      const pfxPasswordInput = dialog.locator('#pfx-password');
      await expect(pfxPasswordInput).toHaveCount(0);
    });

    await test.step('Select PFX format and verify PFX password appears', async () => {
      const dialog = page.getByRole('dialog');
      const pfxRadio = dialog.getByRole('radio', { name: /PFX/i });
      await pfxRadio.click();

      const pfxPasswordInput = dialog.locator('#pfx-password');
      await expect(pfxPasswordInput).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 7: Cancel closes dialog without exporting
  // ---------------------------------------------------------------------------
  test('should close dialog without exporting when Cancel is clicked', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Click Cancel and verify dialog closes', async () => {
      const dialog = page.getByRole('dialog');
      const cancelButton = dialog.getByRole('button', { name: /cancel/i });
      await cancelButton.click();

      await expect(dialog).not.toBeVisible({ timeout: 3000 });
    });

    await test.step('Verify certificate row is still present', async () => {
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 8: Escape key closes dialog
  // ---------------------------------------------------------------------------
  test('should close dialog when Escape key is pressed', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Press Escape and verify dialog closes', async () => {
      const dialog = page.getByRole('dialog');
      await page.keyboard.press('Escape');
      await expect(dialog).not.toBeVisible({ timeout: 3000 });
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 9: Successful PEM export triggers file download
  // ---------------------------------------------------------------------------
  test('should download PEM file on successful export', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Submit export with PEM format and verify download', async () => {
      const dialog = page.getByRole('dialog');

      // PEM is default — click export
      const downloadPromise = page.waitForEvent('download', { timeout: 15000 });
      const exportButton = dialog.locator('[data-testid="export-certificate-submit"]');
      await exportButton.click();

      const download = await downloadPromise;
      expect(download.suggestedFilename()).toMatch(/\.pem$/);
    });

    await test.step('Verify dialog closed after successful export', async () => {
      const dialog = page.getByRole('dialog');
      await expect(dialog).not.toBeVisible({ timeout: 5000 });
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 10: Export with include key but no password is rejected
  // ---------------------------------------------------------------------------
  test('should require password when exporting with private key', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate with key via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Check include key and try to submit without password', async () => {
      const dialog = page.getByRole('dialog');
      const includeKeyCheckbox = dialog.locator('#include-key');
      await includeKeyCheckbox.check();

      // Password field should be required
      const passwordInput = dialog.locator('#export-password');
      await expect(passwordInput).toBeVisible();
      await expect(passwordInput).toHaveAttribute('required', '');
    });

    await test.step('Submit without password — dialog should remain open', async () => {
      const dialog = page.getByRole('dialog');
      const exportButton = dialog.locator('[data-testid="export-certificate-submit"]');
      await exportButton.click();

      // Dialog should still be visible (HTML5 validation prevents submission)
      await expect(dialog).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 11: Dialog resets form state when reopened
  // ---------------------------------------------------------------------------
  test('should reset form state when dialog is reopened', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Change format to DER and check include key', async () => {
      const dialog = page.getByRole('dialog');
      const derRadio = dialog.getByRole('radio', { name: /DER/i });
      await derRadio.click();

      const includeKeyCheckbox = dialog.locator('#include-key');
      await includeKeyCheckbox.check();
    });

    await test.step('Close the dialog', async () => {
      const dialog = page.getByRole('dialog');
      const cancelButton = dialog.getByRole('button', { name: /cancel/i });
      await cancelButton.click();
      await expect(dialog).not.toBeVisible({ timeout: 3000 });
    });

    await test.step('Reopen dialog and verify form is reset', async () => {
      const certRow = page.getByRole('row').filter({ hasText: certName });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);

      const dialog = page.getByRole('dialog');

      // PEM should be selected again (default)
      const pemRadio = dialog.getByRole('radio', { name: /PEM/i });
      await expect(pemRadio).toHaveAttribute('aria-checked', 'true');

      // Include key checkbox should be unchecked
      const includeKeyCheckbox = dialog.locator('#include-key');
      await expect(includeKeyCheckbox).not.toBeChecked();

      // Password field should not be visible
      const passwordInput = dialog.locator('#export-password');
      await expect(passwordInput).toHaveCount(0);
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 12: Dialog accessibility — proper ARIA roles and labels
  // ---------------------------------------------------------------------------
  test('should have proper ARIA roles and keyboard accessibility', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertIds.push(result.id);
      certName = result.certName;
    });

    await test.step('Navigate and open export dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      certRow.getByRole('button', { name: /export/i }).click();
      await waitForDialog(page);
    });

    await test.step('Verify dialog has proper ARIA role', async () => {
      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible();
    });

    await test.step('Verify dialog has a heading', async () => {
      const dialog = page.getByRole('dialog');
      const heading = dialog.getByRole('heading');
      await expect(heading).toBeVisible();
    });

    await test.step('Verify radiogroup has an accessible label', async () => {
      const dialog = page.getByRole('dialog');
      const radioGroup = dialog.getByRole('radiogroup');
      await expect(radioGroup).toHaveAttribute('aria-label');
    });

    await test.step('Verify keyboard navigation through format options', async () => {
      // Tab into the dialog — focus should land on an interactive element
      await page.keyboard.press('Tab');
      const focusedElement = page.locator(':focus');
      await expect(focusedElement).toBeVisible();
    });

    await test.step('Verify export button label', async () => {
      const dialog = page.getByRole('dialog');
      const exportButton = dialog.locator('[data-testid="export-certificate-submit"]');
      await expect(exportButton).toBeVisible();
      await expect(exportButton).toContainText(/export/i);
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 13: Every cert row has an export button
  // ---------------------------------------------------------------------------
  test('should show export button for all certificates in the list', async ({ page }) => {
    await test.step('Navigate to certificates page', async () => {
      await navigateToCertificates(page);
    });

    await test.step('Verify each data row has an export button', async () => {
      const rows = page.locator('tbody tr');
      const rowCount = await rows.count();

      if (rowCount === 0) {
        // Only empty state — nothing to verify
        return;
      }

      for (let i = 0; i < rowCount; i++) {
        const row = rows.nth(i);
        const cellCount = await row.locator('td').count();

        // Skip empty-state row (has colspan)
        if (cellCount < 4) continue;

        const exportBtn = row.getByRole('button', { name: /export/i });
        await expect(exportBtn).toBeVisible();
      }
    });
  });
});
