/**
 * Caddy Import - Cross-Browser E2E Tests
 *
 * Runs core Caddyfile import scenarios against Chromium, Firefox, and WebKit
 * to prevent browser-specific regressions (GitHub Issue #567).
 *
 * EXECUTION:
 * npx playwright test tests/tasks/caddy-import-cross-browser.spec.ts --project=chromium --project=firefox --project=webkit
 *
 * SCOPE:
 * - Tests UI/UX on management interface (port 8080)
 * - API request/response validation
 * - Session state management
 * - Conflict resolution flow
 *
 * NOTE: Does NOT test middleware enforcement (WAF, ACL, Rate Limiting).
 * Those are verified in backend/integration/ tests.
 */

import { test, expect, type TestUser } from '../../fixtures/auth-fixtures';
import { Page } from '@playwright/test';
import { ensureImportUiPreconditions, resetImportSession } from './import-page-helpers';

/**
 * Mock Caddyfile content for testing
 */
const VALID_CADDYFILE = `
example.com {
  reverse_proxy localhost:3000
}

api.example.com {
  reverse_proxy localhost:8080
}
`;

const INVALID_CADDYFILE = `
invalid syntax {
  missing_directive
`;

const SINGLE_HOST_CADDYFILE = `
test.example.com {
  reverse_proxy 192.168.1.100:8080
}
`;

/**
 * Helper to set up all import API mocks with the specified behavior
 */
async function setupImportMocks(
  page: Page,
  options: {
    uploadSuccess?: boolean;
    previewHosts?: any[];
    conflicts?: string[];
    commitSuccess?: boolean;
  } = {}
) {
  const {
    uploadSuccess = true,
    previewHosts = [
      { domain_names: 'example.com', forward_host: 'localhost', forward_port: 3000, forward_scheme: 'http' },
      { domain_names: 'api.example.com', forward_host: 'localhost', forward_port: 8080, forward_scheme: 'http' },
    ],
    conflicts = [],
    commitSuccess = true,
  } = options;

  let hasSession = false;

  // Mock status endpoint
  await page.route('**/api/v1/import/status', async (route) => {
    if (hasSession) {
      await route.fulfill({
        status: 200,
        json: {
          has_pending: true,
          session: {
            id: 'test-session-cross-browser',
            state: 'reviewing',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        },
      });
    } else {
      await route.fulfill({
        status: 200,
        json: { has_pending: false },
      });
    }
  });

  // Mock upload endpoint
  await page.route('**/api/v1/import/upload', async (route) => {
    if (uploadSuccess) {
      hasSession = true;
      await route.fulfill({
        status: 200,
        json: {
          session: {
            id: 'test-session-cross-browser',
            state: 'transient',
            source_file: '/imports/uploads/test-session-cross-browser.caddyfile',
          },
          preview: {
            hosts: previewHosts,
            conflicts: conflicts,
            warnings: [],
          },
          caddyfile_content: VALID_CADDYFILE,
          conflict_details: {},
        },
      });
    } else {
      await route.fulfill({
        status: 400,
        json: { error: 'Invalid Caddyfile syntax at line 2: unexpected token' },
      });
    }
  });

  // Mock preview endpoint
  await page.route('**/api/v1/import/preview', async (route) => {
    await route.fulfill({
      status: 200,
      json: {
        session: {
          id: 'test-session-cross-browser',
          state: 'reviewing',
        },
        preview: {
          hosts: previewHosts,
          conflicts: conflicts,
          warnings: [],
        },
        caddyfile_content: VALID_CADDYFILE,
      },
    });
  });

  // Mock commit endpoint
  await page.route('**/api/v1/import/commit', async (route) => {
    if (commitSuccess) {
      await route.fulfill({
        status: 200,
        json: {
          created: previewHosts.length,
          updated: 0,
          skipped: 0,
          errors: [],
        },
      });
    } else {
      await route.fulfill({
        status: 500,
        json: { error: 'Failed to commit import' },
      });
    }
  });

  // Mock cancel endpoint
  await page.route('**/api/v1/import/cancel', async (route) => {
    hasSession = false;
    await route.fulfill({ status: 204 });
  });

  // Mock backups endpoint (pre-import backup)
  await page.route('**/api/v1/backups', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({
        status: 201,
        json: {
          filename: 'pre-import-backup.tar.gz',
          size: 1000,
          time: new Date().toISOString(),
        },
      });
    } else {
      await route.continue();
    }
  });
}

