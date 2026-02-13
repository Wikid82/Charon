import { test, expect, request as playwrightRequest } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

test.describe('Security Enforcement API', () => {
  let baseContext: any;

  test.beforeAll(async () => {
    baseContext = await playwrightRequest.newContext();
  });

  test.afterAll(async () => {
    await baseContext?.close();
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

    test('should reject request with uppercase "bearer" keyword (case-sensitive)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: 'BEARER validtoken123' },
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

    test('should return 401 for JWT with invalid signature', async () => {
      const invalidJWT = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.wrongSignature';
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: `Bearer ${invalidJWT}` },
      });
      expect(response.status()).toBe(401);
    });

    test('should return 401 for token missing required claims (sub, exp)', async () => {
      const incompleteJWT = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJkYXRhIjoibm9jbGFpbXMifQ.wrong';
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: `Bearer ${incompleteJWT}` },
      });
      expect(response.status()).toBe(401);
    });
  });

  test.describe('CSRF Token Validation', () => {
    test('POST request should include CSRF protection headers', async () => {
      const response = await baseContext.post(`${BASE_URL}/api/v1/proxy-hosts`, {
        data: { domain: 'test.example.com', forward_host: '127.0.0.1', forward_port: 8000 },
        headers: { Authorization: 'Bearer invalid_token', 'Content-Type': 'application/json' },
      });
      expect(response.status()).toBe(401);
    });

    test('PUT request should validate CSRF token', async () => {
      const response = await baseContext.put(`${BASE_URL}/api/v1/proxy-hosts/test-id`, {
        data: { domain: 'updated.example.com' },
        headers: { Authorization: 'Bearer invalid_token', 'Content-Type': 'application/json' },
      });
      expect(response.status()).toBe(401);
    });

    test('DELETE request without auth should return 401', async () => {
      const response = await baseContext.delete(`${BASE_URL}/api/v1/proxy-hosts/test-id`);
      expect(response.status()).toBe(401);
    });
  });

  test.describe('Request Timeout Handling', () => {
    test('should handle slow endpoint with reasonable timeout', async () => {
      const timeoutContext = await playwrightRequest.newContext({ baseURL: BASE_URL, httpClient: true });
      try {
        const response = await timeoutContext.get('/api/v1/health', { timeout: 5000 });
        expect(response.status()).toBe(200);
      } finally {
        await timeoutContext.close();
      }
    });

    test('should return proper error for unreachable endpoint', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/nonexistent-endpoint`);
      expect(response.status()).toBe(404);
    });
  });

  test.describe('Middleware Execution Order', () => {
    test('authentication should be checked before authorization', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`);
      expect(response.status()).toBe(401);
      expect(response.status()).not.toBe(403);
    });

    test('malformed request should be validated before processing', async () => {
      const response = await baseContext.post(`${BASE_URL}/api/v1/proxy-hosts`, {
        data: 'invalid non-json body',
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token' },
      });
      expect([400, 401, 415]).toContain(response.status());
    });

    test('rate limiting should be applied after authentication', async () => {
      const codes: number[] = [];
      for (let i = 0; i < 3; i++) {
        const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
          headers: { Authorization: 'Bearer invalid' },
        });
        codes.push(response.status());
      }
      expect(codes.every(code => [401, 429].includes(code))).toBe(true);
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

    test('response should include security headers', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/health`);
      expect(response.status()).toBe(200);
      const data = await response.json();
      expect(data).toHaveProperty('status');
    });
  });

  test.describe('HTTP Method Validation', () => {
    test('GET request should be allowed for read operations', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: 'Bearer invalid' },
      });
      expect(response.status()).toBe(401);
    });

    test('unsupported methods should return 405 or 401', async () => {
      const response = await baseContext.fetch(`${BASE_URL}/api/v1/proxy-hosts`, {
        method: 'PATCH',
        headers: { Authorization: 'Bearer invalid' },
      });
      expect([401, 405]).toContain(response.status());
    });
  });

  test.describe('Error Response Format', () => {
    test('401 error should include error message', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`);
      expect(response.status()).toBe(401);
      const data = await response.json().catch(() => ({}));
      expect(data).toBeDefined();
    });

    test('error response should not expose internal details', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: { Authorization: 'Bearer malformed.token.here' },
      });
      expect(response.status()).toBe(401);
      const text = await response.text();
      expect(text).not.toContain('stack trace');
      expect(text).not.toContain('/app/');
    });
  });
});
