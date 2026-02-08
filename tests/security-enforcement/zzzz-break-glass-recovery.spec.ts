/**
 * Break Glass Recovery - Restore Cerberus with Universal Bypass
 *
 * CRITICAL: This test MUST run AFTER emergency-reset.spec.ts (break glass test).
 * Uses 'zzz-' prefix to ensure alphabetical ordering places it near the end.
 *
 * Purpose:
 * - Break glass test disables Cerberus framework
 * - Browser UI tests need Cerberus ON to test toggles/navigation
 * - Setting admin_whitelist to test-runner ranges bypasses security checks for E2E
 * - This allows UI tests to run with full security stack enabled but bypassed
 *
 * Execution Order:
 * 1. Global setup → emergency reset (disables Cerberus)
 * 2. Security enforcement tests (ACL, WAF, Rate Limit, etc.)
 * 3. emergency-reset.spec.ts → Break glass test (validates emergency reset)
 * 4. THIS TEST → Restore Cerberus + test-runner whitelist bypass
 * 5. Browser tests → Run with Cerberus ON, ALL modules ON, but bypassed
 *
 * Why the test-runner whitelist is preferred:
 * - Bypasses security for local/private test runners only
 * - Keeps security enabled without opening global access
 * - Still exercises the admin whitelist feature
 * - Works in Docker, CI, and local environments
 *
 * @see /projects/Charon/docs/plans/e2e-test-triage-plan.md
 * @see POST /api/v1/emergency/security-reset
 * @see PATCH /api/v1/config (admin_whitelist)
 */

