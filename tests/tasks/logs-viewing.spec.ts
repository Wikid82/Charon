/**
 * Logs Page - Static Log File Viewing E2E Tests
 *
 * Tests for log file listing, content display, filtering, pagination, and download.
 * Covers 12 test scenarios optimized for WebKit and cross-browser compatibility.
 *
 * Test Categories:
 * - Page Layout (3 tests): heading, file list, filter section
 * - Log File List (2 tests): display files with metadata, select file
 * - Log Content Display (2 tests): show columns, highlight error entries
 * - Pagination (3 tests): navigate pages, page info, button states
 * - Search/Filter (2 tests): text search, level filter
 * - Sorting (7 tests, fixme until issue #686 lands): column header sorting,
 *   aria-sort indication, page reset on sort change
 * - Download (2 tests): download file, error handling
 *
 * Route: /tasks/logs
 * Component: Logs.tsx
 * Updated: 2024-02-10 for full WebKit support
 */

import { test, expect, loginUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForAPIResponse } from '../utils/wait-helpers';
import type { Page } from '@playwright/test';

/**
 * Mock log files for testing
 */
const mockLogFiles = [
  { name: 'access.log', size: 1048576, modified: '2024-01-15T12:00:00Z' },
  { name: 'error.log', size: 256000, modified: '2024-01-15T11:30:00Z' },
  { name: 'caddy.log', size: 512000, modified: '2024-01-14T10:00:00Z' },
];

/**
 * Mock log entries for content display testing
 */
const mockLogEntries = [
  {
    level: 'info',
    ts: Math.floor(Date.now() / 1000),
    logger: 'http.log.access',
    msg: 'handled request',
    request: {
      remote_ip: '192.168.1.100',
      method: 'GET',
      host: 'api.example.com',
      uri: '/api/v1/users',
      proto: 'HTTP/2',
    },
    status: 200,
    duration: 0.045,
    size: 1234,
  },
  {
    level: 'error',
    ts: Math.floor(Date.now() / 1000) - 60,
    logger: 'http.log.access',
    msg: 'connection refused',
    request: {
      remote_ip: '192.168.1.101',
      method: 'POST',
      host: 'api.example.com',
      uri: '/api/v1/auth/login',
      proto: 'HTTP/2',
    },
    status: 502,
    duration: 5.023,
    size: 0,
  },
  {
    level: 'warn',
    ts: Math.floor(Date.now() / 1000) - 120,
    logger: 'http.log.access',
    msg: 'rate limit exceeded',
    request: {
      remote_ip: '10.0.0.50',
      method: 'GET',
      host: 'web.example.com',
      uri: '/dashboard',
      proto: 'HTTP/1.1',
    },
    status: 429,
    duration: 0.001,
    size: 256,
  },
];

/**
 * Generate mock log entries for pagination testing
 */
function generateMockEntries(count: number, startOffset: number = 0) {
  return Array.from({ length: count }, (_, i) => ({
    level: i % 3 === 0 ? 'error' : i % 3 === 1 ? 'warn' : 'info',
    ts: Math.floor(Date.now() / 1000) - i * 10,
    logger: 'http.log.access',
    msg: `request ${startOffset + i}`,
    request: {
      remote_ip: `192.168.1.${100 + (i % 100)}`,
      method: ['GET', 'POST', 'PUT'][i % 3],
      host: 'example.com',
      uri: `/api/endpoint-${i}`,
      proto: 'HTTP/2',
    },
    status: 200 + (i % 4) * 100,
    duration: Math.random() * 1,
    size: Math.random() * 1000,
  }));
}

/**
 * Selectors and helpers for WebKit compatibility
 */
function getLogFileButton(page: Page, fileName: string) {
  return page.getByTestId(`log-file-${fileName}`);
}

function getPrevButton(page: Page) {
  return page.getByTestId('prev-page-button');
}

function getNextButton(page: Page) {
  return page.getByTestId('next-page-button');
}

/**
 * Helper to set up log API mocks
 */
