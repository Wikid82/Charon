/**
 * CrowdSec Admin API Authorization Enforcement (GHSA-3gc6-295r-xm5m)
 *
 * Advisory: a low-privilege `role=user` account (previously obtainable via the
 * public `POST /auth/register` endpoint) can reach the entire
 * `/api/v1/admin/crowdsec/*` surface because those routes are mounted on the
 * bare `management` group, guarded only by `RequireManagementAccess()` which
 * rejects `role=passthrough` only.
 *
 * Target behaviour (implemented in later commits of this feature):
 *  - `role=user` -> 403 on every CrowdSec admin route.
 *  - The same `role=user` token is still a valid session (200 on a genuinely
 *    `role=user`-allowed endpoint).
 *  - `role=admin` reaches the handler (never 401/403; 200 or 500 depending on
 *    whether the CrowdSec LAPI is running in the test environment).
 *  - The `/security/crowdsec` UI route redirects / blocks a non-admin and the
 *    nav entry is hidden for `role=user`.
 *
 * All tests are `test.fixme` until Part A lands (spec §3.1). The suite must
 * collect and report 0 failures / all skipped.
 */

import { test, expect, request as playwrightRequest } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

const TEST_USERS = {
  admin: { email: 'admin@test.local', password: 'AdminPassword123!' },
  user: { email: 'user@test.local', password: 'UserPassword123!' },
};

/** CrowdSec admin routes a non-admin must never reach (spec §3.1.4). */
const CROWDSEC_ADMIN_ROUTES: Array<{
  method: 'get' | 'post';
  path: string;
  data?: Record<string, unknown>;
}> = [
  { method: 'post', path: '/api/v1/admin/crowdsec/stop' },
  { method: 'get', path: '/api/v1/admin/crowdsec/bouncer/key' },
  { method: 'post', path: '/api/v1/admin/crowdsec/ban', data: { ip: '203.0.113.10', duration: '1h', reason: 'e2e' } },
  { method: 'get', path: '/api/v1/admin/crowdsec/file?path=acquis.yaml' },
];

async function loginAndGetToken(
  context: APIRequestContext,
  credentials: { email: string; password: string }
): Promise<string | null> {
  try {
    const response = await context.post(`${BASE_URL}/api/v1/auth/login`, { data: credentials });
    if (response.ok()) {
      const data = await response.json();
      return data.token || data.access_token || null;
    }
    return null;
  } catch {
    return null;
  }
}

test.describe.fixme('CrowdSec Admin API Authorization (GHSA-3gc6-295r-xm5m)', () => {
  let adminContext: APIRequestContext;
  let userContext: APIRequestContext;
  let anonContext: APIRequestContext;
  let adminToken: string | null;
  let userToken: string | null;

  test.beforeAll(async () => {
    adminContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    userContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    anonContext = await playwrightRequest.newContext({
      baseURL: BASE_URL,
      storageState: { cookies: [], origins: [] },
    });

    adminToken = await loginAndGetToken(adminContext, TEST_USERS.admin);
    userToken = await loginAndGetToken(userContext, TEST_USERS.user);
  });

  test.afterAll(async () => {
    await adminContext?.dispose();
    await userContext?.dispose();
    await anonContext?.dispose();
  });

  test('control: the role=user token is a valid session (200 on a user-allowed endpoint)', async () => {
    const response = await userContext.get('/api/v1/proxy-hosts', {
      headers: { Authorization: `Bearer ${userToken}` },
    });
    expect(response.status()).toBe(200);
  });

  for (const route of CROWDSEC_ADMIN_ROUTES) {
    test(`role=user is denied (403) on ${route.method.toUpperCase()} ${route.path}`, async () => {
      const response = await userContext[route.method](route.path, {
        headers: { Authorization: `Bearer ${userToken}` },
        ...(route.data ? { data: route.data } : {}),
      });
      expect(response.status()).toBe(403);
    });

    test(`unauthenticated is rejected (401) on ${route.method.toUpperCase()} ${route.path}`, async () => {
      const response = await anonContext[route.method](route.path, {
        ...(route.data ? { data: route.data } : {}),
      });
      expect(response.status()).toBe(401);
    });
  }

  test('role=admin reaches the CrowdSec handler (never 401/403)', async () => {
    for (const path of ['/api/v1/admin/crowdsec/status', '/api/v1/admin/crowdsec/bouncer/key']) {
      const response = await adminContext.get(path, {
        headers: { Authorization: `Bearer ${adminToken}` },
      });
      expect(response.status()).not.toBe(401);
      expect(response.status()).not.toBe(403);
    }
  });

  test('role=admin can invoke a CrowdSec mutation (never 401/403)', async () => {
    const response = await adminContext.post('/api/v1/admin/crowdsec/ban', {
      headers: { Authorization: `Bearer ${adminToken}` },
      data: { ip: '203.0.113.11', duration: '1h', reason: 'e2e-admin' },
    });
    expect(response.status()).not.toBe(401);
    expect(response.status()).not.toBe(403);
  });

  test('the /security/crowdsec UI route blocks a non-admin', async ({ page }) => {
    await test.step('authenticate the browser session as role=user', async () => {
      await page.goto('/');
      await page.evaluate((token) => {
        window.localStorage.setItem('charon_auth_token', token);
      }, userToken ?? '');
    });

    await test.step('navigating directly to /security/crowdsec redirects away', async () => {
      await page.goto('/security/crowdsec');
      await expect(page).toHaveURL((url) => !url.pathname.startsWith('/security/crowdsec'));
    });

    await test.step('the CrowdSec nav entry is not shown to role=user', async () => {
      await expect(
        page.getByRole('navigation').getByRole('link', { name: /crowdsec/i })
      ).toHaveCount(0);
    });
  });
});
