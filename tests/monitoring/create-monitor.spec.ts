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
  await page.locator('[data-testid="add-monitor-button"]').click();
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

    await expect(page.locator('[role="alert"]')).toBeHidden();
  });

  test('Inline error clears when type changed from TCP to HTTP', async ({ page }) => {
    await openCreateModal(page);

    const typeSelect = page.locator('#create-monitor-type');
    await typeSelect.selectOption('tcp');

    const urlInput = page.locator('#create-monitor-url');
    await urlInput.fill('tcp://192.168.1.1:8080');

    await expect(page.locator('[role="alert"]')).toBeVisible();

    await typeSelect.selectOption('http');

    await expect(page.locator('[role="alert"]')).toBeHidden();
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
    let capturedPayload = null as Record<string, unknown> | null;

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

const DESTINATION_REFUSED_TEXT = 'This destination is not allowed.';
const DESTINATION_REFUSED_CODE = 'destination_refused';

/** Lists monitors through the real API (page.request is not intercepted by page.route mocks). */
async function listMonitorsViaAPI(page: import('@playwright/test').Page): Promise<UptimeMonitor[]> {
  const response = await page.request.get('/api/v1/uptime/monitors');
  expect(response.status(), 'GET /api/v1/uptime/monitors').toBe(200);
  return (await response.json()) as UptimeMonitor[];
}

test.describe('Create Monitor Modal — destination refusals (real backend)', () => {
  const createdMonitorIds: string[] = [];

  test.beforeEach(async ({ page, authenticatedUser }) => {
    await loginUser(page, authenticatedUser);
    await page.goto('/uptime');
    await waitForLoadingComplete(page);
  });

  test.afterEach(async ({ page }) => {
    for (const id of createdMonitorIds.splice(0)) {
      await page.request.delete(`/api/v1/uptime/monitors/${id}`);
    }
  });

  test('TCP monitor to a link-local address is refused with the fixed text and nothing is stored', async ({ page }) => {
    const name = `Refused link-local ${Date.now()}`;

    await test.step('Submit the refused TCP destination', async () => {
      await openCreateModal(page);
      await page.locator('#create-monitor-type').selectOption('tcp');
      await page.locator('#create-monitor-name').fill(name);
      await page.locator('#create-monitor-url').fill('169.254.169.254:80');
      await page.getByRole('button', { name: /create/i }).click();
    });

    await test.step('The error alert shows the fixed text and the modal stays open', async () => {
      await expect(page.getByRole('status').filter({ hasText: DESTINATION_REFUSED_TEXT })).toBeVisible();
      await expect(page.getByRole('heading', { name: /create monitor/i })).toBeVisible();
    });

    await test.step('The monitor is not stored', async () => {
      const monitors = await listMonitorsViaAPI(page);
      expect(monitors.some((monitor) => monitor.name === name)).toBe(false);
    });
  });

  test('HTTP monitor to a reserved Charon port is refused with the same fixed text', async ({ page }) => {
    await openCreateModal(page);
    await page.locator('#create-monitor-name').fill(`Refused reserved ${Date.now()}`);
    await page.locator('#create-monitor-url').fill('http://127.0.0.1:2019/');
    await page.getByRole('button', { name: /create/i }).click();

    await expect(page.getByRole('status').filter({ hasText: DESTINATION_REFUSED_TEXT })).toBeVisible();
    await expect(page.getByRole('heading', { name: /create monitor/i })).toBeVisible();
  });

  test('TCP monitor to an RFC 1918 host:port is created from the modal', async ({ page }) => {
    const name = `LAN database ${Date.now()}`;

    await test.step('Submit a bare host:port on a LAN address', async () => {
      await openCreateModal(page);
      await page.locator('#create-monitor-type').selectOption('tcp');
      await page.locator('#create-monitor-name').fill(name);
      await page.locator('#create-monitor-url').fill('192.168.1.1:5432');
      await page.getByRole('button', { name: /create/i }).click();
      await expect(page.getByRole('heading', { name: /create monitor/i })).not.toBeVisible({ timeout: 10000 });
    });

    await test.step('The stored monitor keeps the bare host:port and is removed by id after the test', async () => {
      const created = (await listMonitorsViaAPI(page)).find((monitor) => monitor.name === name);
      expect(created?.url).toBe('192.168.1.1:5432');
      if (created) createdMonitorIds.push(String(created.id));
    });
  });
});

test.describe('Uptime monitor creation API — destination policy', () => {
  const createdMonitorIds: string[] = [];

  test.beforeEach(async ({ page, authenticatedUser }) => {
    await loginUser(page, authenticatedUser);
  });

  test.afterEach(async ({ page }) => {
    for (const id of createdMonitorIds.splice(0)) {
      await page.request.delete(`/api/v1/uptime/monitors/${id}`);
    }
  });

  test('TCP create to a link-local address and to a reserved port return the same 400 body', async ({ page }) => {
    const linkLocal = await page.request.post('/api/v1/uptime/monitors', {
      data: { name: `API link-local ${Date.now()}`, url: '169.254.169.254:80', type: 'tcp' },
    });
    const reserved = await page.request.post('/api/v1/uptime/monitors', {
      data: { name: `API reserved ${Date.now()}`, url: '127.0.0.1:2019', type: 'tcp' },
    });

    expect(linkLocal.status()).toBe(400);
    expect(reserved.status()).toBe(400);
    const expected = { error: DESTINATION_REFUSED_TEXT, error_code: DESTINATION_REFUSED_CODE };
    expect(await linkLocal.json()).toEqual(expected);
    expect(await reserved.json()).toEqual(expected);
  });

  test('HTTP create to the link-local URL is refused with destination_refused', async ({ page }) => {
    const response = await page.request.post('/api/v1/uptime/monitors', {
      data: { name: `API link-local URL ${Date.now()}`, url: 'http://169.254.169.254/latest/meta-data/', type: 'http' },
    });

    expect(response.status()).toBe(400);
    expect(await response.json()).toEqual({ error: DESTINATION_REFUSED_TEXT, error_code: DESTINATION_REFUSED_CODE });
  });

  test('Invalid monitor input is a 400 with a message, not a server error', async ({ page }) => {
    const response = await page.request.post('/api/v1/uptime/monitors', {
      data: { name: '', url: '', type: 'http' },
    });

    expect(response.status()).toBe(400);
    const body = (await response.json()) as { error?: unknown };
    expect(typeof body.error === 'string' && body.error.length > 0).toBe(true);
  });

  test('TCP create accepts a bare host:port and a tcp:// prefixed host:port (201 for both)', async ({ page }) => {
    const bare = await page.request.post('/api/v1/uptime/monitors', {
      data: { name: `API bare ${Date.now()}`, url: '192.168.1.1:5433', type: 'tcp' },
    });
    expect(bare.status()).toBe(201);
    createdMonitorIds.push(String(((await bare.json()) as UptimeMonitor).id));

    const prefixed = await page.request.post('/api/v1/uptime/monitors', {
      data: { name: `API prefixed ${Date.now()}`, url: 'tcp://192.168.1.1:5434', type: 'tcp' },
    });
    expect(prefixed.status()).toBe(201);
    createdMonitorIds.push(String(((await prefixed.json()) as UptimeMonitor).id));
  });
});
