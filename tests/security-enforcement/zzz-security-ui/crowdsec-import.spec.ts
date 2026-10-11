/**
 * Import CrowdSec Configuration - E2E Tests
 *
 * Tests for the CrowdSec configuration import functionality.
 * Covers 8 test scenarios as defined in phase5-implementation.md.
 *
 * Test Categories:
 * - Page Layout (2 tests): heading, form display
 * - File Validation (3 tests): valid file, only .tar.gz offered, import disabled without a file
 * - Import Execution (3 tests): import success, error handling, already exists
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../../fixtures/auth-fixtures';
import { cleanupBackupsCreatedSince } from '../../utils/backup-cleanup';
import { waitForLoadingComplete } from '../../utils/wait-helpers';

/**
 * Selectors for the Import CrowdSec page
 */
const SELECTORS = {
  // Page elements
  pageTitle: 'h1',
  fileInput: '[data-testid="crowdsec-import-file"]',
  progress: '[data-testid="import-progress"]',

  // Buttons
  importButton: 'button:has-text("Import")',

  // Error/success messages
  errorMessage: '.bg-red-900',
  successToast: '[data-testid="toast-success"]',
};

/**
 * Helper to create a mock tar.gz file buffer
 */
function createMockTarGzBuffer(): Buffer {
  return Buffer.from('mock tar.gz content for crowdsec config');
}

