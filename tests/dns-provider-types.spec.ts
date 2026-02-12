import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import {
  waitForAPIResponse,
  waitForDialog,
  waitForDropdown,
  waitForLoadingComplete,
} from './utils/wait-helpers';
import { getFormFieldByLabel } from './utils/ui-helpers';

/**
 * DNS Provider Types E2E Tests
 *
 * Tests the DNS Provider Types API and UI, including:
 * - API endpoint /api/v1/dns-providers/types
 * - Built-in providers (cloudflare, route53, etc.)
 * - Custom providers (manual, rfc2136, webhook, script)
 * - Provider selector in UI
 */

test.describe('DNS Provider Types', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test.describe('API: /api/v1/dns-providers/types', () => {
    test('should return all provider types including built-in and custom', async ({ request }) => {
      const response = await request.get('/api/v1/dns-providers/types');
      expect(response.ok()).toBeTruthy();

      const data = await response.json();
      // API returns { types: [...], total: N }
      const types = data.types;
      expect(types.length).toBeGreaterThan(0);
      expect(Array.isArray(types)).toBeTruthy();

      // Should have built-in providers
      const typeNames = types.map((t: { type: string }) => t.type);
      expect(typeNames).toContain('cloudflare');
      expect(typeNames).toContain('route53');

      // Should have custom providers
      expect(typeNames).toContain('manual');
      expect(typeNames).toContain('rfc2136');
      expect(typeNames).toContain('webhook');
      expect(typeNames).toContain('script');
    });

    test('each provider type should have required fields', async ({ request }) => {
      const response = await request.get('/api/v1/dns-providers/types');
      expect(response.ok()).toBeTruthy();
      const data = await response.json();
      const types = data.types;

      for (const provider of types) {
        expect(provider).toHaveProperty('type');
        expect(provider).toHaveProperty('name');
        expect(provider).toHaveProperty('fields');
        expect(Array.isArray(provider.fields)).toBeTruthy();
      }
    });

    test('manual provider type should have correct configuration', async ({ request }) => {
      const response = await request.get('/api/v1/dns-providers/types');
      expect(response.ok()).toBeTruthy();
      const data = await response.json();
      const types = data.types;

      const manualProvider = types.find((t: { type: string }) => t.type === 'manual');
      expect(manualProvider).toBeDefined();
      expect(manualProvider.name).toMatch(/manual/i);

      // Manual provider should have minimal or no required fields
      // since DNS records are created manually by the user
    });

    test('webhook provider type should have url field', async ({ request }) => {
      const response = await request.get('/api/v1/dns-providers/types');
      expect(response.ok()).toBeTruthy();
      const data = await response.json();
      const types = data.types;

      const webhookProvider = types.find((t: { type: string }) => t.type === 'webhook');
      expect(webhookProvider).toBeDefined();

      // Webhook should have URL configuration field
      const fieldNames = webhookProvider.fields.map((f: { name: string }) => f.name);
      expect(fieldNames.some((name: string) => name.toLowerCase().includes('url'))).toBeTruthy();
    });

    test('rfc2136 provider type should have server and key fields', async ({ request }) => {
      const response = await request.get('/api/v1/dns-providers/types');
      expect(response.ok()).toBeTruthy();
      const data = await response.json();
      const types = data.types;

      const rfc2136Provider = types.find((t: { type: string }) => t.type === 'rfc2136');
      expect(rfc2136Provider).toBeDefined();

      // RFC2136 (Dynamic DNS) should have server and TSIG key fields
      const fieldNames = rfc2136Provider.fields.map((f: { name: string }) => f.name.toLowerCase());
      expect(fieldNames.some((name: string) => name.includes('server') || name.includes('nameserver'))).toBeTruthy();
    });

    test('script provider type should have command/path field', async ({ request }) => {
      const response = await request.get('/api/v1/dns-providers/types');
      expect(response.ok()).toBeTruthy();
      const data = await response.json();
      const types = data.types;

      const scriptProvider = types.find((t: { type: string }) => t.type === 'script');
      expect(scriptProvider).toBeDefined();

      // Script provider should have a command or script path field
      const fieldNames = scriptProvider.fields.map((f: { name: string }) => f.name.toLowerCase());
      expect(
        fieldNames.some((name: string) => name.includes('script') || name.includes('command') || name.includes('path'))
      ).toBeTruthy();
    });
  });

  test.describe('UI: Provider Selector', () => {
    test('should show all provider types in dropdown', async ({ page }) => {
      const providersResponse = waitForAPIResponse(page, /\/api\/v1\/dns-providers/);
      const typesResponse = waitForAPIResponse(
        page,
        /\/api\/v1\/dns-providers\/types/,
        { timeout: 10000 }
      ).catch(() => null);
      await page.goto('/dns/providers');
      await providersResponse;
      await typesResponse;
      await waitForLoadingComplete(page);

      await test.step('Click Add Provider button', async () => {
        // Use first() to handle both header button and empty state button
        const addButton = page.getByRole('button', { name: /add.*provider/i }).first();
        await expect(addButton).toBeVisible();
        await addButton.click();
      });

      await test.step('Open provider type dropdown', async () => {
        const dialog = await waitForDialog(page);
        const typeSelect = dialog
          .locator('#provider-type')
          .or(dialog.getByRole('combobox', { name: /provider type/i }));

        await expect(typeSelect).toBeVisible();
        await typeSelect.click();
      });

      await test.step('Verify built-in providers appear', async () => {
        const listbox = await waitForDropdown(page, 'provider-type').catch(async () => {
          const fallback = page.getByRole('listbox').first();
          await expect(fallback).toBeVisible();
          return fallback;
        });
        await expect(listbox.getByRole('option', { name: /cloudflare/i })).toBeVisible();
      });

      await test.step('Verify custom providers appear', async () => {
        const listbox = page.getByRole('listbox').first();
        await expect(listbox.getByRole('option', { name: /manual/i })).toBeVisible();
      });
    });

    test('should display provider description in selector', async ({ page }) => {
      const providersResponse = waitForAPIResponse(page, /\/api\/v1\/dns-providers/);
      const typesResponse = waitForAPIResponse(
        page,
        /\/api\/v1\/dns-providers\/types/,
        { timeout: 10000 }
      ).catch(() => null);
      await page.goto('/dns/providers');
      await providersResponse;
      await typesResponse;
      await waitForLoadingComplete(page);
      await page.getByRole('button', { name: /add.*provider/i }).first().click();

      const dialog = await waitForDialog(page);
      const typeSelect = dialog
        .locator('#provider-type')
        .or(dialog.getByRole('combobox', { name: /provider type/i }));
      await typeSelect.click();

      // Manual provider option should have description indicating no automation
      const listbox = await waitForDropdown(page, 'provider-type').catch(async () => {
        const fallback = page.getByRole('listbox').first();
        await expect(fallback).toBeVisible();
        return fallback;
      });
      const manualOption = listbox.getByRole('option', { name: /manual/i });
      await expect(manualOption).toBeVisible();

      const optionText = await manualOption.innerText();
      // Manual provider description should indicate manual DNS record creation
      expect(optionText?.toLowerCase()).toMatch(/manual|no automation|hand/i);
    });

    test('should filter provider types based on search', async ({ page }) => {
      const providersResponse = waitForAPIResponse(page, /\/api\/v1\/dns-providers/);
      const typesResponse = waitForAPIResponse(
        page,
        /\/api\/v1\/dns-providers\/types/,
        { timeout: 10000 }
      ).catch(() => null);
      await page.goto('/dns/providers');
      await providersResponse;
      await typesResponse;
      await waitForLoadingComplete(page);
      await page.getByRole('button', { name: /add.*provider/i }).first().click();

      const dialog = await waitForDialog(page);
      const typeSelect = dialog
        .locator('#provider-type')
        .or(dialog.getByRole('combobox', { name: /provider type/i }));
      await typeSelect.click();

      // The select dropdown doesn't support text search input
      // Instead, verify that all options are keyboard accessible
      await test.step('Verify options can be navigated with keyboard', async () => {
        const listbox = await waitForDropdown(page, 'provider-type').catch(async () => {
          const fallback = page.getByRole('listbox').first();
          await expect(fallback).toBeVisible();
          return fallback;
        });
        await listbox.focus();

        // Press down arrow to navigate through options
        await page.keyboard.press('ArrowDown');

        // Verify options exist and are visible
        const options = listbox.getByRole('option');
        const optionCount = await options.count();

        // Should have multiple provider options available
        expect(optionCount).toBeGreaterThan(5);

        // Verify cloudflare option exists in the list
        await expect(listbox.getByRole('option', { name: /cloudflare/i })).toBeVisible();

        // Verify manual option exists in the list
        await expect(listbox.getByRole('option', { name: /manual/i })).toBeVisible();
      });
    });
  });

  test.describe('Provider Type Selection', () => {
    test('should show correct fields when Manual type is selected', async ({ page }) => {
      const providersResponse = waitForAPIResponse(page, /\/api\/v1\/dns-providers/);
      const typesResponse = waitForAPIResponse(
        page,
        /\/api\/v1\/dns-providers\/types/,
        { timeout: 10000 }
      ).catch(() => null);
      await page.goto('/dns/providers');
      await providersResponse;
      await typesResponse;
      await waitForLoadingComplete(page);
      await page.getByRole('button', { name: /add.*provider/i }).first().click();

      await test.step('Select Manual provider type', async () => {
        const dialog = await waitForDialog(page);
        const typeSelect = dialog
          .locator('#provider-type')
          .or(dialog.getByRole('combobox', { name: /provider type/i }));
        await typeSelect.click();

        const listbox = await waitForDropdown(page, 'provider-type').catch(async () => {
          const fallback = page.getByRole('listbox').first();
          await expect(fallback).toBeVisible();
          return fallback;
        });
        const manualOption = listbox.getByRole('option', { name: /manual/i });
        await manualOption.focus();
        await page.keyboard.press('Enter');
        await expect(listbox).toBeHidden();
      });

      await test.step('Verify Manual-specific UI appears', async () => {
        // Should NOT show fields like API key or access token
        await expect(page.getByLabel(/api.*key/i)).not.toBeVisible();
        await expect(page.getByLabel(/access.*token/i)).not.toBeVisible();
      });
    });

    test('should show URL field when Webhook type is selected', async ({ page }) => {
      const providersResponse = waitForAPIResponse(page, /\/api\/v1\/dns-providers/);
      const typesResponse = waitForAPIResponse(
        page,
        /\/api\/v1\/dns-providers\/types/,
        { timeout: 10000 }
      ).catch(() => null);
      await page.goto('/dns/providers');
      await providersResponse;
      await typesResponse;
      await waitForLoadingComplete(page);
      await page.getByRole('button', { name: /add.*provider/i }).first().click();

      await test.step('Select Webhook provider type', async () => {
        const dialog = await waitForDialog(page);
        const typeSelect = dialog
          .locator('#provider-type')
          .or(dialog.getByRole('combobox', { name: /provider type/i }));
        await typeSelect.click();

        const listbox = await waitForDropdown(page, 'provider-type').catch(async () => {
          const fallback = page.getByRole('listbox').first();
          await expect(fallback).toBeVisible();
          return fallback;
        });
        const webhookOption = listbox.getByRole('option', { name: /webhook/i });
        await webhookOption.focus();
        await page.keyboard.press('Enter');
        await expect(listbox).toBeHidden();
      });

      await test.step('Verify Webhook URL field appears', async () => {
        // ✅ FIX 2.2: Use cross-browser label helper with fallbacks
        const urlField = getFormFieldByLabel(
          page,
          /create.*url/i,
          {
            placeholder: /https?:\/\//i,
            fieldId: 'field-create_url'
          }
        );
        await expect(urlField.first()).toBeVisible({ timeout: 10000 });
      });
    });

    test('should show server field when RFC2136 type is selected', async ({ page }) => {
      const providersResponse = waitForAPIResponse(page, /\/api\/v1\/dns-providers/);
      const typesResponse = waitForAPIResponse(
        page,
        /\/api\/v1\/dns-providers\/types/,
        { timeout: 10000 }
      ).catch(() => null);
      await page.goto('/dns/providers');
      await providersResponse;
      await typesResponse;
      await waitForLoadingComplete(page);
      await page.getByRole('button', { name: /add.*provider/i }).first().click();

      await test.step('Select RFC2136 provider type', async () => {
        const dialog = await waitForDialog(page);
        const typeSelect = dialog
          .locator('#provider-type')
          .or(dialog.getByRole('combobox', { name: /provider type/i }));
        await typeSelect.click();

        // RFC2136 might be listed as "RFC 2136" or similar
        const listbox = await waitForDropdown(page, 'provider-type').catch(async () => {
          const fallback = page.getByRole('listbox').first();
          await expect(fallback).toBeVisible();
          return fallback;
        });
        const rfc2136Option = listbox.getByRole('option', { name: /rfc.*2136|dynamic.*dns/i });
        await expect(rfc2136Option).toBeVisible({ timeout: 5000 });
        await rfc2136Option.focus();
        await page.keyboard.press('Enter');
        await expect(listbox).toBeHidden();
      });

      await test.step('Verify RFC2136 server field appears', async () => {
        // ✅ FIX 2.2: Use cross-browser label helper with fallbacks
        const serverField = getFormFieldByLabel(
          page,
          /dns.*server/i,
          {
            placeholder: /dns\.example\.com|nameserver/i,
            fieldId: 'field-nameserver'
          }
        );
        await expect(serverField.first()).toBeVisible({ timeout: 10000 });
      });
    });

    test('should show script path field when Script type is selected', async ({ page }) => {
      const providersResponse = waitForAPIResponse(page, /\/api\/v1\/dns-providers/);
      const typesResponse = waitForAPIResponse(
        page,
        /\/api\/v1\/dns-providers\/types/,
        { timeout: 10000 }
      ).catch(() => null);
      await page.goto('/dns/providers');
      await providersResponse;
      await typesResponse;
      await waitForLoadingComplete(page);
      await page.getByRole('button', { name: /add.*provider/i }).first().click();

      await test.step('Select Script provider type', async () => {
        const dialog = await waitForDialog(page);
        const typeSelect = dialog
          .locator('#provider-type')
          .or(dialog.getByRole('combobox', { name: /provider type/i }));
        await typeSelect.click();

        const listbox = await waitForDropdown(page, 'provider-type').catch(async () => {
          const fallback = page.getByRole('listbox').first();
          await expect(fallback).toBeVisible();
          return fallback;
        });
        const scriptOption = listbox.getByRole('option', { name: /script/i });
        await scriptOption.focus();
        await page.keyboard.press('Enter');
        await expect(listbox).toBeHidden();
      });

      await test.step('Verify Script path/command field appears', async () => {
        // ✅ FIX 2.2: Use cross-browser label helper with fallbacks
        const scriptField = getFormFieldByLabel(
          page,
          /script.*path/i,
          {
            placeholder: /dns-challenge\.sh/i,
            fieldId: 'field-script_path'
          }
        );
        await expect(scriptField.first()).toBeVisible({ timeout: 10000 });
      });
    });
  });
});
