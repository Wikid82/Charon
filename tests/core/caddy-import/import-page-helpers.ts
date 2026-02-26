import { expect, test, type Page } from '@playwright/test';
import { loginUser, type TestUser } from '../../fixtures/auth-fixtures';
import { readFileSync } from 'fs';
import { STORAGE_STATE } from '../../constants';

const IMPORT_PAGE_PATH = '/tasks/import/caddyfile';
const SETUP_TEST_EMAIL = process.env.E2E_TEST_EMAIL || 'e2e-test@example.com';
const SETUP_TEST_PASSWORD = process.env.E2E_TEST_PASSWORD || 'TestPassword123!';
const IMPORT_BLOCKING_STATUS_CODES = new Set([401, 403, 302, 429]);
const IMPORT_ERROR_PATTERNS = /(cors|cross-origin|same-origin|cookie|csrf|forbidden|unauthorized|security|host)/i;

type ImportDiagnosticsCleanup = () => void;

function diagnosticLog(message: string): void {
  if (process.env.PLAYWRIGHT_IMPORT_DIAGNOSTICS === '0') {
    return;
  }
  console.log(message);
}

async function readCurrentPath(page: Page): Promise<string> {
  return page.evaluate(() => window.location.pathname).catch(() => '');
}

export async function getImportAuthMarkers(page: Page): Promise<{
  currentUrl: string;
  currentPath: string;
  loginRoute: boolean;
  setupRoute: boolean;
  hasLoginForm: boolean;
  hasSetupForm: boolean;
  hasPendingSessionBanner: boolean;
  hasTextarea: boolean;
}> {
  const currentUrl = page.url();
  const currentPath = await readCurrentPath(page);

  const [hasLoginForm, hasSetupForm, hasPendingSessionBanner, hasTextarea] = await Promise.all([
    page.locator('form').filter({ has: page.getByRole('button', { name: /sign in|login/i }) }).first().isVisible().catch(() => false),
    page.getByRole('button', { name: /create admin|finish setup|setup/i }).first().isVisible().catch(() => false),
    page.getByText(/pending import session/i).first().isVisible().catch(() => false),
    page.locator('textarea').first().isVisible().catch(() => false),
  ]);

  return {
    currentUrl,
    currentPath,
    loginRoute: currentUrl.includes('/login') || currentPath.includes('/login'),
    setupRoute: currentUrl.includes('/setup') || currentPath.includes('/setup'),
    hasLoginForm,
    hasSetupForm,
    hasPendingSessionBanner,
    hasTextarea,
  };
}

export async function assertNoAuthRedirect(page: Page, context: string): Promise<void> {
  const markers = await getImportAuthMarkers(page);
  if (markers.loginRoute || markers.setupRoute || markers.hasLoginForm || markers.hasSetupForm) {
    throw new Error(
      `${context}: blocked by auth/setup state (url=${markers.currentUrl}, path=${markers.currentPath}, ` +
      `loginRoute=${markers.loginRoute}, setupRoute=${markers.setupRoute}, ` +
      `hasLoginForm=${markers.hasLoginForm}, hasSetupForm=${markers.hasSetupForm})`
    );
  }
}

export function attachImportDiagnostics(page: Page, scope: string): ImportDiagnosticsCleanup {
  if (process.env.PLAYWRIGHT_IMPORT_DIAGNOSTICS === '0') {
    return () => {};
  }

  const onResponse = (response: { status: () => number; url: () => string }): void => {
    const status = response.status();
    if (!IMPORT_BLOCKING_STATUS_CODES.has(status)) {
      return;
    }

    const url = response.url();
    if (!/\/api\/v1\/(auth|import)|\/login|\/setup/i.test(url)) {
      return;
    }

    diagnosticLog(`[Diag:${scope}] blocking-status=${status} url=${url}`);
  };

  const onConsole = (msg: { type: () => string; text: () => string }): void => {
    const text = msg.text();
    if (!IMPORT_ERROR_PATTERNS.test(text)) {
      return;
    }

    diagnosticLog(`[Diag:${scope}] console.${msg.type()} ${text}`);
  };

  const onPageError = (error: Error): void => {
    const text = error.message || String(error);
    if (!IMPORT_ERROR_PATTERNS.test(text)) {
      return;
    }

    diagnosticLog(`[Diag:${scope}] pageerror ${text}`);
  };

  page.on('response', onResponse);
  page.on('console', onConsole);
  page.on('pageerror', onPageError);

  return () => {
    page.off('response', onResponse);
    page.off('console', onConsole);
    page.off('pageerror', onPageError);
  };
}

export async function logImportFailureContext(page: Page, scope: string): Promise<void> {
  const markers = await getImportAuthMarkers(page);
  diagnosticLog(
    `[Diag:${scope}] failure-context url=${markers.currentUrl} path=${markers.currentPath} ` +
    `loginRoute=${markers.loginRoute} setupRoute=${markers.setupRoute} ` +
    `hasLoginForm=${markers.hasLoginForm} hasSetupForm=${markers.hasSetupForm} ` +
    `hasPendingSessionBanner=${markers.hasPendingSessionBanner} hasTextarea=${markers.hasTextarea}`
  );
}

