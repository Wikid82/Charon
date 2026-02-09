/**
 * Auth Fixtures - Per-test user creation with role-based authentication
 *
 * This module extends the base Playwright test with fixtures for:
 * - TestDataManager with automatic cleanup
 * - Per-test user creation (admin, regular, guest roles)
 * - Isolated authentication state per test
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

    // Validate cookie domain matches baseURL to catch configuration issues early
    try {
      const savedState = JSON.parse(readFileSync(STORAGE_STATE, 'utf-8'));
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
            `   Fix: Set PLAYWRIGHT_BASE_URL=http://localhost:8080 in your environment.`
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
      },
    });

    const manager = new TestDataManager(authenticatedContext, testInfo.title);

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
   * Guest user (read-only) fixture
   * Use for testing read-only access
   */
  guestUser: async ({ testData }, use) => {
    const user = await testData.createUser({
      name: `Test Guest ${Date.now()}`,
      email: `guest-${Date.now()}@test.local`,
      password: TEST_PASSWORD,
      role: 'guest',
    });
    await use({
      ...user,
      role: 'guest',
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
  const loginPayload = { email: user.email, password: TEST_PASSWORD };
  try {
    const response = await page.request.post('/api/v1/auth/login', { data: loginPayload });
    if (response.ok()) {
      const storageState = await page.request.storageState();
      if (storageState.cookies?.length) {
        await page.context().addCookies(storageState.cookies);
      }
    }
  } catch {
  }

  await page.goto('/');
  if (!page.url().includes('/login')) {
    await page.waitForLoadState('networkidle').catch(() => {});
    return;
  }

  await page.goto('/login');
  await page.locator('input[type="email"]').fill(user.email);
  await page.locator('input[type="password"]').fill(TEST_PASSWORD);

  const loginResponsePromise = page.waitForResponse((response) =>
    response.url().includes('/api/v1/auth/login')
  );
  await page.getByRole('button', { name: /sign in/i }).click();

  const loginResponse = await loginResponsePromise;
  if (!loginResponse.ok()) {
    const body = await loginResponse.text();
    throw new Error(`Login failed: ${loginResponse.status()} - ${body}`);
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
  await page.waitForURL(/\/login/, { timeout: 15000 });
}

/**
 * Re-export expect from @playwright/test for convenience
 */
export { expect } from './test';

/**
 * Re-export the default test password for use in tests
 */
export { TEST_PASSWORD };
