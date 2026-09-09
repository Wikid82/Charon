/**
 * CrowdSec Admin API Authorization Enforcement (GHSA-3gc6-295r-xm5m)
 *
 * Advisory: a low-privilege `role=user` account (previously obtainable via the
 * public `POST /auth/register` endpoint) could reach the entire
 * `/api/v1/admin/crowdsec/*` surface because those routes were mounted on the
 * bare `management` group, guarded only by `RequireManagementAccess()` which
 * rejects `role=passthrough` only.
 *
 * Shipped behaviour (Part A — `crowdsecHandler.RegisterRoutes(managementAdmin)`
 * with `RequireRole(admin)`):
 *  - `role=user` -> 403 on every CrowdSec admin route.
 *  - The same `role=user` token is still a valid session (200 on a genuinely
 *    `role=user`-allowed endpoint).
 *  - `role=admin` reaches the handler (never 401/403; 200/404/500 depending on
 *    whether the CrowdSec LAPI is running in the test environment).
 *  - The `/security/crowdsec` UI route redirects a non-admin and the nav entry
 *    is hidden for `role=user`.
 *
 * Runs only under `--project=security-tests` (the browser projects `testIgnore`
 * this directory).
 */

import { test, expect } from '../fixtures/test';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import { TestDataManager } from '../utils/TestDataManager';
import { TEST_PASSWORD } from '../fixtures/auth-fixtures';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

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

test.describe('CrowdSec Admin API Authorization (GHSA-3gc6-295r-xm5m)', () => {
  let testData: TestDataManager;
  let adminApiContext: APIRequestContext;
  let adminContext: APIRequestContext;
  let userContext: APIRequestContext;
  let anonContext: APIRequestContext;
  let adminToken: string;
  let userToken: string;

  test.beforeAll(async () => {
    // Admin-authenticated context from the shared setup session — used to mint
    // the per-suite fixture users via the existing TestDataManager helper.
    adminApiContext = await request.newContext({ baseURL: BASE_URL, storageState: STORAGE_STATE });
    testData = new TestDataManager(adminApiContext, 'crowdsec-admin-authz');

    const userRecord = await testData.createUser({
      name: `CrowdSec AuthZ User ${Date.now()}`,
      email: 'crowdsec-authz-user@test.local',
      password: TEST_PASSWORD,
      role: 'user',
    });
    userToken = userRecord.token;

    const adminRecord = await testData.createUser({
      name: `CrowdSec AuthZ Admin ${Date.now()}`,
      email: 'crowdsec-authz-admin@test.local',
      password: TEST_PASSWORD,
      role: 'admin',
    });
    adminToken = adminRecord.token;

    expect(userToken, 'role=user fixture token').toBeTruthy();
    expect(adminToken, 'role=admin fixture token').toBeTruthy();

    adminContext = await request.newContext({ baseURL: BASE_URL });
    userContext = await request.newContext({ baseURL: BASE_URL });
    anonContext = await request.newContext({
      baseURL: BASE_URL,
      storageState: { cookies: [], origins: [] },
    });
  });

  test.afterAll(async () => {
    await testData?.cleanup();
    await adminApiContext?.dispose();
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
    // Assertion corrected vs. the original fixme draft: it used
    // `POST /admin/crowdsec/ban`, whose handler shells out to `cscli decisions
    // add` and blocks indefinitely when no CrowdSec LAPI is reachable (the
    // local E2E compose ships no CrowdSec service). `POST /admin/crowdsec/stop`
    // is an equivalent privileged, state-changing CrowdSec route that
    // exercises the same `managementAdmin` authorization path without an
    // external dependency.
    const response = await adminContext.post('/api/v1/admin/crowdsec/stop', {
      headers: { Authorization: `Bearer ${adminToken}` },
    });
    expect(response.status()).not.toBe(401);
    expect(response.status()).not.toBe(403);
  });

  test('the /security/crowdsec UI route blocks a non-admin', async ({ page }) => {
    await test.step('authenticate the browser session as role=user', async () => {
      await page.context().clearCookies();
      await page.goto('/');
      await page.evaluate((token) => {
        window.localStorage.setItem('charon_auth_token', token);
      }, userToken);
      await page.reload();
    });

    await test.step('navigating directly to /security/crowdsec redirects away', async () => {
      await page.goto('/security/crowdsec');
      await expect(page).toHaveURL((url) => !url.pathname.startsWith('/security/crowdsec'));
      await expect(page.getByRole('main')).toBeVisible();
    });

    await test.step('the CrowdSec nav entry is not shown to role=user', async () => {
      await expect(
        page.getByRole('navigation').getByRole('link', { name: /crowdsec/i })
      ).toHaveCount(0);
    });
  });
});