import { test, expect, request, APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import { getSecurityStatus } from '../utils/security-helpers';

test.describe.serial('Break Glass Recovery - Test-Runner Whitelist', () => {
  const EMERGENCY_TOKEN = process.env.CHARON_EMERGENCY_TOKEN;
  const EMERGENCY_URL = 'http://localhost:2020';
  const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
  const ADMIN_WHITELIST = '127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16';
  let apiContext: APIRequestContext;

  test.beforeAll(async () => {
    if (!EMERGENCY_TOKEN) {
      throw new Error(
        'CHARON_EMERGENCY_TOKEN required for break glass recovery\n' +
        'Generate with: openssl rand -hex 32'
      );
    }

    apiContext = await request.newContext({
      baseURL: BASE_URL,
      storageState: STORAGE_STATE,
    });
  });

  test.afterAll(async () => {
    if (apiContext) {
      await apiContext.dispose();
    }
  });

  test('Step 1: Configure admin whitelist for test-runner ranges', async () => {
    console.log('\n🔧 Break Glass Recovery: Setting admin whitelist for test runners...');

    await test.step('Set admin_whitelist to test-runner CIDRs', async () => {
      const response = await apiContext.patch('/api/v1/config', {
        data: {
          security: {
            admin_whitelist: ADMIN_WHITELIST,
          },
        },
      });

      expect(response.ok()).toBeTruthy();
      console.log('✅ Admin whitelist set to test-runner CIDRs');
    });

    await test.step('Verify whitelist configuration persisted', async () => {
      // Use /api/v1/security/config for reading (PATCH /api/v1/config has no GET)
      const response = await apiContext.get('/api/v1/security/config');
      expect(response).toBeOK();

      const body = await response.json();
      expect(body.config?.admin_whitelist).toBe(ADMIN_WHITELIST);
      console.log('✅ Whitelist configuration verified');
    });
  });

  test('Step 2: Re-enable Cerberus framework', async () => {
    console.log('\n🔧 Break Glass Recovery: Re-enabling Cerberus framework...');

    await test.step('Enable feature.cerberus.enabled via settings API', async () => {
      // Now that admin_whitelist is set, the settings API won't block us
      const response = await apiContext.patch('/api/v1/settings', {
        data: {
          key: 'feature.cerberus.enabled',
          value: 'true',
        },
      });

      expect(response.ok()).toBeTruthy();
      console.log('✅ Cerberus framework re-enabled');
    });

    await test.step('Verify Cerberus is enabled', async () => {
      const response = await apiContext.get('/api/v1/security/status');
      expect(response.ok()).toBeTruthy();

      const body = await response.json();
      expect(body.cerberus.enabled).toBe(true); // feature.cerberus.enabled = true
      console.log('✅ Cerberus framework status verified: ENABLED');
    });
  });

  test('Step 3: Enable all security modules (bypassed by whitelist)', async () => {
    console.log('\n🔧 Break Glass Recovery: Enabling all security modules...');

    // Enable ACL
    await test.step('Enable ACL module', async () => {
      const response = await apiContext.patch('/api/v1/security/acl', {
        data: { enabled: true },
      });
      expect(response.ok()).toBeTruthy();
      console.log('✅ ACL module enabled');
    });

    // Enable WAF
    await test.step('Enable WAF module', async () => {
      const response = await apiContext.patch('/api/v1/security/waf', {
        data: { enabled: true },
      });
      expect(response.ok()).toBeTruthy();
      console.log('✅ WAF module enabled');
    });

    // Enable Rate Limiting
    await test.step('Enable Rate Limiting module', async () => {
      const response = await apiContext.patch('/api/v1/security/rate-limit', {
        data: { enabled: true },
      });
      expect(response.ok()).toBeTruthy();
      console.log('✅ Rate Limiting module enabled');
    });

    // Enable CrowdSec (may not be running in E2E, but enable the setting)
    await test.step('Enable CrowdSec module', async () => {
      const response = await apiContext.patch('/api/v1/security/crowdsec', {
        data: { enabled: true },
      });

      // CrowdSec may not be running in E2E environment, so we allow failure here
      if (response.ok()) {
        console.log('✅ CrowdSec module enabled');
      } else {
        console.log('⚠️  CrowdSec not available in E2E (expected)');
      }
    });
  });

  test('Step 4: Verify full security stack is enabled with whitelist bypass', async () => {
    console.log('\n🔍 Break Glass Recovery: Verifying final state...');

    await test.step('Verify all security modules are enabled', async () => {
      const body = await getSecurityStatus(apiContext);

      // Cerberus framework
      expect(body.cerberus.enabled).toBe(true);

      // Security modules
      expect(body.acl?.enabled).toBe(true);
      expect(body.waf?.enabled).toBe(true);
      expect(body.rate_limit?.enabled).toBe(true);

      // CrowdSec may or may not be running
      console.log(`   Cerberus: ${body.cerberus.enabled ? '✅ ENABLED' : '❌ DISABLED'}`);
      console.log(`   ACL:      ${body.acl?.enabled ? '✅ ENABLED' : '❌ DISABLED'}`);
      console.log(`   WAF:      ${body.waf?.enabled ? '✅ ENABLED' : '❌ DISABLED'}`);
      console.log(`   Rate Lim: ${body.rate_limit?.enabled ? '✅ ENABLED' : '❌ DISABLED'}`);
      console.log(`   CrowdSec: ${body.crowdsec?.running ? '✅ RUNNING' : '⚠️  Not Available'}`);
    });

    await test.step('Verify admin whitelist is set to test-runner CIDRs', async () => {
      const maxRetries = 5;
      const retryDelayMs = 1000;
      let response = await apiContext.get('/api/v1/security/config');

      for (let attempt = 0; attempt < maxRetries && response.status() === 429; attempt += 1) {
        await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
        response = await apiContext.get('/api/v1/security/config');
      }

      expect(response.ok()).toBeTruthy();

      const body = await response.json();
      // API wraps config in a "config" key
      expect(body.config?.admin_whitelist).toBe(ADMIN_WHITELIST);

      console.log('✅ Admin whitelist confirmed for test-runner CIDRs');
    });

    await test.step('Verify requests bypass security (whitelist working)', async () => {
      // Make a request that would normally be blocked by ACL
      // Since our IP is in the test-runner whitelist, it should succeed
      const maxRetries = 5;
      const retryDelayMs = 1000;
      let response = await apiContext.get('/api/v1/proxy-hosts');

      for (let attempt = 0; attempt < maxRetries && !response.ok(); attempt += 1) {
        await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
        response = await apiContext.get('/api/v1/proxy-hosts');
      }

      expect(response.ok()).toBeTruthy();

      console.log('✅ Request bypassed security via admin whitelist');
    });

    console.log('\n✅ Break Glass Recovery COMPLETE');
    console.log('   State: Cerberus ON + All modules ON + test-runner whitelist bypass');
    console.log('   Ready: Browser UI tests can now test toggles/navigation safely');
  });
});
