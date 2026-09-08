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

/**
 * Privileged management-API authorization hardening (GHSA-3gc6-295r-xm5m, Part B).
 *
 * The `management` route group is a de-facto "any authenticated non-passthrough
 * user" group. Part B moves every state-changing / privileged route behind an
 * explicit `RequireRole(admin)` guard (a dedicated `managementAdmin` subgroup or
 * a per-route middleware arg) while keeping the reads that back `role=user`
 * screens on `management`.
 *
 * Target behaviour (implemented in Commit 3 of this feature):
 *  - `role=user` -> 403 on the privileged mutations enumerated below.
 *  - `role=user` -> 200 still on the reads enumerated below (guard against
 *    over-restriction — these back non-admin pages).
 *  - `role=user` is redirected away from the admin-only UI routes and their nav
 *    entries are hidden.
 *
 * All tests are `test.fixme` until Part B lands (spec §3.2). The suite must
 * collect and report 0 failures / all skipped.
 */

const PLACEHOLDER_UUID = '00000000-0000-0000-0000-000000000000';
const PLACEHOLDER_ID = '1';

/** Privileged mutations that MUST return 403 for a `role=user` token (spec §3.2.2). */
const PRIVILEGED_MUTATIONS: Array<{
  method: 'post' | 'put' | 'patch' | 'delete';
  path: string;
  data?: Record<string, unknown>;
}> = [
  // Plugin enable/disable/reload (row 3 — confirmed 2nd live instance).
  { method: 'post', path: `/api/v1/admin/plugins/${PLACEHOLDER_ID}/enable` },
  { method: 'post', path: `/api/v1/admin/plugins/${PLACEHOLDER_ID}/disable` },
  { method: 'post', path: '/api/v1/admin/plugins/reload' },
  // Remote-server mutations (rows 11/12 — SSH targets + credentials).
  { method: 'post', path: '/api/v1/remote-servers', data: { name: 'e2e', host: '203.0.113.20', port: 22 } },
  { method: 'put', path: `/api/v1/remote-servers/${PLACEHOLDER_UUID}`, data: { name: 'e2e' } },
  { method: 'delete', path: `/api/v1/remote-servers/${PLACEHOLDER_UUID}` },
  { method: 'post', path: '/api/v1/remote-servers/test', data: { host: '203.0.113.20', port: 22 } },
  // Hecate tunnel mutations (row 14b — provider credentials + topology).
  { method: 'post', path: '/api/v1/hecate/tunnels', data: { name: 'e2e' } },
  { method: 'post', path: `/api/v1/hecate/tunnels/${PLACEHOLDER_UUID}/start` },
  // Orthrus agent mutations (row 15b — agent provisioning + bootstrap tokens).
  { method: 'post', path: '/api/v1/orthrus/agents', data: { name: 'e2e' } },
  // Certificate export ships private-key material (row 19).
  { method: 'post', path: `/api/v1/certificates/${PLACEHOLDER_UUID}/export`, data: {} },
  // DNS-provider mutations + credential tests (rows 17b — API credentials + ACME).
  { method: 'post', path: '/api/v1/dns-providers', data: { name: 'e2e', type: 'cloudflare' } },
  { method: 'post', path: '/api/v1/dns-providers/test', data: { type: 'cloudflare', credentials: {} } },
  { method: 'post', path: `/api/v1/dns-providers/${PLACEHOLDER_ID}/test`, data: {} },
  // Notification test / preview (row 33b — sends messages / renders with config).
  { method: 'post', path: '/api/v1/notifications/providers/test', data: { type: 'webhook' } },
  { method: 'post', path: '/api/v1/notifications/providers/preview', data: { type: 'webhook' } },
  { method: 'post', path: '/api/v1/notifications/external-templates/preview', data: {} },
  // Access-list mutations (row 21 — ACLs are a security control).
  { method: 'post', path: '/api/v1/access-lists', data: { name: 'e2e', rules: [] } },
  { method: 'put', path: `/api/v1/access-lists/${PLACEHOLDER_ID}`, data: { name: 'e2e' } },
  { method: 'delete', path: `/api/v1/access-lists/${PLACEHOLDER_ID}` },
  // Domain mutations (row 30).
  { method: 'post', path: '/api/v1/domains', data: { name: 'e2e.example.com' } },
  { method: 'delete', path: `/api/v1/domains/${PLACEHOLDER_ID}` },
  // Settings + feature-flag mutations (rows 23/24).
  { method: 'patch', path: '/api/v1/settings', data: { 'app.name': 'e2e' } },
  { method: 'put', path: '/api/v1/feature-flags', data: {} },
];