export async function waitForSuccessfulImportResponse(
  page: Page,
  triggerAction: () => Promise<void>,
  scope: string,
  expectedPath: RegExp = /\/api\/v1\/import\/(upload|upload-multi)/i
): Promise<import('@playwright/test').Response> {
  await assertNoAuthRedirect(page, `${scope} pre-trigger`);

  try {
    const [response] = await Promise.all([
      page.waitForResponse((r) => expectedPath.test(r.url()) && r.ok(), { timeout: 15000 }),
      triggerAction(),
    ]);
    return response;
  } catch (error) {
    await logImportFailureContext(page, scope);
    throw error;
  }
}

function extractTokenFromState(rawState: unknown): string | null {
  if (!rawState || typeof rawState !== 'object') {
    return null;
  }

  const state = rawState as { origins?: Array<{ localStorage?: Array<{ name?: string; value?: string }> }> };
  const origins = Array.isArray(state.origins) ? state.origins : [];
  for (const origin of origins) {
    const entries = Array.isArray(origin.localStorage) ? origin.localStorage : [];
    const tokenEntry = entries.find((item) => item?.name === 'charon_auth_token' && typeof item.value === 'string');
    if (tokenEntry?.value) {
      return tokenEntry.value;
    }
  }

  return null;
}

async function restoreAuthFromStorageState(page: Page): Promise<boolean> {
  try {
    const state = JSON.parse(readFileSync(STORAGE_STATE, 'utf-8')) as {
      cookies?: Array<{
        name: string;
        value: string;
        domain?: string;
        path?: string;
        expires?: number;
        httpOnly?: boolean;
        secure?: boolean;
        sameSite?: 'Lax' | 'None' | 'Strict';
      }>;
    };
    const token = extractTokenFromState(state);
    const cookies = Array.isArray(state.cookies) ? state.cookies : [];

    if (!token && cookies.length === 0) {
      return false;
    }

    if (cookies.length > 0) {
      await page.context().addCookies(cookies);
    }

    if (token) {
      await page.goto('/', { waitUntil: 'domcontentloaded' });
      await page.evaluate((authToken: string) => {
        localStorage.setItem('charon_auth_token', authToken);
      }, token);
      await page.reload({ waitUntil: 'domcontentloaded' });
      await page.waitForLoadState('networkidle').catch(() => {});
    }

    return true;
  } catch {
    return false;
  }
}

async function loginWithSetupCredentials(page: Page): Promise<void> {
  if (!page.url().includes('/login')) {
    await page.goto('/login', { waitUntil: 'domcontentloaded' });
  }

  await page.locator('input[type="email"]').first().fill(SETUP_TEST_EMAIL);
  await page.locator('input[type="password"]').first().fill(SETUP_TEST_PASSWORD);

  const [loginResponse] = await Promise.all([
    page.waitForResponse((response) => response.url().includes('/api/v1/auth/login'), { timeout: 15000 }),
    page.getByRole('button', { name: /sign in|login/i }).first().click(),
  ]);

  if (!loginResponse.ok()) {
    const body = await loginResponse.text().catch(() => '');
    throw new Error(`Setup-credential login failed: ${loginResponse.status()} ${body}`);
  }

  const payload = (await loginResponse.json().catch(() => ({}))) as { token?: string };
  if (payload.token) {
    await page.evaluate((authToken: string) => {
      localStorage.setItem('charon_auth_token', authToken);
    }, payload.token);
  }

  await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 15000 });
  await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
}

export async function resetImportSession(page: Page): Promise<void> {
  try {
    if (!page.url().includes(IMPORT_PAGE_PATH)) {
      await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
    }
  } catch {
    // Best-effort navigation only
  }

  try {
    const statusResponse = await page.request.get('/api/v1/import/status');
    if (statusResponse.ok()) {
      const statusBody = await statusResponse.json();
      if (statusBody?.has_pending) {
        await page.request.post('/api/v1/import/cancel');
      }
    }
  } catch {
    // Best-effort cleanup only
  }

  try {
    await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
  } catch {
    // Best-effort return to import page only
  }
}

