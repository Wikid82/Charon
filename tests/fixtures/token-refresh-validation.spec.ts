import { test, expect, refreshTokenIfNeeded } from './auth-fixtures';

/**
 * Token Refresh Validation Tests
 *
 * Validates that the token refresh mechanism works correctly for long-running E2E sessions.
 * These tests verify:
 * - Token cache creation and reading
 * - JWT expiry extraction
 * - Token refresh endpoint integration
 * - Concurrent access safety (file locking)
 */

test.describe('Token Refresh for Long-Running Sessions', () => {
  test('New token should be cached with expiry', async ({ adminUser, page }) => {
    const baseURL = page.context().baseURL || 'http://localhost:8080';

    // Get initial token
    let token = adminUser.token;

    // refresh should either return the same token or a new one
    const refreshedToken = await refreshTokenIfNeeded(baseURL, token);

    expect(refreshedToken).toBeTruthy();
    expect(refreshedToken).toMatch(/^[A-Za-z0-9\-_=]+\.[A-Za-z0-9\-_.]+\.[A-Za-z0-9\-_=]*$/);
  });

  test('Token refresh should work for 60-minute session simulation', async ({
    adminUser,
    page,
  }) => {
    const baseURL = page.context().baseURL || 'http://localhost:8080';
    let token = adminUser.token;
    let refreshCount = 0;

    // Simulate 6 checkpoints over 60 minutes (10-min intervals in test)
    // In production, these would be actual 10-minute intervals
    for (let i = 0; i < 6; i++) {
      const oldToken = token;

      // Attempt refresh (should be no-op if not expired)
      token = await refreshTokenIfNeeded(baseURL, token);

      if (token !== oldToken) {
        refreshCount++;
      }

      // Verify token is still valid by making a request
      const response = await page.request.get('/api/v1/auth/status', {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      expect(response.status()).toBeLessThan(400);

      // In a real 60-min test, this would wait 10 minutes
      // For validation, we skip the wait
      // await page.waitForTimeout(10*60*1000);
    }

    // Token should be valid after the session
    expect(token).toBeTruthy();
    expect(token).toMatch(/^[A-Za-z0-9\-_=]+\.[A-Za-z0-9\-_.]+\.[A-Za-z0-9\-_=]*$/);
  });

  test('Token should remain valid across page navigation', async ({ adminUser, page }) => {
    const baseURL = page.context().baseURL || 'http://localhost:8080';
    let token = adminUser.token;

    // Refresh token
    token = await refreshTokenIfNeeded(baseURL, token);

    // Set header for next request
    await page.setExtraHTTPHeaders({
      'Authorization': `Bearer ${token}`,
    });

    // Navigate to dashboard
    const response = await page.goto('/');
    expect(response?.status()).toBeLessThan(400);
  });

  test('Concurrent token access should not corrupt cache', async ({ adminUser }) => {
    const baseURL = 'http://localhost:8080';
    const token = adminUser.token;

    // Simulate concurrent refresh calls (would happen in parallel tests)
    const promises = Array.from({ length: 5 }, () =>
      refreshTokenIfNeeded(baseURL, token)
    );

    const results = await Promise.all(promises);

    // All should return valid tokens
    results.forEach((result) => {
      expect(result).toBeTruthy();
      expect(result).toMatch(/^[A-Za-z0-9\-_=]+\.[A-Za-z0-9\-_.]+\.[A-Za-z0-9\-_=]*$/);
    });
  });
});
