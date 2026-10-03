/**
 * Account Credentials E2E Tests
 *
 * Covers sign-in outcomes and password-change behavior:
 * - Every failed sign-in returns the same response and shows the same message
 *   on the login page
 * - An account locks after repeated wrong passwords, and a correct password
 *   does not change the outcome while it is locked
 * - Changing your own password through PUT /api/v1/users/:id needs the current
 *   password and ends existing sessions
 * - POST /api/v1/auth/change-password keeps the caller signed in and ends
 *   other sessions
 * - An administrator can set another user's password without a current one
 *
 * Every test creates its own dedicated users (removed by the testData fixture)
 * and presents a unique forwarded client address, so the shared admin account
 * and other specs' sign-in budgets are never touched.
 *
 * @see backend/internal/services/auth_service.go
 * @see backend/internal/api/handlers/user_handler.go
 */

import { randomInt, randomUUID } from 'crypto';
import {
  request as playwrightRequest,
  type APIRequestContext,
  type APIResponse,
} from '@playwright/test';
import { test, expect, TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { STORAGE_STATE } from '../constants';
import type { TestDataManager } from '../utils/TestDataManager';
import { sendLoginHonoringThrottle } from '../utils/login-throttle';

const LOGIN_PATH = '/api/v1/auth/login';
const GENERIC_SIGN_IN_ERROR = 'invalid credentials';
const FAILED_ATTEMPTS_BEFORE_LOCK = 5;
const WRONG_PASSWORD = 'WrongPassword123!';
const NEW_PASSWORD = 'BrandNewPass456!';

/** A random address in 198.18.0.0/15 (benchmarking range) so each test gets its own bucket. */
function isolatedClientIp(): string {
  return `198.${randomInt(18, 20)}.${randomInt(0, 256)}.${randomInt(1, 255)}`;
}

/** A request context with no session that presents the given forwarded client address. */
async function anonymousContext(baseURL: string, clientIp: string): Promise<APIRequestContext> {
  return playwrightRequest.newContext({
    baseURL,
    storageState: { cookies: [], origins: [] },
    extraHTTPHeaders: { Accept: 'application/json', 'X-Forwarded-For': clientIp },
  });
}

/** A request context carrying the shared admin session, used only to manage dedicated test users. */
async function adminContext(baseURL: string, clientIp: string): Promise<APIRequestContext> {
  return playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
    extraHTTPHeaders: { Accept: 'application/json', 'X-Forwarded-For': clientIp },
  });
}

function signIn(ctx: APIRequestContext, email: string, password: string): Promise<APIResponse> {
  return sendLoginHonoringThrottle(() => ctx.post(LOGIN_PATH, { data: { email, password } }));
}

/** Sign in and return a context that holds the resulting session cookie. */
async function signedInContext(baseURL: string, clientIp: string, email: string, password: string) {
  const ctx = await anonymousContext(baseURL, clientIp);
  const response = await signIn(ctx, email, password);
  expect(response.status(), await response.text()).toBe(200);
  return ctx;
}

/** Create a dedicated regular user through the shared fixture manager. */
function createDedicatedUser(testData: TestDataManager, label: string) {
  const unique = randomUUID().slice(0, 8);
  return testData.createUser({
    name: `Credentials ${label} ${unique}`,
    email: `cred-${label}-${unique}@test.local`,
    password: TEST_PASSWORD,
    role: 'user',
  });
}

/** Lock an account by sending wrong passwords until the threshold is reached. */
async function lockAccount(ctx: APIRequestContext, email: string): Promise<void> {
  for (let attempt = 0; attempt < FAILED_ATTEMPTS_BEFORE_LOCK; attempt += 1) {
    const response = await signIn(ctx, email, WRONG_PASSWORD);
    expect(response.status()).toBe(401);
  }
}

