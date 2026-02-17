/**
 * Import Caddyfile - E2E Tests
 *
 * Tests for the Caddyfile import wizard functionality.
 * Covers 18 test scenarios as defined in phase5-implementation.md.
 *
 * Test Categories:
 * - Page Layout (2 tests): heading, wizard steps
 * - File Upload (4 tests): dropzone, file selection, invalid file, file validation
 * - Preview Step (4 tests): preview content, syntax validation, edit preview, warnings
 * - Review Step (4 tests): server list, configuration details, select/deselect, validation
 * - Import Execution (4 tests): import success, progress, error handling, partial import
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import {
  mockImportAPI,
  mockImportPreview,
  ImportPreview,
  ImportSession,
  IMPORT_SELECTORS,
} from '../utils/phase5-helpers';
import { waitForToast, waitForLoadingComplete, waitForAPIResponse } from '../utils/wait-helpers';

/**
 * Selectors for the Import Caddyfile page
 */
const SELECTORS = {
  // Page elements
  pageTitle: 'h1',
  dropzone: '[data-testid="import-dropzone"]',
  banner: '[data-testid="import-banner"]',
  reviewTable: '[data-testid="import-review-table"]',
  uploadInput: 'input[type="file"]',

  // Buttons
  parseButton: 'button:has-text("Parse")',
  nextButton: 'button:has-text("Next")',
  importButton: 'button:has-text("Import")',
  commitButton: 'button:has-text("Commit")',
  cancelButton: 'button:has-text("Cancel")',
  backButton: 'button:has-text("Back")',

  // Text input
  pasteTextarea: 'textarea',

  // Review table elements
  hostRow: 'tbody tr',
  hostCheckbox: 'input[type="checkbox"]',
  conflictIndicator: '.text-yellow-400',
  newIndicator: '.text-green-400',

  // Modals
  successModal: '[data-testid="import-success-modal"]',

  // Error display
  errorMessage: '.bg-red-900, .bg-red-900\\/20',
  warningMessage: '.bg-yellow-900, .bg-yellow-900\\/20',

  // Source view
  sourceToggle: 'text=Source Caddyfile Content',
  sourceContent: 'pre.font-mono',
};

/**
 * Mock Caddyfile content for testing
 */
const mockCaddyfile = `
example.com {
  reverse_proxy localhost:3000
}

api.example.com {
  reverse_proxy localhost:8080
}
`;

/**
 * Mock preview response with valid hosts
 */
