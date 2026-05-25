import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForLoadingComplete } from './utils/wait-helpers';

const REMOTE_SERVERS_API = '**/api/v1/remote-servers*';
const DOCKER_CONTAINERS_API = '**/api/v1/docker/containers*';

const MOCK_SERVER_UUID = 'orthrus-sv-aaaa-bbbb-cccc00000001';
const MOCK_SERVER_NAME = 'Orthrus Docker Server';
const MOCK_SERVER_HOST = '10.100.0.5';

/**
 * An Orthrus remote server that appears as a Docker source option in
 * ProxyHostForm. The form filters by provider === 'docker' && enabled.
 */
const MOCK_ORTHRUS_DOCKER_SERVER = {
  uuid: MOCK_SERVER_UUID,
  name: MOCK_SERVER_NAME,
  provider: 'docker',
  host: MOCK_SERVER_HOST,
  port: 2375,
  enabled: true,
  reachable: false,
  connection_type: 'orthrus',
  orthrus_agent_uuid: 'agent-uuid-aaaa-bbbb-cccc00000001',
};

const MOCK_CONTAINERS = [
  {
    id: 'container-abc123',
    names: ['/my-nginx'],
    image: 'nginx:latest',
    status: 'running',
    ports: [{ private_port: 80, public_port: 8080 }],
  },
  {
    id: 'container-def456',
    names: ['/my-api'],
    image: 'node:18-alpine',
    status: 'running',
    ports: [{ private_port: 3000, public_port: 3000 }],
  },
];

async function openProxyHostForm(page: import('@playwright/test').Page) {
  await page.goto('/proxy-hosts');
  await waitForLoadingComplete(page);
  await page.getByRole('button', { name: 'Add Proxy Host' }).first().click();
  const dialog = page.getByRole('dialog', { name: /add proxy host/i });
  await expect(dialog).toBeVisible({ timeout: 8000 });
  return dialog;
}

