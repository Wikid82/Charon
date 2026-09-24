/**
 * Authentication Rate Limiting E2E Tests
 *
 * Covers per-client throttling of sign-in attempts and how it is surfaced:
 * - A client that exhausts its sign-in budget receives a 429 with Retry-After
 * - The throttle is keyed on the real client address behind a trusted proxy,
 *   so one throttled client does not affect others
 * - Session reads stay available while sign-in is throttled
 * - The login page explains the wait and links administrators to the docs
 * - The admin login-protection card on the Security page shows proxy guidance
 *
 * The API tests (throttling, keying, session reads) first run an
 * effectiveness probe against the admin login-protection endpoint and skip
 * when the stack under test cannot isolate clients by forwarded address.
 * The UI tests stub the API responses and always run.
 *
 * @see https://github.com/Wikid82/Charon/issues/1317
 * @see docs/features/login-protection.md
 */

import { randomInt, randomUUID } from 'crypto';
import {
  request as playwrightRequest,
  type APIRequestContext,
  type APIResponse,
} from '@playwright/test';
import { test, expect } from '../fixtures/auth-fixtures';
import { STORAGE_STATE } from '../constants';

const LOGIN_PATH = '/api/v1/auth/login';
const LOGIN_PROTECTION_PATH = '/api/v1/security/login-protection';
const GENERIC_THROTTLE_ERROR = 'Too many requests. Please wait before trying again.';
const LOGIN_BATCH_SIZE = 25;
const LOGIN_ATTEMPT_CAP = 2000;
const PLAYWRIGHT_COMPOSE_FILES =
  '.docker/compose/docker-compose.playwright-local.yml / docker-compose.playwright-ci.yml';

/** Shape of GET /api/v1/security/login-protection. */
interface LoginProtectionStatus {
  enabled: boolean;
  login: { requests: number; window_seconds: number };
  session: { requests: number; window_seconds: number };
  trusted_proxy_count: number;
  caller_client_key: string;
  caller_client_scope: string;
  untrusted_forwarded_headers: {
    local: UntrustedForwardedRecord;
    public: UntrustedForwardedRecord;
  };
}

interface UntrustedForwardedRecord {
  count: number;
  last_seen: string | null;
  last_peer: string;
  last_peer_scope: string;
}

interface ExhaustResult {
  /** Statuses of every response received before (and including) the first 429. */
  statuses: number[];
  /** The first 429 response, or null if the cap was reached without one. */
  throttled: APIResponse | null;
}

/**
 * A random address in 198.18.0.0/15 (RFC 2544 benchmarking range), so each
 * test gets its own throttle bucket that no real client can share.
 */
function isolatedClientIp(): string {
  return `198.${randomInt(18, 20)}.${randomInt(0, 256)}.${randomInt(1, 255)}`;
}

/** A request context with no session, optionally presenting a forwarded client address. */
async function anonymousContext(baseURL: string, clientIp?: string): Promise<APIRequestContext> {
  return playwrightRequest.newContext({
    baseURL,
    storageState: { cookies: [], origins: [] },
    extraHTTPHeaders: {
      Accept: 'application/json',
      ...(clientIp ? { 'X-Forwarded-For': clientIp } : {}),
    },
  });
}

/** A request context carrying the admin session cookie from auth.setup.ts, optionally with a forwarded address. */
async function adminContext(baseURL: string, clientIp?: string): Promise<APIRequestContext> {
  return playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
    extraHTTPHeaders: {
      Accept: 'application/json',
      ...(clientIp ? { 'X-Forwarded-For': clientIp } : {}),
    },
  });
}

/** POST a sign-in for a unique account that does not exist, so no real account is touched. */
function postProbeLogin(ctx: APIRequestContext): Promise<APIResponse> {
  return ctx.post(LOGIN_PATH, {
    data: { email: `probe-${randomUUID()}@test.local`, password: 'not-a-real-password' },
  });
}

/**
 * Send concurrent batches of probe sign-ins until the first 429, capped at
 * LOGIN_ATTEMPT_CAP attempts.
 */
async function exhaustLoginBudget(ctx: APIRequestContext): Promise<ExhaustResult> {
  const statuses: number[] = [];
  for (let sent = 0; sent < LOGIN_ATTEMPT_CAP; sent += LOGIN_BATCH_SIZE) {
    const batch = await Promise.all(
      Array.from({ length: LOGIN_BATCH_SIZE }, () => postProbeLogin(ctx))
    );
    const throttled = batch.find((response) => response.status() === 429) ?? null;
    statuses.push(...batch.map((response) => response.status()));
    if (throttled) {
      return { statuses, throttled };
    }
  }
  return { statuses, throttled: null };
}

