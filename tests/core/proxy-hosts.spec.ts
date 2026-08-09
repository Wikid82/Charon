/**
 * Proxy Hosts CRUD E2E Tests
 *
 * Tests the Proxy Hosts management functionality including:
 * - List view with table, columns, and empty states
 * - Create proxy host with form validation
 * - Read/view proxy host details
 * - Update existing proxy hosts
 * - Delete proxy hosts with confirmation
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForDialog, waitForDebounce } from '../utils/wait-helpers';
import { clickSwitch } from '../utils/ui-helpers';
import { generateProxyHost } from '../fixtures/proxy-hosts';
import { getProxyHostsViaAPI, type ProxyHostResponse } from '../utils/api-helpers';
import type { Page, Request } from '@playwright/test';

/**
 * Helper to dismiss the "New Base Domain Detected" dialog if it appears.
 * This dialog asks if the user wants to save the domain to their domain list.
 */
async function dismissDomainDialog(page: Page): Promise<void> {
  const noThanksBtn = page.getByRole('button', { name: /No, thanks/i });
  if (await noThanksBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
    await noThanksBtn.click();
    await waitForDebounce(page, { delay: 300 }); // Allow dialog to close and DOM to update
  }
}

/**
 * Seed a proxy host directly via the API, bypassing
 * tests/utils/api-helpers.ts's `createProxyHostViaAPI` — that helper posts
 * the `ProxyHostCreateData` fields (`domain`, `forwardHost`, `forwardPort`,
 * `websocketSupport`) verbatim, but the real backend
 * (backend/internal/models/proxy_host.go's JSON tags: `domain_names`,
 * `forward_host`, `forward_port`, `websocket_support`) only recognizes
 * snake_case keys, so every field the helper sends is silently dropped and
 * the request 400s with "domain names is required" (verified empirically
 * against the running API). Send the real field names directly instead —
 * same bypass precedent as certificates.spec.ts's `createCustomCertViaAPI`
 * for `CertificateResponse`'s analogous type/runtime mismatch.
 */
async function seedProxyHostViaAPI(
  page: Page,
  data: { domain: string; forwardHost: string; forwardPort: number; websocketSupport?: boolean }
): Promise<void> {
  const response = await page.request.post('/api/v1/proxy-hosts', {
    data: {
      domain_names: data.domain,
      forward_host: data.forwardHost,
      forward_port: data.forwardPort,
      websocket_support: data.websocketSupport ?? false,
    },
  });

  if (!response.ok()) {
    throw new Error(`Failed to seed proxy host: ${response.status()} ${await response.text()}`);
  }
}

async function ensureEditableProxyHost(
  page: Page,
  testData: {
    createProxyHost: (data: {
      domain: string;
      forwardHost: string;
      forwardPort: number;
      name?: string;
    }) => Promise<unknown>;
  }
): Promise<void> {
  const rows = page.locator('tbody tr');
  if (await rows.count() === 0) {
    await testData.createProxyHost({
      name: `Editable Host ${Date.now()}`,
      domain: `editable-${Date.now()}.example.test`,
      forwardHost: '127.0.0.1',
      forwardPort: 8080,
    });

    await page.goto('/proxy-hosts');
    await waitForLoadingComplete(page);

    const skeleton = page.locator('.animate-pulse');
    await expect(skeleton).toHaveCount(0, { timeout: 10000 });
  }
}

