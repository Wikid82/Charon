/**
 * Certificate Bulk Delete E2E Tests
 *
 * Tests the bulk certificate deletion UX:
 * - Checkbox column present for each deletable cert
 * - No checkbox rendered for valid production LE certs
 * - Selection toolbar appears with count and Delete button
 * - Select-all header checkbox selects all seeded certs
 * - Bulk delete dialog shows correct count
 * - Cancel preserves all selected certs
 * - Confirming bulk delete removes all selected certs from the table
 *
 * @see /projects/Charon/docs/plans/current_spec.md §4 Phase 5
 */

import { readFileSync } from 'fs';
import { test, expect, loginUser } from './fixtures/auth-fixtures';
import { request as playwrightRequest } from '@playwright/test';
import {
  waitForLoadingComplete,
  waitForDialog,
  waitForAPIResponse,
  waitForToast,
} from './utils/wait-helpers';
import { generateUniqueId } from './fixtures/test-data';
import { STORAGE_STATE } from './constants';

const CERTIFICATES_API = /\/api\/v1\/certificates/;

/**
 * Real self-signed certificate and key for upload tests.
 * Generated via: openssl req -x509 -newkey rsa:2048 -nodes -days 365 -subj "/CN=test.local/O=TestOrg"
 * The backend parses X.509 data, so placeholder PEM from fixtures won't work.
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
 * Read the auth JWT from the storage state's localStorage entry.
 * The Charon API requires an Authorization: Bearer header; cookies alone are not
 * sufficient in API request contexts (as opposed to browser contexts).
 */
function getAuthToken(baseURL: string): string | undefined {
  try {
    const state = JSON.parse(readFileSync(STORAGE_STATE, 'utf-8'));
    const origin = new URL(baseURL).origin;
    const match = (state.origins ?? []).find(
      (o: { origin: string }) => o.origin === origin
    );
    return match?.localStorage?.find(
      (e: { name: string }) => e.name === 'charon_auth_token'
    )?.value;
  } catch {
    return undefined;
  }
}

/**
 * Create a custom certificate directly via the API, bypassing TestDataManager's
 * narrow CertificateData type which omits the required `name` field.
 * Returns the numeric cert ID (from list endpoint) and name for later lookup/cleanup.
 */
async function createCustomCertViaAPI(baseURL: string): Promise<{ id: number; certName: string }> {
  const id = generateUniqueId();
  const certName = `bulk-cert-${id}`;
  const token = getAuthToken(baseURL);

  const ctx = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
    ...(token ? { extraHTTPHeaders: { Authorization: `Bearer ${token}` } } : {}),
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
      throw new Error(`Failed to create certificate: ${response.status()} ${await response.text()}`);
    }

    const createResult = await response.json();
    const certUUID: string = createResult.uuid;

    // The create response excludes the numeric ID (json:"-" on model).
    // Query the list endpoint and match by UUID to get the numeric ID.
    const listResponse = await ctx.get('/api/v1/certificates');
    if (!listResponse.ok()) {
      throw new Error(`Failed to list certificates: ${listResponse.status()}`);
    }
    const certs: Array<{ id: number; uuid: string }> = await listResponse.json();
    const match = certs.find((c) => c.uuid === certUUID);
    if (!match) {
      throw new Error(`Certificate with UUID ${certUUID} not found in list after creation`);
    }

    return { id: match.id, certName };
  } finally {
    await ctx.dispose();
  }
}

/**
 * Delete a certificate directly via the API for cleanup.
 */
