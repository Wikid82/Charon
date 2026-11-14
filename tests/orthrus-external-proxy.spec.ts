import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForLoadingComplete } from './utils/wait-helpers';

const ORTHRUS_AGENTS_API = '**/api/v1/orthrus/agents';
const ORTHRUS_AGENT_API = '**/api/v1/orthrus/agents/*';
const ORTHRUS_PROXY_STATUS_API = '**/api/v1/orthrus/agents/*/proxy-status';
const REMOTE_SERVERS_API = '**/api/v1/remote-servers*';
const HECATE_STATUS_API = '**/api/v1/hecate/status*';
const LOCAL_NETWORKS_API = '**/api/v1/orthrus/local-networks';

const MOCK_AGENT_UUID = 'aaaabbbb-cccc-dddd-eeee-ffffffffffff';
const MOCK_AGENT_NAME = 'proxy-test-agent';

const MOCK_AGENT_ONLINE = {
  uuid: MOCK_AGENT_UUID,
  name: MOCK_AGENT_NAME,
  status: 'online',
  capabilities: '',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 0,
};

const MOCK_AGENT_WITH_PROXY = {
  ...MOCK_AGENT_ONLINE,
  external_proxy_port: 2375,
};

const MOCK_LOCAL_NETWORKS = {
  auto: { address: '172.18.0.4', source: 'default_route' },
  networks: [
    { interface: 'eth0', address: '172.18.0.4', cidr: '172.18.0.0/16', primary: true },
  ],
};

const MOCK_PROXY_ACTIVE = {
  agent_uuid: MOCK_AGENT_UUID,
  configured_port: 2375,
  agent_online: true,
  active: true,
  active_port: 2375,
  bind_address: '172.18.0.4:2375',
  connection_string: 'tcp://charon:2375',
  configured_bind: '',
  active_bind: '',
  bind_mode: 'auto',
  bind_fallback: false,
};

const MOCK_PROXY_INACTIVE = {
  agent_uuid: MOCK_AGENT_UUID,
  configured_port: 0,
  agent_online: true,
  active: false,
  active_port: 0,
  bind_address: '',
  connection_string: '',
  error: '',
  configured_bind: '',
  active_bind: '',
  bind_mode: '',
  bind_fallback: false,
};

const MOCK_PROXY_ERROR = {
  agent_uuid: MOCK_AGENT_UUID,
  configured_port: 2375,
  agent_online: true,
  active: false,
  active_port: 0,
  bind_address: '',
  connection_string: '',
  error: 'bind tcp 0.0.0.0:2375: bind: address already in use',
  configured_bind: '',
  active_bind: '',
  bind_mode: 'auto',
  bind_fallback: false,
};

async function setupAgentPage(
  page: import('@playwright/test').Page,
  agents: object[],
  proxyStatus: object | null = null,
) {
  await page.route(REMOTE_SERVERS_API, (route) => route.fulfill({ json: [] }));
  await page.route(HECATE_STATUS_API, (route) => route.fulfill({ json: [] }));
  await page.route(ORTHRUS_AGENTS_API, async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ json: agents });
    } else {
      await route.continue();
    }
  });
  if (proxyStatus !== null) {
    await page.route(ORTHRUS_PROXY_STATUS_API, (route) =>
      route.fulfill({ json: proxyStatus }),
    );
  }
  await page.goto('/hecate/agent');
  await waitForLoadingComplete(page);
}

async function openProxyDialog(page: import('@playwright/test').Page, agentName: string) {
  const configureButton = page.getByRole('button', {
    name: new RegExp(`configure.*proxy.*${agentName}|external.*docker.*proxy.*${agentName}`, 'i'),
  });
  await expect(configureButton).toBeVisible({ timeout: 8000 });
  await configureButton.click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible({ timeout: 5000 });
  return dialog;
}

