/**
 * Auth Fixtures - Per-test user creation with role-based authentication
 *
 * This module extends the base Playwright test with fixtures for:
 * - TestDataManager with automatic cleanup
 * - Per-test user creation (admin, regular, guest roles)
 * - Isolated authentication state per test
 * - Automatic JWT token refresh for long-running sessions (60+ minutes)
 *
 * @example
 * ```typescript
 * import { test, expect } from './fixtures/auth-fixtures';
 *
 * test('admin can access settings', async ({ page, adminUser }) => {
 *   await page.goto('/login');
 *   await page.locator('input[type="email"]').fill(adminUser.email);
 *   await page.locator('input[type="password"]').fill('TestPass123!');
 *   await page.getByRole('button', { name: /sign in/i }).click();
 *   await page.waitForURL('/');
 *   await page.goto('/settings');
 *   await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
 * });
 * ```
 */

import { test as base, expect } from './test';
import { request as playwrightRequest } from '@playwright/test';
import { existsSync, readFileSync } from 'fs';
import { TestDataManager } from '../utils/TestDataManager';
import { STORAGE_STATE } from '../constants';

/**
 * Represents a test user with authentication details
 */
export interface TestUser {
  /** User ID in the database */
  id: string;
  /** User's email address (namespaced) */
  email: string;
  /** Authentication token for API calls */
  token: string;
  /** User's role */
  role: 'admin' | 'user' | 'guest';
}

/**
 * Token cache with TTL tracking for long-running test sessions
 */
interface TokenCache {
  token: string;
  expiresAt: number; // Unix timestamp (ms)
}

/**
 * Custom fixtures for authentication tests
 */
interface AuthFixtures {
  /** Default authenticated user (admin role) */
  authenticatedUser: TestUser;
  /** Explicit admin user fixture */
  adminUser: TestUser;
  /** Regular user (non-admin) */
  regularUser: TestUser;
  /** Guest user (read-only) */
  guestUser: TestUser;
  /** Test data manager with automatic cleanup */
  testData: TestDataManager;
}

/**
 * Default password used for test users
 * Strong password that meets typical validation requirements
 */
const TEST_PASSWORD = 'TestPass123!';

/**
 * Token cache configuration
 */
let tokenCache: TokenCache | null = null;
let tokenCacheQueue: Promise<void> = Promise.resolve();
const TOKEN_REFRESH_THRESHOLD = 5 * 60 * 1000; // Refresh 5 min before expiry

function readAuthTokenFromStorageState(storageStatePath: string): string | null {
  try {
    const savedState = JSON.parse(readFileSync(storageStatePath, 'utf-8'));
    const origins = Array.isArray(savedState.origins) ? savedState.origins : [];

    const extractToken = (value: unknown): string | null => {
      if (typeof value !== 'string' || !value.trim()) {
        return null;
      }

      if (value.startsWith('{')) {
        try {
          const parsed = JSON.parse(value) as { token?: string };
          if (typeof parsed?.token === 'string' && parsed.token.trim()) {
            return parsed.token;
          }
        } catch {
          return null;
        }
      }

      return value;
    };

    for (const originEntry of origins) {
      const localStorageEntries = Array.isArray(originEntry?.localStorage)
        ? originEntry.localStorage
        : [];

      for (const key of ['charon_auth_token', 'token', 'auth']) {
        const tokenEntry = localStorageEntries.find(
          (entry: { name?: string; value?: string }) => entry?.name === key
        );
        const token = extractToken(tokenEntry?.value);
        if (token) {
          return token;
        }
      }
    }

    const cookies = Array.isArray(savedState.cookies) ? savedState.cookies : [];
    const authCookie = cookies.find((cookie: { name?: string; value?: string }) => cookie?.name === 'auth_token');
    const cookieToken = extractToken(authCookie?.value);
    if (cookieToken) {
      return cookieToken;
    }
  } catch {
  }

  return null;
}

/**
 * Test-only helper to reset token refresh state between tests
 */
export function resetTokenRefreshStateForTests(): void {
  tokenCache = null;
  tokenCacheQueue = Promise.resolve();
}

