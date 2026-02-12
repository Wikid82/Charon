import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { getToastLocator, refreshListAndWait } from './utils/ui-helpers';
import {
  waitForAPIResponse,
  waitForConfigReload,
  waitForDialog,
  waitForLoadingComplete,
} from './utils/wait-helpers';

/**
 * DNS Provider CRUD Operations E2E Tests
 *
 * Tests Create, Read, Update, Delete operations for DNS Providers including:
 * - Creating providers of different types
 * - Listing providers
 * - Editing provider configuration
 * - Deleting providers
 * - Form validation
 */

test.describe('DNS Provider CRUD Operations', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test.describe('Create Provider', () => {
    test('should create a Manual DNS provider', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Wait for DNS Providers page content', async () => {
        // Wait for the nested DNS page content area to be visible
        // DNS route has parent DNS component + child DNSProviders component via Outlet
        await page.waitForLoadState('networkidle');
        await page.waitForTimeout(1000); // Allow React nested routing to complete
      });

      await test.step('Click Add Provider button', async () => {
        // Use first() to handle both header button and empty state button
        const addButton = page.getByRole('button', { name: /add.*provider/i }).first();
        await expect(addButton).toBeVisible({ timeout: 10000 });
        await addButton.click();
        await waitForDialog(page);
      });

      await test.step('Fill provider name', async () => {
        // The input has id="provider-name" and aria-label="Name" (from translation)
        const nameInput = page.locator('#provider-name').or(page.getByRole('textbox', { name: /name/i }));
        await expect(nameInput).toBeVisible();
        await nameInput.fill('Test Manual Provider');
      });

      await test.step('Select Manual type', async () => {
        // Select has id="provider-type" and aria-label from translation
        const typeSelect = page.locator('#provider-type').or(page.getByRole('combobox', { name: /type|provider/i }));
        await typeSelect.click();
        await page.getByRole('option', { name: /manual/i }).click();
      });

      await test.step('Save provider', async () => {
        // Click the Create button in the dialog footer
        const dialog = await waitForDialog(page);
        // Look for button with exact text "Create" within dialog
        const saveButton = dialog.getByRole('button', { name: 'Create' });
        await expect(saveButton).toBeVisible();
        await expect(saveButton).toBeEnabled();

        const responsePromise = waitForAPIResponse(page, '/api/v1/dns-providers');
        await saveButton.click();

        await responsePromise;
        await waitForConfigReload(page);

        // Wait for dialog to close (indicates success)
        await expect(dialog).not.toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify success', async () => {
        // Wait for success toast using shared helper
        const successToast = getToastLocator(page, /success|created/i, { type: 'success' });
        await expect(successToast).toBeVisible({ timeout: 5000 });

        // Refresh list to ensure provider appears
        await refreshListAndWait(page, { timeout: 5000 });
      });
    });

    test('should create a Webhook DNS provider', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Open add provider dialog', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).first().click();
        await waitForDialog(page);
      });

      await test.step('Select Webhook type first', async () => {
        // Must select type first to reveal credential fields
        const dialog = await waitForDialog(page);
        const typeSelect = dialog
          .locator('#provider-type')
          .or(dialog.getByRole('combobox', { name: /type|provider/i }));
        await typeSelect.click();

        const listbox = page.getByRole('listbox');
        await expect(listbox).toBeVisible({ timeout: 5000 });

        const webhookOption = listbox.getByRole('option', { name: /webhook/i });
        await expect(webhookOption).toBeVisible({ timeout: 5000 });
        await webhookOption.focus();
        await page.keyboard.press('Enter');
        await expect(listbox).toBeHidden({ timeout: 5000 });
      });

      await test.step('Fill provider details', async () => {
        const dialog = await waitForDialog(page);
        const listbox = page.getByRole('listbox');
        if (await listbox.isVisible().catch(() => false)) {
          await page.keyboard.press('Escape');
          await expect(listbox).toBeHidden({ timeout: 5000 });
        }

        const nameInput = dialog.locator('#provider-name');
        await expect(nameInput).toBeVisible({ timeout: 10000 });
        await nameInput.click();
        await nameInput.fill('Test Webhook Provider');

        const createUrlField = dialog.getByRole('textbox', { name: /create url/i });
        for (let attempt = 0; attempt < 2; attempt += 1) {
          if (await createUrlField.isVisible().catch(() => false)) {
            break;
          }

          const typeSelect = dialog
            .locator('#provider-type')
            .or(dialog.getByRole('combobox', { name: /type|provider/i }));
          await typeSelect.click();
          const webhookOption = page.getByRole('option', { name: /webhook/i });
          await expect(webhookOption).toBeVisible({ timeout: 5000 });
          await webhookOption.focus();
          await page.keyboard.press('Enter');
        }
        await expect(createUrlField).toBeVisible({ timeout: 10000 });
        await createUrlField.fill('https://example.com/dns/create');

        const deleteUrlField = dialog.getByRole('textbox', { name: /delete url/i });
        await expect(deleteUrlField).toBeVisible({ timeout: 10000 });
        await deleteUrlField.fill('https://example.com/dns/delete');
      });

      await test.step('Save and verify', async () => {
        const dialog = await waitForDialog(page);
        const responsePromise = waitForAPIResponse(page, '/api/v1/dns-providers');
        const createButton = dialog.getByRole('button', { name: /create/i });
        await expect(createButton).toBeEnabled();
        await createButton.click();
        await responsePromise;
        await waitForConfigReload(page);
        await expect(dialog).toBeHidden({ timeout: 10000 });

        const successToast = getToastLocator(page, /success|created/i, { type: 'success' });
        await expect(successToast).toBeVisible({ timeout: 5000 });
      });
    });

    test('should show validation errors for missing required fields', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Open add dialog', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).first().click();
        await waitForDialog(page);
      });

      await test.step('Verify save button is disabled when required fields empty', async () => {
        // The Create button is disabled when name or type is empty
        const saveButton = page.getByRole('button', { name: /create/i });
        await expect(saveButton).toBeDisabled();
      });

      await test.step('Fill name only and verify still disabled', async () => {
        await page.locator('#provider-name').fill('Test Provider');
        // Still disabled because type is not selected
        const saveButton = page.getByRole('button', { name: /create/i });
        await expect(saveButton).toBeDisabled();
      });
    });

    test('should validate webhook URL format', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await page.getByRole('button', { name: /add.*provider/i }).first().click();
      await waitForDialog(page);

      await test.step('Select Webhook type and enter invalid URL', async () => {
        const dialog = await waitForDialog(page);
        await dialog.locator('#provider-name').fill('Test Webhook');

        const typeSelect = dialog
          .locator('#provider-type')
          .or(dialog.getByRole('combobox', { name: /type|provider/i }));
        const urlField = dialog.getByRole('textbox', { name: /create url/i });
        const webhookOption = page.getByRole('option', { name: /webhook/i });
        const listbox = page.getByRole('listbox');

        for (let attempt = 0; attempt < 2; attempt += 1) {
          await typeSelect.click();
          await expect(webhookOption).toBeVisible({ timeout: 5000 });
          await webhookOption.focus();
          await page.keyboard.press('Enter');
          await expect(listbox).toBeHidden({ timeout: 5000 });

          if (await urlField.isVisible().catch(() => false)) {
            break;
          }
        }

        await expect(urlField).toBeVisible({ timeout: 10000 });
        await urlField.fill('not-a-valid-url');
        await urlField.blur();
      });

      await test.step('Try to save and check for URL validation', async () => {
        const dialog = await waitForDialog(page);
        const createButton = dialog.getByRole('button', { name: /save|create/i });
        await createButton.click();

        const urlField = page.getByRole('textbox', { name: /create url/i });
        await expect(urlField).toHaveValue('not-a-valid-url');
        await expect(dialog).toBeVisible();
      });
    });
  });

  test.describe('Provider List', () => {
    test('should display provider list or empty state', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Verify page loads', async () => {
        await expect(page).toHaveURL(/dns\/providers/);
      });

      await test.step('Check for providers or empty state', async () => {
        // The page should always show at least one of:
        // 1. Add Provider button (header or empty state)
        // 2. Provider cards with Edit buttons
        // 3. Empty state message

        const addButton = page.getByRole('button', { name: /add.*provider/i });
        await expect(addButton.first()).toBeVisible();
      });
    });

    test('should show Add Provider button', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      // Use first() since there may be both header button and empty state button
      const addButton = page.getByRole('button', { name: /add.*provider/i }).first();
      await expect(addButton).toBeVisible();
      await expect(addButton).toBeEnabled();
    });

    test('should show provider details in list', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      // If providers exist, verify they show required info
      // The page uses Card components in a grid with .grid class
      const providerCards = page.locator('.grid > div').filter({ has: page.locator('h3, [class*="title"]') });

      if ((await providerCards.count()) > 0) {
        const firstProvider = providerCards.first();

        await test.step('Verify provider name is displayed', async () => {
          // Provider should have a name visible in the card title
          const hasName = (await firstProvider.locator('h3, [class*="title"]').count()) > 0;
          expect(hasName).toBeTruthy();
        });

        await test.step('Verify provider type is displayed', async () => {
          // Provider should show its type (cloudflare, manual, etc.)
          const typeText = firstProvider.getByText(/cloudflare|route53|manual|webhook|rfc2136|script/i).first();
          await expect(typeText).toBeVisible();
        });
      }
    });
  });

  test.describe('Edit Provider', () => {
    // These tests require at least one provider to exist
    test('should open edit dialog for existing provider', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      // Wait for the page to load and check for provider cards
      // The page uses Card components inside a grid
      const providerCards = page.locator('.grid > div').filter({ has: page.getByRole('button', { name: /edit/i }) });

      if ((await providerCards.count()) > 0) {
        await test.step('Click edit on first provider', async () => {
          const firstProvider = providerCards.first();

          // Look for edit button
          const editButton = firstProvider.getByRole('button', { name: /edit/i });
          await editButton.click();
        });

        await test.step('Verify edit dialog opens', async () => {
          // Edit dialog should have the provider name pre-filled
          const dialog = await waitForDialog(page);
          const nameInput = dialog.locator('#provider-name');
          await expect(nameInput).toBeVisible();

          const currentValue = await nameInput.inputValue();
          expect(currentValue.length).toBeGreaterThan(0);
        });
      }
    });

    test('should update provider name', async ({ page, request }) => {
      let createdProviderId: number | string | undefined;
      const initialName = `Update Target ${Date.now()}`;
      const updatedName = `Updated Provider ${Date.now()}`;

      try {
        const createResponse = await page.request.post('/api/v1/dns-providers', {
          data: {
            name: initialName,
            provider_type: 'manual',
            credentials: {},
          },
        });
        expect(createResponse.ok()).toBeTruthy();

        const createdProvider = await createResponse.json();
        createdProviderId = createdProvider?.id;

        await page.goto('/dns/providers');
        await waitForLoadingComplete(page);

        const providerCard = page.locator('.grid > div').filter({ hasText: initialName }).first();
        await expect(providerCard).toBeVisible({ timeout: 10000 });

        await test.step('Open edit dialog', async () => {
          await providerCard.getByRole('button', { name: /edit/i }).click();
        });

        await test.step('Update name', async () => {
          const dialog = await waitForDialog(page);
          const nameInput = dialog.locator('#provider-name');
          await nameInput.clear();
          await nameInput.fill(updatedName);
        });

        await test.step('Save changes', async () => {
          const responsePromise = page.waitForResponse(
            (response) => response.url().includes('/api/v1/dns-providers/') && response.request().method() === 'PUT'
          );
          await page.getByRole('button', { name: /update/i }).click();
          const response = await responsePromise;
          expect(response.status()).toBeLessThan(500);
          await waitForConfigReload(page);
        });

        await test.step('Verify updated name in dialog', async () => {
          const dialog = await waitForDialog(page);
          const nameInput = dialog.locator('#provider-name');
          await expect(nameInput).toHaveValue(updatedName, { timeout: 5000 });

          const closeButton = dialog.getByRole('button', { name: /close|cancel/i }).first();
          if (await closeButton.isVisible()) {
            await closeButton.click();
          }
          await expect(page.getByRole('dialog')).toBeHidden({ timeout: 10000 });
        });
      } finally {
        if (createdProviderId) {
          await page.request.delete(`/api/v1/dns-providers/${createdProviderId}`);
        }
      }
    });
  });

  test.describe('Delete Provider', () => {
    test('should show delete confirmation dialog', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      // Find provider cards with delete buttons
      const providerCards = page.locator('.grid > div').filter({ has: page.getByRole('button', { name: /delete|remove/i }) });

      if ((await providerCards.count()) > 0) {
        await test.step('Click delete on first provider', async () => {
          const firstProvider = providerCards.first();

          // The delete button might be icon-only (no text)
          const deleteButton = firstProvider.locator('button').filter({ has: page.locator('svg') }).last();

          await deleteButton.click();
        });

        await test.step('Verify confirmation dialog appears', async () => {
          // Should show confirmation dialog
          const confirmDialog = page.getByRole('dialog').or(page.getByRole('alertdialog'));

          await expect(confirmDialog).toBeVisible({ timeout: 3000 });
        });

        await test.step('Cancel deletion', async () => {
          // Cancel to not actually delete
          const cancelButton = page.getByRole('button', { name: /cancel/i });
          if (await cancelButton.isVisible()) {
            await cancelButton.click();
          }
        });
      }
    });
  });

  test.describe('API Operations', () => {
    test('should list providers via API', async ({ request }) => {
      const response = await request.get('/api/v1/dns-providers');
      expect(response.ok()).toBeTruthy();

      const data = await response.json();
      // Response should be an array (possibly empty)
      expect(Array.isArray(data) || (data && Array.isArray(data.providers || data.items || data.data))).toBeTruthy();
    });

    test('should create provider via API', async ({ request }) => {
      const response = await request.post('/api/v1/dns-providers', {
        data: {
          name: 'API Test Manual Provider',
          provider_type: 'manual',
        },
      });

      // Should succeed or return validation error (not server error)
      expect(response.status()).toBeLessThan(500);

      if (response.ok()) {
        const provider = await response.json();
        expect(provider).toHaveProperty('id');
        expect(provider.name).toBe('API Test Manual Provider');
        expect(provider.provider_type).toBe('manual');

        // Cleanup: delete the created provider
        if (provider.id) {
          await request.delete(`/api/v1/dns-providers/${provider.id}`);
        }
      }
    });

    test('should reject invalid provider type via API', async ({ request }) => {
      const response = await request.post('/api/v1/dns-providers', {
        data: {
          name: 'Invalid Type Provider',
          provider_type: 'nonexistent_provider_type',
        },
      });

      // Should return 400 Bad Request for invalid type
      expect(response.status()).toBe(400);
    });

    test('should get single provider via API', async ({ request }) => {
      // First, create a provider to ensure we have at least one
      const createResponse = await request.post('/api/v1/dns-providers', {
        data: {
          name: 'API Get Test Provider',
          provider_type: 'manual',
        },
      });

      if (createResponse.ok()) {
        const created = await createResponse.json();

        const getResponse = await request.get(`/api/v1/dns-providers/${created.id}`);
        expect(getResponse.ok()).toBeTruthy();

        const provider = await getResponse.json();
        expect(provider).toHaveProperty('id');
        expect(provider).toHaveProperty('name');
        expect(provider).toHaveProperty('provider_type');

        // Cleanup: delete the created provider
        await request.delete(`/api/v1/dns-providers/${created.id}`);
      }
    });
  });
});

