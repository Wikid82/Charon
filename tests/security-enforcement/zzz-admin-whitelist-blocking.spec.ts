/**
 * Admin Whitelist IP Blocking Enforcement Tests
 *
 * CRITICAL: This test MUST run LAST in the security-enforcement suite.
 * Uses 'zzz-' prefix to ensure alphabetical ordering places it at the end.
 *
 * Tests validate that Cerberus admin whitelist correctly blocks non-whitelisted IPs
 * and allows whitelisted IPs or emergency tokens.
 *
 * Recovery: Uses emergency reset in afterAll to unblock test IP.
 */

import { test, expect, request, APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';

test.describe.serial('Admin Whitelist IP Blocking (RUN LAST)', () => {
  const EMERGENCY_TOKEN = process.env.CHARON_EMERGENCY_TOKEN;
  const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
  let apiContext: APIRequestContext;

  test.beforeAll(async () => {
    if (!EMERGENCY_TOKEN) {
      throw new Error(
        'CHARON_EMERGENCY_TOKEN required for admin whitelist tests\n' +
        'Generate with: openssl rand -hex 32'
      );
    }

    apiContext = await request.newContext({
      baseURL: BASE_URL,
      storageState: STORAGE_STATE,
    });
  });

  test.afterAll(async ({ request }) => {
    // CRITICAL: Emergency reset to unblock test IP
    console.log('🔧 Emergency reset - cleaning up admin whitelist test');

    try {
      const response = await request.post('http://localhost:2020/emergency/security-reset', {
        headers: {
          'Authorization': 'Basic ' + Buffer.from('admin:changeme').toString('base64'),
          'X-Emergency-Token': EMERGENCY_TOKEN,
          'Content-Type': 'application/json',
        },
        data: { reason: 'E2E test cleanup - admin whitelist blocking test' },
      });

      if (response.ok()) {
        console.log('✅ Emergency reset completed - test IP unblocked');
      } else {
        console.error(`❌ Emergency reset failed: ${response.status()}`);
      }
    } catch (error) {
      console.error('Emergency reset error:', error);
    }

    if (apiContext) {
      await apiContext.dispose();
    }
  });

  test('Test 1: should block non-whitelisted IP when Cerberus enabled', async () => {
    // Use a fake whitelist IP that will never match the test runner
    const fakeWhitelist = '192.0.2.1/32'; // RFC 5737 TEST-NET-1 (documentation only)

    await test.step('Configure admin whitelist with non-matching IP', async () => {
      const response = await apiContext.patch('/api/v1/security/acl', {
        data: {
          enabled: false, // Ensure disabled first
        },
      });
      expect(response.ok()).toBeTruthy();

      // Set the admin whitelist
      const configResponse = await apiContext.patch('/api/v1/config', {
        data: {
          security: {
            admin_whitelist: fakeWhitelist,
          },
        },
      });
      expect(configResponse.ok()).toBeTruthy();
    });

    await test.step('Enable ACL - expect 403 because IP not in whitelist', async () => {
      const response = await apiContext.patch('/api/v1/security/acl', {
        data: { enabled: true },
      });

      // Should be blocked because our IP is not in the admin_whitelist
      expect(response.status()).toBe(403);

      const body = await response.json().catch(() => ({}));
      expect(body.error || '').toMatch(/whitelist|forbidden|access/i);
    });
  });

  test('Test 2: should allow whitelisted IP to enable Cerberus', async () => {
    // Use localhost/Docker network IP that will match test runner
    // In Docker compose, Playwright runs from host connecting to localhost:8080
    const testWhitelist = '127.0.0.1/32,172.16.0.0/12,192.168.0.0/16,10.0.0.0/8';

    await test.step('Configure admin whitelist with test IP ranges', async () => {
      const response = await apiContext.patch('/api/v1/config', {
        data: {
          security: {
            admin_whitelist: testWhitelist,
          },
        },
      });
      expect(response.ok()).toBeTruthy();
    });

    await test.step('Enable ACL with whitelisted IP', async () => {
      const response = await apiContext.patch('/api/v1/security/acl', {
        data: { enabled: true },
      });
      expect(response.ok()).toBeTruthy();

      const body = await response.json();
      expect(body.enabled).toBe(true);
    });

    await test.step('Verify ACL is enforcing', async () => {
      const response = await apiContext.get('/api/v1/security/status');
      expect(response.ok()).toBeTruthy();

      const body = await response.json();
      expect(body.acl?.enabled).toBe(true);
    });
  });

  test('Test 3: should allow emergency token to bypass admin whitelist', async ({ request }) => {
    await test.step('Configure admin whitelist with non-matching IP', async () => {
      // First disable ACL so we can change config
      await request.post('http://localhost:2020/emergency/security-reset', {
        headers: {
          'Authorization': 'Basic ' + Buffer.from('admin:changeme').toString('base64'),
          'X-Emergency-Token': EMERGENCY_TOKEN,
        },
        data: { reason: 'Test setup - reset for emergency token test' },
      });

      const response = await apiContext.patch('/api/v1/config', {
        data: {
          security: {
            admin_whitelist: '192.0.2.1/32', // Fake IP
          },
        },
      });
      expect(response.ok()).toBeTruthy();
    });

    await test.step('Enable ACL using emergency token despite IP mismatch', async () => {
      const response = await apiContext.patch('/api/v1/security/acl', {
        data: { enabled: true },
        headers: {
          'X-Emergency-Token': EMERGENCY_TOKEN,
        },
      });

      // Should succeed with valid emergency token even though IP not in whitelist
      expect(response.ok()).toBeTruthy();
    });
  });
});
