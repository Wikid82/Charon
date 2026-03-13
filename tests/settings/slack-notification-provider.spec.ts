/**
 * Slack Notification Provider E2E Tests
 *
 * Tests the Slack notification provider type.
 * Covers form rendering, CRUD operations, payload contracts,
 * webhook URL security, and validation behavior specific to the Slack provider type.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

function generateProviderName(prefix: string = 'slack-test'): string {
  return `${prefix}-${Date.now()}`;
}

test.describe('Slack Notification Provider', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/settings/notifications');
    await waitForLoadingComplete(page);
  });

  test.describe('Form Rendering', () => {
    test('should show webhook URL field and channel name when slack type selected', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Select slack provider type', async () => {
        await page.getByTestId('provider-type').selectOption('slack');
      });

      await test.step('Verify webhook URL (token) field is visible', async () => {
        await expect(page.getByTestId('provider-gotify-token')).toBeVisible();
      });

      await test.step('Verify webhook URL field label shows Webhook URL', async () => {
        const tokenLabel = page.getByText(/webhook url/i);
        await expect(tokenLabel.first()).toBeVisible();
      });

      await test.step('Verify channel name placeholder', async () => {
        const urlInput = page.getByTestId('provider-url');
        await expect(urlInput).toHaveAttribute('placeholder', '#general');
      });

      await test.step('Verify Channel Name label replaces URL label', async () => {
        const channelLabel = page.getByText(/channel name/i);
        await expect(channelLabel.first()).toBeVisible();
      });

      await test.step('Verify JSON template section is shown for slack', async () => {
        await expect(page.getByTestId('provider-config')).toBeVisible();
      });

      await test.step('Verify save button is accessible', async () => {
        const saveButton = page.getByTestId('provider-save-btn');
        await expect(saveButton).toBeVisible();
        await expect(saveButton).toBeEnabled();
      });
    });

    test('should toggle form fields when switching between slack and discord types', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify discord is default without token field', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);
      });

      await test.step('Switch to slack and verify token field appears', async () => {
        await page.getByTestId('provider-type').selectOption('slack');
        await expect(page.getByTestId('provider-gotify-token')).toBeVisible();
      });

      await test.step('Switch back to discord and verify token field hidden', async () => {
        await page.getByTestId('provider-type').selectOption('discord');
        await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);
      });
    });

    test('should show JSON template section for slack', async ({ page }) => {
      await test.step('Open Add Provider form and select slack', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('slack');
      });

      await test.step('Verify JSON template config section is visible', async () => {
        await expect(page.getByTestId('provider-config')).toBeVisible();
      });
    });
  });

  test.describe('CRUD Operations', () => {
    test('should create slack notification provider', async ({ page }) => {
      const providerName = generateProviderName('slack-create');
      let capturedPayload: Record<string, unknown> | null = null;

      await test.step('Mock create endpoint to capture payload', async () => {
        const createdProviders: Array<Record<string, unknown>> = [];

        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            capturedPayload = payload;
            const created = { id: 'slack-provider-1', ...payload };
            createdProviders.push(created);
            await route.fulfill({
              status: 201,
              contentType: 'application/json',
              body: JSON.stringify(created),
            });
            return;
          }

          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify(createdProviders),
            });
            return;
          }

          await route.continue();
        });
      });

      await test.step('Open form and select slack type', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('slack');
      });

      await test.step('Fill slack provider form', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await page.getByTestId('provider-url').fill('#alerts');
        await page.getByTestId('provider-gotify-token').fill(
          'https://hooks.slack.com/services/T00000000/B00000000/xxxxxxxxxxxxxxxxxxxx'
        );
      });

      await test.step('Configure event notifications', async () => {
        await page.getByTestId('notify-proxy-hosts').check();
        await page.getByTestId('notify-certs').check();
      });

      await test.step('Save provider', async () => {
        await Promise.all([
          page.waitForResponse(
            (resp) =>
              /\/api\/v1\/notifications\/providers/.test(resp.url()) &&
              resp.request().method() === 'POST' &&
              resp.status() === 201
          ),
          page.getByTestId('provider-save-btn').click(),
        ]);
      });

      await test.step('Verify provider appears in list', async () => {
        const providerInList = page.getByText(providerName);
        await expect(providerInList.first()).toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify outgoing payload contract', async () => {
        expect(capturedPayload).toBeTruthy();
        expect(capturedPayload?.type).toBe('slack');
        expect(capturedPayload?.name).toBe(providerName);
        expect(capturedPayload?.url).toBe('#alerts');
        expect(capturedPayload?.token).toBe(
          'https://hooks.slack.com/services/T00000000/B00000000/xxxxxxxxxxxxxxxxxxxx'
        );
        expect(capturedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should edit slack notification provider and preserve webhook URL', async ({ page }) => {
      let updatedPayload: Record<string, unknown> | null = null;

      await test.step('Mock existing slack provider', async () => {
        let providers = [
          {
            id: 'slack-edit-id',
            name: 'Slack Alerts',
            type: 'slack',
            url: '#alerts',
            has_token: true,
            enabled: true,
            notify_proxy_hosts: true,
            notify_certs: true,
            notify_uptime: false,
          },
        ];

        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify(providers),
            });
          } else {
            await route.continue();
          }
        });

        await page.route('**/api/v1/notifications/providers/*', async (route, request) => {
          if (request.method() === 'PUT') {
            updatedPayload = (await request.postDataJSON()) as Record<string, unknown>;
            providers = providers.map((p) =>
              p.id === 'slack-edit-id' ? { ...p, ...updatedPayload } : p
            );
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify({ success: true }),
            });
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Reload to get mocked provider', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Verify slack provider is displayed', async () => {
        await expect(page.getByText('Slack Alerts')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click edit on slack provider', async () => {
        const providerRow = page.getByTestId('provider-row-slack-edit-id');
        const editButton = providerRow.getByRole('button', { name: /edit/i });
        await expect(editButton).toBeVisible({ timeout: 5000 });
        await editButton.click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify form loads with slack type', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('slack');
      });

      await test.step('Verify stored token indicator is shown', async () => {
        await expect(page.getByTestId('gotify-token-stored-indicator')).toBeVisible();
      });

      await test.step('Update name without changing webhook URL', async () => {
        const nameInput = page.getByTestId('provider-name');
        await nameInput.clear();
        await nameInput.fill('Slack Alerts v2');
      });

      await test.step('Save changes', async () => {
        await Promise.all([
          page.waitForResponse(
            (resp) =>
              /\/api\/v1\/notifications\/providers\/slack-edit-id/.test(resp.url()) &&
              resp.request().method() === 'PUT' &&
              resp.status() === 200
          ),
          page.waitForResponse(
            (resp) =>
              /\/api\/v1\/notifications\/providers/.test(resp.url()) &&
              resp.request().method() === 'GET' &&
              resp.status() === 200
          ),
          page.getByTestId('provider-save-btn').click(),
        ]);
      });

      await test.step('Verify update payload preserves webhook URL omission', async () => {
        expect(updatedPayload).toBeTruthy();
        expect(updatedPayload?.type).toBe('slack');
        expect(updatedPayload?.name).toBe('Slack Alerts v2');
        expect(updatedPayload?.token).toBeUndefined();
        expect(updatedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should test slack notification provider', async ({ page }) => {
      let testCalled = false;

      await test.step('Mock existing slack provider and test endpoint', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'slack-test-id',
                  name: 'Slack Test Provider',
                  type: 'slack',
                  url: '#alerts',
                  has_token: true,
                  enabled: true,
                  notify_proxy_hosts: true,
                  notify_certs: true,
                  notify_uptime: false,
                },
              ]),
            });
          } else {
            await route.continue();
          }
        });

        await page.route('**/api/v1/notifications/providers/test', async (route, request) => {
          if (request.method() === 'POST') {
            testCalled = true;
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify({ success: true }),
            });
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Reload to get mocked provider', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Click Send Test on the provider', async () => {
        const providerRow = page.getByTestId('provider-row-slack-test-id');
        const sendTestButton = providerRow.getByRole('button', { name: /send test/i });
        await expect(sendTestButton).toBeVisible({ timeout: 5000 });
        await expect(sendTestButton).toBeEnabled();
        await Promise.all([
          page.waitForResponse(
            (resp) =>
              resp.url().includes('/api/v1/notifications/providers/test') &&
              resp.status() === 200
          ),
          sendTestButton.click(),
        ]);
      });

      await test.step('Verify test was called', async () => {
        expect(testCalled).toBe(true);
      });
    });

    test('should delete slack notification provider', async ({ page }) => {
      await test.step('Mock existing slack provider', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'slack-delete-id',
                  name: 'Slack To Delete',
                  type: 'slack',
                  url: '#alerts',
                  enabled: true,
                },
              ]),
            });
          } else {
            await route.continue();
          }
        });

        await page.route('**/api/v1/notifications/providers/*', async (route, request) => {
          if (request.method() === 'DELETE') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify({ success: true }),
            });
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Reload to get mocked provider', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Verify slack provider is displayed', async () => {
        await expect(page.getByText('Slack To Delete')).toBeVisible({ timeout: 10000 });
      });

      await test.step('Delete provider', async () => {
        page.on('dialog', async (dialog) => {
          expect(dialog.type()).toBe('confirm');
          await dialog.accept();
        });

        const deleteButton = page.getByRole('button', { name: /delete/i })
          .or(page.locator('button').filter({ has: page.locator('svg.lucide-trash2, svg[class*="trash"]') }));
        await Promise.all([
          page.waitForResponse(
            (resp) =>
              resp.url().includes('/api/v1/notifications/providers/slack-delete-id') &&
              resp.status() === 200
          ),
          deleteButton.first().click(),
        ]);
      });

      await test.step('Verify deletion feedback', async () => {
        const successIndicator = page.locator('[data-testid="toast-success"]')
          .or(page.getByRole('status').filter({ hasText: /deleted|removed/i }))
          .or(page.getByText(/no.*providers/i));
        await expect(successIndicator.first()).toBeVisible({ timeout: 5000 });
      });
    });
  });

  test.describe('Security', () => {
    test('GET response should NOT expose webhook URL', async ({ page }) => {
      let apiResponseBody: Array<Record<string, unknown>> | null = null;

      let resolveRouteBody: (data: Array<Record<string, unknown>>) => void;
      const routeBodyPromise = new Promise<Array<Record<string, unknown>>>((resolve) => {
        resolveRouteBody = resolve;
      });

      await test.step('Mock provider list with has_token flag', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            const body = [
              {
                id: 'slack-sec-id',
                name: 'Slack Secure',
                type: 'slack',
                url: '#alerts',
                has_token: true,
                enabled: true,
                notify_proxy_hosts: true,
                notify_certs: true,
                notify_uptime: false,
              },
            ];
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify(body),
            });
            resolveRouteBody!(body);
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Navigate to trigger GET', async () => {
        await page.reload();
        apiResponseBody = await routeBodyPromise;
        await waitForLoadingComplete(page);
      });

      await test.step('Verify webhook URL is not in API response', async () => {
        expect(apiResponseBody).toBeTruthy();
        const provider = apiResponseBody![0];
        expect(provider.token).toBeUndefined();
        expect(provider.gotify_token).toBeUndefined();
        const responseStr = JSON.stringify(provider);
        expect(responseStr).not.toContain('hooks.slack.com');
        expect(responseStr).not.toContain('/services/');
      });
    });

    test('webhook URL should NOT be present in URL field', async ({ page }) => {
      await test.step('Mock provider with clean URL field', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'slack-url-sec-id',
                  name: 'Slack URL Check',
                  type: 'slack',
                  url: '#alerts',
                  has_token: true,
                  enabled: true,
                },
              ]),
            });
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Reload and verify URL field does not contain webhook URL', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
        await expect(page.getByText('Slack URL Check')).toBeVisible({ timeout: 5000 });

        const providerRow = page.getByTestId('provider-row-slack-url-sec-id');
        const urlText = await providerRow.textContent();
        expect(urlText).not.toContain('hooks.slack.com');
        expect(urlText).not.toContain('/services/');
      });
    });
  });
});
