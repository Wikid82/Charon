/**
 * Certificate Deletion E2E Tests
 *
 * Tests the certificate deletion UX enhancement:
 * - Delete button visibility based on cert type, status, and in-use state
 * - Accessible confirmation dialog (replaces native confirm())
 * - Cancel/confirm flows
 * - Disabled button with tooltip for in-use certs
 * - No delete button for valid production LE certs
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
 * Create a custom certificate directly via the API, bypassing TestDataManager's
 * narrow CertificateData type which omits the required `name` field.
 * Returns the cert's UUID and name for later lookup/cleanup.
 */
async function createCustomCertViaAPI(baseURL: string): Promise<{ uuid: string; certName: string }> {
  const id = generateUniqueId();
  const certName = `test-cert-${id}`;

  const ctx = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
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

    // NOTE: the certificates LIST endpoint (CertificateInfo) only exposes a
    // "uuid" field, never a numeric "id" (the model's numeric ID has json:"-").
    // Both DELETE /api/v1/certificates/:uuid and the proxy host handler's
    // certificate_id resolution accept a UUID directly, so use it everywhere
    // instead of round-tripping through a numeric ID that the API never
    // returns (a previous version of this helper silently produced
    // `id: undefined` here, which JSON.stringify then dropped from request
    // bodies entirely — see git history for the certificate_id linkage bug
    // this caused).
    return { uuid: certUUID, certName };
  } finally {
    await ctx.dispose();
  }
}

/**
 * Delete a certificate directly via the API for cleanup.
 */
async function deleteCertViaAPI(baseURL: string, certUUID: string): Promise<void> {
  const ctx = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
  });

  try {
    await ctx.delete(`/api/v1/certificates/${certUUID}`);
  } finally {
    await ctx.dispose();
  }
}

/**
 * Create a proxy host linked to a certificate via direct API.
 * Returns the proxy host ID for cleanup.
 */
async function createProxyHostWithCertViaAPI(
  baseURL: string,
  certificateUUID: string
): Promise<{ id: string }> {
  const id = generateUniqueId();
  const domain = `proxy-${id}.test.local`;

  const ctx = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
  });

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
    return { id: result.id };
  } finally {
    await ctx.dispose();
  }
}

/**
 * Delete a proxy host via API for cleanup.
 */
async function deleteProxyHostViaAPI(baseURL: string, hostId: string): Promise<void> {
  const ctx = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
  });

  try {
    await ctx.delete(`/api/v1/proxy-hosts/${hostId}`);
  } finally {
    await ctx.dispose();
  }
}

/**
 * Navigate to the certificates page and wait for data to load
 */
async function navigateToCertificates(page: import('@playwright/test').Page): Promise<void> {
  const certsResponse = waitForAPIResponse(page, CERTIFICATES_API);
  await page.goto('/certificates');
  await certsResponse;
  await waitForLoadingComplete(page);
}

