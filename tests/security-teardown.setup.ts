/**
 * Security Teardown Setup
 *
 * This file runs AFTER all security-tests complete (including break glass recovery).
 *
 * NEW APPROACH (Universal Admin Whitelist Bypass):
 * - zzzz-break-glass-recovery.spec.ts sets admin_whitelist to 0.0.0.0/0
 * - This bypasses ALL security checks for ANY IP (CI-friendly)
 * - Cerberus framework and ALL modules are left ENABLED
 * - Browser tests run with full security stack but bypassed via whitelist
 *
 * This teardown now serves as a VERIFICATION step only - it checks that the expected
 * state is set and logs any issues. It does NOT modify configuration.
 *
 * Expected State After Break Glass Recovery:
 *   - Cerberus framework: ENABLED (toggles/buttons work)
 *   - Security modules: ENABLED (ACL, WAF, Rate Limit)
 *   - Admin whitelist: 0.0.0.0/0 (universal bypass for all IPs)
 *
 * @see /projects/Charon/tests/security-enforcement/zzzz-break-glass-recovery.spec.ts
 * @see /projects/Charon/docs/plans/e2e-test-triage-plan.md
 */

import { test as teardown } from '@bgotink/playwright-coverage';
import { request } from '@playwright/test';
import { STORAGE_STATE } from './constants';

teardown('verify-security-state-for-ui-tests', async () => {
  console.log('\n🔍 Security Teardown: Verifying state for UI tests...');
  console.log('   Expected: Cerberus ON + All modules ON + Universal bypass (0.0.0.0/0)');

  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

  // Create authenticated request context with storage state
  const requestContext = await request.newContext({
    baseURL,
    storageState: STORAGE_STATE,
  });

  let allChecksPass = true;

  try {
    // Verify Cerberus framework is enabled via status endpoint
    const statusResponse = await requestContext.get(`${baseURL}/api/v1/security/status`);
    if (statusResponse.ok()) {
      const status = await statusResponse.json();
      if (status.cerberus.enabled === true) {
        console.log('✅ Cerberus framework: ENABLED');
      } else {
        console.log('⚠️  Cerberus framework: DISABLED (expected: ENABLED)');
        allChecksPass = false;
      }

      // Verify security modules status
      console.log(`   ACL module:         ${status.acl?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
      console.log(`   WAF module:         ${status.waf?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
      console.log(`   Rate Limit module:  ${status.rate_limit?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
      console.log(`   CrowdSec module:    ${status.crowdsec?.running ? '✅ RUNNING' : '⚠️  not available (OK for E2E)'}`);

      // ACL, WAF, and Rate Limit should be enabled
      if (!status.acl?.enabled || !status.waf?.enabled || !status.rate_limit?.enabled) {
        console.log('⚠️  Some security modules are disabled (expected: all enabled)');
        allChecksPass = false;
      }
    } else {
      console.log('⚠️  Could not verify security module status');
      allChecksPass = false;
    }

    // Verify admin whitelist via config endpoint
    const configResponse = await requestContext.get(`${baseURL}/api/v1/security/config`);
    if (configResponse.ok()) {
      const configData = await configResponse.json();
      if (configData.config?.admin_whitelist === '0.0.0.0/0') {
        console.log('✅ Admin whitelist: 0.0.0.0/0 (universal bypass)');
      } else {
        console.log(`⚠️  Admin whitelist: ${configData.config?.admin_whitelist || 'none'} (expected: 0.0.0.0/0)`);
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
      console.log('   Expected state: Cerberus ON + All modules ON + Universal bypass (0.0.0.0/0)');
    }
  } catch (error) {
    console.error('Error verifying security state:', error);
    throw new Error('Security teardown verification failed');
  } finally {
    await requestContext.dispose();
  }
});
