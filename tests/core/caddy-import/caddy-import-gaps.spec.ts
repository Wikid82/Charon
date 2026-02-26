/**
 * Caddy Import Gap Coverage - E2E Tests
 *
 * This file addresses 5 identified gaps in Caddy Import E2E test coverage:
 * 1. Success Modal Navigation (tests 1.1-1.4)
 * 2. Conflict Details Expansion (tests 2.1-2.3)
 * 3. Overwrite Resolution Flow (test 3.1)
 * 4. Session Resume via Banner (tests 4.1-4.2)
 * 5. Name Editing in Review (test 5.1)
 *
 * Key Patterns:
 * - Uses stored auth state (no login calls needed)
 * - Response waiters registered BEFORE click actions
 * - Real API calls (no mocking) for reliable integration testing
 * - TestDataManager fixture for automatic resource cleanup
 * - Row-scoped selectors (filter by domain, then find within row)
 */

import { test, expect } from '../../fixtures/auth-fixtures';
import type { TestDataManager } from '../../utils/TestDataManager';
import type { Page } from '@playwright/test';

/**
 * Helper: Generate unique domain with namespace isolation
 * This prevents conflicts when tests run in parallel
 */
function generateDomain(testData: TestDataManager, suffix: string): string {
  return `${testData.getNamespace()}-${suffix}.example.com`;
}

/**
 * Helper: Complete the full import flow from paste to success modal
 * Reusable across multiple tests to reduce duplication
 */
async function completeImportFlow(
  page: Page,
  caddyfile: string
): Promise<void> {
  await test.step('Navigate to import page', async () => {
    await page.goto('/tasks/import/caddyfile');
  });

  await test.step('Paste Caddyfile content', async () => {
    await page.locator('textarea').fill(caddyfile);
  });

  await test.step('Parse and wait for review table', async () => {
    const uploadPromise = page.waitForResponse(r =>
      r.url().includes('/api/v1/import/upload') && r.status() === 200
    );
    await page.getByRole('button', { name: /parse|review/i }).click();
    await uploadPromise;

    await expect(page.getByTestId('import-review-table')).toBeVisible();
  });

  await test.step('Commit import and wait for success modal', async () => {
    const commitPromise = page.waitForResponse(r =>
      r.url().includes('/api/v1/import/commit') && r.status() === 200
    );
    await page.getByRole('button', { name: /commit/i }).click();
    await commitPromise;

    await expect(page.getByTestId('import-success-modal')).toBeVisible();
  });
}

