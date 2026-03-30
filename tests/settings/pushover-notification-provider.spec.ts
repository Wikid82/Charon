/**
 * Pushover Notification Provider E2E Tests
 *
 * Tests the Pushover notification provider type.
 * Covers form rendering, CRUD operations, payload contracts,
 * token security, and validation behavior specific to the Pushover provider type.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

function generateProviderName(prefix: string = 'pushover-test'): string {
  return `${prefix}-${Date.now()}`;
}

test.describe('Pushover Notification Provider', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/settings/notifications');
    await waitForLoadingComplete(page);
  });

  test.describe('Form Rendering', () => {
    test('should show API token field and user key placeholder when pushover type selected', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Select pushover provider type', async () => {
        await page.getByTestId('provider-type').selectOption('pushover');
      });

      await test.step('Verify API token field is visible', async () => {
        await expect(page.getByTestId('provider-gotify-token')).toBeVisible();
      });

      await test.step('Verify token field label shows API Token (Application)', async () => {
        const tokenLabel = page.getByText(/api token.*application/i);
        await expect(tokenLabel.first()).toBeVisible();
      });

      await test.step('Verify user key placeholder', async () => {
        const urlInput = page.getByTestId('provider-url');
        await expect(urlInput).toHaveAttribute('placeholder', 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG');
      });

      await test.step('Verify User Key label replaces URL label', async () => {
        const userKeyLabel = page.getByText(/user key/i);
        await expect(userKeyLabel.first()).toBeVisible();
      });

      await test.step('Verify JSON template section is shown for pushover', async () => {
        await expect(page.getByTestId('provider-config')).toBeVisible();
      });

      await test.step('Verify save button is accessible', async () => {
        const saveButton = page.getByTestId('provider-save-btn');
        await expect(saveButton).toBeVisible();
        await expect(saveButton).toBeEnabled();
      });
    });

    test('should toggle form fields correctly when switching between pushover and discord', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify discord is default without token field', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);
      });

      await test.step('Switch to pushover and verify token field appears', async () => {
        await page.getByTestId('provider-type').selectOption('pushover');
        await expect(page.getByTestId('provider-gotify-token')).toBeVisible();
      });

      await test.step('Switch back to discord and verify token field hidden', async () => {
        await page.getByTestId('provider-type').selectOption('discord');
        await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);
      });
    });

    test('should show JSON template section for pushover', async ({ page }) => {
      await test.step('Open Add Provider form and select pushover', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('pushover');
      });

      await test.step('Verify JSON template config section is visible', async () => {
        await expect(page.getByTestId('provider-config')).toBeVisible();
      });
    });
  });

  test.describe('CRUD Operations', () => {
    test('should create a pushover notification provider', async ({ page }) => {
      const providerName = generateProviderName('po-create');
      let capturedPayload: Record<string, unknown> | null = null;

      await test.step('Mock create endpoint to capture payload', async () => {
        const createdProviders: Array<Record<string, unknown>> = [];

        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            capturedPayload = payload;
            const { token, gotify_token, ...rest } = payload;
            const created: Record<string, unknown> = {
              id: 'po-provider-1',
              ...rest,
              ...(token !== undefined || gotify_token !== undefined ? { has_token: true } : {}),
            };
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

      await test.step('Open form and select pushover type', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('pushover');
      });

      await test.step('Fill pushover provider form', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await page.getByTestId('provider-url').fill('uQiRzpo4DXghDmr9QzzfQu27cmVRsG');
        await page.getByTestId('provider-gotify-token').fill('azGDORePK8gMaC0QOYAMyEEuzJnyUi');
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
        expect(capturedPayload?.type).toBe('pushover');
        expect(capturedPayload?.name).toBe(providerName);
        expect(capturedPayload?.url).toBe('uQiRzpo4DXghDmr9QzzfQu27cmVRsG');
        expect(capturedPayload?.token).toBe('azGDORePK8gMaC0QOYAMyEEuzJnyUi');
        expect(capturedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should edit provider and preserve token when token field left blank', async ({ page }) => {
      let updatedPayload: Record<string, unknown> | null = null;

      await test.step('Mock existing pushover provider', async () => {
        let providers = [
          {
            id: 'po-edit-id',
            name: 'Pushover Alerts',
            type: 'pushover',
            url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG',
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
              p.id === 'po-edit-id' ? { ...p, ...updatedPayload } : p
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

      await test.step('Verify pushover provider is displayed', async () => {
        await expect(page.getByText('Pushover Alerts')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click edit on pushover provider', async () => {
        const providerRow = page.getByTestId('provider-row-po-edit-id');
        const editButton = providerRow.getByRole('button', { name: /edit/i });
        await expect(editButton).toBeVisible({ timeout: 5000 });
        await editButton.click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify form loads with pushover type', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('pushover');
      });

      await test.step('Verify stored token indicator is shown', async () => {
        await expect(page.getByTestId('gotify-token-stored-indicator')).toBeVisible();
      });

      await test.step('Update name without changing token', async () => {
        const nameInput = page.getByTestId('provider-name');
        await nameInput.clear();
        await nameInput.fill('Pushover Alerts v2');
      });

      await test.step('Save changes', async () => {
        await Promise.all([
          page.waitForResponse(
            (resp) =>
              /\/api\/v1\/notifications\/providers\/po-edit-id/.test(resp.url()) &&
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

      await test.step('Verify update payload preserves token omission', async () => {
        expect(updatedPayload).toBeTruthy();
        expect(updatedPayload?.type).toBe('pushover');
        expect(updatedPayload?.name).toBe('Pushover Alerts v2');
        expect(updatedPayload?.token).toBeUndefined();
        expect(updatedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should test a pushover notification provider', async ({ page }) => {
      let testCalled = false;

      await test.step('Mock existing pushover provider and test endpoint', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'po-test-id',
                  name: 'Pushover Test Provider',
                  type: 'pushover',
                  url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG',
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
        const providerRow = page.getByTestId('provider-row-po-test-id');
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

    test('should delete a pushover notification provider', async ({ page }) => {
      await test.step('Mock existing pushover provider', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'po-delete-id',
                  name: 'Pushover To Delete',
                  type: 'pushover',
                  url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG',
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

      await test.step('Verify pushover provider is displayed', async () => {
        await expect(page.getByText('Pushover To Delete')).toBeVisible({ timeout: 10000 });
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
              resp.url().includes('/api/v1/notifications/providers/po-delete-id') &&
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
    test('GET response should NOT expose the API token value', async ({ page }) => {
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
                id: 'po-sec-id',
                name: 'Pushover Secure',
                type: 'pushover',
                url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG',
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
        apiResponseBody = await Promise.race([
          routeBodyPromise,
          new Promise<Array<Record<string, unknown>>>((_resolve, reject) =>
            setTimeout(
              () => reject(new Error('Timed out waiting for GET /api/v1/notifications/providers')),
              15000
            )
          ),
        ]);
        await waitForLoadingComplete(page);
      });

      await test.step('Verify API token is not in API response', async () => {
        expect(apiResponseBody).toBeTruthy();
        const provider = apiResponseBody![0];
        expect(provider.token).toBeUndefined();
        expect(provider.gotify_token).toBeUndefined();
        const responseStr = JSON.stringify(provider);
        expect(responseStr).not.toContain('azGDORePK8gMaC0QOYAMyEEuzJnyUi');
      });
    });

    test('API token should not appear in the url field or any visible field', async ({ page }) => {
      await test.step('Mock provider with clean URL field', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'po-url-sec-id',
                  name: 'Pushover URL Check',
                  type: 'pushover',
                  url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG',
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

      await test.step('Reload and verify API token does not appear in provider row', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
        await expect(page.getByText('Pushover URL Check')).toBeVisible({ timeout: 5000 });

        const providerRow = page.getByTestId('provider-row-po-url-sec-id');
        const urlText = await providerRow.textContent();
        expect(urlText).not.toContain('azGDORePK8gMaC0QOYAMyEEuzJnyUi');
        expect(urlText).not.toContain('api.pushover.net');
      });
    });
  });

  test.describe('Payload Contract', () => {
    test('POST body should include type=pushover, url field = user key, token field is write-only', async ({ page }) => {
      const providerName = generateProviderName('po-contract');
      let capturedPayload: Record<string, unknown> | null = null;
      let capturedGetResponse: Array<Record<string, unknown>> | null = null;

      await test.step('Mock create and list endpoints', async () => {
        const createdProviders: Array<Record<string, unknown>> = [];

        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            capturedPayload = payload;
            const { token, gotify_token, ...rest } = payload;
            const created: Record<string, unknown> = {
              id: 'po-contract-1',
              ...rest,
              has_token: !!(token || gotify_token),
            };
            createdProviders.push(created);
            await route.fulfill({
              status: 201,
              contentType: 'application/json',
              body: JSON.stringify(created),
            });
            return;
          }

          if (request.method() === 'GET') {
            capturedGetResponse = [...createdProviders];
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

      await test.step('Create a pushover provider via the UI', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('pushover');
        await page.getByTestId('provider-name').fill(providerName);
        await page.getByTestId('provider-url').fill('uQiRzpo4DXghDmr9QzzfQu27cmVRsG');
        await page.getByTestId('provider-gotify-token').fill('azGDORePK8gMaC0QOYAMyEEuzJnyUi');

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

      await test.step('Verify POST payload: type=pushover, url=user key, token=api token', async () => {
        expect(capturedPayload).toBeTruthy();
        expect(capturedPayload?.type).toBe('pushover');
        expect(capturedPayload?.url).toBe('uQiRzpo4DXghDmr9QzzfQu27cmVRsG');
        expect(capturedPayload?.token).toBe('azGDORePK8gMaC0QOYAMyEEuzJnyUi');
        expect(capturedPayload?.gotify_token).toBeUndefined();
      });

      await test.step('Verify GET response: has_token=true, token value absent', async () => {
        await expect(page.getByText(providerName).first()).toBeVisible({ timeout: 10000 });
        expect(capturedGetResponse).toBeTruthy();
        const provider = capturedGetResponse![0];
        expect(provider.has_token).toBe(true);
        expect(provider.token).toBeUndefined();
        expect(provider.gotify_token).toBeUndefined();
        const responseStr = JSON.stringify(provider);
        expect(responseStr).not.toContain('azGDORePK8gMaC0QOYAMyEEuzJnyUi');
      });
    });
  });
});
