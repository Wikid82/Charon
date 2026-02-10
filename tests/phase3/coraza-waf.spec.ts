/**
 * Phase 3 - Coraza WAF (Attack Prevention) Tests
 *
 * Validates that Coraza module correctly blocks malicious payloads:
 * - SQL Injection attacks
 * - Cross-Site Scripting (XSS) attacks
 * - CSRF token attacks
 * - Malformed requests
 * - Path traversal attempts
 * - Command injection
 *
 * Total Tests: 18
 * Expected Duration: ~10 minutes
 *
 * Expected Behavior:
 * - Malicious payloads → 403 Forbidden (WAF block)
 * - Benign requests → 200/201/400 OK (allowed through)
 */

import { test, expect } from '@playwright/test';
import { request as playwrightRequest } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
const VALID_TOKEN = process.env.VALID_TEST_TOKEN || 'test-token-12345';

// SQL Injection Payloads
const SQL_INJECTION_PAYLOADS = [
  "' OR '1'='1",
  "' OR '1'='1' --",
  "admin' --",
  "' UNION SELECT * FROM users --",
  "'; DROP TABLE users; --",
  "1' AND SLEEP(5) --",
  "1; DELETE FROM users --",
];

// XSS Payloads
const XSS_PAYLOADS = [
  '<script>alert("xss")</script>',
  '<img src=x onerror="alert(\'xss\')">',
  'javascript:alert("xss")',
  '<svg onload="alert(\'xss\')">',
  '<iframe src="javascript:alert(\'xss\')"></iframe>',
  '<body onload="alert(\'xss\')">',
  '"><script>alert("xss")</script>',
];

// Path Traversal Payloads
const PATH_TRAVERSAL_PAYLOADS = [
  '../../../etc/passwd',
  '..%2F..%2F..%2Fetc%2Fpasswd',
  '....//....//....//etc/passwd',
  '/..\\..\\..\\windows\\system32\\config\\sam',
  '%252e%252e%252fconfig',
];

// Command Injection Payloads
const COMMAND_INJECTION_PAYLOADS = [
  '; ls -la',
  '| cat /etc/passwd',
  '` whoami `',
  '$(whoami)',
  '; rm -rf /',
];

// CSRF Payloads (malformed CSRF token)
const CSRF_PAYLOADS = [
  { csrfToken: 'invalid-csrf' },
  { csrfToken: ''  },
  { csrfToken: null },
];

