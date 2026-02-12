import { test, expect } from '@playwright/test';

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
    target: 'http://localhost:3001',
    description: 'Test proxy for WAF and rate limit',
  };

  test.beforeEach(async ({ page }) => {
    await page.goto('/', { waitUntil: 'networkidle' });
    await page.waitForSelector('[role="main"]', { timeout: 5000 });
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

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"], [class*="waf"] input[type="checkbox"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send malicious SQL injection payload', async () => {
      const start = Date.now();

      const response = await page.request.get(
        `http://127.0.0.1:8080/?id=1' OR '1'='1`,
        { ignoreHTTPSErrors: true }
      );

      const duration = Date.now() - start;
      console.log(`✓ Malicious request responded in ${duration}ms with status ${response.status()}`);

      expect(response.status()).toBe(403);
    });
  });

  // Rate limiting blocks excessive requests (429)
  test('Rate limiting blocks requests exceeding threshold', async ({ page }) => {
    await test.step('Create proxy with rate limiting enabled', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

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

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
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
      expect(response.status()).toBe(429);
    });
  });

  // WAF and rate limit enforced independently
  test('WAF enforces regardless of rate limit status', async ({ page }) => {
    await test.step('Create proxy with both WAF and rate limiting', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

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

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Malicious request blocked by WAF (403)', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/?id=1' UNION SELECT NULL--`,
        { ignoreHTTPSErrors: true }
      );
      expect(response.status()).toBe(403);
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
      expect(responses[responses.length - 1]).toBe(429);
    });
  });

  // Request within limit but triggers WAF
  test('Malicious request gets 403 (WAF) not 429 (rate limit)', async ({ page }) => {
    await test.step('Create proxy with both modules', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

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

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
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
      expect(maliciousResponse.status()).toBe(403);
    });
  });

  // Request exceeds limit (429) without malicious content
  test('Clean request gets 429 when rate limit exceeded', async ({ page }) => {
    await test.step('Setup proxy with rate limiting', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

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

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
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
      expect(res3.status()).toBe(429);
    });
  });
});
