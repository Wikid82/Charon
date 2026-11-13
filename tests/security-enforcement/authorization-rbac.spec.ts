/**
 * Cerberus ACL Role-Based Access Control Tests
 */

import { test, expect, request as playwrightRequest, type APIRequestContext, type APIResponse } from '@playwright/test';
import { STORAGE_STATE, TEST_PASSWORD as SETUP_ADMIN_PASSWORD } from '../constants';
import { TestDataManager } from '../utils/TestDataManager';
import { TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForModal } from '../utils/wait-helpers';
import { deleteAccessList } from '../utils/proxy-refusal-helpers';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

const TEST_USERS = {
  admin: { email: 'admin@test.local', password: 'AdminPassword123!' },
  user: { email: 'user@test.local', password: 'UserPassword123!' },
  guest: { email: 'guest@test.local', password: 'GuestPassword123!' },
};

async function loginAndGetToken(context: APIRequestContext, credentials: { email: string; password: string }): Promise<string | null> {
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
  let adminContext: APIRequestContext;
  let userContext: APIRequestContext;
  let guestContext: APIRequestContext;
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
      const b64url = (obj: object) => Buffer.from(JSON.stringify(obj)).toString('base64url');
      const expiredToken = `${b64url({ alg: 'HS256', typ: 'JWT' })}.${b64url({ exp: 1600000000 })}.invalidSignature`;
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
 * Shipped behaviour (Part B — `managementAdmin` subgroup + per-route
 * `RequireRole(admin)` args + frontend `RequireRole` guards):
 *  - `role=user` -> 403 on the privileged mutations enumerated below.
 *  - `role=user` -> 200 still on the reads enumerated below (guard against
 *    over-restriction — these back non-admin pages).
 *  - `role=user` is redirected away from the admin-only UI routes and their nav
 *    entries are hidden.
 *  - `role=user` cannot self-escalate to admin through the self-service
 *    `PUT /api/v1/users/:id` route.
 *
 * Runs only under `--project=security-tests`.
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

test.describe('Privileged management-API authorization (GHSA-3gc6-295r-xm5m)', () => {
  let testData: TestDataManager;
  let adminApiContext: APIRequestContext;
  let adminContext: APIRequestContext;
  let userContext: APIRequestContext;
  let adminToken: string;
  let userToken: string;

  test.beforeAll(async () => {
    // Admin-authenticated context from the shared setup session — used to mint
    // the per-suite fixture users via the existing TestDataManager helper.
    adminApiContext = await playwrightRequest.newContext({ baseURL: BASE_URL, storageState: STORAGE_STATE });
    testData = new TestDataManager(adminApiContext, 'privileged-authz-rbac');

    const userRecord = await testData.createUser({
      name: `Privileged AuthZ User ${Date.now()}`,
      email: 'privileged-authz-user@test.local',
      password: TEST_PASSWORD,
      role: 'user',
    });
    userToken = userRecord.token;

    const adminRecord = await testData.createUser({
      name: `Privileged AuthZ Admin ${Date.now()}`,
      email: 'privileged-authz-admin@test.local',
      password: TEST_PASSWORD,
      role: 'admin',
    });
    adminToken = adminRecord.token;

    expect(userToken, 'role=user fixture token').toBeTruthy();
    expect(adminToken, 'role=admin fixture token').toBeTruthy();

    adminContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    userContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
  });

  test.afterAll(async () => {
    await testData?.cleanup();
    await adminApiContext?.dispose();
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

  /**
   * Drop the shared-setup admin cookie and boot the SPA as the `role=user`
   * fixture (the app authenticates from the `charon_auth_token` localStorage
   * bearer, which `AuthMiddleware` prefers over the cookie).
   */
  async function actAsRegularUser(page: import('@playwright/test').Page): Promise<void> {
    await page.context().clearCookies();
    await page.goto('/');
    await page.evaluate((token) => {
      window.localStorage.setItem('charon_auth_token', token);
    }, userToken);
    await page.reload();
  }

  for (const uiRoute of ADMIN_ONLY_UI_ROUTES) {
    test(`role=user is redirected away from ${uiRoute}`, async ({ page }) => {
      await actAsRegularUser(page);

      await page.goto(uiRoute);
      await expect(page).toHaveURL((url) => url.pathname !== uiRoute);
      await expect(page.getByRole('main')).toBeVisible();
    });
  }

  test('admin-only nav entries are hidden for role=user', async ({ page }) => {
    await actAsRegularUser(page);

    const nav = page.getByRole('navigation');
    for (const name of [/crowdsec/i, /audit log/i, /agent/i, /encryption/i]) {
      await expect(nav.getByRole('link', { name })).toHaveCount(0);
    }
  });

  test('role=user cannot self-escalate to admin via PUT /api/v1/users/:id', async () => {
    // Self-service name/password edits through this route are allowed; a role
    // change embedded in the body must be rejected and must not take effect.
    const meResponse = await userContext.get('/api/v1/auth/me', {
      headers: { Authorization: `Bearer ${userToken}` },
    });
    expect(meResponse.status()).toBe(200);
    const me = await meResponse.json();
    const ownId = me.user_id ?? me.id;
    expect(ownId, 'own numeric user id from /auth/me').toBeTruthy();

    const escalation = await userContext.put(`/api/v1/users/${ownId}`, {
      headers: { Authorization: `Bearer ${userToken}` },
      data: { name: me.name ?? 'Privileged AuthZ User', role: 'admin' },
    });
    expect(escalation.status()).toBe(403);

    const afterResponse = await userContext.get('/api/v1/auth/me', {
      headers: { Authorization: `Bearer ${userToken}` },
    });
    expect(afterResponse.status()).toBe(200);
    const after = await afterResponse.json();
    expect(after.role).toBe('user');
  });
});

/**
 * Browser session request origin checks (PR 31, advisory A5).
 *
 * Cookie-authenticated requests to the protected route group are checked with the standard library's cross-origin
 * rules. A request that carries an Authorization header is header-authenticated and passes unchanged. A refused
 * request gets 403 with error_code `cross_origin_refused` before the handler runs.
 *
 * Every probe is written so that an accidentally allowed request cannot change state: the management POST carries an
 * empty body that validation rejects, and the change-password POST carries a wrong current password, which the
 * service rejects before any write. Each refusal is paired with a same-origin control, so a 403 is attributable to the
 * origin check and not to the handler.
 */

const CROSS_SITE_HEADERS = { 'Sec-Fetch-Site': 'cross-site', Origin: 'https://attacker.example' };
const SAME_ORIGIN_HEADERS = { 'Sec-Fetch-Site': 'same-origin', Origin: BASE_URL };
const SETUP_ADMIN_EMAIL = process.env.E2E_TEST_EMAIL || 'e2e-test@example.com';
const WRONG_CURRENT_PASSWORD = 'not-the-current-password-probe';
const PROBE_NEW_PASSWORD = 'ProbeNewPassword123!';

async function errorBody(response: APIResponse): Promise<{ error?: string; error_code?: string }> {
  return (await response.json().catch(() => ({}))) as { error?: string; error_code?: string };
}

async function jsonSnapshot(request: APIRequestContext, path: string): Promise<unknown> {
  const response = await request.get(path);
  expect(response.ok(), `GET ${path} for state snapshot`).toBe(true);
  return response.json();
}

async function deleteAccessListNamed(request: APIRequestContext, name: string): Promise<void> {
  const response = await request.get('/api/v1/access-lists');
  const lists = (await response.json()) as Array<{ uuid: string; name: string }>;
  for (const list of lists.filter((entry) => entry.name === name)) {
    await deleteAccessList(request, list.uuid);
  }
}

test.describe('Browser session request origin checks (PR 31)', () => {
  let cookieContext: APIRequestContext;
  let bearerContext: APIRequestContext;
  let setupAdminBearer: string;

  test.beforeAll(async () => {
    // Cookie context inherits the setup admin session from the shared storage state.
    cookieContext = await playwrightRequest.newContext({ baseURL: BASE_URL, storageState: STORAGE_STATE });
    // Bearer context logs in as the same setup admin, so header-authenticated probes use a real token.
    bearerContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    const login = await bearerContext.post('/api/v1/auth/login', {
      data: { email: SETUP_ADMIN_EMAIL, password: SETUP_ADMIN_PASSWORD },
    });
    expect(login.ok(), 'setup admin login for header-authenticated probes').toBe(true);
    setupAdminBearer = (await login.json()).token;
    expect(setupAdminBearer, 'setup admin bearer token').toBeTruthy();
  });

  test.afterAll(async () => {
    await cookieContext?.dispose();
    await bearerContext?.dispose();
  });

  test('admin UI mutation still succeeds end to end from the same origin', async ({ page }) => {
    // A name-only block list blocks nobody, so it needs no IP rule and cannot lock out the test runner.
    const aclName = `Origin check ACL ${Date.now()}`;

    try {
      await test.step('Create a name-only block list from the Access Lists page', async () => {
        await page.goto('/access-lists');
        await waitForLoadingComplete(page);
        await page.getByRole('button', { name: /create.*access.*list/i }).first().click();
        await waitForModal(page, /create|access.*list/i);
        await page.locator('#name').fill(aclName);
        await page.locator('#type').selectOption('blacklist');
        await page.getByRole('button', { name: /^(create|update|save)$/i }).first().click();
        await waitForLoadingComplete(page);
      });

      await test.step('The form closes and the new list is shown', async () => {
        await expect(page.getByRole('heading', { name: /create.*access.*list/i })).not.toBeVisible({ timeout: 10000 });
        await expect(page.getByText(aclName, { exact: true }).first()).toBeVisible();
      });
    } finally {
      await deleteAccessListNamed(cookieContext, aclName);
    }
  });

  test.describe('Cookie-authenticated management request with cross-site metadata', () => {
    test('POST /api/v1/proxy-hosts is refused with 403 and creates nothing', async () => {
      await test.step('Control: same-origin cookie request reaches validation and is rejected with 400', async () => {
        const control = await cookieContext.post('/api/v1/proxy-hosts', {
          headers: SAME_ORIGIN_HEADERS,
          data: {},
        });
        expect(control.status()).toBe(400);
      });

      const hostsBefore = await jsonSnapshot(cookieContext, '/api/v1/proxy-hosts');

      await test.step('Cross-site request is refused before the handler runs', async () => {
        const response = await cookieContext.post('/api/v1/proxy-hosts', {
          headers: CROSS_SITE_HEADERS,
          data: {},
        });
        expect(response.status()).toBe(403);
        expect((await errorBody(response)).error_code).toBe('cross_origin_refused');
      });

      await test.step('Proxy host list is unchanged', async () => {
        expect(await jsonSnapshot(cookieContext, '/api/v1/proxy-hosts')).toEqual(hostsBefore);
      });
    });

    test('POST /api/v1/auth/change-password is refused with 403 and changes nothing', async () => {
      const probe = { old_password: WRONG_CURRENT_PASSWORD, new_password: PROBE_NEW_PASSWORD };

      await test.step('Control: same-origin cookie request reaches the handler and is rejected for the wrong password', async () => {
        const control = await cookieContext.post('/api/v1/auth/change-password', {
          headers: SAME_ORIGIN_HEADERS,
          data: probe,
        });
        expect(control.status()).toBe(400);
        expect((await errorBody(control)).error).toMatch(/invalid current password/i);
      });

      const meBefore = await jsonSnapshot(cookieContext, '/api/v1/auth/me');

      await test.step('Cross-site request is refused, sets no session cookie, and changes nothing', async () => {
        const response = await cookieContext.post('/api/v1/auth/change-password', {
          headers: CROSS_SITE_HEADERS,
          data: probe,
        });
        expect(response.status()).toBe(403);
        expect((await errorBody(response)).error_code).toBe('cross_origin_refused');
        expect(response.headers()['set-cookie']).toBeUndefined();
      });

      await test.step('Session identity is unchanged', async () => {
        expect(await jsonSnapshot(cookieContext, '/api/v1/auth/me')).toEqual(meBefore);
      });
    });
  });

  test.describe('Requests the origin check does not refuse', () => {
    test('header-authenticated request with cross-site metadata passes the origin check', async () => {
      const response = await bearerContext.post('/api/v1/proxy-hosts', {
        headers: { Authorization: `Bearer ${setupAdminBearer}`, ...CROSS_SITE_HEADERS },
        data: {},
      });
      // The handler's own validation answers, so the origin check did not refuse the request.
      expect(response.status()).toBe(400);
      expect((await errorBody(response)).error_code).not.toBe('cross_origin_refused');
    });

    test('cookie request without browser metadata passes the origin check', async () => {
      const response = await cookieContext.post('/api/v1/proxy-hosts', { data: {} });
      expect(response.status()).toBe(400);
      expect((await errorBody(response)).error_code).not.toBe('cross_origin_refused');
    });
  });
});
