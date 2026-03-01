/**
 * Caddy Import - Firefox-Specific E2E Tests
 *
 * Tests Firefox-specific edge cases and behaviors to prevent regressions
 * like GitHub Issue #567 (Parse button not working in Firefox).
 *
 * EXECUTION:
 * npx playwright test tests/firefox-specific --project=firefox
 *
 * SCOPE:
 * - Event listener attachment and propagation
 * - Async state update race conditions
 * - CORS preflight handling (if applicable)
 * - Cookie/auth header transmission
 * - Button double-click protection
 * - Large file handling
 *
 * NOTE: Tests are skipped if not running in Firefox browser.
 */

import { test, expect } from '../../fixtures/auth-fixtures';
import { Page } from '@playwright/test';
import {
  ensureImportUiPreconditions,
  resetImportSession,
  waitForSuccessfulImportResponse,
} from './import-page-helpers';

/**
 * Helper to set up import API mocks
 */
async function setupImportMocks(page: Page, success: boolean = true) {
  let hasSession = false;

  await page.route('**/api/v1/import/status', async (route) => {
    await route.fulfill({
      status: 200,
      json: hasSession
        ? {
            has_pending: true,
            session: {
              id: 'firefox-test-session',
              state: 'reviewing',
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          }
        : { has_pending: false },
    });
  });

  await page.route('**/api/v1/import/upload', async (route) => {
    if (success) {
      hasSession = true;
      await route.fulfill({
        status: 200,
        json: {
          session: {
            id: 'firefox-test-session',
            state: 'transient',
            source_file: '/imports/uploads/firefox-test-session.caddyfile',
          },
          preview: {
            hosts: [
              { domain_names: 'test.example.com', forward_host: 'localhost', forward_port: 3000, forward_scheme: 'http' },
            ],
            conflicts: [],
            warnings: [],
          },
          caddyfile_content: 'test.example.com { reverse_proxy localhost:3000 }',
        },
      });
    } else {
      await route.fulfill({
        status: 400,
        json: { error: 'Invalid Caddyfile syntax' },
      });
    }
  });

  await page.route('**/api/v1/backups', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({
        status: 201,
        json: { filename: 'backup.tar.gz', size: 1000, time: new Date().toISOString() },
      });
    } else {
      await route.continue();
    }
  });
}