/**
 * Execute token cache operations sequentially to avoid refresh storms
 */
async function withTokenCacheLock<T>(operation: () => Promise<T>): Promise<T> {
  const previous = tokenCacheQueue;
  let releaseLock!: () => void;
  tokenCacheQueue = new Promise<void>((resolve) => {
    releaseLock = resolve;
  });

  await previous;
  try {
    return await operation();
  } finally {
    releaseLock();
  }
}

/**
 * Read token from in-memory cache
 */
async function readTokenCache(): Promise<TokenCache | null> {
  return tokenCache;
}

/**
 * Write token to in-memory cache
 */
async function saveTokenCache(token: string, expirySeconds: number): Promise<void> {
  tokenCache = {
    token,
    expiresAt: Date.now() + expirySeconds * 1000,
  };
}

/**
 * Extract expiry seconds from JWT token
 * JWT format: header.payload.signature (payload is base64-encoded JSON)
 */
function extractJWTExpiry(token: string): number {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) {
      console.warn('Invalid JWT format: expected 3 parts, got', parts.length);
      return 3600; // Default to 1 hour
    }

    // Add padding if needed
    let payload = parts[1];
    const padding = 4 - (payload.length % 4);
    if (padding < 4) {
      payload += '='.repeat(padding);
    }

    const decoded = JSON.parse(Buffer.from(payload, 'base64').toString());
    if (decoded.exp) {
      // exp is in seconds, convert to seconds remaining
      const now = Math.floor(Date.now() / 1000);
      const remaining = Math.max(0, decoded.exp - now);
      return remaining;
    }
  } catch (e) {
    console.warn('Failed to extract JWT expiry:', e);
  }
  return 3600; // Default to 1 hour
}

/**
 * Check if cached token is expired (considering refresh threshold)
 */
async function isTokenExpired(): Promise<boolean> {
  const cache = await readTokenCache();
  if (!cache) return true;

  // Refresh if within threshold (5 min before actual expiry)
  return Date.now() >= cache.expiresAt - TOKEN_REFRESH_THRESHOLD;
}

/**
 * Refresh token if expired (for long-running test sessions)
 * Supports the /api/v1/auth/refresh endpoint
 */
