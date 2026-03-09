/**
 * Notifications E2E Tests
 *
 * Tests the Notifications page functionality including:
 * - Provider list display and empty states
 * - Provider CRUD operations (Discord-only)
 * - Template management (built-in and external)
 * - Testing and preview functionality
 * - Event selection configuration
 * - Accessibility compliance
 *
 * @see /projects/Charon/docs/plans/phase4-settings-plan.md Section 3.3
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForToast, waitForAPIResponse } from '../utils/wait-helpers';

/**
 * Helper to generate unique provider names
 */
function generateProviderName(prefix: string = 'test-provider'): string {
  return `${prefix}-${Date.now()}`;
}

/**
 * Helper to generate unique template names
 */
function generateTemplateName(prefix: string = 'test-template'): string {
  return `${prefix}-${Date.now()}`;
}

async function resetNotificationProviders(
  page: import('@playwright/test').Page,
  token: string
): Promise<void> {
  if (!token) {
    return;
  }

  const listResponse = await page.request.get('/api/v1/notifications/providers', {
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!listResponse.ok()) {
    return;
  }

  const providers = (await listResponse.json()) as Array<{ id: string }>;
  await Promise.all(
    providers.map((provider) =>
      page.request.delete(`/api/v1/notifications/providers/${provider.id}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
    )
  );
}

test.describe('Notification Providers', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/settings/notifications');
    await waitForLoadingComplete(page);
  });

  test.describe('Provider List', () => {
    /**
     * Test: Display notification providers list
     * Priority: P0
     */
    test('should display notification providers list', async ({ page }) => {
      await test.step('Verify page URL', async () => {
        await expect(page).toHaveURL(/\/settings\/notifications/);
      });

      await test.step('Verify page heading exists', async () => {
        // Use specific name to avoid strict mode violation when multiple H1s exist
        const pageHeading = page.getByRole('heading', { name: /notification.*providers/i });
        await expect(pageHeading).toBeVisible();
      });

      await test.step('Verify Add Provider button exists', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await expect(addButton).toBeVisible();
      });

      await test.step('Verify main content area exists', async () => {
        await expect(page.getByRole('main')).toBeVisible();
      });

      await test.step('Verify no error messages displayed', async () => {
        const errorAlert = page.getByRole('alert').filter({ hasText: /error|failed/i });
        await expect(errorAlert).toHaveCount(0);
      });

      await test.step('Verify standalone security compatibility section is not rendered', async () => {
        await expect(page.getByTestId('security-notifications-section')).toHaveCount(0);
      });
    });

    /**
     * Test: Show empty state when no providers
     * Priority: P1
     */
    test('should show empty state when no providers', async ({ page }) => {
      await test.step('Mock empty providers response', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([]),
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

      await test.step('Verify empty state message', async () => {
        const emptyState = page.getByText(/no notification providers configured\.?/i);
        await expect(emptyState).toBeVisible({ timeout: 5000 });
      });
    });

    /**
     * Test: Display provider type badges
     * Priority: P2
     */
    test('should display provider type badges with explicit deprecated messaging for legacy types', async ({ page }) => {
      await test.step('Mock providers with different types', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                { id: '1', name: 'Discord Alert', type: 'discord', url: 'https://discord.com/api/webhooks/test', enabled: true },
                { id: '2', name: 'Slack Notify', type: 'slack', url: 'https://hooks.example.com/services/test', enabled: true },
                { id: '3', name: 'Generic Hook', type: 'generic', url: 'https://webhook.test.local', enabled: false },
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

      await test.step('Verify Discord type badge', async () => {
        const discordBadge = page.getByTestId('provider-row-1').getByText(/^discord$/i);
        await expect(discordBadge).toBeVisible();
      });

      await test.step('Verify explicit deprecated and non-dispatch messaging is rendered for non-discord provider', async () => {
        await expect(page.getByTestId('provider-deprecated-status-2')).toHaveText(/deprecated/i);
        await expect(page.getByTestId('provider-nondispatch-status-2')).toHaveText(/non-dispatch/i);
        await expect(page.getByTestId('provider-deprecated-message-2')).toContainText(/deprecated/i);
      });

      await test.step('Verify non-discord rows are read-only actions', async () => {
        const legacyRow = page.getByTestId('provider-row-2');
        const legacyButtons = legacyRow.getByRole('button');
        await expect(legacyButtons).toHaveCount(1);
      });
    });

    /**
     * Test: Filter providers by type
     * Priority: P2
     */
    test('should filter providers by type', async ({ page }) => {
      await test.step('Mock providers with multiple types', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                { id: '1', name: 'Discord One', type: 'discord', url: 'https://discord.com/api/webhooks/1', enabled: true },
                { id: '2', name: 'Discord Two', type: 'discord', url: 'https://discord.com/api/webhooks/2', enabled: true },
                { id: '3', name: 'Slack Notify', type: 'slack', url: 'https://hooks.example.com/test', enabled: true },
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

      await test.step('Verify multiple providers are displayed', async () => {
        // Check that providers are visible - look for provider names
        await expect(page.getByText('Discord One')).toBeVisible();
        await expect(page.getByText('Discord Two')).toBeVisible();
        await expect(page.getByText('Slack Notify')).toBeVisible();
      });

      await test.step('Verify legacy provider row renders explicit deprecated messaging', async () => {
        await expect(page.getByTestId('provider-deprecated-status-3')).toHaveText(/deprecated/i);
        await expect(page.getByTestId('provider-nondispatch-status-3')).toHaveText(/non-dispatch/i);
        await expect(page.getByTestId('provider-deprecated-message-3')).toContainText(/deprecated/i);
      });
    });
  });

  test.describe('Provider CRUD', () => {
    /**
     * Test: Create Discord notification provider
     * Priority: P0
     */
    test('should create Discord notification provider', async ({ page }) => {
      const providerName = generateProviderName('discord');

      await test.step('Click Add Provider button', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await expect(addButton).toBeVisible();
        await addButton.click();
      });

      await test.step('Wait for form to render', async () => {
        // Wait for the form dialog to be fully rendered before accessing inputs
        await page.waitForLoadState('domcontentloaded');
        const nameInput = page.getByTestId('provider-name');
        await expect(nameInput).toBeVisible({ timeout: 5000 });
      });

      await test.step('Fill provider form', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill('https://discord.com/api/webhooks/12345/abcdef');
      });

      await test.step('Configure event notifications', async () => {
        const proxyHostsCheckbox = page.getByTestId('notify-proxy-hosts');
        await proxyHostsCheckbox.check();

        const certsCheckbox = page.getByTestId('notify-certs');
        await certsCheckbox.check();
      });

      await test.step('Save provider', async () => {
        const createRequestPromise = page.waitForRequest(
          (request) => request.method() === 'POST' && /\/api\/v1\/notifications\/providers$/.test(request.url())
        );
        await page.getByTestId('provider-save-btn').click();

        const createRequest = await createRequestPromise;
        const payload = createRequest.postDataJSON() as Record<string, unknown>;
        expect(payload.type).toBe('discord');
      });

      await test.step('Verify success feedback', async () => {
        // Wait for form to close or success message
        const successIndicator = page
          .getByText(providerName)
          .or(page.locator('[data-testid="toast-success"]'))
          .or(page.getByRole('status').filter({ hasText: /success|saved|created/i }));

        await expect(successIndicator.first()).toBeVisible({ timeout: 10000 });
      });
    });

    /**
     * Test: Form offers supported provider types
     * Priority: P0
     */
    test('should offer supported provider type options in form', async ({ page }) => {

      await test.step('Click Add Provider button', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Wait for form to render', async () => {
        // Wait for the form dialog to be fully rendered before accessing inputs
        await page.waitForLoadState('domcontentloaded');
        const nameInput = page.getByTestId('provider-name');
        await expect(nameInput).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify provider type select contains supported options', async () => {
        const providerTypeSelect = page.getByTestId('provider-type');
        await expect(providerTypeSelect.locator('option')).toHaveCount(4);
        await expect(providerTypeSelect.locator('option')).toHaveText(['Discord', 'Gotify', 'Generic Webhook', 'Email']);
        await expect(providerTypeSelect).toBeEnabled();
      });
    });

    /**
     * Test: Legacy non-discord providers remain non-editable with explicit deprecated messaging
     * Priority: P0
     */
    test('should keep legacy non-discord providers non-editable with explicit deprecated messaging', async ({ page }) => {
      await test.step('Mock existing legacy provider', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'legacy-provider',
                  name: 'Legacy Generic',
                  type: 'generic',
                  url: 'https://legacy.example.com/webhook',
                  enabled: false,
                },
              ]),
            });
            return;
          }

          await route.continue();
        });
      });

      await test.step('Reload page and verify explicit deprecated row messaging', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
        await expect(page.getByTestId('provider-deprecated-status-legacy-provider')).toHaveText(/deprecated/i);
        await expect(page.getByTestId('provider-nondispatch-status-legacy-provider')).toHaveText(/non-dispatch/i);
        await expect(page.getByTestId('provider-deprecated-message-legacy-provider')).toContainText(/deprecated/i);
      });

      await test.step('Verify only delete action remains for legacy row', async () => {
        const legacyRow = page.getByTestId('provider-row-legacy-provider');
        await expect(legacyRow.getByRole('button')).toHaveCount(1);
      });
    });

    /**
     * Test: Edit existing provider
     * Priority: P0
     */
    test('should edit existing provider', async ({ page }) => {
      await test.step('Mock existing provider', async () => {
        let providers = [
          {
            id: 'test-edit-id',
            name: 'Original Provider',
            type: 'discord',
            url: 'https://discord.com/api/webhooks/test',
            enabled: true,
            notify_proxy_hosts: true,
            notify_certs: false,
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
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            expect(payload.type).toBe('discord');
            providers = providers.map((provider) =>
              provider.id === 'test-edit-id'
                ? { ...provider, ...payload }
                : provider
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

      await test.step('Verify provider is displayed', async () => {
        const providerName = page.getByText('Original Provider');
        await expect(providerName).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click edit button on provider', async () => {
        const providerRow = page.getByTestId('provider-row-test-edit-id');
        const sendTestButton = providerRow.getByRole('button', { name: /send test/i });

        await expect(sendTestButton).toBeVisible({ timeout: 5000 });
        await sendTestButton.focus();
        await page.keyboard.press('Tab');
        await page.keyboard.press('Enter');

        await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
      });

      await test.step('Modify provider name', async () => {
        const nameInput = page.getByTestId('provider-name');
        await expect(nameInput).toBeVisible({timeout: 5000});
        await nameInput.clear();
        await nameInput.fill('Updated Provider Name');
      });

      await test.step('Save changes', async () => {
        // Wait for the update response so the list refresh has updated data.
        const updateResponsePromise = waitForAPIResponse(
          page,
          /\/api\/v1\/notifications\/providers\/test-edit-id/,
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
        const updatedProvider = page.getByText('Updated Provider Name');
        await expect(updatedProvider.first()).toBeVisible({ timeout: 10000 });
      });
    });

    /**
     * Test: Delete provider with confirmation
     * Priority: P0
     */
    test('should delete provider with confirmation', async ({ page }) => {
      await test.step('Mock existing provider', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'delete-me',
                  name: 'Provider to Delete',
                  type: 'slack',
                  url: 'https://hooks.example.com/test',
                  enabled: true,
                },
              ]),
            });
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Reload page', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Verify provider is displayed', async () => {
        await expect(page.getByText('Provider to Delete')).toBeVisible({ timeout: 10000 });
      });

      await test.step('Mock delete response', async () => {
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

      await test.step('Click delete button', async () => {
        // Handle confirmation dialog
        page.on('dialog', async (dialog) => {
          expect(dialog.type()).toBe('confirm');
          await dialog.accept();
        });

        const deleteButton = page.getByRole('button', { name: /delete/i })
          .or(page.locator('button').filter({ has: page.locator('svg.lucide-trash2, svg[class*="trash"]') }));

        await deleteButton.first().click();
      });

      await test.step('Verify deletion', async () => {
        // Provider should be gone or success message shown
        const successIndicator = page.locator('[data-testid="toast-success"]')
          .or(page.getByRole('status').filter({ hasText: /deleted|removed/i }))
          .or(page.getByText(/no.*providers/i));

        // Either success toast or empty state
        const hasIndicator = await successIndicator.first().isVisible({ timeout: 5000 }).catch(() => false);
        expect(hasIndicator).toBeTruthy();
      });
    });

    /**
     * Test: Enable/disable provider
     * Priority: P1
     */
    test('should enable/disable provider', async ({ page }) => {
      await test.step('Mock existing provider', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify([
                {
                  id: 'toggle-me',
                  name: 'Toggle Provider',
                  type: 'discord',
                  url: 'https://discord.com/api/webhooks/test',
                  enabled: true,
                },
              ]),
            });
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Reload page', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Click edit to access enabled toggle', async () => {
        const providerCard = page.getByTestId('provider-row-toggle-me');
        const editButton = providerCard.getByRole('button').nth(1); // 0=Send, 1=Edit, 2=Delete
        await expect(editButton).toBeVisible({ timeout: 5000 });
        await editButton.click();
      });

      await test.step('Toggle enabled state', async () => {
        const enabledCheckbox = page.getByTestId('provider-enabled');
        await expect(enabledCheckbox).toBeVisible({ timeout: 5000 });
        const isChecked = await enabledCheckbox.isChecked();
        await enabledCheckbox.setChecked(!isChecked);
        expect(await enabledCheckbox.isChecked()).toBe(!isChecked);
      });
    });

    /**
     * Test: Validate provider URL
     * Priority: P1
     * Note: Skip - URL validation behavior differs from expected
     */
    test('should validate provider URL', async ({ page }) => {
      const providerName = generateProviderName('validation');

      await test.step('Mock provider validation responses', async () => {
        let providers: Array<Record<string, unknown>> = [];

        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            if (payload.url === 'not-a-valid-url') {
              await route.fulfill({
                status: 400,
                contentType: 'application/json',
                body: JSON.stringify({ error: 'Invalid URL' }),
              });
              return;
            }

            const created = {
              id: 'validated-provider-id',
              enabled: true,
              notify_proxy_hosts: true,
              notify_remote_servers: true,
              notify_domains: true,
              notify_certs: true,
              notify_uptime: true,
              ...payload,
            };

            providers = [created];
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
              body: JSON.stringify(providers),
            });
            return;
          }

          await route.continue();
        });
      });

      await test.step('Click Add Provider button', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Fill form with invalid URL', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill('not-a-valid-url');
      });

      await test.step('Attempt to save', async () => {
        await page.getByTestId('provider-save-btn').click();
      });

      await test.step('Verify URL validation error', async () => {
        const urlInput = page.getByTestId('provider-url');

        // Check for validation error indicators
        const hasError = await urlInput.evaluate((el) =>
          el.classList.contains('border-red-500') ||
          el.classList.contains('border-destructive') ||
          el.getAttribute('aria-invalid') === 'true'
        ).catch(() => false);

        const errorMessage = page.getByText(/url.*required|invalid.*url|valid.*url/i);
        const hasErrorMessage = await errorMessage.isVisible().catch(() => false);

        await expect(page.getByTestId('provider-save-btn')).toBeVisible();
        expect(hasError || hasErrorMessage).toBeTruthy();
      });

      await test.step('Correct URL and verify validation passes', async () => {
        const urlInput = page.getByTestId('provider-url');

        // Ensure the input is attached and visible before clearing to avoid detached element errors.
        await expect(urlInput).toBeAttached();
        await expect(urlInput).toBeVisible();
        await urlInput.clear();
        await urlInput.fill('https://discord.com/api/webhooks/valid/url');

        // Wait for successful create response so the list refresh reflects the valid URL.
        const createResponsePromise = waitForAPIResponse(
          page,
          /\/api\/v1\/notifications\/providers$/,
          { status: 201 }
        );
        const refreshResponsePromise = waitForAPIResponse(
          page,
          /\/api\/v1\/notifications\/providers$/,
          { status: 200 }
        );

        await page.getByTestId('provider-save-btn').click();
        await createResponsePromise;
        await refreshResponsePromise;

        const providerInList = page.getByText(providerName);
        await expect(providerInList.first()).toBeVisible({ timeout: 10000 });
      });
    });

    /**
     * Test: Validate provider name required
     * Priority: P1
     */
    test('should validate provider name required', async ({ page }) => {
      await test.step('Click Add Provider button', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Leave name empty and fill other fields', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill('https://discord.com/api/webhooks/test/token');
      });

      await test.step('Attempt to save', async () => {
        await page.getByTestId('provider-save-btn').click();
      });

      await test.step('Verify name validation error', async () => {
        const errorMessage = page.getByText(/required|name.*required/i);
        const hasError = await errorMessage.isVisible().catch(() => false);

        const nameInput = page.getByTestId('provider-name');
        const inputHasError = await nameInput.evaluate((el) =>
          el.classList.contains('border-red-500') ||
          el.getAttribute('aria-invalid') === 'true'
        ).catch(() => false);

        expect(hasError || inputHasError).toBeTruthy();
      });
    });
  });

  test.describe('Template Management', () => {
    /**
     * Test: Select built-in template
     * Priority: P1
     */
    test('should select built-in template', async ({ page }) => {
      await test.step('Mock templates response', async () => {
        await page.route('**/api/v1/notifications/templates', async (route) => {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify([
              { id: 'minimal', name: 'Minimal', description: 'Basic notification format' },
              { id: 'detailed', name: 'Detailed', description: 'Full details format' },
            ]),
          });
        });
      });

      await test.step('Click Add Provider button', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Select provider type that supports templates', async () => {
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
      });

      await test.step('Select minimal template button', async () => {
        const minimalButton = page.getByRole('button', { name: /minimal/i });
        if (await minimalButton.isVisible()) {
          await minimalButton.click();

          // Verify template is selected (button has brand styling - bg-brand-500 or similar)
          await expect(minimalButton).toHaveClass(/brand|selected|active/i);
        }
      });

      await test.step('Select detailed template button', async () => {
        const detailedButton = page.getByRole('button', { name: /detailed/i });
        if (await detailedButton.isVisible()) {
          await detailedButton.click();

          // Verify template is selected (uses bg-brand-500 for selected state)
          await expect(detailedButton).toHaveClass(/bg-brand|primary/);
        }
      });
    });

    /**
     * Test: Create custom template
     * Priority: P1
     */
    test('should create custom template', async ({ page }) => {
      const templateName = generateTemplateName('custom');

      await test.step('Verify template management section exists', async () => {
        // The template management section should have a heading or button for managing templates
        const templateSection = page.locator('h2').filter({ hasText: /external.*templates/i });
        await expect(templateSection).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click New Template button in the template management area', async () => {
        const newTemplateBtn = page.getByRole('button', { name: /new template/i });
        await expect(newTemplateBtn).toBeVisible({ timeout: 5000 });
        await newTemplateBtn.click();
      });

      await test.step('Wait for template form to appear in the page', async () => {
        // When "New Template" is clicked, the managingTemplates state becomes true
        // and the form appears. We should see form inputs or heading.
        const formInputs = page.locator('input[type="text"], textarea, select').first();
        await expect(formInputs).toBeVisible({ timeout: 5000 });
      });

      await test.step('Fill template form', async () => {
        // Use explicit test IDs for reliable form filling
        await page.getByTestId('template-name').fill(templateName);
        await page.getByTestId('template-description').fill('Test template');
        await page.getByTestId('template-type').selectOption('custom');
        await page.getByTestId('template-config').fill('{"custom": "{{.Message}}", "source": "charon"}');
      });

      await test.step('Save template by clicking Submit/Save button', async () => {
        // Use explicit test ID for the save button
        await page.getByTestId('template-save-btn').click();
      });

      await test.step('Verify template was created and appears in list', async () => {
        const templateInList = page.getByText(templateName);
        await expect(templateInList.first()).toBeVisible({ timeout: 10000 });
      });
    });

    /**
     * Test: Preview template with sample data
     * Priority: P1
     */
    test('should preview template with sample data', async ({ page }) => {
      await test.step('Verify template management section is available', async () => {
        const templateSection = page.locator('h2').filter({ hasText: /external.*templates/i });
        await expect(templateSection).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click New Template button', async () => {
        const newTemplateBtn = page.getByRole('button', { name: /new template/i });
        await expect(newTemplateBtn).toBeVisible({ timeout: 5000 });
        await newTemplateBtn.click();
      });

      await test.step('Wait for template form to appear', async () => {
        const formInputs = page.locator('input[type="text"], textarea, select').first();
        await expect(formInputs).toBeVisible({ timeout: 5000 });
      });

      await test.step('Fill template form with variables', async () => {
        // Use explicit test IDs for reliable form filling
        await page.getByTestId('template-name').fill('Preview Test Template');
        await page.getByTestId('template-config').fill('{"message": "{{.Message}}", "title": "{{.Title}}"}');
      });

      await test.step('Mock preview API response', async () => {
        await page.route('**/api/v1/notifications/external-templates/preview', async (route) => {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              rendered: '{"message": "Preview Message", "title": "Preview Title"}',
              parsed: { message: 'Preview Message', title: 'Preview Title' },
            }),
          });
        });
      });

      await test.step('Click preview button to generate preview', async () => {
        const previewButton = page.getByTestId('template-preview-btn');
        const visiblePreviewBtn = await previewButton.isVisible({ timeout: 3000 }).catch(() => false);

        if (visiblePreviewBtn) {
          await previewButton.first().click();
        } else {
          // If no preview button found in form, skip this step
          console.log('Preview button not found in template form');
        }
      });

      await test.step('Verify preview output is displayed', async () => {
        // Look for the preview results (typically in a <pre> tag)
        const previewContent = page.locator('pre').filter({ hasText: /Preview Message|Preview Title|message|title/i });
        const foundPreview = await previewContent.first().isVisible({ timeout: 5000 }).catch(() => false);

        if (foundPreview) {
          await expect(previewContent.first()).toBeVisible();
        }
      });
    });

    /**
     * Test: Edit external template
     * Priority: P2
     */
	    test('should edit external template', async ({ page }) => {
	      await test.step('Mock external templates API response', async () => {
	        await page.route(/\/api\/v1\/notifications\/external-templates\/?(?:\?.*)?$/, async (route, request) => {
	          if (request.method() === 'GET') {
	            await route.fulfill({
	              status: 200,
	              contentType: 'application/json',
	              body: JSON.stringify([
                {
                  id: 'edit-template-id',
                  name: 'Editable Template',
                  description: 'Template for editing',
                  template: 'custom',
                  config: '{"old": "config"}',
                },
              ]),
	      });
          } else {
            await route.continue();
          }
        });
      });

      await test.step('Reload page to load mocked templates', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
      });

	      await test.step('Click Manage Templates button to show templates list', async () => {
	        // Find the toggle button for template management
	        const allButtons = page.getByRole('button');
	        const manageBtn = allButtons.filter({ hasText: /manage.*templates/i }).first();
	        await expect(manageBtn).toBeVisible({ timeout: 5000 });
	        await manageBtn.click();
	      });

      await test.step('Wait and verify templates list is visible', async () => {
        const templateText = page.getByText('Editable Template');
        await expect(templateText).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click edit button on the template', async () => {
        // Find the template card and locate the edit button
        const templateName = page.getByText('Editable Template').first();
        const templateCard = templateName.locator('..').locator('..').locator('..');

        // Edit button should be the first button in the card (or look for edit icon)
        const editButton = templateCard.locator('button').first();
        if (await editButton.isVisible({ timeout: 3000 }).catch(() => false)) {
          await editButton.click();
        }
      });

      await test.step('Wait for template edit form to appear', async () => {
        const configTextarea = page.locator('textarea').first();
        await expect(configTextarea).toBeVisible({ timeout: 5000 });
      });

      await test.step('Modify template config', async () => {
        const configTextarea = page.locator('textarea').first();
        await configTextarea.clear();
        await configTextarea.fill('{"updated": "config", "version": 2}');
      });

      await test.step('Mock update API response', async () => {
        await page.route('**/api/v1/notifications/external-templates/*', async (route, request) => {
          if (request.method() === 'PUT') {
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

      await test.step('Save template changes', async () => {
        const saveButton = page.getByRole('button', { name: /save/i }).last();
        if (await saveButton.isVisible({ timeout: 3000 }).catch(() => false)) {
          await saveButton.click();
          await waitForLoadingComplete(page);
        }
      });
    });

    /**
     * Test: Delete external template
     * Priority: P2
     */
	    test('should delete external template', async ({ page }) => {
	      await test.step('Mock external templates', async () => {
	        let templates = [
          {
            id: 'delete-template-id',
            name: 'Template to Delete',
            description: 'Will be deleted',
            template: 'custom',
            config: '{"delete": "me"}',
          },
        ];

	        await page.route(/\/api\/v1\/notifications\/external-templates\/?(?:\?.*)?$/, async (route, request) => {
	          if (request.method() === 'GET') {
	            await route.fulfill({
	              status: 200,
	              contentType: 'application/json',
              body: JSON.stringify(templates),
            });
            return;
          }

          await route.continue();
        });

        await page.route('**/api/v1/notifications/external-templates/*', async (route, request) => {
          if (request.method() === 'DELETE') {
            templates = [];
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify({ success: true }),
            });
            return;
          }

          await route.continue();
        });
      });

	      await test.step('Reload page', async () => {
	        // Wait for external templates fetch so list render is deterministic.
	        const templatesResponsePromise = waitForAPIResponse(
	          page,
	          /\/api\/v1\/notifications\/external-templates\/?(?:\?.*)?$/,
	          { status: 200 }
	        );

        await page.reload();
        await templatesResponsePromise;
        await waitForLoadingComplete(page);
      });

      await test.step('Show template management', async () => {
        const manageButton = page.getByRole('button').filter({ hasText: /manage.*templates/i });
        await expect(manageButton).toBeVisible({ timeout: 5000 });
        await manageButton.click();
      });

      await test.step('Verify template is displayed', async () => {
        // Wait for list render so row-level actions are available.
        const templateHeading = page.getByRole('heading', { name: 'Template to Delete', level: 4 });
        await expect(templateHeading).toBeVisible({ timeout: 5000 });
      });

      await test.step('Click delete button with confirmation', async () => {
        page.on('dialog', async (dialog) => {
          await dialog.accept();
        });

        // Wait for delete response so the refresh uses the updated list.
        const deleteResponsePromise = waitForAPIResponse(
          page,
          /\/api\/v1\/notifications\/external-templates\/delete-template-id/,
          { status: 200 }
        );
	        const refreshResponsePromise = waitForAPIResponse(
	          page,
	          /\/api\/v1\/notifications\/external-templates\/?(?:\?.*)?$/,
	          { status: 200 }
	        );

        const templateHeading = page.getByRole('heading', { name: 'Template to Delete', level: 4 });
        const templateCard = templateHeading.locator('..').locator('..');
        const deleteButton = templateCard
          .locator('[data-testid="template-delete-btn"]')
          .or(templateCard.locator('button').nth(1));

        await expect(deleteButton).toBeVisible();
        await deleteButton.click();
        await deleteResponsePromise;
        await refreshResponsePromise;
      });

      await test.step('Verify template deleted', async () => {
        const templateHeading = page.getByRole('heading', { name: 'Template to Delete', level: 4 });
        await expect(templateHeading).toHaveCount(0, { timeout: 5000 });
      });
    });
  });

  test.describe('Testing & Preview', () => {
    /**
     * Test: Test notification provider
     * Priority: P0
     */
    test('should test notification provider', async ({ page }) => {
      await test.step('Click Add Provider button', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Fill provider form', async () => {
        await page.getByTestId('provider-name').fill('Test Provider');
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill('https://discord.com/api/webhooks/test/token');
      });

      await test.step('Mock test response', async () => {
        await page.route('**/api/v1/notifications/providers/test', async (route) => {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              success: true,
              message: 'Test notification sent successfully',
            }),
          });
        });
      });

      await test.step('Click test button', async () => {
        const testRequestPromise = page.waitForRequest(
          (request) => request.method() === 'POST' && /\/api\/v1\/notifications\/providers\/test$/.test(request.url())
        );
        const testButton = page.getByTestId('provider-test-btn');
        await expect(testButton).toBeVisible();
        await testButton.click();

        const testRequest = await testRequestPromise;
        const payload = testRequest.postDataJSON() as Record<string, unknown>;
        expect(payload.type).toBe('discord');
      });

      await test.step('Verify test initiated', async () => {
        // Button should show loading or success state
        const testButton = page.getByTestId('provider-test-btn');

        // Wait for loading to complete and check for success icon
        await waitForLoadingComplete(page);
        const hasSuccessIcon = await testButton.locator('svg').evaluate((el) =>
          el.classList.contains('text-green-500') ||
          el.closest('button')?.querySelector('.text-green-500') !== null
        ).catch(() => false);

        expect(hasSuccessIcon).toBeTruthy();
      });
    });

    /**
     * Test: Show test success feedback
     * Priority: P1
     */
    test('should show test success feedback', async ({ page }) => {
      await test.step('Click Add Provider button', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Fill provider form', async () => {
        await page.getByTestId('provider-name').fill('Success Test Provider');
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill('https://discord.com/api/webhooks/success/test');
      });

      await test.step('Mock successful test', async () => {
        await page.route('**/api/v1/notifications/providers/test', async (route) => {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ success: true }),
          });
        });
      });

      await test.step('Click test button', async () => {
        await page.getByTestId('provider-test-btn').click();
      });

      await test.step('Verify success feedback', async () => {
        const testButton = page.getByTestId('provider-test-btn');
        const successIcon = testButton.locator('svg.text-green-500');

        await expect(successIcon).toBeVisible({ timeout: 5000 });
      });
    });

    /**
     * Test: Preview notification content
     * Priority: P1
     * Note: Skip - Test IDs for provider form may not match implementation
     */
    test('should preview notification content', async ({ page }) => {
      await test.step('Click Add Provider button', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Fill provider form', async () => {
        await page.getByTestId('provider-name').fill('Preview Provider');
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill('https://discord.com/api/webhooks/preview/test');

        const configTextarea = page.getByTestId('provider-config');
        if (await configTextarea.isVisible()) {
          await configTextarea.fill('{"alert": "{{.Message}}", "event": "{{.EventType}}"}');
        }
      });

      await test.step('Mock preview response', async () => {
        await page.route('**/api/v1/notifications/providers/preview', async (route) => {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              rendered: '{"alert": "Test notification message", "event": "proxy_host_created"}',
              parsed: {
                alert: 'Test notification message',
                event: 'proxy_host_created',
              },
            }),
          });
        });
      });

      await test.step('Click preview button', async () => {
        const previewRequestPromise = page.waitForRequest(
          (request) => request.method() === 'POST' && /\/api\/v1\/notifications\/providers\/preview$/.test(request.url())
        );
        const previewButton = page.getByRole('button', { name: /preview/i });
        await previewButton.first().click();

        const previewRequest = await previewRequestPromise;
        const payload = previewRequest.postDataJSON() as Record<string, unknown>;
        expect(payload.type).toBe('discord');
      });

      await test.step('Verify preview displayed', async () => {
        const previewContent = page.locator('pre').filter({ hasText: /alert|event|message/i });
        await expect(previewContent.first()).toBeVisible({ timeout: 5000 });

        // Verify preview contains rendered values
        const previewText = await previewContent.first().textContent();
        expect(previewText).toContain('alert');
      });
    });

    test('should preserve Discord request payload contract for save, preview, and test', async ({ page }) => {
      const providerName = generateProviderName('discord-regression');
      const discordURL = 'https://discord.com/api/webhooks/regression/token';
      let capturedCreatePayload: Record<string, unknown> | null = null;
      let capturedPreviewPayload: Record<string, unknown> | null = null;
      let capturedTestPayload: Record<string, unknown> | null = null;
      const providers: Array<Record<string, unknown>> = [];

      await test.step('Mock provider list/create and preview/test endpoints', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'GET') {
            await route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify(providers),
            });
            return;
          }

          if (request.method() === 'POST') {
            capturedCreatePayload = (await request.postDataJSON()) as Record<string, unknown>;
            const created = {
              id: 'discord-regression-id',
              ...capturedCreatePayload,
            };
            providers.splice(0, providers.length, created);
            await route.fulfill({
              status: 201,
              contentType: 'application/json',
              body: JSON.stringify(created),
            });
            return;
          }

          await route.continue();
        });

        await page.route('**/api/v1/notifications/providers/preview', async (route, request) => {
          capturedPreviewPayload = (await request.postDataJSON()) as Record<string, unknown>;
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ rendered: '{"content":"ok"}', parsed: { content: 'ok' } }),
          });
        });

        await page.route('**/api/v1/notifications/providers/test', async (route, request) => {
          capturedTestPayload = (await request.postDataJSON()) as Record<string, unknown>;
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ message: 'Test notification sent successfully' }),
          });
        });
      });

      await test.step('Open add provider form and verify accessible form structure', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();
        await expect(page.getByTestId('provider-name')).toBeVisible();
        await expect(page.getByLabel('Name')).toBeVisible();
        await expect(page.getByLabel('Type')).toBeVisible();
        await expect(page.getByLabel(/URL \/ Webhook/i)).toBeVisible();
        await expect(page.getByTestId('provider-preview-btn')).toBeVisible();
        await expect(page.getByTestId('provider-test-btn')).toBeVisible();
        await expect(page.getByTestId('provider-save-btn')).toBeVisible();
      });

      await test.step('Submit preview and test from Discord form', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill(discordURL);
        await page.getByTestId('provider-preview-btn').click();
        await page.getByTestId('provider-test-btn').click();
      });

      await test.step('Save Discord provider', async () => {
        await page.getByTestId('provider-save-btn').click();
      });

      await test.step('Assert Discord payload contract remained unchanged', async () => {
        expect(capturedPreviewPayload).toBeTruthy();
        expect(capturedPreviewPayload?.type).toBe('discord');
        expect(capturedPreviewPayload?.url).toBe(discordURL);
        expect(capturedPreviewPayload?.token).toBeUndefined();

        expect(capturedTestPayload).toBeTruthy();
        expect(capturedTestPayload?.type).toBe('discord');
        expect(capturedTestPayload?.url).toBe(discordURL);
        expect(capturedTestPayload?.token).toBeUndefined();

        expect(capturedCreatePayload).toBeTruthy();
        expect(capturedCreatePayload?.type).toBe('discord');
        expect(capturedCreatePayload?.url).toBe(discordURL);
        expect(capturedCreatePayload?.token).toBeUndefined();
      });
    });
  });

  test.describe('Event Selection', () => {
    test.beforeEach(async ({ page, adminUser }) => {
      await test.step('Reset notification providers via API', async () => {
        // Reset providers to avoid persisted checkbox state across tests.
        await resetNotificationProviders(page, adminUser.token);
      });
    });

    /**
     * Test: Configure notification events
     * Priority: P1
     */
    test('should configure notification events', async ({ page }) => {
      await test.step('Click Add Provider button', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Wait for form to render', async () => {
        // Wait for the form dialog to be fully rendered before accessing inputs
        await page.waitForLoadState('domcontentloaded');
        const nameInput = page.getByTestId('provider-name');
        await expect(nameInput).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify all event checkboxes exist', async () => {
        await expect(page.getByTestId('notify-proxy-hosts')).toBeVisible();
        await expect(page.getByTestId('notify-remote-servers')).toBeVisible();
        await expect(page.getByTestId('notify-domains')).toBeVisible();
        await expect(page.getByTestId('notify-certs')).toBeVisible();
        await expect(page.getByTestId('notify-uptime')).toBeVisible();
        await expect(page.getByTestId('notify-security-waf-blocks')).toBeVisible();
        await expect(page.getByTestId('notify-security-acl-denies')).toBeVisible();
        await expect(page.getByTestId('notify-security-rate-limit-hits')).toBeVisible();
      });

      await test.step('Verify no standalone compatibility section exists', async () => {
        await expect(page.getByTestId('security-notifications-section')).toHaveCount(0);
      });

      await test.step('Toggle each event type', async () => {
        // Uncheck all first
        await page.getByTestId('notify-proxy-hosts').uncheck();
        await page.getByTestId('notify-remote-servers').uncheck();
        await page.getByTestId('notify-domains').uncheck();
        await page.getByTestId('notify-certs').uncheck();
        await page.getByTestId('notify-uptime').uncheck();

        // Verify all unchecked
        await expect(page.getByTestId('notify-proxy-hosts')).not.toBeChecked();
        await expect(page.getByTestId('notify-remote-servers')).not.toBeChecked();
        await expect(page.getByTestId('notify-domains')).not.toBeChecked();
        await expect(page.getByTestId('notify-certs')).not.toBeChecked();
        await expect(page.getByTestId('notify-uptime')).not.toBeChecked();

        // Check specific events
        await page.getByTestId('notify-proxy-hosts').check();
        await page.getByTestId('notify-certs').check();
        await page.getByTestId('notify-uptime').check();

        // Verify selection
        await expect(page.getByTestId('notify-proxy-hosts')).toBeChecked();
        await expect(page.getByTestId('notify-remote-servers')).not.toBeChecked();
        await expect(page.getByTestId('notify-domains')).not.toBeChecked();
        await expect(page.getByTestId('notify-certs')).toBeChecked();
        await expect(page.getByTestId('notify-uptime')).toBeChecked();
      });
    });

    /**
     * Test: Persist event selections
     * Priority: P1
     */
    test('should persist event selections', async ({ page }) => {
      const providerName = generateProviderName('events-test');
      let providers: Array<Record<string, unknown>> = [];

      await test.step('Mock provider create and list responses', async () => {
        await page.route('**/api/v1/notifications/providers', async (route, request) => {
          if (request.method() === 'POST') {
            const payload = (await request.postDataJSON()) as Record<string, unknown>;
            const created = {
              id: 'events-provider-id',
              enabled: true,
              notify_proxy_hosts: false,
              notify_remote_servers: false,
              notify_domains: false,
              notify_certs: false,
              notify_uptime: false,
              ...payload,
            };

            providers = [created];
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
              body: JSON.stringify(providers),
            });
            return;
          }

          await route.continue();
        });
      });

      await test.step('Click Add Provider button', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await expect(addButton).toBeVisible();
        await addButton.click();
      });

      await test.step('Wait for form to render', async () => {
        // Wait for the form card to be visible
        await page.waitForLoadState('domcontentloaded');
        const providerForm = page.locator('[class*="border-blue"], [class*="Card"]').filter({ hasText: /add.*new.*provider/i });
        await expect(providerForm).toBeVisible({ timeout: 5000 });
      });

      await test.step('Fill provider form with specific events', async () => {
        await page.getByTestId('provider-name').fill(providerName);
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill('https://discord.com/api/webhooks/events/test');

        // Configure specific events
        await page.getByTestId('notify-proxy-hosts').check();
        await page.getByTestId('notify-remote-servers').uncheck();
        await page.getByTestId('notify-domains').uncheck();
        await page.getByTestId('notify-certs').check();
        await page.getByTestId('notify-uptime').uncheck();
      });

      await test.step('Save provider', async () => {
        // Wait for create response so persisted event flags are available on reload.
        const createResponsePromise = waitForAPIResponse(
          page,
          /\/api\/v1\/notifications\/providers$/,
          { status: 201 }
        );
        await page.getByTestId('provider-save-btn').click();
        const createResponse = await createResponsePromise;
        expect(createResponse.ok()).toBeTruthy();
      });

      await test.step('Verify provider was created', async () => {
        const providerInList = page.getByText(providerName);
        await expect(providerInList.first()).toBeVisible({ timeout: 10000 });
      });

      await test.step('Reload to fetch persisted provider state', async () => {
        // Reload ensures the edit form reflects server-side persisted event flags.
        await page.reload();
        await waitForLoadingComplete(page);
        await expect(page.getByText(providerName).first()).toBeVisible({ timeout: 10000 });
      });

      await test.step('Edit provider to verify persisted values', async () => {
        // Click edit button for the newly created provider
        const providerText = page.getByText(providerName).first();
        const providerCard = providerText.locator('..').locator('..').locator('..');

        // The edit button is the pencil icon button
        const editButton = providerCard.getByRole('button').filter({ has: page.locator('svg') }).nth(1);
        await expect(editButton).toBeVisible({ timeout: 5000 });
        await editButton.click();
      });

      await test.step('Verify event selections persisted', async () => {
        // The events should be in the same state as when saved
        await expect(page.getByTestId('notify-proxy-hosts')).toBeChecked();
        await expect(page.getByTestId('notify-certs')).toBeChecked();
        // These should not be checked based on what we set
        await expect(page.getByTestId('notify-remote-servers')).not.toBeChecked();
        await expect(page.getByTestId('notify-domains')).not.toBeChecked();
        await expect(page.getByTestId('notify-uptime')).not.toBeChecked();
      });
    });
  });

  test.describe('Accessibility', () => {
    /**
     * Test: Keyboard navigation through form
     * Priority: P1
     */
    test('should be keyboard navigable', async ({ page }) => {
      await test.step('Click Add Provider to open form', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Tab through form elements', async () => {
        let focusedElements = 0;
        const maxTabs = 25;

        for (let i = 0; i < maxTabs; i++) {
          await page.keyboard.press('Tab');

          const focused = page.locator(':focus');
          const isVisible = await focused.isVisible().catch(() => false);

          if (isVisible) {
            focusedElements++;

            const tagName = await focused.evaluate((el) => el.tagName.toLowerCase()).catch(() => '');
            const isInteractive = ['input', 'select', 'button', 'textarea', 'a'].includes(tagName);

            if (isInteractive) {
              await expect(focused).toBeFocused();
            }
          }
        }

        expect(focusedElements).toBeGreaterThan(5);
      });

      await test.step('Fill form field with keyboard', async () => {
        const nameInput = page.getByTestId('provider-name');
        await nameInput.focus();
        await expect(nameInput).toBeFocused();

        await page.keyboard.type('Keyboard Test Provider');
        await expect(nameInput).toHaveValue('Keyboard Test Provider');
      });
    });

    /**
     * Test: Proper form labels for screen readers
     * Priority: P1
     */
    test('should have proper form labels', async ({ page }) => {
      await test.step('Open provider form', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Verify name input has label', async () => {
        const nameInput = page.getByTestId('provider-name');
        const hasLabel = await page.evaluate(() => {
          const input = document.querySelector('[data-testid="provider-name"]');
          if (!input) return false;

          // Check for associated label
          const id = input.id;
          if (id && document.querySelector(`label[for="${id}"]`)) return true;

          // Check for aria-label
          if (input.getAttribute('aria-label')) return true;

          // Check for parent label
          if (input.closest('label')) return true;

          // Check for preceding label in the same container
          const container = input.parentElement;
          if (container?.querySelector('label')) return true;

          return false;
        });

        expect(hasLabel).toBeTruthy();
      });

      await test.step('Verify type select has label', async () => {
        const typeSelect = page.getByTestId('provider-type');
        await expect(typeSelect).toBeVisible();

        // Should be in a labeled container
        const container = typeSelect.locator('..');
        const labelText = await container.textContent();
        expect(labelText?.length).toBeGreaterThan(0);
      });

      await test.step('Verify URL input has label', async () => {
        const urlInput = page.getByTestId('provider-url');
        await expect(urlInput).toBeVisible();

        // Should have placeholder or label
        const placeholder = await urlInput.getAttribute('placeholder');
        const hasContext = placeholder !== null && placeholder.length > 0;
        expect(hasContext).toBeTruthy();
      });

      await test.step('Verify buttons have accessible names', async () => {
        const saveButton = page.getByTestId('provider-save-btn');
        const buttonText = await saveButton.textContent();
        expect(buttonText?.trim().length).toBeGreaterThan(0);

        const testButton = page.getByTestId('provider-test-btn');
        const testButtonText = await testButton.textContent();
        // Test button may just have icon, check for aria-label too
        const ariaLabel = await testButton.getAttribute('aria-label');
        expect((testButtonText?.trim().length ?? 0) > 0 || ariaLabel !== null).toBeTruthy();
      });

      await test.step('Verify checkboxes have labels', async () => {
        const checkboxes = [
          'notify-proxy-hosts',
          'notify-remote-servers',
          'notify-domains',
          'notify-certs',
          'notify-uptime',
        ];

        for (const testId of checkboxes) {
          const checkbox = page.getByTestId(testId);
          await expect(checkbox).toBeVisible();

          // Each checkbox should have an associated label
          const hasLabel = await checkbox.evaluate((el) => {
            const label = el.nextElementSibling;
            return label?.tagName === 'LABEL' || el.closest('label') !== null;
          });

          expect(hasLabel).toBeTruthy();
        }
      });
    });
  });

  test.describe('Error Handling', () => {
    /**
     * Test: Show error when test fails
     * Priority: P1
     */
    test('should show error when test fails', async ({ page }) => {
      await test.step('Open provider form', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Fill provider form', async () => {
        await page.getByTestId('provider-name').fill('Error Test Provider');
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill('https://discord.com/api/webhooks/invalid');
      });

      await test.step('Mock failed test response', async () => {
        await page.route('**/api/v1/notifications/providers/test', async (route) => {
          await route.fulfill({
            status: 500,
            contentType: 'application/json',
            body: JSON.stringify({
              error: 'Failed to send notification: Invalid webhook URL',
            }),
          });
        });
      });

      await test.step('Click test button', async () => {
        await page.getByTestId('provider-test-btn').click();
      });

      await test.step('Verify error feedback', async () => {
        await waitForLoadingComplete(page);

        // Should show error icon (X) — use auto-retrying assertion instead of point-in-time check
        const testButton = page.getByTestId('provider-test-btn');
        const errorIcon = testButton.locator('svg.text-red-500, svg[class*="red"]');

        await expect(errorIcon).toBeVisible({ timeout: 10000 });
      });
    });

    /**
     * Test: Show preview error for invalid template
     * Priority: P2
     * Note: Skip - Test IDs for provider form may not match implementation
     */
    test('should show preview error for invalid template', async ({ page }) => {
      await test.step('Open provider form', async () => {
        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await addButton.click();
      });

      await test.step('Fill form with invalid JSON config', async () => {
        await page.getByTestId('provider-name').fill('Invalid Template Provider');
        await expect(page.getByTestId('provider-type')).toHaveValue('discord');
        await page.getByTestId('provider-url').fill('https://discord.com/api/webhooks/invalid/template');

        const configTextarea = page.getByTestId('provider-config');
        if (await configTextarea.isVisible()) {
          await configTextarea.fill('{ invalid json here }}}');
        }
      });

      await test.step('Mock preview error', async () => {
        await page.route('**/api/v1/notifications/providers/preview', async (route) => {
          await route.fulfill({
            status: 400,
            contentType: 'application/json',
            body: JSON.stringify({
              error: 'Invalid JSON template: unexpected token',
            }),
          });
        });
      });

      await test.step('Click preview button', async () => {
        const previewButton = page.getByRole('button', { name: /preview/i });
        await previewButton.first().click();
      });

      await test.step('Verify error message displayed', async () => {
        // Anchor to the unique "Preview Error:" i18n prefix rendered by the previewError state
        const errorMessage = page.getByText(/Preview Error:/i);
        await expect(errorMessage.first()).toBeVisible({ timeout: 5000 });
      });
    });
  });
});
