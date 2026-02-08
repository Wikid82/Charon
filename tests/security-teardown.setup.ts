/**
 * Security Teardown Setup
 *
 * This file runs AFTER all security-tests complete (including break glass recovery).
 *
 * NEW APPROACH (Universal Admin Whitelist Bypass):
 * - zzzz-break-glass-recovery.spec.ts sets admin_whitelist to test-runner CIDRs
 * - This bypasses ALL security checks for ANY IP (CI-friendly)
 * - Cerberus framework and ALL modules are left ENABLED
 * - Browser tests run with full security stack but bypassed via whitelist
 *
 * This teardown verifies the expected state and restores it if needed.
 *
 * Expected State After Break Glass Recovery:
 *   - Cerberus framework: ENABLED (toggles/buttons work)
 *   - Security modules: ENABLED (ACL, WAF, Rate Limit)
 *   - Admin whitelist: test-runner CIDRs (local/private ranges)
 *
 * @see /projects/Charon/tests/security-enforcement/zzzz-break-glass-recovery.spec.ts
 * @see /projects/Charon/docs/plans/e2e-test-triage-plan.md
 */

import { test as teardown } from './fixtures/test';
import { request } from '@playwright/test';
import { STORAGE_STATE } from './constants';

teardown('verify-security-state-for-ui-tests', async () => {
  console.log('\n🔍 Security Teardown: Verifying state for UI tests...');
  console.log('   Expected: Cerberus ON + All modules ON + test-runner whitelist bypass');

  const adminWhitelist = '127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16';

  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';

  // Create authenticated request context with storage state
  const requestContext = await request.newContext({
    baseURL,
    storageState: STORAGE_STATE,
  });

  let allChecksPass = true;

  const patchWithRetry = async (url: string, data: Record<string, unknown>) => {
    const maxRetries = 5;
    const retryDelayMs = 1000;

    for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
      const response = await requestContext.patch(url, { data });

      if (response.ok()) {
        return;
      }

      if (response.status() !== 429 || attempt === maxRetries) {
        throw new Error(`PATCH ${url} failed: ${response.status()} ${await response.text()}`);
      }

      await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
    }
  };

  const enableModuleWithRetry = async (url: string, label: string) => {
    try {
      await patchWithRetry(url, { enabled: true });
      console.log(`✅ ${label} module enabled`);
    } catch (error) {
      console.log(`⚠️  ${label} module enable failed: ${String(error)}`);
      allChecksPass = false;
    }
  };

  try {
    // Ensure admin whitelist is set before enabling Cerberus/modules
    const configResponse = await requestContext.get(`${baseURL}/api/v1/security/config`);
    if (configResponse.ok()) {
      const configData = await configResponse.json();
      if (configData.config?.admin_whitelist !== adminWhitelist) {
        await patchWithRetry(`${baseURL}/api/v1/config`, {
          security: { admin_whitelist: adminWhitelist },
        });
        console.log('✅ Admin whitelist set to test-runner CIDRs');
      }
    } else {
      console.log('⚠️  Could not read admin whitelist configuration');
      allChecksPass = false;
    }

    // Verify Cerberus framework is enabled via status endpoint
    const statusResponse = await requestContext.get(`${baseURL}/api/v1/security/status`);
    if (statusResponse.ok()) {
      const status = await statusResponse.json();
      if (status.cerberus.enabled === true) {
        console.log('✅ Cerberus framework: ENABLED');
      } else {
        console.log('⚠️  Cerberus framework: DISABLED (expected: ENABLED)');
        await patchWithRetry(`${baseURL}/api/v1/settings`, {
          key: 'feature.cerberus.enabled',
          value: 'true',
        });
        console.log('✅ Cerberus framework re-enabled');
      }

      // Verify security modules status
      console.log(`   ACL module:         ${status.acl?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
      console.log(`   WAF module:         ${status.waf?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
      console.log(`   Rate Limit module:  ${status.rate_limit?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
      console.log(`   CrowdSec module:    ${status.crowdsec?.running ? '✅ RUNNING' : '⚠️  not available (OK for E2E)'}`);

      // ACL, WAF, and Rate Limit should be enabled
      if (!status.acl?.enabled) {
        await enableModuleWithRetry(`${baseURL}/api/v1/security/acl`, 'ACL');
      }
      if (!status.waf?.enabled) {
        await enableModuleWithRetry(`${baseURL}/api/v1/security/waf`, 'WAF');
      }
      if (!status.rate_limit?.enabled) {
        await enableModuleWithRetry(`${baseURL}/api/v1/security/rate-limit`, 'Rate Limit');
      }
    } else {
      console.log('⚠️  Could not verify security module status');
      allChecksPass = false;
    }

    // Re-check admin whitelist after any updates
    const verifiedConfig = await requestContext.get(`${baseURL}/api/v1/security/config`);
    if (verifiedConfig.ok()) {
      const verifiedData = await verifiedConfig.json();
      if (verifiedData.config?.admin_whitelist === adminWhitelist) {
        console.log('✅ Admin whitelist: test-runner CIDRs');
      } else {
        console.log(`⚠️  Admin whitelist: ${verifiedData.config?.admin_whitelist || 'none'} (expected: test-runner CIDRs)`);
        allChecksPass = false;
      }
    } else {
      console.log('⚠️  Could not verify admin whitelist configuration');
      allChecksPass = false;
    }

    if (allChecksPass) {
      console.log('\n✅ Security Teardown COMPLETE: State verified for UI tests');
      console.log('   Browser tests can now safely test toggles/navigation');
    } else {
      console.log('\n⚠️  Security Teardown: Some checks failed (see warnings above)');
      console.log('   UI tests may encounter issues if configuration is incorrect');
      console.log('   Expected state: Cerberus ON + All modules ON + test-runner whitelist bypass');
    }
  } catch (error) {
    console.error('Error verifying security state:', error);
    throw new Error('Security teardown verification failed');
  } finally {
    await requestContext.dispose();
  }
});
