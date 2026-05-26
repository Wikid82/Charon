import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForLoadingComplete } from './utils/wait-helpers';

const ORTHRUS_AGENTS_API = '**/api/v1/orthrus/agents';
const REMOTE_SERVERS_API = '**/api/v1/remote-servers*';
const HECATE_STATUS_API = '**/api/v1/hecate/status*';

const MOCK_AGENT_UUID_1 = 'aaaa0001-cccc-dddd-eeee-ffffffffffff';
const MOCK_AGENT_UUID_2 = 'aaaa0002-cccc-dddd-eeee-ffffffffffff';
const MOCK_AGENT_UUID_3 = 'aaaa0003-cccc-dddd-eeee-ffffffffffff';
const MOCK_AGENT_UUID_4 = 'aaaa0004-cccc-dddd-eeee-ffffffffffff';

const MOCK_AGENT_ONLINE = {
  uuid: MOCK_AGENT_UUID_1,
  name: 'online-agent',
  status: 'online',
  capabilities: '',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 0,
};

const MOCK_AGENT_OFFLINE = {
  uuid: MOCK_AGENT_UUID_2,
  name: 'offline-agent',
  status: 'offline',
  capabilities: '',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 0,
};

const MOCK_AGENT_PENDING = {
  uuid: MOCK_AGENT_UUID_3,
  name: 'pending-agent',
  status: 'pending',
  capabilities: '',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 0,
};

const MOCK_AGENT_WITH_PROXY = {
  uuid: MOCK_AGENT_UUID_4,
  name: 'proxy-agent',
  status: 'online',
  capabilities: '',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 2375,
};

async function setupAgentPage(
  page: import('@playwright/test').Page,
  agents: object[],
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
  await page.goto('/hecate/agent');
  await waitForLoadingComplete(page);
}

