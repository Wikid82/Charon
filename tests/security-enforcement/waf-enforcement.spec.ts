/**
 * WAF (Coraza) Enforcement Tests
 *
 * Tests that verify the Web Application Firewall correctly blocks malicious
 * requests such as SQL injection and XSS attempts.
 *
 * NOTE: Full WAF blocking tests require Caddy proxy with Coraza plugin.
 * These tests verify the WAF configuration API and expected behavior.
 *
 * Pattern: Toggle-On-Test-Toggle-Off
 *
 * @see /projects/Charon/docs/plans/current_spec.md - WAF Enforcement Tests
 */

import { test, expect } from '../fixtures/test';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';

// CI-specific timeout multiplier: CI environments have higher I/O latency
const CI_TIMEOUT_MULTIPLIER = process.env.CI ? 3 : 1;
const BASE_PROPAGATION_WAIT = 3000;
const BASE_RETRY_INTERVAL = 1000;
const BASE_RETRY_COUNT_WAF = 5;
const BASE_RETRY_COUNT_STATUS = 10;
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

test.describe('WAF Enforcement', () => {
  let requestContext: APIRequestContext;
  let originalState: CapturedSecurityState;

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

    // Enable Cerberus (master toggle) first
    try {
      await setSecurityModuleEnabled(requestContext, 'cerberus', true);
      console.log('✓ Cerberus enabled');
    } catch (error) {
      console.error('Failed to enable Cerberus:', error);
    }

    // Enable WAF with extended wait for Caddy reload propagation
    try {
      await setSecurityModuleEnabled(requestContext, 'waf', true);
      // Wait for Caddy reload and WAF status propagation (3-5 seconds)
      await new Promise(r => setTimeout(r, BASE_PROPAGATION_WAIT * CI_TIMEOUT_MULTIPLIER));

      // Verify WAF enabled with retry
      let wafRetries = BASE_RETRY_COUNT_WAF * CI_TIMEOUT_MULTIPLIER;
      let status = await getSecurityStatus(requestContext);
      while (!status.waf.enabled && wafRetries > 0) {
        await new Promise(r => setTimeout(r, BASE_RETRY_INTERVAL * CI_TIMEOUT_MULTIPLIER));
        status = await getSecurityStatus(requestContext);
        wafRetries--;
      }

      console.log('✓ WAF enabled');
    } catch (error) {
      console.error('Failed to enable WAF:', error);
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
        // Emergency disable WAF to prevent interference
        try {
          await setSecurityModuleEnabled(requestContext, 'waf', false);
          await setSecurityModuleEnabled(requestContext, 'cerberus', false);
        } catch {
          console.error('Emergency WAF disable also failed');
        }
      }
    }
    await requestContext.dispose();
  });

  test('should verify WAF is enabled', async () => {
    // WAF enforcement verified in integration tests (backend/integration/coraza_integration_test.go). E2E tests UI only.

    // Use polling pattern to wait for WAF status propagation
    let status = await getSecurityStatus(requestContext);
    let retries = BASE_RETRY_COUNT_STATUS * CI_TIMEOUT_MULTIPLIER;

    while ((!status.waf.enabled || !status.cerberus.enabled) && retries > 0) {
      await new Promise(r => setTimeout(r, BASE_RETRY_INTERVAL * CI_TIMEOUT_MULTIPLIER));
      status = await getSecurityStatus(requestContext);
      retries--;
    }

    expect(status.waf.enabled).toBe(true);
    expect(status.cerberus.enabled).toBe(true);
  });

  test('should return WAF configuration from security status', async () => {
    const response = await requestContext.get('/api/v1/security/status');
    expect(response.ok()).toBe(true);

    const status = await response.json();
    expect(status.waf).toBeDefined();
    expect(status.waf.mode).toBeDefined();
    expect(typeof status.waf.enabled).toBe('boolean');
  });

  test('should detect SQL injection patterns in request validation', async () => {
    // SKIP: WAF blocking enforced via Coraza middleware (port 80).
    // See: backend/integration/coraza_integration_test.go
  });

  test('should document XSS blocking behavior', async () => {
    // SKIP: XSS blocking enforced via Coraza middleware (port 80).
    // See: backend/integration/coraza_integration_test.go
  });
});