/**
 * Effectiveness probe: skip unless the stack under test has login protection
 * on, a fast enough refill for E2E, and honors X-Forwarded-For end to end.
 */
async function requireIsolatableStack(baseURL: string): Promise<void> {
  const probeIp = isolatedClientIp();
  const ctx = await adminContext(baseURL, probeIp);
  try {
    const response = await ctx.get(LOGIN_PROTECTION_PATH);
    const status = response.ok() ? ((await response.json()) as LoginProtectionStatus) : null;
    const isolatable =
      status !== null &&
      status.enabled &&
      status.login.window_seconds > 0 &&
      status.login.requests / status.login.window_seconds >= 1 &&
      status.caller_client_key === probeIp;

    test.skip(
      !isolatable,
      `Login protection is not isolatable on this stack (GET ${LOGIN_PROTECTION_PATH} → ` +
        `${response.status()}). Configure the E2E auth budgets and trusted proxies in ${PLAYWRIGHT_COMPOSE_FILES}.`
    );
  } finally {
    await ctx.dispose();
  }
}

/** A login-protection payload for stubbing the admin card. */
function loginProtectionStub(
  overrides: Partial<LoginProtectionStatus['untrusted_forwarded_headers']> = {}
): LoginProtectionStatus {
  const empty: UntrustedForwardedRecord = { count: 0, last_seen: null, last_peer: '', last_peer_scope: '' };
  return {
    enabled: true,
    login: { requests: 10, window_seconds: 600 },
    session: { requests: 60, window_seconds: 60 },
    trusted_proxy_count: 1,
    caller_client_key: '203.0.113.7',
    caller_client_scope: 'public',
    untrusted_forwarded_headers: { local: empty, public: empty, ...overrides },
  };
}