export async function refreshTokenIfNeeded(
  baseURL: string | undefined,
  currentToken: string
): Promise<string> {
  if (!baseURL) {
    console.warn('baseURL not provided, skipping token refresh');
    return currentToken;
  }

  return withTokenCacheLock(async () => {
    // Check if cached token is still valid
    if (!(await isTokenExpired())) {
      const cache = await readTokenCache();
      if (cache) {
        return cache.token;
      }
    }

    // Token expired or missing - refresh it
    try {
      const response = await fetch(`${baseURL}/api/v1/auth/refresh`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${currentToken}`,
        },
        body: JSON.stringify({}),
      });

      if (!response.ok) {
        console.warn(
          `Token refresh failed: ${response.status} ${response.statusText}`
        );
        return currentToken; // Fall back to current token
      }

      const data = (await response.json()) as { token?: string };
      const newToken = data.token;

      if (!newToken) {
        console.warn('Token refresh response missing token field');
        return currentToken;
      }

      // Extract expiry from JWT and cache new token
      const expirySeconds = extractJWTExpiry(newToken);
      await saveTokenCache(newToken, expirySeconds);

      return newToken;
    } catch (error) {
      console.warn('Token refresh error:', error);
      return currentToken; // Fall back to current token
    }
  });
}

/**
 * Extended Playwright test with authentication fixtures
 */
export const test = base.extend<AuthFixtures>({
  /**
   * TestDataManager fixture with automatic cleanup
   *
   * FIXED: Now creates an authenticated API context using stored auth state.
   * This ensures API calls (like createUser, deleteUser) inherit the admin
   * session established by auth.setup.ts.
   *
   * Previous Issue: The base `request` fixture was unauthenticated, causing
   * "Admin access required" errors on protected endpoints.
   */
  testData: async ({ baseURL }, use, testInfo) => {
    // Defensive check: Verify auth state file exists (created by auth.setup.ts)
    if (!existsSync(STORAGE_STATE)) {
      throw new Error(
        `Auth state file not found at ${STORAGE_STATE}. ` +
          'Ensure auth.setup has run first. Check that dependencies: ["setup"] is configured.'
      );
    }

    const savedState = JSON.parse(readFileSync(STORAGE_STATE, 'utf-8'));
    const authToken = readAuthTokenFromStorageState(STORAGE_STATE);

    // Validate cookie domain matches baseURL to catch configuration issues early
    try {
      const cookies = savedState.cookies || [];
      const authCookie = cookies.find((c: { name: string }) => c.name === 'auth_token');

      if (authCookie?.domain && baseURL) {
        const expectedHost = new URL(baseURL).hostname;
        const cookieDomain = authCookie.domain.replace(/^\./, ''); // Remove leading dot

        if (cookieDomain !== expectedHost) {
          console.warn(
            `⚠️ TestDataManager: Cookie domain mismatch detected!\n` +
            `   Cookie domain: "${authCookie.domain}"\n` +
            `   Base URL host: "${expectedHost}"\n` +
            `   API calls will likely fail with 401/403.\n` +
            `   Fix: Set PLAYWRIGHT_BASE_URL to the same host used by Playwright baseURL.`
          );
        }
      }
    } catch (err) {
      console.warn('⚠️ Could not validate cookie domain:', err instanceof Error ? err.message : err);
    }

    // Create an authenticated API request context using stored auth state
    // This inherits the admin session from auth.setup.ts
    const authenticatedContext = await playwrightRequest.newContext({
      baseURL,
      storageState: STORAGE_STATE,
      extraHTTPHeaders: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
      },
    });

    const manager = new TestDataManager(authenticatedContext, testInfo.title, authToken ?? undefined);

    try {
      await use(manager);
    } finally {
      // Ensure cleanup runs even if test fails
      await manager.cleanup();
      // Dispose the API context to release resources
      await authenticatedContext.dispose();
    }
  },

  /**
   * Default authenticated user (admin role)
   * Use this for tests that need a logged-in admin user
   */
  authenticatedUser: async ({ testData }, use) => {
    const user = await testData.createUser({
      name: `Test Admin ${Date.now()}`,
      email: `admin-${Date.now()}@test.local`,
      password: TEST_PASSWORD,
      role: 'admin',
    });
    await use({
      ...user,
      role: 'admin',
    });
  },

  /**
   * Explicit admin user fixture
   * Same as authenticatedUser but with explicit naming for clarity
   */
  adminUser: async ({ testData }, use) => {
    const user = await testData.createUser({
      name: `Test Admin ${Date.now()}`,
      email: `admin-${Date.now()}@test.local`,
      password: TEST_PASSWORD,
      role: 'admin',
    });
    await use({
      ...user,
      role: 'admin',
    });
  },

  /**
   * Regular user (non-admin) fixture
   * Use for testing permission restrictions
   */
  regularUser: async ({ testData }, use) => {
    const user = await testData.createUser({
      name: `Test User ${Date.now()}`,
      email: `user-${Date.now()}@test.local`,
      password: TEST_PASSWORD,
      role: 'user',
    });
    await use({
      ...user,
      role: 'user',
    });
  },

  /**
   * Guest user (restricted access) fixture — using 'passthrough' role
   * (the 'guest' role was removed in PR-3; 'passthrough' is the equivalent
   * lowest-privilege role in the Admin / User / Passthrough model)
   */
  guestUser: async ({ testData }, use) => {
    const user = await testData.createUser({
      name: `Test Guest ${Date.now()}`,
      email: `guest-${Date.now()}@test.local`,
      password: TEST_PASSWORD,
      role: 'passthrough',
    });
    await use({
      ...user,
      role: 'passthrough',
    });
  },
});

/**
 * Helper function to log in a user via the UI
 * @param page - Playwright Page instance
 * @param user - Test user to log in
 */
export async function loginUser(
  page: import('@playwright/test').Page,
  user: TestUser
): Promise<void> {
  const hasVisibleSignInControls = async (): Promise<boolean> => {
    const signInButtonVisible = await page.getByRole('button', { name: /sign in|login/i }).first().isVisible().catch(() => false);
    const emailInputVisible = await page.getByRole('textbox', { name: /email/i }).first().isVisible().catch(() => false);
    return signInButtonVisible || emailInputVisible;
  };

  const loginPayload = { email: user.email, password: TEST_PASSWORD };
  let apiLoginError: Error | null = null;
  try {
    const response = await page.request.post('/api/v1/auth/login', { data: loginPayload });
    if (response.ok()) {
      const body = await response.json().catch(() => ({})) as { token?: string };
      if (body.token) {
        // Navigate first, then set token via evaluate to avoid addInitScript race condition
        await page.goto('/');
        await page.evaluate((token: string) => {
          localStorage.setItem('charon_auth_token', token);
        }, body.token);

        const storageState = await page.request.storageState();
        if (storageState.cookies?.length) {
          await page.context().addCookies(storageState.cookies);
        }

        // Reload so the app picks up the token from localStorage
        await page.reload({ waitUntil: 'domcontentloaded' });
        await page.waitForLoadState('networkidle').catch(() => {});

        // Guard: if app is stuck at loading splash, force reload
        const loadingVisible = await page.locator('text=Loading application').isVisible().catch(() => false);
        if (loadingVisible) {
          await page.reload({ waitUntil: 'domcontentloaded' });
          await page.waitForLoadState('networkidle').catch(() => {});
        }
        return;
      }

      const storageState = await page.request.storageState();
      if (storageState.cookies?.length) {
        await page.context().addCookies(storageState.cookies);
      }
    }
  } catch (error) {
    apiLoginError = error instanceof Error ? error : new Error(String(error));
    console.error(`API login bootstrap failed for ${user.email}: ${apiLoginError.message}`);
  }

  await page.goto('/');
  const loginRouteDetected = page.url().includes('/login');
  const loginUiDetected = await hasVisibleSignInControls();
  let authSessionConfirmed = false;
  if (!loginRouteDetected && !loginUiDetected) {
    const authProbeResponse = await page.request.get('/api/v1/auth/me').catch(() => null);
    authSessionConfirmed = authProbeResponse?.ok() ?? false;
  }

  if (!loginRouteDetected && !loginUiDetected && authSessionConfirmed) {
    if (apiLoginError) {
      console.warn(`Continuing with existing authenticated session after API login bootstrap failure for ${user.email}`);
    }
    await page.waitForLoadState('networkidle').catch(() => {});
    return;
  }

  if (!loginRouteDetected) {
    await page.goto('/login');
  }
  await page.locator('input[type="email"]').fill(user.email);
  await page.locator('input[type="password"]').fill(TEST_PASSWORD);

  const loginResponsePromise = page.waitForResponse((response) =>
    response.url().includes('/api/v1/auth/login')
  );
  await page.getByRole('button', { name: /sign in/i }).click();

  const loginResponse = await loginResponsePromise;
  if (!loginResponse.ok()) {
    const body = await loginResponse.text();
    const fallbackMessage = `Login failed: ${loginResponse.status()} - ${body}`;
    if (apiLoginError) {
      throw new Error(`${fallbackMessage}; API login bootstrap error: ${apiLoginError.message}`);
    }
    throw new Error(fallbackMessage);
  }

  await page.waitForURL(/\/(?:$|dashboard)/, { timeout: 15000 });
}

/**
 * Helper function to log out the current user
 * @param page - Playwright Page instance
 */
export async function logoutUser(page: import('@playwright/test').Page): Promise<void> {
  // Use text-based selector that handles emoji prefix (🚪 Logout)
  // The button text contains "Logout" - use case-insensitive text matching
  const logoutButton = page.getByText(/logout/i).first();

  // Wait for the logout button to be visible and click it
  await logoutButton.waitFor({ state: 'visible', timeout: 10000 });
  await logoutButton.click();

  // Wait for redirect to login page
  await page.waitForURL(/\/login/, { timeout: 15000, waitUntil: 'domcontentloaded' });
}

/**
 * Re-export expect from @playwright/test for convenience
 */
export { expect } from './test';

/**
 * Re-export the default test password for use in tests
 */
export { TEST_PASSWORD };
