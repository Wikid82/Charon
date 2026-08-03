import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForLoadingComplete } from './utils/wait-helpers';

const ORTHRUS_AGENTS_API = '**/api/v1/orthrus/agents';
const ORTHRUS_AGENT_API = '**/api/v1/orthrus/agents/*';
const ORTHRUS_PROXY_STATUS_API = '**/api/v1/orthrus/agents/*/proxy-status';
const REMOTE_SERVERS_API = '**/api/v1/remote-servers*';
const HECATE_STATUS_API = '**/api/v1/hecate/status*';

const MOCK_AGENT_UUID = 'aaaabbbb-cccc-dddd-eeee-ffffffffffff';
const MOCK_AGENT_NAME = 'write-mode-test-agent';

const MOCK_AGENT_BASE = {
  uuid: MOCK_AGENT_UUID,
  name: MOCK_AGENT_NAME,
  status: 'online',
  capabilities: '',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 2375,
  write_enabled: false,
};

const MOCK_AGENT_WRITE_ENABLED = {
  ...MOCK_AGENT_BASE,
  write_enabled: true,
};

const MOCK_PROXY_STATUS_WRITE_OFF = {
  agent_uuid: MOCK_AGENT_UUID,
  configured_port: 2375,
  configured_write_enabled: false,
  active_write_enabled: false,
  agent_online: true,
  active: true,
  active_port: 2375,
  bind_address: '0.0.0.0:2375',
  connection_string: 'tcp://charon:2375',
};

const MOCK_PROXY_STATUS_WRITE_ON_PENDING_RECONNECT = {
  ...MOCK_PROXY_STATUS_WRITE_OFF,
  configured_write_enabled: true,
  active_write_enabled: false, // agent hasn't reconnected since the flag was set
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
    await page.route(ORTHRUS_PROXY_STATUS_API, (route) => route.fulfill({ json: proxyStatus }));
  }
  await page.goto('/hecate/agent');
  await waitForLoadingComplete(page);
}

// The Switch component's underlying <input role="switch"> is visually
// hidden (sr-only) behind a custom-styled track, with near-zero hit area.
// A user actually clicks the wrapping <label> (which contains both the
// input and the visible track, and natively toggles the input per standard
// HTML label semantics); clicking the input directly — even with
// { force: true } — can land on a clipped coordinate that never reaches the
// input. Clicking the label is both more robust and closer to real
// end-user interaction.
function toggleLabel(toggle: import('@playwright/test').Locator) {
  return toggle.locator('xpath=..');
}

async function openWriteModeDialog(page: import('@playwright/test').Page, agentName: string) {
  const writeModeButton = page.getByRole('button', {
    name: new RegExp(`write.*mode.*${agentName}`, 'i'),
  });
  await expect(writeModeButton).toBeVisible({ timeout: 8000 });
  await writeModeButton.click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible({ timeout: 5000 });
  return dialog;
}

