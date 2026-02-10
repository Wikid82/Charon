import { test, expect } from '@playwright/test';

/**
 * Phase 4 Integration: Authentication Middleware Cascade
 *
 * Purpose: Validate authentication flows through all middleware layers
 * Scenarios: Token validation, ACL enforcement, WAF, rate limiting, all in sequence
 * Success: Valid tokens pass all layers, invalid tokens fail at auth layer
 */

test.describe('INT-004: Auth Middleware Cascade', () => {
  const testProxy = {
    domain: 'auth-cascade-test.local',
    target: 'http://localhost:3001',
    description: 'Test proxy for auth cascade',
  };

  test.beforeEach(async ({ page }) => {
    await page.goto('/', { waitUntil: 'networkidle' });
    await page.waitForSelector('[role="main"]', { timeout: 5000 });
  });

  test.afterEach(async ({ page }) => {
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

  // INT-004-1: Missing token → 401 at auth layer
  test('Request without token gets 401 Unauthorized', async ({ page }) => {
    await test.step('Create test proxy', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send request without Authorization header', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/api/protected`,
        {
          headers: {
            // Explicitly no Authorization header
          },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.status()).toBe(401);
    });
  });

  // INT-004-2: Invalid token → 401 at auth layer
  test('Request with invalid token gets 401 Unauthorized', async ({ page }) => {
    await test.step('Create test proxy', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send request with malformed token', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/api/protected`,
        {
          headers: {
            'Authorization': 'Bearer invalid_token_xyz_malformed',
          },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.status()).toBe(401);
    });

    await test.step('Send request with expired token', async () => {
      const expiredToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjE1MTYyMzkwMjJ9.TJVA95OrM7E2cBab30RMHrHDcEfxjoYZgeFONFh7HgQ';

      const response = await page.request.get(
        `http://127.0.0.1:8080/api/protected`,
        {
          headers: {
            'Authorization': `Bearer ${expiredToken}`,
          },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.status()).toBe(401);
    });
  });

  // INT-004-3: Valid token passes through ACL layer
  test('Valid token passes ACL validation', async ({ page }) => {
    await test.step('Create proxy with ACL', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send request with valid token', async () => {
      const validToken = await page.evaluate(() => localStorage.getItem('token'));

      const response = await page.request.get(
        `http://127.0.0.1:8080/api/test`,
        {
          headers: {
            'Authorization': `Bearer ${validToken || ''}`,
          },
          ignoreHTTPSErrors: true,
        }
      );

      // Should pass auth (not 401), may be 404/503 depending on target
      expect(response.status()).not.toBe(401);
    });
  });

  // INT-004-4: Valid token passes through WAF layer
  test('Valid token passes WAF validation', async ({ page }) => {
    await test.step('Create proxy with WAF', async () => {
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

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send valid request (passes auth, passes WAF)', async () => {
      const validToken = await page.evaluate(() => localStorage.getItem('token'));

      const response = await page.request.get(
        `http://127.0.0.1:8080/api/legitimate`,
        {
          headers: {
            'Authorization': `Bearer ${validToken || ''}`,
          },
          ignoreHTTPSErrors: true,
        }
      );

      // Should not be 401 (auth failed), not 403 (WAF blocked)
      expect([200, 404, 503]).toContain(response.status());
    });
  });

  // INT-004-5: Valid token passes through rate limiting layer
  test('Valid token passes rate limiting validation', async ({ page }) => {
    await test.step('Create proxy with rate limiting', async () => {
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

      const limitInput = page.locator('input[name*="limit"]').first();
      if (await limitInput.isVisible()) {
        await limitInput.fill('10');
      }

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send multiple valid requests within limit', async () => {
      const validToken = await page.evaluate(() => localStorage.getItem('token'));

      for (let i = 0; i < 5; i++) {
        const response = await page.request.get(
          `http://127.0.0.1:8080/api/test-${i}`,
          {
            headers: {
              'Authorization': `Bearer ${validToken || ''}`,
            },
            ignoreHTTPSErrors: true,
          }
        );

        // Should pass rate limiting
        expect(response.status()).not.toBe(429);
      }
    });
  });

  // INT-004-6: Valid token passes ALL middleware layers
  test('Valid token passes auth, ACL, WAF, and rate limiting', async ({ page }) => {
    await test.step('Create proxy with all protections', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      // Enable WAF
      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      // Enable rate limiting
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

    await test.step('Send legitimate requests through full middleware stack', async () => {
      const validToken = await page.evaluate(() => localStorage.getItem('token'));

      const start = Date.now();

      const response = await page.request.get(
        `http://127.0.0.1:8080/api/full-stack`,
        {
          headers: {
            'Authorization': `Bearer ${validToken || ''}`,
            'Content-Type': 'application/json',
          },
          ignoreHTTPSErrors: true,
        }
      );

      const duration = Date.now() - start;

      // Should pass all middleware
      expect([200, 404, 503]).toContain(response.status());
      console.log(`✓ Request passed all middleware layers in ${duration}ms`);
    });

    await test.step('Verify each middleware would block if violated', async () => {
      const validToken = await page.evaluate(() => localStorage.getItem('token'));

      // Test: Missing token → should fail at auth
      const noAuthResponse = await page.request.get(
        `http://127.0.0.1:8080/api/full-stack`,
        {
          ignoreHTTPSErrors: true,
        }
      );
      expect(noAuthResponse.status()).toBe(401);

      // Test: Malicious payload → should fail at WAF (403)
      const maliciousResponse = await page.request.get(
        `http://127.0.0.1:8080/?id=1' UNION SELECT NULL--`,
        {
          headers: {
            'Authorization': `Bearer ${validToken || ''}`,
          },
          ignoreHTTPSErrors: true,
        }
      );
      expect(maliciousResponse.status()).toBe(403);
    });
  });
});
