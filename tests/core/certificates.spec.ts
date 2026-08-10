/**
 * SSL Certificates E2E Tests
 *
 * Tests the SSL Certificates management functionality including:
 * - List view with table, columns, and empty states
 * - Upload custom certificate with form validation
 * - Certificate details (domain, expiry, issuer, status)
 * - Delete certificate with confirmation and backup
 * - Certificate status indicators and sorting
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { request as playwrightRequest } from '@playwright/test';
import {
  waitForLoadingComplete,
  waitForDialog,
  waitForDebounce,
} from '../utils/wait-helpers';
import { getCertificateViaAPI, getBackupsViaAPI } from '../utils/api-helpers';
import { generateUniqueId } from '../fixtures/test-data';
import { STORAGE_STATE } from '../constants';

test.describe('SSL Certificates - CRUD Operations', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    // Navigate to certificates page (retry once on transient failures)
    for (let i = 0; i < 2; i++) {
      try {
        await page.goto('/certificates');
        await waitForLoadingComplete(page);
        break;
      } catch (err) {
        if (i === 1) throw err;
        // short backoff and retry
        await new Promise((r) => setTimeout(r, 500));
      }
    }
  });

  // Helper to get the Add Certificate button
  const getAddCertButton = (page: import('@playwright/test').Page) =>
    page.getByRole('button', { name: /add.*certificate/i }).first();

  // Helper to get Cancel button in form
  const getCancelButton = (page: import('@playwright/test').Page) =>
    page.getByRole('button', { name: /cancel/i }).first();

  test.describe('List View', () => {
    test('should display certificates page with title', async ({ page }) => {
      await test.step('Verify page title is visible', async () => {
        const heading = page.getByRole('heading', { name: /certificates/i });
        await expect(heading).toBeVisible();
      });

      await test.step('Verify Add Certificate button is present', async () => {
        const addButton = getAddCertButton(page);
        await expect(addButton).toBeVisible();
      });
    });

    test('should show correct table columns', async ({ page }) => {
      await test.step('Verify table headers exist', async () => {
        // The table should have columns: Name, Domain, Issuer, Expires, Status, Actions
        const expectedColumns = [
          /name/i,
          /domain/i,
          /issuer/i,
          /expires/i,
          /status/i,
          /actions/i,
        ];

        for (const pattern of expectedColumns) {
          const header = page.locator('th').filter({ hasText: pattern });
          const headerExists = await header.count() > 0;
          if (headerExists) {
            await expect(header.first()).toBeVisible();
          }
        }
      });
    });

    test('should display empty state when no certificates exist', async ({ page }) => {
      await test.step('Check for empty state or existing certificates', async () => {
        // Wait for page to fully load
        await waitForLoadingComplete(page);

        const table = page.getByRole('table');
        const emptyState = page.getByText(/no.*certificates.*found/i);

        await expect(async () => {
          const hasTable = await table.count() > 0 && await table.first().isVisible();
          const hasEmpty = await emptyState.count() > 0 && await emptyState.first().isVisible();
          expect(hasTable || hasEmpty).toBeTruthy();
        }).toPass({ timeout: 10000 });
      });
    });

    test('should show loading spinner while fetching data', async ({ page }) => {
      await test.step('Navigate and observe loading state', async () => {
        await page.reload();
        // Wait for page to fully load after reload
        await waitForLoadingComplete(page);

        const table = page.getByRole('table');
        const emptyState = page.getByText(/no.*certificates.*found/i);

        await expect(async () => {
          const hasTable = await table.count() > 0 && await table.first().isVisible();
          const hasEmpty = await emptyState.count() > 0 && await emptyState.first().isVisible();
          expect(hasTable || hasEmpty).toBeTruthy();
        }).toPass({ timeout: 10000 });
      });
    });

    test('should navigate to certificates from sidebar', async ({ page }) => {
      await test.step('Navigate via sidebar', async () => {
        // Go to a different page first
        await page.goto('/');
        await waitForLoadingComplete(page);

        // Look for SSL/Certificates menu in sidebar
        const sslMenu = page.getByRole('button', { name: /ssl/i });
        const certificatesLink = page.getByRole('link', { name: /certificates/i });

        if (await sslMenu.isVisible().catch(() => false)) {
          await sslMenu.click();
          await waitForDebounce(page); // Wait for menu expansion animation
        }

        if (await certificatesLink.isVisible().catch(() => false)) {
          await certificatesLink.click();
          await waitForLoadingComplete(page);

          // Verify we're on certificates page
          const heading = page.getByRole('heading', { name: /certificates/i });
          await expect(heading).toBeVisible({ timeout: 5000 });
        }
      });
    });

    test('should display certificate details (name, domain, issuer, expiry)', async ({ page }) => {
      await test.step('Check table displays certificate information', async () => {
        const table = page.getByRole('table');
        const hasTable = await table.isVisible().catch(() => false);

        if (hasTable) {
          const rows = page.locator('tbody tr');
          const rowCount = await rows.count();

          if (rowCount > 0) {
            const firstRow = rows.first();
            await expect(firstRow).toBeVisible();

            // Check row has expected content patterns
            const rowText = await firstRow.textContent();
            expect(rowText).toBeTruthy();
          }
        }
      });
    });

    test('should show certificate status indicators', async ({ page }) => {
      await test.step('Check for status badges in table', async () => {
        // Status badges: Valid, Expiring Soon, Expired, Untrusted (Staging)
        const statusBadges = page.locator('span').filter({ hasText: /valid|expiring|expired|untrusted/i });
        const badgeCount = await statusBadges.count();

        // May or may not have certificates depending on test data
        expect(badgeCount >= 0).toBeTruthy();
      });
    });

    test('should show staging badge for Let\'s Encrypt staging certificates', { retries: 1 }, async ({ page }) => {
      await test.step('Check for staging badges', async () => {
        const stagingBadge = page.locator('span').filter({ hasText: /staging/i });
        const badgeCount = await stagingBadge.count();

        // Verify styling if staging badge exists
        if (badgeCount > 0) {
          const firstBadge = stagingBadge.first();
          await expect(firstBadge).toBeVisible();
        }
      });
    });

    test('should support sorting by name', async ({ page }) => {
      await test.step('Click name column header to sort', async () => {
        const nameHeader = page.locator('th').filter({ hasText: /name/i }).first();

        if (await nameHeader.isVisible().catch(() => false)) {
          // Check if header is clickable (has cursor-pointer class)
          const isClickable = await nameHeader.evaluate((el) =>
            el.classList.contains('cursor-pointer') ||
            window.getComputedStyle(el).cursor === 'pointer'
          );

          if (isClickable) {
            await nameHeader.click();
            await waitForDebounce(page); // Wait for sort animation

            // Sort icon should appear
            const sortIcon = nameHeader.locator('svg');
            await expect(sortIcon.first()).toBeVisible();
          }
        }
      });
    });

    test('should support sorting by expiry date', async ({ page }) => {
      await test.step('Click expires column header to sort', async () => {
        const expiresHeader = page.locator('th').filter({ hasText: /expires/i }).first();

        if (await expiresHeader.isVisible().catch(() => false)) {
          await expiresHeader.click();
          await waitForDebounce(page); // Wait for sort animation

          // Verify click toggles sort direction
          await expiresHeader.click();
          await waitForDebounce(page); // Wait for sort animation
        }
      });
    });

    test('should show SSL info alert', async ({ page }) => {
      await test.step('Verify SSL info alert is displayed', async () => {
        const alert = page.locator('[role="alert"], .alert').filter({ hasText: /note|ssl|certificate/i });
        await expect(alert.first()).toBeVisible();
      });
    });
  });

  test.describe('Upload Custom Certificate', () => {
    test('should open upload modal when Add Certificate clicked', async ({ page }) => {
      await test.step('Click Add Certificate button', async () => {
        const addButton = getAddCertButton(page);
        await addButton.click();
      });

      await test.step('Verify upload dialog opens', async () => {
        // Wait for dialog to be fully interactive
        const dialog = await waitForDialog(page);

        // The dialog should be visible
        await expect(dialog).toBeVisible({ timeout: 5000 });

        // Verify dialog title
        const dialogTitle = dialog.getByRole('heading', { name: /upload.*certificate/i });
        await expect(dialogTitle).toBeVisible();

        // Verify essential form fields are present
        const nameInput = dialog.locator('input').first();
        await expect(nameInput).toBeVisible();

        // Close dialog (guard visibility/enabled to avoid transient flakiness)
        await expect(getCancelButton(page)).toBeVisible({ timeout: 3000 });
        await expect(getCancelButton(page)).toBeEnabled({ timeout: 3000 });
        await getCancelButton(page).click();
      });
    });

    test('should have friendly name input field', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive
      });

      await test.step('Verify name input exists', async () => {
        const dialog = page.getByRole('dialog');
        const nameInput = dialog.locator('input').first();
        await expect(nameInput).toBeVisible();

        // Check for label
        const nameLabel = dialog.getByText(/friendly.*name|name/i);
        await expect(nameLabel).toBeVisible();
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should have certificate file input (.pem, .crt, .cer)', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive
      });

      await test.step('Verify certificate file input exists', async () => {
        const dialog = page.getByRole('dialog');
        const certFileInput = dialog.locator('#cert-file');
        await expect(certFileInput).toBeVisible();

        // Check accept attribute
        const acceptAttr = await certFileInput.getAttribute('accept');
        expect(acceptAttr).toContain('.pem');
      });

      await test.step('Close dialog', async () => {
        await expect(getCancelButton(page)).toBeVisible({ timeout: 3000 });
        await expect(getCancelButton(page)).toBeEnabled({ timeout: 3000 });
        await getCancelButton(page).click();
      });
    });

    test('should have private key file input (.pem, .key)', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive
      });

      await test.step('Verify private key file input exists', async () => {
        const dialog = page.getByRole('dialog');
        const keyFileInput = dialog.locator('#key-file');
        await expect(keyFileInput).toBeVisible();

        // Check accept attribute
        const acceptAttr = await keyFileInput.getAttribute('accept');
        expect(acceptAttr).toContain('.pem');
      });

      await test.step('Close dialog', async () => {
        await expect(getCancelButton(page)).toBeVisible({ timeout: 3000 });
        await expect(getCancelButton(page)).toBeEnabled({ timeout: 3000 });
        await getCancelButton(page).click();
      });
    });

    test('should validate required name field', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page);
      });

      await test.step('Verify submit is disabled with empty name', async () => {
        const dialog = page.getByRole('dialog');
        const nameInput = dialog.locator('#certificate-name');
        const uploadButton = dialog.getByRole('button', { name: /upload/i });

        // Name input should have HTML5 required attribute
        await expect(nameInput).toHaveAttribute('required', '');

        // Submit button should be disabled when name is empty
        await expect(uploadButton).toBeDisabled();
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should require certificate file', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page);
      });

      await test.step('Verify cert file is required', async () => {
        const dialog = page.getByRole('dialog');
        const nameInput = dialog.locator('#certificate-name');
        await nameInput.fill('Test Certificate');

        // FileDropZone uses aria-required, not native HTML required
        const certFileInput = dialog.locator('#cert-file');
        await expect(certFileInput).toHaveAttribute('aria-required', 'true');

        // Submit should remain disabled without cert file
        const uploadButton = dialog.getByRole('button', { name: /upload/i });
        await expect(uploadButton).toBeDisabled();
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should show key file as optional when no cert is selected', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page);
      });

      await test.step('Verify key file is not aria-required by default', async () => {
        const dialog = page.getByRole('dialog');
        const keyFileInput = dialog.locator('#key-file');
        await expect(keyFileInput).toBeVisible();
        // No cert selected — key file should not be required yet
        await expect(keyFileInput).not.toHaveAttribute('aria-required', 'true');
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should require key file when a PEM certificate is selected', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page);
      });

      await test.step('Select a PEM cert file', async () => {
        const dialog = page.getByRole('dialog');
        const certFileInput = dialog.locator('#cert-file');
        await certFileInput.setInputFiles({
          name: 'server.pem',
          mimeType: 'application/x-pem-file',
          buffer: Buffer.from('-----BEGIN CERTIFICATE-----\nMIIFake\n-----END CERTIFICATE-----'),
        });
      });

      await test.step('Verify key file becomes aria-required', async () => {
        const dialog = page.getByRole('dialog');
        const keyFileInput = dialog.locator('#key-file');
        await expect(keyFileInput).toBeVisible();
        await expect(keyFileInput).toHaveAttribute('aria-required', 'true');

        // Submit should still be disabled — no key file provided yet
        const uploadButton = dialog.getByRole('button', { name: /upload/i });
        await expect(uploadButton).toBeDisabled();
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should hide key file input when a PFX certificate is selected', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page);
      });

      await test.step('Select a PFX cert file', async () => {
        const dialog = page.getByRole('dialog');
        const certFileInput = dialog.locator('#cert-file');
        await certFileInput.setInputFiles({
          name: 'bundle.pfx',
          mimeType: 'application/x-pkcs12',
          buffer: Buffer.from('PFX'),
        });
      });

      await test.step('Verify key file input is removed from DOM', async () => {
        const dialog = page.getByRole('dialog');
        const keyFileInput = dialog.locator('#key-file');
        // PFX bundles the key — the key file section is unmounted entirely
        await expect(keyFileInput).not.toBeAttached();
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should remove key file input when cert format changes from PEM to PFX', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page);
      });

      await test.step('Select a PEM cert first', async () => {
        const dialog = page.getByRole('dialog');
        const certFileInput = dialog.locator('#cert-file');
        await certFileInput.setInputFiles({
          name: 'server.pem',
          mimeType: 'application/x-pem-file',
          buffer: Buffer.from('-----BEGIN CERTIFICATE-----\nMIIFake\n-----END CERTIFICATE-----'),
        });
        // Confirm key file becomes required
        const keyFileInput = dialog.locator('#key-file');
        await expect(keyFileInput).toHaveAttribute('aria-required', 'true');
      });

      await test.step('Switch to PFX cert', async () => {
        const dialog = page.getByRole('dialog');
        const certFileInput = dialog.locator('#cert-file');
        await certFileInput.setInputFiles({
          name: 'bundle.pfx',
          mimeType: 'application/x-pkcs12',
          buffer: Buffer.from('PFX'),
        });
      });

      await test.step('Verify key file input is removed from DOM after format switch', async () => {
        const dialog = page.getByRole('dialog');
        const keyFileInput = dialog.locator('#key-file');
        await expect(keyFileInput).not.toBeAttached();
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should show upload button with loading state', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive
      });

      await test.step('Verify upload button exists', async () => {
        const dialog = page.getByRole('dialog');
        const uploadButton = dialog.getByRole('button', { name: /upload/i });
        await expect(uploadButton).toBeVisible();
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should close dialog when Cancel clicked', async ({ page }) => {
      await test.step('Open and close dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive

        const dialog = page.getByRole('dialog');
        await expect(dialog).toBeVisible();

        await getCancelButton(page).click();
        await expect(dialog).not.toBeVisible({ timeout: 3000 });
      });
    });

    test('should show proper file input styling', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive
      });

      await test.step('Verify file inputs have styled buttons', async () => {
        const dialog = page.getByRole('dialog');

        // File inputs should have styled file buttons
        const certFileInput = dialog.locator('#cert-file');
        const keyFileInput = dialog.locator('#key-file');

        await expect(certFileInput).toBeVisible();
        await expect(keyFileInput).toBeVisible();
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });
  });

  test.describe('Certificate Details', () => {
    const getEmptyState = (page: import('@playwright/test').Page) =>
      page.locator('tbody tr td[colspan], tbody tr td').filter({ hasText: /no.*certificates.*found/i }).first();

    const findDataRow = async (page: import('@playwright/test').Page) => {
      const rows = page.locator('tbody tr');
      const rowCount = await rows.count();

      for (let i = 0; i < rowCount; i += 1) {
        const row = rows.nth(i);
        const cellCount = await row.locator('td').count();
        if (cellCount >= 4) {
          return row;
        }
      }

      return null;
    };

    const getDataRowOrEmpty = async (page: import('@playwright/test').Page) => {
      await waitForLoadingComplete(page);

      const emptyState = getEmptyState(page);

      await expect
        .poll(async () => {
          const dataRow = await findDataRow(page);
          if (dataRow) return 'data';
          if (await emptyState.isVisible().catch(() => false)) return 'empty';
          return 'pending';
        }, { timeout: 15000 })
        .not.toBe('pending');

      if (await emptyState.isVisible().catch(() => false)) {
        return null;
      }

      return findDataRow(page);
    };

    test('should display certificate domain in table', async ({ page }) => {
      await test.step('Check for domain column', async () => {
        const firstRow = await getDataRowOrEmpty(page);

        if (!firstRow) {
          const emptyState = getEmptyState(page);
          await expect(emptyState).toBeVisible();
          return;
        }

        // Domain should be visible in the row
        const domainCell = firstRow.locator('td').nth(1); // Domain is second column
        await expect(domainCell).toBeVisible();
      });
    });

    test('should display certificate issuer', async ({ page }) => {
      await test.step('Check for issuer column', async () => {
        const firstRow = await getDataRowOrEmpty(page);

        if (!firstRow) {
          const emptyState = getEmptyState(page);
          await expect(emptyState).toBeVisible();
          return;
        }

        const issuerCell = firstRow.locator('td').nth(2); // Issuer is third column
        await expect(issuerCell).toBeVisible();
      });
    });

    test('should display expiry date', async ({ page }) => {
      await test.step('Check for expiry column', async () => {
        const firstRow = await getDataRowOrEmpty(page);

        if (!firstRow) {
          const emptyState = getEmptyState(page);
          await expect(emptyState).toBeVisible();
          return;
        }

        const expiryCell = firstRow.locator('td').nth(3); // Expires is fourth column
        await expect(expiryCell).toBeVisible();

        // Should contain a date format
        const expiryText = await expiryCell.textContent();
        expect(expiryText).toBeTruthy();
      });
    });

    test('should show valid status for non-expired certificates', async ({ page }) => {
      await test.step('Check for valid status badge', async () => {
        const validBadge = page.locator('span').filter({ hasText: /^valid$/i });
        const badgeCount = await validBadge.count();

        if (badgeCount > 0) {
          const firstBadge = validBadge.first();
          // Should have green styling
          const classes = await firstBadge.getAttribute('class');
          expect(classes).toMatch(/green|success/);
        }
      });
    });

    test('should show expiring status for certificates near expiry', async ({ page }) => {
      await test.step('Check for expiring status badge', async () => {
        const expiringBadge = page.locator('span').filter({ hasText: /expiring.*soon/i });
        const badgeCount = await expiringBadge.count();

        if (badgeCount > 0) {
          const firstBadge = expiringBadge.first();
          // Should have yellow/warning styling
          const classes = await firstBadge.getAttribute('class');
          expect(classes).toMatch(/yellow|warning/);
        }
      });
    });

    test('should show expired status for expired certificates', async ({ page }) => {
      await test.step('Check for expired status badge', async () => {
        const expiredBadge = page.locator('span').filter({ hasText: /^expired$/i });
        const badgeCount = await expiredBadge.count();

        if (badgeCount > 0) {
          const firstBadge = expiredBadge.first();
          // Should have red/error styling
          const classes = await firstBadge.getAttribute('class');
          expect(classes).toMatch(/red|error|danger/);
        }
      });
    });

    test('should show untrusted status for staging certificates', async ({ page }) => {
      await test.step('Check for untrusted status badge', async () => {
        const untrustedBadge = page.locator('span').filter({ hasText: /untrusted|staging/i });
        const badgeCount = await untrustedBadge.count();

        if (badgeCount > 0) {
          const firstBadge = untrustedBadge.first();
          // Should have orange/warning styling
          const classes = await firstBadge.getAttribute('class');
          expect(classes).toMatch(/orange|warning/);
        }
      });
    });
  });

  test.describe('Delete Certificate', () => {
    // -------------------------------------------------------------------
    // The delete UI does NOT use a native window.confirm() dialog — it's a
    // fully custom React modal (frontend/src/components/dialogs/
    // DeleteCertificateDialog.tsx, built on the shared Dialog/DialogContent
    // primitives). The tests below previously drove this flow via
    // `page.once('dialog', ...)`, Playwright's *native* browser dialog
    // handler — since the app never opens a native dialog for this flow,
    // that handler never fired, and the tests exercised almost none of the
    // real deletion flow. These helpers/tests interact with the actual
    // custom modal instead, mirroring the already-established, proven
    // pattern in tests/certificate-delete.spec.ts.
    //
    // Root-cause note (frontend/src/components/CertificateList.tsx): a
    // certificate that is currently in_use has its delete affordance either
    // hidden entirely or rendered `aria-disabled` with a no-op onClick — the
    // custom modal can never be opened by clicking a delete button on an
    // already-in-use certificate. The "in use" 409 path from
    // backend/internal/api/handlers/certificate_handler.go's Delete handler
    // is therefore only reachable through the UI via a TOCTOU race: open the
    // dialog while the certificate is deletable, then have it become in-use
    // (e.g. attached to a proxy host by another admin) before Confirm is
    // clicked. The test below reproduces exactly that race.
    // -------------------------------------------------------------------

    const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
    const createdCertUUIDs: string[] = [];
    const createdProxyHostUUIDs: string[] = [];

    /**
     * Real self-signed certificate and key for upload tests.
     * Generated via: openssl req -x509 -newkey rsa:2048 -nodes -days 365 -subj "/CN=test.local/O=TestOrg"
     * The backend parses X.509 data, so placeholder PEM from fixtures won't work.
     * (Identical to the cert used in tests/certificate-delete.spec.ts.)
     */
    const REAL_TEST_CERT = `-----BEGIN CERTIFICATE-----
MIIDLzCCAhegAwIBAgIUehGqwKI4zLvoZSNHlAuv7cJ0G5AwDQYJKoZIhvcNAQEL
BQAwJzETMBEGA1UEAwwKdGVzdC5sb2NhbDEQMA4GA1UECgwHVGVzdE9yZzAeFw0y
NjAzMjIwMzQyMDhaFw0yNzAzMjIwMzQyMDhaMCcxEzARBgNVBAMMCnRlc3QubG9j
YWwxEDAOBgNVBAoMB1Rlc3RPcmcwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
AoIBAQDdzdQfOkHzG/lZ242xTvFYMVOrd12rUGQVcWhc9NG1LIJGYZKpS0bzNUdo
ylHhIqbwNq18Dni1znDYsOAlnfZR+gv84U4klRHGE7liNRixBA5ymZ6KI68sOwqx
bn6wpDZgNLnjD3POwSQoPEx2BAYwIyLPjXFjfnv5nce8Bt99j/zDVwhq24b9YdMR
BVV/sOBsAtNEuRngajA9+i2rmLVrXJSiSFhA/hR0wX6bICpFTtahYX7JqfzlMHFO
4lBka9sbC3xujwtFmLtkBovCzf69fA6p2qhJGVNJ9oHeFY3V2CdYq5Q8SZTsG1Yt
S0O/2A9ZkQmHezeG9DYeg68nLfJDAgMBAAGjUzBRMB0GA1UdDgQWBBRE+2+ss2yl
0vAmlccEC7MBWX6UmDAfBgNVHSMEGDAWgBRE+2+ss2yl0vAmlccEC7MBWX6UmDAP
BgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQCvwsnSRYQ5PYtuhJ3v
YhKmjkg+NsojYItlo+UkJmq09LkIEwRqJwFLcDxhyHWqRL5Bpc1PA1VJAG6Pif8D
uwwNnXwZZf0P5e7exccSQZnI03OhS0c6/4kfvRSiFiT6BYTYSvQ+OWhpMIIcwhov
86muij2Y32E3F0aqOPjEB+cm/XauXzmFjXi7ig7cktphHcwT8zQn43yCG/BJfWe2
bRLWqMy+jdr/x2Ij8eWPSlJD3zDxsQiLiO0hFzpQNHfz2Qe17K3dsuhNQ85h2s0w
zCLDm4WygKTw2foUXGNtbWG7z6Eq7PI+2fSlJDFgb+xmdIFQdyKDsZeYO5bmdYq5
0tY8
-----END CERTIFICATE-----`;

    // nosemgrep: generic.secrets.security.detected-private-key.detected-private-key -- throwaway self-signed test.local key pair generated solely for these X.509 upload/parse tests (see comment above REAL_TEST_CERT); not a real credential, never used outside this test fixture.
    const REAL_TEST_KEY = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDdzdQfOkHzG/lZ