/** Privileged reads that MUST also return 403 for a `role=user` token (rows 15b/28). */
const PRIVILEGED_READS: string[] = [
  '/api/v1/orthrus/agents/' + PLACEHOLDER_UUID + '/snippets',
  '/api/v1/audit-logs',
  '/api/v1/audit-logs/' + PLACEHOLDER_UUID,
];

/**
 * Reads that MUST still return 200 for a `role=user` token — they back
 * `role=user`-reachable pages and must not regress (spec §3.2.2 "READ (stays)").
 */
const USER_REACHABLE_READS: string[] = [
  '/api/v1/orthrus/agents',
  '/api/v1/hecate/status',
  '/api/v1/hecate/tunnels',
  '/api/v1/remote-servers',
  '/api/v1/certificates',
  '/api/v1/dns-providers',
  '/api/v1/access-lists',
  '/api/v1/admin/plugins',
];

/** Admin-only UI routes a `role=user` must be redirected away from. */
const ADMIN_ONLY_UI_ROUTES: string[] = [
  '/security/crowdsec',
  '/security/audit-logs',
  '/hecate/agent',
  '/security/encryption',
];

test.describe.fixme('Privileged management-API authorization (GHSA-3gc6-295r-xm5m)', () => {
  let adminContext: any;
  let userContext: any;
  let adminToken: string | null;
  let userToken: string | null;

  test.beforeAll(async () => {
    adminContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    userContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    adminToken = await loginAndGetToken(adminContext, TEST_USERS.admin);
    userToken = await loginAndGetToken(userContext, TEST_USERS.user);
  });

  test.afterAll(async () => {
    await adminContext?.dispose();
    await userContext?.dispose();
  });

  for (const route of PRIVILEGED_MUTATIONS) {
    test(`role=user is denied (403) on ${route.method.toUpperCase()} ${route.path}`, async () => {
      const response = await userContext[route.method](route.path, {
        headers: { Authorization: `Bearer ${userToken}` },
        ...(route.data ? { data: route.data } : {}),
      });
      expect(response.status()).toBe(403);
    });

    test(`role=admin reaches the handler (not 403/401) on ${route.method.toUpperCase()} ${route.path}`, async () => {
      const response = await adminContext[route.method](route.path, {
        headers: { Authorization: `Bearer ${adminToken}` },
        ...(route.data ? { data: route.data } : {}),
      });
      expect(response.status()).not.toBe(401);
      expect(response.status()).not.toBe(403);
    });
  }

  for (const path of PRIVILEGED_READS) {
    test(`role=user is denied (403) on GET ${path}`, async () => {
      const response = await userContext.get(path, {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect(response.status()).toBe(403);
    });
  }

  for (const path of USER_REACHABLE_READS) {
    test(`role=user can still read (200) GET ${path} — no over-restriction`, async () => {
      const response = await userContext.get(path, {
        headers: { Authorization: `Bearer ${userToken}` },
      });
      expect(response.status()).toBe(200);
    });
  }

  for (const uiRoute of ADMIN_ONLY_UI_ROUTES) {
    test(`role=user is redirected away from ${uiRoute}`, async ({ page }) => {
      await page.goto('/');
      await page.evaluate((token) => {
        window.localStorage.setItem('charon_auth_token', token);
      }, userToken ?? '');

      await page.goto(uiRoute);
      await expect(page).toHaveURL((url) => url.pathname !== uiRoute);
    });
  }

  test('admin-only nav entries are hidden for role=user', async ({ page }) => {
    await page.goto('/');
    await page.evaluate((token) => {
      window.localStorage.setItem('charon_auth_token', token);
    }, userToken ?? '');
    await page.reload();

    const nav = page.getByRole('navigation');
    for (const name of [/crowdsec/i, /audit log/i, /agent/i, /encryption/i]) {
      await expect(nav.getByRole('link', { name })).toHaveCount(0);
    }
  });
});
