/**
 * Caddy Import - WebKit-Specific E2E Tests
 *
 * Tests WebKit (Safari) specific edge cases and behaviors to ensure
 * Caddyfile import works correctly on macOS/iOS Safari.
 *
 * EXECUTION:
 * npx playwright test tests/webkit-specific --project=webkit
 *
 * SCOPE:
 * - Event listener attachment and propagation
 * - Async state management  * - Form submission behavior
 * - Cookie/session storage handling
 * - Touch event handling (iOS simulation)
 * - Large file performance
 *
 * NOTE: Tests are skipped if not running in WebKit browser.
 */

import { test, expect } from '../../fixtures/auth-fixtures';
import { Page } from '@playwright/test';
import {
  attachImportDiagnostics,
  ensureImportUiPreconditions,
  logImportFailureContext,
  resetImportSession,
  waitForSuccessfulImportResponse,
} from './import-page-helpers';

function webkitOnly(browserName: string) {
  test.skip(browserName !== 'webkit', 'This suite only runs on WebKit');
}

const WEBKIT_TEST_EMAIL = process.env.E2E_TEST_EMAIL || 'e2e-test@example.com';
const WEBKIT_TEST_PASSWORD = process.env.E2E_TEST_PASSWORD || 'TestPassword123!';

async function ensureWebkitAuthSession(page: Page): Promise<void> {
  await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });

  const emailInput = page
    .getByRole('textbox', { name: /email/i })
    .first()
    .or(page.locator('input[type="email"]').first());
  const passwordInput = page.locator('input[type="password"]').first();
  const loginButton = page.getByRole('button', { name: /login|sign in/i }).first();

  const [emailVisible, passwordVisible, loginButtonVisible] = await Promise.all([
    emailInput.isVisible().catch(() => false),
    passwordInput.isVisible().catch(() => false),
    loginButton.isVisible().catch(() => false),
  ]);

  const loginUiPresent = emailVisible && passwordVisible && loginButtonVisible;
  const loginRoute = page.url().includes('/login');

  if (loginUiPresent || loginRoute) {
    if (!loginRoute) {
      await page.goto('/login', { waitUntil: 'domcontentloaded' });
    }

    await emailInput.fill(WEBKIT_TEST_EMAIL);
    await passwordInput.fill(WEBKIT_TEST_PASSWORD);

    const loginResponsePromise = page
      .waitForResponse(
        (response) => response.url().includes('/api/v1/auth/login') && response.request().method() === 'POST',
        { timeout: 15000 }
      )
      .catch(() => null);

    await loginButton.click();
    await loginResponsePromise;
    await page.waitForURL((url) => !url.pathname.includes('/login'), {
      timeout: 15000,
      waitUntil: 'domcontentloaded',
    });
  }

  const meResponse = await page.request.get('/api/v1/auth/me');
  if (!meResponse.ok()) {
    throw new Error(
      `WebKit auth bootstrap verification failed: /api/v1/auth/me returned ${meResponse.status()} at ${page.url()}`
    );
  }
}

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
              id: 'webkit-test-session',
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
            id: 'webkit-test-session',
            state: 'transient',
            source_file: '/imports/uploads/webkit-test-session.caddyfile',
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

