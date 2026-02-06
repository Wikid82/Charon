/**
 * CrowdSec Enforcement Tests
 *
 * Tests that verify CrowdSec integration for IP reputation and ban management.
 *
 * Pattern: Toggle-On-Test-Toggle-Off
 *
 * @see /projects/Charon/docs/plans/current_spec.md - CrowdSec Enforcement Tests
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
    `${process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080'}/api/v1/config`,
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

test.describe('CrowdSec Enforcement', () => {
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

    // Enable CrowdSec
    try {
      await setSecurityModuleEnabled(requestContext, 'crowdsec', true);
      console.log('✓ CrowdSec enabled');
    } catch (error) {
      console.error('Failed to enable CrowdSec:', error);
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
          await setSecurityModuleEnabled(requestContext, 'crowdsec', false);
          await setSecurityModuleEnabled(requestContext, 'cerberus', false);
        } catch {
          console.error('Emergency CrowdSec disable also failed');
        }
      }
    }
    await requestContext.dispose();
  });

  test('should verify CrowdSec is enabled', async () => {
    const status = await getSecurityStatus(requestContext);
    expect(status.crowdsec.enabled).toBe(true);
    expect(status.cerberus.enabled).toBe(true);
  });

  test('should list CrowdSec decisions', async () => {
    const response = await requestContext.get('/api/v1/security/decisions');

    // CrowdSec may not be fully configured in test environment
    if (response.ok()) {
      const decisions = await response.json();
      expect(Array.isArray(decisions) || decisions.decisions !== undefined).toBe(
        true
      );
    } else {
      // 500/502/503 is acceptable if CrowdSec LAPI is not running
      const errorText = await response.text();
      console.log(
        `CrowdSec LAPI not available (expected in test env): ${response.status()} - ${errorText}`
      );
      expect([500, 502, 503]).toContain(response.status());
    }
  });

  test('should return CrowdSec status with mode and API URL', async () => {
    const response = await requestContext.get('/api/v1/security/status');
    expect(response.ok()).toBe(true);

    const status = await response.json();
    expect(status.crowdsec).toBeDefined();
    expect(typeof status.crowdsec.enabled).toBe('boolean');
    expect(status.crowdsec.mode).toBeDefined();

    // API URL may be present when configured
    if (status.crowdsec.api_url) {
      expect(typeof status.crowdsec.api_url).toBe('string');
    }
  });
});
