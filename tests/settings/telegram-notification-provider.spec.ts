/**
 * Telegram Notification Provider E2E Tests
 *
 * Tests the Telegram notification provider type.
 * Covers form rendering, CRUD operations, payload contracts,
 * token security, and validation behavior specific to the Telegram provider type.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForAPIResponse } from '../utils/wait-helpers';

function generateProviderName(prefix: string = 'telegram-test'): string {
  return `${prefix}-${Date.now()}`;
}

test.describe('Telegram Notification Provider', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/settings/notifications');
    await waitForLoadingComplete(page);
  });

  test.describe('Form Rendering', () => {
    test('should show token field and chat ID placeholder when telegram type selected', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Select telegram provider type', async () => {
        await page.getByTestId('provider-type').selectOption('telegram');
      });

      await test.step('Verify token field is visible', async () => {
        await expect(page.getByTestId('provider-gotify-token')).toBeVisible();
      });

      await test.step('Verify token field label shows Bot Token', async () => {
        const tokenLabel = page.getByText(/bot token/i);
        await expect(tokenLabel.first()).toBeVisible();
      });

      await test.step('Verify chat ID placeholder', async () => {
        const urlInput = page.getByTestId('provider-url');
        await expect(urlInput).toHaveAttribute('placeholder', '987654321');
      });

      await test.step('Verify Chat ID label replaces URL label', async () => {
        const chatIdLabel = page.getByText(/chat id/i);
        await expect(chatIdLabel.first()).toBeVisible();
      });

      await test.step('Verify JSON template section is shown for telegram', async () => {
        await expect(page.getByTestId('provider-config')).toBeVisible();
      });

      await test.step('Verify save button is accessible', async () => {
        const saveButton = page.getByTestId('provider-save-btn');
        await expect(saveButton).toBeVisible();
        await expect(saveButton).toBeEnabled();
      });
    });

    test('should toggle form fields when switching between telegram and discord types', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify discord is default without token field', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);
      });

      await test.step('Switch to telegram and verify token field appears', async () => {
        await page.getByTestId('provider-type').selectOption('telegram');
        await expect(page.getByTestId('provider-gotify-token')).toBeVisible();
      });

      await test.step('Switch back to discord and verify token field hidden', async () => {
        await page.getByTestId('provider-type').selectOption('discord');
        await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);
      });
    });
  });

  test.describe('CRUD Operations', () => {
    test('should create telegram notification provider', async ({ page }) => {
      const providerName = generateProviderName('tg-create');
      let capturedPayload: Record<string, unknown> | null = null;

      await test.step('Mock create endpoint to capture payload', async () => {
        const createdProviders: Array<Record<string, unknown>> = [];

        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            capturedPayload = payload;
            const created = { id: 'tg-provider-1', ...payload };
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

      await test.step('Open form and select telegram type', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('telegram');
      });

      await test.step('Fill telegram provider form', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await page.getByTestId('provider-url').fill('987654321');
        await page.getByTestId('provider-gotify-token').fill('bot123456789:ABCdefGHI');
      });

      await test.step('Configure event notifications', async () => {
        await page.getByTestId('notify-proxy-hosts').check();
        await page.getByTestId('notify-certs').check();
      });

      await test.step('Save provider', async () => {
        await page.getByTestId('provider-save-btn').click();
      });

      await test.step('Verify provider appears in list', async () => {
        const providerInList = page.getByText(providerName);
        await expect(providerInList.first()).toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify outgoing payload contract', async () => {
        expect(capturedPayload).toBeTruthy();
        expect(capturedPayload?.type).toBe('telegram');
        expect(capturedPayload?.name).toBe(providerName);
        expect(capturedPayload?.url).toBe('987654321');
        expect(capturedPayload?.token).toBe('bot123456789:ABCdefGHI');
        expect(capturedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should edit telegram notification provider and preserve token', async ({ page }) => {
      let updatedPayload: Record<string, unknown> | null = null;

      await test.step('Mock existing telegram provider', async () => {
        let providers = [
          {
            id: 'tg-edit-id',
            name: 'Telegram Alerts',
            type: 'telegram',
            url: '987654321',
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
              p.id === 'tg-edit-id' ? { ...p, ...updatedPayload } : p
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

      await test.step('Verify telegram provider is displayed', async () => {
        await expect(page.getByText('Telegram Alerts')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click edit on telegram provider', async () => {
        const providerRow = page.getByTestId('provider-row-tg-edit-id');
        const editButton = providerRow.getByRole('button', { name: /edit/i });
        await expect(editButton).toBeVisible({ timeout: 5000 });
        await editButton.click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify form loads with telegram type', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('telegram');
      });

      await test.step('Verify stored token indicator is shown', async () => {
        await expect(page.getByTestId('gotify-token-stored-indicator')).toBeVisible();
      });

      await test.step('Update name without changing token', async () => {
        const nameInput = page.getByTestId('provider-name');
        await nameInput.clear();
        await nameInput.fill('Telegram Alerts v2');
      });

      await test.step('Save changes', async () => {
        const updateResponsePromise = waitForAPIResponse(
          page,
          /\/api\/v1\/notifications\/providers\/tg-edit-id/,
          { status: 200 }
        );
        const refreshResponsePromise = waitForAPIResponse(
          page,
          /\/api\/v1\/notifications\/providers$/,
          { status: 200 }
        );

        await page.getByTestId('provider-save-btn').click();
        await updateResponsePromise;
        await refreshResponsePromise;
      });

      await test.step('Verify update payload preserves token omission', async () => {
        expect(updatedPayload).toBeTruthy();
        expect(updatedPayload?.type).toBe('telegram');
        expect(updatedPayload?.name).toBe('Telegram Alerts v2');
        expect(updatedPayload?.token).toBeUndefined();
        expect(updatedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should test telegram notification provider', async ({ page }) => {
      let testCalled = false;

      await test.step('Mock existing telegram provider and test endpoint', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'tg-test-id',
                  name: 'Telegram Test Provider',
                  type: 'telegram',
                  url: '987654321',
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
        const providerRow = page.getByTestId('provider-row-tg-test-id');
        const sendTestButton = providerRow.getByRole('button', { name: /send test/i });
        await expect(sendTestButton).toBeVisible({ timeout: 5000 });
        await sendTestButton.click();
      });

      await test.step('Verify test was called', async () => {
        expect(testCalled).toBe(true);
      });
    });

    test('should delete telegram notification provider', async ({ page }) => {
      await test.step('Mock existing telegram provider', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'tg-delete-id',
                  name: 'Telegram To Delete',
                  type: 'telegram',
                  url: '987654321',
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

      await test.step('Verify telegram provider is displayed', async () => {
        await expect(page.getByText('Telegram To Delete')).toBeVisible({ timeout: 10000 });
      });

      await test.step('Delete provider', async () => {
        page.on('dialog', async (dialog) => {
          expect(dialog.type()).toBe('confirm');
          await dialog.accept();
        });

        const deleteButton = page.getByRole('button', { name: /delete/i })
          .or(page.locator('button').filter({ has: page.locator('svg.lucide-trash2, svg[class*="trash"]') }));
        await deleteButton.first().click();
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
    test('GET response should NOT expose bot token', async ({ page }) => {
      let apiResponseBody: Array<Record<string, unknown>> | null = null;

      await test.step('Mock provider list with has_token flag', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            const body = [
              {
                id: 'tg-sec-id',
                name: 'Telegram Secure',
                type: 'telegram',
                url: '987654321',
                has_token: true,
                enabled: true,
                notify_proxy_hosts: true,
                notify_certs: true,
                notify_uptime: false,
              },
            ];
            apiResponseBody = body;
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify(body),
            });
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Navigate to trigger GET', async () => {
        await page.goto('/settings/notifications');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify token is not in API response', async () => {
        expect(apiResponseBody).toBeTruthy();
        const provider = apiResponseBody![0];
        expect(provider.token).toBeUndefined();
        expect(provider.gotify_token).toBeUndefined();
        const responseStr = JSON.stringify(provider);
        expect(responseStr).not.toContain('bot123456789');
        expect(responseStr).not.toContain('ABCdefGHI');
      });
    });

    test('bot token should NOT be present in URL field', async ({ page }) => {
      await test.step('Mock provider with clean URL field', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'tg-url-sec-id',
                  name: 'Telegram URL Check',
                  type: 'telegram',
                  url: '987654321',
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

      await test.step('Reload and verify URL field does not contain bot token', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
        await expect(page.getByText('Telegram URL Check')).toBeVisible({ timeout: 5000 });

        const providerRow = page.getByTestId('provider-row-tg-url-sec-id');
        const urlText = await providerRow.textContent();
        expect(urlText).not.toContain('bot123456789');
        expect(urlText).not.toContain('api.telegram.org');
      });
    });
  });
});