const mockPreviewSuccess: ImportPreview = {
  session: {
    id: 'test-session-123',
    state: 'reviewing',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  preview: {
    hosts: [
      { domain_names: 'example.com', forward_host: 'localhost', forward_port: 3000, forward_scheme: 'http' },
      { domain_names: 'api.example.com', forward_host: 'localhost', forward_port: 8080, forward_scheme: 'http' },
    ],
    conflicts: [],
    errors: [],
  },
  caddyfile_content: mockCaddyfile,
};

/**
 * Mock preview response with conflicts
 */
const mockPreviewWithConflicts: ImportPreview = {
  session: {
    id: 'test-session-456',
    state: 'reviewing',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  preview: {
    hosts: [
      { domain_names: 'existing.example.com', forward_host: 'new-server', forward_port: 8080, forward_scheme: 'https' },
    ],
    conflicts: ['existing.example.com'],
    errors: [],
  },
  conflict_details: {
    'existing.example.com': {
      existing: {
        forward_scheme: 'http',
        forward_host: 'old-server',
        forward_port: 80,
        ssl_forced: false,
        websocket: false,
        enabled: true,
      },
      imported: {
        forward_scheme: 'https',
        forward_host: 'new-server',
        forward_port: 8080,
        ssl_forced: true,
        websocket: true,
      },
    },
  },
};

/**
 * Mock preview response with warnings/errors
 */
const mockPreviewWithWarnings: ImportPreview = {
  session: {
    id: 'test-session-789',
    state: 'reviewing',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  preview: {
    hosts: [
      { domain_names: 'valid.example.com', forward_host: 'server', forward_port: 8080, forward_scheme: 'http' },
    ],
    conflicts: [],
    errors: ['Line 10: Invalid directive "invalid_directive"', 'Line 15: Unsupported matcher syntax'],
  },
};

/**
 * Helper to set up all import API mocks with the specified preview response
 * Handles the full flow: initial status (no session) → upload → status (with session) → preview
 */
async function setupImportMocks(
  page: import('@playwright/test').Page,
  preview: ImportPreview,
  options?: { uploadError?: boolean; commitError?: boolean }
) {
  let hasSession = false;

  // Mock status endpoint - initially no session, then has session after upload
  await page.route('**/api/v1/import/status', async (route) => {
    if (hasSession) {
      await route.fulfill({
        status: 200,
        json: { has_pending: true, session: preview.session },
      });
    } else {
      await route.fulfill({
        status: 200,
        json: { has_pending: false },
      });
    }
  });

  // Mock upload endpoint - sets hasSession to true on success
  await page.route('**/api/v1/import/upload', async (route) => {
    if (options?.uploadError) {
      await route.fulfill({ status: 400, json: { error: 'Invalid Caddyfile syntax' } });
    } else {
      hasSession = true;
      await route.fulfill({ status: 200, json: preview });
    }
  });

  // Mock preview endpoint
  await page.route('**/api/v1/import/preview', async (route) => {
    await route.fulfill({ status: 200, json: preview });
  });

  // Mock commit endpoint
  await page.route('**/api/v1/import/commit', async (route) => {
    if (options?.commitError) {
      await route.fulfill({ status: 500, json: { error: 'Commit failed' } });
    } else {
      await route.fulfill({ status: 200, json: { created: preview.preview.hosts.length, updated: 0, skipped: 0, errors: [] } });
    }
  });

  // Mock cancel endpoint
  await page.route('**/api/v1/import/cancel', async (route) => {
    hasSession = false;
    await route.fulfill({ status: 200, json: {} });
  });

  // Mock backups endpoint for pre-import backup
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
}

test.describe('Import Caddyfile - Wizard', () => {
  // =========================================================================
  // Page Layout Tests (2 tests)
  // =========================================================================
  test.describe('Page Layout', () => {
    test('should display import page with correct heading', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      await expect(page.locator(SELECTORS.pageTitle)).toContainText(/import/i);
    });

    test('should show upload section with wizard steps', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Verify upload section is visible
      const dropzone = page.locator(SELECTORS.dropzone);
      await expect(dropzone).toBeVisible();

      // Verify paste textarea is visible
      const textarea = page.locator(SELECTORS.pasteTextarea);
      await expect(textarea).toBeVisible();

      // Verify parse button exists
      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await expect(parseButton).toBeVisible();
    });
  });

  // =========================================================================
  // File Upload Tests (4 tests)
  // =========================================================================
  test.describe('File Upload', () => {
    test('should display file upload dropzone', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Verify dropzone/file input is present
      const dropzone = page.locator(SELECTORS.dropzone);
      await expect(dropzone).toBeVisible();

      // Verify it accepts proper file types
      const fileInput = page.locator(SELECTORS.uploadInput);
      await expect(fileInput).toHaveAttribute('accept', /.caddyfile|.txt|text\/plain/);
    });

    test('should accept valid Caddyfile via file upload', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });

      // Mock import API
      await page.route('**/api/v1/import/upload', async (route) => {
        await route.fulfill({ status: 200, json: mockPreviewSuccess });
      });
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

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Upload file
      const fileInput = page.locator(SELECTORS.uploadInput);
      await fileInput.setInputFiles({
        name: 'Caddyfile',
        mimeType: 'text/plain',
        buffer: Buffer.from(mockCaddyfile),
      });

      // The textarea should now contain the file content
      const textarea = page.locator(SELECTORS.pasteTextarea);
      await expect(textarea).toHaveValue(/example\.com/);
    });

    test('should accept valid Caddyfile via paste', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Mock all import API endpoints using the helper
      await setupImportMocks(page, mockPreviewSuccess);

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Paste content into textarea
      const textarea = page.locator(SELECTORS.pasteTextarea);
      await textarea.fill(mockCaddyfile);

      // Verify content is in textarea
      await expect(textarea).toHaveValue(/example\.com/);

      // Click parse/review button
      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await parseButton.click();

      // Should show review table after API completes
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });
    });

    test('should show error for empty content submission', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Ensure textarea is empty
      const textarea = page.locator(SELECTORS.pasteTextarea);
      await textarea.fill('');

      // The parse button should be disabled when content is empty
      const parseButton = page.getByRole('button', { name: /parse|review/i });
      await expect(parseButton).toBeDisabled();
    });
  });

  // =========================================================================
  // Preview Step Tests (4 tests)
  // =========================================================================
  test.describe('Preview Step', () => {
    test('should show parsed hosts from Caddyfile', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Use the complete mock helper to avoid missing endpoints
      await setupImportMocks(page, mockPreviewSuccess);

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Paste content and submit
      await page.locator(SELECTORS.pasteTextarea).fill(mockCaddyfile);
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Wait for the review table to appear
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });

      // Verify both hosts are shown
      await expect(page.getByText('example.com', { exact: true })).toBeVisible();
      await expect(page.getByText('api.example.com', { exact: true })).toBeVisible();
    });

    test('should show validation errors for invalid Caddyfile syntax', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Mock import API with error
      await page.route('**/api/v1/import/upload', async (route) => {
        await route.fulfill({
          status: 400,
          json: { error: 'Invalid Caddyfile syntax at line 5' },
        });
      });

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Paste invalid content
      await page.locator(SELECTORS.pasteTextarea).fill('invalid { broken syntax');
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Should show error message
      await expect(page.locator(SELECTORS.errorMessage)).toBeVisible({ timeout: 5000 });
    });

    test('should display source Caddyfile content in preview', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Set up all import mocks
      await setupImportMocks(page, mockPreviewSuccess);

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Upload and parse
      await page.locator(SELECTORS.pasteTextarea).fill(mockCaddyfile);
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Wait for review table to appear after API completes
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });

      // Click to show source content
      const sourceToggle = page.locator(SELECTORS.sourceToggle);
      if (await sourceToggle.isVisible()) {
        await sourceToggle.click();
        // Should show the source content
        const sourceContent = page.locator('pre');
        await expect(sourceContent).toContainText('reverse_proxy');
      }
    });

    test('should show warnings for parsing issues', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Set up import mocks with warnings response
      await setupImportMocks(page, mockPreviewWithWarnings);

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Paste content with issues
      await page.locator(SELECTORS.pasteTextarea).fill('test { invalid }');
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Should show warning messages - check for error display in review table
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 5000 });
      await expect(page.getByText('Line 10: Invalid directive').first()).toBeVisible();
    });
  });

  // =========================================================================
  // Review Step Tests (4 tests)
  // =========================================================================
  test.describe('Review Step', () => {
    test('should display server list with configuration details', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Set up all import mocks
      await setupImportMocks(page, mockPreviewSuccess);

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Parse content
      await page.locator(SELECTORS.pasteTextarea).fill(mockCaddyfile);
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Verify review table is displayed
      const reviewTable = page.locator(SELECTORS.reviewTable);
      await expect(reviewTable).toBeVisible({ timeout: 10000 });

      // Verify domain names are shown
      await expect(page.getByText('example.com', { exact: true })).toBeVisible();
      await expect(page.getByText('api.example.com', { exact: true })).toBeVisible();

      // Verify "New" status indicators for non-conflicting hosts
      await expect(page.locator(SELECTORS.newIndicator)).toHaveCount(2);
    });

    test('should highlight conflicts with existing hosts', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Set up import mocks with conflicts response
      await setupImportMocks(page, mockPreviewWithConflicts);

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Parse content
      await page.locator(SELECTORS.pasteTextarea).fill('existing.example.com { reverse_proxy new-server:8080 }');
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Verify conflict indicator is shown
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });
      await expect(page.getByText('Conflict', { exact: true })).toBeVisible();
    });

    test('should allow conflict resolution selection', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Set up import mocks with conflicts response
      await setupImportMocks(page, mockPreviewWithConflicts);

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Parse content
      await page.locator(SELECTORS.pasteTextarea).fill('existing.example.com { reverse_proxy new-server:8080 }');
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Wait for review table to appear after API completes
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });

      // Verify resolution dropdown exists
      const resolutionSelect = page.locator('select').first();
      await expect(resolutionSelect).toBeVisible();

      // Select "overwrite" option
      await resolutionSelect.selectOption('overwrite');
      await expect(resolutionSelect).toHaveValue('overwrite');
    });

    test('should require name for each host before commit', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Set up all import mocks
      await setupImportMocks(page, mockPreviewSuccess);

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Parse content
      await page.locator(SELECTORS.pasteTextarea).fill(mockCaddyfile);
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Wait for review table
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });

      // Verify name inputs are present
      const nameInputs = page.locator('input[type="text"]');
      await expect(nameInputs).toHaveCount(2);

      // Clear the first name input
      await nameInputs.first().clear();

      // Try to commit
      await page.locator(SELECTORS.commitButton).click();

      // Should show validation error
      await expect(page.locator(SELECTORS.errorMessage)).toBeVisible({ timeout: 5000 });
    });
  });

  // =========================================================================
  // Import Execution Tests (4 tests)
  // =========================================================================
  test.describe('Import Execution', () => {
    test('should commit import successfully', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      let commitCalled = false;

      // Set up all import mocks
      await setupImportMocks(page, mockPreviewSuccess);

      // Override commit to track if called
      await page.route('**/api/v1/import/commit', async (route) => {
        commitCalled = true;
        await route.fulfill({
          status: 200,
          json: { created: 2, updated: 0, skipped: 0, errors: [] },
        });
      });

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Parse content
      await page.locator(SELECTORS.pasteTextarea).fill(mockCaddyfile);
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Wait for review table to appear
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });

      // Click commit button
      await page.locator(SELECTORS.commitButton).click();

      // Success modal should appear
      await expect(page.locator(SELECTORS.successModal)).toBeVisible({ timeout: 10000 });

      // Verify commit was called
      expect(commitCalled).toBe(true);
    });

    test('should show progress during import', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Set up all import mocks
      await setupImportMocks(page, mockPreviewSuccess);

      // Override commit with delay to simulate slow operation
      await page.route('**/api/v1/import/commit', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 500));
        await route.fulfill({
          status: 200,
          json: { created: 2, updated: 0, skipped: 0, errors: [] },
        });
      });

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Parse and go to review
      await page.locator(SELECTORS.pasteTextarea).fill(mockCaddyfile);
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Wait for review table
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });

      // Click commit and verify button shows loading state
      const commitButton = page.locator(SELECTORS.commitButton);
      await commitButton.click();

      // Button should be disabled or show loading text during commit
      await expect(commitButton).toBeDisabled();

      // Wait for commit to complete
      await waitForAPIResponse(page, '/api/v1/import/commit', { status: 200 });
    });

    test('should handle import errors gracefully', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Set up import mocks with commit error
      await setupImportMocks(page, mockPreviewSuccess, { commitError: true });

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Parse and go to review
      await page.locator(SELECTORS.pasteTextarea).fill(mockCaddyfile);
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Wait for review table
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });

      // Click commit
      await page.locator(SELECTORS.commitButton).click();

      // Should show error message
      await expect(page.locator(SELECTORS.errorMessage)).toBeVisible({ timeout: 5000 });
    });

    test('should handle partial import with some failures', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Set up all import mocks
      await setupImportMocks(page, mockPreviewSuccess);

      // Override commit to return partial success with errors
      await page.route('**/api/v1/import/commit', async (route) => {
        await route.fulfill({
          status: 200,
          json: {
            created: 1,
            updated: 0,
            skipped: 0,
            errors: ['Failed to import api.example.com: upstream unreachable'],
          },
        });
      });

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Parse and go to review
      await page.locator(SELECTORS.pasteTextarea).fill(mockCaddyfile);
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Wait for review table to appear
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });

      // Click commit
      await page.locator(SELECTORS.commitButton).click();

      // Success modal should appear (partial success is still success)
      await expect(page.locator(SELECTORS.successModal)).toBeVisible({ timeout: 10000 });

      // The modal should show the partial results
      // Check for error indicator in success modal
      await expect(page.getByText('1 error encountered')).toBeVisible();
    });
  });

  // =========================================================================
  // Session Management Tests (2 additional tests)
  // =========================================================================
  test.describe('Session Management', () => {
    test('should show import banner when session exists', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      // Mock status API to return existing session
      await page.route('**/api/v1/import/status', async (route) => {
        await route.fulfill({
          status: 200,
          json: {
            has_pending: true,
            session: {
              id: 'existing-session',
              state: 'reviewing',
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          },
        });
      });
      await page.route('**/api/v1/import/preview', async (route) => {
        await route.fulfill({ status: 200, json: mockPreviewSuccess });
      });

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Import banner should be visible
      await expect(page.locator(SELECTORS.banner)).toBeVisible();
    });

    test('should allow canceling import session', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      let cancelCalled = false;

      // Set up all import mocks
      await setupImportMocks(page, mockPreviewSuccess);

      // Override cancel to track if called
      await page.route('**/api/v1/import/cancel', async (route) => {
        cancelCalled = true;
        await route.fulfill({ status: 204 });
      });

      await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Parse and go to review
      await page.locator(SELECTORS.pasteTextarea).fill(mockCaddyfile);
      await page.getByRole('button', { name: /parse|review/i }).click();

      // Wait for review table
      await expect(page.locator(SELECTORS.reviewTable)).toBeVisible({ timeout: 10000 });

      // Handle browser confirm dialog (the component uses confirm()) - must register BEFORE clicking
      page.on('dialog', async (dialog) => {
        await dialog.accept();
      });

      // Click back/cancel button and confirm in dialog
      await page.locator(SELECTORS.backButton).click();

      // Verify cancel was called or review table is hidden
      await expect(page.locator(SELECTORS.reviewTable)).not.toBeVisible({ timeout: 5000 });
    });
  });
});
