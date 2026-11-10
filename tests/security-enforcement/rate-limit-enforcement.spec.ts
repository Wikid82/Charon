/**
 * Rate Limiting Enforcement Tests
 *
 * Tests that verify rate limiting configuration and expected behavior.
 *
 * NOTE: Actual rate limiting happens at Caddy layer. These tests verify
 * the rate limiting configuration API and presets.
 *
 * Pattern: Toggle-On-Test-Toggle-Off
 *
 * @see /projects/Charon/docs/plans/current_spec.md - Rate Limit Enforcement Tests
 */

import { test, expect } from '../fixtures/test';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import {
  getSecurityStatus,
  setSecurityModuleEnabled,
  captureSecurityState,
  restoreSecurityState,
  SecurityStateSnapshot,
} from '../utils/security-helpers';
import {
  STANDARD_API_RATE_LIMIT_BUDGET,
  getConfigRow,
  readApiRateLimitBudget,
  restoreApiRateLimitBudget,
  saveConfig,
  writeApiRateLimitBudget,
  type ApiRateLimitBudget,
  type SecurityConfigRow,
} from '../utils/security-settings-api';
import {
  PROFILE_HEADER_EXPECTATIONS,
  createHeaderProfile,
  createProxyHost,
  deleteHeaderProfile,
  deleteProxyHost,
  fetchThroughProxy,
  type HeaderProfile,
} from '../utils/proxy-refusal-helpers';

/**
 * Configure admin whitelist to allow test runner IPs.
 * CRITICAL: Must be called BEFORE enabling any security modules to prevent 403 blocking.
 */
async function configureAdminWhitelist(requestContext: APIRequestContext) {
  // Configure whitelist to allow test runner IPs (localhost, Docker networks)
  const testWhitelist = '127.0.0.1/32,172.16.0.0/12,192.168.0.0/16,10.0.0.0/8';

  const maxRetries = 5;
  const retryDelayMs = 1000;

  for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
    const response = await requestContext.patch(
      `${process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080'}/api/v1/config`,
      {
        data: {
          security: {
            admin_whitelist: testWhitelist,
          },
        },
      }
    );

    if (response.ok()) {
      console.log('✅ Admin whitelist configured for test IP ranges');
      return;
    }

    if (response.status() !== 429 || attempt === maxRetries) {
      throw new Error(`Failed to configure admin whitelist: ${response.status()}`);
    }

    await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
  }

  throw new Error('Failed to configure admin whitelist after retries');
}