test.describe('Caddy Import - WebKit-Specific @webkit-only', () => {
  const diagnosticsByPage = new WeakMap<Page, () => void>();

  test.beforeEach(async ({ browserName, page, adminUser }) => {
    webkitOnly(browserName);
    diagnosticsByPage.set(page, attachImportDiagnostics(page, 'caddy-import-webkit'));
    await setupImportMocks(page);
    await ensureWebkitAuthSession(page);
    await resetImportSession(page);
    await ensureImportUiPreconditions(page, adminUser);
  });

  test.afterEach(async ({ page }, testInfo) => {
    diagnosticsByPage.get(page)?.();
    if (testInfo.status !== 'passed') {
      await logImportFailureContext(page, 'caddy-import-webkit');
    }
    await resetImportSession(page).catch(() => {
      // Best-effort cleanup to avoid leaking pending import sessions to subsequent tests.
    });
  });

  /**
   * TEST 1: Event listener attachment verification
   * Safari/WebKit may handle React event delegation differently
   */
  test('should have click handler attached to Parse button', async ({ page, adminUser }) => {
    await test.step('Navigate to import page', async () => {
      await ensureImportUiPreconditions(page, adminUser);
    });

    await test.step('Verify Parse button is clickable in WebKit', async () => {
      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await expect(parseButton).toBeVisible();

      // Fill content to enable button
      const textarea = page.locator('textarea');
      await textarea.fill('webkit-test.example.com { reverse_proxy localhost:3000 }');
      await expect(parseButton).toBeEnabled();

      // Verify button responds to pointer events (Safari-specific check)
      const hasPointerEvents = await parseButton.evaluate((btn) => {
        const style = window.getComputedStyle(btn);
        return style.pointerEvents !== 'none';
      });
      expect(hasPointerEvents).toBeTruthy();
    });

    await test.step('Verify click sends API request', async () => {
      const parseButton = page.getByRole('button', { name: /parse|review/i });
      const response = await waitForSuccessfulImportResponse(
        page,
        () => parseButton.click(),
        'webkit-click-handler'
      );
      const request = response.request();
      expect(request.url()).toContain('/api/v1/import/upload');
      expect(request.method()).toBe('POST');
    });
  });

  /**
   * TEST 2: Async state update race condition
   * WebKit's JavaScript engine (JavaScriptCore) may have different timing
   */
  test('should handle async state updates correctly', async ({ page, adminUser }) => {
    await test.step('Navigate to import page', async () => {
      await ensureImportUiPreconditions(page, adminUser);
    });

    await test.step('Set up API mock with delay', async () => {
      await page.route('**/api/v1/import/upload', async (route) => {
        // Simulate network latency
        await new Promise((resolve) => setTimeout(resolve, 150));
        await route.fulfill({
          status: 200,
          json: {
            session: {
              id: 'webkit-async-session',
              state: 'transient',
            },
            preview: {
              hosts: [
                { domain_names: 'async.example.com', forward_host: 'localhost', forward_port: 3000, forward_scheme: 'http' },
              ],
              conflicts: [],
              warnings: [],
            },
          },
        });
      });
    });

    await test.step('Fill and submit rapidly', async () => {
      const textarea = page.locator('textarea');
      await textarea.fill('async.example.com { reverse_proxy localhost:3000 }');

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await waitForSuccessfulImportResponse(page, () => parseButton.click(), 'webkit-async-state');

      // Verify UI updates correctly after async operation
      const reviewTable = page.locator('[data-testid="import-review-table"]');
      await expect(reviewTable).toBeVisible({ timeout: 10000 });
      await expect(page.getByText('async.example.com')).toBeVisible();
    });
  });

  /**
   * TEST 3: Form submission behavior
   * Safari may treat button clicks inside forms differently
   */
  test('should handle button click without form submission', async ({ page, adminUser }) => {
    await test.step('Navigate to import page', async () => {
      await ensureImportUiPreconditions(page, adminUser);
    });

    const navigationOccurred: string[] = [];
    await test.step('Monitor for unexpected navigation', async () => {
      page.on('framenavigated', (frame) => {
        if (frame === page.mainFrame()) {
          navigationOccurred.push(frame.url());
        }
      });
    });

    await test.step('Click Parse button and verify no form submission', async () => {
      const textarea = page.locator('textarea');
      await textarea.fill('form-test.example.com { reverse_proxy localhost:3000 }');

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await waitForSuccessfulImportResponse(page, () => parseButton.click(), 'webkit-form-submit');

      // Verify no full-page navigation occurred (only initial + maybe same URL)
      const uniqueUrls = [...new Set(navigationOccurred)];
      expect(uniqueUrls.length).toBeLessThanOrEqual(1);

      // Review table should appear without page reload
      const reviewTable = page.locator('[data-testid="import-review-table"]');
      await expect(reviewTable).toBeVisible();
    });
  });

  /**
   * TEST 4: Cookie/session storage handling
   * WebKit's cookie/storage behavior may differ from Chromium
   */
  test('should maintain session state and send cookies', async ({ page, adminUser }) => {
    await test.step('Navigate to import page', async () => {
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

    await test.step('Perform import and verify cookies sent', async () => {
      const textarea = page.locator('textarea');
      await textarea.fill('cookie-test.example.com { reverse_proxy localhost:3000 }');

      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await waitForSuccessfulImportResponse(page, () => parseButton.click(), 'webkit-cookie-session');

      // Verify headers captured
      expect(Object.keys(requestHeaders).length).toBeGreaterThan(0);

      // Verify cookie or authorization present
      const hasCookie = !!requestHeaders['cookie'];
      const hasAuth = !!requestHeaders['authorization'];
      expect(hasCookie || hasAuth).toBeTruthy();
    });
  });

  /**
   * TEST 5: Button interaction after rapid state changes
   * Safari may handle rapid state updates differently
   */
  test('should handle button state changes correctly', async ({ page, adminUser }) => {
    await test.step('Navigate to import page with clean import state', async () => {
      await resetImportSession(page);
      await ensureImportUiPreconditions(page, adminUser);

      const textarea = page.locator('textarea').first();
      await expect(textarea).toBeVisible();
      await expect(page.getByText(/pending import session/i).first()).toBeHidden();

      // Deterministic baseline: empty import input must keep Parse disabled.
      await textarea.clear();
      await expect(textarea).toHaveValue('');

      const parseButton = page.getByRole('button', { name: /parse|review/i }).first();
      await expect(parseButton).toBeVisible();
      await expect(parseButton).toBeDisabled();
    });

    await test.step('Rapidly fill content and check button state', async () => {
      const textarea = page.locator('textarea').first();
      const parseButton = page.getByRole('button', { name: /parse|review/i }).first();

      // Fill content - button should enable
      await textarea.fill('rapid.example.com { reverse_proxy localhost:3000 }');
      await expect(parseButton).toBeEnabled();

      // Clear content - button should disable again
      await textarea.clear();
      await expect(parseButton).toBeDisabled();

      // Fill again - button should enable
      await textarea.fill('rapid2.example.com { reverse_proxy localhost:3001 }');
      await expect(parseButton).toBeEnabled();
    });

    await test.step('Click button and verify loading state', async () => {
      await page.route('**/api/v1/import/upload', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 250));
        await route.fulfill({
          status: 200,
          json: {
            session: {
              id: 'webkit-button-state-session',
              state: 'transient',
            },
            preview: {
              hosts: [
                {
                  domain_names: 'rapid2.example.com',
                  forward_host: 'localhost',
                  forward_port: 3001,
                  forward_scheme: 'http',
                },
              ],
              conflicts: [],
              warnings: [],
            },
          },
        });
      });

      const parseButton = page.getByRole('button', { name: /parse and review/i }).first();
      const importResponsePromise = waitForSuccessfulImportResponse(
        page,
        () => parseButton.click(),
        'webkit-button-state'
      );
      await importResponsePromise;

      // After completion, review table should appear
      const reviewTable = page.locator('[data-testid="import-review-table"]');
      await expect(reviewTable).toBeVisible({ timeout: 10000 });
      await expect(page.getByRole('button', { name: /review changes/i }).first()).toBeEnabled();
    });
  });

  /**
   * TEST 6: Large file handling
   * WebKit memory management may differ from Chromium/Firefox
   */
  test('should handle large Caddyfile upload without memory issues', async ({ page, adminUser }) => {
    await test.step('Navigate to import page', async () => {
      await ensureImportUiPreconditions(page, adminUser);
    });

    await test.step('Generate and paste large Caddyfile', async () => {
      // Generate 100 host entries
      let largeCaddyfile = '';
      for (let i = 0; i < 100; i++) {
        largeCaddyfile += `
safari-host${i}.example.com {
  reverse_proxy backend${i}:${3000 + i}
  tls internal
  encode gzip
  header -Server
}
`;
      }

      const textarea = page.locator('textarea');
      await textarea.fill(largeCaddyfile);

      // Verify textarea updated correctly
      const value = await textarea.inputValue();
      expect(value.length).toBeGreaterThan(10000);
      expect(value).toContain('safari-host99.example.com');
    });

    await test.step('Upload large file', async () => {
      await page.route('**/api/v1/import/upload', async (route) => {
        const postData = route.request().postData();
        expect(postData).toBeTruthy();
        expect(postData!.length).toBeGreaterThan(10000);

        await route.fulfill({
          status: 200,
          json: {
            session: {
              id: 'webkit-large-file-session',
              state: 'transient',
            },
            preview: {
              hosts: Array.from({ length: 100 }, (_, i) => ({
                domain_names: `safari-host${i}.example.com`,
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
      await waitForSuccessfulImportResponse(page, () => parseButton.click(), 'webkit-large-file');

      // Should complete within reasonable time
      const reviewTable = page.locator('[data-testid="import-review-table"]');
      await expect(reviewTable).toBeVisible({ timeout: 15000 });

      // Verify hosts rendered
      await expect(page.getByText('safari-host0.example.com')).toBeVisible();
    });
  });
});
