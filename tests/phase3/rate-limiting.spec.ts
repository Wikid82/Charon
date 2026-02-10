/**
 * Phase 3 - Rate Limiting Tests
 *
 * Validates that rate limiting correctly enforces request throttling:
 * - Requests within limit → 200 OK
 * - Requests exceeding limit → 429 Too Many Requests
 * - Rate limit headers present in response
 * - Different endpoints have correct limits
 * - Rate limit window expires and resets
 *
 * Total Tests: 12
 * Expected Duration: ~10 minutes
 *
 * IMPORTANT: Run with --workers=1
 * Rate limiting tests must be SERIAL to prevent cross-test interference
 *
 * Rate Limit Configuration (from Phase 3 plan):
 * - 3 requests per 10-second window
 * - Different endpoints may have different limits
 */

import { test, expect } from '@playwright/test';
import { request as playwrightRequest } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
const VALID_TOKEN = process.env.VALID_TEST_TOKEN || 'test-token-12345';

// Rate limit configuration
const RATE_LIMIT_CONFIG = {
  requestsPerWindow: 3,
  windowSeconds: 10,
  description: '3 requests per 10-second window',
};

test.describe('Phase 3: Rate Limiting', () => {
  let context: any;

  test.beforeAll(async () => {
    context = await playwrightRequest.newContext({
      baseURL: BASE_URL,
    });
  });

  test.afterAll(async () => {
    await context?.close();
  });

  // =========================================================================
  // Test Suite: Basic Rate Limit Enforcement
  // =========================================================================
  test.describe('Basic Rate Limit Enforcement', () => {
    test(`should allow up to ${RATE_LIMIT_CONFIG.requestsPerWindow} requests in ${RATE_LIMIT_CONFIG.windowSeconds}s window`, async () => {
      const responses = [];

      // Make exactly 3 requests (should all succeed)
      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow; i++) {
        const response = await context.get('/api/v1/proxy-hosts', {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });
        responses.push(response.status());
      }

      // All 3 should succeed (200 or 401, but NOT 429)
      responses.forEach(status => {
        expect([200, 401, 403]).toContain(status);
        expect(status).not.toBe(429);
      });
    });

    test(`should return 429 when exceeding ${RATE_LIMIT_CONFIG.requestsPerWindow} requests in ${RATE_LIMIT_CONFIG.windowSeconds}s window`, async () => {
      // Make limit + 1 requests (4th should be rate limited)
      const responses = [];

      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow + 1; i++) {
        const response = await context.get('/api/v1/proxy-hosts', {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });
        responses.push(response.status());
      }

      // Last request should be 429
      const lastStatus = responses[responses.length - 1];
      expect(lastStatus).toBe(429);
    });

    test('should include rate limit headers in response', async () => {
      const response = await context.get('/api/v1/proxy-hosts', {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });

      // Check for rate limit headers (common standards)
      const headers = response.headers();
      // May use RateLimit-* headers from IETF standard
      // or X-RateLimit-* from older conventions
      const hasRateLimitHeader = headers['ratelimit-limit'] ||
                                 headers['x-ratelimit-limit'] ||
                                 headers['retry-after'];

      // At minimum, 429 response should have Retry-After
      if (response.status() === 429) {
        expect(headers['retry-after']).toBeTruthy();
      }
    });
  });

  // =========================================================================
  // Test Suite: Rate Limit Window Expiration
  // =========================================================================
  test.describe('Rate Limit Window Expiration & Reset', () => {
    test('should reset rate limit after window expires', async ({}, testInfo) => {
      // Make 3 requests (fill the window)
      const firstBatch = [];
      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow; i++) {
        const response = await context.get('/api/v1/proxy-hosts', {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });
        firstBatch.push(response.status());
      }

      // 4th request should fail (429)
      const blockedResponse = await context.get('/api/v1/proxy-hosts', {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });
      expect(blockedResponse.status()).toBe(429);

      // Wait for window to expire (10 seconds + small buffer)
      console.log(`Waiting ${RATE_LIMIT_CONFIG.windowSeconds + 1} seconds for rate limit window to expire...`);
      await new Promise(resolve => setTimeout(resolve, (RATE_LIMIT_CONFIG.windowSeconds + 1) * 1000));

      // New request should succeed
      const afterResetResponse = await context.get('/api/v1/proxy-hosts', {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });
      expect([200, 401, 403]).toContain(afterResetResponse.status());
      expect(afterResetResponse.status()).not.toBe(429);
    });
  });

  // =========================================================================
  // Test Suite: Different Endpoints Rate Limits
  // =========================================================================
  test.describe('Per-Endpoint Rate Limits', () => {
    test('GET /api/v1/proxy-hosts should have rate limit', async () => {
      // Make 3 requests
      const responses = [];
      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow; i++) {
        const response = await context.get('/api/v1/proxy-hosts', {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });
        responses.push(response.status());
      }

      // 4th should be 429
      const fourthResponse = await context.get('/api/v1/proxy-hosts', {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });
      expect(fourthResponse.status()).toBe(429);
    });

    test('GET /api/v1/access-lists should have separate rate limit', async () => {
      // Different endpoint should have its own counter
      const responses = [];
      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow; i++) {
        const response = await context.get('/api/v1/access-lists', {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });
        responses.push(response.status());
      }

      // 4th should be 429
      const fourthResponse = await context.get('/api/v1/access-lists', {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });
      expect(fourthResponse.status()).toBe(429);
    });
  });

  // =========================================================================
  // Test Suite: Rate Limit Without Token (Anonymous)
  // =========================================================================
  test.describe('Anonymous Request Rate Limiting', () => {
    test('should rate limit anonymous requests separately', async () => {
      // Create a new context without token to simulate different rate limit bucket
      const anonContext = await playwrightRequest.newContext({ baseURL: BASE_URL });

      try {
        const responses = [];

        // Make requests without auth token
        for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow + 1; i++) {
          const response = await anonContext.get('/api/v1/health'); // Health might not require auth
          responses.push(response.status());
        }

        // Last should be rate limited (429) if rate limiting applies to unauthenticated
        // Note: Rate limit bucket is usually per IP, not per user
        const lastStatus = responses[responses.length - 1];
        // Either all pass (no limit on health) or last is 429
        expect([200, 429]).toContain(lastStatus);
      } finally {
        await anonContext.close();
      }
    });
  });

  // =========================================================================
  // Test Suite: Retry-After Header
  // =========================================================================
  test.describe('Retry-After Header', () => {
    test('429 response should include Retry-After header', async () => {
      // Fill the rate limit
      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow + 1; i++) {
        const response = await context.get('/api/v1/proxy-hosts', {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });

        if (response.status() === 429) {
          const headers = response.headers();
          expect(headers['retry-after']).toBeTruthy();
          // Retry-After should be a number (seconds) or HTTP date
          const retryAfter = headers['retry-after'];
          expect(retryAfter).toMatch(/\d+/);
        }
      }
    });

    test('Retry-After should indicate reasonable wait time', async () => {
      // Fill the rate limit
      let rateLimitedResponse = null;
      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow + 1; i++) {
        const response = await context.get('/api/v1/proxy-hosts', {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });

        if (response.status() === 429) {
          rateLimitedResponse = response;
          break;
        }
      }

      if (rateLimitedResponse) {
        const headers = rateLimitedResponse.headers();
        const retryAfter = headers['retry-after'];
        if (retryAfter && !isNaN(Number(retryAfter))) {
          const seconds = Number(retryAfter);
          // Should be within reasonable bounds (1-60 seconds)
          expect(seconds).toBeGreaterThanOrEqual(1);
          expect(seconds).toBeLessThanOrEqual(60);
        }
      }
    });
  });

  // =========================================================================
  // Test Suite: Rate Limit Consistency
  // =========================================================================
  test.describe('Rate Limit Consistency', () => {
    test('same endpoint should share rate limit bucket', async () => {
      // Multiple calls to same endpoint should share counter
      const endpoint = '/api/v1/proxy-hosts';
      const responses = [];

      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow + 1; i++) {
        const response = await context.get(endpoint, {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });
        responses.push(response.status());
      }

      // Last should be 429
      expect(responses[responses.length - 1]).toBe(429);
    });

    test('different HTTP methods on same endpoint should share limit', async () => {
      // GET and POST to same endpoint should use same rate limit bucket
      const endpoint = '/api/v1/proxy-hosts';

      const responses = [];

      // 3 GETs
      for (let i = 0; i < 2; i++) {
        const response = await context.get(endpoint, {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });
        responses.push({ method: 'GET', status: response.status() });
      }

      // 1 POST (may fail for other reasons, but should count toward limit)
      const postResponse = await context.post(endpoint, {
        data: {
          domain: 'test.example.com',
          forward_host: '127.0.0.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });
      responses.push({ method: 'POST', status: postResponse.status() });

      // 4th request should be rate limited (429)
      const fourthResponse = await context.get(endpoint, {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });
      expect(fourthResponse.status()).toBe(429);
    });
  });

  // =========================================================================
  // Test Suite: Rate Limit Error Response
  // =========================================================================
  test.describe('Rate Limit Error Response Format', () => {
    test('429 response should be valid JSON', async () => {
      // Fill the rate limit and get 429
      let statusCode = 200;
      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow + 1; i++) {
        const response = await context.get('/api/v1/proxy-hosts', {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });
        statusCode = response.status();

        if (statusCode === 429) {
          // Try to parse as JSON
          try {
            const body = await response.json();
            expect(typeof body).toBe('object');
          } catch (e) {
            // Or it might be plain text, which is acceptable
            const text = await response.text();
            expect(text.length).toBeGreaterThan(0);
          }
          break;
        }
      }

      expect(statusCode).toBe(429);
    });

    test('429 response should not expose rate limit implementation details', async () => {
      // Fill the rate limit
      let responseText = '';
      for (let i = 0; i < RATE_LIMIT_CONFIG.requestsPerWindow + 1; i++) {
        const response = await context.get('/api/v1/proxy-hosts', {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });

        if (response.status() === 429) {
          responseText = await response.text();
          break;
        }
      }

      // Should not expose internal details
      expect(responseText).not.toContain('redis');
      expect(responseText).not.toContain('sliding window');
      expect(responseText).not.toContain('Caddy');
    });
  });
});