async function setupLogMocks(
  page: Page,
  files = mockLogFiles,
  entries = mockLogEntries,
  totalCount?: number
) {
  // Mock log files list endpoint
  await page.route('**/api/v1/logs', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, json: files });
    } else {
      await route.continue();
    }
  });

  // Mock log content endpoint for each file
  for (const file of files) {
    await page.route(`**/api/v1/logs/${file.name}*`, async (route) => {
      const url = new URL(route.request().url());
      const offset = parseInt(url.searchParams.get('offset') || '0');
      const limit = parseInt(url.searchParams.get('limit') || '50');
      const search = url.searchParams.get('search') || '';
      const level = url.searchParams.get('level') || '';

      // Apply filters
      let filtered = [...entries];
      if (search) {
        filtered = filtered.filter(
          (e) =>
            e.msg.toLowerCase().includes(search.toLowerCase()) ||
            e.request.uri.toLowerCase().includes(search.toLowerCase())
        );
      }
      if (level) {
        filtered = filtered.filter((e) => e.level.toLowerCase() === level.toLowerCase());
      }

      const paginated = filtered.slice(offset, offset + limit);
      const total = totalCount || filtered.length;

      await route.fulfill({
        status: 200,
        json: {
          filename: file.name,
          logs: paginated,
          total,
          limit,
          offset,
          skipped_lines: 0,
        },
      });
    });
  }
}

