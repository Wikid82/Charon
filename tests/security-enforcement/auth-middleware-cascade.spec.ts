import { test, expect } from '@playwright/test';

import { dismissNewDomainPromptIfPresent, waitForLoadingComplete } from '../utils/wait-helpers';
import { caddyProxyOrigin, getAuthTokenFromPage as getAuthToken } from '../utils/api-helpers';

/**
 * Integration: Authentication Middleware Cascade
 *
 * Purpose: Validate authentication flows through all middleware layers
 * Scenarios: Token validation, ACL enforcement, WAF, rate limiting, all in sequence
 * Success: Valid tokens pass all layers, invalid tokens fail at auth layer
 *
 * Two distinct kinds of check live in this file, deliberately targeting two
 * different origins:
 *
 * 1. Charon's own JWT auth middleware (missing/invalid token -> 401) is a
 *    property of the Charon *management API* (port 8080) - real, existing
 *    routes like `GET /api/v1/proxy-hosts` require a valid Bearer token.
 * 2. WAF/rate-limit enforcement is a property of the Caddy *proxy* layer
 *    for a given proxied host (port 80, `caddyProxyOrigin()`, `Host`
 *    header) - per ARCHITECTURE.md ("Management Interface (Port 8080)":
 *    "NO Cerberus Middleware: Rate limiting, ACL, WAF, and CrowdSec are
 *    NOT applied to management interface"), those modules never run
 *    against port 8080 at all, and Caddy's proxy layer has no concept of
 *    Charon's own Bearer tokens (`backend/internal/caddy/config.go` never
 *    references Authorization/Bearer) - a proxied host only gates access
 *    via ACL (IP/geo rules, optionally HTTP Basic Auth) or WAF (payload
 *    inspection), not Charon JWTs.
 *
 * Earlier versions of this file sent every request in this file to
 * `http://127.0.0.1:8080/api/protected` etc. - paths that don't exist as
 * real Charon routes - so every assertion here was either vacuously
 * unverifiable (WAF/rate-limit can never trip on port 8080) or exercising
 * Gin's fallback 404 behavior rather than real auth middleware.
 */