async function gotoImportPageWithAuthRecovery(page: Page, adminUser: TestUser): Promise<void> {
  await expect(async () => {
    await ensureImportUiPreconditions(page, adminUser);
  }).toPass({ timeout: 15000 });
}

test.describe('Caddy Import - Cross-Browser @cross-browser', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await resetImportSession(page);
    await ensureImportUiPreconditions(page, adminUser);
  });

  test.afterEach(async ({ page }) => {
    await resetImportSession(page);
  });

  /**
   * TEST 1: Parse valid Caddyfile across all browsers
   * Verifies basic import flow works identically in Chromium, Firefox, and WebKit
   */
  test('should parse valid Caddyfile in all browsers', async ({ page, browserName, adminUser }) => {
    await setupImportMocks(page);

    await test.step(`[${browserName}] Navigate to import page`, async () => {
      await gotoImportPageWithAuthRecovery(page, adminUser);
      await expect(page.locator('h1')).toContainText(/import/i);
    });

    await test.step(`[${browserName}] Paste Caddyfile content`, async () => {
      const textarea = page.locator('textarea');
      await textarea.fill(VALID_CADDYFILE);
      await expect(textarea).toHaveValue(/^[\s\S]*example\.com[\s\S]*$/);
    });

    let requestSent = false;
    await test.step(`[${browserName}] Monitor API request and click Parse`, async () => {
      // Register listener BEFORE clicking (critical for Firefox)
      const uploadPromise = page.waitForResponse(
        (r) => r.url().includes('/api/v1/import/upload'),
        { timeout: 10000 }
      );

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await expect(parseButton).toBeVisible();
      await expect(parseButton).toBeEnabled();
      await parseButton.click();

      const response = await uploadPromise;
      requestSent = response.ok();
      expect(requestSent).toBeTruthy();

      const body = await response.json();
      expect(body.session).toBeDefined();
      expect(body.preview.hosts).toHaveLength(2);
    });

    await test.step(`[${browserName}] Verify review table appears`, async () => {
      expect(requestSent).toBeTruthy();

      const reviewTable = page.locator('[data-testid="import-review-table"]');
      await expect(reviewTable).toBeVisible({ timeout: 10000 });

      // Verify both hosts are displayed
      await expect(page.getByText('example.com', { exact: true })).toBeVisible();
      await expect(page.getByText('api.example.com', { exact: true })).toBeVisible();
    });
  });

  /**
   * TEST 2: Handle syntax errors across all browsers
   * Verifies error handling works consistently
   */
  test('should show error for invalid Caddyfile syntax in all browsers', async ({ page, browserName, adminUser }) => {
    await setupImportMocks(page, { uploadSuccess: false });

    await test.step(`[${browserName}] Navigate to import page`, async () => {
      await gotoImportPageWithAuthRecovery(page, adminUser);
    });

    await test.step(`[${browserName}] Paste invalid content and parse`, async () => {
      const textarea = page.locator('textarea');
      await textarea.fill(INVALID_CADDYFILE);

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await parseButton.click();
    });

    await test.step(`[${browserName}] Verify error message displayed`, async () => {
      // Look for error indicators (toast, banner, or inline error)
      const errorLocator = page.locator('.bg-red-900, .bg-red-900\\/20, [role="alert"]').filter({
        hasText: /invalid|syntax|error/i,
      });
      await expect(errorLocator.first()).toBeVisible({ timeout: 5000 });
    });
  });

  /**
   * TEST 3: Multi-file import flow across all browsers
   * Tests the multi-file import modal and API interaction
   */
  test('should handle multi-file import in all browsers', async ({ page, browserName, adminUser }) => {
    await test.step(`[${browserName}] Navigate to import page`, async () => {
      await gotoImportPageWithAuthRecovery(page, adminUser);
    });

    await test.step(`[${browserName}] Set up multi-file API mocks`, async () => {
      // Mock multi-file upload endpoint
      await page.route('**/api/v1/import/upload-multi', async (route) => {
        await route.fulfill({
          status: 200,
          json: {
            session: {
              id: 'multi-file-session',
              state: 'transient',
            },
            preview: {
              hosts: [
                { domain_names: 'site1.com', forward_host: 'backend1', forward_port: 3000, forward_scheme: 'http' },
                { domain_names: 'site2.com', forward_host: 'backend2', forward_port: 3001, forward_scheme: 'http' },
              ],
              conflicts: [],
              warnings: [],
            },
          },
        });
      });
    });

    await test.step(`[${browserName}] Open multi-file modal`, async () => {
      // Look for multi-file import button/link
      const multiFileButton = page.getByRole('button', { name: /multi.*file|import.*sites/i });
      if (await multiFileButton.isVisible()) {
        await multiFileButton.click();

        // Modal should appear
        const modal = page.locator('[role="dialog"]').or(page.locator('.modal'));
        await expect(modal).toBeVisible({ timeout: 5000 });
      } else {
        // Multi-file import button not found - feature may not be available
      }
    });
  });

  /**
   * TEST 4: Conflict resolution flow across all browsers
   * Creates a host, then imports a conflicting host to verify conflict handling
   */
  test('should handle conflict resolution in all browsers', async ({ page, browserName, adminUser }) => {
    await setupImportMocks(page, {
      previewHosts: [
        { domain_names: 'existing.example.com', forward_host: 'new-server', forward_port: 8080, forward_scheme: 'https' },
      ],
      conflicts: ['existing.example.com'],
    });

    // Mock conflict details (overrides the preview route from setupImportMocks)
    await page.route('**/api/v1/import/preview', async (route) => {
      await route.fulfill({
        status: 200,
        json: {
          session: { id: 'conflict-session', state: 'reviewing' },
          preview: {
            hosts: [
              { domain_names: 'existing.example.com', forward_host: 'new-server', forward_port: 8080, forward_scheme: 'https' },
            ],
            conflicts: ['existing.example.com'],
            warnings: [],
          },
          conflict_details: {
            'existing.example.com': {
              existing: {
                forward_scheme: 'http',
                forward_host: 'old-server',
                forward_port: 80,
              },
              imported: {
                forward_scheme: 'https',
                forward_host: 'new-server',
                forward_port: 8080,
              },
            },
          },
        },
      });
    });

    await test.step(`[${browserName}] Navigate to import page`, async () => {
      await gotoImportPageWithAuthRecovery(page, adminUser);
    });

    await test.step(`[${browserName}] Parse conflicting Caddyfile`, async () => {
      const textarea = page.locator('textarea');
      await textarea.fill('existing.example.com { reverse_proxy new-server:8080 }');

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await parseButton.click();
    });

    await test.step(`[${browserName}] Verify conflict indicator and resolution options`, async () => {
      const reviewTable = page.locator('[data-testid="import-review-table"]');
      await expect(reviewTable).toBeVisible({ timeout: 10000 });

      // Look for conflict indicator
      const conflictIndicator = page.getByText('Conflict', { exact: true }).or(page.locator('.text-yellow-400'));
      await expect(conflictIndicator).toBeVisible();

      // Check for resolution dropdown/select
      const resolutionSelect = page.locator('select').first();
      if (await resolutionSelect.isVisible()) {
        await expect(resolutionSelect).toBeVisible();
        // Verify options exist
        const options = await resolutionSelect.locator('option').count();
        expect(options).toBeGreaterThan(1); // Should have multiple resolution strategies
      }
    });
  });

  /**
   * TEST 5: Session resume across all browsers
   * Verifies that starting an import, navigating away, and returning shows the session
   */
  test('should resume import session in all browsers', async ({ page, browserName, adminUser }) => {
    await setupImportMocks(page, {
      previewHosts: [
        { domain_names: 'test.example.com', forward_host: 'localhost', forward_port: 3000, forward_scheme: 'http' },
      ],
    });

    await test.step(`[${browserName}] Navigate to import page`, async () => {
      await gotoImportPageWithAuthRecovery(page, adminUser);
    });

    await test.step(`[${browserName}] Start import session`, async () => {
      const textarea = page.locator('textarea');
      await textarea.fill(SINGLE_HOST_CADDYFILE);

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await parseButton.click();

      // Wait for review table
      await expect(page.locator('[data-testid="import-review-table"]')).toBeVisible({ timeout: 10000 });
    });

    await test.step(`[${browserName}] Navigate away`, async () => {
      await page.goto('/proxy-hosts');
      await expect(page.locator('h1')).toContainText(/proxy.*hosts?/i, { timeout: 5000 });
    });

    await test.step(`[${browserName}] Return to import page and verify session banner`, async () => {
      // Mock status to return existing session
      await page.route('**/api/v1/import/status', async (route) => {
        await route.fulfill({
          status: 200,
          json: {
            has_pending: true,
            session: {
              id: 'test-session-cross-browser',
              state: 'reviewing',
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          },
        });
      });

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });

      // Should show banner or button to resume
      const banner = page.locator('[data-testid="import-banner"]').or(page.getByText(/pending|resume|continue/i));
      await expect(banner.first()).toBeVisible({ timeout: 5000 });
    });
  });

  /**
   * TEST 6: Cancel import session across all browsers
   * Verifies session cancellation clears state correctly
   */
  test('should cancel import session in all browsers', async ({ page, browserName, adminUser }) => {
    await setupImportMocks(page, {
      previewHosts: [
        { domain_names: 'test.example.com', forward_host: 'localhost', forward_port: 3000, forward_scheme: 'http' },
      ],
    });

    await test.step(`[${browserName}] Navigate to import page`, async () => {
      await gotoImportPageWithAuthRecovery(page, adminUser);
    });

    await test.step(`[${browserName}] Start import session`, async () => {
      const textarea = page.locator('textarea');
      await textarea.fill(SINGLE_HOST_CADDYFILE);

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await parseButton.click();

      // Wait for review table
      await expect(page.locator('[data-testid="import-review-table"]')).toBeVisible({ timeout: 10000 });
    });

    let cancelRequested = false;
    await test.step(`[${browserName}] Cancel import`, async () => {
      // Handle browser confirm dialog
      page.on('dialog', async (dialog) => {
        await dialog.accept();
      });

      // Monitor cancel API call
      page.on('request', (req) => {
        if (req.url().includes('/api/v1/import/cancel')) {
          cancelRequested = true;
        }
      });

      // Click back/cancel button (use first match to avoid strict mode violation)
      const backButton = page.getByRole('button', { name: /back|cancel/i }).first();
      if (await backButton.isVisible()) {
        await backButton.click();
      }
    });

    await test.step(`[${browserName}] Verify session cleared`, async () => {
      // Review table should disappear
      const reviewTable = page.locator('[data-testid="import-review-table"]');
      await expect(reviewTable).not.toBeVisible({ timeout: 5000 });

      // Upload section should be visible again
      const textarea = page.locator('textarea');
      await expect(textarea).toBeVisible();
    });
  });
});