test.describe('Orthrus Docker Proxy Paths', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test.describe('ProxyHostForm Docker source panel', () => {
    test('Source dropdown includes Orthrus remote server when provider is docker', async ({
      page,
    }) => {
      await page.route(REMOTE_SERVERS_API, (route) =>
        route.fulfill({ json: [MOCK_ORTHRUS_DOCKER_SERVER] }),
      );

      const dialog = await openProxyHostForm(page);

      await test.step('Open Source dropdown', async () => {
        const sourceSelect = dialog.getByRole('combobox', { name: 'Source' });
        await expect(sourceSelect).toBeVisible({ timeout: 8000 });
        await sourceSelect.click();
      });

      await test.step('Verify Orthrus server appears as an option', async () => {
        await expect(
          page.getByRole('option', {
            name: new RegExp(MOCK_SERVER_NAME, 'i'),
          }),
        ).toBeVisible({ timeout: 5000 });
      });
    });

    test('servers without docker provider do not appear in Source dropdown', async ({
      page,
    }) => {
      const nonDockerServer = {
        ...MOCK_ORTHRUS_DOCKER_SERVER,
        uuid: 'non-docker-uuid-aabb',
        name: 'Non-Docker Orthrus Server',
        provider: 'orthrus',
      };
      await page.route(REMOTE_SERVERS_API, (route) =>
        route.fulfill({ json: [nonDockerServer] }),
      );

      const dialog = await openProxyHostForm(page);
      const sourceSelect = dialog.getByRole('combobox', { name: 'Source' });
      await expect(sourceSelect).toBeVisible({ timeout: 8000 });
      await sourceSelect.click();

      await expect(
        page.getByRole('option', { name: /Non-Docker Orthrus Server/i }),
      ).not.toBeVisible();
    });

    test('containers dropdown is disabled by default (Custom / Manual source)', async ({
      page,
    }) => {
      await page.route(REMOTE_SERVERS_API, (route) =>
        route.fulfill({ json: [MOCK_ORTHRUS_DOCKER_SERVER] }),
      );

      const dialog = await openProxyHostForm(page);
      const containersSelect = dialog.getByRole('combobox', {
        name: 'Containers',
      });
      await expect(containersSelect).toBeDisabled({ timeout: 8000 });
    });

    test('selecting Orthrus server as source triggers Docker containers API call', async ({
      page,
    }) => {
      let containersApiCalled = false;

      await page.route(REMOTE_SERVERS_API, (route) =>
        route.fulfill({ json: [MOCK_ORTHRUS_DOCKER_SERVER] }),
      );
      await page.route(DOCKER_CONTAINERS_API, (route) => {
        containersApiCalled = true;
        route.fulfill({ json: [] });
      });

      const dialog = await openProxyHostForm(page);

      await test.step('Select Orthrus server in Source dropdown', async () => {
        const sourceSelect = dialog.getByRole('combobox', { name: 'Source' });
        await expect(sourceSelect).toBeVisible({ timeout: 8000 });
        await sourceSelect.click();
        await page
          .getByRole('option', { name: new RegExp(MOCK_SERVER_NAME, 'i') })
          .click();
      });

      await expect
        .poll(() => containersApiCalled, {
          timeout: 8000,
          message: 'Expected Docker containers API to be called',
        })
        .toBe(true);
    });

    test('Docker containers API error shows Docker Connection Failed message', async ({
      page,
    }) => {
      await page.route(REMOTE_SERVERS_API, (route) =>
        route.fulfill({ json: [MOCK_ORTHRUS_DOCKER_SERVER] }),
      );
      await page.route(DOCKER_CONTAINERS_API, (route) =>
        route.fulfill({
          status: 503,
          body: 'Service Unavailable',
          headers: { 'Content-Type': 'text/plain' },
        }),
      );

      const dialog = await openProxyHostForm(page);

      await test.step('Select Orthrus server as Docker source', async () => {
        const sourceSelect = dialog.getByRole('combobox', { name: 'Source' });
        await expect(sourceSelect).toBeVisible({ timeout: 8000 });
        await sourceSelect.click();
        await page
          .getByRole('option', { name: new RegExp(MOCK_SERVER_NAME, 'i') })
          .click();
      });

      await test.step('Verify Docker Connection Failed error is displayed', async () => {
        await expect(dialog.getByText('Docker Connection Failed')).toBeVisible({
          timeout: 8000,
        });
      });
    });

    test('Docker containers API error shows troubleshooting guidance', async ({
      page,
    }) => {
      await page.route(REMOTE_SERVERS_API, (route) =>
        route.fulfill({ json: [MOCK_ORTHRUS_DOCKER_SERVER] }),
      );
      await page.route(DOCKER_CONTAINERS_API, (route) =>
        route.fulfill({
          status: 503,
          body: 'Service Unavailable',
          headers: { 'Content-Type': 'text/plain' },
        }),
      );

      const dialog = await openProxyHostForm(page);

      await test.step('Select Orthrus server as Docker source', async () => {
        const sourceSelect = dialog.getByRole('combobox', { name: 'Source' });
        await expect(sourceSelect).toBeVisible({ timeout: 8000 });
        await sourceSelect.click();
        await page
          .getByRole('option', { name: new RegExp(MOCK_SERVER_NAME, 'i') })
          .click();
      });

      await test.step('Verify troubleshooting section is present', async () => {
        await expect(
          dialog.getByText(/Docker Connection Failed/).first(),
        ).toBeVisible({ timeout: 8000 });
        await expect(dialog.getByText(/Troubleshooting/i)).toBeVisible({
          timeout: 5000,
        });
      });
    });

    test('Docker containers API success populates containers dropdown', async ({
      page,
    }) => {
      await page.route(REMOTE_SERVERS_API, (route) =>
        route.fulfill({ json: [MOCK_ORTHRUS_DOCKER_SERVER] }),
      );
      await page.route(DOCKER_CONTAINERS_API, (route) =>
        route.fulfill({ json: MOCK_CONTAINERS }),
      );

      const dialog = await openProxyHostForm(page);

      await test.step('Select Orthrus server as Docker source', async () => {
        const sourceSelect = dialog.getByRole('combobox', { name: 'Source' });
        await expect(sourceSelect).toBeVisible({ timeout: 8000 });
        await sourceSelect.click();
        await page
          .getByRole('option', { name: new RegExp(MOCK_SERVER_NAME, 'i') })
          .click();
      });

      await test.step('Containers dropdown becomes enabled after loading', async () => {
        const containersSelect = dialog.getByRole('combobox', {
          name: 'Containers',
        });
        await expect(containersSelect).not.toBeDisabled({ timeout: 8000 });
      });

      await test.step('Container options appear in the dropdown', async () => {
        const containersSelect = dialog.getByRole('combobox', {
          name: 'Containers',
        });
        await containersSelect.click();
        await expect(
          page.getByRole('option', { name: /my-nginx/i }),
        ).toBeVisible({ timeout: 5000 });
        await expect(
          page.getByRole('option', { name: /my-api/i }),
        ).toBeVisible({ timeout: 5000 });
      });
    });

    test('switching back to Custom / Manual disables containers dropdown', async ({
      page,
    }) => {
      await page.route(REMOTE_SERVERS_API, (route) =>
        route.fulfill({ json: [MOCK_ORTHRUS_DOCKER_SERVER] }),
      );
      await page.route(DOCKER_CONTAINERS_API, (route) =>
        route.fulfill({ json: MOCK_CONTAINERS }),
      );

      const dialog = await openProxyHostForm(page);
      const sourceSelect = dialog.getByRole('combobox', { name: 'Source' });

      await test.step('Select Orthrus server', async () => {
        await expect(sourceSelect).toBeVisible({ timeout: 8000 });
        await sourceSelect.click();
        await page
          .getByRole('option', { name: new RegExp(MOCK_SERVER_NAME, 'i') })
          .click();
      });

      await test.step('Switch back to Custom / Manual', async () => {
        await sourceSelect.click();
        await page
          .getByRole('option', { name: /custom \/ manual/i })
          .click();
      });

      await test.step('Containers dropdown is disabled again', async () => {
        const containersSelect = dialog.getByRole('combobox', {
          name: 'Containers',
        });
        await expect(containersSelect).toBeDisabled({ timeout: 5000 });
      });
    });

    test('Test Connection button is visible for Orthrus Docker Server type', async ({
      page,
    }) => {
      await page.route(REMOTE_SERVERS_API, (route) =>
        route.fulfill({ json: [MOCK_ORTHRUS_DOCKER_SERVER] }),
      );

      const dialog = await openProxyHostForm(page);

      await test.step('Select Orthrus server as Docker source', async () => {
        const sourceSelect = dialog.getByRole('combobox', { name: 'Source' });
        await expect(sourceSelect).toBeVisible({ timeout: 8000 });
        await sourceSelect.click();
        await page
          .getByRole('option', { name: new RegExp(MOCK_SERVER_NAME, 'i') })
          .click();
      });

      await test.step('Verify Test Connection button is accessible', async () => {
        await expect(
          page.getByRole('button', { name: /test connection/i }),
        ).toBeVisible();
      });
    });
  });
});