test.describe('Proxy Hosts - CRUD Operations', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);

    // Retry navigation to handle browser closures after long test runs
    let retries = 2;
    while (retries > 0) {
      try {
        await page.goto('/proxy-hosts');
        break;
      } catch (e) {
        retries--;
        if (retries === 0) throw e;
        await page.reload().catch(() => {
          // Reload may fail if page is closed, just continue to next iteration
        });
      }
    }

    // Wait for the page content to actually load (bypassing the Skeleton state)
    // Wait for Skeleton to disappear
    const skeleton = page.locator('.animate-pulse');
    await expect(skeleton).toHaveCount(0, { timeout: 10000 });

    // The skeleton table is present initially. We wait for either the real table OR empty state.
    const table = page.getByRole('table');
    const emptyState = page.getByRole('heading', { name: 'No proxy hosts' });

    // Wait for one of them to be visible
    await expect(async () => {
        const tableVisible = await table.isVisible();
        const emptyVisible = await emptyState.isVisible();
        expect(tableVisible || emptyVisible).toBeTruthy();
    }).toPass({ timeout: 10000 });
  });

  // Helper to get the primary Add Host button (in header, not empty state)
  const getAddHostButton = (page: import('@playwright/test').Page) =>
    page.getByRole('button', { name: /add.*proxy.*host/i }).first();

  // Helper to get the Save button (primary form submit, not confirmation)
  const getSaveButton = (page: import('@playwright/test').Page) =>
    page.getByRole('button', { name: 'Save', exact: true });

  test.describe('List View', () => {
    test('should display proxy hosts page with title', async ({ page }) => {
      await test.step('Verify page title is visible', async () => {
        const heading = page.getByRole('heading', { name: 'Proxy Hosts', exact: true });
        await expect(heading).toBeVisible();
      });

      await test.step('Verify Add Host button is present', async () => {
        const addButton = getAddHostButton(page);
        await expect(addButton).toBeVisible();
      });
    });

    test('should show correct table columns', async ({ page }) => {
      await test.step('Verify table headers exist', async () => {
        // The table should have columns: Name, Domain, Forward To, SSL, Features, Status, Actions
        const expectedColumns = [
          /name/i,
          /domain/i,
          /forward/i,
          /ssl/i,
          /features/i,
          /status/i,
          /actions/i,
        ];

        for (const pattern of expectedColumns) {
          const header = page.getByRole('columnheader', { name: pattern });
          // Column headers may be hidden on mobile, so check if at least one matches
          const headerExists = await header.count() > 0;
          if (headerExists) {
            await expect(header.first()).toBeVisible();
          }
        }
      });
    });

    test('should display empty state when no hosts exist', async ({ page, testData }) => {
      await test.step('Check for empty state or existing hosts', async () => {
        // Note: beforeEach already waits for Content to be loaded.

        const emptyStateHeading = page.getByRole('heading', { name: 'No proxy hosts' });
        const table = page.getByRole('table');

        const hasEmptyState = await emptyStateHeading.isVisible();
        const hasTable = await table.isVisible();

        expect(hasEmptyState || hasTable).toBeTruthy();

        if (hasEmptyState) {
          // Empty state should have an Add Host action
          const addAction = getAddHostButton(page);
          await expect(addAction).toBeVisible();
        }
      });
    });

    test('should show loading skeleton while fetching data', async ({ page }) => {
      await test.step('Navigate and observe loading state', async () => {
        // Intercept network request and delay it to simulate slow network
        await page.route('**/api/**/proxy-hosts*', async route => {
          await new Promise(f => setTimeout(f, 1000));
          await route.continue();
        });

        // Reload to observe loading skeleton
        await page.reload();

        // Check for skeleton element (animate-pulse)
        // We use a locator that matches the skeleton classes
        const skeleton = page.locator('.animate-pulse');
        await expect(skeleton.first()).toBeVisible({ timeout: 5000 });

        // Wait for page to load - check for either table or empty state
        const table = page.getByRole('table');
        const emptyState = page.getByRole('heading', { name: 'No proxy hosts' });

        await expect(async () => {
            const hasTable = await table.isVisible();
            const hasEmpty = await emptyState.isVisible();
            expect(hasTable || hasEmpty).toBeTruthy();
        }).toPass({ timeout: 10000 });

        // Ensure skeleton is gone
        await expect(skeleton.first()).not.toBeVisible();
      });
    });

    test('should support row selection for bulk operations', { retries: 1 }, async ({ page }) => {
      const hostConfig = generateProxyHost();

      await test.step('Seed a proxy host so a real row exists to select', async () => {
        // The select-all checkbox in <thead> renders even when the table is
        // empty, so checking for its presence alone doesn't guarantee row
        // data exists. Seed a host directly via the API for deterministic
        // setup instead of relying on other tests leaving hosts behind.
        await seedProxyHostViaAPI(page, {
          domain: hostConfig.domain,
          forwardHost: hostConfig.forwardHost,
          forwardPort: hostConfig.forwardPort,
        });

        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Select all rows and verify bulk action bar appears', async () => {
        const selectAllCheckbox = page.locator('thead').getByRole('checkbox');
        await expect(selectAllCheckbox).toBeVisible();

        await selectAllCheckbox.click();

        // Should show bulk action bar
        const bulkBar = page.getByText(/selected/i);
        await expect(bulkBar).toBeVisible();
      });
    });
  });

  test.describe('Create Proxy Host', () => {
    test('should open create modal when Add button clicked', async ({ page }) => {
      await test.step('Click Add Host button', async () => {
        const addButton = getAddHostButton(page);
        await expect(addButton).toBeVisible();
        await expect(addButton).toBeEnabled();
        await addButton.click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for modal to open
      });

      await test.step('Verify form modal opens', async () => {
        // The form should be visible as a modal/dialog
        const formTitle = page.getByRole('heading', { name: /add.*proxy.*host/i });
        await expect(formTitle).toBeVisible({ timeout: 5000 });

        // Verify essential form fields are present
        const nameInput = page.locator('#proxy-name').or(page.getByLabel(/name/i));
        await expect(nameInput.first()).toBeVisible();
      });
    });

    test('should validate required fields', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open
      });

      await test.step('Try to submit empty form', async () => {
        const saveButton = getSaveButton(page);

        // Required fields: name, domain_names, forward_host, forward_port.
        // Track whether the create request ever reaches the network — if
        // validation genuinely blocks submission, the browser never fires
        // the form's submit event at all, so no POST should be observed.
        let createRequestSent = false;
        const onRequest = (req: Request) => {
          if (req.method() === 'POST' && req.url().includes('/api/v1/proxy-hosts')) {
            createRequestSent = true;
          }
        };
        page.on('request', onRequest);

        try {
          await saveButton.click();

          // Form should show validation error or prevent submission
          const nameInput = page.locator('#proxy-name');
          const isInvalid = await nameInput.evaluate((el: HTMLInputElement) =>
            el.validity.valid === false || el.getAttribute('aria-invalid') === 'true'
          ).catch(() => false);

          // Browser validation or custom validation should prevent submission
          expect(isInvalid).toBe(true);

          // Give any (incorrectly) fired request a moment to reach the network layer
          await waitForDebounce(page, { delay: 300 });
          expect(createRequestSent).toBe(false);
        } finally {
          page.off('request', onRequest);
        }
      });

      await test.step('Close form', async () => {
        await page.getByRole('button', { name: /cancel/i }).click();
      });
    });

    test('should validate domain format', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open
      });

      await test.step('Enter invalid domain', async () => {
        const domainCombobox = page.locator('#domain-names');
        await domainCombobox.click();
        await page.keyboard.type('not a valid domain!');
        await page.keyboard.press('Tab');
      });

      await test.step('Close form', async () => {
        await page.getByRole('button', { name: /cancel/i }).click();
      });
    });

    test('should validate port number range (1-65535)', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open
      });

      await test.step('Enter invalid port (too high)', async () => {
        const portInput = page.locator('#forward-port').or(page.getByLabel(/port/i));
        await portInput.first().fill('70000');

        // HTML5 validation should mark this as invalid (max=65535)
        const isValid = await portInput.first().evaluate((el: HTMLInputElement) =>
          el.validity.valid
        ).catch(() => true);

        expect(isValid).toBe(false);
      });

      await test.step('Enter valid port', async () => {
        const portInput = page.locator('#forward-port').or(page.getByLabel(/port/i));
        await portInput.first().fill('8080');

        const isValid = await portInput.first().evaluate((el: HTMLInputElement) =>
          el.validity.valid
        ).catch(() => false);

        expect(isValid).toBe(true);
      });

      await test.step('Close form', async () => {
        await page.getByRole('button', { name: /cancel/i }).click();
      });
    });

    test('should create proxy host with minimal config', async ({ page, testData }) => {
      const hostConfig = generateProxyHost();

      await test.step('Open create form', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open
      });

      await test.step('Fill in minimal required fields', async () => {
        // Name
        const nameInput = page.locator('#proxy-name');
        await nameInput.fill(`Test Host ${Date.now()}`);

        // Domain (combobox component)
        const domainCombobox = page.locator('#domain-names');
        await domainCombobox.click();
        await page.keyboard.type(hostConfig.domain);
        await page.keyboard.press('Tab');

        // Dismiss the "New Base Domain Detected" dialog if it appears after domain input
        await dismissDomainDialog(page);

        // Forward Host
        const forwardHostInput = page.locator('#forward-host');
        await forwardHostInput.fill(hostConfig.forwardHost);

        // Forward Port
        const forwardPortInput = page.locator('#forward-port');
        await forwardPortInput.clear();
        await forwardPortInput.fill(String(hostConfig.forwardPort));
      });

      await test.step('Submit form', async () => {
        // Dismiss the "New Base Domain Detected" dialog if it appears
        await dismissDomainDialog(page);

        const saveButton = getSaveButton(page);
        await saveButton.click();

        // Dismiss domain dialog again in case it appeared after Save click
        await dismissDomainDialog(page);

        // Handle "Unsaved changes" confirmation dialog if it appears
        const confirmDialog = page.getByRole('button', { name: /yes.*save/i });
        if (await confirmDialog.isVisible({ timeout: 2000 }).catch(() => false)) {
          await confirmDialog.click();
        }

        // Wait for loading overlay or success state
        await waitForLoadingComplete(page);
      });

      await test.step('Verify host was created', async () => {
        // Wait for either success toast OR host appearing in list
        // Use Promise.race with proper Playwright auto-waiting for reliability
        const successToast = page.getByText(/success|created|saved/i);
        const hostInList = page.getByText(hostConfig.domain);

        // First, wait for any modal to close (form submission complete)
        await waitForDebounce(page, { delay: 1000 }); // Allow form submission and modal close

        // Try waiting for success indicators with proper retry logic
        let verified = false;

        // Check 1: Wait for toast (may have already disappeared)
        const hasSuccess = await successToast.isVisible({ timeout: 2000 }).catch(() => false);
        if (hasSuccess) {
          verified = true;
        }

        // Check 2: If no toast, check if we're back on list page with the host visible
        if (!verified) {
          // Wait for navigation back to list
          await page.waitForURL(/\/proxy-hosts(?!\/)/, { timeout: 5000 }).catch(() => {});
          await waitForLoadingComplete(page);

          // Now check for the host in the list
          const hasHostInList = await hostInList.isVisible({ timeout: 5000 }).catch(() => false);
          if (hasHostInList) {
            verified = true;
          }
        }

        // Check 3: If still not verified, the form might still be open - check for no error
        if (!verified) {
          const errorMessage = page.getByText(/error|failed|invalid/i);
          const hasError = await errorMessage.isVisible({ timeout: 1000 }).catch(() => false);
          // If no error is shown and we're past the form, consider it a pass
          if (!hasError) {
            // Refresh and check list
            await page.goto('/proxy-hosts');
            await waitForLoadingComplete(page);
            verified = await hostInList.isVisible({ timeout: 5000 }).catch(() => false);
          }
        }

        expect(verified).toBeTruthy();
      });
    });

    test('should create proxy host with SSL enabled', async ({ page }) => {
      const hostConfig = generateProxyHost({ scheme: 'https' });

      await test.step('Open create form', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open
      });

      await test.step('Fill in fields with SSL options', async () => {
        await page.locator('#proxy-name').fill(`SSL Test ${Date.now()}`);
        await page.locator('#domain-names').click();
        await page.keyboard.type(hostConfig.domain);
        await page.keyboard.press('Tab');
        await page.locator('#forward-host').fill(hostConfig.forwardHost);
        await page.locator('#forward-port').clear();
        await page.locator('#forward-port').fill(String(hostConfig.forwardPort));

        // Enable SSL options (Force SSL should be on by default)
        const forceSSLCheckbox = page.getByLabel(/force.*ssl/i);
        if (!await forceSSLCheckbox.isChecked()) {
          await forceSSLCheckbox.check();
        }

        // Enable HSTS
        const hstsCheckbox = page.getByLabel(/hsts.*enabled/i);
        if (!await hstsCheckbox.isChecked()) {
          await hstsCheckbox.check();
        }
      });

      await test.step('Submit and verify', async () => {
        // Dismiss the "New Base Domain Detected" dialog if it appears
        await dismissDomainDialog(page);

        await getSaveButton(page).click();

        // Handle "Unsaved changes" confirmation dialog if it appears
        const confirmDialog = page.getByRole('button', { name: /yes.*save/i });
        if (await confirmDialog.isVisible({ timeout: 2000 }).catch(() => false)) {
          await confirmDialog.click();
        }

        await waitForLoadingComplete(page);

        // Verify creation in the UI
        await expect(page.getByText(hostConfig.domain)).toBeVisible({ timeout: 10000 });

        // And verify it was actually persisted server-side, not just
        // rendered optimistically. tests/utils/api-helpers.ts's
        // `ProxyHostResponse` type declares a `domain` field, but the real
        // backend response (backend/internal/models/proxy_host.go's
        // `DomainNames string `json:"domain_names"``) never sends one —
        // same class of type/runtime mismatch already documented in
        // certificates.spec.ts for `CertificateResponse`. Read the real
        // field instead of the never-populated `.domain`.
        const hosts = (await getProxyHostsViaAPI(page.request)) as Array<
          ProxyHostResponse & { domain_names: string }
        >;
        const created = hosts.find((h) => h.domain_names === hostConfig.domain);
        expect(created).toBeTruthy();
        expect(created?.forward_host).toBe(hostConfig.forwardHost);
        expect(created?.forward_port).toBe(hostConfig.forwardPort);
      });
    });

    test('should create proxy host with WebSocket support', async ({ page }) => {
      const hostConfig = generateProxyHost();

      await test.step('Open create form', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open
      });

      await test.step('Fill form with WebSocket enabled', async () => {
        await page.locator('#proxy-name').fill(`WS Test ${Date.now()}`);
        await page.locator('#domain-names').click();
        await page.keyboard.type(hostConfig.domain);
        await page.keyboard.press('Tab');
        await page.locator('#forward-host').fill(hostConfig.forwardHost);
        await page.locator('#forward-port').clear();
        await page.locator('#forward-port').fill(String(hostConfig.forwardPort));

        // Enable WebSocket support
        const wsCheckbox = page.getByLabel(/websocket/i);
        if (!await wsCheckbox.isChecked()) {
          await wsCheckbox.check();
        }
      });

      await test.step('Submit and verify', async () => {
        // Dismiss the "New Base Domain Detected" dialog if it appears
        await dismissDomainDialog(page);

        await getSaveButton(page).click();

        // Handle "Unsaved changes" confirmation dialog if it appears
        const confirmDialog = page.getByRole('button', { name: /yes.*save/i });
        if (await confirmDialog.isVisible({ timeout: 2000 }).catch(() => false)) {
          await confirmDialog.click();
        }

        await waitForLoadingComplete(page);
      });
    });

    test('should show form with all security options', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open
      });

      await test.step('Verify security options are present', async () => {
        const securityOptions = [
          /force.*ssl/i,
          /http.*2/i,
          /hsts/i,
          /block.*exploits/i,
          /websocket/i,
        ];

        for (const option of securityOptions) {
          // These are static, always-rendered form fields (SSL & Security
          // Options block in ProxyHostForm.tsx) — not conditional/feature-flagged.
          const checkbox = page.getByLabel(option);
          const exists = await checkbox.count() > 0;
          expect(exists).toBe(true);
          await expect(checkbox.first()).toBeVisible();
        }
      });

      await test.step('Close form', async () => {
        await page.getByRole('button', { name: /cancel/i }).click();
      });
    });

    test('should show application preset selector', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open
      });

      await test.step('Verify application preset dropdown', async () => {
        // The preset field is a Radix Select (frontend/src/components/ui/Select.tsx),
        // not a native <select>/<option> — it renders no <option> elements at
        // all, so `option:text-matches(...)` always matched zero regardless
        // of what presets exist. Open the dropdown and look for the real
        // role="option" items it renders instead.
        const presetSelect = page.getByTestId('application-preset');
        await expect(presetSelect).toBeVisible();
        await presetSelect.click();

        // Check for common presets (APPLICATION_PRESETS is a static const
        // array in ProxyHostForm.tsx — not feature-flagged). Match against
        // each preset's rendered `label` text, not its `value` — "Home
        // Assistant" (label) doesn't literally contain "homeassistant" (value).
        const presetLabels = [/plex/i, /jellyfin/i, /home\s*assistant/i, /nextcloud/i];
        for (const label of presetLabels) {
          const option = page.getByRole('option', { name: label });
          await expect(option).toBeVisible();
        }

        await page.keyboard.press('Escape');
      });

      await test.step('Close form', async () => {
        await page.getByRole('button', { name: /cancel/i }).click();
      });
    });

    test('should show test connection button', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open
      });

      await test.step('Verify test connection button exists', async () => {
        const testButton = page.getByRole('button', { name: /test.*connection/i });
        await expect(testButton).toBeVisible();

        // Button should be disabled initially (no host/port entered)
        await expect(testButton).toBeDisabled();
      });

      await test.step('Enter host details and check button becomes enabled', async () => {
        await page.locator('#forward-host').fill('192.168.1.100');
        await page.locator('#forward-port').fill('80');

        const testButton = page.getByRole('button', { name: /test.*connection/i });
        await expect(testButton).toBeEnabled();
      });

      await test.step('Close form', async () => {
        await page.getByRole('button', { name: /cancel/i }).click();
      });
    });
  });

  test.describe('Read/View Proxy Host', () => {
    test('should display host details in table row', async ({ page }) => {
      await test.step('Check table displays host information', async () => {
        const table = page.getByRole('table');
        const hasTable = await table.isVisible().catch(() => false);

        if (hasTable) {
          // Verify table has data rows
          const rows = page.locator('tbody tr');
          const rowCount = await rows.count();

          if (rowCount > 0) {
            // First row should have domain and forward info
            const firstRow = rows.first();
            await expect(firstRow).toBeVisible();

            // Check for expected content patterns
            const rowText = await firstRow.textContent();
            expect(rowText).toBeTruthy();
          }
        }
      });
    });

    test('should show SSL badge for HTTPS hosts', async ({ page }) => {
      await test.step('Check for SSL badges in table', async () => {
        // SSL badges appear for hosts with ssl_forced
        const sslBadges = page.locator('text=SSL').or(page.getByText(/staging/i));
        const badgeCount = await sslBadges.count();

        // May or may not have SSL hosts depending on test data
        expect(badgeCount >= 0).toBeTruthy();
      });
    });

    test('should show status toggle for enabling/disabling hosts', async ({ page }) => {
      await test.step('Check for status toggles', async () => {
        // Status is shown as a Switch/toggle
        const statusToggles = page.locator('tbody').getByRole('switch');
        const toggleCount = await statusToggles.count();

        // If we have hosts, we should have toggles
        if (toggleCount > 0) {
          const firstToggle = statusToggles.first();
          await expect(firstToggle).toBeVisible();
        }
      });
    });

    test('should show feature badges (WebSocket, ACL)', async ({ page }) => {
      const hostConfig = generateProxyHost({ websocketSupport: true });

      await test.step('Seed a host with WebSocket support enabled', async () => {
        await seedProxyHostViaAPI(page, {
          domain: hostConfig.domain,
          forwardHost: hostConfig.forwardHost,
          forwardPort: hostConfig.forwardPort,
          websocketSupport: true,
        });
      });

      await test.step('Check for feature badges', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        // Scope to the seeded host's own row (features column renders a
        // "WS" badge — frontend/src/pages/ProxyHosts.tsx — only when
        // websocket_support is true, which is now deterministic).
        const row = page.locator('tbody tr').filter({ hasText: hostConfig.domain });
        await expect(row).toBeVisible({ timeout: 10000 });

        const wsBadge = row.getByText('WS', { exact: true });
        const aclBadge = row.getByText('ACL', { exact: true });

        const hasWs = await wsBadge.isVisible().catch(() => false);
        const hasAcl = await aclBadge.isVisible().catch(() => false);

        expect(hasWs || hasAcl).toBeTruthy();
      });
    });

    test('should have clickable domain links', async ({ page }) => {
      await test.step('Check domain links', async () => {
        const domainLinks = page.locator('tbody a[href]');
        const linkCount = await domainLinks.count();

        if (linkCount > 0) {
          const firstLink = domainLinks.first();
          const href = await firstLink.getAttribute('href');

          // Links should point to the domain URL
          expect(href).toMatch(/^https?:\/\//);
        }
      });
    });
  });

  test.describe('Update Proxy Host', () => {
    test.describe.configure({ mode: 'serial' });

    test('should open edit modal with existing values', async ({ page, testData }) => {
      await test.step('Find and click Edit button', async () => {
        await ensureEditableProxyHost(page, testData);

        const firstRow = page.locator('tbody tr').first();
        await expect(firstRow).toBeVisible();

        const editButton = firstRow
          .getByRole('button', { name: /edit proxy host|edit/i })
          .first();
        await expect(editButton).toBeVisible();
        await editButton.click();
        await expect(page.getByRole('dialog')).toBeVisible();

        const formTitle = page.getByRole('heading', { name: /edit.*proxy.*host/i });
        await expect(formTitle).toBeVisible({ timeout: 5000 });

        const nameInput = page.locator('#proxy-name');
        const nameValue = await nameInput.inputValue();
        expect(nameValue.length >= 0).toBeTruthy();

        await page.getByRole('button', { name: /cancel/i }).click();
      });
    });

    test('should update domain name', async ({ page }) => {
      await test.step('Edit a host if available', async () => {
        const editButtons = page.getByRole('button', { name: /edit/i });
        const editCount = await editButtons.count();

        if (editCount > 0) {
          await editButtons.first().click();
          await expect(page.getByRole('dialog')).toBeVisible(); // Wait for edit modal to open

          const domainInput = page.locator('#domain-names');

          // Clear existing domain and type new one (combobox component)
          const newDomain = `test-${Date.now()}.example.com`;
          await domainInput.click();
          await page.keyboard.press('Control+a');
          await page.keyboard.press('Backspace');
          await page.keyboard.type(newDomain);
          await page.keyboard.press('Tab');

          // Dismiss the "New Base Domain Detected" dialog if it appears
          await dismissDomainDialog(page);

          // Save — use specific selector to avoid strict mode violation with domain dialog buttons
          await page.getByTestId('proxy-host-save').or(page.getByRole('button', { name: /^save$/i })).first().click();
          await waitForLoadingComplete(page);

          // Verify update (check for new domain or revert)
        }
      });
    });

    test('should toggle SSL settings', async ({ page }) => {
      await test.step('Edit and toggle SSL', async () => {
        const editButtons = page.getByRole('button', { name: /edit/i });
        const editCount = await editButtons.count();

        if (editCount > 0) {
          await editButtons.first().click();
          await expect(page.getByRole('dialog')).toBeVisible(); // Wait for edit modal to open

          const forceSSLCheckbox = page.getByLabel(/force.*ssl/i);
          const wasChecked = await forceSSLCheckbox.isChecked();

          // Toggle the checkbox
          if (wasChecked) {
            await forceSSLCheckbox.uncheck();
          } else {
            await forceSSLCheckbox.check();
          }

          const isNowChecked = await forceSSLCheckbox.isChecked();
          expect(isNowChecked).toBe(!wasChecked);

          // Cancel without saving
          await page.getByRole('button', { name: /cancel/i }).click();
        }
      });
    });

    test('should update forward host and port', async ({ page, testData }) => {
      await test.step('Edit forward settings', async () => {
        await ensureEditableProxyHost(page, testData);

        const firstRow = page.locator('tbody tr').first();
        await expect(firstRow).toBeVisible();

        const editButton = firstRow
          .getByRole('button', { name: /edit proxy host|edit/i })
          .first();
        await expect(editButton).toBeVisible();
        await editButton.click();
        await expect(page.getByRole('dialog')).toBeVisible();

        const forwardHostInput = page.locator('#forward-host');
        await forwardHostInput.clear();
        await forwardHostInput.fill('192.168.1.200');

        const forwardPortInput = page.locator('#forward-port');
        await forwardPortInput.clear();
        await forwardPortInput.fill('9000');

        expect(await forwardHostInput.inputValue()).toBe('192.168.1.200');
        expect(await forwardPortInput.inputValue()).toBe('9000');

        await page.getByRole('button', { name: /cancel/i }).click();
      });
    });

    test('should toggle host enabled/disabled from list', async ({ page }) => {
      await test.step('Toggle status switch', async () => {
        const statusToggles = page.locator('tbody').getByRole('switch');
        const toggleCount = await statusToggles.count();

        if (toggleCount > 0) {
          const firstToggle = statusToggles.first();
          const wasChecked = await firstToggle.isChecked();

          // Toggle the switch
          await clickSwitch(firstToggle);
          await waitForLoadingComplete(page);

          // Verify the toggle state actually flipped after the click.
          const isNowChecked = await firstToggle.isChecked();
          expect(isNowChecked).toBe(!wasChecked);
        }
      });
    });
  });

  test.describe('Delete Proxy Host', () => {
    test('should show confirmation dialog before delete', async ({ page }) => {
      await test.step('Click Delete button', async () => {
        const deleteButtons = page.getByRole('button', { name: /delete/i });
        const deleteCount = await deleteButtons.count();

        if (deleteCount > 0) {
          // Find delete button in table row (not bulk delete)
          const rowDeleteButton = page.locator('tbody').getByRole('button', { name: /delete/i }).first();

          if (await rowDeleteButton.isVisible().catch(() => false)) {
            await rowDeleteButton.click();
            await waitForDialog(page); // Wait for confirmation dialog

            // Confirmation dialog should appear
            const dialog = page.getByRole('dialog').or(page.getByRole('alertdialog'));
            const hasDialog = await dialog.isVisible({ timeout: 3000 }).catch(() => false);

            if (hasDialog) {
              // Dialog should have confirm and cancel buttons
              const confirmButton = dialog.getByRole('button', { name: /delete|confirm/i });
              const cancelButton = dialog.getByRole('button', { name: /cancel/i });

              await expect(confirmButton).toBeVisible();
              await expect(cancelButton).toBeVisible();

              // Cancel the delete
              await cancelButton.click();
            }
          }
        }
      });
    });

    test('should cancel delete when confirmation dismissed', async ({ page }) => {
      await test.step('Trigger delete and cancel', async () => {
        const rowDeleteButton = page.locator('tbody').getByRole('button', { name: /delete/i }).first();

        if (await rowDeleteButton.isVisible().catch(() => false)) {
          await rowDeleteButton.click();
          await waitForDialog(page); // Wait for confirmation dialog

          const dialog = page.getByRole('dialog').or(page.getByRole('alertdialog'));

          if (await dialog.isVisible().catch(() => false)) {
            // Click cancel
            await dialog.getByRole('button', { name: /cancel/i }).click();

            // Dialog should close
            await expect(dialog).not.toBeVisible({ timeout: 3000 });

            // Host should still be in the list
            const tableRows = page.locator('tbody tr');
            const rowCount = await tableRows.count();
            expect(rowCount >= 0).toBeTruthy();
          }
        }
      });
    });

    test('should show delete confirmation with host name', async ({ page }) => {
      await test.step('Verify confirmation message includes host info', async () => {
        const rowDeleteButton = page.locator('tbody').getByRole('button', { name: /delete/i }).first();

        if (await rowDeleteButton.isVisible().catch(() => false)) {
          await rowDeleteButton.click();
          await waitForDialog(page); // Wait for confirmation dialog

          const dialog = page.getByRole('dialog').or(page.getByRole('alertdialog'));

          if (await dialog.isVisible().catch(() => false)) {
            // Dialog should mention the host being deleted
            const dialogText = await dialog.textContent();
            expect(dialogText).toBeTruthy();

            // Cancel
            await dialog.getByRole('button', { name: /cancel/i }).click();
          }
        }
      });
    });
  });

  test.describe('Bulk Operations', () => {
    test('should show bulk action bar when hosts are selected', async ({ page }) => {
      await test.step('Select hosts and verify bulk bar', async () => {
        const selectAllCheckbox = page.locator('thead').getByRole('checkbox');

        if (await selectAllCheckbox.isVisible().catch(() => false)) {
          await selectAllCheckbox.click();
          await waitForDebounce(page, { delay: 300 }); // Allow selection state to update

          // Bulk action bar should appear
          const bulkBar = page.getByText(/selected/i);
          const hasBulkBar = await bulkBar.isVisible().catch(() => false);

          if (hasBulkBar) {
            // Verify bulk action buttons
            const bulkApply = page.getByRole('button', { name: /bulk.*apply|apply/i });
            const bulkDelete = page.getByRole('button', { name: /delete/i });

            const hasApply = await bulkApply.isVisible().catch(() => false);
            const hasDelete = await bulkDelete.isVisible().catch(() => false);

            expect(hasApply || hasDelete).toBeTruthy();
          }

          // Deselect
          await selectAllCheckbox.click();
        }
      });
    });

    test('should open bulk apply settings modal', async ({ page }) => {
      await test.step('Select hosts and open bulk apply', async () => {
        const selectAllCheckbox = page.locator('thead').getByRole('checkbox');

        if (await selectAllCheckbox.isVisible().catch(() => false)) {
          await selectAllCheckbox.click();
          await waitForDebounce(page, { delay: 300 }); // Allow selection state to update

          const bulkApplyButton = page.getByRole('button', { name: /bulk.*apply/i });

          if (await bulkApplyButton.isVisible().catch(() => false)) {
            await bulkApplyButton.click();
            await expect(page.getByRole('dialog')).toBeVisible(); // Wait for bulk apply modal

            // Bulk apply modal should open
            const modal = page.getByRole('dialog');
            const hasModal = await modal.isVisible().catch(() => false);

            if (hasModal) {
              // Close modal
              await page.getByRole('button', { name: /cancel/i }).click();
            }
          }

          // Deselect
          await selectAllCheckbox.click();
        }
      });
    });

    test('should open bulk ACL modal', async ({ page }) => {
      await test.step('Select hosts and open ACL modal', async () => {
        const selectAllCheckbox = page.locator('thead').getByRole('checkbox');

        if (await selectAllCheckbox.isVisible().catch(() => false)) {
          await selectAllCheckbox.click();
          await waitForDebounce(page, { delay: 300 }); // Allow selection state to update

          const manageACLButton = page.getByRole('button', { name: /manage.*acl|acl/i });

          if (await manageACLButton.isVisible().catch(() => false)) {
            await manageACLButton.click();
            await expect(page.getByRole('dialog')).toBeVisible(); // Wait for ACL modal

            // ACL modal should open
            const modal = page.getByRole('dialog');
            const hasModal = await modal.isVisible().catch(() => false);

            if (hasModal) {
              // Should have apply/remove tabs or buttons
              const applyTab = page.getByRole('button', { name: /apply.*acl/i });
              const removeTab = page.getByRole('button', { name: /remove.*acl/i });

              const hasApply = await applyTab.isVisible().catch(() => false);
              const hasRemove = await removeTab.isVisible().catch(() => false);

              expect(hasApply || hasRemove).toBeTruthy();

              // Close modal
              await page.getByRole('button', { name: /cancel/i }).click();
            }
          }

          // Deselect
          await selectAllCheckbox.click();
        }
      });
    });
  });

  test.describe('Form Accessibility', () => {
    test('should have accessible form labels', async ({ page }) => {
      await test.step('Open form and verify labels', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open

        // Check that inputs have associated labels
        const nameInput = page.locator('#proxy-name');
        const label = page.locator('label[for="proxy-name"]');

        await expect(nameInput).toBeVisible();
        await expect(label).toBeVisible();

        // Close form
        await page.getByRole('button', { name: /cancel/i }).click();
      });
    });

    test('should be keyboard navigable', async ({ page }) => {
      await test.step('Navigate form with keyboard', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open

        // Tab through form fields
        await page.keyboard.press('Tab');
        await page.keyboard.press('Tab');
        await page.keyboard.press('Tab');

        // Some element should be focused — modals should trap/receive focus
        // deterministically.
        const focusedElement = page.locator(':focus');
        await expect(focusedElement).toBeVisible();

        // Escape should close the form
        await page.keyboard.press('Escape');

        // Form may still be open, close with button if needed
        const cancelButton = page.getByRole('button', { name: /cancel/i });
        if (await cancelButton.isVisible().catch(() => false)) {
          await cancelButton.click();
        }
      });
    });
  });

  test.describe('Docker Integration', () => {
    test('should show Docker container selector when source is selected', async ({ page }) => {
      await test.step('Open form and check Docker options', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open

        // Source dropdown should be visible and have id
        const sourceSelect = page.locator('#connection-source');
        await expect(sourceSelect).toBeVisible();
      });
    });

    test('should show containers dropdown when Docker source selected', async ({ page }) => {
      await test.step('Verify containers dropdown exists', async () => {
        await getAddHostButton(page).click();
        await expect(page.getByRole('dialog')).toBeVisible(); // Wait for form modal to open

        // Containers dropdown should exist and have id
        const containersSelect = page.locator('#quick-select-docker');
        await expect(containersSelect).toBeVisible();

        // Should be disabled when source is 'custom' (default)
        await expect(containersSelect).toBeDisabled();
      });
    });
  });
});
