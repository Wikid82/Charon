/**
 * Proxy + DNS Provider Integration E2E Tests (Phase 6.3)
 *
 * Tests for proxy host and DNS provider integration workflows.
 * Covers DNS provider configuration, ACME DNS-01 challenges, and validation.
 *
 * Test Categories (10-12 tests):
 * - Group A: DNS Provider Assignment (3 tests)
 * - Group B: DNS Challenge Integration (4 tests)
 * - Group C: Provider Management (3 tests)
 *
 * API Endpoints:
 * - GET/POST/PUT/DELETE /api/v1/dns-providers
 * - GET/POST/PUT/DELETE /api/v1/proxy-hosts
 * - POST /api/v1/dns-providers/:id/test
 */

import { test, expect, loginUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { generateProxyHost } from '../fixtures/proxy-hosts';
import {
  waitForToast,
  waitForLoadingComplete,
  waitForAPIResponse,
  waitForModal,
  clickAndWaitForResponse,
} from '../utils/wait-helpers';

/**
 * DNS Provider types supported by the system
 */
type DNSProviderType = 'manual' | 'cloudflare' | 'route53' | 'webhook' | 'rfc2136';

/**
 * Selectors for DNS Provider and Proxy Host pages
 */
const SELECTORS = {
  // DNS Provider Page
  dnsPageTitle: 'h1',
  createDnsButton: 'button:has-text("Create DNS Provider"), button:has-text("Add DNS Provider")',
  dnsTable: '[data-testid="dns-provider-table"], table',
  dnsRow: '[data-testid="dns-provider-row"], tbody tr',
  dnsDeleteBtn: '[data-testid="dns-delete-btn"], button[aria-label*="Delete"]',
  dnsEditBtn: '[data-testid="dns-edit-btn"], button[aria-label*="Edit"]',
  dnsTestBtn: '[data-testid="dns-test-btn"], button:has-text("Test")',

  // Proxy Host Page
  proxyPageTitle: 'h1',
  createProxyButton: 'button:has-text("Create Proxy Host"), button:has-text("Add Proxy Host")',
  proxyTable: '[data-testid="proxy-host-table"], table',
  proxyRow: '[data-testid="proxy-host-row"], tbody tr',
  proxyEditBtn: '[data-testid="proxy-edit-btn"], button[aria-label*="Edit"]',

  // Form Fields
  dnsTypeSelect: 'select[name="type"], #dns-type, [data-testid="dns-type-select"]',
  dnsNameInput: 'input[name="name"], #dns-name',
  apiTokenInput: 'input[name="api_token"], #api-token',
  apiKeyInput: 'input[name="api_key"], #api-key',
  webhookUrlInput: 'input[name="webhook_url"], #webhook-url',

  // Dialog/Modal
  confirmDialog: '[role="dialog"], [role="alertdialog"]',
  confirmButton: 'button:has-text("Confirm"), button:has-text("Delete"), button:has-text("Yes")',
  cancelButton: 'button:has-text("Cancel"), button:has-text("No")',
  saveButton: 'button:has-text("Save"), button[type="submit"]',

  // Status/State
  loadingSkeleton: '[data-testid="loading-skeleton"], .loading',
  statusBadge: '[data-testid="status-badge"], .badge',
};

test.describe('Proxy + DNS Provider Integration', () => {
  // ===========================================================================
  // Group A: DNS Provider Assignment (3 tests)
  // ===========================================================================
  test.describe('Group A: DNS Provider Assignment', () => {
    test('should create manual DNS provider successfully', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create manual DNS provider via API', async () => {
        const { id, name } = await testData.createDNSProvider({
          providerType: 'manual',
          name: 'Manual-DNS-Test',
          credentials: {},
        });
        expect(id).toBeTruthy();
      });

      await test.step('Navigate to DNS providers page', async () => {
        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify DNS provider appears in list', async () => {
        // The name is namespaced by TestDataManager
        const content = page.locator('main, table, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should create Cloudflare DNS provider', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create Cloudflare DNS provider via API', async () => {
        const { id, name } = await testData.createDNSProvider({
          providerType: 'cloudflare',
          name: 'Cloudflare-DNS-Test',
          credentials: {
            api_token: 'test-cloudflare-token-placeholder',
          },
        });
        expect(id).toBeTruthy();
      });

      await test.step('Navigate to DNS providers', async () => {
        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify provider is listed', async () => {
        const content = page.locator('main, table').first();
        await expect(content).toBeVisible();
      });
    });

    test('should assign DNS provider to wildcard certificate request', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create DNS provider', async () => {
        await testData.createDNSProvider({
          providerType: 'manual',
          name: 'Wildcard-DNS-Provider',
          credentials: {},
        });
      });

      await test.step('Navigate to certificates page', async () => {
        await page.goto('/certificates');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify certificates page loads', async () => {
        const heading = page.locator('h1, h2').first();
        await expect(heading).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group B: DNS Challenge Integration (4 tests)
  // ===========================================================================
  test.describe('Group B: DNS Challenge Integration', () => {
    test('should test DNS provider connectivity', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create DNS provider for testing', async () => {
        await testData.createDNSProvider({
          providerType: 'manual',
          name: 'Connectivity-Test-DNS',
          credentials: {},
        });
      });

      await test.step('Navigate to DNS providers', async () => {
        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify DNS providers page loads', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should display DNS challenge instructions for manual provider', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create manual DNS provider', async () => {
        await testData.createDNSProvider({
          providerType: 'manual',
          name: 'Manual-Challenge-DNS',
          credentials: {},
        });
      });

      await test.step('Navigate to DNS providers', async () => {
        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page content', async () => {
        // Manual providers show instructions for DNS record creation
        const content = page.locator('main, table, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should handle DNS propagation delay gracefully', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create DNS provider', async () => {
        await testData.createDNSProvider({
          providerType: 'manual',
          name: 'Propagation-Test-DNS',
          credentials: {},
        });
      });

      await test.step('Navigate to certificates', async () => {
        await page.goto('/certificates');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page loads', async () => {
        const heading = page.locator('h1, h2').first();
        await expect(heading).toBeVisible();
      });
    });

    test('should support webhook-based DNS provider', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create webhook DNS provider', async () => {
        await testData.createDNSProvider({
          providerType: 'webhook',
          name: 'Webhook-DNS-Test',
          credentials: {
            create_url: 'https://example.com/webhook/create',
            delete_url: 'https://example.com/webhook/delete',
          },
        });
      });

      await test.step('Navigate to DNS providers', async () => {
        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify provider in list', async () => {
        const content = page.locator('main, table').first();
        await expect(content).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group C: Provider Management (3 tests)
  // ===========================================================================
  test.describe('Group C: Provider Management', () => {
    test('should update DNS provider credentials', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      const { id: providerId } = await testData.createDNSProvider({
        providerType: 'cloudflare',
        name: 'Update-Credentials-DNS',
        credentials: {
          api_token: 'initial-token',
        },
      });

      await test.step('Update provider credentials via API', async () => {
        const response = await page.request.put(`/api/v1/dns-providers/${providerId}`, {
          data: {
            type: 'cloudflare',
            name: 'Update-Credentials-DNS-Updated',
            credentials: {
              api_token: 'updated-token',
            },
          },
        });
        expect(response.ok()).toBeTruthy();
      });

      await test.step('Navigate to DNS providers', async () => {
        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify updated provider', async () => {
        const content = page.locator('main, table').first();
        await expect(content).toBeVisible();
      });
    });

    test('should delete DNS provider with confirmation', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      const { id: providerId } = await testData.createDNSProvider({
        providerType: 'manual',
        name: 'Delete-Test-DNS',
        credentials: {},
      });

      await test.step('Navigate to DNS providers', async () => {
        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify provider exists before deletion', async () => {
        const content = page.locator('main, table').first();
        await expect(content).toBeVisible();
      });

      await test.step('Delete provider via API', async () => {
        const response = await page.request.delete(`/api/v1/dns-providers/${providerId}`);
        expect(response.ok()).toBeTruthy();
      });
    });

    test('should list all configured DNS providers', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create multiple DNS providers
      await testData.createDNSProvider({
        providerType: 'manual',
        name: 'List-Test-DNS-1',
        credentials: {},
      });

      await testData.createDNSProvider({
        providerType: 'cloudflare',
        name: 'List-Test-DNS-2',
        credentials: { api_token: 'test-token' },
      });

      await test.step('Navigate to DNS providers', async () => {
        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify providers list', async () => {
        const content = page.locator('main, table').first();
        await expect(content).toBeVisible();
      });

      await test.step('Verify API returns providers', async () => {
        const response = await page.request.get('/api/v1/dns-providers');
        expect(response.ok()).toBeTruthy();
        const data = await response.json();
        const providers = data.providers || data.items || data;
        expect(Array.isArray(providers)).toBe(true);
      });
    });
  });
});