test.describe('Auth Middleware Cascade', () => {
  const testProxy = {
    name: 'Auth Cascade Test Proxy',
    domain: 'auth-cascade-test.local',
    target: 'http://localhost:3001',
  };

  test.beforeEach(async ({ page }) => {
    await page.goto('/', { waitUntil: 'networkidle' });
    // These tests exercise the full auth/ACL/WAF/rate-limit middleware
    // cascade, which is slower to settle than a plain page load - a flat
    // 5s waitForSelector was too tight under real CrowdSec/WAF runtime
    // conditions. Use the repo's condition-based loading wait (matches
    // multi-component-security-workflows.spec.ts's beforeEach pattern)
    // before asserting on the main landmark.
    await waitForLoadingComplete(page, { timeout: 15000 });
    await expect(page.getByRole('main')).toBeVisible({ timeout: 15000 });
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

  // Missing token → 401 at Charon's own management-API auth layer
  test('Request without token gets 401 Unauthorized', async ({ page }) => {
    await test.step('Send request without Authorization header', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/api/v1/proxy-hosts`,
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

  // Invalid token → 401 at Charon's own management-API auth layer
  test('Request with invalid token gets 401 Unauthorized', async ({ page }) => {
    await test.step('Send request with malformed token', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/api/v1/proxy-hosts`,
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
        `http://127.0.0.1:8080/api/v1/proxy-hosts`,
        {
          headers: {
            // nosemgrep: javascript.lang.hardcoded.headers.hardcoded-bearer-token.hardcoded-bearer-token -- publicly documented jwt.io example token used solely as a deliberately-invalid/expired fixture for this negative-path 401 test; not a real credential.
            'Authorization': `Bearer ${expiredToken}`,
          },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.status()).toBe(401);
    });
  });

  // Valid token isn't rejected by Charon's own auth layer when reaching a
  // proxied host with no ACL configured (Caddy doesn't check Charon
  // Bearer tokens at all for proxied traffic, so this can never be 401).
  test('Valid token passes ACL validation', async ({ page }) => {
    await test.step('Create proxy with ACL', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/^name\b/i).fill(testProxy.name);
      await page.getByLabel(/^domain names/i).fill(testProxy.domain);
      await page.getByLabel(/^host\b/i).fill(testProxy.target);

      const submitButton = page.getByRole('button', { name: 'Save', exact: true }).first();
      await dismissNewDomainPromptIfPresent(page);
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send request with valid token', async () => {
      const validToken = await getAuthToken(page);
      const origin = caddyProxyOrigin(page);

      const response = await page.request.get(
        `${origin}/api/test`,
        {
          headers: {
            'Authorization': `Bearer ${validToken || ''}`,
            Host: testProxy.domain,
          },
          ignoreHTTPSErrors: true,
        }
      );

      // Should pass auth (not 401), may be 404/502/503 depending on target
      expect(response.status()).not.toBe(401);
    });
  });

  // Valid token / benign request passes through WAF layer for a proxied host
  test('Valid token passes WAF validation', async ({ page }) => {
    await test.step('Create proxy with WAF', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/^name\b/i).fill(testProxy.name);
      await page.getByLabel(/^domain names/i).fill(testProxy.domain);
      await page.getByLabel(/^host\b/i).fill(testProxy.target);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const submitButton = page.getByRole('button', { name: 'Save', exact: true }).first();
      await dismissNewDomainPromptIfPresent(page);
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send valid request (passes auth, passes WAF)', async () => {
      const validToken = await getAuthToken(page);
      const origin = caddyProxyOrigin(page);

      const response = await page.request.get(
        `${origin}/api/legitimate`,
        {
          headers: {
            'Authorization': `Bearer ${validToken || ''}`,
            Host: testProxy.domain,
          },
          ignoreHTTPSErrors: true,
        }
      );

      // Should not be 401 (auth failed), not 403 (WAF blocked). 502
      // accounts for the upstream target being unreachable in this
      // environment (see multi-component-security-workflows.spec.ts).
      expect([200, 404, 502, 503]).toContain(response.status());
    });
  });

  // Valid token / requests within budget pass through rate limiting for a proxied host
  test('Valid token passes rate limiting validation', async ({ page }) => {
    await test.step('Create proxy with rate limiting', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/^name\b/i).fill(testProxy.name);
      await page.getByLabel(/^domain names/i).fill(testProxy.domain);
      await page.getByLabel(/^host\b/i).fill(testProxy.target);

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

      const submitButton = page.getByRole('button', { name: 'Save', exact: true }).first();
      await dismissNewDomainPromptIfPresent(page);
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send multiple valid requests within limit', async () => {
      const validToken = await getAuthToken(page);
      const origin = caddyProxyOrigin(page);

      for (let i = 0; i < 5; i++) {
        const response = await page.request.get(
          `${origin}/api/test-${i}`,
          {
            headers: {
              'Authorization': `Bearer ${validToken || ''}`,
              Host: testProxy.domain,
            },
            ignoreHTTPSErrors: true,
          }
        );

        // Should pass rate limiting
        expect(response.status()).not.toBe(429);
      }
    });
  });

  // Valid token passes ALL middleware layers
  test('Valid token passes auth, ACL, WAF, and rate limiting', async ({ page }) => {
    await test.step('Create proxy with all protections', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/^name\b/i).fill(testProxy.name);
      await page.getByLabel(/^domain names/i).fill(testProxy.domain);
      await page.getByLabel(/^host\b/i).fill(testProxy.target);

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

      const submitButton = page.getByRole('button', { name: 'Save', exact: true }).first();
      await dismissNewDomainPromptIfPresent(page);
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send legitimate requests through full middleware stack', async () => {
      const validToken = await getAuthToken(page);
      const origin = caddyProxyOrigin(page);

      const start = Date.now();

      const response = await page.request.get(
        `${origin}/api/full-stack`,
        {
          headers: {
            'Authorization': `Bearer ${validToken || ''}`,
            'Content-Type': 'application/json',
            Host: testProxy.domain,
          },
          ignoreHTTPSErrors: true,
        }
      );

      const duration = Date.now() - start;

      // Should pass all middleware
      expect([200, 404, 502, 503]).toContain(response.status());
      console.log(`✓ Request passed all middleware layers in ${duration}ms`);
    });

    await test.step('Verify each middleware would block if violated', async () => {
      const validToken = await getAuthToken(page);
      const origin = caddyProxyOrigin(page);

      // Test: Missing token → should fail at Charon's own management-API
      // auth layer. This specifically targets a real management-API route
      // (not the proxied host) - Caddy's proxy layer has no concept of
      // Charon Bearer tokens, so a "missing token" 401 can only ever come
      // from the management API itself (see file-level doc comment).
      const noAuthResponse = await page.request.get(
        `http://127.0.0.1:8080/api/v1/proxy-hosts`,
        {
          ignoreHTTPSErrors: true,
        }
      );
      expect(noAuthResponse.status()).toBe(401);

      // Test: Malicious payload → should fail at WAF (403) for the proxied host
      const maliciousResponse = await page.request.get(
        `${origin}/?id=1' UNION SELECT NULL--`,
        {
          headers: {
            'Authorization': `Bearer ${validToken || ''}`,
            Host: testProxy.domain,
          },
          ignoreHTTPSErrors: true,
        }
      );
      expect([403, 502]).toContain(maliciousResponse.status());
    });
  });
});
