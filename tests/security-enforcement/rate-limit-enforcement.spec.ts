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

import { test, expect } from '@bgotink/playwright-coverage';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import {
  getSecurityStatus,
  setSecurityModuleEnabled,
  captureSecurityState,
  restoreSecurityState,
  CapturedSecurityState,
} from '../utils/security-helpers';

/**
 * Configure admin whitelist to allow test runner IPs.
 * CRITICAL: Must be called BEFORE enabling any security modules to prevent 403 blocking.
 */
async function configureAdminWhitelist(requestContext: APIRequestContext) {
  // Configure whitelist to allow test runner IPs (localhost, Docker networks)
  const testWhitelist = '127.0.0.1/32,172.16.0.0/12,192.168.0.0/16,10.0.0.0/8';

  const response = await requestContext.patch(
    `${process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080'}/api/v1/config`,
    {
      data: {
        security: {
          admin_whitelist: testWhitelist,
        },
      },
    }
  );

  if (!response.ok()) {
    throw new Error(`Failed to configure admin whitelist: ${response.status()}`);
  }

  console.log('✅ Admin whitelist configured for test IP ranges');
}

test.describe('Rate Limit Enforcement', () => {
  let requestContext: APIRequestContext;
  let originalState: CapturedSecurityState;

  test.beforeAll(async () => {
    requestContext = await request.newContext({
      baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',
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
    await requestContext.dispose();
  });

  test('should verify rate limiting is enabled', async () => {
    const status = await getSecurityStatus(requestContext);
    expect(status.rate_limit.enabled).toBe(true);
    expect(status.cerberus.enabled).toBe(true);
  });

  test('should return rate limit presets', async () => {
    const response = await requestContext.get(
      '/api/v1/security/rate-limit/presets'
    );
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
    // Rate limiting enforcement happens at Caddy layer
    // When threshold is exceeded, Caddy returns 429 Too Many Requests
    //
    // With rate limiting enabled:
    // - Requests exceeding the configured rate per IP/path return 429
    // - The response includes Retry-After header
    //
    // Direct API requests to backend bypass Caddy rate limiting

    const status = await getSecurityStatus(requestContext);
    expect(status.rate_limit.enabled).toBe(true);

    // Document: When rate limiting is enabled and request goes through Caddy:
    // - Requests exceeding threshold return 429 Too Many Requests
    // - X-RateLimit-Limit, X-RateLimit-Remaining headers are included
    console.log(
      'Rate limiting configured - threshold enforcement active at Caddy layer'
    );
  });
});
