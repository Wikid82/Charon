import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForLoadingComplete } from './utils/wait-helpers';

const UPTIME_MONITORS_API = '**/api/v1/uptime/monitors';
const UPTIME_MONITOR_HISTORY_API = '**/api/v1/uptime/monitors/*/history*';

const MOCK_AGENT_UUID = 'aaaabbbb-cccc-dddd-eeee-ffffffffffff';
const MOCK_TAILSCALE_IP = '100.99.23.57';

const MOCK_ORTHRUS_MONITOR_BASE = {
  id: 'monitor-orthrus-1',
  name: 'Remote Server (Orthrus)',
  type: 'orthrus',
  url: MOCK_AGENT_UUID,
  enabled: true,
  interval: 60,
  latency: 1,
  max_retries: 3,
  last_check: '2025-01-01T00:01:00Z',
  upstream_host: MOCK_TAILSCALE_IP,
};

const MOCK_TCP_MONITOR_SAME_IP = {
  id: 'monitor-tcp-1',
  name: 'Dockhand Service',
  type: 'tcp',
  url: `${MOCK_TAILSCALE_IP}:3001`,
  enabled: true,
  interval: 60,
  latency: 5,
  max_retries: 3,
  last_check: '2025-01-01T00:01:00Z',
  upstream_host: MOCK_TAILSCALE_IP,
};

function makeHeartbeat(monitorId: string, status: 'up' | 'down', message: string): object {
  return {
    id: 1,
    monitor_id: monitorId,
    status,
    latency: 1,
    message,
    created_at: '2025-01-01T00:01:00Z',
  };
}

async function setupUptimePage(
  page: import('@playwright/test').Page,
  monitors: object[],
  historyByMonitorId: Record<string, object[]> = {},
) {
  await page.route(UPTIME_MONITOR_HISTORY_API, (route) => {
    const url = route.request().url();
    const match = url.match(/\/uptime\/monitors\/([^/]+)\/history/);
    const monitorId = match?.[1] ?? '';
    const history = historyByMonitorId[monitorId] ?? [];
    route.fulfill({ json: history });
  });

  await page.route(UPTIME_MONITORS_API, (route) => {
    if (route.request().method() === 'GET') {
      route.fulfill({ json: monitors });
    } else {
      route.continue();
    }
  });

  await page.goto('/uptime');
  await waitForLoadingComplete(page);
}

test.describe('Uptime monitoring — Orthrus monitor type', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test('Uptime dashboard — Orthrus monitor shows "up" badge when API reports connected', async ({ page }) => {
    const monitor = { ...MOCK_ORTHRUS_MONITOR_BASE, status: 'up' };
    const heartbeat = makeHeartbeat(monitor.id, 'up', 'Orthrus session active');

    await setupUptimePage(page, [monitor], { [monitor.id]: [heartbeat] });

    await test.step('verify UP status badge is displayed for orthrus monitor', async () => {
      const statusBadge = page.getByTestId('status-badge');
      await expect(statusBadge).toBeVisible({ timeout: 8000 });
      await expect(statusBadge).toHaveAttribute('data-status', 'up');
      await expect(statusBadge).toHaveAttribute('aria-label', 'UP');
    });

    await test.step('verify monitor card shows ORTHRUS type badge', async () => {
      const monitorCard = page.getByTestId('monitor-card');
      await expect(monitorCard).toBeVisible();
      await expect(monitorCard).toContainText('ORTHRUS');
    });
  });

  test('Uptime dashboard — Orthrus monitor shows "down" badge when API reports disconnected', async ({ page }) => {
    const monitor = { ...MOCK_ORTHRUS_MONITOR_BASE, status: 'down' };
    const heartbeat = makeHeartbeat(monitor.id, 'down', 'Orthrus agent not connected');

    await setupUptimePage(page, [monitor], { [monitor.id]: [heartbeat] });

    await test.step('verify DOWN status badge is displayed for disconnected orthrus monitor', async () => {
      const statusBadge = page.getByTestId('status-badge');
      await expect(statusBadge).toBeVisible({ timeout: 8000 });
      await expect(statusBadge).toHaveAttribute('data-status', 'down');
      await expect(statusBadge).toHaveAttribute('aria-label', 'DOWN');
    });
  });

  test('Uptime monitor target column — type "orthrus" does not show a raw IP:port', async ({ page }) => {
    const monitor = { ...MOCK_ORTHRUS_MONITOR_BASE, status: 'up' };
    const heartbeat = makeHeartbeat(monitor.id, 'up', 'Orthrus session active');

    await setupUptimePage(page, [monitor], { [monitor.id]: [heartbeat] });

    await test.step('verify monitor URL is agent UUID, not a raw IP:port address', async () => {
      const monitorCard = page.getByTestId('monitor-card');
      await expect(monitorCard).toBeVisible();

      // The URL field must not contain an IP:port pattern like 100.99.23.57:2375
      await expect(monitorCard).not.toContainText(/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d+/);

      // The agent UUID must be displayed as the target
      await expect(monitorCard).toContainText(MOCK_AGENT_UUID);
    });

    await test.step('verify ORTHRUS type badge is present alongside the UUID target', async () => {
      const monitorCard = page.getByTestId('monitor-card');
      await expect(monitorCard).toContainText('ORTHRUS');
    });
  });

  test('Uptime dashboard — non-Orthrus monitor at same IP is checked independently', async ({ page }) => {
    const orthrusMonitor = { ...MOCK_ORTHRUS_MONITOR_BASE, status: 'up' };
    const tcpMonitor = { ...MOCK_TCP_MONITOR_SAME_IP, status: 'up' };
    const orthrusHeartbeat = makeHeartbeat(orthrusMonitor.id, 'up', 'Orthrus session active');
    const tcpHeartbeat = makeHeartbeat(tcpMonitor.id, 'up', 'TCP check passed');

    await setupUptimePage(page, [orthrusMonitor, tcpMonitor], {
      [orthrusMonitor.id]: [orthrusHeartbeat],
      [tcpMonitor.id]: [tcpHeartbeat],
    });

    await test.step('verify both monitor cards are rendered', async () => {
      const cards = page.getByTestId('monitor-card');
      await expect(cards).toHaveCount(2);
    });

    await test.step('verify each monitor card shows its own independent status badge', async () => {
      const badges = page.getByTestId('status-badge');
      await expect(badges).toHaveCount(2);
      await expect(badges.first()).toHaveAttribute('data-status', 'up');
      await expect(badges.nth(1)).toHaveAttribute('data-status', 'up');
    });

    await test.step('verify Orthrus monitor card shows ORTHRUS type and TCP monitor card shows TCP type', async () => {
      const cards = page.getByTestId('monitor-card');
      await expect(cards.first()).toContainText('ORTHRUS');
      await expect(cards.nth(1)).toContainText('TCP');
    });
  });
});