async function deleteCertViaAPI(baseURL: string, certId: number): Promise<void> {
  const token = getAuthToken(baseURL);
  const ctx = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
    ...(token ? { extraHTTPHeaders: { Authorization: `Bearer ${token}` } } : {}),
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
async function navigateToCertificates(page: import('@playwright/test').Page): Promise<void> {
  const certsResponse = waitForAPIResponse(page, CERTIFICATES_API);
  await page.goto('/certificates');
  await certsResponse;
  await waitForLoadingComplete(page);
}

// serial mode: tests share createdCerts[] state via beforeAll/afterAll;
// parallelising across workers would give each worker its own isolated array.
test.describe.serial('Certificate Bulk Delete', () => {
  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
  const createdCerts: Array<{ id: number; certName: string }> = [];

  test.beforeAll(async () => {
    for (let i = 0; i < 3; i++) {
      const cert = await createCustomCertViaAPI(baseURL);
      createdCerts.push(cert);
    }
  });

  test.afterAll(async () => {
    // .catch(() => {}) handles certs already deleted by test 7
    for (const cert of createdCerts) {
      await deleteCertViaAPI(baseURL, cert.id).catch(() => {});
    }
  });

  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await navigateToCertificates(page);
  });

  // ---------------------------------------------------------------------------
  // Scenario 1: Checkbox column present for each deletable (custom) cert
  // ---------------------------------------------------------------------------
  test('Checkbox column present — checkboxes appear for each deletable cert', async ({ page }) => {
    await test.step('Verify each seeded cert row has a selectable checkbox', async () => {
      for (const { certName } of createdCerts) {
        const row = page.getByRole('row').filter({ hasText: certName });
        await expect(row).toBeVisible({ timeout: 10000 });

        const checkbox = row.getByRole('checkbox', {
          name: new RegExp(`Select certificate ${certName}`, 'i'),
        });
        await expect(checkbox).toBeVisible();
        await expect(checkbox).toBeEnabled();
      }
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 2: Valid production LE cert row has no checkbox rendered
  // ---------------------------------------------------------------------------
  test('No checkbox for valid LE — valid production LE cert row has no checkbox', async ({ page }) => {
    await test.step('Find valid production LE cert rows and verify no checkbox', async () => {
      const leRows = page.getByRole('row').filter({ hasText: /let.*encrypt/i });
      const leCount = await leRows.count();

      if (leCount === 0) {
        test.skip(true, 'No Let\'s Encrypt certificates present in this environment');
        return;
      }

      for (let i = 0; i < leCount; i++) {
        const row = leRows.nth(i);
        const rowText = await row.textContent();
        const isExpiredOrStaging = /expired|staging/i.test(rowText ?? '');
        if (isExpiredOrStaging) continue;

        // Valid production LE cert: first cell is aria-hidden with no checkbox
        const firstCell = row.locator('td').first();
        await expect(firstCell).toHaveAttribute('aria-hidden', 'true');
        await expect(row.getByRole('checkbox')).toHaveCount(0);
      }
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 3: Select one → toolbar appears with count and Delete button
  // ---------------------------------------------------------------------------
  test('Select one — checking one cert shows count and Delete button in toolbar', async ({ page }) => {
    const { certName } = createdCerts[0];

    await test.step('Click checkbox for first seeded cert', async () => {
      const row = page.getByRole('row').filter({ hasText: certName });
      await expect(row).toBeVisible({ timeout: 10000 });
      const checkbox = row.getByRole('checkbox', {
        name: new RegExp(`Select certificate ${certName}`, 'i'),
      });
      await checkbox.click();
    });

    await test.step('Verify toolbar appears with count 1 and bulk Delete button', async () => {
      const toolbar = page.getByRole('status').filter({ hasText: /selected/i });
      await expect(toolbar).toBeVisible();
      await expect(toolbar).toContainText('1 certificate(s) selected');

      const bulkDeleteBtn = toolbar.getByRole('button', { name: /Delete \d+ Certificate/i });
      await expect(bulkDeleteBtn).toBeVisible();
      await expect(bulkDeleteBtn).toBeEnabled();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 4: Select-all → header checkbox selects all seeded certs
  // ---------------------------------------------------------------------------
  test('Select-all — header checkbox selects all seeded certs; toolbar shows count', async ({ page }) => {
    await test.step('Click the select-all header checkbox', async () => {
      const selectAllCheckbox = page.getByRole('checkbox', {
        name: /Select all deletable certificates/i,
      });
      await expect(selectAllCheckbox).toBeVisible({ timeout: 10000 });
      await selectAllCheckbox.click();
    });

    await test.step('Verify all seeded cert row checkboxes are checked', async () => {
      for (const { certName } of createdCerts) {
        const row = page.getByRole('row').filter({ hasText: certName });
        await expect(row).toBeVisible({ timeout: 10000 });
        const checkbox = row.getByRole('checkbox');
        await expect(checkbox).toBeChecked();
      }
    });

    await test.step('Verify toolbar is visible with bulk Delete button', async () => {
      const toolbar = page.getByRole('status').filter({ hasText: /selected/i });
      await expect(toolbar).toBeVisible();
      const bulkDeleteBtn = toolbar.getByRole('button', { name: /Delete \d+ Certificate/i });
      await expect(bulkDeleteBtn).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 5: Dialog shows correct count ("Delete 3 Certificate(s)")
  // ---------------------------------------------------------------------------
  test('Dialog shows correct count — bulk dialog shows "Delete 3 Certificate(s)" for 3 selected', async ({ page }) => {
    await test.step('Select each of the 3 seeded certs individually', async () => {
      for (const { certName } of createdCerts) {
        const row = page.getByRole('row').filter({ hasText: certName });
        await expect(row).toBeVisible({ timeout: 10000 });
        const checkbox = row.getByRole('checkbox', {
          name: new RegExp(`Select certificate ${certName}`, 'i'),
        });
        await checkbox.click();
      }
    });

    await test.step('Click the bulk Delete button in the toolbar', async () => {
      const toolbar = page.getByRole('status').filter({ hasText: /selected/i });
      await expect(toolbar).toBeVisible();
      const bulkDeleteBtn = toolbar.getByRole('button', { name: /Delete \d+ Certificate/i });
      await bulkDeleteBtn.click();
    });

    await test.step('Verify dialog title shows "Delete 3 Certificate(s)"', async () => {
      const dialog = await waitForDialog(page);
      await expect(dialog).toBeVisible();
      await expect(dialog).toContainText('Delete 3 Certificate(s)');
    });

    await test.step('Cancel the dialog to preserve certs for subsequent tests', async () => {
      const dialog = page.getByRole('dialog');
      await dialog.getByRole('button', { name: /cancel/i }).click();
      await expect(dialog).not.toBeVisible({ timeout: 5000 });
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 6: Cancel preserves all selected certs in the list
  // ---------------------------------------------------------------------------
  test('Cancel preserves certs — cancelling bulk dialog leaves all certs in list', async ({ page }) => {
    await test.step('Select all 3 seeded certs and open bulk delete dialog', async () => {
      for (const { certName } of createdCerts) {
        const row = page.getByRole('row').filter({ hasText: certName });
        await expect(row).toBeVisible({ timeout: 10000 });
        await row.getByRole('checkbox', {
          name: new RegExp(`Select certificate ${certName}`, 'i'),
        }).click();
      }
      const toolbar = page.getByRole('status').filter({ hasText: /selected/i });
      await toolbar.getByRole('button', { name: /Delete \d+ Certificate/i }).click();
    });

    await test.step('Click Cancel in the bulk delete dialog', async () => {
      const dialog = await waitForDialog(page);
      await expect(dialog).toBeVisible();
      await dialog.getByRole('button', { name: /cancel/i }).click();
    });

    await test.step('Verify dialog is closed and all 3 certs remain in the list', async () => {
      await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 5000 });
      for (const { certName } of createdCerts) {
        const row = page.getByRole('row').filter({ hasText: certName });
        await expect(row).toBeVisible({ timeout: 5000 });
      }
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 7: Confirming bulk delete removes all selected certs from the table
  // ---------------------------------------------------------------------------
  test('Confirm deletes all selected — bulk delete removes all selected certs', async ({ page }) => {
    await test.step('Select all 3 seeded certs and open bulk delete dialog', async () => {
      for (const { certName } of createdCerts) {
        const row = page.getByRole('row').filter({ hasText: certName });
        await expect(row).toBeVisible({ timeout: 10000 });
        await row.getByRole('checkbox', {
          name: new RegExp(`Select certificate ${certName}`, 'i'),
        }).click();
      }
      const toolbar = page.getByRole('status').filter({ hasText: /selected/i });
      await toolbar.getByRole('button', { name: /Delete \d+ Certificate/i }).click();
    });

    await test.step('Confirm bulk deletion', async () => {
      const dialog = await waitForDialog(page);
      await expect(dialog).toBeVisible();
      const confirmBtn = dialog.getByRole('button', { name: /Delete \d+ Certificate/i });
      await expect(confirmBtn).toBeVisible();
      await expect(confirmBtn).toBeEnabled();
      await confirmBtn.click();
    });

    await test.step('Await success toast confirming all deletions settled', async () => {
      // toast.success fires in onSuccess after Promise.allSettled resolves
      await waitForToast(page, /certificate.*deleted/i, { type: 'success' });
    });

    await test.step('Verify all 3 certs are removed from the table', async () => {
      for (const { certName } of createdCerts) {
        await expect(
          page.getByRole('row').filter({ hasText: certName })
        ).toHaveCount(0, { timeout: 10000 });
      }
    });
  });
});