test.describe('Account credentials', () => {
  test.describe('Sign-in responses (API)', () => {
    test('every failed sign-in returns the same response', async ({
      baseURL,
      testData,
    }) => {
      const clientIp = isolatedClientIp();
      const anonymous = await anonymousContext(baseURL!, clientIp);
      const admin = await adminContext(baseURL!, clientIp);
      try {
        const wrongPasswordUser = await createDedicatedUser(testData, 'wrong');
        const disabledUser = await createDedicatedUser(testData, 'disabled');
        const lockedUser = await createDedicatedUser(testData, 'locked');

        await test.step('Disable one account and lock another', async () => {
          const disable = await admin.put(`/api/v1/users/${disabledUser.id}`, { data: { enabled: false } });
          expect(disable.status(), await disable.text()).toBe(200);
          await lockAccount(anonymous, lockedUser.email);
        });

        const outcomes = await test.step('Collect the response for each kind of failed sign-in', async () => {
          const attempts = {
            unknown: { email: `nobody-${randomUUID()}@test.local`, password: TEST_PASSWORD },
            wrongPassword: { email: wrongPasswordUser.email, password: WRONG_PASSWORD },
            disabled: { email: disabledUser.email, password: TEST_PASSWORD },
            locked: { email: lockedUser.email, password: TEST_PASSWORD },
          };
          const results: Record<string, { status: number; body: unknown }> = {};
          for (const [kind, credentials] of Object.entries(attempts)) {
            const response = await signIn(anonymous, credentials.email, credentials.password);
            results[kind] = { status: response.status(), body: await response.json() };
          }
          return results;
        });

        await test.step('Every failure is the same 401 with the same body', () => {
          for (const [kind, outcome] of Object.entries(outcomes)) {
            expect(outcome.status, kind).toBe(401);
            expect(outcome.body, kind).toEqual({ error: GENERIC_SIGN_IN_ERROR });
          }
        });
      } finally {
        await Promise.all([anonymous.dispose(), admin.dispose()]);
      }
    });
  });

  test.describe('Account lock (API)', () => {
    test('locks after repeated wrong passwords and a correct password still fails the same way', async ({
      baseURL,
      testData,
    }) => {
      const clientIp = isolatedClientIp();
      const anonymous = await anonymousContext(baseURL!, clientIp);
      try {
        const user = await createDedicatedUser(testData, 'lock');

        await test.step('Fewer wrong passwords than the threshold leave the account usable', async () => {
          for (let attempt = 0; attempt < FAILED_ATTEMPTS_BEFORE_LOCK - 1; attempt += 1) {
            const response = await signIn(anonymous, user.email, WRONG_PASSWORD);
            expect(response.status()).toBe(401);
          }
          const response = await signIn(anonymous, user.email, TEST_PASSWORD);
          expect(response.status()).toBe(200);
        });

        await test.step('A successful sign-in restarts the failure count', async () => {
          for (let attempt = 0; attempt < FAILED_ATTEMPTS_BEFORE_LOCK - 1; attempt += 1) {
            const response = await signIn(anonymous, user.email, WRONG_PASSWORD);
            expect(response.status()).toBe(401);
          }
          const response = await signIn(anonymous, user.email, TEST_PASSWORD);
          expect(response.status()).toBe(200);
        });

        await test.step('Reaching the threshold locks the account', async () => {
          await lockAccount(anonymous, user.email);
        });

        await test.step('A correct password is rejected identically while locked', async () => {
          const response = await signIn(anonymous, user.email, TEST_PASSWORD);
          expect(response.status()).toBe(401);
          expect(await response.json()).toEqual({ error: GENERIC_SIGN_IN_ERROR });
        });
      } finally {
        await anonymous.dispose();
      }
    });

    test('an administrator password reset clears the lock', async ({ baseURL, testData }) => {
      const clientIp = isolatedClientIp();
      const anonymous = await anonymousContext(baseURL!, clientIp);
      const admin = await adminContext(baseURL!, clientIp);
      try {
        const user = await createDedicatedUser(testData, 'reset-lock');

        await test.step('Lock the account', async () => {
          await lockAccount(anonymous, user.email);
          expect((await signIn(anonymous, user.email, TEST_PASSWORD)).status()).toBe(401);
        });

        await test.step('The administrator sets a new password', async () => {
          const response = await admin.put(`/api/v1/users/${user.id}`, { data: { password: NEW_PASSWORD } });
          expect(response.status(), await response.text()).toBe(200);
        });

        await test.step('The user can sign in with the new password', async () => {
          expect((await signIn(anonymous, user.email, NEW_PASSWORD)).status()).toBe(200);
        });
      } finally {
        await Promise.all([anonymous.dispose(), admin.dispose()]);
      }
    });
  });

  test.describe('Changing your own password (API)', () => {
    test('requires the current password', async ({ baseURL, testData }) => {
      const clientIp = isolatedClientIp();
      const user = await createDedicatedUser(testData, 'self-change');
      const session = await signedInContext(baseURL!, clientIp, user.email, TEST_PASSWORD);
      const anonymous = await anonymousContext(baseURL!, clientIp);
      try {
        await test.step('A request without the current password is rejected as incomplete', async () => {
          const response = await session.put(`/api/v1/users/${user.id}`, { data: { password: NEW_PASSWORD } });
          expect(response.status()).toBe(400);
        });

        await test.step('A request with the wrong current password is rejected as unauthorized', async () => {
          const response = await session.put(`/api/v1/users/${user.id}`, {
            data: { password: NEW_PASSWORD, current_password: WRONG_PASSWORD },
          });
          expect(response.status()).toBe(401);
        });

        await test.step('The original password still works and the session is still valid', async () => {
          expect((await session.get('/api/v1/auth/me')).status()).toBe(200);
          expect((await signIn(anonymous, user.email, TEST_PASSWORD)).status()).toBe(200);
          expect((await signIn(anonymous, user.email, NEW_PASSWORD)).status()).toBe(401);
        });
      } finally {
        await Promise.all([session.dispose(), anonymous.dispose()]);
      }
    });

    test('with the current password it succeeds and ends existing sessions', async ({ baseURL, testData }) => {
      const clientIp = isolatedClientIp();
      const user = await createDedicatedUser(testData, 'self-change-ok');
      const session = await signedInContext(baseURL!, clientIp, user.email, TEST_PASSWORD);
      const otherSession = await signedInContext(baseURL!, clientIp, user.email, TEST_PASSWORD);
      const anonymous = await anonymousContext(baseURL!, clientIp);
      try {
        await test.step('Change the password with the current one', async () => {
          const response = await session.put(`/api/v1/users/${user.id}`, {
            data: { password: NEW_PASSWORD, current_password: TEST_PASSWORD },
          });
          expect(response.status(), await response.text()).toBe(200);
        });

        await test.step('Existing sessions are ended', async () => {
          expect((await session.get('/api/v1/auth/me')).status()).toBe(401);
          expect((await otherSession.get('/api/v1/auth/me')).status()).toBe(401);
        });

        await test.step('Only the new password signs in', async () => {
          expect((await signIn(anonymous, user.email, TEST_PASSWORD)).status()).toBe(401);
          expect((await signIn(anonymous, user.email, NEW_PASSWORD)).status()).toBe(200);
        });
      } finally {
        await Promise.all([session.dispose(), otherSession.dispose(), anonymous.dispose()]);
      }
    });

    test('change-password keeps the caller signed in and ends other sessions', async ({ baseURL, testData }) => {
      const clientIp = isolatedClientIp();
      const user = await createDedicatedUser(testData, 'change-endpoint');
      const caller = await signedInContext(baseURL!, clientIp, user.email, TEST_PASSWORD);
      const otherSession = await signedInContext(baseURL!, clientIp, user.email, TEST_PASSWORD);
      const anonymous = await anonymousContext(baseURL!, clientIp);
      try {
        await test.step('Change the password from the caller session', async () => {
          const response = await caller.post('/api/v1/auth/change-password', {
            data: { old_password: TEST_PASSWORD, new_password: NEW_PASSWORD },
          });
          expect(response.status(), await response.text()).toBe(200);
        });

        await test.step('The caller remains signed in', async () => {
          expect((await caller.get('/api/v1/auth/me')).status()).toBe(200);
        });

        await test.step('The other session is ended', async () => {
          expect((await otherSession.get('/api/v1/auth/me')).status()).toBe(401);
        });

        await test.step('Only the new password signs in', async () => {
          expect((await signIn(anonymous, user.email, TEST_PASSWORD)).status()).toBe(401);
          expect((await signIn(anonymous, user.email, NEW_PASSWORD)).status()).toBe(200);
        });
      } finally {
        await Promise.all([caller.dispose(), otherSession.dispose(), anonymous.dispose()]);
      }
    });
  });

  test.describe('Administrator password reset (API)', () => {
    test("setting another user's password needs no current password", async ({ baseURL, testData }) => {
      const clientIp = isolatedClientIp();
      const admin = await adminContext(baseURL!, clientIp);
      const anonymous = await anonymousContext(baseURL!, clientIp);
      try {
        const user = await createDedicatedUser(testData, 'admin-reset');
        const userSession = await signedInContext(baseURL!, clientIp, user.email, TEST_PASSWORD);

        await test.step('The administrator sets a new password without a current one', async () => {
          const response = await admin.put(`/api/v1/users/${user.id}`, { data: { password: NEW_PASSWORD } });
          expect(response.status(), await response.text()).toBe(200);
        });

        await test.step("The user's earlier session ends and only the new password works", async () => {
          expect((await userSession.get('/api/v1/auth/me')).status()).toBe(401);
          expect((await signIn(anonymous, user.email, TEST_PASSWORD)).status()).toBe(401);
          expect((await signIn(anonymous, user.email, NEW_PASSWORD)).status()).toBe(200);
        });

        await userSession.dispose();
      } finally {
        await Promise.all([admin.dispose(), anonymous.dispose()]);
      }
    });
  });

  test.describe('Login page', () => {
    test.use({
      storageState: { cookies: [], origins: [] },
      extraHTTPHeaders: { 'X-Forwarded-For': isolatedClientIp() },
    });

    /** Submit the login form and return the visible error alert's text. */
    async function submitAndReadError(
      page: import('@playwright/test').Page,
      email: string,
      password: string
    ): Promise<string> {
      await page.goto('/login');
      await page.getByRole('textbox', { name: /email/i }).fill(email);
      await page.getByLabel('Password', { exact: true }).fill(password);
      await page.getByRole('button', { name: /sign in/i }).click();

      const alert = page.getByTestId('toast-error');
      await expect(alert).toBeVisible();
      await expect(alert).toHaveRole('alert');
      await expect(page).toHaveURL(/login/);
      return (await alert.innerText()).trim();
    }

    test('shows the same message for every failed sign-in', async ({
      page,
      baseURL,
      testData,
    }) => {
      const clientIp = isolatedClientIp();
      const anonymous = await anonymousContext(baseURL!, clientIp);
      const admin = await adminContext(baseURL!, clientIp);
      try {
        const wrongPasswordUser = await createDedicatedUser(testData, 'ui-wrong');
        const disabledUser = await createDedicatedUser(testData, 'ui-disabled');
        const lockedUser = await createDedicatedUser(testData, 'ui-locked');

        await test.step('Disable one account and lock another', async () => {
          const disable = await admin.put(`/api/v1/users/${disabledUser.id}`, { data: { enabled: false } });
          expect(disable.status(), await disable.text()).toBe(200);
          await lockAccount(anonymous, lockedUser.email);
        });

        const messages: Record<string, string> = {};

        await test.step('Wrong password', async () => {
          messages.wrongPassword = await submitAndReadError(page, wrongPasswordUser.email, WRONG_PASSWORD);
        });
        await test.step('Unknown account', async () => {
          messages.unknown = await submitAndReadError(page, `nobody-${randomUUID()}@test.local`, TEST_PASSWORD);
        });
        await test.step('Disabled account', async () => {
          messages.disabled = await submitAndReadError(page, disabledUser.email, TEST_PASSWORD);
        });
        await test.step('Locked account with the correct password', async () => {
          messages.locked = await submitAndReadError(page, lockedUser.email, TEST_PASSWORD);
        });

        await test.step('All four messages are identical', () => {
          expect(messages.wrongPassword.toLowerCase()).toContain(GENERIC_SIGN_IN_ERROR);
          expect(messages.unknown).toBe(messages.wrongPassword);
          expect(messages.disabled).toBe(messages.wrongPassword);
          expect(messages.locked).toBe(messages.wrongPassword);
        });

        await test.step('The error alert exposes the expected structure', async () => {
          await expect(page.getByTestId('toast-error')).toMatchAriaSnapshot(`
            - alert:
              - text: ✗invalid credentials
              - button "Close": ×
          `);
        });
      } finally {
        await Promise.all([anonymous.dispose(), admin.dispose()]);
      }
    });
  });
});