test.describe('Caddy Import - Firefox-Specific @firefox-only', () => {
  /**
   * TEST 1: Event listener attachment verification
   * Ensures the Parse button has proper click handlers in Firefox
   */
  test('should have click handler attached to Parse button', async ({ page, adminUser }) => {

    await test.step('Navigate to import page', async () => {
      await setupImportMocks(page);
      await resetImportSession(page);
      await ensureImportUiPreconditions(page, adminUser);
    });

    await test.step('Verify Parse button exists and is interactive', async () => {
      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await expect(parseButton).toBeVisible();

      // Button should not be disabled when content exists
      const textarea = page.locator('textarea');
      await textarea.fill('test.example.com { reverse_proxy localhost:3000 }');
      await expect(parseButton).toBeEnabled();

      // Firefox-safe actionability check without mutating state.
      await parseButton.click({ trial: true });
    });

    await test.step('Verify click event fires in Firefox', async () => {
      const parseButton = page.getByRole('button', { name: /parse|review/i });
      const response = await waitForSuccessfulImportResponse(
        page,
        () => parseButton.click(),
        'firefox-click-handler'
      );
      const request = response.request();
      expect(request.url()).toContain('/api/v1/import/upload');
      expect(request.method()).toBe('POST');
    });
  });

  /**
   * TEST 2: Async state update race condition
   * Firefox's event loop may expose race conditions in state updates
   */
  test('should handle rapid click and state updates', async ({ page, adminUser }) => {

    await test.step('Navigate to import page', async () => {
      await setupImportMocks(page);
      await ensureImportUiPreconditions(page, adminUser);
    });

    await test.step('Set up API mock with slight delay', async () => {
      await page.route('**/api/v1/import/upload', async (route) => {
        // Simulate network delay
        await new Promise((resolve) => setTimeout(resolve, 100));
        await route.fulfill({
          status: 200,
          json: {
            session: {
              id: 'rapid-click-session',
              state: 'transient',
            },
            preview: {
              hosts: [
                { domain_names: 'rapid.example.com', forward_host: 'localhost', forward_port: 3000, forward_scheme: 'http' },
              ],
              conflicts: [],
              warnings: [],
            },
          },
        });
      });
    });

    await test.step('Fill content and click immediately', async () => {
      const textarea = page.locator('textarea');
      // Fill content rapidly
      await textarea.fill('rapid.example.com { reverse_proxy localhost:3000 }');

      // Click parse button immediately after fill (no extra wait)
      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await parseButton.click();

      // Verify request was sent despite rapid action
      const reviewTable = page.locator('[data-testid="import-review-table"]');
      await expect(reviewTable).toBeVisible({ timeout: 10000 });
      await expect(page.getByText('rapid.example.com')).toBeVisible();
    });
  });

  /**
   * TEST 3: CORS preflight handling
   * Firefox has stricter CORS enforcement; verify no preflight issues
   */
  test('should handle CORS correctly (same-origin)', async ({ page, adminUser }) => {
    await test.step('Navigate to import page', async () => {
      await setupImportMocks(page);
      await ensureImportUiPreconditions(page, adminUser);
    });

    const corsIssues: string[] = [];
    await test.step('Monitor for CORS errors', async () => {
      // Listen for failed requests
      page.on('requestfailed', (request) => {
        corsIssues.push(`Failed: ${request.url()} - ${request.failure()?.errorText}`);
      });

      // Listen for console errors
      page.on('console', (msg) => {
        if (msg.type() === 'error' && msg.text().toLowerCase().includes('cors')) {
          corsIssues.push(`Console: ${msg.text()}`);
        }
      });
    });

    await test.step('Perform import and check for CORS issues', async () => {
      const textarea = page.locator('textarea');
      await textarea.fill('cors-test.example.com { reverse_proxy localhost:3000 }');

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await waitForSuccessfulImportResponse(
        page,
        () => parseButton.click(),
        'firefox-cors-same-origin',
        /\/api\/v1\/import\/upload/i
      );

      // Verify no CORS issues
      expect(corsIssues).toHaveLength(0);
    });
  });

  /**
   * TEST 4: Cookie/auth header verification
   * Ensures Firefox sends auth cookies correctly with API requests
   */
  test('should send authentication cookies with requests', async ({ page, adminUser }) => {
    await test.step('Navigate to import page', async () => {
      await setupImportMocks(page);
      await ensureImportUiPreconditions(page, adminUser);
    });

    let requestHeaders: Record<string, string> = {};
    await test.step('Monitor request headers', async () => {
      page.on('request', (request) => {
        if (request.url().includes('/api/v1/import/upload')) {
          requestHeaders = request.headers();
        }
      });
    });

    await test.step('Perform import and verify auth headers', async () => {
      const textarea = page.locator('textarea');
      await textarea.fill('auth-test.example.com { reverse_proxy localhost:3000 }');

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await parseButton.click();

      // Wait for request to complete
      await page.waitForResponse((r) => r.url().includes('/api/v1/import/upload'), { timeout: 5000 });

      // Verify headers were captured
      expect(Object.keys(requestHeaders).length).toBeGreaterThan(0);

      // Verify cookie or authorization header present
      const hasCookie = !!requestHeaders['cookie'];
      const hasAuth = !!requestHeaders['authorization'];
      expect(hasCookie || hasAuth).toBeTruthy();

      // Verify content-type is correct
      expect(requestHeaders['content-type']).toContain('application/json');
    });
  });

  /**
   * TEST 5: Button double-click protection
   * Firefox must prevent duplicate API requests from rapid clicks
   */
  test('should prevent duplicate requests on double-click', async ({ page, adminUser }) => {
    await test.step('Navigate to import page', async () => {
      await setupImportMocks(page);
      await ensureImportUiPreconditions(page, adminUser);
    });

    const requestCount: string[] = [];
    await test.step('Monitor API request count', async () => {
      page.on('request', (request) => {
        if (request.url().includes('/api/v1/import/upload')) {
          requestCount.push(request.method());
        }
      });
    });

    await test.step('Double-click Parse button rapidly', async () => {
      const textarea = page.locator('textarea');
      await textarea.fill('doubleclick.example.com { reverse_proxy localhost:3000 }');

      const parseButton = page.getByRole('button', { name: /parse|review/i });

      // Double-click rapidly (Firefox may handle differently than Chromium)
      await parseButton.click({ clickCount: 2, delay: 50 });

      // Wait for any requests to complete
      await page.waitForTimeout(2000);

      // Should only send ONE request despite double-click
      expect(requestCount.length).toBeLessThanOrEqual(1);
    });

    await test.step('Verify button disabled during processing', async () => {
      // If a request was sent, button should have been disabled
      if (requestCount.length > 0) {
        const reviewTable = page.locator('[data-testid="import-review-table"]');
        await expect(reviewTable).toBeVisible({ timeout: 10000 });
      }
    });
  });

  /**
   * TEST 6: Large file handling
   * Verifies Firefox handles large Caddyfile uploads without lag or timeout
   */
  test('should handle large Caddyfile upload (10KB+)', async ({ page, adminUser }) => {
    await test.step('Navigate to import page', async () => {
      await setupImportMocks(page);
      await resetImportSession(page);
      await ensureImportUiPreconditions(page, adminUser);
    });

    await test.step('Generate large Caddyfile content', async () => {
      // Generate deterministic payload >10KB for all browsers/runtimes.
      let largeCaddyfile = '';
      for (let i = 0; i < 180; i++) {
        largeCaddyfile += `
host${i}.example.com {
  reverse_proxy backend${i}:${3000 + i}
  tls internal
  encode gzip
}
`;
      }

      const textarea = page.locator('textarea');
      await textarea.fill(largeCaddyfile);

      // Verify no UI lag (textarea should update immediately)
      const value = await textarea.inputValue();
      expect(value.length).toBeGreaterThan(10000);
      expect(value).toContain('host179.example.com');
    });

    await test.step('Upload large file to API', async () => {
      // Mock upload with large payload
      await page.route('**/api/v1/import/upload', async (route) => {
        const postData = route.request().postData();
        expect(postData).toBeTruthy();
        expect(postData!.length).toBeGreaterThan(10000);

        await route.fulfill({
          status: 200,
          json: {
            session: {
              id: 'large-file-session',
              state: 'transient',
            },
            preview: {
              hosts: Array.from({ length: 100 }, (_, i) => ({
                domain_names: `host${i}.example.com`,
                forward_host: `backend${i}`,
                forward_port: 3000 + i,
                forward_scheme: 'http',
              })),
              conflicts: [],
              warnings: [],
            },
          },
        });
      });

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await parseButton.click();

      // Should complete within reasonable time (15 seconds)
      const reviewTable = page.locator('[data-testid="import-review-table"]');
      await expect(reviewTable).toBeVisible({ timeout: 15000 });

      // Verify hosts are displayed
      await expect(page.getByText('host0.example.com')).toBeVisible();
    });
  });
});
