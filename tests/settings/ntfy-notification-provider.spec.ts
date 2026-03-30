/**
 * Ntfy Notification Provider E2E Tests
 *
 * Tests the Ntfy notification provider type.
 * Covers form rendering, CRUD operations, payload contracts,
 * token security, and validation behavior specific to the Ntfy provider type.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

function generateProviderName(prefix: string = 'ntfy-test'): string {
  return `${prefix}-${Date.now()}`;
}

test.describe('Ntfy Notification Provider', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/settings/notifications');
    await waitForLoadingComplete(page);
  });

  test.describe('Form Rendering', () => {
    test('should show token field and topic URL placeholder when ntfy type selected', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Select ntfy provider type', async () => {
        await page.getByTestId('provider-type').selectOption('ntfy');
      });

      await test.step('Verify token field is visible', async () => {
        await expect(page.getByTestId('provider-gotify-token')).toBeVisible();
      });

      await test.step('Verify token field label shows Access Token (optional)', async () => {
        const tokenLabel = page.getByText(/access token.*optional/i);
        await expect(tokenLabel.first()).toBeVisible();
      });

      await test.step('Verify topic URL placeholder', async () => {
        const urlInput = page.getByTestId('provider-url');
        await expect(urlInput).toHaveAttribute('placeholder', 'https://ntfy.sh/my-topic');
      });

      await test.step('Verify JSON template section is shown for ntfy', async () => {
        await expect(page.getByTestId('provider-config')).toBeVisible();
      });

      await test.step('Verify save button is accessible', async () => {
        const saveButton = page.getByTestId('provider-save-btn');
        await expect(saveButton).toBeVisible();
        await expect(saveButton).toBeEnabled();
      });
    });

    test('should toggle form fields correctly when switching between ntfy and discord', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify discord is default without token field', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);
      });

      await test.step('Switch to ntfy and verify token field appears', async () => {
        await page.getByTestId('provider-type').selectOption('ntfy');
        await expect(page.getByTestId('provider-gotify-token')).toBeVisible();
      });

      await test.step('Switch back to discord and verify token field hidden', async () => {
        await page.getByTestId('provider-type').selectOption('discord');
        await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);
      });
    });

    test('should show JSON template section for ntfy', async ({ page }) => {
      await test.step('Open Add Provider form and select ntfy', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('ntfy');
      });

      await test.step('Verify JSON template config section is visible', async () => {
        await expect(page.getByTestId('provider-config')).toBeVisible();
      });
    });
  });

  test.describe('CRUD Operations', () => {
    test('should create an ntfy notification provider with URL and token', async ({ page }) => {
      const providerName = generateProviderName('ntfy-create');
      let capturedPayload: Record<string, unknown> | null = null;

      await test.step('Mock create endpoint to capture payload', async () => {
        const createdProviders: Array<Record<string, unknown>> = [];

        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            capturedPayload = payload;
            const { token, gotify_token, ...rest } = payload;
            const created: Record<string, unknown> = {
              id: 'ntfy-provider-1',
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

      await test.step('Open form and select ntfy type', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('ntfy');
      });

      await test.step('Fill ntfy provider form with URL and token', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await page.getByTestId('provider-url').fill('https://ntfy.sh/my-topic');
        await page.getByTestId('provider-gotify-token').fill('tk_abc123xyz789');
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
        expect(capturedPayload?.type).toBe('ntfy');
        expect(capturedPayload?.name).toBe(providerName);
        expect(capturedPayload?.url).toBe('https://ntfy.sh/my-topic');
        expect(capturedPayload?.token).toBe('tk_abc123xyz789');
        expect(capturedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should create an ntfy notification provider with URL only (no token)', async ({ page }) => {
      const providerName = generateProviderName('ntfy-notoken');
      let capturedPayload: Record<string, unknown> | null = null;

      await test.step('Mock create endpoint to capture payload', async () => {
        const createdProviders: Array<Record<string, unknown>> = [];

        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            capturedPayload = payload;
            const { token, gotify_token, ...rest } = payload;
            const created: Record<string, unknown> = {
              id: 'ntfy-notoken-1',
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

      await test.step('Open form and select ntfy type', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('ntfy');
      });

      await test.step('Fill ntfy provider form with URL only', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await page.getByTestId('provider-url').fill('https://ntfy.sh/public-topic');
      });

      await test.step('Configure event notifications', async () => {
        await page.getByTestId('notify-proxy-hosts').check();
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

      await test.step('Verify outgoing payload has no token', async () => {
        expect(capturedPayload).toBeTruthy();
        expect(capturedPayload?.type).toBe('ntfy');
        expect(capturedPayload?.name).toBe(providerName);
        expect(capturedPayload?.url).toBe('https://ntfy.sh/public-topic');
        expect(capturedPayload?.token).toBeUndefined();
        expect(capturedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should edit ntfy provider and preserve token when token field left blank', async ({ page }) => {
      let updatedPayload: Record<string, unknown> | null = null;

      await test.step('Mock existing ntfy provider', async () => {
        let providers = [
          {
            id: 'ntfy-edit-id',
            name: 'Ntfy Alerts',
            type: 'ntfy',
            url: 'https://ntfy.sh/my-topic',
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
              p.id === 'ntfy-edit-id' ? { ...p, ...updatedPayload } : p
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

      await test.step('Verify ntfy provider is displayed', async () => {
        await expect(page.getByText('Ntfy Alerts')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click edit on ntfy provider', async () => {
        const providerRow = page.getByTestId('provider-row-ntfy-edit-id');
        const editButton = providerRow.getByRole('button', { name: /edit/i });
        await expect(editButton).toBeVisible({ timeout: 5000 });
        await editButton.click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify form loads with ntfy type', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('ntfy');
      });

      await test.step('Verify stored token indicator is shown', async () => {
        await expect(page.getByTestId('gotify-token-stored-indicator')).toBeVisible();
      });

      await test.step('Update name without changing token', async () => {
        const nameInput = page.getByTestId('provider-name');
        await nameInput.clear();
        await nameInput.fill('Ntfy Alerts v2');
      });

      await test.step('Save changes', async () => {
        await Promise.all([
          page.waitForResponse(
            (resp) =>
              /\/api\/v1\/notifications\/providers\/ntfy-edit-id/.test(resp.url()) &&
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
        expect(updatedPayload?.type).toBe('ntfy');
        expect(updatedPayload?.name).toBe('Ntfy Alerts v2');
        expect(updatedPayload?.token).toBeUndefined();
        expect(updatedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should test an ntfy notification provider', async ({ page }) => {
      let testCalled = false;

      await test.step('Mock existing ntfy provider and test endpoint', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'ntfy-test-id',
                  name: 'Ntfy Test Provider',
                  type: 'ntfy',
                  url: 'https://ntfy.sh/my-topic',
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
        const providerRow = page.getByTestId('provider-row-ntfy-test-id');
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

    test('should delete an ntfy notification provider', async ({ page }) => {
      await test.step('Mock existing ntfy provider', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'ntfy-delete-id',
                  name: 'Ntfy To Delete',
                  type: 'ntfy',
                  url: 'https://ntfy.sh/my-topic',
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

      await test.step('Verify ntfy provider is displayed', async () => {
        await expect(page.getByText('Ntfy To Delete')).toBeVisible({ timeout: 10000 });
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
              resp.url().includes('/api/v1/notifications/providers/ntfy-delete-id') &&
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
    test('GET response should NOT expose the access token value', async ({ page }) => {
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
                id: 'ntfy-sec-id',
                name: 'Ntfy Secure',
                type: 'ntfy',
                url: 'https://ntfy.sh/my-topic',
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

      await test.step('Verify access token is not in API response', async () => {
        expect(apiResponseBody).toBeTruthy();
        const provider = apiResponseBody![0];
        expect(provider.token).toBeUndefined();
        expect(provider.gotify_token).toBeUndefined();
        const responseStr = JSON.stringify(provider);
        expect(responseStr).not.toContain('tk_abc123xyz789');
      });
    });

    test('access token should not appear in the url field or any visible field', async ({ page }) => {
      await test.step('Mock provider with clean URL field', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'ntfy-url-sec-id',
                  name: 'Ntfy URL Check',
                  type: 'ntfy',
                  url: 'https://ntfy.sh/my-topic',
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

      await test.step('Reload and verify access token does not appear in provider row', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
        await expect(page.getByText('Ntfy URL Check')).toBeVisible({ timeout: 5000 });

        const providerRow = page.getByTestId('provider-row-ntfy-url-sec-id');
        const urlText = await providerRow.textContent();
        expect(urlText).not.toContain('tk_abc123xyz789');
      });
    });
  });

  test.describe('Payload Contract', () => {
    test('POST body should include type=ntfy, url field = topic URL, token field is write-only', async ({ page }) => {
      const providerName = generateProviderName('ntfy-contract');
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
              id: 'ntfy-contract-1',
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

      await test.step('Create an ntfy provider via the UI', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('ntfy');
        await page.getByTestId('provider-name').fill(providerName);
        await page.getByTestId('provider-url').fill('https://ntfy.sh/my-topic');
        await page.getByTestId('provider-gotify-token').fill('tk_abc123xyz789');

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

      await test.step('Verify POST payload: type=ntfy, url=topic URL, token=access token', async () => {
        expect(capturedPayload).toBeTruthy();
        expect(capturedPayload?.type).toBe('ntfy');
        expect(capturedPayload?.url).toBe('https://ntfy.sh/my-topic');
        expect(capturedPayload?.token).toBe('tk_abc123xyz789');
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
        expect(responseStr).not.toContain('tk_abc123xyz789');
      });
    });
  });
});