test.describe('Authentication rate limiting', () => {
  test.describe('Sign-in throttle (API)', () => {
    test.beforeEach(async ({ baseURL }) => {
      await requireIsolatableStack(baseURL!);
    });

    test.fixme('throttles a client that exhausts its sign-in budget', async ({ baseURL }) => {
      const ipA = isolatedClientIp();
      const ctx = await anonymousContext(baseURL!, ipA);
      try {
        const { statuses, throttled } = await test.step('Exhaust the sign-in budget', () =>
          exhaustLoginBudget(ctx)
        );

        await test.step('Every attempt before the throttle is a normal rejection', async () => {
          expect(throttled, `no 429 within ${LOGIN_ATTEMPT_CAP} attempts`).not.toBeNull();
          expect(statuses.filter((status) => status !== 429).every((status) => status === 401)).toBe(true);
        });

        await test.step('The throttled response carries Retry-After and a generic body', async () => {
          const retryAfter = throttled!.headers()['retry-after'];
          expect(retryAfter).toMatch(/^\d+$/);
          expect(Number(retryAfter)).toBeGreaterThanOrEqual(1);

          const bodyText = await throttled!.text();
          expect(JSON.parse(bodyText)).toEqual({ error: GENERIC_THROTTLE_ERROR });
          expect(bodyText).not.toContain('probe-');
        });
      } finally {
        await ctx.dispose();
      }
    });

    test.fixme('keys the throttle on the real client behind a trusted proxy', async ({ baseURL }) => {
      const ipA = isolatedClientIp();
      const ipB = isolatedClientIp();
      const ctxA = await anonymousContext(baseURL!, ipA);
      const ctxB = await anonymousContext(baseURL!, ipB);
      const runner = await anonymousContext(baseURL!);
      try {
        await test.step('Exhaust the sign-in budget for client A', async () => {
          const { throttled } = await exhaustLoginBudget(ctxA);
          expect(throttled, `no 429 within ${LOGIN_ATTEMPT_CAP} attempts`).not.toBeNull();
        });

        await test.step('Client B still gets a normal rejection', async () => {
          expect((await postProbeLogin(ctxB)).status()).toBe(401);
        });

        await test.step('The runner without a forwarded address is unaffected', async () => {
          expect((await runner.get('/api/v1/auth/status')).status()).toBe(200);
          expect((await postProbeLogin(runner)).status()).toBe(401);
        });
      } finally {
        await Promise.all([ctxA.dispose(), ctxB.dispose(), runner.dispose()]);
      }
    });

    test.fixme('keeps session reads available while sign-in is throttled', async ({ baseURL }) => {
      const ipA = isolatedClientIp();
      const anonymous = await anonymousContext(baseURL!, ipA);
      const admin = await adminContext(baseURL!, ipA);
      try {
        await test.step('Exhaust the sign-in budget for client A', async () => {
          const { throttled } = await exhaustLoginBudget(anonymous);
          expect(throttled, `no 429 within ${LOGIN_ATTEMPT_CAP} attempts`).not.toBeNull();
        });

        await test.step('An authenticated session from client A can still read its state', async () => {
          expect((await admin.get('/api/v1/auth/me')).status()).toBe(200);
          expect((await admin.get('/api/v1/auth/status')).status()).toBe(200);
        });
      } finally {
        await Promise.all([anonymous.dispose(), admin.dispose()]);
      }
    });
  });

  test.describe('Login page notice', () => {
    test.use({ storageState: { cookies: [], origins: [] } });

    test.fixme('login page explains the wait and points administrators to the docs', async ({ page }) => {
      await test.step('Stub a throttled sign-in response', async () => {
        await page.route(`**${LOGIN_PATH}`, (route) =>
          route.fulfill({
            status: 429,
            headers: { 'Retry-After': '42' },
            contentType: 'application/json',
            body: JSON.stringify({ error: GENERIC_THROTTLE_ERROR }),
          })
        );
      });

      await test.step('Submit the login form', async () => {
        await page.goto('/login');
        await page.getByRole('textbox', { name: /email/i }).fill('someone@test.local');
        await page.getByLabel('Password', { exact: true }).fill('not-a-real-password');
        await page.getByRole('button', { name: /sign in/i }).click();
      });

      await test.step('The notice shows the wait and a docs link for administrators', async () => {
        const notice = page.getByTestId('login-rate-limit-notice');
        await expect(notice).toHaveRole('alert');
        await expect(notice).toContainText(/wait 42 seconds/i);
        await expect(notice).toContainText(/are you the administrator\?/i);

        const docsLink = notice.getByRole('link', { name: /how login protection works behind a proxy/i });
        await expect(docsLink).toHaveAttribute('href', /\/configuration\/trusted-proxies/);
        await expect(docsLink).toHaveAttribute('target', '_blank');
        await expect(docsLink).toHaveAttribute('rel', /noopener/);

        await expect(notice).toMatchAriaSnapshot(`
          - alert:
            - link /How login protection works behind a proxy/
        `);
      });
    });
  });

  test.describe('Admin login-protection card', () => {
    const loginProtectionCard = (page: import('@playwright/test').Page) =>
      page.getByRole('region', { name: /login protection/i });

    test.fixme('admin card suggests trusting a private proxy that sends forwarded headers', async ({ page }) => {
      await test.step('Stub a recent forwarded-header observation from a private peer', async () => {
        const body = loginProtectionStub({
          local: {
            count: 3,
            last_seen: new Date().toISOString(),
            last_peer: '172.18.0.5',
            last_peer_scope: 'private',
          },
        });
        await page.route(`**${LOGIN_PROTECTION_PATH}`, (route) =>
          route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
        );
      });

      await test.step('Open the Security page', async () => {
        await page.goto('/security');
      });

      await test.step('The card suggests trusting the proxy address', async () => {
        const card = loginProtectionCard(page);
        await expect(card).toContainText('172.18.0.5');
        await expect(card).toContainText('172.18.0.5/32');
        await expect(card).toContainText('CHARON_TRUSTED_PROXIES');
        await expect(card.getByRole('link', { name: /docs|documentation|learn more/i })).toHaveAttribute(
          'href',
          /wikid82\.github\.io\/Charon\/docs/
        );
      });
    });

    test.fixme('admin card never suggests trusting a public peer', async ({ page }) => {
      await test.step('Stub a recent forwarded-header observation from a public peer', async () => {
        const body = loginProtectionStub({
          public: {
            count: 2,
            last_seen: new Date().toISOString(),
            last_peer: '203.0.113.9',
            last_peer_scope: 'public',
          },
        });
        await page.route(`**${LOGIN_PROTECTION_PATH}`, (route) =>
          route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
        );
      });

      await test.step('Open the Security page', async () => {
        await page.goto('/security');
      });

      await test.step('The card shows an informational note without a trust suggestion', async () => {
        const card = loginProtectionCard(page);
        await expect(card).toContainText('203.0.113.9');
        await expect(card).toContainText(/no action is needed/i);
        await expect(card).not.toContainText('CHARON_TRUSTED_PROXIES');
        await expect(card).not.toContainText('/32');
      });
    });
  });
});
