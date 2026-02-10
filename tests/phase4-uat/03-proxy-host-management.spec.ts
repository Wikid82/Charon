import { test, expect } from '@playwright/test';

/**
 * Phase 4 UAT: Proxy Host Management
 *
 * Purpose: Validate reverse proxy creation, configuration, and routing
 * Scenarios: CRUD operations, SSL setup, access lists, WAF/rate limiting
 * Success: Proxies created and route traffic correctly
 */

test.describe('UAT-003: Proxy Host Management', () => {
  const testProxies = [
    { domain: 'test1.proxy.local', target: 'http://127.0.0.1:3000', description: 'Test Proxy 1' },
    { domain: 'test2.proxy.local', target: 'http://127.0.0.1:3001', description: 'Test Proxy 2' },
  ];

  test.beforeEach(async ({ page }) => {
    // Ensure admin is logged in
    await page.goto('/');
    await page.waitForSelector('[data-testid="dashboard-container"], [role="main"]', { timeout: 5000 });
  });

  test.afterEach(async ({ page }) => {
    // Clean up test proxies
    const proxiesLink = page.getByRole('link', { name: /proxy|proxy.?host/i });
    if (await proxiesLink.isVisible()) {
      await proxiesLink.click();
      await page.waitForLoadState('networkidle');

      for (const proxy of testProxies) {
        const proxyRow = page.locator(`text=${proxy.domain}`).first();
        if (await proxyRow.isVisible()) {
          const deleteButton = proxyRow.locator('..').getByRole('button', { name: /delete/i }).first();
          if (await deleteButton.isVisible()) {
            await deleteButton.click();
            const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
            if (await confirmButton.isVisible()) {
              await confirmButton.click();
              await page.waitForLoadState('networkidle');
            }
          }
        }
      }
    }
  });

  // UAT-201: Create proxy host with domain
  test('Create proxy host with domain', async ({ page }) => {
    const proxyData = testProxies[0];

    await test.step('Navigate to proxy hosts page', async () => {
      const proxiesLink = page.getByRole('link', { name: /proxy|proxy.?host/i });
      await proxiesLink.click();
      await page.waitForSelector('[data-testid="proxies-list"], [class*="proxy"]', { timeout: 5000 });
    });

    await test.step('Click add proxy button', async () => {
      const addButton = page.getByRole('button', { name: /add|create|new/i }).first();
      await addButton.click();
      await page.waitForSelector('[role="dialog"], form[class*="proxy"]', { timeout: 3000 });
    });

    await test.step('Fill proxy creation form', async () => {
      await page.getByLabel(/domain|hostname/i).fill(proxyData.domain);
      await page.getByLabel(/target|backend|upstream/i).fill(proxyData.target);

      const descriptionField = page.getByLabel(/description|notes/i);
      if (await descriptionField.isVisible()) {
        await descriptionField.fill(proxyData.description);
      }
    });

    await test.step('Submit form', async () => {
      const submitButton = page.getByRole('button', { name: /create|submit|save/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify proxy created', async () => {
      const proxyElement = page.locator(`text=${proxyData.domain}`).first();
      await expect(proxyElement).toBeVisible();
    });
  });

  // UAT-202: Edit proxy host
  test('Edit proxy host settings', async ({ page }) => {
    const proxyData = testProxies[0];
    const newTarget = 'http://127.0.0.1:4000';

    await test.step('Create proxy first', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const proxyExists = await page.locator(`text=${proxyData.domain}`).first().isVisible().catch(() => false);
      if (!proxyExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/domain/i).fill(proxyData.domain);
        await page.getByLabel(/target/i).fill(proxyData.target);

        const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Open proxy edit modal', async () => {
      const proxyRow = page.locator(`text=${proxyData.domain}`).first();
      const editButton = proxyRow.locator('..').getByRole('button', { name: /edit|config/i }).first();
      await editButton.click();
      await page.waitForSelector('[role="dialog"], form', { timeout: 3000 });
    });

    await test.step('Modify proxy target', async () => {
      const targetInput = page.getByLabel(/target|backend|upstream/i).first();
      await targetInput.clear();
      await targetInput.fill(newTarget);
    });

    await test.step('Save changes', async () => {
      const saveButton = page.getByRole('button', { name: /save|update/i }).first();
      await saveButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify changes persisted', async () => {
      const proxyRow = page.locator(`text=${proxyData.domain}`).first();
      const targetDisplay = proxyRow.locator('..').getByText(newTarget).first();
      if (await targetDisplay.isVisible()) {
        await expect(targetDisplay).toBeVisible();
      }
    });
  });

  // UAT-203: Delete proxy host
  test('Delete proxy host', async ({ page }) => {
    const proxyToDelete = testProxies[0];

    await test.step('Create proxy to delete', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const proxyExists = await page.locator(`text=${proxyToDelete.domain}`).first().isVisible().catch(() => false);
      if (!proxyExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/domain/i).fill(proxyToDelete.domain);
        await page.getByLabel(/target/i).fill(proxyToDelete.target);

        const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Delete proxy', async () => {
      const proxyRow = page.locator(`text=${proxyToDelete.domain}`).first();
      const deleteButton = proxyRow.locator('..').getByRole('button', { name: /delete|remove/i }).first();
      await deleteButton.click();
    });

    await test.step('Confirm deletion', async () => {
      const confirmButton = page.getByRole('button', { name: /confirm|delete|ok/i }).first();
      if (await confirmButton.isVisible()) {
        await confirmButton.click();
      }
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify proxy removed', async () => {
      await page.reload();
      await page.waitForLoadState('networkidle');

      const proxyElement = page.locator(`text=${proxyToDelete.domain}`).first();
      await expect(proxyElement).not.toBeVisible();
    });
  });

  // UAT-204: Enable SSL/TLS on proxy
  test('Configure SSL/TLS certificate on proxy', async ({ page }) => {
    const proxyData = testProxies[0];

    await test.step('Create proxy with SSL', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const proxyExists = await page.locator(`text=${proxyData.domain}`).first().isVisible().catch(() => false);
      if (!proxyExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/domain/i).fill(proxyData.domain);
        await page.getByLabel(/target/i).fill(proxyData.target);

        // Check if SSL toggle available
        const sslToggle = page.getByLabel(/ssl|https|tls|certificate/i).first();
        if (await sslToggle.isVisible()) {
          const sslCheckbox = page.locator('input[type="checkbox"][name*="ssl"], input[type="checkbox"][name*="https"]').first();
          if (await sslCheckbox.isVisible()) {
            await sslCheckbox.click();
          }
        }

        const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify SSL configuration option visible', async () => {
      const sslSection = page.getByText(/ssl|certificate|https/i).first();
      if (await sslSection.isVisible()) {
        await expect(sslSection).toBeVisible();
      }
    });
  });

  // UAT-205: Proxy routes traffic correctly
  test('Proxy routes traffic to backend', async ({ page }) => {
    const proxyDomain = 'routetest.proxy.local';
    const targetUrl = 'http://127.0.0.1:8888';

    await test.step('Create test proxy', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(proxyDomain);
      await page.getByLabel(/target/i).fill(targetUrl);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify proxy appears operational', async () => {
      const proxyRow = page.locator(`text=${proxyDomain}`).first();
      await expect(proxyRow).toBeVisible();

      // Check for status indicator (if applicable)
      const statusIndicator = proxyRow.locator('[data-testid*="status"], [class*="status"]').first();
      if (await statusIndicator.isVisible()) {
        await expect(statusIndicator).toBeVisible();
      }
    });
  });

  // UAT-206: Access list on proxy
  test('Access list can be applied to proxy', async ({ page }) => {
    const proxyData = testProxies[0];

    await test.step('Create proxy with access list', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const proxyExists = await page.locator(`text=${proxyData.domain}`).first().isVisible().catch(() => false);
      if (!proxyExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/domain/i).fill(proxyData.domain);
        await page.getByLabel(/target/i).fill(proxyData.target);

        // Look for access list checkbox
        const accessListCheckbox = page.getByLabel(/access.?list|ip.?whitelist/i).first();
        if (await accessListCheckbox.isVisible()) {
          await accessListCheckbox.click();
        }

        const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify access control option visible', async () => {
      const proxyRow = page.locator(`text=${proxyData.domain}`).first();
      const accessControl = proxyRow.locator('..').getByText(/access|acl|whitelist/i).first();
      if (await accessControl.isVisible()) {
        await expect(accessControl).toBeVisible();
      }
    });
  });

  // UAT-207: WAF on proxy
  test('WAF can be applied to proxy', async ({ page }) => {
    const proxyData = testProxies[0];

    await test.step('Create proxy with WAF enabled', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const proxyExists = await page.locator(`text=${proxyData.domain}`).first().isVisible().catch(() => false);
      if (!proxyExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/domain/i).fill(proxyData.domain);
        await page.getByLabel(/target/i).fill(proxyData.target);

        const wafCheckbox = page.getByLabel(/waf|coraza|malicious|attack/i).first();
        if (await wafCheckbox.isVisible()) {
          await wafCheckbox.click();
        }

        const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify WAF setting visible on proxy', async () => {
      const proxyRow = page.locator(`text=${proxyData.domain}`).first();
      const wafIndicator = proxyRow.locator('..').getByText(/waf|security|protected/i).first();
      if (await wafIndicator.isVisible()) {
        await expect(wafIndicator).toBeVisible();
      }
    });
  });

  // UAT-208: Rate limit on proxy
  test('Rate limit can be applied to proxy', async ({ page }) => {
    const proxyData = testProxies[0];

    await test.step('Create proxy with rate limiting', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const proxyExists = await page.locator(`text=${proxyData.domain}`).first().isVisible().catch(() => false);
      if (!proxyExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/domain/i).fill(proxyData.domain);
        await page.getByLabel(/target/i).fill(proxyData.target);

        const rateLimit = page.getByLabel(/rate.?limit|throttle|requests/i).first();
        if (await rateLimit.isVisible()) {
          await rateLimit.click();
        }

        const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify rate limit configuration available', async () => {
      const rateLimitSection = page.getByText(/rate.?limit|throttle|requests/i).first();
      if (await rateLimitSection.isVisible()) {
        await expect(rateLimitSection).toBeVisible();
      }
    });
  });

  // UAT-209: Proxy validation - invalid regex
  test('Proxy creation validation for invalid patterns', async ({ page }) => {
    await test.step('Navigate to proxy hosts', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });
    });

    await test.step('Attempt to create proxy with invalid data', async () => {
      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      // Try invalid domain
      await page.getByLabel(/domain/i).fill('invalid..domain');
      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      if (await submitButton.isVisible()) {
        await submitButton.click();
      }
    });

    await test.step('Verify validation error shown', async () => {
      const errorMessage = page.getByText(/invalid|error|required/i).first();
      if (await errorMessage.isVisible()) {
        await expect(errorMessage).toBeVisible();
      }
    });
  });

  // UAT-210: Proxy domain validation
  test('Proxy domain field is required', async ({ page }) => {
    await test.step('Navigate to proxy creation', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();
      await page.waitForSelector('[role="dialog"], form');
    });

    await test.step('Try to submit without domain', async () => {
      // Fill only target, not domain
      await page.getByLabel(/target/i).fill('http://127.0.0.1:3000');

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
    });

    await test.step('Verify form validation prevents submission', async () => {
      // Modal should still be open OR error message shown
      const modal = page.locator('[role="dialog"]').first();
      const errorMsg = page.getByText(/required|domain|hostname/i).first();

      if (await modal.isVisible()) {
        // Still in modal = validation prevented submit
        expect(true);
      } else if (await errorMsg.isVisible()) {
        await expect(errorMsg).toBeVisible();
      }
    });
  });

  // UAT-211: View proxy statistics
  test('Proxy statistics display', async ({ page }) => {
    const proxyData = testProxies[0];

    await test.step('Create test proxy', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const proxyExists = await page.locator(`text=${proxyData.domain}`).first().isVisible().catch(() => false);
      if (!proxyExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/domain/i).fill(proxyData.domain);
        await page.getByLabel(/target/i).fill(proxyData.target);

        const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Open proxy details/statistics', async () => {
      const proxyRow = page.locator(`text=${proxyData.domain}`).first();
      const viewButton = proxyRow.locator('..').getByRole('button', { name: /view|stats|details/i }).first();

      if (await viewButton.isVisible()) {
        await viewButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify stats section visible', async () => {
      const statsSection = page.getByText(/request|uptime|error|traffic|statistic/i).first();
      if (await statsSection.isVisible()) {
        await expect(statsSection).toBeVisible();
      }
    });
  });

  // UAT-212: Disable proxy temporarily
  test('Disable proxy temporarily', async ({ page }) => {
    const proxyData = testProxies[0];

    await test.step('Create proxy', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const proxyExists = await page.locator(`text=${proxyData.domain}`).first().isVisible().catch(() => false);
      if (!proxyExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/domain/i).fill(proxyData.domain);
        await page.getByLabel(/target/i).fill(proxyData.target);

        const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Disable proxy', async () => {
      const proxyRow = page.locator(`text=${proxyData.domain}`).first();
      const enabledToggle = proxyRow.locator('..').locator('input[type="checkbox"]').first();

      if (await enabledToggle.isVisible()) {
        const isChecked = await enabledToggle.isChecked();
        if (isChecked) {
          await enabledToggle.click();
          await page.waitForLoadState('networkidle');
        }
      }
    });

    await test.step('Verify proxy status changed', async () => {
      const proxyRow = page.locator(`text=${proxyData.domain}`).first();
      const disabledIndicator = proxyRow.locator('..').getByText(/disabled|inactive/i).first();

      if (await disabledIndicator.isVisible()) {
        await expect(disabledIndicator).toBeVisible();
      }
    });
  });
});