test.describe('Orthrus Write Mode', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test(
    'write mode is off by default for a newly provisioned agent',
    async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_BASE], MOCK_PROXY_STATUS_WRITE_OFF);

      const dialog = await openWriteModeDialog(page, MOCK_AGENT_NAME);

      await test.step('Toggle reflects write_enabled: false', async () => {
        const toggle = dialog.getByRole('switch');
        await expect(toggle).not.toBeChecked();
      });

      await test.step('No typed-confirmation input is shown while off', async () => {
        const confirmInput = dialog.getByRole('textbox', { name: /type .* to confirm/i });
        await expect(confirmInput).not.toBeVisible();
      });
    },
  );

  test(
    'enabling write mode requires typing the exact agent name before Save is enabled',
    async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_BASE], MOCK_PROXY_STATUS_WRITE_OFF);

      const dialog = await openWriteModeDialog(page, MOCK_AGENT_NAME);

      await test.step('Flip toggle on and reveal typed-confirmation input', async () => {
        const toggle = dialog.getByRole('switch');
        await toggleLabel(toggle).click();
        await expect(dialog.getByRole('textbox', { name: /type .* to confirm/i })).toBeVisible();
      });

      await test.step('Save disabled with empty or mismatched confirmation text', async () => {
        const confirmInput = dialog.getByRole('textbox', { name: /type .* to confirm/i });
        const saveButton = dialog.getByRole('button', { name: /save|enable/i });

        await expect(saveButton).toBeDisabled();

        await confirmInput.fill('wrong-name');
        await expect(saveButton).toBeDisabled();

        await confirmInput.fill('');
        await expect(saveButton).toBeDisabled();
      });

      await test.step('Save enabled once typed value exactly matches agent name', async () => {
        const confirmInput = dialog.getByRole('textbox', { name: /type .* to confirm/i });
        const saveButton = dialog.getByRole('button', { name: /save|enable/i });

        await confirmInput.fill(MOCK_AGENT_NAME);
        await expect(saveButton).toBeEnabled();
      });
    },
  );

  test(
    'enabling write mode succeeds, closes the dialog, and PATCHes write_enabled: true',
    async ({ page }) => {
      let patchBody: Record<string, unknown> | null = null;

      await setupAgentPage(page, [MOCK_AGENT_BASE], MOCK_PROXY_STATUS_WRITE_OFF);
      await page.route(ORTHRUS_AGENT_API, async (route) => {
        if (route.request().method() === 'PATCH') {
          patchBody = (await route.request().postDataJSON()) as Record<string, unknown>;
          route.fulfill({ json: MOCK_AGENT_WRITE_ENABLED });
        } else {
          route.continue();
        }
      });

      const dialog = await openWriteModeDialog(page, MOCK_AGENT_NAME);

      await test.step('Enable write mode with correct typed confirmation', async () => {
        const toggle = dialog.getByRole('switch');
        await toggleLabel(toggle).click();
        await dialog.getByRole('textbox', { name: /type .* to confirm/i }).fill(MOCK_AGENT_NAME);
        await dialog.getByRole('button', { name: /save|enable/i }).click();
      });

      await test.step('PATCH sent write_enabled: true', async () => {
        expect(patchBody).toBeTruthy();
        expect(patchBody?.write_enabled).toBe(true);
      });

      await test.step('Dialog closes on success, matching AgentExternalProxyDialog\'s pattern', async () => {
        await expect(dialog).not.toBeVisible({ timeout: 5000 });
      });
    },
  );

  test(
    'reconnect notice shown when reopening the dialog before the agent has reconnected',
    async ({ page }) => {
      // Simulates coming back to an agent some time after enabling write
      // mode: the DB/list already reflects write_enabled: true (a real
      // save always closes the dialog — see the previous test — so the
      // only way to observe the reconnect notice is on a later open,
      // exactly as AgentExternalProxyDialog's equivalent test does for
      // the port setting).
      await setupAgentPage(page, [MOCK_AGENT_WRITE_ENABLED], MOCK_PROXY_STATUS_WRITE_ON_PENDING_RECONNECT);

      const dialog = await openWriteModeDialog(page, MOCK_AGENT_NAME);

      await test.step('Reconnect notice shown while active_write_enabled is still false', async () => {
        await expect(dialog.getByText(/next agent reconnect/i)).toBeVisible({ timeout: 5000 });
      });
    },
  );

  test('disabling write mode requires no typed confirmation', async ({ page }) => {
    let patchBody: Record<string, unknown> | null = null;

    await setupAgentPage(page, [MOCK_AGENT_WRITE_ENABLED], {
      ...MOCK_PROXY_STATUS_WRITE_OFF,
      configured_write_enabled: true,
      active_write_enabled: true,
    });
    await page.route(ORTHRUS_AGENT_API, async (route) => {
      if (route.request().method() === 'PATCH') {
        patchBody = (await route.request().postDataJSON()) as Record<string, unknown>;
        route.fulfill({ json: MOCK_AGENT_BASE });
      } else {
        route.continue();
      }
    });

    const dialog = await openWriteModeDialog(page, MOCK_AGENT_NAME);

    await test.step('Flip toggle off and save without typing anything', async () => {
      const toggle = dialog.getByRole('switch');
      await expect(toggle).toBeChecked();
      await toggleLabel(toggle).click();

      const confirmInput = dialog.getByRole('textbox', { name: /type .* to confirm/i });
      await expect(confirmInput).not.toBeVisible();

      await dialog.getByRole('button', { name: /save|disable/i }).click();
    });

    await test.step('PATCH sent write_enabled: false', async () => {
      expect(patchBody?.write_enabled).toBe(false);
    });
  });

  test('WRITE badge appears in agent row when write_enabled is true', async ({ page }) => {
    await setupAgentPage(page, [MOCK_AGENT_WRITE_ENABLED], null);

    await test.step('WRITE badge is visible in agent row', async () => {
      await expect(page.getByText('WRITE', { exact: true })).toBeVisible({ timeout: 8000 });
    });
  });

  test('WRITE badge absent when write_enabled is false', async ({ page }) => {
    await setupAgentPage(page, [MOCK_AGENT_BASE], null);

    await test.step('WRITE badge is not visible', async () => {
      await expect(page.getByText('WRITE', { exact: true })).not.toBeVisible({ timeout: 5000 });
    });
  });

  test(
    'audit log link from write-mode dialog lands on a pre-filtered view',
    async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_WRITE_ENABLED], {
        ...MOCK_PROXY_STATUS_WRITE_OFF,
        configured_write_enabled: true,
        active_write_enabled: true,
      });

      const dialog = await openWriteModeDialog(page, MOCK_AGENT_NAME);

      await test.step('Click the audit log link', async () => {
        await dialog.getByRole('link', { name: /write.access.audit.log/i }).click();
      });

      await test.step('Lands on /audit-logs with resource_uuid and event_category applied', async () => {
        await expect(page).toHaveURL(
          new RegExp(`/audit-logs\\?.*resource_uuid=${MOCK_AGENT_UUID}.*event_category=orthrus_write`),
        );
      });
    },
  );
});