test.describe('External Docker Proxy', () => {
  test.beforeEach(async ({ request, page }) => {
    await waitForAPIHealth(request);
    await page.route(LOCAL_NETWORKS_API, (route) => route.fulfill({ json: MOCK_LOCAL_NETWORKS }));
  });

  test('configure proxy port — saves port and shows success', async ({ page }) => {
    let patchBody: Record<string, unknown> | null = null;

    await page.route(REMOTE_SERVERS_API, (route) => route.fulfill({ json: [] }));
    await page.route(HECATE_STATUS_API, (route) => route.fulfill({ json: [] }));
    await page.route(ORTHRUS_AGENTS_API, async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ json: [MOCK_AGENT_ONLINE] });
      } else {
        await route.continue();
      }
    });
    await page.route(ORTHRUS_PROXY_STATUS_API, (route) => route.fulfill({ json: MOCK_PROXY_INACTIVE }));
    await page.route(ORTHRUS_AGENT_API, async (route) => {
      if (route.request().method() === 'PATCH') {
        patchBody = await route.request().postDataJSON() as Record<string, unknown>;
        await route.fulfill({ json: { ...MOCK_AGENT_ONLINE, external_proxy_port: 2376 } });
      } else {
        await route.continue();
      }
    });

    await page.goto('/hecate/agent');
    await waitForLoadingComplete(page);

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Enter port 2376', async () => {
      const portInput = dialog.locator('#external-proxy-port');
      await expect(portInput).toBeVisible({ timeout: 5000 });
      await portInput.fill('2376');
    });

    await test.step('Click Save and verify API call', async () => {
      const saveButton = dialog.getByRole('button', { name: /save/i });
      await saveButton.click();

      await expect(dialog).not.toBeVisible({ timeout: 8000 });
      expect(patchBody).toBeTruthy();
      expect(patchBody?.external_proxy_port).toBe(2376);
    });
  });

  test('disable proxy — set port to 0', async ({ page }) => {
    let patchBody: Record<string, unknown> | null = null;

    await page.route(REMOTE_SERVERS_API, (route) => route.fulfill({ json: [] }));
    await page.route(HECATE_STATUS_API, (route) => route.fulfill({ json: [] }));
    await page.route(ORTHRUS_AGENTS_API, async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ json: [MOCK_AGENT_WITH_PROXY] });
      } else {
        await route.continue();
      }
    });
    await page.route(ORTHRUS_PROXY_STATUS_API, (route) => route.fulfill({ json: MOCK_PROXY_ACTIVE }));
    await page.route(ORTHRUS_AGENT_API, async (route) => {
      if (route.request().method() === 'PATCH') {
        patchBody = await route.request().postDataJSON() as Record<string, unknown>;
        await route.fulfill({ json: { ...MOCK_AGENT_WITH_PROXY, external_proxy_port: 0 } });
      } else {
        await route.continue();
      }
    });

    await page.goto('/hecate/agent');
    await waitForLoadingComplete(page);

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Clear port and set to 0', async () => {
      const portInput = dialog.locator('#external-proxy-port');
      await portInput.fill('0');
    });

    await test.step('Save and verify port 0 sent to API', async () => {
      const saveButton = dialog.getByRole('button', { name: /save/i });
      await saveButton.click();
      await expect(dialog).not.toBeVisible({ timeout: 8000 });
      expect(patchBody?.external_proxy_port).toBe(0);
    });
  });

  test('invalid port rejected — client-side validation prevents API call', async ({ page }) => {
    let apiCalled = false;

    await setupAgentPage(page, [MOCK_AGENT_ONLINE], MOCK_PROXY_INACTIVE);
    await page.route(ORTHRUS_AGENT_API, async (route) => {
      if (route.request().method() === 'PATCH') {
        apiCalled = true;
      }
      await route.continue();
    });

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Enter privileged port 80', async () => {
      const portInput = dialog.locator('#external-proxy-port');
      await portInput.fill('80');
    });

    await test.step('Save button is disabled and alert shown for invalid port', async () => {
      const saveButton = dialog.getByRole('button', { name: /save/i });
      await expect(saveButton).toBeDisabled();

      const error = dialog.locator('[role="alert"]');
      await expect(error).toBeVisible({ timeout: 3000 });
      await expect(error).toContainText(/1024/);
      expect(apiCalled).toBe(false);
    });

    await test.step('Dialog remains open after validation error', async () => {
      await expect(dialog).toBeVisible();
    });
  });

  test('connection string displayed when agent online with active proxy', async ({ page }) => {
    await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY], MOCK_PROXY_ACTIVE);

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Connection string is visible', async () => {
      await expect(dialog.getByText('tcp://charon:2375')).toBeVisible({ timeout: 5000 });
    });

    await test.step('Copy button is present with accessible label', async () => {
      const copyButton = dialog.getByRole('button', {
        name: /copy docker connection string/i,
      });
      await expect(copyButton).toBeVisible();
    });

    await test.step('Active status indicator is shown', async () => {
      // Status region uses role=status with aria-live
      const statusRegion = dialog.locator('[role="status"]');
      await expect(statusRegion).toBeVisible();
      await expect(statusRegion).toContainText(/active/i);
    });
  });

  test('reconnect notice shown when configured port differs from active port', async ({ page }) => {
    // Agent has external_proxy_port=2376 (saved in DB) but proxy is still
    // running on 2375 (active_port). The form initialises to 2376, so
    // active_port(2375) !== portValue(2376) → reconnect notice is shown.
    const divergedAgent = { ...MOCK_AGENT_WITH_PROXY, external_proxy_port: 2376 };
    const divergedStatus = {
      ...MOCK_PROXY_ACTIVE,
      configured_port: 2376,
      active_port: 2375,
    };

    await setupAgentPage(page, [divergedAgent], divergedStatus);

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Reconnect notice is visible', async () => {
      await expect(
        dialog.getByText(/next agent reconnect/i),
      ).toBeVisible({ timeout: 5000 });
    });
  });

  test('error state displayed when bind fails', async ({ page }) => {
    await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY], MOCK_PROXY_ERROR);

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Error message is displayed with role=alert', async () => {
      const errorAlert = dialog.locator('[role="alert"]');
      await expect(errorAlert).toBeVisible({ timeout: 5000 });
      await expect(errorAlert).toContainText(/address already in use/i);
    });
  });

  test('listen on all networks — saves 0.0.0.0 and shows the exposure note', async ({ page }) => {
    let patchBody: Record<string, unknown> | null = null;

    await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY], MOCK_PROXY_ACTIVE);
    await page.route(ORTHRUS_AGENT_API, async (route) => {
      if (route.request().method() === 'PATCH') {
        patchBody = await route.request().postDataJSON() as Record<string, unknown>;
        await route.fulfill({ json: { ...MOCK_AGENT_WITH_PROXY, external_proxy_bind: '0.0.0.0' } });
      } else {
        await route.continue();
      }
    });

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Choose all networks', async () => {
      await dialog.getByLabel('Listen on').selectOption({ value: 'all' });
      await expect(dialog.getByText(/no login/i)).toBeVisible();
    });

    await test.step('Save sends 0.0.0.0 as the listen setting', async () => {
      await dialog.getByRole('button', { name: /save/i }).click();
      await expect(dialog).not.toBeVisible({ timeout: 8000 });
      expect(patchBody?.external_proxy_bind).toBe('0.0.0.0');
    });
  });

  test('listen on a detected network — saves its CIDR', async ({ page }) => {
    let patchBody: Record<string, unknown> | null = null;

    await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY], MOCK_PROXY_ACTIVE);
    await page.route(ORTHRUS_AGENT_API, async (route) => {
      if (route.request().method() === 'PATCH') {
        patchBody = await route.request().postDataJSON() as Record<string, unknown>;
        await route.fulfill({ json: { ...MOCK_AGENT_WITH_PROXY, external_proxy_bind: '172.18.0.0/16' } });
      } else {
        await route.continue();
      }
    });

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Choose the detected eth0 network', async () => {
      await dialog.getByLabel('Listen on').selectOption({ label: 'eth0 · 172.18.0.4 (172.18.0.0/16)' });
    });

    await test.step('Save sends the CIDR', async () => {
      await dialog.getByRole('button', { name: /save/i }).click();
      await expect(dialog).not.toBeVisible({ timeout: 8000 });
      expect(patchBody?.external_proxy_bind).toBe('172.18.0.0/16');
    });
  });

  test('server refusal of a custom listen address is shown in the dialog', async ({ page }) => {
    await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY], MOCK_PROXY_ACTIVE);
    await page.route(ORTHRUS_AGENT_API, async (route) => {
      if (route.request().method() === 'PATCH') {
        await route.fulfill({
          status: 400,
          json: {
            error: 'Listen address must be automatic, all interfaces, an address or a subnet',
            error_code: 'external_proxy_bind_invalid',
          },
        });
      } else {
        await route.continue();
      }
    });

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Enter a custom value the server rejects', async () => {
      await dialog.getByLabel('Listen on').selectOption({ value: 'custom' });
      await dialog.getByLabel('Address or subnet').fill('not-an-address');
      await dialog.getByRole('button', { name: /save/i }).click();
    });

    await test.step('Refusal is shown as an alert and the dialog stays open', async () => {
      await expect(dialog.getByRole('alert')).toContainText(/IP address or a subnet/i);
      await expect(dialog).toBeVisible();
    });
  });

  test('loopback fallback — explains it is not reachable from other containers or hosts', async ({ page }) => {
    const fallbackStatus = {
      ...MOCK_PROXY_ACTIVE,
      bind_address: '127.0.0.1:2375',
      bind_fallback: true,
    };
    await page.route(LOCAL_NETWORKS_API, (route) =>
      route.fulfill({
        json: { auto: { address: '127.0.0.1', source: 'loopback_fallback' }, networks: [] },
      }),
    );

    await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY], fallbackStatus);

    await test.step('Agent card carries the passive note', async () => {
      await expect(
        page.getByText('Not reachable from other containers or hosts.').first(),
      ).toBeVisible({ timeout: 8000 });
    });

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Dialog explains the fallback and never says "this server only"', async () => {
      await expect(dialog.getByText(/found no network/i)).toBeVisible();
      await expect(dialog.getByText('Not reachable from other containers or hosts.').first()).toBeVisible();
      await expect(dialog.getByText(/this server only/i)).toHaveCount(0);
    });
  });

  test('reconnect notice shown when the saved listen setting differs from the live one', async ({ page }) => {
    const divergedStatus = {
      ...MOCK_PROXY_ACTIVE,
      configured_bind: '172.18.0.0/16',
      active_bind: '',
    };

    await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY], divergedStatus);

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Listen address reconnect notice is visible', async () => {
      await expect(
        dialog.getByText('The listen address change takes effect when the agent reconnects.'),
      ).toBeVisible({ timeout: 5000 });
    });
  });

  test('routable automatic listener — shows the bound address and no fallback or loopback note', async ({ page }) => {
    await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY], MOCK_PROXY_ACTIVE);

    await test.step('Agent card notes the automatic listen address', async () => {
      await expect(
        page.getByText('Listen address is automatic. Choose a network in the proxy settings if other hosts need access.').first(),
      ).toBeVisible({ timeout: 8000 });
    });

    const dialog = await openProxyDialog(page, MOCK_AGENT_NAME);

    await test.step('Dialog shows the live bound address', async () => {
      await expect(dialog.getByText('Listening on 172.18.0.4:2375')).toBeVisible({ timeout: 5000 });
    });

    await test.step('Neither the fallback nor the loopback explanation is shown', async () => {
      await expect(dialog.getByText(/found no network/i)).toHaveCount(0);
      await expect(dialog.getByText('Not reachable from other containers or hosts.')).toHaveCount(0);
    });
  });

  test('PROXY badge appears in agent row when external_proxy_port > 0', async ({ page }) => {
    await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY], null);

    await test.step('PROXY badge is visible in agent row', async () => {
      const proxyBadge = page.getByText('PROXY', { exact: true });
      await expect(proxyBadge).toBeVisible({ timeout: 8000 });
    });
  });

  test('PROXY badge absent when external_proxy_port is 0', async ({ page }) => {
    await setupAgentPage(page, [MOCK_AGENT_ONLINE], null);

    await test.step('PROXY badge is not visible', async () => {
      const proxyBadge = page.getByText('PROXY', { exact: true });
      await expect(proxyBadge).not.toBeVisible({ timeout: 5000 });
    });
  });
});
