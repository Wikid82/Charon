import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForLoadingComplete } from './utils/wait-helpers';

const ORTHRUS_AGENTS_API = '**/api/v1/orthrus/agents';
const ORTHRUS_AGENT_API = '**/api/v1/orthrus/agents/*';
const ORTHRUS_PROXY_STATUS_API = '**/api/v1/orthrus/agents/*/proxy-status';
const REMOTE_SERVERS_API = '**/api/v1/remote-servers*';
const HECATE_STATUS_API = '**/api/v1/hecate/status*';

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

const MOCK_PROXY_ACTIVE = {
  agent_uuid: MOCK_AGENT_UUID,
  configured_port: 2375,
  agent_online: true,
  active: true,
  active_port: 2375,
  bind_address: '0.0.0.0:2375',
  connection_string: 'tcp://charon:2375',
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
};

async function setupAgentPage(
  page: import('@playwright/test').Page,
  agents: object[],
  proxyStatus: object | null = null,
) {
  await page.route(REMOTE_SERVERS_API, (route) => route.fulfill({ json: [] }));
  await page.route(HECATE_STATUS_API, (route) => route.fulfill({ json: [] }));
  await page.route(ORTHRUS_AGENTS_API, (route) => {
    if (route.request().method() === 'GET') {
      route.fulfill({ json: agents });
    } else {
      route.continue();
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
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test('configure proxy port — saves port and shows success', async ({ page }) => {
    let patchBody: Record<string, unknown> | null = null;

    await page.route(REMOTE_SERVERS_API, (route) => route.fulfill({ json: [] }));
    await page.route(HECATE_STATUS_API, (route) => route.fulfill({ json: [] }));
    await page.route(ORTHRUS_AGENTS_API, (route) => {
      if (route.request().method() === 'GET') {
        route.fulfill({ json: [MOCK_AGENT_ONLINE] });
      } else {
        route.continue();
      }
    });
    await page.route(ORTHRUS_PROXY_STATUS_API, (route) => route.fulfill({ json: MOCK_PROXY_INACTIVE }));
    await page.route(ORTHRUS_AGENT_API, async (route) => {
      if (route.request().method() === 'PATCH') {
        patchBody = await route.request().postDataJSON() as Record<string, unknown>;
        route.fulfill({ json: { ...MOCK_AGENT_ONLINE, external_proxy_port: 2376 } });
      } else {
        route.continue();
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
    await page.route(ORTHRUS_AGENTS_API, (route) => {
      if (route.request().method() === 'GET') {
        route.fulfill({ json: [MOCK_AGENT_WITH_PROXY] });
      } else {
        route.continue();
      }
    });
    await page.route(ORTHRUS_PROXY_STATUS_API, (route) => route.fulfill({ json: MOCK_PROXY_ACTIVE }));
    await page.route(ORTHRUS_AGENT_API, async (route) => {
      if (route.request().method() === 'PATCH') {
        patchBody = await route.request().postDataJSON() as Record<string, unknown>;
        route.fulfill({ json: { ...MOCK_AGENT_WITH_PROXY, external_proxy_port: 0 } });
      } else {
        route.continue();
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
    await page.route(ORTHRUS_AGENT_API, (route) => {
      if (route.request().method() === 'PATCH') {
        apiCalled = true;
      }
      route.continue();
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
