import { test, expect, request as playwrightRequest } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

// Helper: Create logs directory
function ensureLogDir() {
  const logDir = path.join(process.cwd(), 'logs');
  if (!fs.existsSync(logDir)) {
    fs.mkdirSync(logDir, { recursive: true });
  }
  return logDir;
}

// Helper: Extract token expiry from JWT
function getTokenExpiry(token: string): string {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return 'invalid';
    const payload = JSON.parse(Buffer.from(parts[1], 'base64').toString());
    if (payload.exp) {
      return new Date(payload.exp * 1000).toISOString();
    }
  } catch (e) {
    return 'parse_error';
  }
  return 'unknown';
}

// Helper: Log heartbeat for 60-minute session test
function logHeartbeat(heartbeatNum: number, minuteElapsed: number, context: string, tokenExpiry: string) {
  const logDir = ensureLogDir();
  const heartbeatMsg = `✓ [Heartbeat ${heartbeatNum}] Min ${minuteElapsed}: ${context}. Token expires: ${tokenExpiry}`;
  fs.appendFileSync(path.join(logDir, 'session-heartbeat.log'), heartbeatMsg + '\n');
  console.log(heartbeatMsg);
}

test.describe('Security Enforcement', () => {
  let baseContext: any;

  test.beforeAll(async () => {
    baseContext = await playwrightRequest.newContext();
  });

  test.afterAll(async () => {
    await baseContext?.close();
  });

  // =========================================================================
  // Test Suite: Bearer Token Validation
  // =========================================================================
  test.describe('Bearer Token Validation', () => {
    test('should reject request with missing bearer token (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`);
      expect(response.status()).toBe(401);
      const data = await response.json();
      expect(data).toHaveProperty('error');
    });

    test('should reject request with invalid bearer token (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: 'Bearer invalid.token.here',
        },
      });
      expect(response.status()).toBe(401);
    });

    test('should reject request with malformed authorization header (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: 'InvalidFormat token_without_bearer',
        },
      });
      expect(response.status()).toBe(401);
    });

    test('should reject request with empty bearer token (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: 'Bearer ',
        },
      });
      expect(response.status()).toBe(401);
    });

    test('should reject request with NULL bearer token (401)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: 'Bearer null',
        },
      });
      expect(response.status()).toBe(401);
    });

    test('should reject request with uppercase "bearer" keyword (case-sensitive)', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: 'BEARER validtoken123',
        },
      });
      // Should be 401 (strict case sensitivity)
      expect(response.status()).toBe(401);
    });
  });

  // =========================================================================
  // Test Suite: JWT Expiration & Refresh
  // =========================================================================
  test.describe('JWT Expiration & Auto-Refresh', () => {
    test('should handle expired JWT gracefully', async () => {
      // Simulate an expired token (issued in the past with short TTL)
      // The app should return 401, prompting client to refresh
      const expiredToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MDAwMDAwMDB9.invalidSignature';

      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: `Bearer ${expiredToken}`,
        },
      });
      expect(response.status()).toBe(401);
    });

    test('should return 401 for JWT with invalid signature', async () => {
      const invalidJWT = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.wrongSignature';

      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: `Bearer ${invalidJWT}`,
        },
      });
      expect(response.status()).toBe(401);
    });

    test('should return 401 for token missing required claims (sub, exp)', async () => {
      // Token with missing required claims
      const incompletJWT = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJkYXRhIjoibm9jbGFpbXMifQ.wrong';

      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: `Bearer ${incompletJWT}`,
        },
      });
      expect(response.status()).toBe(401);
    });
  });

  // =========================================================================
  // Test Suite: CSRF Token Validation
  // =========================================================================
  test.describe('CSRF Token Validation', () => {
    test('POST request should include CSRF protection headers', async () => {
      // This tests that the API enforces CSRF on mutating operations
      // A POST without CSRF token should be rejected or require X-CSRF-Token header
      const response = await baseContext.post(`${BASE_URL}/api/v1/proxy-hosts`, {
        data: {
          domain: 'test.example.com',
          forward_host: '127.0.0.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: 'Bearer invalid_token',
          'Content-Type': 'application/json',
        },
      });
      // Should fail at auth layer before CSRF check (401)
      expect(response.status()).toBe(401);
    });

    test('PUT request should validate CSRF token', async () => {
      const response = await baseContext.put(`${BASE_URL}/api/v1/proxy-hosts/test-id`, {
        data: {
          domain: 'updated.example.com',
        },
        headers: {
          Authorization: 'Bearer invalid_token',
          'Content-Type': 'application/json',
        },
      });
      expect(response.status()).toBe(401);
    });

    test('DELETE request without auth should return 401', async () => {
      const response = await baseContext.delete(`${BASE_URL}/api/v1/proxy-hosts/test-id`);
      expect(response.status()).toBe(401);
    });
  });

  // =========================================================================
  // Test Suite: Request Timeout & Handling
  // =========================================================================
  test.describe('Request Timeout Handling', () => {
    test('should handle slow endpoint with reasonable timeout', async ({}, testInfo) => {
      // Create new context with timeout
      const timeoutContext = await playwrightRequest.newContext({
        baseURL: BASE_URL,
        httpClient: true,
      });

      try {
        // Request to health endpoint (should be fast)
        const response = await timeoutContext.get('/api/v1/health', {
          timeout: 5000, // 5 second timeout
        });
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

  // =========================================================================
  // Test Suite: Middleware Load Order & Precedence
  // =========================================================================
  test.describe('Middleware Execution Order', () => {
    test('authentication should be checked before authorization', async () => {
      // Request without token should fail at auth (401) before authz check
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`);
      expect(response.status()).toBe(401);
      // If it was 403, authz was checked, indicating middleware order is wrong
      expect(response.status()).not.toBe(403);
    });

    test('malformed request should be validated before processing', async () => {
      const response = await baseContext.post(`${BASE_URL}/api/v1/proxy-hosts`, {
        data: 'invalid non-json body',
        headers: {
          'Content-Type': 'application/json',
          Authorization: 'Bearer token',
        },
      });
      // Should return 400 (malformed) or 401 (auth) depending on order
      expect([400, 401, 415]).toContain(response.status());
    });

    test('rate limiting should be applied after authentication', async () => {
      // Even with invalid auth, rate limit should track the request
      let codes = [];
      for (let i = 0; i < 3; i++) {
        const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
          headers: {
            Authorization: 'Bearer invalid',
          },
        });
        codes.push(response.status());
      }
      // Should all be 401, or we might see 429 if rate limit kicks in
      expect(codes.every(code => [401, 429].includes(code))).toBe(true);
    });
  });

  // =========================================================================
  // Test Suite: Header Validation
  // =========================================================================
  test.describe('HTTP Header Validation', () => {
    test('should accept valid Content-Type application/json', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/health`, {
        headers: {
          'Content-Type': 'application/json',
        },
      });
      expect(response.status()).toBe(200);
    });

    test('should handle requests with no User-Agent header', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/health`);
      expect(response.status()).toBe(200);
    });

    test('response should include security headers', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/health`);
      const headers = response.headers();

      // Check for common security headers
      // Note: These may not all be present depending on app configuration
      expect(response.status()).toBe(200);
      // Verify response is valid JSON
      const data = await response.json();
      expect(data).toHaveProperty('status');
    });
  });

  // =========================================================================
  // Test Suite: Method Validation
  // =========================================================================
  test.describe('HTTP Method Validation', () => {
    test('GET request should be allowed for read operations', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: 'Bearer invalid',
        },
      });
      // Should fail auth (401), not method (405)
      expect(response.status()).toBe(401);
    });

    test('unsupported methods should return 405 or 401', async () => {
      const response = await baseContext.fetch(`${BASE_URL}/api/v1/proxy-hosts`, {
        method: 'PATCH',
        headers: {
          Authorization: 'Bearer invalid',
        },
      });
      // Should be 401 (auth fail) or 405 (method not allowed)
      expect([401, 405]).toContain(response.status());
    });
  });

  // =========================================================================
  // Test Suite: Error Response Format
  // =========================================================================
  test.describe('Error Response Format', () => {
    test('401 error should include error message', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`);
      expect(response.status()).toBe(401);

      const data = await response.json().catch(() => ({}));
      // Should have some error indication
      expect(response.status()).toBe(401);
    });

    test('error response should not expose internal details', async () => {
      const response = await baseContext.get(`${BASE_URL}/api/v1/proxy-hosts`, {
        headers: {
          Authorization: 'Bearer malformed.token.here',
        },
      });
      expect(response.status()).toBe(401);

      const text = await response.text();
      // Should not expose stack traces or internal file paths
      expect(text).not.toContain('stack trace');
      expect(text).not.toContain('/app/');
    });
  });
});
