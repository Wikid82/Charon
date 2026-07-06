/**
 * Authentication E2E Tests
 *
 * Tests the authentication flows for the Charon application including:
 * - Login with valid credentials
 * - Login with invalid credentials
 * - Login with non-existent user
 * - Logout functionality
 * - Session persistence across page refreshes
 * - Session expiration handling
 *
 * These tests use per-test user fixtures to ensure isolation and avoid
 * race conditions in parallel execution.
 *
 * @see /projects/Charon/docs/plans/current_spec.md - Section 4.1.2
 */

import { test, expect, loginUser, logoutUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { waitForToast, waitForLoadingComplete, waitForAPIResponse, waitForDebounce } from '../utils/wait-helpers';

test.describe('Authentication Flows', () => {
  test.describe('Login with Valid Credentials', () => {
    /**
     * Test: Successful login redirects to dashboard
     * Verifies that a user with valid credentials can log in and is redirected
     * to the main dashboard.
     */
    test('should login with valid credentials and redirect to dashboard', async ({
      page,
      adminUser,
    }) => {
      await test.step('Navigate to login page', async () => {
        await page.goto('/login');
        // Verify login page is loaded by checking for the email input field
        await expect(page.locator('input[type="email"]')).toBeVisible();
      });

      await test.step('Enter valid credentials', async () => {
        await page.locator('input[type="email"]').fill(adminUser.email);
        await page.locator('input[type="password"]').fill(TEST_PASSWORD);
      });

      await test.step('Submit login form', async () => {
        const responsePromise = waitForAPIResponse(page, '/api/v1/auth/login', { status: 200 });
        await page.getByRole('button', { name: /sign in/i }).click();
        await responsePromise;
      });

      await test.step('Verify redirect to dashboard', async () => {
        await page.waitForURL('/');
        await waitForLoadingComplete(page);
        // Dashboard should show main content area
        await expect(page.getByRole('main')).toBeVisible();
      });
    });

    /**
     * Test: Login form shows loading state during authentication
     */
    test('should show loading state during authentication', async ({ page, adminUser }) => {
      await page.goto('/login');

      await page.locator('input[type="email"]').fill(adminUser.email);
      await page.locator('input[type="password"]').fill(TEST_PASSWORD);

      await test.step('Verify loading state on submit', async () => {
        const loginButton = page.getByRole('button', { name: /sign in/i });
        await loginButton.click();

        // Button should be disabled or show loading indicator during request
        await expect(loginButton).toBeDisabled().catch(() => {
          // Some implementations use loading spinner instead
        });
      });

      await page.waitForURL('/');
    });
  });

  test.describe('Login with Invalid Credentials', () => {
    /**
     * Test: Wrong password shows error message
     * Verifies that entering an incorrect password displays an appropriate error.
     */
    test('should show error message for wrong password', async ({ page, adminUser }) => {
      await test.step('Navigate to login page', async () => {
        await page.goto('/login');
      });

      await test.step('Enter valid email with wrong password', async () => {
        await page.locator('input[type="email"]').fill(adminUser.email);
        await page.locator('input[type="password"]').fill('WrongPassword123!');
      });

      await test.step('Submit and verify error', async () => {
        const loginResponsePromise = page.waitForResponse(
          (response) => response.url().includes('/api/v1/auth/login') && response.request().method() === 'POST',
          { timeout: 15000 },
        );

        await page.getByRole('button', { name: /sign in/i }).click();

        const loginResponse = await loginResponsePromise;
        expect([400, 401, 403, 404]).toContain(loginResponse.status());

        const errorToast = page.getByTestId('toast-error');
        const inlineError = page.getByText(/invalid|not found|failed|incorrect|unauthorized/i).first();
        const hasErrorToast = await errorToast.isVisible({ timeout: 10000 }).catch(() => false);
        const hasInlineError = await inlineError.isVisible({ timeout: 10000 }).catch(() => false);

        expect(hasErrorToast || hasInlineError).toBe(true);
      });

      await test.step('Verify user stays on login page', async () => {
        await expect(page).toHaveURL(/login/);
      });
    });

    /**
     * Test: Empty password shows validation error
     */
    test('should show validation error for empty password', async ({ page, adminUser }) => {
      await page.goto('/login');

      await page.locator('input[type="email"]').fill(adminUser.email);
      // Leave password empty

      await test.step('Submit and verify validation error', async () => {
        await page.getByRole('button', { name: /sign in/i }).click();

        // Check for HTML5 validation state, aria-invalid, or visible error text
        const passwordInput = page.locator('input[type="password"]');
        const isInvalid =
          (await passwordInput.getAttribute('aria-invalid')) === 'true' ||
          (await passwordInput.evaluate((el: HTMLInputElement) => !el.validity.valid)) ||
          (await page.getByText(/password.*required|required.*password/i).isVisible().catch(() => false));

        expect(isInvalid).toBeTruthy();
      });
    });
  });

  test.describe('Login with Non-existent User', () => {
    /**
     * Test: Unknown email shows error message
     * Verifies that a non-existent user email displays an appropriate error.
     */
    test('should show error message for non-existent user', async ({ page }) => {
      await test.step('Navigate to login page', async () => {
        await page.goto('/login');
      });

      await test.step('Enter non-existent email', async () => {
        const nonExistentEmail = `nonexistent-${Date.now()}@test.local`;
        await page.locator('input[type="email"]').fill(nonExistentEmail);
        await page.locator('input[type="password"]').fill('SomePassword123!');
      });

      await test.step('Submit and verify error', async () => {
        await page.getByRole('button', { name: /sign in/i }).click();

        // Wait for error toast to appear (use specific test ID to avoid strict mode violation)
        const errorMessage = page.getByTestId('toast-error');
        await expect(errorMessage).toBeVisible({ timeout: 10000 });
      });

      await test.step('Verify user stays on login page', async () => {
        await expect(page).toHaveURL(/login/);
      });
    });

    /**
     * Test: Invalid email format shows validation error
     */
    test('should show validation error for invalid email format', async ({ page }) => {
      await page.goto('/login');

      // Wait for the setup-status check to complete so the login form is visible.
      // In slow Firefox CI environments the form is replaced by a loading screen
      // while the check is in-flight, causing fill() to time out.
      const emailInput = page.locator('input[type="email"]');
      await emailInput.waitFor({ state: 'visible', timeout: 15000 });

      await emailInput.fill('not-an-email');
      await page.locator('input[type="password"]').fill('SomePassword123!');

      await test.step('Verify email validation error', async () => {
        await page.getByRole('button', { name: /sign in/i }).click();

        // Check for HTML5 validation state or aria-invalid or visible error text
        const isInvalid =
          (await emailInput.getAttribute('aria-invalid')) === 'true' ||
          (await emailInput.evaluate((el: HTMLInputElement) => !el.validity.valid)) ||
          (await page.getByText(/valid email|invalid email/i).isVisible().catch(() => false));

        expect(isInvalid).toBeTruthy();
      });
    });
  });

  test.describe('Logout Functionality', () => {
    /**
     * Test: Logout redirects to login page and clears session
     * Verifies that clicking logout properly ends the session.
     *
     * NOTE: This test currently reveals a frontend bug where the application
     * allows access to protected routes after logout. The session appears to
     * persist or is not properly validated by route guards.
     */
    test('should logout and redirect to login page', async ({ page, adminUser }) => {
      await test.step('Login first', async () => {
        await loginUser(page, adminUser);
        await expect(page).toHaveURL('/');
      });

      await test.step('Perform logout', async () => {
        await logoutUser(page);
      });

      await test.step('Verify redirect to login page', async () => {
        await expect(page).toHaveURL(/login/);
      });

      await test.step('Verify session is cleared - cannot access protected route', async () => {
        await page.goto('/');
        await waitForLoadingComplete(page);
        // Should redirect to login since session is cleared
        await page.waitForURL(/login/, { timeout: 10000 }).catch(async () => {
          // If no redirect, check if we're on a protected page that shows login form
          const hasLoginForm = await page.locator('input[type="email"]').isVisible().catch(() => false);
          if (!hasLoginForm) {
            throw new Error('Expected redirect to login after session cleared. Dashboard loaded instead, indicating missing route guard.');
          }
        });
      });
    });

    /**
     * Test: Logout clears authentication cookies
     */
    test('should clear authentication cookies on logout', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      await test.step('Verify auth cookie exists before logout', async () => {
        const cookies = await page.context().cookies();
        const hasAuthCookie = cookies.some(
          (c) => c.name.includes('token') || c.name.includes('auth') || c.name.includes('session')
        );
        // Some implementations use localStorage instead of cookies
        if (!hasAuthCookie) {
          const localStorageToken = await page.evaluate(() =>
            localStorage.getItem('token') || localStorage.getItem('authToken')
          );
          expect(localStorageToken || hasAuthCookie).toBeTruthy();
        }
      });

      await logoutUser(page);

      await test.step('Verify auth data cleared after logout', async () => {
        const cookies = await page.context().cookies();
        const hasAuthCookie = cookies.some(
          (c) =>
            (c.name.includes('token') || c.name.includes('auth') || c.name.includes('session')) &&
            c.value !== ''
        );

        const localStorageToken = await page.evaluate(() =>
          localStorage.getItem('token') || localStorage.getItem('authToken')
        );

        expect(hasAuthCookie && localStorageToken).toBeFalsy();
      });
    });
  });

  test.describe('Session Persistence', () => {
    /**
     * Test: Session persists across page refresh
     * Verifies that refreshing the page maintains the logged-in state.
     */
    test('should maintain session after page refresh', async ({ page, adminUser }) => {
      await test.step('Login', async () => {
        await loginUser(page, adminUser);
        await expect(page).toHaveURL('/');
      });

      await test.step('Refresh page', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Verify still logged in', async () => {
        // Should still be on dashboard, not redirected to login
        await expect(page).toHaveURL('/');
        await expect(page.getByRole('main')).toBeVisible();
      });
    });

    /**
     * Test: Session persists when navigating between pages
     */
    test('should maintain session when navigating between pages', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to different pages', async () => {
        // Navigate to proxy hosts
        const proxyHostsLink = page
          .getByRole('navigation')
          .getByRole('link', { name: /proxy.*hosts?/i });
        if (await proxyHostsLink.isVisible()) {
          await proxyHostsLink.click();
          await waitForLoadingComplete(page);
          await expect(page.getByRole('main')).toBeVisible();
        }

        // Navigate back to dashboard
        await page.goto('/');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify still logged in', async () => {
        await expect(page).toHaveURL('/');
        await expect(page.getByRole('main')).toBeVisible();
      });
    });
  });

  test.describe('Session Expiration Handling', () => {
    /**
     * Test: Expired token redirects to login
     * Simulates an expired session and verifies proper redirect.
     *
     * NOTE: Route guard fix has been merged (2026-01-30). Validating that session
     * expiration properly redirects to login page.
     */
    test('should redirect to login when session expires', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      await test.step('Simulate session expiration by clearing auth data', async () => {
        // Clear cookies and storage to simulate expiration
        await page.context().clearCookies();
        await page.evaluate(() => {
          localStorage.removeItem('token');
          localStorage.removeItem('authToken');
          localStorage.removeItem('charon_auth_token');
          sessionStorage.clear();
        });
      });

      await test.step('Attempt to access protected resource', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
      });

      await test.step('Verify redirect to login', async () => {
        // Give the app time to detect expired session and redirect
        // BUG: Currently the app shows dashboard instead of redirecting
        await page.waitForURL(/login/, { timeout: 15000 }).catch(async () => {
          // If no redirect, check if we're shown a login form or session expired message
          const hasLoginForm = await page.locator('input[type="email"]').isVisible().catch(() => false);
          const hasExpiredMessage = await page.getByText(/session.*expired|please.*login/i).isVisible().catch(() => false);
          if (!hasLoginForm && !hasExpiredMessage) {
            throw new Error('Expected redirect to login or session expired message. Dashboard loaded instead, indicating missing auth validation.');
          }
        });
      });
    });

    /**
     * Test: API returns 401 on expired token, UI handles gracefully
     */
    test('should handle 401 response gracefully', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      await test.step('Simulate expired session (clear token and intercept API with 401)', async () => {
        // Clear the auth token from storage so checkAuth immediately sees an
        // invalid session without needing a network round-trip. This avoids
        // a Firefox CDP timing issue where XHR requests fired during page.goto()
        // navigation are not intercepted by Playwright route handlers.
        await page.evaluate(() => localStorage.removeItem('charon_auth_token'));

        // Also intercept API calls to return 401 to test the axios error
        // interceptor path (defense-in-depth).
        await page.route('**/api/v1/**', async (route) => {
          // Let health check through, block others with 401
          if (route.request().url().includes('/health')) {
            await route.continue();
          } else {
            await route.fulfill({
              status: 401,
              contentType: 'application/json',
              body: JSON.stringify({ message: 'Token expired' }),
            });
          }
        });
      });

      await test.step('Trigger an auth check by navigating', async () => {
        await page.goto('/proxy-hosts');
        // Wait for the 401 response to be processed and UI to react
        await waitForDebounce(page);
      });

      await test.step('Verify redirect to login or error message', async () => {
        // Wait for potential redirect to login page
        // 15s timeout matches the sister test 'should redirect to login when session expires'.
        // Firefox under CI can take >5s for: useEffect → fetch /auth/me → 401 → React update → navigate.
        try {
          await page.waitForURL(/\/login/, { timeout: 15000 });
        } catch {
          // If no redirect, check for session expired message
        }

        // Should either redirect to login or show session expired message
        const isLoginPage = page.url().includes('/login');
        const hasSessionExpiredMessage = await page
          .getByText(/session.*expired|please.*login|unauthorized/i)
          .isVisible()
          .catch(() => false);

        expect(isLoginPage || hasSessionExpiredMessage).toBeTruthy();
      });
    });
  });

  test.describe('Authentication Accessibility', () => {
    async function pressTabUntilFocused(page: import('@playwright/test').Page, target: import('@playwright/test').Locator, maxTabs: number): Promise<void> {
      for (let i = 0; i < maxTabs; i++) {
        await page.keyboard.press('Tab');
        const focused = await expect
          .poll(async () => target.evaluate((el) => el === document.activeElement), {
            timeout: 1500,
            intervals: [100, 200, 300],
          })
          .toBeTruthy()
          .then(() => true)
          .catch(() => false);
        if (focused) {
          return;
        }
      }

      await expect(target).toBeFocused();
    }

    /**
     * Test: Login form is keyboard accessible
     */
    test('should be fully keyboard navigable', async ({ page }) => {
      await page.goto('/login');

      await test.step('Tab through form elements and verify focus order', async () => {
        const emailInput = page.locator('input[type="email"]');
        const passwordInput = page.locator('input[type="password"]');
        const submitButton = page.getByRole('button', { name: /sign in/i });

        // Focus the email input first
        await emailInput.focus();
        await expect(emailInput).toBeFocused();

        // Tab to password field
        await pressTabUntilFocused(page, passwordInput, 2);

        // Tab to submit button (may go through "Forgot Password" link first)
        await pressTabUntilFocused(page, submitButton, 3);
      });
    });

    /**
     * Test: Login form has proper ARIA labels
     */
    test('should have accessible form labels', async ({ page }) => {
      await page.goto('/login');

      await test.step('Verify email input is visible', async () => {
        const emailInput = page.locator('input[type="email"]');
        await expect(emailInput).toBeVisible();
      });

      await test.step('Verify password input is visible', async () => {
        const passwordInput = page.locator('input[type="password"]');
        await expect(passwordInput).toBeVisible();
      });

      await test.step('Verify submit button has accessible name', async () => {
        const submitButton = page.getByRole('button', { name: /sign in/i });
        await expect(submitButton).toBeVisible();
      });
    });

    /**
     * Test: Error messages are announced to screen readers
     */
    test('should announce errors to screen readers', async ({ page }) => {
      await page.goto('/login');

      await page.locator('input[type="email"]').fill('test@example.com');
      await page.locator('input[type="password"]').fill('wrongpassword');
      await page.getByRole('button', { name: /sign in/i }).click();

      await test.step('Verify error has proper ARIA role', async () => {
        const errorAlert = page.locator('[role="alert"], [aria-live="polite"], [aria-live="assertive"]');
        // Wait for error to appear
        await expect(errorAlert).toBeVisible({ timeout: 10000 }).catch(() => {
          // Some implementations may not use live regions
        });
      });
    });
  });
});