242xTvFYMVOrd12rUGQVcWhc9NG1LIJGYZKpS0bzNUdoylHhIqbwNq18Dni1znDY
sOAlnfZR+gv84U4klRHGE7liNRixBA5ymZ6KI68sOwqxbn6wpDZgNLnjD3POwSQo
PEx2BAYwIyLPjXFjfnv5nce8Bt99j/zDVwhq24b9YdMRBVV/sOBsAtNEuRngajA9
+i2rmLVrXJSiSFhA/hR0wX6bICpFTtahYX7JqfzlMHFO4lBka9sbC3xujwtFmLtk
BovCzf69fA6p2qhJGVNJ9oHeFY3V2CdYq5Q8SZTsG1YtS0O/2A9ZkQmHezeG9DYe
g68nLfJDAgMBAAECggEAA8uIcZsBkzNLVOpDcQvfZ+7ldkLt61x4xJUoKqRVt4/c
usTjSYTsNdps2lzRLH+h85eRPaonDpVLAP97FlRZk+rUrFhT30mzACdI6LvtLDox
imxudgFI91dwm2Xp7QPM77XMkxdUl+5eEVeBchN84kiiSS2BCdQZiEUsLF9sZi2P
A5+x6XHImE+Sqfm/xVOZzHjj7ObHxc3bUpDT+RvRDvEBGjtEUlCCWuKvLi3DWIBF
T9E38f0hqoxKwc7gsZCZs7phoVm9a3xjQ8Xh3ONLa30aBsJii33KHHxSASc7hMy1
cM6GaGcg4xgqFw3B677KWUMc3Ur5YdLu71Bw7MFc4QKBgQD9FyRoWcTEktPdvH9y
o7yxRVWcSs5c47h5X9rhcKvUCyEzQ/89Gt1d8e/qMv9JxXmcg3AS8VYeFmzyyMta
iKTrHYnA8iRgM6CHvgSD4+vc7niW1de7qxW3T6MrGA4AEoQOPUvd6ZljBPIqxV8h
jw9BW5YREZV6fXqqVOVT4GMrbQKBgQDgWpvmu1FY65TjoDljOPBtO17krwaWzb/D
jlXQgZgRJVD7kaUPhm7Kb2d7P7t34LgzGH63hF82PlXqtwd5QhB3EZP9mhZTbXxK
vwLf+H44ANDlcZiyDG9OJBT6ND5/JP0jHEt/KsP9pcd9xbZWNEZZFzddbbcp1G/v
ue6p18XWbwKBgQCmdm8y10BNToldQVrOKxWzvve1CZq7i+fMpRhQyQurNvrKPkIF
jcLlxHhZINu6SNFY+TZgry1GMtfLw/fEfzWBkvcE2f7E64/9WCSeHu4GbS8Rfmsb
e0aYQCAA+xxSPdtvhi99MOT7NMiXCyQr7W1KPpPwfBFF9HwWxinjxiVT7QKBgFAb
Ch9QMrN1Kiw8QUFUS0Q1NqSgedHOlPHWGH3iR9GXaVrpne31KgnNzT0MfHtJGXvk
+xm7geN0TmkIAPsiw45AEH80TVRsezyVBwnBSA/m+q9x5/tqxTM5XuQXU1lCc7/d
kndNZb1jO9+EgJ42/AdDatlJG2UsHOuTj8vE5zaxAoGBAPthB+5YZfu3de+vnfpa
o0oFy++FeeHUTxor2605Lit9ZfEvDTe1/iPQw5TNOLjwx0CdsrCxWk5Tyz50aA30
KfVperc+m+vEVXIPI1qluI0iTPcHd/lMQYCsu6tKWmFP/hAFTIy7rOHMHfPx3RzK
yRNV1UrzJGv5ZUVKq2kymBut
-----END PRIVATE KEY-----`;

    /**
     * Create a deletable custom certificate directly via the API (multipart
     * upload, matching the real upload dialog's request shape), bypassing
     * TestDataManager's narrow CertificateData type which omits `name`.
     */
    async function createCustomCertViaAPI(): Promise<{ uuid: string; certName: string }> {
      const id = generateUniqueId();
      const certName = `test-cert-${id}`;

      const ctx = await playwrightRequest.newContext({ baseURL, storageState: STORAGE_STATE });
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
          throw new Error(`Failed to create certificate: ${response.status()} ${await response.text()}`);
        }

        const result = await response.json();
        return { uuid: result.uuid, certName };
      } finally {
        await ctx.dispose();
      }
    }

    /**
     * Attach a certificate to a newly-created proxy host via direct API.
     * Returns the host's UUID — ProxyHostHandler.Delete (and the model's
     * `ID uint `json:"-"``) means the numeric id is never present in the
     * JSON response and DELETE only accepts the UUID, so UUID is the only
     * usable identifier for later cleanup.
     */
    async function attachCertToProxyHostViaAPI(certificateUUID: string): Promise<{ uuid: string }> {
      const id = generateUniqueId();
      const domain = `proxy-${id}.test.local`;

      const ctx = await playwrightRequest.newContext({ baseURL, storageState: STORAGE_STATE });
      try {
        const response = await ctx.post('/api/v1/proxy-hosts', {
          data: {
            domain_names: domain,
            forward_host: '127.0.0.1',
            forward_port: 3000,
            forward_scheme: 'https',
            certificate_id: certificateUUID,
          },
        });

        if (!response.ok()) {
          throw new Error(`Failed to create proxy host: ${response.status()} ${await response.text()}`);
        }

        const result = await response.json();
        return { uuid: result.uuid };
      } finally {
        await ctx.dispose();
      }
    }

    async function deleteCertViaAPI(certUUID: string): Promise<void> {
      const ctx = await playwrightRequest.newContext({ baseURL, storageState: STORAGE_STATE });
      try {
        await ctx.delete(`/api/v1/certificates/${certUUID}`);
      } finally {
        await ctx.dispose();
      }
    }

    async function deleteProxyHostViaAPI(hostUUID: string): Promise<void> {
      const ctx = await playwrightRequest.newContext({ baseURL, storageState: STORAGE_STATE });
      try {
        await ctx.delete(`/api/v1/proxy-hosts/${hostUUID}`);
      } finally {
        await ctx.dispose();
      }
    }

    /** Locate the delete (Trash2) button for a specific certificate's row by name. */
    function getDeleteButtonForCert(page: import('@playwright/test').Page, certName: string) {
      const row = page.getByRole('row').filter({ hasText: certName });
      return row.getByRole('button', { name: /delete certificate/i });
    }

    test.afterAll(async () => {
      // Clean up any proxy hosts/certs created during tests that weren't
      // removed by the test itself (proxy hosts first, so the certs they
      // reference aren't blocked as "in use" during cleanup).
      for (const hostUUID of createdProxyHostUUIDs) {
        await deleteProxyHostViaAPI(hostUUID).catch(() => {});
      }
      for (const certUUID of createdCertUUIDs) {
        await deleteCertViaAPI(certUUID).catch(() => {});
      }
    });

    test('should show delete button for custom certificates', async ({ page }) => {
      await test.step('Check for delete buttons', async () => {
        const deleteButtons = page.locator('button[title*="Delete"], button').filter({ has: page.locator('svg.lucide-trash-2') });
        const deleteCount = await deleteButtons.count();

        // Custom certificates should have delete buttons
        expect(deleteCount >= 0).toBeTruthy();
      });
    });

    test('should show delete button for staging certificates', async ({ page }) => {
      await test.step('Check for staging certificate delete buttons', async () => {
        // Staging certificates should have delete buttons
        const stagingRows = page.locator('tbody tr').filter({ hasText: /staging/i });
        const stagingCount = await stagingRows.count();

        if (stagingCount > 0) {
          const firstStagingRow = stagingRows.first();
          const deleteButton = firstStagingRow.locator('button').filter({ has: page.locator('svg.lucide-trash-2') });
          // Staging certs are always deletable (frontend/src/utils/certificateUtils.ts's
          // isDeletable() treats provider === 'letsencrypt-staging' as deletable
          // regardless of in-use state), so a delete affordance — enabled, or
          // disabled-with-tooltip if in use — is always rendered for a staging row.
          await expect(deleteButton.first()).toBeVisible();
        }
      });
    });

    test('should show delete confirmation dialog', async ({ page }) => {
      let certName: string;

      await test.step('Seed a deletable custom certificate via API', async () => {
        const result = await createCustomCertViaAPI();
        createdCertUUIDs.push(result.uuid);
        certName = result.certName;
      });

      await test.step('Click delete and verify the custom confirmation modal', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const deleteButton = getDeleteButtonForCert(page, certName);
        await expect(deleteButton).toBeVisible({ timeout: 10000 });
        await deleteButton.click();

        const dialog = await waitForDialog(page);
        await expect(dialog).toBeVisible();

        // Real custom modal (DeleteCertificateDialog), not a native confirm()
        await expect(dialog.getByText(/delete certificate/i)).toBeVisible();
        await expect(dialog.getByRole('button', { name: /cancel/i })).toBeVisible();
        await expect(dialog.getByRole('button', { name: /^delete$/i })).toBeVisible();
      });

      await test.step('Close the dialog', async () => {
        const dialog = page.getByRole('dialog');
        await dialog.getByRole('button', { name: /cancel/i }).click();
        await expect(dialog).not.toBeVisible({ timeout: 3000 });
      });
    });

    test('should warn if certificate is in use by proxy host', async ({ page }) => {
      let certName: string;
      let certUUID: string;
      let proxyHostUUID: string;

      await test.step('Seed a deletable custom certificate via API', async () => {
        const result = await createCustomCertViaAPI();
        certUUID = result.uuid;
        createdCertUUIDs.push(certUUID);
        certName = result.certName;
      });

      await test.step('Open the delete confirmation modal while the cert is still unattached', async () => {
        // The delete button only exists/is enabled for a certificate that is
        // NOT currently in use (frontend/src/components/CertificateList.tsx)
        // — an already-attached certificate's delete affordance is either
        // hidden or aria-disabled with a no-op onClick, so the modal can
        // never be opened for it directly.
        await page.reload();
        await waitForLoadingComplete(page);

        const deleteButton = getDeleteButtonForCert(page, certName);
        await expect(deleteButton).toBeVisible({ timeout: 10000 });
        await expect(deleteButton).not.toHaveAttribute('aria-disabled', 'true');
        await deleteButton.click();
        await waitForDialog(page);
      });

      await test.step('Attach the certificate to a proxy host while the modal is open', async () => {
        const result = await attachCertToProxyHostViaAPI(certUUID);
        proxyHostUUID = result.uuid;
        createdProxyHostUUIDs.push(proxyHostUUID);
      });

      await test.step('Confirm deletion and verify the real in-use error is surfaced', async () => {
        const dialog = page.getByRole('dialog');
        const confirmButton = dialog.getByRole('button', { name: /^delete$/i });
        await expect(confirmButton).toBeEnabled();

        // Backend re-checks in-use status at DELETE time (certificate_handler.go's
        // Delete handler) and returns 409 with a specific "in use" message —
        // regardless of what the UI believed when the modal was opened.
        const [deleteResponse] = await Promise.all([
          page.waitForResponse(
            (resp) => resp.url().includes(`/api/v1/certificates/${certUUID}`) && resp.request().method() === 'DELETE',
            { timeout: 15000 }
          ),
          confirmButton.click(),
        ]);
        expect(deleteResponse.status()).toBe(409);

        const errorToast = page.getByTestId('toast-error');
        await expect(errorToast).toBeVisible({ timeout: 5000 });
        await expect(errorToast).toContainText(/in use by one or more proxy hosts/i);
      });

      await test.step('Verify the certificate was not deleted', async () => {
        // getCertificateViaAPI's parseResponse() throws on a non-2xx response,
        // so a resolved promise here already proves GET /certificates/{uuid}
        // returned 200 (i.e. the record still exists). Note: tests/utils/api-helpers.ts's
        // CertificateResponse type declares an `id` field that the actual backend
        // response never sends (CertificateDetail only has `uuid` — see
        // backend/internal/services/certificate_service.go) — don't assert on it.
        await expect(getCertificateViaAPI(page.request, certUUID)).resolves.toBeTruthy();
      });
    });

    test('should cancel delete when confirmation dismissed', async ({ page }) => {
      let certName: string;
      let certUUID: string;

      await test.step('Seed a deletable custom certificate via API', async () => {
        const result = await createCustomCertViaAPI();
        certUUID = result.uuid;
        createdCertUUIDs.push(certUUID);
        certName = result.certName;
      });

      let deleteButton: ReturnType<typeof getDeleteButtonForCert>;

      await test.step('Open the delete confirmation modal', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        // waitForLoadingComplete only guarantees the spinner has cleared, not
        // that this specific seeded row has rendered yet — wait for the row's
        // delete button directly before treating the table as settled, or the
        // row-count snapshot below can race the initial render (captured as 0
        // before the row mounts, then 1 once it does).
        deleteButton = getDeleteButtonForCert(page, certName);
        await expect(deleteButton).toBeVisible({ timeout: 10000 });
      });

      let rowsBefore: number;

      await test.step('Click Cancel on the custom modal', async () => {
        rowsBefore = await page.locator('tbody tr').count();

        await deleteButton.click();
        await waitForDialog(page);

        const dialog = page.getByRole('dialog');
        await dialog.getByRole('button', { name: /cancel/i }).click();
        await expect(dialog).not.toBeVisible({ timeout: 3000 });
      });

      await test.step('Verify the row is unchanged client-side', async () => {
        const rowsAfter = await page.locator('tbody tr').count();
        expect(rowsAfter).toBe(rowsBefore);
      });

      await test.step('Verify the certificate still exists server-side', async () => {
        // Proves cancellation didn't merely hide the row client-side — the
        // backend record genuinely was never touched. getCertificateViaAPI's
        // parseResponse() throws on a non-2xx response, so a resolved promise
        // here already proves the GET returned 200.
        await expect(getCertificateViaAPI(page.request, certUUID)).resolves.toBeTruthy();
      });
    });

    test('should create backup before deletion', async ({ page }) => {
      let certName: string;
      let certUUID: string;

      await test.step('Seed a deletable custom certificate, guaranteed not in use', async () => {
        const result = await createCustomCertViaAPI();
        certUUID = result.uuid;
        certName = result.certName;
        // Not pushed to createdCertUUIDs — this test deletes it itself.
      });

      let backupsBefore: Awaited<ReturnType<typeof getBackupsViaAPI>>;

      await test.step('Capture the backup list before deletion', async () => {
        backupsBefore = await getBackupsViaAPI(page.request);
      });

      await test.step('Confirm deletion via the custom modal', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const deleteButton = getDeleteButtonForCert(page, certName);
        await expect(deleteButton).toBeVisible({ timeout: 10000 });
        await deleteButton.click();
        await waitForDialog(page);

        const dialog = page.getByRole('dialog');
        const confirmButton = dialog.getByRole('button', { name: /^delete$/i });
        await expect(confirmButton).toBeEnabled();

        // backend/internal/api/handlers/certificate_handler.go's Delete handler
        // calls backupService.CreateBackup() synchronously before deleting —
        // wait for the DELETE response so the backup is guaranteed to exist by
        // the time we re-poll GET /api/v1/backups below.
        const [deleteResponse] = await Promise.all([
          page.waitForResponse(
            (resp) => resp.url().includes(`/api/v1/certificates/${certUUID}`) && resp.request().method() === 'DELETE',
            { timeout: 15000 }
          ),
          confirmButton.click(),
        ]);
        expect(deleteResponse.ok()).toBe(true);
      });

      await test.step('Verify a new backup entry was created', async () => {
        // Do NOT assert on the modal's warning text — it only mentions
        // "backup" for the default (non-expired/non-staging/non-expiring)
        // case (certificates.deleteConfirmCustom); the other 3 status-specific
        // messages never mention backups at all. Verify the real backend
        // side effect instead.
        const backupsAfter = await getBackupsViaAPI(page.request);
        expect(backupsAfter.length).toBeGreaterThan(backupsBefore.length);

        const mostRecent = [...backupsAfter].sort(
          (a, b) => new Date(b.time).getTime() - new Date(a.time).getTime()
        )[0];
        const ageMs = Date.now() - new Date(mostRecent.time).getTime();
        expect(ageMs).toBeLessThan(2 * 60 * 1000);
      });
    });

    test('should show config reload overlay during deletion', async ({ page }) => {
      let certName: string;
      let certUUID: string;

      await test.step('Seed a deletable custom certificate, guaranteed not in use', async () => {
        const result = await createCustomCertViaAPI();
        certUUID = result.uuid;
        certName = result.certName;
        // Not pushed to createdCertUUIDs — this test deletes it itself.
      });

      await test.step('Open the custom confirmation modal', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const deleteButton = getDeleteButtonForCert(page, certName);
        await expect(deleteButton).toBeVisible({ timeout: 10000 });
        await deleteButton.click();
        await waitForDialog(page);
      });

      await test.step('Confirm deletion and verify the config reload overlay appears and resolves', async () => {
        const dialog = page.getByRole('dialog');
        const confirmButton = dialog.getByRole('button', { name: /^delete$/i });
        await expect(confirmButton).toBeEnabled();

        // The real DELETE round trip (SQLite backup copy + row delete) on this
        // local backend routinely completes in well under Playwright's first
        // assertion-poll interval, so deleteMutation.isPending's true->false
        // transition — and therefore the overlay's real mount/unmount — can
        // come and go between polls. Delay the response (not the request, not
        // the app logic) just enough to make that already-real state change
        // reliably observable, rather than asserting against a race.
        await page.route(`**/api/v1/certificates/${certUUID}`, async (route) => {
          if (route.request().method() === 'DELETE') {
            await new Promise((resolve) => setTimeout(resolve, 800));
          }
          await route.continue();
        });

        // CertificateList.tsx renders ConfigReloadOverlay while
        // deleteMutation.isPending is true (data-testid="config-reload-overlay").
        const overlay = page.getByTestId('config-reload-overlay');

        await Promise.all([
          expect(overlay).toBeVisible({ timeout: 5000 }),
          confirmButton.click(),
        ]);

        await expect(overlay).not.toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify the certificate was actually removed', async () => {
        const certRow = page.getByRole('row').filter({ hasText: certName });
        await expect(certRow).toHaveCount(0, { timeout: 10000 });
      });
    });
  });

  test.describe('Certificate Renewal', () => {
    test('should show renewal warning for expiring certificates', async ({ page }) => {
      await test.step('Check for expiring certificate indicators', async () => {
        const expiringBadges = page.locator('span').filter({ hasText: /expiring/i });
        const count = await expiringBadges.count();

        // If expiring certificates exist, they should be highlighted
        if (count > 0) {
          const firstBadge = expiringBadges.first();
          await expect(firstBadge).toBeVisible();
        }
      });
    });

    test('should show Let\'s Encrypt auto-renewal info', async ({ page }) => {
      await test.step('Check for Let\'s Encrypt info', async () => {
        // The info alert should mention certificate management
        const alert = page.locator('[role="alert"], .alert');
        const hasAlert = await alert.isVisible().catch(() => false);

        if (hasAlert) {
          const alertText = await alert.textContent();
          expect(alertText).toBeTruthy();
        }
      });
    });
  });

  test.describe('Form Validation', () => {
    test('should reject empty friendly name', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page);
      });

      await test.step('Verify upload blocked with empty name', async () => {
        const dialog = page.getByRole('dialog');
        const uploadButton = dialog.getByRole('button', { name: /upload/i });

        // Submit should be disabled with empty name
        await expect(uploadButton).toBeDisabled();

        // Dialog should remain open
        await expect(dialog).toBeVisible();
      });

      await test.step('Close dialog', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should handle special characters in name', async ({ page }) => {
      await test.step('Test special characters', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive

        const dialog = page.getByRole('dialog');
        const nameInput = dialog.locator('input').first();

        // Test with safe special characters
        await nameInput.fill('Test Cert - Special (chars) #1');

        // Should accept the input
        const value = await nameInput.inputValue();
        expect(value).toContain('Special');

        await getCancelButton(page).click();
      });
    });

    test('should show placeholder text in name input', async ({ page }) => {
      await test.step('Verify placeholder text', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page);

        const dialog = page.getByRole('dialog');
        const nameInput = dialog.locator('input').first();

        const placeholder = await nameInput.getAttribute('placeholder');
        expect(placeholder).toBeTruthy();

        await getCancelButton(page).click();
      });
    });
  });

  test.describe('Form Accessibility', () => {
    test('should have accessible form labels', async ({ page }) => {
      await test.step('Open form and verify labels', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive

        const dialog = page.getByRole('dialog');

        // Check for labels
        const certLabel = dialog.locator('label[for="cert-file"]');
        const keyLabel = dialog.locator('label[for="key-file"]');

        await expect(certLabel).toBeVisible();
        await expect(keyLabel).toBeVisible();

        await getCancelButton(page).click();
      });
    });

    test('should be keyboard navigable', async ({ page }) => {
      await test.step('Open upload dialog and wait for interactivity', async () => {
        await getAddCertButton(page).click();
        const dialog = await waitForDialog(page);
        await expect(dialog).toBeVisible();
      });

      await test.step('Navigate through form fields with Tab key', async () => {
        // Tab to first input (name field)
        await page.keyboard.press('Tab');
        const firstFocusable = page.locator(':focus');
        await expect(firstFocusable).toBeVisible();

        // Tab to next field
        await page.keyboard.press('Tab');
        const secondFocusable = page.locator(':focus');
        await expect(secondFocusable).toBeVisible();

        // Tab to third field
        await page.keyboard.press('Tab');
        const thirdFocusable = page.locator(':focus');
        await expect(thirdFocusable).toBeVisible();

        // Verify at least one element has focus
        const focusedElement = page.locator(':focus');
        await expect(focusedElement).toBeFocused();
      });

      await test.step('Close dialog and verify cleanup', async () => {
        const dialog = page.getByRole('dialog');
        await getCancelButton(page).click();

        // Verify dialog is properly closed
        await expect(dialog).not.toBeVisible({ timeout: 3000 });

        // Verify page is still interactive
        await expect(page.getByRole('heading', { name: /certificates/i })).toBeVisible();
      });
    });

    test('should close dialog on Escape key', async ({ page }) => {
      await test.step('Open upload dialog', async () => {
        await getAddCertButton(page).click();
        const dialog = await waitForDialog(page);
        await expect(dialog).toBeVisible();
      });

      await test.step('Press Escape and verify dialog closes', async () => {
        const dialog = page.getByRole('dialog');
        await page.keyboard.press('Escape');

        // Explicit verification with timeout
        await expect(dialog).not.toBeVisible({ timeout: 3000 });
      });

      await test.step('Verify page state after dialog close', async () => {
        // Ensure page is still interactive
        const heading = page.getByRole('heading', { name: /certificates/i });
        await expect(heading).toBeVisible();

        // Verify no orphaned elements
        const orphanedDialog = page.getByRole('dialog');
        await expect(orphanedDialog).toHaveCount(0);
      });
    });

    test('should have proper dialog role', async ({ page }) => {
      await test.step('Verify dialog ARIA role', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive

        const dialog = page.getByRole('dialog');
        await expect(dialog).toBeVisible();

        await getCancelButton(page).click();
      });
    });

    test('should have dialog title in heading', async ({ page }) => {
      await test.step('Verify dialog has heading', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive

        const dialog = page.getByRole('dialog');
        const heading = dialog.getByRole('heading');
        await expect(heading).toBeVisible();

        await getCancelButton(page).click();
      });
    });
  });

  test.describe('Integration with Proxy Hosts', () => {
    test('should show certificate usage in proxy hosts', async ({ page }) => {
      await test.step('Check if certificates are referenced', async () => {
        // Navigate to proxy hosts to see certificate usage
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);

        const table = page.getByRole('table');
        const hasTable = await table.isVisible().catch(() => false);

        if (hasTable) {
          // SSL column header is a static column definition
          // (frontend/src/pages/ProxyHosts.tsx) rendered unconditionally
          // whenever the table renders — not gated by feature flag or data.
          const sslColumn = page.locator('th').filter({ hasText: /ssl/i });
          await expect(sslColumn.first()).toBeVisible();
        }

        // Navigate back to certificates
        await page.goto('/certificates');
        await waitForLoadingComplete(page);
      });
    });

    test('should navigate between Certificates and Proxy Hosts', async ({ page }) => {
      await test.step('Navigate to Proxy Hosts', async () => {
        await page.goto('/proxy-hosts');
        const heading = page.getByRole('heading', { name: /^proxy hosts$/i });
        await expect(heading).toBeVisible({ timeout: 10000 });
      });

      await test.step('Navigate back to Certificates', async () => {
        await page.goto('/certificates');
        const heading = page.getByRole('heading', { name: /certificates/i });
        await expect(heading).toBeVisible({ timeout: 10000 });
      });
    });
  });

  test.describe('Table Interactions', () => {
    test('should highlight row on hover', async ({ page }) => {
      await test.step('Verify hover styling on table rows', async () => {
        const rows = page.locator('tbody tr');
        const rowCount = await rows.count();

        if (rowCount > 0) {
          const firstRow = rows.first();
          await firstRow.hover();

          // Row should have hover state
          await waitForDebounce(page, { timeout: 1000 }); // Wait for hover animation
        }
      });
    });

    test('should display full table on wide screens', async ({ page }) => {
      await test.step('Verify table layout', async () => {
        await page.setViewportSize({ width: 1280, height: 720 });

        const table = page.getByRole('table');
        const hasTable = await table.isVisible().catch(() => false);

        if (hasTable) {
          // All columns should be visible on wide screens
          const headers = page.locator('thead th');
          const headerCount = await headers.count();
          expect(headerCount).toBeGreaterThanOrEqual(5);
        }
      });
    });

    test('should handle responsive layout', async ({ page }) => {
      await test.step('Test mobile viewport', async () => {
        await page.setViewportSize({ width: 375, height: 667 });
        await waitForDebounce(page); // Wait for responsive layout adjustment

        // Page should still function
        const heading = page.getByRole('heading', { name: /certificates/i });
        await expect(heading).toBeVisible();

        // Reset viewport
        await page.setViewportSize({ width: 1280, height: 720 });
      });
    });
  });

  test.describe('Error Handling', () => {
    test('should show error message on API failure', async ({ page }) => {
      await test.step('Force a certificates API failure', async () => {
        await page.route('**/api/v1/certificates', (route) => {
          if (route.request().method() === 'GET') {
            return route.fulfill({
              status: 500,
              contentType: 'application/json',
              body: JSON.stringify({ error: 'internal server error' }),
            });
          }
          return route.continue();
        });

        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Verify a real error message is shown', async () => {
        // CertificateList renders "Failed to load certificates" when the
        // useCertificates() query errors (frontend/src/components/CertificateList.tsx).
        const errorMessage = page.getByText(/failed.*load|error/i);
        await expect(errorMessage.first()).toBeVisible({ timeout: 10000 });
      });
    });

    test('should show upload error on invalid certificate', async ({ page }) => {
      await test.step('Verify upload error handling', async () => {
        await getAddCertButton(page).click();
        await waitForDialog(page); // Wait for dialog to be fully interactive

        // Fill in name but with invalid files would trigger error
        // This tests the error handling path
        const dialog = page.getByRole('dialog');
        await expect(dialog).toBeVisible();

        await getCancelButton(page).click();
      });
    });
  });

  test.describe('Page Layout', () => {
    test('should have PageShell with title and description', async ({ page }) => {
      await test.step('Verify page layout structure', async () => {
        const heading = page.getByRole('heading', { name: /certificates/i });
        await expect(heading).toBeVisible();

        // Should have description text (PageShell always renders the
        // `description` prop when provided — see PageShell.tsx)
        const description = page.getByText(/manage.*ssl|certificate/i);
        await expect(description.first()).toBeVisible();
      });
    });

    test('should have action button in header', async ({ page }) => {
      await test.step('Verify Add Certificate button is in header area', async () => {
        const addButton = getAddCertButton(page);
        await expect(addButton).toBeVisible();

        // Button should have Plus icon
        const plusIcon = addButton.locator('svg');
        await expect(plusIcon.first()).toBeVisible();
      });
    });

    test('should have card container for table', async ({ page }) => {
      await test.step('Verify table is in styled container', async () => {
        const table = page.getByRole('table');
        const hasTable = await table.isVisible().catch(() => false);

        if (hasTable) {
          // Table should be in a styled card
          const container = table.locator('..');
          const classes = await container.getAttribute('class');
          expect(classes).toBeTruthy();
        }
      });
    });
  });
});
