/**
 * Email Notification Provider E2E Tests
 *
 * Tests the email notification provider type added in PR #800.
 * Covers form rendering, CRUD operations, payload contracts,
 * and validation behavior specific to the email provider type.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForAPIResponse } from '../utils/wait-helpers';

function generateProviderName(prefix: string = 'email-test'): string {
  return `${prefix}-${Date.now()}`;
}

test.describe('Email Notification Provider', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/settings/notifications');
    await waitForLoadingComplete(page);
  });

  test.describe('Form Rendering', () => {
    test('should show recipients field and hide URL/token when email type selected', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Select email provider type', async () => {
        await page.getByTestId('provider-type').selectOption('email');
      });

      await test.step('Verify recipients label replaces URL label', async () => {
        const recipientsLabel = page.getByText(/recipients/i);
        await expect(recipientsLabel.first()).toBeVisible();
      });

      await test.step('Verify recipients help text is shown', async () => {
        const helpText = page.locator('#email-recipients-help');
        await expect(helpText).toBeVisible();
      });

      await test.step('Verify recipients placeholder', async () => {
        const urlInput = page.getByTestId('provider-url');
        await expect(urlInput).toHaveAttribute('placeholder', /user@example\.com/);
      });

      await test.step('Verify Gotify token field is hidden', async () => {
        await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);
      });

      await test.step('Verify JSON template section is hidden for email', async () => {
        await expect(page.getByTestId('provider-config')).toHaveCount(0);
      });

      await test.step('Verify SMTP notice is shown', async () => {
        const smtpNotice = page.getByRole('note');
        await expect(smtpNotice).toBeVisible();
        await expect(smtpNotice).toContainText(/smtp/i);
      });

      await test.step('Verify save button is accessible', async () => {
        const saveButton = page.getByTestId('provider-save-btn');
        await expect(saveButton).toBeVisible();
        await expect(saveButton).toBeEnabled();
      });
    });

    test('should toggle form fields when switching between email and discord types', async ({ page }) => {
      await test.step('Open Add Provider form', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify discord is default with URL label', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        const urlLabel = page.getByText(/url.*webhook/i);
        await expect(urlLabel.first()).toBeVisible();
        await expect(page.getByRole('note')).toHaveCount(0);
      });

      await test.step('Switch to email and verify recipients label', async () => {
        await page.getByTestId('provider-type').selectOption('email');
        const recipientsLabel = page.getByText(/recipients/i);
        await expect(recipientsLabel.first()).toBeVisible();
        await expect(page.getByRole('note')).toBeVisible();
      });

      await test.step('Switch back to discord and verify URL label returns', async () => {
        await page.getByTestId('provider-type').selectOption('discord');
        const urlLabel = page.getByText(/url.*webhook/i);
        await expect(urlLabel.first()).toBeVisible();
        await expect(page.getByRole('note')).toHaveCount(0);
      });
    });
  });

  test.describe('CRUD Operations', () => {
    test('should create email notification provider', async ({ page }) => {
      const providerName = generateProviderName('email-create');
      let capturedPayload: Record<string, unknown> | null = null;

      await test.step('Mock create endpoint to capture payload', async () => {
        const createdProviders: Array<Record<string, unknown>> = [];

        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            capturedPayload = payload;
            const created = { id: `email-provider-1`, ...payload };
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

      await test.step('Open form and select email type', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('email');
      });

      await test.step('Fill email provider form', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await page.getByTestId('provider-url').fill('admin@example.com, alerts@example.com');
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
        expect(capturedPayload?.type).toBe('email');
        expect(capturedPayload?.name).toBe(providerName);
        expect(capturedPayload?.url).toBe('admin@example.com, alerts@example.com');
        expect(capturedPayload?.token).toBeUndefined();
        expect(capturedPayload?.gotify_token).toBeUndefined();
      });
    });

    test('should edit email notification provider recipients', async ({ page }) => {
      let updatedPayload: Record<string, unknown> | null = null;

      await test.step('Mock existing email provider', async () => {
        let providers = [
          {
            id: 'email-edit-id',
            name: 'Email Alert Provider',
            type: 'email',
            url: 'old-recipient@example.com',
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
              p.id === 'email-edit-id' ? { ...p, ...updatedPayload } : p
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

      await test.step('Verify email provider is displayed', async () => {
        await expect(page.getByText('Email Alert Provider')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click edit on email provider', async () => {
        const providerRow = page.getByTestId('provider-row-email-edit-id');
        const sendTestButton = providerRow.getByRole('button', { name: /send test/i });
        await expect(sendTestButton).toBeVisible({ timeout: 5000 });
        await sendTestButton.focus();
        await page.keyboard.press('Tab');
        await page.keyboard.press('Enter');
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify form loads with email type', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('email');
      });

      await test.step('Update recipients', async () => {
        const urlInput = page.getByTestId('provider-url');
        await urlInput.clear();
        await urlInput.fill('new-admin@example.com, ops@example.com');
      });

      await test.step('Save changes', async () => {
        const updateResponsePromise = waitForAPIResponse(
          page,
          /\/api\/v1\/notifications\/providers\/email-edit-id/,
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

      await test.step('Verify update success', async () => {
        await expect(page.getByText('Email Alert Provider').first()).toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify updated payload', async () => {
        expect(updatedPayload).toBeTruthy();
        expect(updatedPayload?.type).toBe('email');
        expect(updatedPayload?.url).toBe('new-admin@example.com, ops@example.com');
      });
    });

    test('should delete email notification provider', async ({ page }) => {
      await test.step('Mock existing email provider', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'email-delete-id',
                  name: 'Email To Delete',
                  type: 'email',
                  url: 'delete-me@example.com',
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

      await test.step('Verify email provider is displayed', async () => {
        await expect(page.getByText('Email To Delete')).toBeVisible({ timeout: 10000 });
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

    test('should display email provider type badge in list', async ({ page }) => {
      await test.step('Mock providers with email type', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                { id: 'email-badge-id', name: 'Email Notifications', type: 'email', url: 'team@example.com', enabled: true },
                { id: 'discord-badge-id', name: 'Discord Alerts', type: 'discord', url: 'https://discord.com/api/webhooks/test', enabled: true },
              ]),
            });
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Reload to get mocked response', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Verify email type badge is displayed', async () => {
        const emailRow = page.getByTestId('provider-row-email-badge-id');
        await expect(emailRow).toBeVisible();
        const emailBadge = emailRow.getByText(/^email$/i);
        await expect(emailBadge).toBeVisible();
      });

      await test.step('Verify email is a supported type with full actions', async () => {
        const emailRow = page.getByTestId('provider-row-email-badge-id');
        const rowButtons = emailRow.getByRole('button');
        const buttonCount = await rowButtons.count();
        expect(buttonCount).toBeGreaterThanOrEqual(2);
      });
    });
  });

  test.describe('Validation', () => {
    test('should allow saving email provider without URL (recipients optional)', async ({ page }) => {
      let createCalled = false;

      await test.step('Mock create endpoint', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            createCalled = true;
            await route.fulfill({
              status: 201,
              contentType: 'application/json',
              body: JSON.stringify({ id: 'email-no-url', ...((await request.postDataJSON()) as Record<string, unknown>) }),
            });
            return;
          }
          await route.continue();
        });
      });

      await test.step('Open form and select email', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('email');
      });

      await test.step('Fill name but leave recipients empty', async () => {
        await page.getByTestId('provider-name').fill(generateProviderName('email-empty-recipients'));
      });

      await test.step('Save and verify no URL validation error for email type', async () => {
        await page.getByTestId('provider-save-btn').click();
        await expect(page.getByTestId('provider-url-error')).toHaveCount(0);
      });

      await test.step('Verify create was called (URL not required for email)', async () => {
        expect(createCalled).toBeTruthy();
      });
    });

    test('should require URL for non-email types but not for email', async ({ page }) => {
      let createCalled = false;

      await test.step('Intercept create calls', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            createCalled = true;
          }
          await route.continue();
        });
      });

      await test.step('Open form with discord type (default)', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
      });

      await test.step('Submit discord form with name but no URL', async () => {
        await page.getByTestId('provider-name').fill('discord-validation-test');
        await page.getByTestId('provider-save-btn').click();
      });

      await test.step('Verify URL error shown for discord', async () => {
        await expect(page.getByTestId('provider-url-error')).toBeVisible();
        expect(createCalled).toBeFalsy();
      });
    });

    test('should not show URL validation error for email type', async ({ page }) => {
      await test.step('Open form and select email', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('email');
      });

      await test.step('Verify URL error element is not rendered for email', async () => {
        await expect(page.getByTestId('provider-url-error')).toHaveCount(0);
      });

      await test.step('Submit form with only name filled', async () => {
        await page.getByTestId('provider-name').fill('email-no-url-error-test');
        await page.getByTestId('provider-save-btn').click();
      });

      await test.step('Verify no URL-related error appears', async () => {
        await expect(page.getByTestId('provider-url-error')).toHaveCount(0);
      });
    });

    test('should validate name is required for email provider', async ({ page }) => {
      await test.step('Open form and select email', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('provider-type').selectOption('email');
      });

      await test.step('Leave name empty and fill recipients', async () => {
        await page.getByTestId('provider-url').fill('test@example.com');
      });

      await test.step('Attempt to save', async () => {
        await page.getByTestId('provider-save-btn').click();
      });

      await test.step('Verify name validation error', async () => {
        const nameInput = page.getByTestId('provider-name');
        const inputHasError = await nameInput.evaluate((el) =>
          el.getAttribute('aria-invalid') === 'true'
        ).catch(() => false);

        const errorMessage = page.getByText(/required/i);
        const hasError = await errorMessage.isVisible().catch(() => false);

        expect(hasError || inputHasError).toBeTruthy();
      });
    });
  });

  test.describe('Payload Contract', () => {
    test('should send correct payload for email provider create', async ({ page }) => {
      let capturedPayload: Record<string, unknown> | null = null;

      await test.step('Mock create endpoint to capture payload', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            capturedPayload = (await request.postDataJSON()) as Record<string, unknown>;
            await route.fulfill({
              status: 201,
              contentType: 'application/json',
              body: JSON.stringify({ id: 'email-payload-id', ...capturedPayload }),
            });
            return;
          }

          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([]),
            });
            return;
          }

          await route.continue();
        });
      });

      await test.step('Create email provider with recipients', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });

        await page.getByTestId('provider-type').selectOption('email');
        await page.getByTestId('provider-name').fill('payload-test-email');
        await page.getByTestId('provider-url').fill('alert@example.com, ops@example.com');

        await page.getByTestId('provider-save-btn').click();
      });

      await test.step('Verify email payload contract', async () => {
        expect(capturedPayload).toBeTruthy();
        expect(capturedPayload?.type).toBe('email');
        expect(capturedPayload?.name).toBe('payload-test-email');
        expect(capturedPayload?.url).toBe('alert@example.com, ops@example.com');
        expect(capturedPayload?.token).toBeUndefined();
        expect(capturedPayload?.gotify_token).toBeUndefined();
      });
    });
  });
});