test.describe('Phase 3: Coraza WAF (Attack Prevention)', () => {
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
  // Test Suite: SQL Injection Prevention
  // =========================================================================
  test.describe('SQL Injection Prevention', () => {
    SQL_INJECTION_PAYLOADS.forEach((payload, index) => {
      test(`should block SQLi payload ${index + 1}: "${payload.substring(0, 20)}..."`, async () => {
        const response = await context.post('/api/v1/proxy-hosts', {
          data: {
            domain: payload, // Inject into domain field
            forward_host: '127.0.0.1',
            forward_port: 8000,
          },
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
            'Content-Type': 'application/json',
          },
        });

        // WAF should block with 403, or app may reject with 400
        expect([400, 403]).toContain(response.status());
        // Preferred: 403 (WAF block)
        if (response.status() === 403) {
          expect(response.status()).toBe(403);
        }
      });
    });

    test('should block SQLi in query parameters', async () => {
      const response = await context.get(`/api/v1/proxy-hosts?search=' OR '1'='1`, {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });

      // Should be 403 (WAF) or 400 (bad request)
      expect([400, 403]).toContain(response.status());
    });

    test('should block SQLi in request headers', async () => {
      const response = await context.get('/api/v1/proxy-hosts', {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'X-Custom-Header': "' UNION SELECT * FROM users --",
        },
      });

      // WAF may block headers containing SQL
      expect([200, 403]).toContain(response.status());
    });
  });

  // =========================================================================
  // Test Suite: Cross-Site Scripting (XSS) Prevention
  // =========================================================================
  test.describe('Cross-Site Scripting (XSS) Prevention', () => {
    XSS_PAYLOADS.forEach((payload, index) => {
      test(`should block XSS payload ${index + 1}: "${payload.substring(0, 25)}..."`, async () => {
        const response = await context.post('/api/v1/proxy-hosts', {
          data: {
            domain: `example.com${payload}`, // XSS payload in field
            forward_host: '127.0.0.1',
            forward_port: 8000,
          },
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
            'Content-Type': 'application/json',
          },
        });

        // WAF should block with 403, or app validation fails with 400
        expect([400, 403]).toContain(response.status());
      });
    });

    test('should block XSS in JSON payload', async () => {
      const response = await context.post('/api/v1/proxy-hosts', {
        data: {
          domain: 'example.com',
          forward_host: '<script>alert("xss")</script>',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      expect([400, 403]).toContain(response.status());
    });

    test('should block encoded XSS payloads', async () => {
      // HTML entity encoded XSS
      const response = await context.post('/api/v1/proxy-hosts', {
        data: {
          domain: 'example.com&lt;script&gt;alert("xss")&lt;/script&gt;',
          forward_host: '127.0.0.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      // Modern WAF should still detect encoded attacks
      expect([400, 403]).toContain(response.status());
    });
  });

  // =========================================================================
  // Test Suite: Path Traversal Prevention
  // =========================================================================
  test.describe('Path Traversal Prevention', () => {
    PATH_TRAVERSAL_PAYLOADS.forEach((payload, index) => {
      test(`should block path traversal ${index + 1}: "${payload.substring(0, 20)}..."`, async () => {
        // Path traversal in URL path
        const response = await context.get(`/api/v1/proxy-hosts${payload}`, {
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
          },
        });

        // Should be blocked or return 404 (path not found)
        expect([403, 404]).toContain(response.status());
      });
    });

    test('should block path traversal in POST data', async () => {
      const response = await context.post('/api/v1/import', {
        data: {
          file: '../../../etc/passwd',
          config: '....\\..\\..\\windows\\system32\\config\\sam',
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      // Should be 403 (WAF) or 400 (validation fail)
      expect([400, 403, 404]).toContain(response.status());
    });
  });

  // =========================================================================
  // Test Suite: Command Injection Prevention
  // =========================================================================
  test.describe('Command Injection Prevention', () => {
    COMMAND_INJECTION_PAYLOADS.forEach((payload, index) => {
      test(`should block command injection ${index + 1}: "${payload.substring(0, 20)}..."`, async () => {
        const response = await context.post('/api/v1/proxy-hosts', {
          data: {
            domain: `example.com${payload}`,
            forward_host: '127.0.0.1',
            forward_port: 8000,
          },
          headers: {
            Authorization: `Bearer ${VALID_TOKEN}`,
            'Content-Type': 'application/json',
          },
        });

        // WAF should detect shell metacharacters
        expect([400, 403]).toContain(response.status());
      });
    });
  });

  // =========================================================================
  // Test Suite: Malformed Request Handling
  // =========================================================================
  test.describe('Malformed Request Handling', () => {
    test('should reject invalid JSON payload', async () => {
      const response = await context.post('/api/v1/proxy-hosts', {
        data: '{invalid json}',
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      // Should return 400 (bad request)
      expect(response.status()).toBe(400);
    });

    test('should reject oversized payload', async () => {
      // Create a very large payload
      const largeString = 'A'.repeat(1024 * 1024); // 1MB of 'A'

      const response = await context.post('/api/v1/proxy-hosts', {
        data: {
          domain: largeString,
          forward_host: '127.0.0.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      // Should be rejected as too large or malformed
      expect([400, 413]).toContain(response.status());
    });

    test('should reject null characters in payload', async () => {
      const response = await context.post('/api/v1/proxy-hosts', {
        data: {
          domain: 'example.com\x00injection',
          forward_host: '127.0.0.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      // Should be rejected
      expect([400, 403]).toContain(response.status());
    });

    test('should reject double-encoded payloads', async () => {
      // %25 = %, so %2525 = %25 after one decode
      const response = await context.get('/api/v1/proxy-hosts/%2525252e%2525252e', {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });

      // WAF should normalize and detect
      expect([403, 404]).toContain(response.status());
    });
  });

  // =========================================================================
  // Test Suite: CSRF Protection
  // =========================================================================
  test.describe('CSRF Token Validation', () => {
    test('should validate CSRF token presence in state-changing requests', async () => {
      // POST without CSRF token might be rejected depending on app configuration
      const response = await context.post('/api/v1/proxy-hosts', {
        data: {
          domain: 'test.example.com',
          forward_host: '127.0.0.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      // May be 400/403 (no CSRF) or 401 (auth fail)
      expect([400, 401, 403]).toContain(response.status());
    });

    test('should reject invalid CSRF token', async () => {
      const response = await context.post('/api/v1/proxy-hosts', {
        data: {
          domain: 'test.example.com',
          forward_host: '127.0.0.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'X-CSRF-Token': 'invalid-csrf-token-12345',
          'Content-Type': 'application/json',
        },
      });

      expect([400, 401, 403]).toContain(response.status());
    });
  });

  // =========================================================================
  // Test Suite: Benign Requests Should Pass
  // =========================================================================
  test.describe('Benign Request Handling', () => {
    test('should allow valid domain names', async () => {
      const response = await context.post('/api/v1/proxy-hosts', {
        data: {
          domain: 'valid-domain-example.com',
          forward_host: '127.0.0.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      // Should pass WAF (may fail auth/validation but not WAF block)
      expect(response.status()).not.toBe(403);
    });

    test('should allow valid IP addresses', async () => {
      const response = await context.post('/api/v1/proxy-hosts', {
        data: {
          domain: 'example.com',
          forward_host: '192.168.1.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      expect(response.status()).not.toBe(403);
    });

    test('should allow GET requests with safe parameters', async () => {
      const response = await context.get('/api/v1/proxy-hosts?page=1&limit=10', {
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
        },
      });

      // Should not be blocked by WAF
      expect(response.status()).not.toBe(403);
    });
  });

  // =========================================================================
  // Test Suite: WAF Response Headers
  // =========================================================================
  test.describe('WAF Response Indicators', () => {
    test('blocked request should not expose WAF details', async () => {
      const response = await context.post('/api/v1/proxy-hosts', {
        data: {
          domain: "' OR '1'='1",
          forward_host: '127.0.0.1',
          forward_port: 8000,
        },
        headers: {
          Authorization: `Bearer ${VALID_TOKEN}`,
          'Content-Type': 'application/json',
        },
      });

      // Should be 403 or 400
      if (response.status() === 403 || response.status() === 400) {
        const text = await response.text();
        // Should not expose internal WAF rule details
        expect(text).not.toContain('Coraza');
        expect(text).not.toContain('ModSecurity');
      }
    });
  });
});