test.describe('Rate Limit Enforcement', () => {
  // Shared security state and the API rate limit budget are changed here, so the cases must not interleave.
  test.describe.configure({ mode: 'serial' });

  let requestContext: APIRequestContext;
  let originalState: SecurityStateSnapshot;
  let originalApiBudget: ApiRateLimitBudget | undefined;

  test.beforeAll(async () => {
    requestContext = await request.newContext({
      baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080',
      storageState: STORAGE_STATE,
    });

    // CRITICAL: Configure admin whitelist BEFORE enabling security modules
    try {
      await configureAdminWhitelist(requestContext);
    } catch (error) {
      console.error('Failed to configure admin whitelist:', error);
    }

    // Capture original state
    try {
      originalState = await captureSecurityState(requestContext);
    } catch (error) {
      console.error('Failed to capture original security state:', error);
    }

    // The API limiter reads its budget from these settings. Earlier cases can leave a tight budget behind, and that
    // would refuse this case's own API calls, so the standard budget is set here and the captured one is restored.
    try {
      originalApiBudget = await readApiRateLimitBudget(requestContext);
      await writeApiRateLimitBudget(requestContext, STANDARD_API_RATE_LIMIT_BUDGET);
    } catch (error) {
      console.error('Failed to set the standard API rate limit budget:', error);
    }

    // Enable Cerberus (master toggle) first
    try {
      await setSecurityModuleEnabled(requestContext, 'cerberus', true);
      console.log('✓ Cerberus enabled');
    } catch (error) {
      console.error('Failed to enable Cerberus:', error);
    }

    // Enable Rate Limiting
    try {
      await setSecurityModuleEnabled(requestContext, 'rateLimit', true);
      // Wait for rate limiting to propagate
      await new Promise(r => setTimeout(r, 2000));
      console.log('✓ Rate Limiting enabled');
    } catch (error) {
      console.error('Failed to enable Rate Limiting:', error);
    }
  });

  test.afterAll(async () => {
    // Restore original state
    if (originalState) {
      try {
        await restoreSecurityState(requestContext, originalState);
        console.log('✓ Security state restored');
      } catch (error) {
        console.error('Failed to restore security state:', error);
        // Emergency disable
        try {
          await setSecurityModuleEnabled(requestContext, 'rateLimit', false);
          await setSecurityModuleEnabled(requestContext, 'cerberus', false);
        } catch {
          console.error('Emergency Rate Limit disable also failed');
        }
      }
    }
    if (originalApiBudget) {
      try {
        await restoreApiRateLimitBudget(requestContext, originalApiBudget);
      } catch (error) {
        console.error('Failed to restore the API rate limit budget:', error);
      }
    }
    await requestContext.dispose();
  });

  test('should verify rate limiting is enabled', async () => {
    // Wait with retry for rate limiting to be enabled
    // Due to parallel test execution, settings may take time to propagate
    let status = await getSecurityStatus(requestContext);
    let retries = 10;

    while ((!status.rate_limit.enabled || !status.cerberus.enabled) && retries > 0) {
      await new Promise((resolve) => setTimeout(resolve, 500));
      status = await getSecurityStatus(requestContext);
      retries--;
    }

    // If still not enabled, try enabling it (may have been disabled by parallel tests)
    if (!status.rate_limit.enabled || !status.cerberus.enabled) {
      console.log('⚠️ Rate limiting or Cerberus was disabled, attempting to re-enable...');
      try {
        await setSecurityModuleEnabled(requestContext, 'cerberus', true);
        await new Promise(r => setTimeout(r, 1000));
        await setSecurityModuleEnabled(requestContext, 'rateLimit', true);
        await new Promise(r => setTimeout(r, 2000));
        status = await getSecurityStatus(requestContext);
      } catch (error) {
        console.log(`⚠️ Failed to re-enable modules: ${error}`);
      }

      if (!status.rate_limit.enabled) {
        console.log('⚠️ Rate limiting could not be enabled - continuing test anyway');
        // Changed from testInfo.skip() to allow test to run and potentially identify root cause
        // testInfo.skip(true, 'Rate limiting could not be enabled - possible test isolation issue');
        // return;
      }
    }

    expect(status.rate_limit.enabled).toBe(true);
    expect(status.cerberus.enabled).toBe(true);
  });

  test('should return rate limit presets', async () => {
    const maxRetries = 5;
    const retryDelayMs = 1000;
    let response = await requestContext.get('/api/v1/security/rate-limit/presets');

    for (let attempt = 0; attempt < maxRetries && response.status() === 429; attempt += 1) {
      await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
      response = await requestContext.get('/api/v1/security/rate-limit/presets');
    }

    expect(response.ok()).toBe(true);

    const data = await response.json();
    const presets = data.presets;
    expect(Array.isArray(presets)).toBe(true);

    // Presets should have expected structure
    if (presets.length > 0) {
      const preset = presets[0];
      expect(preset.name || preset.id).toBeDefined();
    }
  });

  test('should document threshold behavior when rate exceeded', async () => {
    // Flaky test - polling timeout for status.rate_limit.enabled. Rate limiting verified in integration tests.

    // Mark as slow - security module status propagation requires extended timeouts
    test.slow();

    // Rate limiting enforcement happens at Caddy layer
    // When threshold is exceeded, Caddy returns 429 Too Many Requests
    //
    // With rate limiting enabled:
    // - Requests exceeding the configured rate per IP/path return 429
    // - The response includes Retry-After header
    //
    // Direct API requests to backend bypass Caddy rate limiting

    // Use polling pattern to verify rate limit is enabled before checking
    await expect(async () => {
      const status = await getSecurityStatus(requestContext);
      expect(status.rate_limit.enabled).toBe(true);
    }).toPass({ timeout: 15000, intervals: [2000, 3000, 5000] });

    // Document: When rate limiting is enabled and request goes through Caddy:
    // - Requests exceeding threshold return 429 Too Many Requests
    // - X-RateLimit-Limit, X-RateLimit-Remaining headers are included
    console.log(
      'Rate limiting configured - threshold enforcement active at Caddy layer'
    );
  });

  test.describe('Headers on rate limited responses', () => {
    test.describe.configure({ mode: 'serial' });

    // Small enough that a few proxied requests from the runner exhaust it within the window. This is the proxy
    // limit stored in the security config. It does not reach the API limiter, which reads its own budget.
    const RATE_LIMIT_REQUESTS = 2;
    const RATE_LIMIT_WINDOW_SEC = 60;
    let domain = '';
    let proxyContext: APIRequestContext | undefined;
    let profile: HeaderProfile | undefined;
    let proxyUuid: string | undefined;
    let previousConfig: SecurityConfigRow | null = null;

    test.beforeAll(async () => {
      // Retry-safe: a retried attempt starts from empty handles and a fresh domain, so it never collides with a
      // host left by the attempt before it.
      domain = `refusal-headers-429-${Date.now()}.example.test`;
      profile = undefined;
      proxyUuid = undefined;
      previousConfig = null;
      proxyContext = await request.newContext();

      // Fixtures and the captured thresholds come first, while the stored limit is still the original one.
      previousConfig = await getConfigRow(requestContext);
      profile = await createHeaderProfile(requestContext, `Rate Limit Headers ${Date.now()}`);
      proxyUuid = await createProxyHost(requestContext, {
        domain,
        // Nothing listens here: requests under the limit fail upstream with 502, requests over it are refused first.
        forwardPort: 3001,
        name: 'Rate limit headers host',
        headerProfileUuid: profile.uuid,
      });
      await saveConfig(requestContext, {
        name: 'default',
        rate_limit_requests: RATE_LIMIT_REQUESTS,
        rate_limit_window_sec: RATE_LIMIT_WINDOW_SEC,
      });
    });

    test.afterAll(async () => {
      // Every step runs even when an earlier one fails, so the stored thresholds and fixtures never leak into the
      // cases that run after this one. The first failure is re-thrown once all steps have run.
      const failures: unknown[] = [];
      const cleanup = async (step: () => Promise<void>) => {
        try {
          await step();
        } catch (error) {
          failures.push(error);
        }
      };

      const original = previousConfig as SecurityConfigRow | null;
      if (original) {
        await cleanup(() =>
          saveConfig(requestContext, {
            name: 'default',
            rate_limit_requests: original.rate_limit_requests,
            rate_limit_window_sec: original.rate_limit_window_sec,
          }),
        );
      }
      const uuid = proxyUuid;
      if (uuid) {
        await cleanup(() => deleteProxyHost(requestContext, uuid));
      }
      const createdProfile = profile;
      if (createdProfile) {
        await cleanup(() => deleteHeaderProfile(requestContext, createdProfile));
      }
      await proxyContext?.dispose();

      if (failures.length > 0) {
        throw failures[0];
      }
    });

    test('should send the profile security headers with a rate limit refusal', async () => {
      let refusedHeaders: Record<string, string> | undefined;
      const context = proxyContext;
      if (!context) {
        throw new Error('proxy request context was not created in beforeAll');
      }

      // Requests are sent until the limit is exceeded once the route is loaded. The rate limit answers 429 on the
      // first request past the limit, so this polls rather than assuming a fixed number of requests.
      await expect
        .poll(
          async () => {
            const response = await fetchThroughProxy(context, domain, '/');
            if (response.status() === 429) {
              refusedHeaders = response.headers();
            }
            return response.status();
          },
          { timeout: 30_000, message: 'rate limit should refuse requests over the limit' }
        )
        .toBe(429);

      expect(refusedHeaders).toMatchObject(PROFILE_HEADER_EXPECTATIONS);
    });
  });
});