test.describe('Caddy Import Gap Coverage @caddy-import-gaps', () => {
  // =========================================================================
  // Gap 1: Success Modal Navigation
  // =========================================================================
  test.describe('Success Modal Navigation', () => {
    test('1.1: should display success modal after successful import commit', async ({ page, testData }) => {
      const domain = generateDomain(testData, 'success-modal-test');
      const caddyfile = `${domain} { reverse_proxy localhost:3000 }`;

      await completeImportFlow(page, caddyfile);

      // Verify success modal is visible
      await expect(page.getByTestId('import-success-modal')).toBeVisible();

      // Verify modal contains expected text
      const modal = page.getByTestId('import-success-modal');
      await expect(modal).toContainText(/import.*completed/i);

      // Verify count is shown (should be 1 host created)
      await expect(modal).toContainText(/1.*created/i);
    });

    test('1.2: should navigate to /proxy-hosts when clicking View Proxy Hosts button', async ({ page, testData }) => {
      const domain = generateDomain(testData, 'view-hosts-nav');
      const caddyfile = `${domain} { reverse_proxy localhost:3000 }`;

      await completeImportFlow(page, caddyfile);

      await test.step('Click View Proxy Hosts button', async () => {
        const modal = page.getByTestId('import-success-modal');
        await modal.getByRole('button', { name: /view.*proxy.*hosts/i }).click();
      });

      await test.step('Verify navigation to proxy hosts page', async () => {
        await expect(page).toHaveURL(/\/proxy-hosts/);
        await expect(page.getByTestId('import-success-modal')).not.toBeVisible();
      });
    });

    test('1.3: should navigate to /dashboard when clicking Go to Dashboard button', async ({ page, testData }) => {
      const domain = generateDomain(testData, 'dashboard-nav');
      const caddyfile = `${domain} { reverse_proxy localhost:3000 }`;

      await completeImportFlow(page, caddyfile);

      await test.step('Click Go to Dashboard button', async () => {
        const modal = page.getByTestId('import-success-modal');
        await modal.getByRole('button', { name: /dashboard/i }).click();
      });

      await test.step('Verify navigation to dashboard', async () => {
        // Dashboard can be at / or /dashboard - check the pathname portion
        await expect(page).toHaveURL(/\/(dashboard)?$/);
        await expect(page.getByTestId('import-success-modal')).not.toBeVisible();
      });
    });

    test('1.4: should close modal and stay on import page when clicking Close', async ({ page, testData }) => {
      const domain = generateDomain(testData, 'close-modal');
      const caddyfile = `${domain} { reverse_proxy localhost:3000 }`;

      await completeImportFlow(page, caddyfile);

      await test.step('Click Close button', async () => {
        const modal = page.getByTestId('import-success-modal');
        await modal.getByRole('button', { name: /close/i }).click();
      });

      await test.step('Verify modal is closed and still on import page', async () => {
        await expect(page.getByTestId('import-success-modal')).not.toBeVisible();
        await expect(page).toHaveURL(/\/tasks\/import\/caddyfile/);
        // Verify textarea is still visible (back to import form)
        await expect(page.locator('textarea')).toBeVisible();
      });
    });
  });

  // =========================================================================
  // Gap 2: Conflict Details Expansion
  // =========================================================================
  test.describe('Conflict Details Expansion', () => {
    test('2.1: should show conflict indicator and expand button for conflicting domain', async ({ page, testData }) => {
      // Create existing host via API to generate conflict
      const result = await testData.createProxyHost({
        domain: 'conflict-test.example.com',
        forwardHost: 'localhost',
        forwardPort: 8080,
        name: 'Existing Conflict Host',
      });
      const namespacedDomain = result.domain;

      await test.step('Navigate to import page and paste conflicting Caddyfile', async () => {
        await page.goto('/tasks/import/caddyfile');
        const caddyfile = `${namespacedDomain} { reverse_proxy localhost:9000 }`;
        await page.locator('textarea').fill(caddyfile);
      });

      await test.step('Parse and wait for review table', async () => {
        const uploadPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/upload') && r.status() === 200
        );
        await page.getByRole('button', { name: /parse|review/i }).click();
        await uploadPromise;

        await expect(page.getByTestId('import-review-table')).toBeVisible();
      });

      await test.step('Verify conflict indicator and expand button in row', async () => {
        // Use row-scoped selector: filter by domain, then find elements within
        const domainRow = page.locator('tr').filter({ hasText: namespacedDomain });

        // Verify conflict indicator (yellow badge with "Conflict" text)
        const conflictBadge = domainRow.locator('.text-yellow-400, .text-yellow-600, .bg-yellow-500').filter({ hasText: /conflict/i });
        await expect(conflictBadge.first()).toBeVisible();

        // Verify expand button is present (▶ or similar)
        const expandButton = domainRow.getByRole('button', { name: /expand|details/i }).or(
          domainRow.locator('button[aria-label*="expand"], button:has-text("▶")')
        );
        await expect(expandButton.first()).toBeVisible();
      });
    });

    test('2.2: should display side-by-side configuration comparison when expanding conflict row', async ({ page, testData }) => {
      // Create existing host with specific config
      const result = await testData.createProxyHost({
        domain: 'expand-test.example.com',
        forwardHost: 'old-server',
        forwardPort: 8080,
        name: 'Expand Test Host',
      });
      const namespacedDomain = result.domain;

      await test.step('Navigate to import page and parse conflicting Caddyfile', async () => {
        await page.goto('/tasks/import/caddyfile');
        const caddyfile = `${namespacedDomain} { reverse_proxy new-server:9000 }`;
        await page.locator('textarea').fill(caddyfile);

        const uploadPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/upload') && r.status() === 200
        );
        await page.getByRole('button', { name: /parse|review/i }).click();
        await uploadPromise;

        await expect(page.getByTestId('import-review-table')).toBeVisible();
      });

      await test.step('Click expand button to show conflict details', async () => {
        const domainRow = page.locator('tr').filter({ hasText: namespacedDomain });
        const expandButton = domainRow.getByRole('button', { name: /expand|details/i }).or(
          domainRow.locator('button[aria-label*="expand"], button:has-text("▶")')
        );
        await expandButton.first().click();
      });

      await test.step('Verify side-by-side comparison is displayed', async () => {
        // Look for "Current Configuration" and "Imported Configuration" section headings
        // Use getByRole('heading') to be more specific and avoid matching recommendation text
        await expect(page.getByRole('heading', { name: /current.*configuration/i })).toBeVisible();
        await expect(page.getByRole('heading', { name: /imported.*configuration/i })).toBeVisible();

        // Port is displayed within Target string like "http://old-server:8080"
        // Find Target label and verify ports in the description definition (dd) elements
        const targetLabels = page.locator('dt').filter({ hasText: /target/i });
        await expect(targetLabels).toHaveCount(2);

        // Verify old config shows port 8080 and new shows port 9000
        await expect(page.getByText(/old-server:8080/)).toBeVisible();
        await expect(page.getByText(/new-server:9000/)).toBeVisible();
      });
    });

    test('2.3: should show recommendation text in expanded conflict details', async ({ page, testData }) => {
      // Create existing host
      const result = await testData.createProxyHost({
        domain: 'recommendation-test.example.com',
        forwardHost: 'server1',
        forwardPort: 3000,
        name: 'Recommendation Test Host',
      });
      const namespacedDomain = result.domain;

      await test.step('Navigate to import page and parse conflicting Caddyfile', async () => {
        await page.goto('/tasks/import/caddyfile');
        const caddyfile = `${namespacedDomain} { reverse_proxy server2:4000 }`;
        await page.locator('textarea').fill(caddyfile);

        const uploadPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/upload') && r.status() === 200
        );
        await page.getByRole('button', { name: /parse|review/i }).click();
        await uploadPromise;

        await expect(page.getByTestId('import-review-table')).toBeVisible();
      });

      await test.step('Expand conflict details', async () => {
        const domainRow = page.locator('tr').filter({ hasText: namespacedDomain });
        const expandButton = domainRow.getByRole('button', { name: /expand|details/i }).or(
          domainRow.locator('button[aria-label*="expand"], button:has-text("▶")')
        );
        await expandButton.first().click();
      });

      await test.step('Verify recommendation text is displayed', async () => {
        // Look for recommendation box (typically has blue left border or contains "Recommendation:")
        const recommendationText = page.locator('.border-l-4.border-blue-500, .border-blue-500').or(
          page.getByText(/💡.*recommendation/i)
        );
        await expect(recommendationText.first()).toBeVisible();
      });
    });
  });

  // =========================================================================
  // Gap 3: Overwrite Resolution Flow
  // =========================================================================
  test.describe('Overwrite Resolution Flow', () => {
    test('3.1: should update existing host when selecting Replace with Imported resolution', async ({ page, request, testData }) => {
      // Create existing host with initial config
      const result = await testData.createProxyHost({
        domain: 'overwrite-test.example.com',
        forwardHost: 'old-server',
        forwardPort: 3000,
        name: 'Overwrite Test Host',
      });
      const namespacedDomain = result.domain;
      const hostId = result.id;

      await test.step('Navigate to import page and parse conflicting Caddyfile', async () => {
        await page.goto('/tasks/import/caddyfile');
        // Import with different config (new-server:9000)
        const caddyfile = `${namespacedDomain} { reverse_proxy new-server:9000 }`;
        await page.locator('textarea').fill(caddyfile);

        const uploadPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/upload') && r.status() === 200
        );
        await page.getByRole('button', { name: /parse|review/i }).click();
        await uploadPromise;

        await expect(page.getByTestId('import-review-table')).toBeVisible();
      });

      await test.step('Select "Replace with Imported" resolution', async () => {
        // Find the row for this domain
        const domainRow = page.locator('tr').filter({ hasText: namespacedDomain });

        // Find the resolution dropdown within this row
        const resolutionDropdown = domainRow.locator('select');
        await expect(resolutionDropdown).toBeVisible();

        // Select the overwrite/replace option
        await resolutionDropdown.selectOption({ label: 'Replace with Imported' });
      });

      await test.step('Commit import', async () => {
        const commitPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/commit') && r.status() === 200
        );
        await page.getByRole('button', { name: /commit/i }).click();
        await commitPromise;

        await expect(page.getByTestId('import-success-modal')).toBeVisible();
      });

      await test.step('Verify existing host was updated (not duplicated)', async () => {
        // Fetch the host via API
        const response = await request.get(`/api/v1/proxy-hosts/${hostId}`);
        expect(response.ok()).toBeTruthy();

        const host = await response.json();
        // Verify forward_host was updated to new-server
        expect(host.forward_host).toBe('new-server');
        // Verify forward_port was updated to 9000
        expect(host.forward_port).toBe(9000);

        // Verify no duplicate was created - fetch all hosts and check count
        const allHostsResponse = await request.get('/api/v1/proxy-hosts');
        expect(allHostsResponse.ok()).toBeTruthy();
        const allHosts = await allHostsResponse.json();

        // Count hosts with this domain
        const matchingHosts = allHosts.filter((h: { domain_names: string }) =>
          h.domain_names === namespacedDomain
        );
        expect(matchingHosts.length).toBe(1);
      });
    });
  });

  // =========================================================================
  // Gap 4: Session Resume via Banner
  // =========================================================================
  test.describe('Session Resume via Banner', () => {
    test('4.1: should show pending session banner when returning to import page', async ({ page, testData }) => {
      // SKIP: Browser-uploaded import sessions are transient (file-based only) and not persisted
      // to the database. The import-banner only appears for database-backed sessions or
      // Docker-mounted Caddyfiles. This tests an unimplemented feature for browser uploads.
      const domain = generateDomain(testData, 'session-resume-test');
      const caddyfile = `${domain} { reverse_proxy localhost:4000 }`;

      await test.step('Create import session by parsing content', async () => {
        await page.goto('/tasks/import/caddyfile');
        await page.locator('textarea').fill(caddyfile);

        const uploadPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/upload') && r.status() === 200
        );
        await page.getByRole('button', { name: /parse|review/i }).click();
        await uploadPromise;

        // Session now exists
        await expect(page.getByTestId('import-review-table')).toBeVisible();
      });

      await test.step('Navigate away from import page', async () => {
        await page.goto('/proxy-hosts');
        await expect(page).toHaveURL(/\/proxy-hosts/);
      });

      await test.step('Navigate back to import page', async () => {
        // Wait for status API to be called after navigation
        const statusPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/status') && r.status() === 200
        );
        await page.goto('/tasks/import/caddyfile');
        await statusPromise;
      });

      await test.step('Verify pending session banner is displayed', async () => {
        // Banner should appear after status query confirms pending session
        const banner = page.getByTestId('import-banner');
        await expect(banner).toBeVisible({ timeout: 10000 });
        await expect(banner).toContainText(/pending.*import.*session/i);

        // Verify "Review Changes" button is visible
        await expect(banner.getByRole('button', { name: /review.*changes/i })).toBeVisible();

        // Review table should NOT be visible initially (until clicking Review Changes)
        await expect(page.getByTestId('import-review-table')).not.toBeVisible();
      });
    });

    test('4.2: should restore review table with previous content when clicking Review Changes', async ({ page, testData }) => {
      // SKIP: Browser-uploaded import sessions are transient (file-based only) and not persisted
      // to the database. Session resume only works for Docker-mounted Caddyfiles.
      // See test 4.1 skip reason for details.
      const domain = generateDomain(testData, 'review-changes-test');
      const caddyfile = `${domain} { reverse_proxy localhost:5000 }`;

      await test.step('Create import session', async () => {
        await page.goto('/tasks/import/caddyfile');
        await page.locator('textarea').fill(caddyfile);

        const uploadPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/upload') && r.status() === 200
        );
        await page.getByRole('button', { name: /parse|review/i }).click();
        await uploadPromise;

        await expect(page.getByTestId('import-review-table')).toBeVisible();
      });

      await test.step('Navigate away and back', async () => {
        await page.goto('/proxy-hosts');
        // Wait for status API to be called after navigation
        const statusPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/status') && r.status() === 200
        );
        await page.goto('/tasks/import/caddyfile');
        await statusPromise;
        await expect(page.getByTestId('import-banner')).toBeVisible({ timeout: 10000 });
      });

      await test.step('Click Review Changes button', async () => {
        const banner = page.getByTestId('import-banner');
        await banner.getByRole('button', { name: /review.*changes/i }).click();
      });

      await test.step('Verify review table is restored with original content', async () => {
        const reviewTable = page.getByTestId('import-review-table');
        await expect(reviewTable).toBeVisible();

        // Verify the table contains the domain from original upload
        await expect(reviewTable.getByText(domain)).toBeVisible();

        // Banner should no longer be visible (or integrated into review UI)
        // Note: Some implementations keep banner visible but change its content
        // If banner remains, it should show different text
      });
    });
  });

  // =========================================================================
  // Gap 5: Name Editing in Review
  // =========================================================================
  test.describe('Name Editing in Review', () => {
    test('5.1: should create proxy host with custom name from review table input', async ({ page, request, testData }) => {
      const domain = generateDomain(testData, 'custom-name-test');
      const customName = 'My Custom Proxy Name';
      const caddyfile = `${domain} { reverse_proxy localhost:5000 }`;

      await test.step('Navigate to import page and parse Caddyfile', async () => {
        await page.goto('/tasks/import/caddyfile');
        await page.locator('textarea').fill(caddyfile);

        const uploadPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/upload') && r.status() === 200
        );
        await page.getByRole('button', { name: /parse|review/i }).click();
        await uploadPromise;

        await expect(page.getByTestId('import-review-table')).toBeVisible();
      });

      await test.step('Edit the name field in review table', async () => {
        // Find the row for this domain
        const domainRow = page.locator('tr').filter({ hasText: domain });

        // Find name input within this row
        const nameInput = domainRow.locator('input[type="text"]');
        await expect(nameInput).toBeVisible();

        // Clear and fill with custom name
        await nameInput.clear();
        await nameInput.fill(customName);

        // Verify input value was set
        await expect(nameInput).toHaveValue(customName);
      });

      await test.step('Commit import', async () => {
        const commitPromise = page.waitForResponse(r =>
          r.url().includes('/api/v1/import/commit') && r.status() === 200
        );
        await page.getByRole('button', { name: /commit/i }).click();
        await commitPromise;

        await expect(page.getByTestId('import-success-modal')).toBeVisible();
      });

      await test.step('Verify created host has custom name', async () => {
        // Fetch all proxy hosts
        const response = await request.get('/api/v1/proxy-hosts');
        expect(response.ok()).toBeTruthy();

        const hosts = await response.json();
        // Find the host with our domain
        const createdHost = hosts.find((h: { domain_names: string }) =>
          h.domain_names === domain
        );

        expect(createdHost).toBeDefined();
        expect(createdHost.name).toBe(customName);
      });
    });
  });
});