test.describe('Certificate Deletion', () => {
  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
  const createdCertUUIDs: string[] = [];

  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.afterAll(async () => {
    // Clean up any certs created during tests that weren't deleted by the tests
    for (const certUUID of createdCertUUIDs) {
      await deleteCertViaAPI(baseURL, certUUID).catch(() => {});
    }
  });

  // ---------------------------------------------------------------------------
  // Scenario 1: Certificates page loads and shows certificate list
  // ---------------------------------------------------------------------------
  test('should display certificates page with heading and list', async ({ page }) => {
    await test.step('Navigate to certificates page', async () => {
      await navigateToCertificates(page);
    });

    await test.step('Verify page heading is visible', async () => {
      const heading = page.getByRole('heading', { name: /certificates/i });
      await expect(heading).toBeVisible();
    });

    await test.step('Verify certificate list or empty state is present', async () => {
      const table = page.getByRole('table');
      const emptyState = page.getByText(/no.*certificates/i);

      await expect(async () => {
        const hasTable = (await table.count()) > 0 && (await table.first().isVisible());
        const hasEmpty = (await emptyState.count()) > 0 && (await emptyState.first().isVisible());
        expect(hasTable || hasEmpty).toBeTruthy();
      }).toPass({ timeout: 10000 });
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 2: Custom cert not in use shows delete button
  // ---------------------------------------------------------------------------
  test('should show delete button for custom cert not in use', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertUUIDs.push(result.uuid);
      certName = result.certName;
    });

    await test.step('Navigate to certificates page', async () => {
      await navigateToCertificates(page);
    });

    await test.step('Verify delete button is visible for the custom cert', async () => {
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });

      const deleteButton = certRow.getByRole('button', { name: /delete/i });
      await expect(deleteButton).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 3: Delete button opens confirmation dialog
  // ---------------------------------------------------------------------------
  test('should open confirmation dialog when delete button is clicked', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertUUIDs.push(result.uuid);
      certName = result.certName;
    });

    await test.step('Navigate to certificates page', async () => {
      await navigateToCertificates(page);
    });

    await test.step('Click the delete button', async () => {
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      const deleteButton = certRow.getByRole('button', { name: /delete/i });
      await deleteButton.click();
    });

    await test.step('Verify confirmation dialog is visible', async () => {
      const dialog = await waitForDialog(page);
      await expect(dialog).toBeVisible();

      await expect(dialog.getByText(/Delete Certificate/)).toBeVisible();
      await expect(dialog.getByRole('button', { name: /Cancel/i })).toBeVisible();
      await expect(dialog.getByRole('button', { name: /^Delete$/i })).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 4: Cancel closes dialog without deleting
  // ---------------------------------------------------------------------------
  test('should close dialog and keep cert when Cancel is clicked', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      createdCertUUIDs.push(result.uuid);
      certName = result.certName;
    });

    await test.step('Navigate to certificates and open delete dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      const deleteButton = certRow.getByRole('button', { name: /delete/i });
      await deleteButton.click();
      await waitForDialog(page);
    });

    await test.step('Click Cancel button', async () => {
      const dialog = page.getByRole('dialog');
      const cancelButton = dialog.getByRole('button', { name: /cancel/i });
      await cancelButton.click();
    });

    await test.step('Verify dialog is closed and cert still exists', async () => {
      await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 5000 });
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible();
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 5: Successful deletion removes cert from list
  // ---------------------------------------------------------------------------
  test('should delete cert and show success toast on confirm', async ({ page }) => {
    let certName: string;

    await test.step('Seed a custom certificate via API', async () => {
      const result = await createCustomCertViaAPI(baseURL);
      // Don't push to createdCertUUIDs — this test will delete it via UI
      certName = result.certName;
    });

    await test.step('Navigate to certificates and open delete dialog', async () => {
      await navigateToCertificates(page);
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });
      const deleteButton = certRow.getByRole('button', { name: /Delete Certificate/i });
      await deleteButton.click();
      await waitForDialog(page);
    });

    await test.step('Confirm deletion and verify cert is removed', async () => {
      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible();

      // Wait for the dialog's confirm Delete button
      const confirmDeleteButton = dialog.getByRole('button', { name: /^Delete$/i });
      await expect(confirmDeleteButton).toBeVisible();
      await expect(confirmDeleteButton).toBeEnabled();

      // Click confirm and wait for the DELETE API response simultaneously
      const [deleteResponse] = await Promise.all([
        page.waitForResponse(
          (resp) => resp.url().includes('/api/v1/certificates/') && resp.request().method() === 'DELETE',
          { timeout: 15000 }
        ),
        confirmDeleteButton.click(),
      ]);

      // Verify the API call succeeded
      expect(deleteResponse.status()).toBeLessThan(400);

      // Verify the cert row is removed from the list
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toHaveCount(0, { timeout: 10000 });
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 6: In-use cert shows disabled delete button with tooltip
  // ---------------------------------------------------------------------------
  test('should show disabled delete button with tooltip for in-use cert', async ({
    page,
  }) => {
    let certName: string;
    let proxyHostId: string;

    await test.step('Seed a custom cert and attach it to a proxy host', async () => {
      const certResult = await createCustomCertViaAPI(baseURL);
      createdCertUUIDs.push(certResult.uuid);
      certName = certResult.certName;

      // Create a proxy host that references this certificate via certificate_id
      const proxyResult = await createProxyHostWithCertViaAPI(baseURL, certResult.uuid);
      proxyHostId = proxyResult.id;
    });

    await test.step('Navigate to certificates page', async () => {
      await navigateToCertificates(page);
    });

    await test.step('Verify delete button is disabled for the in-use cert', async () => {
      const certRow = page.getByRole('row').filter({ hasText: certName });
      await expect(certRow).toBeVisible({ timeout: 10000 });

      const deleteButton = certRow.getByRole('button', { name: /Delete Certificate/i });
      await expect(deleteButton).toBeVisible();
      await expect(deleteButton).toHaveAttribute('aria-disabled', 'true');
    });

    await test.step('Verify tooltip on hover', async () => {
      const certRow = page.getByRole('row').filter({ hasText: certName });
      const deleteButton = certRow.getByRole('button', { name: /Delete Certificate/i });

      await deleteButton.hover();

      const tooltip = page.getByRole('tooltip').or(
        page.getByText(/cannot delete/i)
      );
      await expect(tooltip.first()).toBeVisible({ timeout: 5000 });
    });

    // Cleanup: delete proxy host first (so cert can be cleaned up), then cert
    await test.step('Cleanup proxy host', async () => {
      if (proxyHostId) {
        await deleteProxyHostViaAPI(baseURL, proxyHostId).catch(() => {});
      }
    });
  });

  // ---------------------------------------------------------------------------
  // Scenario 7: Valid production LE cert not in use has no delete button
  // ---------------------------------------------------------------------------
  test('should not show delete button for valid production LE cert', async ({ page }) => {
    await test.step('Navigate to certificates page', async () => {
      await navigateToCertificates(page);
    });

    await test.step('Check for valid production LE certs', async () => {
      const leCertRows = page
        .getByRole('row')
        .filter({ hasText: /let.*encrypt/i });

      const leCount = await leCertRows.count();
      if (leCount === 0) {
        test.skip(true, 'No Let\'s Encrypt certificates present in this environment to verify');
        return;
      }

      for (let i = 0; i < leCount; i++) {
        const row = leCertRows.nth(i);
        const rowText = await row.textContent();

        // Skip expired/expiring LE certs — they ARE expected to have a delete
        // button per frontend/src/utils/certificateUtils.ts's isDeletable(),
        // which treats both status === 'expired' and status === 'expiring'
        // (shown as an "Expiring Soon" badge) as deletable.
        const isExpiredOrExpiring = /expir(ed|ing)/i.test(rowText ?? '');
        if (isExpiredOrExpiring) continue;

        // Skip staging LE certs — the "letsencrypt-staging" provider text also
        // matches the /let.*encrypt/i row filter above (it contains
        // "letsencrypt" as a substring), but staging certs are intentionally
        // deletable per frontend/src/utils/certificateUtils.ts's isDeletable()
        // and are visually flagged with a "STAGING" badge. Only a genuinely
        // valid *production* LE cert should have no delete button.
        const isStaging = /staging/i.test(rowText ?? '');
        if (isStaging) continue;

        // Valid production LE cert should NOT have a delete button
        const deleteButton = row.getByRole('button', { name: /delete/i });
        await expect(deleteButton).toHaveCount(0);
      }
    });
  });
});
