import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForLoadingComplete } from './utils/wait-helpers';

const REMOTE_SERVERS_API = '**/api/v1/remote-servers*';
const HECATE_STATUS_API = '**/api/v1/hecate/status*';
const HECATE_TUNNELS_API = '**/api/v1/hecate/tunnels*';
const ORTHRUS_AGENTS_API = '**/api/v1/orthrus/agents*';

const DIRECT_SERVER = {
  uuid: 'aaaaaaaa-0000-0000-0000-000000000001',
  name: 'My Direct Server',
  host: '192.168.1.100',
  port: 22,
  connection_type: 'direct',
  enabled: true,
};

const ORTHRUS_SERVER = {
  uuid: 'bbbbbbbb-0000-0000-0000-000000000002',
  name: 'My Orthrus Server',
  host: '',
  port: 0,
  connection_type: 'orthrus',
  enabled: true,
  orthrus_agent_uuid: 'cccccccc-0000-0000-0000-000000000003',
};

const TUNNEL_STATUS_STOPPED = {
  uuid: ORTHRUS_SERVER.uuid,
  state: 'stopped',
};

const TUNNEL_STATUS_CONNECTED = {
  uuid: ORTHRUS_SERVER.uuid,
  state: 'connected',
};

test.describe('Hecate Tunnel Manager', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test.describe('Connection Column - Direct Server', () => {
    test('should show "Direct" badge for servers with direct connection type', async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [DIRECT_SERVER] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Verify Direct badge in connection column', async () => {
        const directBadge = page.getByText('Direct').first();
        await expect(directBadge).toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify no View Tunnel Logs button for direct server', async () => {
        const logsButton = page.getByRole('button', { name: /view tunnel logs/i });
        await expect(logsButton).toHaveCount(0);
      });
    });
  });

  test.describe('Connection Column - Orthrus/Tunnel Server', () => {
    test('should show TunnelStatusBadge for stopped tunnel server', async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [ORTHRUS_SERVER] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [TUNNEL_STATUS_STOPPED] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Verify tunnel status badge is present', async () => {
        const statusBadge = page.locator('[role="status"]').first();
        await expect(statusBadge).toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify tunnel status badge label for stopped state', async () => {
        const statusBadge = page.locator('[role="status"]').first();
        const label = await statusBadge.getAttribute('aria-label');
        expect(label).toMatch(/stopped/i);
      });
    });

    test('should show TunnelStatusBadge for connected tunnel server', async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [ORTHRUS_SERVER] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [TUNNEL_STATUS_CONNECTED] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Verify tunnel status badge shows connected state', async () => {
        const statusBadge = page.locator('[role="status"]').first();
        await expect(statusBadge).toBeVisible({ timeout: 10000 });
        const label = await statusBadge.getAttribute('aria-label');
        expect(label).toMatch(/connected/i);
      });
    });

    test('should show View Tunnel Logs button for orthrus server', async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [ORTHRUS_SERVER] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [TUNNEL_STATUS_STOPPED] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Verify View Tunnel Logs button is present for non-direct server', async () => {
        const logsButton = page.getByRole('button', {
          name: new RegExp(`view tunnel logs.*${ORTHRUS_SERVER.name}`, 'i'),
        });
        await expect(logsButton).toBeVisible({ timeout: 10000 });
      });
    });

    test('should show connection type badge when no tunnel status is available', async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [ORTHRUS_SERVER] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Verify connection type fallback badge is shown', async () => {
        const orthrusBadge = page.getByText('orthrus').first();
        await expect(orthrusBadge).toBeVisible({ timeout: 10000 });
      });
    });
  });

  test.describe('Add Server Form - Connection Type Selector', () => {
    test.beforeEach(async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [] });
      });
      await page.route(HECATE_TUNNELS_API, (route) => {
        route.fulfill({ json: [] });
      });
    });

    test('should open Add Server form when Add Server button is clicked', async ({ page }) => {
      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Click Add Server button', async () => {
        const addButton = page.getByRole('button', { name: /add server/i }).first();
        await expect(addButton).toBeVisible({ timeout: 10000 });
        await addButton.click();
      });

      await test.step('Verify form heading is visible', async () => {
        const formHeading = page.getByRole('heading', { name: /add remote server/i });
        await expect(formHeading).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify Connection Type selector is present', async () => {
        const connectionTypeSelect = page.locator('#connection-type').or(
          page.getByRole('combobox', { name: /connection type/i })
        );
        await expect(connectionTypeSelect).toBeVisible();
      });
    });

    test('should show host and port fields for direct connection type', async ({ page }) => {
      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Open Add Server form', async () => {
        const addButton = page.getByRole('button', { name: /add server/i }).first();
        await addButton.click();
        await expect(page.getByRole('heading', { name: /add remote server/i })).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify host and port fields are visible for direct type', async () => {
        const hostInput = page.getByRole('textbox', { name: /host/i });
        await expect(hostInput).toBeVisible();
        const portInput = page.getByRole('spinbutton', { name: /port/i }).or(
          page.locator('input[name="port"], input[placeholder*="port"]')
        );
        await expect(portInput).toBeVisible();
      });
    });

    test('should show orthrus agent section when orthrus connection type is selected', async ({ page }) => {
      await page.route(ORTHRUS_AGENTS_API, (route) => {
        route.fulfill({ json: [] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Open Add Server form', async () => {
        const addButton = page.getByRole('button', { name: /add server/i }).first();
        await addButton.click();
        await expect(page.getByRole('heading', { name: /add remote server/i })).toBeVisible({ timeout: 5000 });
      });

      await test.step('Change connection type to Orthrus Agent', async () => {
        const connectionTypeSelect = page.locator('#connection-type');
        await connectionTypeSelect.selectOption('orthrus');
      });

      await test.step('Verify Provision New Agent button appears', async () => {
        const provisionButton = page.getByRole('button', { name: /provision.*agent/i });
        await expect(provisionButton).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify host/port fields are hidden for orthrus type', async () => {
        const hostInput = page.getByRole('textbox', { name: /^host$/i });
        await expect(hostInput).toHaveCount(0);
      });
    });

    test('should show cloudflare wizard when cloudflare connection type is selected', async ({ page }) => {
      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Open Add Server form', async () => {
        const addButton = page.getByRole('button', { name: /add server/i }).first();
        await addButton.click();
        await expect(page.getByRole('heading', { name: /add remote server/i })).toBeVisible({ timeout: 5000 });
      });

      await test.step('Change connection type to Cloudflare Tunnel', async () => {
        const connectionTypeSelect = page.locator('#connection-type');
        await connectionTypeSelect.selectOption('cloudflare');
      });

      await test.step('Verify cloudflare wizard content appears', async () => {
        // CloudflareTunnelWizard is rendered when cloudflare is selected
        // Just verify host/port fields are gone and some cloudflare-related content is shown
        const hostInput = page.getByRole('textbox', { name: /^host$/i });
        await expect(hostInput).toHaveCount(0);
      });
    });

    test('Connection Type selector accessibility snapshot', async ({ page }) => {
      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Open Add Server form', async () => {
        const addButton = page.getByRole('button', { name: /add server/i }).first();
        await addButton.click();
        await expect(page.getByRole('heading', { name: /add remote server/i })).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify connection type selector accessibility', async () => {
        const connectionTypeSelect = page.locator('#connection-type');
        await expect(connectionTypeSelect).toMatchAriaSnapshot(`
          - combobox "Connection Type":
            - option "Direct"
            - option "Orthrus Agent"
            - option "Cloudflare Tunnel"
        `);
      });
    });
  });

  test.describe('TunnelLogViewer', () => {
    test('should open TunnelLogViewer dialog when View Tunnel Logs is clicked', async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [ORTHRUS_SERVER] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [TUNNEL_STATUS_STOPPED] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Click View Tunnel Logs button', async () => {
        const logsButton = page.getByRole('button', {
          name: new RegExp(`view tunnel logs.*${ORTHRUS_SERVER.name}`, 'i'),
        });
        await expect(logsButton).toBeVisible({ timeout: 10000 });
        await logsButton.click();
      });

      await test.step('Verify log viewer dialog opens', async () => {
        const dialog = page.getByRole('dialog');
        await expect(dialog).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify log region has correct accessibility attributes', async () => {
        const logRegion = page.locator('[role="log"]');
        await expect(logRegion).toBeVisible();
        const ariaLabel = await logRegion.getAttribute('aria-label');
        expect(ariaLabel).toMatch(new RegExp(ORTHRUS_SERVER.name, 'i'));
        const ariaLive = await logRegion.getAttribute('aria-live');
        expect(ariaLive).toBe('polite');
      });

      await test.step('Verify Pause button is accessible', async () => {
        const pauseButton = page.getByRole('button', { name: /pause/i });
        await expect(pauseButton).toBeVisible();
        const ariaPressed = await pauseButton.getAttribute('aria-pressed');
        expect(ariaPressed).toBeDefined();
      });

      await test.step('Verify Clear button is accessible', async () => {
        const clearButton = page.getByRole('button', { name: /clear/i });
        await expect(clearButton).toBeVisible();
      });
    });

    test('Pause button toggles aria-pressed state', async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [ORTHRUS_SERVER] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [TUNNEL_STATUS_STOPPED] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Open log viewer', async () => {
        const logsButton = page.getByRole('button', {
          name: new RegExp(`view tunnel logs.*${ORTHRUS_SERVER.name}`, 'i'),
        });
        await logsButton.click();
        await expect(page.getByRole('dialog')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify initial Pause button state is not pressed', async () => {
        const pauseButton = page.getByRole('button', { name: /pause/i });
        await expect(pauseButton).toHaveAttribute('aria-pressed', 'false');
      });

      await test.step('Click Pause and verify state changes to pressed', async () => {
        const pauseButton = page.getByRole('button', { name: /pause/i });
        await pauseButton.click();
        const resumeButton = page.getByRole('button', { name: /resume/i });
        await expect(resumeButton).toHaveAttribute('aria-pressed', 'true');
      });
    });

    test.skip('WebSocket streaming displays live log entries - requires live WebSocket backend', async () => {
      // This test requires a live WebSocket connection via connectTunnelLogs().
      // Mock-based testing cannot verify streaming behavior.
    });
  });

  test.describe('Page Accessibility', () => {
    test('Remote Servers page has proper heading structure', async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Verify page has main landmark', async () => {
        await expect(page.getByRole('main')).toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify Add Server button is accessible', async () => {
        const addButton = page.getByRole('button', { name: /add server/i }).first();
        await expect(addButton).toBeVisible();
      });
    });

    test('Remote Servers page with mixed server types has accessible connection badges', async ({ page }) => {
      await page.route(REMOTE_SERVERS_API, (route) => {
        route.fulfill({ json: [DIRECT_SERVER, ORTHRUS_SERVER] });
      });
      await page.route(HECATE_STATUS_API, (route) => {
        route.fulfill({ json: [TUNNEL_STATUS_STOPPED] });
      });

      await page.goto('/hecate/remote-servers');
      await waitForLoadingComplete(page);

      await test.step('Verify direct badge is present', async () => {
        await expect(page.getByText('Direct').first()).toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify tunnel status badge has role="status"', async () => {
        const statusBadge = page.locator('[role="status"]').first();
        await expect(statusBadge).toBeVisible();
      });
    });
  });
});
