/**
 * Public Registration Endpoint Removed (GHSA-3gc6-295r-xm5m, Part C)
 *
 * Shipped behaviour:
 *  - `POST /api/v1/auth/register` no longer exists -> 404 (route deleted).
 *  - Any method on `/api/v1/auth/register` -> 404.
 *  - First-admin bootstrap via `POST /api/v1/setup` still works when setup is
 *    required; on an already-bootstrapped instance it returns 403
 *    "Setup already completed".
 *  - Post-bootstrap account creation is served by the existing admin surfaces:
 *      * `POST /api/v1/users` (direct create), and
 *      * the admin invite flow `POST /api/v1/users/invite` ->
 *        `GET /api/v1/invite/validate` -> `POST /api/v1/invite/accept`.
 *
 * NOTE ON THE INVITE FLOW COVERAGE (corrected vs. the original fixme draft):
 * The raw invite token is never returned by any HTTP response in shipped
 * builds — `InviteUser` redacts `invite_url` to "" / "[REDACTED]"
 * (`redactInviteURL`, shipped since 2026-02) and `PreviewInviteURL` returns a
 * placeholder `SAMPLE_TOKEN_PREVIEW`. The token only reaches an invitee via a
 * configured-SMTP email. A full validate->accept->login round-trip therefore
 * cannot be driven end-to-end over HTTP in the E2E environment. This spec
 * instead asserts the invite endpoints exist, are admin-guarded, create a
 * pending user, and reject invalid tokens — and separately proves the
 * admin direct-create path yields a working `role=user` login (the concrete
 * replacement for public self-registration).
 *
 * Runs only under `--project=security-tests`.
 */

import { test, expect } from '../fixtures/test';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import { TEST_PASSWORD } from '../fixtures/auth-fixtures';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

test.describe('Public registration endpoint removed (GHSA-3gc6-295r-xm5m)', () => {
  let anonContext: APIRequestContext;
  let adminContext: APIRequestContext;

  test.beforeAll(async () => {
    anonContext = await request.newContext({
      baseURL: BASE_URL,
      storageState: { cookies: [], origins: [] },
    });
    // Admin-authenticated via the shared setup session (cookie auth).
    adminContext = await request.newContext({ baseURL: BASE_URL, storageState: STORAGE_STATE });
  });

  test.afterAll(async () => {
    await anonContext?.dispose();
    await adminContext?.dispose();
  });

  test('POST /api/v1/auth/register returns 404 (route deleted)', async () => {
    const response = await anonContext.post('/api/v1/auth/register', {
      data: { email: `attacker-${Date.now()}@example.com`, password: 'AttackerPass123!', name: 'Attacker' },
    });
    expect(response.status()).toBe(404);
  });

  test('GET /api/v1/auth/register returns 404 (route deleted for every method)', async () => {
    const response = await anonContext.get('/api/v1/auth/register');
    expect(response.status()).toBe(404);
  });

  test('first-admin bootstrap via POST /api/v1/setup still works / stays closed', async () => {
    const statusResponse = await anonContext.get('/api/v1/setup');
    expect(statusResponse.status()).toBe(200);
    const status = await statusResponse.json();
    expect(typeof status.setupRequired).toBe('boolean');

    if (status.setupRequired) {
      const setupResponse = await anonContext.post('/api/v1/setup', {
        data: { name: 'First Admin', email: 'first-admin@test.local', password: 'FirstAdminPass123!' },
      });
      expect(setupResponse.status()).toBe(201);
    } else {
      // Already bootstrapped (the E2E setup fixture created the first admin):
      // the endpoint stays closed.
      const setupResponse = await anonContext.post('/api/v1/setup', {
        data: { name: 'Second Admin', email: 'second-admin@test.local', password: 'SecondAdminPass123!' },
      });
      expect(setupResponse.status()).toBe(403);
      const body = await setupResponse.json();
      expect(String(body.error)).toMatch(/already completed/i);
    }
  });

  test('the admin invite endpoints remain available and reject invalid tokens', async () => {
    const inviteeEmail = `invitee-${Date.now()}@test.local`;

    await test.step('admin can issue an invite for a role=user account', async () => {
      const response = await adminContext.post('/api/v1/users/invite', {
        data: { email: inviteeEmail, role: 'user' },
      });
      expect(response.status()).toBe(201);
      const body = await response.json();
      expect(body.role).toBe('user');
      // Token material is masked in the response — never returned raw.
      expect(body.invite_token_masked).toBe('********');
      expect(body.invite_url ?? '').not.toContain('token=');
    });

    await test.step('the invitee now exists as a pending, disabled user', async () => {
      const response = await adminContext.get('/api/v1/users');
      expect(response.status()).toBe(200);
      const users = await response.json();
      const invitee = users.find((u: { email?: string }) => u.email === inviteeEmail);
      expect(invitee, 'invited user present in the user list').toBeTruthy();
      expect(invitee.invite_status).toBe('pending');
      expect(invitee.enabled).toBe(false);
    });

    await test.step('the public invite-validation endpoint exists and guards its input', async () => {
      const missing = await anonContext.get('/api/v1/invite/validate');
      expect(missing.status()).toBe(400);

      const bogus = await anonContext.get('/api/v1/invite/validate', {
        params: { token: 'bogus-token-that-does-not-exist' },
      });
      expect(bogus.status()).toBe(404);
    });

    await test.step('the public invite-accept endpoint rejects an unknown token', async () => {
      const response = await anonContext.post('/api/v1/invite/accept', {
        data: { token: 'bogus-token-that-does-not-exist', name: 'Nope', password: 'NopePass123!' },
      });
      expect(response.status()).toBe(404);
    });
  });

  test('an admin-created account works as a non-admin (self-registration replacement)', async () => {
    const email = `direct-user-${Date.now()}@test.local`;

    const createResponse = await adminContext.post('/api/v1/users', {
      data: { name: 'Direct User', email, password: TEST_PASSWORD, role: 'user' },
    });
    expect(createResponse.status()).toBe(201);

    const loginContext = await request.newContext({
      baseURL: BASE_URL,
      storageState: { cookies: [], origins: [] },
    });
    try {
      const loginResponse = await loginContext.post('/api/v1/auth/login', {
        data: { email, password: TEST_PASSWORD },
      });
      expect(loginResponse.status()).toBe(200);
      const loginBody = await loginResponse.json();
      const token = loginBody.token || loginBody.access_token;
      expect(token).toBeTruthy();

      const meResponse = await loginContext.get('/api/v1/auth/me', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(meResponse.status()).toBe(200);
      const me = await meResponse.json();
      expect(me.role).toBe('user');
    } finally {
      await loginContext.dispose();
    }
  });
});
