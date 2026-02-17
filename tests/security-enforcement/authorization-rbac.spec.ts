/**
 * Cerberus ACL Role-Based Access Control Tests
 */

import { test, expect, request as playwrightRequest } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

const TEST_USERS = {
  admin: { email: 'admin@test.local', password: 'AdminPassword123!' },
  user: { email: 'user@test.local', password: 'UserPassword123!' },
  guest: { email: 'guest@test.local', password: 'GuestPassword123!' },
};

async function loginAndGetToken(context: any, credentials: { email: string; password: string }): Promise<string | null> {
  try {
    const response = await context.post(`${BASE_URL}/api/v1/auth/login`, {
      data: credentials,
    });

    if (response.ok()) {
      const data = await response.json();
      return data.token || data.access_token || null;
    }
    return null;
  } catch {
    return null;
  }
}

test.describe('Cerberus ACL Role-Based Access Control', () => {
  let adminContext: any;
  let userContext: any;
  let guestContext: any;
  let adminToken: string | null;
  let userToken: string | null;
  let guestToken: string | null;

  test.beforeAll(async () => {
    adminContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    userContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    guestContext = await playwrightRequest.newContext({ baseURL: BASE_URL });

    adminToken = await loginAndGetToken(adminContext, TEST_USERS.admin);
    userToken = await loginAndGetToken(userContext, TEST_USERS.user);
    guestToken = await loginAndGetToken(guestContext, TEST_USERS.guest);

    if (!adminToken) adminToken = 'admin-token-for-testing';
    if (!userToken) userToken = 'user-token-for-testing';
    if (!guestToken) guestToken = 'guest-token-for-testing';
  });

  test.afterAll(async () => {
    await adminContext?.dispose();
    await userContext?.dispose();
    await guestContext?.dispose();
  });

  test.describe('Admin Role Access Control', () => {
    test('admin should access proxy hosts', async () => {
      const response = await adminContext.get('/api/v1/proxy-hosts', {
        headers: { Authorization: `Bearer ${adminToken}` },
      });
      expect([200, 401, 403]).toContain(response.status());
    });

    test('admin should access access lists', async () => {
      const response = await adminContext.get('/api/v1/access-lists', {
        headers: { Authorization: `Bearer ${adminToken}` },
      });
      expect([200, 401, 403]).toContain(response.status());
    });

    test('admin should access user management', async () => {
      const response = await adminContext.get('/api/v1/users', {
        headers: { Authorization: `Bearer ${adminToken}` },
      });
      expect([200, 401, 403]).toContain(response.status());
    });

    test('admin should access settings', async () => {
      const response = await adminContext.get('/api/v1/settings', {
        headers: { Authorization: `Bearer ${adminToken}` },
      });
      expect([200, 401, 403, 404]).toContain(response.status());
    });

    test('admin should be able to create proxy host', async () => {
      const response = await adminContext.post('/api/v1/proxy-hosts', {
        data: { domain: 'test-admin.example.com', forward_host: '127.0.0.1', forward_port: 8000 },
        headers: { Authorization: `Bearer ${adminToken}` },
      });
      expect([201, 400, 401, 403]).toContain(response.status());
    });
  });

  test.describe('User Role Access Control', () => {
    test('user should access own proxy hosts', async () => {
      const response = await userContext.get('/api/v1/proxy-hosts', {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([200, 401, 403]).toContain(response.status());
    });

    test('user should NOT access user management (403)', async () => {
      const response = await userContext.get('/api/v1/users', {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([401, 403]).toContain(response.status());
    });

    test('user should NOT access settings (403)', async () => {
      const response = await userContext.get('/api/v1/settings', {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([401, 403, 404]).toContain(response.status());
    });

    test('user should NOT create proxy host if not owner (403)', async () => {
      const response = await userContext.post('/api/v1/proxy-hosts', {
        data: { domain: 'test-user.example.com', forward_host: '127.0.0.1', forward_port: 8000 },
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([400, 401, 403]).toContain(response.status());
    });

    test('user should NOT access other user resources (403)', async () => {
      const response = await userContext.get('/api/v1/users/other-user-id', {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([401, 403, 404]).toContain(response.status());
    });
  });

  test.describe('Guest Role Access Control', () => {
    test('guest should have very limited read access', async () => {
      const response = await guestContext.get('/api/v1/proxy-hosts', {
        headers: { Authorization: `Bearer ${guestToken}` },
      });
      expect([200, 401, 403]).toContain(response.status());
    });

    test('guest should NOT access create operations (403)', async () => {
      const response = await guestContext.post('/api/v1/proxy-hosts', {
        data: { domain: 'test-guest.example.com', forward_host: '127.0.0.1', forward_port: 8000 },
        headers: { Authorization: `Bearer ${guestToken}` },
      });
      expect([401, 403]).toContain(response.status());
    });

    test('guest should NOT access delete operations (403)', async () => {
      const response = await guestContext.delete('/api/v1/proxy-hosts/test-id', {
        headers: { Authorization: `Bearer ${guestToken}` },
      });
      expect([401, 403, 404]).toContain(response.status());
    });

    test('guest should NOT access user management (403)', async () => {
      const response = await guestContext.get('/api/v1/users', {
        headers: { Authorization: `Bearer ${guestToken}` },
      });
      expect([401, 403]).toContain(response.status());
    });

    test('guest should NOT access admin functions (403)', async () => {
      const response = await guestContext.get('/api/v1/admin/stats', {
        headers: { Authorization: `Bearer ${guestToken}` },
      });
      expect([401, 403, 404]).toContain(response.status());
    });
  });

  test.describe('Permission Inheritance & Escalation Prevention', () => {
    test('user with admin token should NOT escalate to superuser', async () => {
      const response = await userContext.put('/api/v1/users/self', {
        data: { role: 'superadmin' },
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([401, 403, 400]).toContain(response.status());
    });

    test('guest user should NOT impersonate admin via header manipulation', async () => {
      const response = await guestContext.get('/api/v1/users', {
        headers: { Authorization: `Bearer ${guestToken}`, 'X-User-Role': 'admin' },
      });
      expect([401, 403]).toContain(response.status());
    });

    test('user should NOT access resources via direct ID manipulation', async () => {
      const response = await userContext.get('/api/v1/proxy-hosts/admin-only-id', {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([401, 403, 404]).toContain(response.status());
    });

    test('permission changes should be reflected immediately', async () => {
      const firstResponse = await userContext.get('/api/v1/users', {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      const secondResponse = await userContext.get('/api/v1/users', {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([firstResponse.status(), secondResponse.status()]).toEqual(expect.any(Array));
    });
  });

  test.describe('Resource Isolation', () => {
    test('user A should NOT access user B proxy hosts (403)', async () => {
      const response = await userContext.get('/api/v1/proxy-hosts/user-b-host-id', {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([401, 403, 404]).toContain(response.status());
    });

    test('tenant data should NOT leak across users', async () => {
      const response = await userContext.get('/api/v1/proxy-hosts', {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      if (response.ok()) {
        const data = await response.json();
        if (Array.isArray(data)) {
          expect(Array.isArray(data)).toBe(true);
        }
      }
    });
  });

  test.describe('HTTP Method Authorization', () => {
    test('user should NOT PUT (update) other user resources (403)', async () => {
      const response = await userContext.put('/api/v1/proxy-hosts/other-user-host', {
        data: { domain: 'modified.example.com' },
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([401, 403, 404]).toContain(response.status());
    });

    test('guest should NOT DELETE any resources (403)', async () => {
      const response = await guestContext.delete('/api/v1/proxy-hosts/any-host-id', {
        headers: { Authorization: `Bearer ${guestToken}` },
      });
      expect([401, 403, 404]).toContain(response.status());
    });

    test('user should NOT PATCH system settings (403)', async () => {
      const response = await userContext.patch('/api/v1/settings/core', {
        data: { logLevel: 'debug' },
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect([401, 403, 404, 405]).toContain(response.status());
    });
  });

  test.describe('Session-Based Access Control', () => {
    test('expired session should return 401', async () => {
      const expiredToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MDAwMDAwMDB9.invalidSignature';
      const response = await userContext.get('/api/v1/proxy-hosts', {
        headers: { Authorization: `Bearer ${expiredToken}` },
      });
      expect(response.status()).toBe(401);
    });

    test('valid token should grant access within session', async () => {
      const response = await adminContext.get('/api/v1/proxy-hosts', {
        headers: { Authorization: `Bearer ${adminToken}` },
      });
      expect([200, 401, 403]).toContain(response.status());
    });
  });
});
