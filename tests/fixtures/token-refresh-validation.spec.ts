import {
  test,
  expect,
  refreshTokenIfNeeded,
  resetTokenRefreshStateForTests,
} from './auth-fixtures';
import { readdirSync } from 'fs';
import { tmpdir } from 'os';

function toBase64Url(value: string): string {
  return Buffer.from(value)
    .toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/g, '');
}

function createJwt(expiresInSeconds: number): string {
  const header = toBase64Url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = toBase64Url(
    JSON.stringify({
      exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
      sub: 'test-user',
    })
  );
  return `${header}.${payload}.signature`;
}

function createJsonResponse(body: Record<string, unknown>, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      'Content-Type': 'application/json',
    },
  });
}

/**
 * Token Refresh Validation Tests
 *
 * Validates that the token refresh mechanism works correctly for long-running E2E sessions.
 * These tests verify:
 * - Token cache creation and reading
 * - JWT expiry extraction
 * - Token refresh endpoint integration
 * - Concurrent access safety (in-memory serialization)
 */

function getTokenCacheDirCount(): number {
  return readdirSync(tmpdir(), { withFileTypes: true }).filter(
    (entry) => entry.isDirectory() && entry.name.startsWith('charon-test-token-cache-')
  ).length;
}

test.describe('Token Refresh for Long-Running Sessions', () => {
  test('New token should be cached with expiry', async ({ adminUser, page }) => {
    const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

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
    const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
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
    const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
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
    const cacheDirCountBefore = getTokenCacheDirCount();

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

    // In-memory cache should not depend on temp-directory artifacts
    const cacheDirCountAfter = getTokenCacheDirCount();
    expect(cacheDirCountAfter).toBe(cacheDirCountBefore);
  });
});

test.describe('refreshTokenIfNeeded branch and concurrency regression', () => {
  let originalFetch: typeof globalThis.fetch | undefined;

  test.beforeEach(async () => {
    originalFetch = globalThis.fetch;
    resetTokenRefreshStateForTests();
  });

  test.afterEach(async () => {
    if (originalFetch) {
      globalThis.fetch = originalFetch;
    }
    resetTokenRefreshStateForTests();
  });

  test('coalesces N concurrent callers to one refresh request with consistent token', async () => {
    const currentToken = createJwt(60);
    const refreshedToken = createJwt(3600);
    const baseURL = 'http://localhost:8080';
    const concurrentCallers = 20;
    let refreshInvocationCount = 0;

    let releaseRefreshResponse!: () => void;
    const refreshResponseGate = new Promise<void>((resolve) => {
      releaseRefreshResponse = resolve;
    });

    let markFirstRefreshStarted!: () => void;
    const firstRefreshStarted = new Promise<void>((resolve) => {
      markFirstRefreshStarted = resolve;
    });

    globalThis.fetch = (async () => {
      refreshInvocationCount += 1;
      if (refreshInvocationCount === 1) {
        markFirstRefreshStarted();
      }
      await refreshResponseGate;
      return createJsonResponse({ token: refreshedToken });
    }) as typeof globalThis.fetch;

    const resultsPromise = Promise.all(
      Array.from({ length: concurrentCallers }, () =>
        refreshTokenIfNeeded(baseURL, currentToken)
      )
    );

    await firstRefreshStarted;
    expect(refreshInvocationCount).toBe(1);

    releaseRefreshResponse();
    const results = await resultsPromise;

    expect(refreshInvocationCount).toBe(1);
    expect(results).toHaveLength(concurrentCallers);
    results.forEach((result) => {
      expect(result).toBe(refreshedToken);
    });
  });

  test('returns currentToken when refresh response is non-OK', async () => {
    const currentToken = createJwt(60);
    const baseURL = 'http://localhost:8080';
    let refreshInvocationCount = 0;

    globalThis.fetch = (async () => {
      refreshInvocationCount += 1;
      return createJsonResponse({ error: 'unauthorized' }, 401);
    }) as typeof globalThis.fetch;

    const result = await refreshTokenIfNeeded(baseURL, currentToken);

    expect(refreshInvocationCount).toBe(1);
    expect(result).toBe(currentToken);
  });

  test('returns currentToken when refresh response omits token field', async () => {
    const currentToken = createJwt(60);
    const baseURL = 'http://localhost:8080';
    let refreshInvocationCount = 0;

    globalThis.fetch = (async () => {
      refreshInvocationCount += 1;
      return createJsonResponse({});
    }) as typeof globalThis.fetch;

    const result = await refreshTokenIfNeeded(baseURL, currentToken);

    expect(refreshInvocationCount).toBe(1);
    expect(result).toBe(currentToken);
  });

  test('returns currentToken when fetch throws network error', async () => {
    const currentToken = createJwt(60);
    const baseURL = 'http://localhost:8080';
    let refreshInvocationCount = 0;

    globalThis.fetch = (async () => {
      refreshInvocationCount += 1;
      throw new Error('network down');
    }) as typeof globalThis.fetch;

    const result = await refreshTokenIfNeeded(baseURL, currentToken);

    expect(refreshInvocationCount).toBe(1);
    expect(result).toBe(currentToken);
  });

  test('returns currentToken and skips fetch when baseURL is undefined', async () => {
    const currentToken = createJwt(60);
    let refreshInvocationCount = 0;

    globalThis.fetch = (async () => {
      refreshInvocationCount += 1;
      return createJsonResponse({ token: createJwt(3600) });
    }) as typeof globalThis.fetch;

    const result = await refreshTokenIfNeeded(undefined, currentToken);

    expect(refreshInvocationCount).toBe(0);
    expect(result).toBe(currentToken);
  });
});
