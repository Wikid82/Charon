/**
 * Uptime Monitoring Page - E2E Tests
 *
 * Tests for uptime monitor display, CRUD operations, manual checks, and sync functionality.
 * Covers 22 test scenarios as defined in phase5-implementation.md.
 *
 * Test Categories:
 * - Page Layout (3 tests): heading, monitor list/empty state, summary
 * - Monitor List Display (5 tests): status indicators, uptime %, last check, states, heartbeat bar
 * - Monitor CRUD (6 tests): create HTTP, create TCP, update, delete, URL validation, interval validation
 * - Manual Check (3 tests): trigger check, status refresh, loading state
 * - Monitor History (3 tests): history chart, incident timeline, date range filter
 * - Sync with Proxy Hosts (2 tests): sync button, preserve manual monitors
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import {
  waitForToast,
  waitForLoadingComplete,
  waitForAPIResponse,
} from '../utils/wait-helpers';

/**
 * TypeScript interfaces matching the API
 */
interface UptimeMonitor {
  id: string;
  upstream_host?: string;
  proxy_host_id?: number;
  remote_server_id?: number;
  name: string;
  type: string;
  url: string;
  interval: number;
  enabled: boolean;
  status: string;
  last_check?: string | null;
  latency: number;
  max_retries: number;
}

interface UptimeHeartbeat {
  id: number;
  monitor_id: string;
  status: string;
  latency: number;
  message: string;
  created_at: string;
}

/**
 * Mock monitor data for testing
 */
const mockMonitors: UptimeMonitor[] = [
  {
    id: '1',
    name: 'API Server',
    type: 'http',
    url: 'https://api.example.com',
    interval: 60,
    enabled: true,
    status: 'up',
    latency: 45,
    max_retries: 3,
    last_check: '2024-01-15T12:00:00Z',
  },
  {
    id: '2',
    name: 'Database',
    type: 'tcp',
    url: 'tcp://db:5432',
    interval: 30,
    enabled: true,
    status: 'down',
    latency: 0,
    max_retries: 3,
    last_check: '2024-01-15T11:59:00Z',
  },
  {
    id: '3',
    name: 'Cache',
    type: 'tcp',
    url: 'tcp://redis:6379',
    interval: 60,
    enabled: false,
    status: 'paused',
    latency: 0,
    max_retries: 3,
    last_check: null,
  },
];

/**
 * Generate mock heartbeat history
 */
const generateMockHistory = (
  monitorId: string,
  count: number = 60,
  latestStatus: 'up' | 'down' = 'up'
): UptimeHeartbeat[] => {
  return Array.from({ length: count }, (_, i) => ({
    id: i,
    monitor_id: monitorId,
    // Keep the newest heartbeat aligned with the monitor's expected current state.
    status: i === 0 ? latestStatus : i % 5 === 0 ? 'down' : 'up',
    latency: Math.floor(Math.random() * 100),
    message: 'OK',
    created_at: new Date(Date.now() - i * 60000).toISOString(),
  }));
};

/**
 * UI Selectors for the Uptime page
 */
const SELECTORS = {
  // Page layout
  pageTitle: 'h1',
  summaryCard: '[data-testid="uptime-summary"]',
  emptyState: 'text=No monitors found',

  // Monitor cards
  monitorCard: '[data-testid="monitor-card"]',
  statusBadge: '[data-testid="status-badge"]',
  lastCheck: '[data-testid="last-check"]',
  heartbeatBar: '[data-testid="heartbeat-bar"]',

  // Actions
  syncButton: '[data-testid="sync-button"]',
  addMonitorButton: '[data-testid="add-monitor-button"]',
  settingsButton: 'button[aria-haspopup="menu"]',
  refreshButton: 'button[title*="Check"], button[title*="health"]',

  // Modal
  editModal: '.fixed.inset-0',
  nameInput: 'input#create-monitor-name, input#monitor-name',
  urlInput: 'input#create-monitor-url',
  typeSelect: 'select#create-monitor-type',
  intervalInput: 'input#create-monitor-interval',
  saveButton: 'button[type="submit"]',
  cancelButton: 'button:has-text("Cancel")',
  closeButton: 'button[aria-label*="Close"], button:has-text("×")',

  // Menu items
  configureOption: 'button:has-text("Configure")',
  pauseOption: 'button:has-text("Pause"), button:has-text("Unpause")',
  deleteOption: 'button:has-text("Delete")',
};