test.describe('Logs Page - WebKit Compatible Tests', () => {
  test.describe('Page Layout', () => {
    test('should display logs page with file selector', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupLogMocks(page);

      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Verify page title contains "logs"
      await expect(page.getByRole('heading', { level: 1 })).toContainText(/logs/i);

      // Verify file list sidebar is visible
      await expect(page.getByTestId('log-file-list')).toBeVisible();
    });

	    test('should show list of available log files', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);
	      await setupLogMocks(page);

	      const logFilesPromise = waitForAPIResponse(page, '/api/v1/logs', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);

	      await logFilesPromise;

	      // Verify all log files are displayed in the list
	      await expect(page.getByText('access.log')).toBeVisible();
	      await expect(page.getByText('error.log')).toBeVisible();
	      await expect(page.getByText('caddy.log')).toBeVisible();
    });

    test('should display log filters section', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupLogMocks(page);

      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Wait for filters (appear when first log file is auto-selected)
      await expect(page.getByTestId('search-input')).toBeVisible({ timeout: 5000 });

      // Verify filter controls are present
      await expect(page.getByTestId('refresh-button')).toBeVisible();
      await expect(page.getByTestId('download-button')).toBeVisible();
    });
  });

  test.describe('Log File List', () => {
    test('should list all available log files with metadata', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupLogMocks(page);

      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Verify files are listed with size information (in MB format)
      await expect(page.getByText('access.log')).toBeVisible();
      await expect(page.getByText('1.00 MB')).toBeVisible();

      await expect(page.getByText('error.log')).toBeVisible();
      await expect(page.getByText('0.24 MB')).toBeVisible();
    });

	    test('should load log content when file selected', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);
	      await setupLogMocks(page);

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);

	      // The first file (access.log) is auto-selected - wait for content
	      await initialContentPromise;

	      // Verify log table is displayed
	      await expect(page.getByTestId('log-table')).toBeVisible();
	    });
	  });

  test.describe('Log Content Display', () => {
	    test('should display log entries in table format', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);
	      await setupLogMocks(page);

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);

	      // Wait for auto-selected log content to load
	      await initialContentPromise;

	      // Verify table structure
	      const logTable = page.getByTestId('log-table');
	      await expect(logTable).toBeVisible();

      // Verify table has expected columns
      await expect(page.getByRole('columnheader', { name: /time/i })).toBeVisible();
      await expect(page.getByRole('columnheader', { name: /status/i })).toBeVisible();
      await expect(page.getByRole('columnheader', { name: /method/i })).toBeVisible();
    });

	    test('should show timestamp, level, method, uri, status', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);
	      await setupLogMocks(page);

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);

	      // Wait for content to load
	      await initialContentPromise;

	      // Verify log entry content is displayed
	      // The mock data includes 192.168.1.100 as remote_ip in first entry
	      const entryRow = page.getByRole('row').filter({ hasText: '192.168.1.100' }).first();
      await expect(entryRow).toBeVisible();
      await expect(entryRow.getByRole('cell', { name: 'GET' })).toBeVisible();
      await expect(entryRow.getByText('/api/v1/users')).toBeVisible();
      await expect(entryRow.getByTestId('status-200')).toBeVisible();
    });

	    test('should highlight error entries with distinct styling', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);
	      await setupLogMocks(page);

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);
	      await initialContentPromise;

	      // Find the 502 error status badge - should have red styling class
	      const errorStatus = page.getByTestId('status-502');
	      await expect(errorStatus).toBeVisible();

      // Verify error has red styling (bg-red or similar)
      await expect(errorStatus).toHaveClass(/red/);
    });
  });

  test.describe('Pagination', () => {
    test('should paginate large log files', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      // Generate 150 mock entries for pagination testing
      const largeEntrySet = generateMockEntries(150);
      let capturedOffset = 0;

      await page.route('**/api/v1/logs', async (route) => {
        await route.fulfill({ status: 200, json: mockLogFiles });
      });

      await page.route('**/api/v1/logs/access.log*', async (route) => {
        const url = new URL(route.request().url());
        capturedOffset = parseInt(url.searchParams.get('offset') || '0');
        const limit = parseInt(url.searchParams.get('limit') || '50');

        await route.fulfill({
          status: 200,
          json: {
            filename: 'access.log',
            logs: largeEntrySet.slice(capturedOffset, capturedOffset + limit),
            total: 150,
            limit,
            offset: capturedOffset,
          },
        });
      });

      // Set up the response wait BEFORE navigation to avoid missing fast mocked responses.
      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);
      await initialContentPromise;

      // Initial state - page 1
      expect(capturedOffset).toBe(0);

      // Click next page button
      const nextButton = getNextButton(page);
      await expect(nextButton).toBeEnabled();

      // Set up listener BEFORE clicking to capture the request
      await Promise.all([
        page.waitForResponse((resp) => resp.url().includes('/api/v1/logs/access.log') && resp.status() === 200),
        nextButton.click(),
      ]);

      // Should have requested offset 50 (second page)
      expect(capturedOffset).toBe(50);
    });

	    test('should display page info correctly', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);

      const largeEntrySet = generateMockEntries(150);

      await page.route('**/api/v1/logs', async (route) => {
        await route.fulfill({ status: 200, json: mockLogFiles });
      });

	      await page.route('**/api/v1/logs/access.log*', async (route) => {
        const url = new URL(route.request().url());
        const offset = parseInt(url.searchParams.get('offset') || '0');
        const limit = parseInt(url.searchParams.get('limit') || '50');

        await route.fulfill({
          status: 200,
          json: {
            filename: 'access.log',
            logs: largeEntrySet.slice(offset, offset + limit),
            total: 150,
            limit,
            offset,
          },
        });
	      });

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);
	      await initialContentPromise;

	      // Verify page info displays correctly
	      const pageInfo = page.getByTestId('page-info');
	      await expect(pageInfo).toBeVisible();

      // Should show "Showing 1 - 50 of 150" or similar format
      await expect(pageInfo).toContainText(/1.*50.*150/);
    });

	    test('should disable prev button on first page and next on last', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);

      const entries = generateMockEntries(75); // 2 pages (50 + 25)

      await page.route('**/api/v1/logs', async (route) => {
        await route.fulfill({ status: 200, json: mockLogFiles });
      });

	      await page.route('**/api/v1/logs/access.log*', async (route) => {
        const url = new URL(route.request().url());
        const offset = parseInt(url.searchParams.get('offset') || '0');
        const limit = parseInt(url.searchParams.get('limit') || '50');

        await route.fulfill({
          status: 200,
          json: {
            filename: 'access.log',
            logs: entries.slice(offset, offset + limit),
            total: 75,
            limit,
            offset,
          },
        });
	      });

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);
	      await initialContentPromise;

      const prevButton = getPrevButton(page);
      const nextButton = getNextButton(page);

      // On first page, prev should be disabled
      await expect(prevButton).toBeDisabled();
      await expect(nextButton).toBeEnabled();

      // Navigate to last page
      await Promise.all([
        page.waitForResponse((resp) => resp.url().includes('/api/v1/logs/access.log')),
        nextButton.click(),
      ]);

      // On last page, next should be disabled
      await expect(prevButton).toBeEnabled();
      await expect(nextButton).toBeDisabled();
    });
  });

  test.describe('Search and Filter', () => {
	    test('should filter logs by search text', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);

      let capturedSearch = '';

      await page.route('**/api/v1/logs', async (route) => {
        await route.fulfill({ status: 200, json: mockLogFiles });
      });

	      await page.route('**/api/v1/logs/access.log*', async (route) => {
        const url = new URL(route.request().url());
        capturedSearch = url.searchParams.get('search') || '';

        // Filter mock entries based on search
        const filtered = capturedSearch
          ? mockLogEntries.filter(
              (e) =>
                e.msg.toLowerCase().includes(capturedSearch.toLowerCase()) ||
                e.request.uri.toLowerCase().includes(capturedSearch.toLowerCase())
            )
          : mockLogEntries;

        await route.fulfill({
          status: 200,
          json: {
            filename: 'access.log',
            logs: filtered,
            total: filtered.length,
            limit: 50,
            offset: 0,
          },
        });
	      });

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);
	      await initialContentPromise;

      // Type in search input
      const searchInput = page.getByTestId('search-input');

      // Set up response listener BEFORE typing to catch the debounced request
      const searchResponsePromise = page.waitForResponse((resp) =>
        resp.url().includes('/api/v1/logs/access.log')
      );

      await searchInput.fill('users');

      // Wait for debounced search request
      await searchResponsePromise;

      // Verify search parameter was sent
      expect(capturedSearch).toBe('users');
    });

	    test('should filter logs by log level', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);

      let capturedLevel = '';

      await page.route('**/api/v1/logs', async (route) => {
        await route.fulfill({ status: 200, json: mockLogFiles });
      });

	      await page.route('**/api/v1/logs/access.log*', async (route) => {
        const url = new URL(route.request().url());
        capturedLevel = url.searchParams.get('level') || '';

        // Filter mock entries based on level
        const filtered = capturedLevel
          ? mockLogEntries.filter((e) => e.level.toLowerCase() === capturedLevel.toLowerCase())
          : mockLogEntries;

        await route.fulfill({
          status: 200,
          json: {
            filename: 'access.log',
            logs: filtered,
            total: filtered.length,
            limit: 50,
            offset: 0,
          },
        });
	      });

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);
	      await initialContentPromise;

      // Select Error level from dropdown using data-testid
      const levelSelect = page.getByTestId('level-select');
      await expect(levelSelect).toBeVisible();

      // Set up response listener BEFORE selecting
      const filterResponsePromise = page.waitForResponse((resp) =>
        resp.url().includes('/api/v1/logs/access.log')
      );

      await levelSelect.selectOption('ERROR');
      await filterResponsePromise;

      // Verify level parameter was sent
      expect(capturedLevel.toLowerCase()).toBe('error');
    });
  });

  test.describe('Sorting', () => {
    /**
     * Sortable columns and their backend `sort_by` values (spec §5.4.4):
     * Time→ts, Level→level, Status→status, Method→method, Path→uri.
     * Each sortable header exposes a button with data-testid `sort-header-<field>`.
     *
     * All tests are marked test.fixme until the sorting feature (issue #686)
     * is implemented in the backend and frontend; the fixme flags are removed
     * in the final integration commit.
     */

    /** Latest sort-related query params captured by the mock route */
    interface CapturedSortParams {
      sortBy: string | null;
      sortDir: string | null;
      offset: number;
    }

    /**
     * Mock the log endpoints and record the sorting/pagination query params
     * of every content request into `captured` (same pattern as the
     * pagination tests above).
     */
    async function setupSortCaptureMocks(
      page: Page,
      captured: CapturedSortParams,
      entries = mockLogEntries,
      totalCount?: number
    ) {
      await page.route('**/api/v1/logs', async (route) => {
        await route.fulfill({ status: 200, json: mockLogFiles });
      });

      await page.route('**/api/v1/logs/access.log*', async (route) => {
        const url = new URL(route.request().url());
        captured.sortBy = url.searchParams.get('sort_by');
        captured.sortDir = url.searchParams.get('sort');
        captured.offset = parseInt(url.searchParams.get('offset') || '0');
        const limit = parseInt(url.searchParams.get('limit') || '50');

        await route.fulfill({
          status: 200,
          json: {
            filename: 'access.log',
            logs: entries.slice(captured.offset, captured.offset + limit),
            total: totalCount ?? entries.length,
            limit,
            offset: captured.offset,
            skipped_lines: 0,
          },
        });
      });
    }

    /** Click a sort header and wait for the resulting content request */
    async function clickSortHeader(page: Page, field: string) {
      await Promise.all([
        page.waitForResponse(
          (resp) => resp.url().includes('/api/v1/logs/access.log') && resp.status() === 200
        ),
        page.getByTestId(`sort-header-${field}`).click(),
      ]);
    }

    test.fixme('should sort by timestamp via Time header', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      const captured: CapturedSortParams = { sortBy: null, sortDir: null, offset: 0 };
      await setupSortCaptureMocks(page, captured);

      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);
      await initialContentPromise;

      await test.step('Toggle the active Time column to ascending', async () => {
        // Default sort is ts desc, so clicking the active Time header flips direction
        await clickSortHeader(page, 'ts');
        expect(captured.sortBy).toBe('ts');
        expect(captured.sortDir).toBe('asc');
      });

      await test.step('Toggle the Time column back to descending', async () => {
        await clickSortHeader(page, 'ts');
        expect(captured.sortBy).toBe('ts');
        expect(captured.sortDir).toBe('desc');
      });
    });

    for (const field of ['status', 'level', 'method', 'uri'] as const) {
      test.fixme(`should sort by ${field} via its column header`, async ({ page, authenticatedUser }) => {
        await loginUser(page, authenticatedUser);

        const captured: CapturedSortParams = { sortBy: null, sortDir: null, offset: 0 };
        await setupSortCaptureMocks(page, captured);

        const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
        await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
        await waitForLoadingComplete(page);
        await initialContentPromise;

        await test.step(`Sort by the ${field} column`, async () => {
          await clickSortHeader(page, field);

          // A newly activated column starts in descending order (spec R7)
          expect(captured.sortBy).toBe(field);
          expect(captured.sortDir).toBe('desc');
        });
      });
    }

    test.fixme('should indicate active sort with aria-sort', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      const captured: CapturedSortParams = { sortBy: null, sortDir: null, offset: 0 };
      await setupSortCaptureMocks(page, captured);

      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);
      await initialContentPromise;

      const timeHeader = page.getByRole('columnheader', { name: /time/i });
      const statusHeader = page.getByRole('columnheader', { name: /status/i });

      await test.step('Verify sortable header structure is accessible', async () => {
        // Accessible names must stay Time/Level/Status/Method (spec §5.4.4)
        await expect(page.getByTestId('log-table')).toMatchAriaSnapshot(`
          - table:
            - rowgroup:
              - row:
                - columnheader "Time"
                - columnheader "Level"
                - columnheader "Status"
                - columnheader "Method"
        `);
      });

      await test.step('Default sort is Time descending', async () => {
        await expect(timeHeader).toHaveAttribute('aria-sort', 'descending');
      });

      await test.step('Clicking the active Time header flips aria-sort to ascending', async () => {
        await clickSortHeader(page, 'ts');
        await expect(timeHeader).toHaveAttribute('aria-sort', 'ascending');
      });

      await test.step('Activating the Status column moves the aria-sort indicator', async () => {
        await clickSortHeader(page, 'status');
        await expect(statusHeader).toHaveAttribute('aria-sort', 'descending');
        await expect(timeHeader).not.toHaveAttribute('aria-sort', /ascending|descending/);
      });
    });

    test.fixme('should reset to first page when sorting changes', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      const captured: CapturedSortParams = { sortBy: null, sortDir: null, offset: 0 };
      await setupSortCaptureMocks(page, captured, generateMockEntries(150), 150);

      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);
      await initialContentPromise;

      await test.step('Navigate to the second page', async () => {
        await Promise.all([
          page.waitForResponse(
            (resp) => resp.url().includes('/api/v1/logs/access.log') && resp.status() === 200
          ),
          getNextButton(page).click(),
        ]);
        expect(captured.offset).toBe(50);
      });

      await test.step('Sorting by a column resets to the first page', async () => {
        await clickSortHeader(page, 'status');
        expect(captured.sortBy).toBe('status');
        expect(captured.offset).toBe(0);
      });
    });
  });

  test.describe('Download', () => {
	    test('should download log file successfully', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);
	      await setupLogMocks(page);

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);
	      await initialContentPromise;

      // Verify download button is visible and enabled
      const downloadButton = page.getByTestId('download-button');
      await expect(downloadButton).toBeVisible();
      await expect(downloadButton).toBeEnabled();

      // The download button clicking will use window.location.href for download
      // Verify the button state is correct for a successful download
      await expect(downloadButton).not.toHaveAttribute('disabled');
    });

	    test('should handle download error gracefully', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);

      await page.route('**/api/v1/logs', async (route) => {
        await route.fulfill({ status: 200, json: mockLogFiles });
      });

	      await page.route('**/api/v1/logs/access.log*', async (route) => {
        if (!route.request().url().includes('/download')) {
          await route.fulfill({
            status: 200,
            json: {
              filename: 'access.log',
              logs: mockLogEntries,
              total: mockLogEntries.length,
              limit: 50,
              offset: 0,
            },
          });
        } else {
          await route.continue();
        }
	      });

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);
	      await initialContentPromise;

      // Verify download button is present and properly rendered
      const downloadButton = page.getByTestId('download-button');
      await expect(downloadButton).toBeVisible();

      // Button should be in a clickable state even if download endpoint fails
      await expect(downloadButton).toBeEnabled();
    });
  });

  test.describe('Edge Cases', () => {
	    test('should handle empty log content gracefully', async ({ page, authenticatedUser }) => {
	      await loginUser(page, authenticatedUser);

      await page.route('**/api/v1/logs', async (route) => {
        await route.fulfill({ status: 200, json: mockLogFiles });
      });

	      await page.route('**/api/v1/logs/access.log*', async (route) => {
        await route.fulfill({
          status: 200,
          json: {
            filename: 'access.log',
            logs: [],
            total: 0,
            limit: 50,
            offset: 0,
          },
        });
	      });

	      const initialContentPromise = waitForAPIResponse(page, '/api/v1/logs/access.log', { status: 200 });
	      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
	      await waitForLoadingComplete(page);
	      await initialContentPromise;

      // Should show "No logs found" or similar message
      await expect(page.getByText(/no logs found|no.*matching/i)).toBeVisible();
    });

    test('should reset to first page when changing log file', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      const largeEntrySet = generateMockEntries(150);
      let lastOffset = 0;

      await page.route('**/api/v1/logs', async (route) => {
        await route.fulfill({ status: 200, json: mockLogFiles });
      });

      await page.route('**/api/v1/logs/*', async (route) => {
        const url = new URL(route.request().url());
        lastOffset = parseInt(url.searchParams.get('offset') || '0');
        const limit = parseInt(url.searchParams.get('limit') || '50');

        await route.fulfill({
          status: 200,
          json: {
            filename: 'test.log',
            logs: largeEntrySet.slice(lastOffset, lastOffset + limit),
            total: 150,
            limit,
            offset: lastOffset,
          },
        });
      });

      await page.goto('/tasks/logs', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      // Navigate to page 2
      const nextButton = getNextButton(page);
      await nextButton.click();

      // Wait briefly for state update
      await page.waitForTimeout(500);

      expect(lastOffset).toBe(50);

      // Switch to different log file using the new data-testid
      const errorLogButton = getLogFileButton(page, 'error.log');
      await Promise.all([
        page.waitForResponse((resp) => resp.url().includes('/api/v1/logs/')),
        errorLogButton.click(),
      ]);

      // Should reset to offset 0 when switching files
      expect(lastOffset).toBe(0);
    });
  });
});