test.describe('Import CrowdSec Configuration', () => {
  // The import UI creates a real application backup before each import (#1558).
  let specStartedAt = '';
  test.beforeAll(() => {
    specStartedAt = new Date().toISOString();
  });
  test.afterAll(async () => {
    await cleanupBackupsCreatedSince(specStartedAt);
  });

  // =========================================================================
  // Page Layout Tests (2 tests)
  // =========================================================================
  test.describe('Page Layout', () => {
    test('should display import page with correct heading', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await page.goto('/tasks/import/crowdsec');
      await waitForLoadingComplete(page);

      await expect(page.locator(SELECTORS.pageTitle)).toContainText(/crowdsec|import/i);
    });

    test('should show file upload form with accepted formats', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await page.goto('/tasks/import/crowdsec');
      await waitForLoadingComplete(page);

      // Verify file input is visible
      const fileInput = page.locator(SELECTORS.fileInput);
      await expect(fileInput).toBeVisible();

      // Verify it accepts only .tar.gz
      await expect(fileInput).toHaveAttribute('accept', '.tar.gz');

      // Verify import button exists
      const importButton = page.locator(SELECTORS.importButton);
      await expect(importButton).toBeVisible();

      // Verify progress section exists
      await expect(page.locator(SELECTORS.progress)).toBeVisible();
    });
  });

  // =========================================================================
  // File Validation Tests (3 tests)
  // =========================================================================
  test.describe('File Validation', () => {
    test('should accept valid .tar.gz configuration files', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Mock backup and import APIs
      await page.route('**/api/v1/backups', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 201,
            json: { filename: 'pre-import-backup.tar.gz', size: 1000, time: new Date().toISOString() },
          });
        } else {
          await route.continue();
        }
      });
      await page.route('**/api/v1/admin/crowdsec/import', async (route) => {
        await route.fulfill({
          status: 200,
          json: { message: 'Import successful' },
        });
      });

      await page.goto('/tasks/import/crowdsec');
      await waitForLoadingComplete(page);

      // Upload .tar.gz file
      const fileInput = page.locator(SELECTORS.fileInput);
      await fileInput.setInputFiles({
        name: 'crowdsec-config.tar.gz',
        mimeType: 'application/gzip',
        buffer: createMockTarGzBuffer(),
      });

      // Verify file was accepted (import button should be enabled)
      await expect(page.locator(SELECTORS.importButton)).toBeEnabled();
    });

    test('should offer only .tar.gz in the file picker', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await page.goto('/tasks/import/crowdsec');
      await waitForLoadingComplete(page);

      await expect(page.locator(SELECTORS.fileInput)).toHaveAttribute('accept', '.tar.gz');
    });

    test('should disable import button when no file selected', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await page.goto('/tasks/import/crowdsec');
      await waitForLoadingComplete(page);

      // Import button should be disabled when no file is selected
      await expect(page.locator(SELECTORS.importButton)).toBeDisabled();
    });
  });

  // =========================================================================
  // Import Execution Tests (3 tests)
  // =========================================================================
  test.describe('Import Execution', () => {
    test('should create backup before import and complete successfully', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      let backupCalled = false;
      let importCalled = false;
      const callOrder: string[] = [];

      // Mock backup API
      await page.route('**/api/v1/backups', async (route) => {
        if (route.request().method() === 'POST') {
          backupCalled = true;
          callOrder.push('backup');
          await route.fulfill({
            status: 201,
            json: { filename: 'pre-import-backup.tar.gz', size: 1000, time: new Date().toISOString() },
          });
        } else {
          await route.continue();
        }
      });

      // Mock import API
      await page.route('**/api/v1/admin/crowdsec/import', async (route) => {
        importCalled = true;
        callOrder.push('import');
        await route.fulfill({
          status: 200,
          json: { message: 'CrowdSec configuration imported successfully' },
        });
      });

      await page.goto('/tasks/import/crowdsec');
      await waitForLoadingComplete(page);

      // Upload file
      const fileInput = page.locator(SELECTORS.fileInput);
      await fileInput.setInputFiles({
        name: 'crowdsec-config.tar.gz',
        mimeType: 'application/gzip',
        buffer: createMockTarGzBuffer(),
      });

      // Click import button and wait for import API response concurrently
      await Promise.all([
        page.waitForResponse(r => r.url().includes('/api/v1/admin/crowdsec/import') && r.status() === 200),
        page.locator(SELECTORS.importButton).click(),
      ]);

      // Verify backup was called FIRST, then import
      expect(backupCalled).toBe(true);
      expect(importCalled).toBe(true);
      expect(callOrder).toEqual(['backup', 'import']);

      // Verify success toast - use specific text match
      await expect(page.getByText('CrowdSec config imported')).toBeVisible({ timeout: 10000 });
    });

    test('should handle import errors gracefully', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Mock backup API (success)
      await page.route('**/api/v1/backups', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 201,
            json: { filename: 'pre-import-backup.tar.gz', size: 1000, time: new Date().toISOString() },
          });
        } else {
          await route.continue();
        }
      });

      // Mock import API (failure)
      await page.route('**/api/v1/admin/crowdsec/import', async (route) => {
        await route.fulfill({
          status: 400,
          json: { error: 'Invalid configuration format: missing required field "lapi_url"' },
        });
      });

      await page.goto('/tasks/import/crowdsec');
      await waitForLoadingComplete(page);

      // Upload file
      const fileInput = page.locator(SELECTORS.fileInput);
      await fileInput.setInputFiles({
        name: 'crowdsec-config.tar.gz',
        mimeType: 'application/gzip',
        buffer: createMockTarGzBuffer(),
      });

      // Click import button and wait for import API response concurrently
      await Promise.all([
        page.waitForResponse(r => r.url().includes('/api/v1/admin/crowdsec/import') && r.status() === 400),
        page.locator(SELECTORS.importButton).click(),
      ]);

      // Verify error toast - use specific text match
      await expect(page.getByText(/Import failed/i)).toBeVisible({ timeout: 10000 });
    });

    test('should show loading state during import', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Mock backup API with delay
      await page.route('**/api/v1/backups', async (route) => {
        if (route.request().method() === 'POST') {
          await new Promise((resolve) => setTimeout(resolve, 300));
          await route.fulfill({
            status: 201,
            json: { filename: 'pre-import-backup.tar.gz', size: 1000, time: new Date().toISOString() },
          });
        } else {
          await route.continue();
        }
      });

      // Mock import API with delay
      await page.route('**/api/v1/admin/crowdsec/import', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 500));
        await route.fulfill({
          status: 200,
          json: { message: 'Import successful' },
        });
      });

      await page.goto('/tasks/import/crowdsec');
      await waitForLoadingComplete(page);

      // Upload file
      const fileInput = page.locator(SELECTORS.fileInput);
      await fileInput.setInputFiles({
        name: 'crowdsec-config.tar.gz',
        mimeType: 'application/gzip',
        buffer: createMockTarGzBuffer(),
      });

      // Set up response promise before clicking to capture loading state
      const importResponsePromise = page.waitForResponse(r => r.url().includes('/api/v1/admin/crowdsec/import') && r.status() === 200);

      // Click import button
      const importButton = page.locator(SELECTORS.importButton);
      await importButton.click();

      // Button should be disabled during import (loading state)
      await expect(importButton).toBeDisabled();

      // Wait for import to complete
      await importResponsePromise;

      // Button should be enabled again after completion
      await expect(importButton).toBeEnabled();
    });
  });

  // =========================================================================
  // Blanked notification values (pending until the feature is enabled)
  // =========================================================================
  test.describe('Notification files that need values', () => {
    const NEEDS_VALUES_LINE = '[data-testid="import-needs-values"]';

    /** Mocks the backup and import answers, then imports a file through the CrowdSec page. */
    async function importWithResult(page: Page, result: Record<string, unknown>) {
      await page.route('**/api/v1/backups', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 201,
            json: { filename: 'pre-import-backup.tar.gz', size: 1000, time: new Date().toISOString() },
          });
        } else {
          await route.continue();
        }
      });
      await page.route('**/api/v1/admin/crowdsec/import', (route) =>
        route.fulfill({ status: 200, json: { status: 'imported', backup: 'backup-1', ...result } }),
      );
      await page.goto('/security/crowdsec');
      await waitForLoadingComplete(page);
      await page.getByTestId('import-file').setInputFiles({
        name: 'crowdsec-config.tar.gz',
        mimeType: 'application/gzip',
        buffer: createMockTarGzBuffer(),
      });
      await page.getByTestId('import-btn').click();
    }

    test.fixme('shows the files that need values inline in the import card', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      await importWithResult(page, { needs_values: ['notifications/http_default.yaml'] });

      const line = page.locator(NEEDS_VALUES_LINE);
      await expect(line).toBeVisible();
      await expect(line).toContainText('notifications/http_default.yaml');
    });

    test.fixme('adds no notification beyond the existing success toast', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      await importWithResult(page, { needs_values: ['notifications/http_default.yaml'] });

      await expect(page.locator(NEEDS_VALUES_LINE)).toBeVisible();
      await expect(page.locator(SELECTORS.successToast)).toHaveCount(1);
      await expect(page.getByRole('alert')).toHaveCount(0);
    });

    test.fixme('shows no line when no file needs values', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      await importWithResult(page, {});

      await expect(page.locator(SELECTORS.successToast)).toBeVisible();
      await expect(page.locator(NEEDS_VALUES_LINE)).toHaveCount(0);
    });

    test.fixme('removes the line when another file is chosen', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await importWithResult(page, { needs_values: ['notifications/http_default.yaml'] });
      await expect(page.locator(NEEDS_VALUES_LINE)).toBeVisible();

      await page.getByTestId('import-file').setInputFiles({
        name: 'other-config.tar.gz',
        mimeType: 'application/gzip',
        buffer: createMockTarGzBuffer(),
      });

      await expect(page.locator(NEEDS_VALUES_LINE)).toHaveCount(0);
    });
  });
});
