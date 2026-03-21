import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

interface UptimeMonitor {
  id: string;
  name: string;
  type: string;
  url: string;
  interval: number;
  enabled: boolean;
  status: string;
  latency: number;
  max_retries: number;
  last_check?: string | null;
}

const emptyMonitorsRoute = async (page: import('@playwright/test').Page) => {
  await page.route('**/api/v1/uptime/monitors', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, json: [] });
    } else {
      await route.continue();
    }
  });
  await page.route('**/api/v1/uptime/monitors/*/history*', async (route) => {
    await route.fulfill({ status: 200, json: [] });
  });
};

async function openCreateModal(page: import('@playwright/test').Page) {
  await page.click('[data-testid="add-monitor-button"]');
  await expect(page.getByRole('heading', { name: /create monitor/i })).toBeVisible();
}

test.describe('Create Monitor Modal — TCP UX', () => {
  test.beforeEach(async ({ page, authenticatedUser }) => {
    await loginUser(page, authenticatedUser);
    await emptyMonitorsRoute(page);
    await page.goto('/uptime');
    await waitForLoadingComplete(page);
  });

  test('HTTP type shows URL placeholder', async ({ page }) => {
    await openCreateModal(page);

    const urlInput = page.locator('#create-monitor-url');
    await expect(urlInput).toHaveAttribute('placeholder', 'https://example.com');
  });

  test('TCP type shows bare host:port placeholder', async ({ page }) => {
    await openCreateModal(page);

    const typeSelect = page.locator('#create-monitor-type');
    await typeSelect.selectOption('tcp');

    const urlInput = page.locator('#create-monitor-url');
    await expect(urlInput).toHaveAttribute('placeholder', '192.168.1.1:8080');
  });

  test('Type selector precedes URL input in DOM order', async ({ page }) => {
    await openCreateModal(page);

    await expect(page.locator('#create-monitor-type')).toBeVisible();
    await expect(page.locator('#create-monitor-url')).toBeVisible();

    const typeComesBeforeUrl = await page.evaluate(() => {
      const typeEl = document.getElementById('create-monitor-type');
      const urlEl = document.getElementById('create-monitor-url');
      if (!typeEl || !urlEl) return false;
      return !!(typeEl.compareDocumentPosition(urlEl) & Node.DOCUMENT_POSITION_FOLLOWING);
    });

    expect(typeComesBeforeUrl).toBe(true);
  });

  test('Helper text updates dynamically when type changes', async ({ page }) => {
    await openCreateModal(page);

    const helperText = page.locator('#create-monitor-url-helper');

    await expect(helperText).toContainText(/scheme/i);

    await page.locator('#create-monitor-type').selectOption('tcp');

    await expect(helperText).toContainText(/host:port/i);
  });

  test('Inline error appears when tcp:// scheme entered for TCP type', async ({ page }) => {
    await openCreateModal(page);

    await page.locator('#create-monitor-type').selectOption('tcp');
    await page.locator('#create-monitor-url').fill('tcp://192.168.1.1:8080');

    const errorAlert = page.locator('[role="alert"]');
    await expect(errorAlert).toBeVisible();
    await expect(errorAlert).toContainText(/host:port format/i);
  });

  test('Inline error clears when scheme prefix removed', async ({ page }) => {
    await openCreateModal(page);

    await page.locator('#create-monitor-type').selectOption('tcp');
    const urlInput = page.locator('#create-monitor-url');
    await urlInput.fill('tcp://192.168.1.1:8080');

    await expect(page.locator('[role="alert"]')).toBeVisible();

    await urlInput.fill('192.168.1.1:8080');

    await expect(page.locator('[role="alert"]')).not.toBeVisible();
  });

  test('Inline error clears when type changed from TCP to HTTP', async ({ page }) => {
    await openCreateModal(page);

    const typeSelect = page.locator('#create-monitor-type');
    await typeSelect.selectOption('tcp');

    const urlInput = page.locator('#create-monitor-url');
    await urlInput.fill('tcp://192.168.1.1:8080');

    await expect(page.locator('[role="alert"]')).toBeVisible();

    await typeSelect.selectOption('http');

    await expect(page.locator('[role="alert"]')).not.toBeVisible();
  });

  test('Submit with tcp:// prefix is prevented client-side', async ({ page }) => {
    let createCalled = false;

    await page.route('**/api/v1/uptime/monitors', async (route) => {
      if (route.request().method() === 'POST') {
        createCalled = true;
        await route.continue();
      } else {
        await route.fulfill({ status: 200, json: [] });
      }
    });

    await openCreateModal(page);

    await page.locator('#create-monitor-type').selectOption('tcp');
    await page.locator('#create-monitor-name').fill('DB Server');
    await page.locator('#create-monitor-url').fill('tcp://192.168.1.1:8080');

    await page.getByRole('button', { name: /create/i }).click();

    // Inline error confirms client-side validation blocked the submit
    await expect(page.locator('[role="alert"]')).toBeVisible();
    // Modal still open — form was not submitted
    await expect(page.getByRole('heading', { name: /create monitor/i })).toBeVisible();

    expect(createCalled).toBe(false);
  });

  test('TCP monitor created successfully with bare host:port', async ({ page }) => {
    let capturedPayload: Record<string, unknown> | null = null;

    const createdMonitor: UptimeMonitor = {
      id: 'm-test',
      name: 'DB Server',
      type: 'tcp',
      url: '192.168.1.1:5432',
      interval: 60,
      enabled: true,
      status: 'pending',
      latency: 0,
      max_retries: 3,
    };

    await page.route('**/api/v1/uptime/monitors', async (route) => {
      if (route.request().method() === 'POST') {
        capturedPayload = route.request().postDataJSON() as Record<string, unknown>;
        await route.fulfill({ status: 201, json: createdMonitor });
      } else {
        await route.fulfill({ status: 200, json: [] });
      }
    });
    await page.route(`**/api/v1/uptime/monitors/${createdMonitor.id}/history*`, async (route) => {
      await route.fulfill({ status: 200, json: [] });
    });

    await openCreateModal(page);

    await page.locator('#create-monitor-type').selectOption('tcp');
    await page.locator('#create-monitor-name').fill('DB Server');
    await page.locator('#create-monitor-url').fill('192.168.1.1:5432');

    await page.getByRole('button', { name: /create/i }).click();

    await expect(page.getByRole('heading', { name: /create monitor/i })).not.toBeVisible({ timeout: 5000 });

    expect(capturedPayload).not.toBeNull();
    expect(capturedPayload?.url).toBe('192.168.1.1:5432');
    expect(capturedPayload?.type).toBe('tcp');
  });
});