export async function ensureImportFormReady(page: Page): Promise<void> {
  await assertNoAuthRedirect(page, 'ensureImportFormReady initial check');

  const headingByRole = page.getByRole('heading', { name: /import|caddyfile/i }).first();
  const headingLike = page
    .locator('h1, h2, [data-testid="page-title"], [aria-label*="import" i], [aria-label*="caddyfile" i]')
    .first();

  if (await headingByRole.count()) {
    await expect(headingByRole).toBeVisible();
  } else if (await headingLike.count()) {
    await expect(headingLike).toBeVisible();
  } else {
    await expect(page.locator('main, body').first()).toContainText(/import|caddyfile/i);
  }

  const textarea = page.locator('textarea').first();
  const textareaVisible = await textarea.isVisible().catch(() => false);
  if (!textareaVisible) {
    const pendingSessionVisible = await page.getByText(/pending import session/i).first().isVisible().catch(() => false);
    if (pendingSessionVisible) {
      diagnosticLog('[Diag:import-ready] pending import session detected, canceling to restore textarea');

      const browserCancelStatus = await page
        .evaluate(async () => {
          const token = localStorage.getItem('charon_auth_token');
          const commonHeaders = token ? { Authorization: `Bearer ${token}` } : {};

          const statusResponse = await fetch('/api/v1/import/status', {
            method: 'GET',
            credentials: 'include',
            headers: commonHeaders,
          });
          let sessionId = '';
          if (statusResponse.ok) {
            const statusBody = (await statusResponse.json()) as { session?: { id?: string } };
            sessionId = statusBody?.session?.id || '';
          }

          const cancelUrl = sessionId
            ? `/api/v1/import/cancel?session_uuid=${encodeURIComponent(sessionId)}`
            : '/api/v1/import/cancel';

          const response = await fetch(cancelUrl, {
            method: 'DELETE',
            credentials: 'include',
            headers: commonHeaders,
          });
          return response.status;
        })
        .catch(() => null);
      diagnosticLog(`[Diag:import-ready] browser cancel status=${browserCancelStatus ?? 'n/a'}`);

      const cancelButton = page.getByRole('button', { name: /^cancel$/i }).first();
      const cancelButtonVisible = await cancelButton.isVisible().catch(() => false);

      if (cancelButtonVisible) {
        await Promise.all([
          page.waitForResponse((response) => response.url().includes('/api/v1/import/cancel'), { timeout: 10000 }).catch(() => null),
          cancelButton.click(),
        ]);
      }

      await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
      await assertNoAuthRedirect(page, 'ensureImportFormReady after pending-session reset');
    }
  }

  await expect(textarea).toBeVisible();
  await expect(page.getByRole('button', { name: /parse|review/i }).first()).toBeVisible();
}

async function hasLoginUiMarkers(page: Page): Promise<boolean> {
  const currentUrl = page.url();
  const currentPath = await readCurrentPath(page);
  if (currentUrl.includes('/login') || currentPath.includes('/login')) {
    return true;
  }

  const signInHeading = page.getByRole('heading', { name: /sign in|login/i }).first();
  const signInButton = page.getByRole('button', { name: /sign in|login/i }).first();
  const emailTextbox = page.getByRole('textbox', { name: /email/i }).first();

  const [headingVisible, buttonVisible, emailVisible] = await Promise.all([
    signInHeading.isVisible().catch(() => false),
    signInButton.isVisible().catch(() => false),
    emailTextbox.isVisible().catch(() => false),
  ]);

  return headingVisible || buttonVisible || emailVisible;
}

export async function ensureAuthenticatedImportFormReady(page: Page, adminUser?: TestUser): Promise<void> {
  const recoverIfNeeded = async (): Promise<boolean> => {
    const loginDetected = await test.step('Auth precheck: detect login redirect or sign-in controls', async () => {
      return hasLoginUiMarkers(page);
    });
    if (!loginDetected) {
      return false;
    }

    if (!adminUser) {
      throw new Error('Import auth recovery failed: login UI detected but no admin user fixture was provided.');
    }

    return test.step('Auth recovery: perform one deterministic login and return to import page', async () => {
      try {
        await loginUser(page, adminUser);
        await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });

        if (await hasLoginUiMarkers(page) && adminUser.token) {
          await test.step('Auth recovery fallback: restore fixture token and reload import page', async () => {
            await page.goto('/', { waitUntil: 'domcontentloaded' });
            await page.evaluate((token: string) => {
              localStorage.setItem('charon_auth_token', token);
            }, adminUser.token);
            await page.reload({ waitUntil: 'domcontentloaded' });
            await page.waitForLoadState('networkidle').catch(() => {});
            await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
          });
        }

        if (await hasLoginUiMarkers(page)) {
          await test.step('Auth recovery fallback: restore auth from setup storage state', async () => {
            const restored = await restoreAuthFromStorageState(page);
            if (!restored) {
              throw new Error(`Unable to restore auth from ${STORAGE_STATE}`);
            }
            await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
          });
        }

        if (await hasLoginUiMarkers(page)) {
          await test.step('Auth recovery fallback: UI login with setup credentials', async () => {
            await loginWithSetupCredentials(page);
          });
        }

        await ensureImportFormReady(page);
        return true;
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        throw new Error(`Import auth recovery failed after one re-auth attempt: ${message}`);
      }
    });
  };

  if (await recoverIfNeeded()) {
    return;
  }

  try {
    await ensureImportFormReady(page);
  } catch (error) {
    if (await recoverIfNeeded()) {
      return;
    }

    throw error;
  }
}

export async function ensureImportUiPreconditions(page: Page, adminUser?: TestUser): Promise<void> {
  await test.step('Precondition: open Caddy import page', async () => {
    await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
  });

  await ensureAuthenticatedImportFormReady(page, adminUser);

  await test.step('Precondition: verify import textarea is visible', async () => {
    await expect(page.locator('textarea')).toBeVisible();
  });
}
