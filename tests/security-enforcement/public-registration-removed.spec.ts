/**
 * Public Registration Endpoint Removed (GHSA-3gc6-295r-xm5m, Part C)
 *
 * Target behaviour (implemented in later commits of this feature):
 *  - `POST /api/v1/auth/register` no longer exists -> 404 (route deleted).
 *  - Any method on `/api/v1/auth/register` -> 404.
 *  - First-admin bootstrap via `POST /api/v1/setup` still works when setup is
 *    required; on an already-bootstrapped instance it returns 403
 *    "Setup already completed".
 *  - The existing admin "Invite User" -> `/accept-invite` flow still creates a
 *    working, non-admin (`role=user`) account:
 *      admin POST /api/v1/users/invite  ->  GET /api/v1/invite/validate
 *      ->  POST /api/v1/invite/accept   ->  POST /api/v1/auth/login (200)
 *
 * All tests are `test.fixme` until Part C lands (spec §3.3). The suite must
 * collect and report 0 failures / all skipped.
 */

import { test, expect, request as playwrightRequest } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

const TEST_USERS = {
  admin: { email: 'admin@test.local', password: 'AdminPassword123!' },
};

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

/** Extract the raw invite token from an invite URL query string. */
function tokenFromInviteUrl(inviteUrl: string | undefined): string | null {
  if (!inviteUrl) return null;
  try {
    return new URL(inviteUrl, BASE_URL).searchParams.get('token');
  } catch {
    return null;
  }
}

test.describe.fixme('Public registration endpoint removed (GHSA-3gc6-295r-xm5m)', () => {
  let anonContext: APIRequestContext;
  let adminContext: APIRequestContext;
  let adminToken: string | null;

  test.beforeAll(async () => {
    anonContext = await playwrightRequest.newContext({
      baseURL: BASE_URL,
      storageState: { cookies: [], origins: [] },
    });
    adminContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    adminToken = await loginAndGetToken(adminContext, TEST_USERS.admin);
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

  test('first-admin bootstrap via POST /api/v1/setup still works when setup is required', async () => {
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
      // Already bootstrapped: the endpoint stays closed.
      const setupResponse = await anonContext.post('/api/v1/setup', {
        data: { name: 'Second Admin', email: 'second-admin@test.local', password: 'SecondAdminPass123!' },
      });
      expect(setupResponse.status()).toBe(403);
      const body = await setupResponse.json();
      expect(String(body.error)).toMatch(/already completed/i);
    }
  });

  test('admin invite flow still creates a working non-admin account', async () => {
    const inviteeEmail = `invitee-${Date.now()}@test.local`;
    const inviteePassword = 'InviteePass123!';
    let inviteToken: string | null = null;

    await test.step('admin creates an invite', async () => {
      const response = await adminContext.post('/api/v1/users/invite', {
        headers: { Authorization: `Bearer ${adminToken}` },
        data: { email: inviteeEmail, role: 'user' },
      });
      expect(response.status()).toBe(201);
      const body = await response.json();
      inviteToken = tokenFromInviteUrl(body.invite_url);
      // If `app.public_url` is unset the backend omits `invite_url`; implementing
      // agents must expose the raw token for E2E (see report). Fall back to the
      // preview endpoint which returns the same accept-invite URL.
      if (!inviteToken) {
        const preview = await adminContext.post('/api/v1/users/preview-invite-url', {
          headers: { Authorization: `Bearer ${adminToken}` },
          data: { email: inviteeEmail },
        });
        if (preview.ok()) {
          const previewBody = await preview.json();
          inviteToken = tokenFromInviteUrl(previewBody.preview_url);
        }
      }
      expect(inviteToken).toBeTruthy();
    });

    await test.step('the invite token validates', async () => {
      const response = await anonContext.get('/api/v1/invite/validate', {
        params: { token: inviteToken ?? '' },
      });
      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.email).toBe(inviteeEmail);
    });

    await test.step('the invitee accepts and sets a password', async () => {
      const response = await anonContext.post('/api/v1/invite/accept', {
        data: { token: inviteToken ?? '', name: 'Invited User', password: inviteePassword },
      });
      expect(response.status()).toBe(200);
    });

    await test.step('the new account can log in and is a non-admin', async () => {
      const loginContext = await playwrightRequest.newContext({
        baseURL: BASE_URL,
        storageState: { cookies: [], origins: [] },
      });
      try {
        const loginResponse = await loginContext.post('/api/v1/auth/login', {
          data: { email: inviteeEmail, password: inviteePassword },
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
});