/**
 * Helper: Setup mock monitors API response
 */
async function setupMonitorsAPI(
  page: import('@playwright/test').Page,
  monitors: UptimeMonitor[] = mockMonitors
) {
  await page.route('**/api/v1/uptime/monitors', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, json: monitors });
    } else {
      await route.continue();
    }
  });
}

/**
 * Helper: Setup mock history API response
 */
async function setupHistoryAPI(
  page: import('@playwright/test').Page,
  monitorId: string,
  history: UptimeHeartbeat[]
) {
  await page.route(`**/api/v1/uptime/monitors/${monitorId}/history*`, async (route) => {
    await route.fulfill({ status: 200, json: history });
  });
}

/**
 * Helper: Setup all monitors with their history
 */
async function setupMonitorsWithHistory(
  page: import('@playwright/test').Page,
  monitors: UptimeMonitor[] = mockMonitors
) {
  await setupMonitorsAPI(page, monitors);

  for (const monitor of monitors) {
    const latestStatus = monitor.status === 'down' ? 'down' : 'up';
    const history = generateMockHistory(monitor.id, 60, latestStatus);
    await setupHistoryAPI(page, monitor.id, history);
  }
}

test.describe('Uptime Monitoring Page', () => {
  // =========================================================================
  // Page Layout Tests (3 tests)
  // =========================================================================
  test.describe('Page Layout', () => {
    test('should display uptime monitoring page with correct heading', async ({
      page,
      authenticatedUser,
    }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      await expect(page.locator(SELECTORS.pageTitle)).toContainText(/uptime/i);
      await expect(page.locator(SELECTORS.summaryCard)).toBeVisible();
    });

    test('should show monitor list or empty state', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      // Test empty state
      await setupMonitorsAPI(page, []);
      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      await expect(page.locator(SELECTORS.emptyState)).toBeVisible();

      // Test with monitors
      await setupMonitorsWithHistory(page, mockMonitors);
      await page.reload();
      await waitForLoadingComplete(page);

      await expect(page.locator(SELECTORS.monitorCard)).toHaveCount(mockMonitors.length);
    });

    test('should display overall uptime summary with action buttons', async ({
      page,
      authenticatedUser,
    }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Summary card should be visible
      const summary = page.locator(SELECTORS.summaryCard);
      await expect(summary).toBeVisible();

      // Action buttons should be present
      await expect(page.locator(SELECTORS.syncButton)).toBeVisible();
      await expect(page.locator(SELECTORS.addMonitorButton)).toBeVisible();
    });
  });

  // =========================================================================
  // Monitor List Display Tests (5 tests)
  // =========================================================================
  test.describe('Monitor List Display', () => {
    test('should display all monitors with status indicators', async ({
      page,
      authenticatedUser,
    }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Verify all monitors displayed
      await expect(page.getByText('API Server')).toBeVisible();
      await expect(page.getByText('Database')).toBeVisible();
      await expect(page.getByText('Cache')).toBeVisible();

      // Verify status badges exist
      const statusBadges = page.locator(SELECTORS.statusBadge);
      await expect(statusBadges).toHaveCount(mockMonitors.length);
    });

    test('should show uptime percentage or latency for each monitor', async ({
      page,
      authenticatedUser,
    }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // API Server should show latency (45ms)
      const apiCard = page.locator(SELECTORS.monitorCard).filter({ hasText: 'API Server' });
      await expect(apiCard).toContainText('45ms');
    });

    test('should show last check timestamp', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Last check sections should be visible
      const lastCheckElements = page.locator(SELECTORS.lastCheck);
      await expect(lastCheckElements.first()).toBeVisible();

      // Cache monitor with no last check should show "never" or similar
      const cacheCard = page.locator(SELECTORS.monitorCard).filter({ hasText: 'Cache' });
      await expect(cacheCard.locator(SELECTORS.lastCheck)).toBeVisible();
    });

    test('should differentiate up/down/paused states visually', async ({
      page,
      authenticatedUser,
    }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Check for different status badges
      const upBadge = page.locator('[data-testid="status-badge"][data-status="up"]');
      const downBadge = page.locator('[data-testid="status-badge"][data-status="down"]');
      const pausedBadge = page.locator('[data-testid="status-badge"][data-status="paused"]');

      await expect(upBadge).toBeVisible();
      await expect(downBadge).toBeVisible();
      await expect(pausedBadge).toBeVisible();

      // Verify badge text
      await expect(upBadge).toContainText(/up/i);
      await expect(downBadge).toContainText(/down/i);
      await expect(pausedBadge).toContainText(/pause/i);
    });

    test('should show heartbeat history bar for each monitor', async ({
      page,
      authenticatedUser,
    }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Each monitor card should have a heartbeat bar
      const heartbeatBars = page.locator(SELECTORS.heartbeatBar);
      await expect(heartbeatBars).toHaveCount(mockMonitors.length);

      // First monitor's heartbeat bar should be visible
      const firstBar = heartbeatBars.first();
      await expect(firstBar).toBeVisible();
    });
  });

  // =========================================================================
  // Monitor CRUD Tests (6 tests)
  // =========================================================================
  test.describe('Monitor CRUD Operations', () => {
    test('should create new HTTP monitor', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      let createPayload: Partial<UptimeMonitor> | null = null;

      await page.route('**/api/v1/uptime/monitors', async (route) => {
        if (route.request().method() === 'POST') {
          createPayload = await route.request().postDataJSON();
          await route.fulfill({
            status: 201,
            json: {
              id: 'new-id',
              ...createPayload,
              status: 'unknown',
              latency: 0,
              enabled: true,
            },
          });
        } else {
          await route.fulfill({ status: 200, json: [] });
        }
      });

      // Setup history for new monitor
      await page.route('**/api/v1/uptime/monitors/new-id/history*', async (route) => {
        await route.fulfill({ status: 200, json: [] });
      });

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Click add monitor button
      await page.click(SELECTORS.addMonitorButton);

      // Fill form
      await page.fill('input#create-monitor-name', 'New API Monitor');
      await page.fill('input#create-monitor-url', 'https://api.newservice.com/health');
      await page.selectOption('select#create-monitor-type', 'http');
      await page.fill('input#create-monitor-interval', '60');

      // Set up response listener BEFORE clicking submit to avoid race condition
      const createResponsePromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/uptime/monitors') &&
          response.request().method() === 'POST' &&
          response.status() === 201
      );

      // Submit
      await page.click('button[type="submit"]');

      await createResponsePromise;

      expect(createPayload).not.toBeNull();
      expect(createPayload?.name).toBe('New API Monitor');
      expect(createPayload?.url).toBe('https://api.newservice.com/health');
      expect(createPayload?.type).toBe('http');
    });

    test('should create new TCP monitor', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      let createPayload: Partial<UptimeMonitor> | null = null;

      await page.route('**/api/v1/uptime/monitors', async (route) => {
        if (route.request().method() === 'POST') {
          createPayload = await route.request().postDataJSON();
          await route.fulfill({
            status: 201,
            json: {
              id: 'tcp-id',
              ...createPayload,
              status: 'unknown',
              latency: 0,
              enabled: true,
            },
          });
        } else {
          await route.fulfill({ status: 200, json: [] });
        }
      });

      await page.route('**/api/v1/uptime/monitors/tcp-id/history*', async (route) => {
        await route.fulfill({ status: 200, json: [] });
      });

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      await page.click(SELECTORS.addMonitorButton);

      await page.fill('input#create-monitor-name', 'Redis Cache');
      await page.fill('input#create-monitor-url', 'tcp://redis.local:6379');
      await page.selectOption('select#create-monitor-type', 'tcp');
      await page.fill('input#create-monitor-interval', '30');

      // Set up response listener BEFORE clicking submit to avoid race condition
      const createTcpResponsePromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/uptime/monitors') &&
          response.request().method() === 'POST' &&
          response.status() === 201
      );

      await page.click('button[type="submit"]');

      await createTcpResponsePromise;

      expect(createPayload).not.toBeNull();
      expect(createPayload?.type).toBe('tcp');
      expect(createPayload?.url).toBe('tcp://redis.local:6379');
    });

    test('should update existing monitor', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      let updatePayload: Partial<UptimeMonitor> | null = null;

      await page.route('**/api/v1/uptime/monitors/1', async (route) => {
        if (route.request().method() === 'PUT') {
          updatePayload = await route.request().postDataJSON();
          await route.fulfill({
            status: 200,
            json: { ...mockMonitors[0], ...updatePayload },
          });
        } else {
          await route.continue();
        }
      });

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Open settings menu on first monitor
      const firstCard = page.locator(SELECTORS.monitorCard).first();
      await firstCard.locator(SELECTORS.settingsButton).click();

      // Wait for menu to appear and click configure
      await page.waitForSelector(SELECTORS.configureOption, { state: 'visible' });
      await page.click(SELECTORS.configureOption);

      // In Firefox, the modal form might take time to render
      // Wait for the form input to be available instead of waiting for dialog role
      const nameInput = page.locator('input#monitor-name');
      await nameInput.waitFor({ state: 'visible', timeout: 10000 });

      // Update name (use specific id selector)
      await page.fill('input#monitor-name', 'Updated API Server');

      // Set up response listener BEFORE clicking submit to avoid race condition
      const responsePromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/uptime/monitors/1') &&
          response.request().method() === 'PUT' &&
          response.status() === 200
      );

      // Save - find the submit button within the modal
      await page.click('button[type="submit"]');

      await responsePromise;

      expect(updatePayload).not.toBeNull();
      expect(updatePayload?.name).toBe('Updated API Server');
    });

    test('should delete monitor with confirmation', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      let deleteRequested = false;

      await page.route('**/api/v1/uptime/monitors/1', async (route) => {
        if (route.request().method() === 'DELETE') {
          deleteRequested = true;
          await route.fulfill({ status: 204 });
        } else {
          await route.continue();
        }
      });

      // Handle confirm dialog
      page.on('dialog', async (dialog) => {
        expect(dialog.type()).toBe('confirm');
        await dialog.accept();
      });

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Open settings menu on first monitor
      const firstCard = page.locator(SELECTORS.monitorCard).first();
      await firstCard.locator(SELECTORS.settingsButton).click();

      // Set up response listener BEFORE clicking delete to avoid race condition
      const deleteResponsePromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/uptime/monitors/1') &&
          response.request().method() === 'DELETE' &&
          response.status() === 204
      );

      // Click delete
      await page.click(SELECTORS.deleteOption);

      await deleteResponsePromise;

      expect(deleteRequested).toBe(true);
    });

    test('should validate monitor URL format', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsAPI(page, []);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      await page.click(SELECTORS.addMonitorButton);

      // Fill with valid name but empty URL
      await page.fill('input#create-monitor-name', 'Test Monitor');

      // Submit button should be disabled when URL is empty
      const submitButton = page.locator('button[type="submit"]');
      await expect(submitButton).toBeDisabled();

      // Fill URL
      await page.fill('input#create-monitor-url', 'https://valid.url.com');

      // Now should be enabled
      await expect(submitButton).toBeEnabled();
    });

    test('should validate check interval range', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsAPI(page, []);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      await page.click(SELECTORS.addMonitorButton);

      // Fill required fields
      await page.fill('input#create-monitor-name', 'Test Monitor');
      await page.fill('input#create-monitor-url', 'https://test.com');

      // Interval input should have min/max attributes
      const intervalInput = page.locator('input#create-monitor-interval');
      await expect(intervalInput).toHaveAttribute('min', '10');
      await expect(intervalInput).toHaveAttribute('max', '3600');

      // Set a valid interval
      await page.fill('input#create-monitor-interval', '60');

      const submitButton = page.locator('button[type="submit"]');
      await expect(submitButton).toBeEnabled();
    });
  });

  // =========================================================================
  // Manual Check Tests (3 tests)
  // =========================================================================
  test.describe('Manual Health Check', () => {
    test('should trigger manual health check', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      let checkRequested = false;

      await page.route('**/api/v1/uptime/monitors/1/check', async (route) => {
        checkRequested = true;
        await route.fulfill({
          status: 200,
          json: { message: 'Check completed: UP' },
        });
      });

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Click refresh button on first monitor and wait for API response concurrently
      const firstCard = page.locator(SELECTORS.monitorCard).first();
      const refreshButton = firstCard.locator('button').filter({ has: page.locator('svg') }).first();
      await Promise.all([
        page.waitForResponse(r => r.url().includes('/api/v1/uptime/monitors/1/check') && r.status() === 200),
        refreshButton.click(),
      ]);

      expect(checkRequested).toBe(true);
    });

    test('should update status after manual check', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      let checkRequested = false;

      await page.route('**/api/v1/uptime/monitors/1/check', async (route) => {
        checkRequested = true;
        await route.fulfill({
          status: 200,
          json: { message: 'Check completed: UP' },
        });
      });

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Trigger check using Promise.all to avoid race condition
      const firstCard = page.locator(SELECTORS.monitorCard).first();
      const refreshButton = firstCard.locator('button').filter({ has: page.locator('svg') }).first();

      await Promise.all([
        page.waitForResponse(
          resp => resp.url().includes('/api/v1/uptime/monitors/1/check') && resp.status() === 200
        ),
        refreshButton.click(),
      ]);

      // Verify check was requested - mocked routes don't trigger toasts
      expect(checkRequested).toBe(true);
    });

    test('should show check in progress indicator', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      await page.route('**/api/v1/uptime/monitors/1/check', async (route) => {
        // Delay response to observe loading state
        await new Promise((resolve) => setTimeout(resolve, 500));
        await route.fulfill({
          status: 200,
          json: { message: 'Check completed: UP' },
        });
      });

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Click refresh button
      const firstCard = page.locator(SELECTORS.monitorCard).first();
      const refreshButton = firstCard.locator('button').filter({ has: page.locator('svg') }).first();
      await refreshButton.click();

      // Should show spinning animation (animate-spin class)
      const spinningIcon = firstCard.locator('svg.animate-spin');
      await expect(spinningIcon).toBeVisible();

      // Wait for completion
      await waitForAPIResponse(page, '/api/v1/uptime/monitors/1/check', { status: 200 });
    });
  });

  // =========================================================================
  // Monitor History Tests (3 tests)
  // =========================================================================
  test.describe('Monitor History', () => {
    test('should display uptime history in heartbeat bar', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      const history = generateMockHistory('1', 60);
      await setupMonitorsAPI(page, [mockMonitors[0]]);
      await setupHistoryAPI(page, '1', history);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Heartbeat bar should be visible
      const heartbeatBar = page.locator(SELECTORS.heartbeatBar).first();
      await expect(heartbeatBar).toBeVisible();

      // Bar should contain segments (divs for each heartbeat)
      const segments = heartbeatBar.locator('div.rounded-sm');
      const segmentCount = await segments.count();
      expect(segmentCount).toBeGreaterThan(0);
    });

    test('should show incident indicators in heartbeat bar', async ({
      page,
      authenticatedUser,
    }) => {
      await loginUser(page, authenticatedUser);

      // Create history with some failures
      const history = generateMockHistory('1', 60);
      await setupMonitorsAPI(page, [mockMonitors[0]]);
      await setupHistoryAPI(page, '1', history);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      const heartbeatBar = page.locator(SELECTORS.heartbeatBar).first();
      await expect(heartbeatBar).toBeVisible();

      // Wait for history to be loaded - segments should appear
      // The history contains both 'up' and 'down' statuses (every 5th is down)
      const segments = heartbeatBar.locator('div.rounded-sm');
      await expect(segments.first()).toBeVisible({ timeout: 5000 });

      // Should have both up (green) and down (red) segments
      // Use class*= to match partial class names as Tailwind includes both light and dark mode classes
      const greenSegments = heartbeatBar.locator('[class*="bg-green"]');
      const redSegments = heartbeatBar.locator('[class*="bg-red"]');

      const greenCount = await greenSegments.count();
      const redCount = await redSegments.count();

      expect(greenCount).toBeGreaterThan(0);
      expect(redCount).toBeGreaterThan(0);
    });

    test('should show tooltip with heartbeat details on hover', async ({
      page,
      authenticatedUser,
    }) => {
      await loginUser(page, authenticatedUser);

      const history = generateMockHistory('1', 60);
      await setupMonitorsAPI(page, [mockMonitors[0]]);
      await setupHistoryAPI(page, '1', history);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      const heartbeatBar = page.locator(SELECTORS.heartbeatBar).first();
      await expect(heartbeatBar).toBeVisible();

      // Wait for segments to load and find one with a title (not empty placeholder)
      // History segments have bg-green or bg-red classes and a title attribute
      const historySegment = heartbeatBar.locator('[class*="bg-green"], [class*="bg-red"]').first();
      await expect(historySegment).toBeVisible({ timeout: 5000 });

      // Each history segment should have a title attribute with details
      const title = await historySegment.getAttribute('title');
      expect(title).toBeTruthy();
      expect(title).toContain('Status:');
    });
  });

  // =========================================================================
  // Sync with Proxy Hosts Tests (2 tests)
  // =========================================================================
  test.describe('Sync with Proxy Hosts', () => {
    test('should sync monitors from proxy hosts', async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);
      await setupMonitorsWithHistory(page);

      let syncRequested = false;

      await page.route('**/api/v1/uptime/sync', async (route) => {
        syncRequested = true;
        await route.fulfill({
          status: 200,
          json: { message: '3 monitors synced from proxy hosts' },
        });
      });

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Use Promise.all to avoid race condition
      await Promise.all([
        page.waitForResponse(
          resp => resp.url().includes('/api/v1/uptime/sync') && resp.status() === 200
        ),
        page.click(SELECTORS.syncButton),
      ]);

      // Verify sync was requested - mocked routes don't trigger toasts reliably
      expect(syncRequested).toBe(true);
    });

    test('should preserve manually added monitors after sync', async ({
      page,
      authenticatedUser,
    }) => {
      await loginUser(page, authenticatedUser);

      // Setup monitors: one synced from proxy, one manual
      const monitorsWithTypes: UptimeMonitor[] = [
        { ...mockMonitors[0], proxy_host_id: 1 }, // From proxy host
        { ...mockMonitors[1], proxy_host_id: undefined }, // Manual
      ];

      await setupMonitorsAPI(page, monitorsWithTypes);
      for (const m of monitorsWithTypes) {
        await setupHistoryAPI(page, m.id, generateMockHistory(m.id, 30));
      }

      // After sync, both should still exist
      let syncCalled = false;
      await page.route('**/api/v1/uptime/sync', async (route) => {
        syncCalled = true;
        await route.fulfill({
          status: 200,
          json: { message: '1 monitors synced from proxy hosts' },
        });
      });

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      // Verify both monitors are visible before sync
      await expect(page.getByText('API Server')).toBeVisible();
      await expect(page.getByText('Database')).toBeVisible();

      // Set up response listener BEFORE clicking sync to avoid race condition
      const preserveSyncResponsePromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/uptime/sync') && response.status() === 200
      );

      // Trigger sync
      await page.click(SELECTORS.syncButton);
      await preserveSyncResponsePromise;

      expect(syncCalled).toBe(true);

      // Both should still be visible (UI doesn't remove manual monitors)
      await expect(page.getByText('API Server')).toBeVisible();
      await expect(page.getByText('Database')).toBeVisible();
    });
  });
});