test.describe('Orthrus Agent Manager', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test.describe('Page structure', () => {
    test('renders the Orthrus Agents section heading', async ({ page }) => {
      await setupAgentPage(page, []);
      await expect(page.getByRole('heading', { name: 'Orthrus Agents' })).toBeVisible();
    });

    test('renders the Provision New Agent button', async ({ page }) => {
      await setupAgentPage(page, []);
      await expect(
        page.getByRole('button', { name: /provision new agent/i }),
      ).toBeVisible();
    });
  });

  test.describe('Empty state', () => {
    test('shows no-agents message when agent list is empty', async ({
      page,
    }) => {
      await setupAgentPage(page, []);
      await expect(page.getByText('No agents registered yet.')).toBeVisible();
    });

    test('does not render a table when agent list is empty', async ({
      page,
    }) => {
      await setupAgentPage(page, []);
      await expect(page.getByRole('table')).not.toBeVisible();
    });
  });

  test.describe('Agent table', () => {
    test('renders a table when agents are present', async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await expect(page.getByRole('table')).toBeVisible();
    });

    test('table has accessible label "Orthrus agents"', async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await expect(
        page.getByRole('table', { name: 'Orthrus agents' }),
      ).toBeVisible();
    });

    test('shows one data row per agent plus a header row', async ({ page }) => {
      await setupAgentPage(page, [
        MOCK_AGENT_ONLINE,
        MOCK_AGENT_OFFLINE,
        MOCK_AGENT_PENDING,
      ]);
      const rows = page.getByRole('table').getByRole('row');
      // header row + 3 agent rows
      await expect(rows).toHaveCount(4);
    });
  });

  test.describe('Status badges', () => {
    test('online agent shows Online status text', async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await expect(page.getByRole('table').getByText('Online', { exact: true })).toBeVisible();
    });

    test('offline agent shows Offline status text', async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_OFFLINE]);
      await expect(page.getByRole('table').getByText('Offline', { exact: true })).toBeVisible();
    });

    test('pending agent shows Pending status text', async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_PENDING]);
      await expect(page.getByRole('table').getByText('Pending', { exact: true })).toBeVisible();
    });

    test('PROXY badge is visible when external_proxy_port is non-zero', async ({
      page,
    }) => {
      await setupAgentPage(page, [MOCK_AGENT_WITH_PROXY]);
      await expect(page.getByRole('table').getByText('PROXY', { exact: true })).toBeVisible();
    });

    test('PROXY badge is absent when external_proxy_port is zero', async ({
      page,
    }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await expect(
        page.getByRole('table').getByText('PROXY'),
      ).not.toBeVisible();
    });
  });

  test.describe('Agent action buttons', () => {
    test('configure proxy button has accessible label containing agent name', async ({
      page,
    }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await expect(
        page.getByRole('button', {
          name: /configure proxy for online-agent/i,
        }),
      ).toBeVisible();
    });

    test('assign provider button has accessible label containing agent name', async ({
      page,
    }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await expect(
        page.getByRole('button', {
          name: /assign provider to online-agent/i,
        }),
      ).toBeVisible();
    });

    test('delete button has accessible label containing agent name', async ({
      page,
    }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await expect(
        page.getByRole('button', { name: /delete agent online-agent/i }),
      ).toBeVisible();
    });
  });

  test.describe('Delete agent dialog', () => {
    test('clicking delete opens a confirmation dialog', async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await page
        .getByRole('button', { name: /delete agent online-agent/i })
        .click();
      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible({ timeout: 5000 });
      await expect(dialog.getByRole('heading', { name: 'Delete Agent' })).toBeVisible();
    });

    test('delete dialog has Cancel and Delete buttons', async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await page
        .getByRole('button', { name: /delete agent online-agent/i })
        .click();
      const dialog = page.getByRole('dialog');
      await expect(dialog.getByRole('button', { name: /cancel/i })).toBeVisible(
        { timeout: 5000 },
      );
      await expect(
        dialog.getByRole('button', { name: /^delete$/i }),
      ).toBeVisible({ timeout: 5000 });
    });

    test('Cancel button dismisses the delete dialog', async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await page
        .getByRole('button', { name: /delete agent online-agent/i })
        .click();
      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible({ timeout: 5000 });
      await dialog.getByRole('button', { name: /cancel/i }).click();
      await expect(dialog).not.toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Provision agent dialog', () => {
    test('clicking Provision New Agent opens the dialog', async ({ page }) => {
      await setupAgentPage(page, []);
      await page
        .getByRole('button', { name: /provision new agent/i })
        .click();
      await expect(page.getByRole('dialog')).toBeVisible({ timeout: 5000 });
    });

    test('provision dialog has a required name input', async ({ page }) => {
      await setupAgentPage(page, []);
      await page
        .getByRole('button', { name: /provision new agent/i })
        .click();
      const dialog = page.getByRole('dialog');
      const nameInput = dialog.locator('#provision-name');
      await expect(nameInput).toBeVisible({ timeout: 5000 });
      await expect(nameInput).toHaveAttribute('aria-required', 'true');
    });
  });

  test.describe('Inline agent rename', () => {
    test('clicking agent name in the table shows a rename input', async ({
      page,
    }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await page.getByRole('button', { name: /rename agent online-agent/i }).click();
      await expect(
        page.getByRole('textbox', { name: /new name for online-agent/i }),
      ).toBeVisible({ timeout: 5000 });
    });

    test('rename input shows Save name and Cancel rename buttons', async ({
      page,
    }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await page.getByRole('button', { name: /rename agent online-agent/i }).click();
      await expect(
        page.getByRole('button', { name: 'Save name' }),
      ).toBeVisible({ timeout: 5000 });
      await expect(
        page.getByRole('button', { name: 'Cancel rename' }),
      ).toBeVisible({ timeout: 5000 });
    });

    test('Cancel rename exits rename mode', async ({ page }) => {
      await setupAgentPage(page, [MOCK_AGENT_ONLINE]);
      await page.getByRole('button', { name: /rename agent online-agent/i }).click();
      await expect(
        page.getByRole('button', { name: 'Cancel rename' }),
      ).toBeVisible({ timeout: 5000 });
      await page.getByRole('button', { name: 'Cancel rename' }).click();
      await expect(
        page.getByRole('textbox', { name: /new name for online-agent/i }),
      ).not.toBeVisible();
    });
  });
});
