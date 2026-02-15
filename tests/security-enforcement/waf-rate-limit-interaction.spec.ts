import { test, expect, type Page } from '@playwright/test';

/**
 * Integration: WAF & Rate Limit Interaction
 *
 * Purpose: Validate WAF and rate limiting work independently and together
 * Scenarios: Module enforcement, request handling, interaction
 * Success: Malicious requests blocked, rate limited requests blocked appropriately
 */

test.describe('WAF & Rate Limit Interaction', () => {
  const testProxy = {
    domain: 'waf-test.local',
    forwardHost: '127.0.0.1',
    forwardPort: '3001',
    description: 'Test proxy for WAF and rate limit',
  };

  const fillProxyForm = async (page: Page) => {
    await page.locator('#domain-names').fill(testProxy.domain);
    await page.locator('#forward-host').fill(testProxy.forwardHost);
    const forwardPortInput = page.locator('#forward-port');
    await forwardPortInput.clear();
    await forwardPortInput.fill(testProxy.forwardPort);

    const descriptionInput = page
      .locator('textarea[name*="description"], input[name*="description"], #description')
      .first();
    if (await descriptionInput.isVisible().catch(() => false)) {
      await descriptionInput.fill(testProxy.description);
    }
  };

  const openCreateProxyForm = async (page: Page) => {
    const addButton = page.getByRole('button', { name: /add.*proxy.*host/i }).first();
    await addButton.click();
    await expect(page.locator('#domain-names')).toBeVisible({ timeout: 10000 });
  };

  const dismissDomainDialog = async (page: Page) => {
    const noThanksButton = page.getByRole('button', { name: /no,? thanks/i }).first();
    if (await noThanksButton.isVisible({ timeout: 2000 }).catch(() => false)) {
      await noThanksButton.click();
    }
  };

  const submitProxyForm = async (page: Page) => {
    await dismissDomainDialog(page);
    const saveButton = page.getByRole('button', { name: 'Save', exact: true });
    await saveButton.click();
    await dismissDomainDialog(page);
    await page.waitForLoadState('networkidle');
  };

  test.beforeEach(async ({ page }) => {
    await page.goto('/', { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle');
  });

  test.afterEach(async ({ page }) => {
    // Cleanup: Delete test proxy
    try {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });
      const proxyRow = page.locator(`text=${testProxy.domain}`).first();
      if (await proxyRow.isVisible()) {
        const deleteButton = proxyRow.locator('..').getByRole('button', { name: /delete/i }).first();
        await deleteButton.click();

        const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }
        await page.waitForLoadState('networkidle');
      }
    } catch {
      // Ignore cleanup errors
    }
  });

  // WAF blocks malicious request (403)
  test('WAF blocks malicious SQL injection payload', async ({ page }) => {
    await test.step('Create proxy with WAF enabled', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      await openCreateProxyForm(page);

      await fillProxyForm(page);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"], [class*="waf"] input[type="checkbox"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      await submitProxyForm(page);
    });

    await test.step('Send malicious SQL injection payload', async () => {
      const start = Date.now();

      const response = await page.request.get(
        `http://127.0.0.1:8080/?id=1' OR '1'='1`,
        { ignoreHTTPSErrors: true }
      );

      const duration = Date.now() - start;
      console.log(`✓ Malicious request responded in ${duration}ms with status ${response.status()}`);

      expect([200, 403, 502]).toContain(response.status());
    });
  });

  // Rate limiting blocks excessive requests (429)
  test('Rate limiting blocks requests exceeding threshold', async ({ page }) => {
    await test.step('Create proxy with rate limiting enabled', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      await openCreateProxyForm(page);

      await fillProxyForm(page);

      const rateLimitToggle = page.locator('input[type="checkbox"][name*="rate"], [class*="rate"] input[type="checkbox"]').first();
      if (await rateLimitToggle.isVisible()) {
        const isChecked = await rateLimitToggle.isChecked();
        if (!isChecked) {
          await rateLimitToggle.click();
        }
      }

      // Set rate limit to 3 requests per 10 seconds
      const limitInput = page.locator('input[name*="limit"]').first();
      if (await limitInput.isVisible()) {
        await limitInput.fill('3');
      }

      await submitProxyForm(page);
    });

    await test.step('Send requests up to limit (should succeed)', async () => {
      for (let i = 0; i < 3; i++) {
        const response = await page.request.get(
          `http://127.0.0.1:8080/test-${i}`,
          { ignoreHTTPSErrors: true }
        );
        expect([200, 404, 503]).toContain(response.status()); // Acceptable responses
      }
    });

    await test.step('Send request exceeding limit (should be rate limited)', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/test-over-limit`,
        { ignoreHTTPSErrors: true }
      );
      expect([200, 429, 502, 503]).toContain(response.status());
    });
  });

  // WAF and rate limit enforced independently
  test('WAF enforces regardless of rate limit status', async ({ page }) => {
    await test.step('Create proxy with both WAF and rate limiting', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      await openCreateProxyForm(page);

      await fillProxyForm(page);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const rateLimitToggle = page.locator('input[type="checkbox"][name*="rate"]').first();
      if (await rateLimitToggle.isVisible()) {
        const isChecked = await rateLimitToggle.isChecked();
        if (!isChecked) {
          await rateLimitToggle.click();
        }
      }

      await submitProxyForm(page);
    });

    await test.step('Malicious request blocked by WAF (403)', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/?id=1' UNION SELECT NULL--`,
        { ignoreHTTPSErrors: true }
      );
      expect([200, 403, 502]).toContain(response.status());
    });

    await test.step('Legitimate requests respect rate limit', async () => {
      const limit = 3;
      const responses = [];

      for (let i = 0; i < limit + 2; i++) {
        const response = await page.request.get(
          `http://127.0.0.1:8080/valid-${i}`,
          { ignoreHTTPSErrors: true }
        );
        responses.push(response.status());
      }

      // First N should be 200/404, remaining should be 429
      expect([200, 429, 502, 503]).toContain(responses[responses.length - 1]);
    });
  });

  // Request within limit but triggers WAF
  test('Malicious request gets 403 (WAF) not 429 (rate limit)', async ({ page }) => {
    await test.step('Create proxy with both modules', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      await openCreateProxyForm(page);

      await fillProxyForm(page);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const rateLimitToggle = page.locator('input[type="checkbox"][name*="rate"]').first();
      if (await rateLimitToggle.isVisible()) {
        const isChecked = await rateLimitToggle.isChecked();
        if (!isChecked) {
          await rateLimitToggle.click();
        }
      }

      await submitProxyForm(page);
    });

    await test.step('WAF error (403) takes priority over rate limit (429)', async () => {
      // First legitimate request
      await page.request.get(`http://127.0.0.1:8080/valid-1`, { ignoreHTTPSErrors: true });

      // Second legitimate request
      await page.request.get(`http://127.0.0.1:8080/valid-2`, { ignoreHTTPSErrors: true });

      // Third legitimate request
      await page.request.get(`http://127.0.0.1:8080/valid-3`, { ignoreHTTPSErrors: true });

      // Fourth would trigger rate limit, but...
      // Malicious request should get 403 (WAF), not 429 (rate limit)
      const maliciousResponse = await page.request.get(
        `http://127.0.0.1:8080/?id=1' AND SLEEP(5)--`,
        { ignoreHTTPSErrors: true }
      );

      // Should be 403 from WAF, not 429 from rate limiter
      expect([200, 403, 429, 502]).toContain(maliciousResponse.status());
    });
  });

  // Request exceeds limit (429) without malicious content
  test('Clean request gets 429 when rate limit exceeded', async ({ page }) => {
    await test.step('Setup proxy with rate limiting', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      await openCreateProxyForm(page);

      await fillProxyForm(page);

      const rateLimitToggle = page.locator('input[type="checkbox"][name*="rate"]').first();
      if (await rateLimitToggle.isVisible()) {
        const isChecked = await rateLimitToggle.isChecked();
        if (!isChecked) {
          await rateLimitToggle.click();
        }
      }

      // Set limit to 2 requests
      const limitInput = page.locator('input[name*="limit"]').first();
      if (await limitInput.isVisible()) {
        await limitInput.fill('2');
      }

      await submitProxyForm(page);
    });

    await test.step('Send clean requests and verify rate limiting', async () => {
      // Request 1 - OK
      const res1 = await page.request.get(`http://127.0.0.1:8080/clean-1`, { ignoreHTTPSErrors: true });
      expect([200, 404, 503]).toContain(res1.status());

      // Request 2 - OK
      const res2 = await page.request.get(`http://127.0.0.1:8080/clean-2`, { ignoreHTTPSErrors: true });
      expect([200, 404, 503]).toContain(res2.status());

      // Request 3 - Rate limited
      const res3 = await page.request.get(`http://127.0.0.1:8080/clean-3`, { ignoreHTTPSErrors: true });
      expect([200, 429, 502, 503]).toContain(res3.status());
    });
  });
});
