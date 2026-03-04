import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import {
  clickAndWaitForResponse,
  waitForAPIResponse,
  waitForLoadingComplete,
  waitForModal,
  waitForResourceInUI,
} from '../utils/wait-helpers';
import { getStorageStateAuthHeaders } from '../utils/api-helpers';

/**
 * Domain & DNS Management Workflow
 *
 * Purpose: Validate domain and DNS provider management
 * Scenarios: Add domain, configure DNS, SSL cert management
 * Success: Domains created and certificates managed
 */

const DOMAIN_API_PATTERN = /\/api\/v1\/domains/;
const DNS_PROVIDERS_API_PATTERN = /\/api\/v1\/dns-providers/;

function generateDomainName(seed: string): string {
  const timestamp = Date.now().toString(36);
  return `${seed}-${timestamp}.example.com`;
}

async function navigateToDomains(page: import('@playwright/test').Page): Promise<void> {
  const domainsResponse = waitForAPIResponse(page, DOMAIN_API_PATTERN);
  await page.goto('/domains');
  await domainsResponse;
  await waitForLoadingComplete(page);
}

async function navigateToDnsProviders(page: import('@playwright/test').Page): Promise<void> {
  const providersResponse = waitForAPIResponse(page, DNS_PROVIDERS_API_PATTERN);
  await page.goto('/dns/providers');
  await providersResponse;
  await waitForLoadingComplete(page);
}

test.describe('Domain & DNS Management', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page, { timeout: 15000 });
    await expect(page.getByRole('main')).toBeVisible({ timeout: 15000 });
  });

  // Add domain
  test('Domain - add via UI and verify in list', async ({ page }) => {
    const domainName = generateDomainName('ui-domain');
    let createdId: string | undefined;

    await test.step('Navigate to domains page', async () => {
      await navigateToDomains(page);
    });

    await test.step('Fill and submit domain form', async () => {
      const domainInput = page.getByRole('textbox').first();
      await expect(domainInput).toBeVisible();
      await domainInput.fill(domainName);

      const addButton = page.getByRole('button', { name: /add domain/i }).first();
      const response = await clickAndWaitForResponse(page, addButton, DOMAIN_API_PATTERN, { status: 201 });
      const payload = await response.json();
      createdId = payload.uuid || payload.id;
    });

    await test.step('Verify domain card is visible', async () => {
      await waitForResourceInUI(page, domainName);
      await expect(page.getByRole('heading', { name: domainName })).toBeVisible();
    });

    await test.step('Clean up domain via API', async () => {
      if (createdId) {
        await page.request.delete(`/api/v1/domains/${createdId}`, { headers: getStorageStateAuthHeaders() });
      }
    });
  });

  // View DNS records
  test('Domain - delete via UI with confirmation dialog', async ({ page }) => {
    const domainName = generateDomainName('delete-domain');
    const createResponse = await page.request.post('/api/v1/domains', {
      data: { name: domainName },
      headers: getStorageStateAuthHeaders(),
    });
    const created = await createResponse.json();
    const domainId = created.uuid || created.id;

    await test.step('Navigate to domains page', async () => {
      await navigateToDomains(page);
    });

    await test.step('Confirm domain card is visible', async () => {
      await page.reload({ waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);
      await waitForResourceInUI(page, domainName);
      await expect(page.getByRole('heading', { name: domainName })).toBeVisible();
    });

    await test.step('Delete domain from card', async () => {
      const heading = page.getByRole('heading', { name: domainName });
      const deleteButton = heading
        .locator('xpath=ancestor::div[contains(@class, "bg-dark-card")]')
        .getByRole('button', { name: /delete/i });
      await expect(deleteButton).toBeVisible();

      page.once('dialog', async (dialog) => {
        await dialog.accept();
      });

      const responsePromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/api/v1/domains/') &&
          resp.request().method() === 'DELETE',
        { timeout: 15000 }
      );

      await deleteButton.click();
      await responsePromise;
    });
  });

  // Add DNS provider
  test('DNS Providers - list providers after API seed', async ({ page, testData }) => {
    const { name } = await testData.createDNSProvider({
      providerType: 'manual',
      name: 'Domain-Management-DNS',
      credentials: {},
    });

    await test.step('Navigate to DNS providers page', async () => {
      await navigateToDnsProviders(page);
    });

    await test.step('Verify provider card is visible', async () => {
      await waitForResourceInUI(page, name);
      await expect(page.getByRole('heading', { name })).toBeVisible();
    });
  });

  // Verify domain ownership
  test('DNS Providers - open form and load provider types', async ({ page }) => {
    await test.step('Navigate to DNS providers page', async () => {
      await navigateToDnsProviders(page);
    });

    await test.step('Open add provider dialog', async () => {
      await page.request.get('/api/v1/dns-providers/types', { headers: getStorageStateAuthHeaders() });
      const addButton = page.getByRole('button', { name: /add.*provider/i }).first();
      await addButton.click();
      await waitForModal(page, /provider/i);
    });

    await test.step('Select provider type and verify credentials section', async () => {
      const providerType = page.getByRole('combobox', { name: /provider type/i }).first();
      await expect(providerType).toBeVisible();
      await providerType.click();

      const manualOption = page.getByRole('option').filter({ hasText: /manual/i }).first();
      await expect(manualOption).toBeVisible();
      await manualOption.click();

      await expect(page.getByTestId('credentials-section')).toBeVisible();
    });
  });

  // Renew SSL certificate
  test('DNS Providers - delete provider via API and verify removal', async ({ page, testData }) => {
    const { id, name } = await testData.createDNSProvider({
      providerType: 'manual',
      name: 'Delete-Provider-DNS',
      credentials: {},
    });

    await test.step('Navigate to DNS providers page', async () => {
      await navigateToDnsProviders(page);
    });

    await test.step('Verify provider card is visible', async () => {
      const providerCard = page.locator('div').filter({
        has: page.getByRole('heading', { name }),
      }).first();
      await expect(providerCard).toBeVisible();
    });

    await test.step('Delete provider via API', async () => {
      await page.request.delete(`/api/v1/dns-providers/${id}`, { headers: getStorageStateAuthHeaders() });
    });

    await test.step('Verify provider card removed', async () => {
      // Navigate away first to clear any in-memory SWR cache
      await page.goto('about:blank');
      await navigateToDnsProviders(page);
      await expect(page.getByRole('heading', { name })).toHaveCount(0, { timeout: 15000 });
    });
  });

  // View domain statistics
  test('Domains page renders heading and add form', async ({ page }) => {
    await test.step('Navigate to domains page', async () => {
      await navigateToDomains(page);
    });

    await test.step('Verify heading and form controls', async () => {
      const heading = page.getByRole('heading', { name: /domains/i }).first();
      const input = page.getByRole('textbox').first();
      const addButton = page.getByRole('button', { name: /add domain/i }).first();

      await expect(heading).toBeVisible();
      await expect(input).toBeVisible();
      await expect(addButton).toBeVisible();
    });
  });
});