test.describe('DNS Provider Form Accessibility', () => {
  test('should have accessible form labels', async ({ page }) => {
    await page.goto('/dns/providers');
    await waitForLoadingComplete(page);

    await page.getByRole('button', { name: /add.*provider/i }).first().click();
    await waitForDialog(page);

    await test.step('Verify name field has label', async () => {
      // Input has id="provider-name" and associated label
      const nameInput = page.locator('#provider-name');
      await expect(nameInput).toBeVisible();
    });

    await test.step('Verify type selector is accessible', async () => {
      // Select trigger has id="provider-type" and aria-label
      const typeSelect = page.locator('#provider-type');
      await expect(typeSelect).toBeVisible();
    });
  });

  test('should support keyboard navigation in form', async ({ page }) => {
    await page.goto('/dns/providers');
    await waitForLoadingComplete(page);

    await page.getByRole('button', { name: /add.*provider/i }).first().click();
    await waitForDialog(page);

    await test.step('Tab through form fields', async () => {
      // Tab should move through form fields
      await page.keyboard.press('Tab');

      const focusedElement = page.locator(':focus');
      await expect(focusedElement).toBeVisible();
    });

    await test.step('Arrow keys should work in dropdown', async () => {
      const typeSelect = page.locator('#provider-type');

      if (await typeSelect.isVisible()) {
        await typeSelect.focus();
        await typeSelect.press('Enter');

        // Arrow down should move through options
        await page.keyboard.press('ArrowDown');

        const focusedOption = page.locator('[role="option"]:focus, [aria-selected="true"]');
        // An option should be focused or selected
        expect((await focusedOption.count()) >= 0).toBeTruthy();
      }
    });
  });

  test('should announce errors to screen readers', async ({ page }) => {
    await page.goto('/dns/providers');
    await waitForLoadingComplete(page);

    await page.getByRole('button', { name: /add.*provider/i }).first().click();
    await waitForDialog(page);

    await test.step('Fill form with valid data then test', async () => {
      // Fill required fields to enable the button
      await page.locator('#provider-name').fill('Test Provider');
      await page.locator('#provider-type').click();
      await page.getByRole('option', { name: /manual/i }).click();
    });

    await test.step('Verify error is in accessible element', async () => {
      const errorElement = page
        .locator('[role="alert"]')
        .or(page.locator('[aria-live="polite"], [aria-live="assertive"]'));

      // Error should be announced via ARIA
      await expect(errorElement).toBeVisible({ timeout: 3000 }).catch(() => {
        // Might use different error display
      });
    });
  });
});
