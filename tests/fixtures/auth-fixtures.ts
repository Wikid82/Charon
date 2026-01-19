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

import { test as base, expect } from '@bgotink/playwright-coverage';
import { TestDataManager } from '../utils/TestDataManager';

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
   * Creates a unique namespace per test and cleans up all resources after
   */
  testData: async ({ request }, use, testInfo) => {
    const manager = new TestDataManager(request, testInfo.title);
    await use(manager);
    await manager.cleanup();
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
  await page.goto('/login');
  await page.locator('input[type="email"]').fill(user.email);
  await page.locator('input[type="password"]').fill(TEST_PASSWORD);
  await page.getByRole('button', { name: /sign in/i }).click();
  await page.waitForURL('/');
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
export { expect } from '@bgotink/playwright-coverage';

/**
 * Re-export the default test password for use in tests
 */
export { TEST_PASSWORD };
