import { test, expect, request as playwrightRequest } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

test.describe('Security Enforcement API', () => {
  let baseContext: any;

  test.beforeAll(async () => {
    baseContext = await playwrightRequest.newContext();
  });

  test.afterAll(async () => {
    await baseContext?.dispose();
  });

  test.describe('Bearer Token Validation', () => {
    test('should reject request with missing bearer token (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`);
      expect(response.status()).toBe(401);
      const data = await response.json();
      expect(data).toHaveProperty('error');
    });

    test('should reject request with invalid bearer token (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: 'Bearer invalid.token.here' },
      });
      expect(response.status()).toBe(401);
    });

    test('should reject request with malformed authorization header (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: 'InvalidFormat token_without_bearer' },
      });
      expect(response.status()).toBe(401);
    });

    test('should reject request with empty bearer token (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: 'Bearer ' },
      });
      expect(response.status()).toBe(401);
    });

    test('should reject request with NULL bearer token (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: 'Bearer null' },
      });
      expect(response.status()).toBe(401);
    });
  });

  test.describe('JWT Expiration & Auto-Refresh', () => {
    test('should handle expired JWT gracefully', async () => {
      const expiredToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MDAwMDAwMDB9.invalidSignature';
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: `Bearer ${expiredToken}` },
      });
      expect(response.status()).toBe(401);
    });
  });

  test.describe('HTTP Header Validation', () => {
    test('should accept valid Content-Type application/json', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/health`, {
        headers: { 'Content-Type': 'application/json' },
      });
      expect(response.status()).toBe(200);
    });

    test('should handle requests with no User-Agent header', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/health`);
      expect(response.status()).toBe(200);
    });
  });

  test.describe('Error Response Format', () => {
    test('error response should not expose internal details', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: 'Bearer malformed.token.here' },
      });
      expect([401, 404]).toContain(response.status());
      const text = await response.text();
      expect(text).not.toContain('stack trace');
      expect(text).not.toContain('/app/');
    });
  });
});
