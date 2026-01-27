/**
 * Global Setup - Runs once before all tests
 *
 * This setup ensures a clean test environment by:
 * 1. Cleaning up any orphaned test data from previous runs
 * 2. Verifying the application is accessible
 * 3. Performing emergency ACL reset to prevent deadlock from previous failed runs
 * 4. Health-checking emergency server (tier 2) and admin endpoint
 */

import { request, APIRequestContext } from '@playwright/test';
import { existsSync } from 'fs';
import { TestDataManager } from './utils/TestDataManager';
import { STORAGE_STATE } from './constants';

/**
 * Get the base URL for the application
 */
function getBaseURL(): string {
  return process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
}

/**
 * Check if Caddy admin API is enabled and healthy (port 2019 - read-only config inspection)
 */
async function checkCaddyAdminHealth(): Promise<boolean> {
  const caddyAdminHost = process.env.CADDY_ADMIN_HOST || 'http://localhost:2019';
  console.log(`🔍 Checking Caddy admin API health at ${caddyAdminHost}...`);

  const caddyContext = await request.newContext({ baseURL: caddyAdminHost });
  try {
    const response = await caddyContext.get('/config', { timeout: 3000 });
    if (response.ok()) {
      console.log('  ✅ Caddy admin API (port 2019) is healthy');
      return true;
    } else {
      console.log(`  ⚠️  Caddy admin API returned: ${response.status()}`);
      return false;
    }
  } catch (e) {
    console.log('  ⏭️  Caddy admin API unavailable (non-blocking)');
    return false;
  } finally {
    await caddyContext.dispose();
  }
}

/**
 * Check if emergency tier-2 server is enabled and healthy (port 2020 - break-glass with auth)
 */
async function checkEmergencyServerHealth(): Promise<boolean> {
  const emergencyHost = process.env.EMERGENCY_SERVER_HOST || 'http://localhost:2020';
  console.log(`🔍 Checking emergency tier-2 server health at ${emergencyHost}...`);

  const emergencyContext = await request.newContext({ baseURL: emergencyHost });
  try {
    const response = await emergencyContext.get('/health', { timeout: 3000 });
    if (response.ok()) {
      console.log('  ✅ Emergency tier-2 server (port 2020) is healthy');
      return true;
    } else {
      console.log(`  ⚠️  Emergency tier-2 server returned: ${response.status()}`);
      return false;
    }
  } catch (e) {
    console.log('  ⏭️  Emergency tier-2 server unavailable (tests will skip tier-2 features)');
    return false;
  } finally {
    await emergencyContext.dispose();
  }
}

async function globalSetup(): Promise<void> {
  console.log('\n🧹 Running global test setup...');

  const baseURL = getBaseURL();
  console.log(`📍 Base URL: ${baseURL}`);

  // Log URL analysis for IPv4 vs IPv6 debugging
  try {
    const parsedURL = new URL(baseURL);
    const isIPv6 = parsedURL.hostname.includes(':') || parsedURL.hostname.startsWith('[');
    const isLocalhost = parsedURL.hostname === 'localhost';
    console.log(`  🔍 URL Analysis: host=${parsedURL.hostname} port=${parsedURL.port} IPv6=${isIPv6} localhost=${isLocalhost}`);
  } catch (e) {
    console.log('  ⚠️  Could not parse base URL');
  }

  // Health-check Caddy admin and emergency tier-2 servers (non-blocking)
  await checkCaddyAdminHealth();
  await checkEmergencyServerHealth();

  // Pre-auth security reset attempt (crash protection failsafe)
  // This attempts to disable security modules BEFORE auth, in case a previous run crashed
  // with security enabled blocking the auth endpoint.
  // SKIPPED in CI when CHARON_EMERGENCY_TOKEN is not set - fresh containers don't need reset
  if (process.env.CHARON_EMERGENCY_TOKEN && process.env.CHARON_EMERGENCY_TOKEN !== 'test-emergency-token-for-e2e-32chars') {
    const preAuthContext = await request.newContext({ baseURL });
    try {
      await emergencySecurityReset(preAuthContext);
    } catch (e) {
      console.log('⏭️  Pre-auth security reset skipped (may require auth)');
    }
    await preAuthContext.dispose();
  } else {
    console.log('⏭️  Pre-auth security reset skipped (fresh container, no custom token)');
  }

  // Create a request context
  const requestContext = await request.newContext({
    baseURL,
    extraHTTPHeaders: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
  });

  try {
    // Verify the application is accessible
    console.log('🔍 Checking application health...');
    const healthResponse = await requestContext.get('/api/v1/health', {
      timeout: 10000,
    }).catch(() => null);

    if (!healthResponse || !healthResponse.ok()) {
      console.warn('⚠️  Health check failed - application may not be ready');
      // Try the base URL as fallback
      const baseResponse = await requestContext.get('/').catch(() => null);
      if (!baseResponse || !baseResponse.ok()) {
        console.error('❌ Application is not accessible at', baseURL);
        throw new Error(`Application not accessible at ${baseURL}`);
      }
    }
    console.log('✅ Application is accessible');

    // Clean up orphaned test data from previous runs
    console.log('🗑️  Cleaning up orphaned test data...');
    const cleanupResults = await TestDataManager.forceCleanupAll(requestContext);

    if (
      cleanupResults.proxyHosts > 0 ||
      cleanupResults.accessLists > 0 ||
      cleanupResults.dnsProviders > 0 ||
      cleanupResults.certificates > 0
    ) {
      console.log('   Cleaned up:');
      if (cleanupResults.proxyHosts > 0) {
        console.log(`   - ${cleanupResults.proxyHosts} proxy hosts`);
      }
      if (cleanupResults.accessLists > 0) {
        console.log(`   - ${cleanupResults.accessLists} access lists`);
      }
      if (cleanupResults.dnsProviders > 0) {
        console.log(`   - ${cleanupResults.dnsProviders} DNS providers`);
      }
      if (cleanupResults.certificates > 0) {
        console.log(`   - ${cleanupResults.certificates} certificates`);
      }
    } else {
      console.log('   No orphaned test data found');
    }

    console.log('✅ Global setup complete\n');
  } catch (error) {
    console.error('❌ Global setup failed:', error);
    throw error;
  } finally {
    await requestContext.dispose();
  }

  // Emergency security reset with auth (more complete)
  if (existsSync(STORAGE_STATE)) {
    const authenticatedContext = await request.newContext({
      baseURL,
      storageState: STORAGE_STATE,
    });
    try {
      await emergencySecurityReset(authenticatedContext);
      console.log('✓ Authenticated security reset complete');

      // Deterministic ACL disable verification
      await verifySecurityDisabled(authenticatedContext);
    } catch (error) {
      console.warn('⚠️ Authenticated security reset failed:', error);
    }
    await authenticatedContext.dispose();
  } else {
    console.log('⏭️  Skipping authenticated security reset (no auth state file)');
  }
}

/**
 * Verify that security modules (ACL, rate limiting) are disabled.
 * Retries once if still enabled, then fails fast with actionable error.
 */
async function verifySecurityDisabled(requestContext: APIRequestContext): Promise<void> {
  console.log('🔒 Verifying security modules are disabled...');

  for (let attempt = 1; attempt <= 2; attempt++) {
    try {
      const configResponse = await requestContext.get('/api/v1/security/config', { timeout: 3000 });
      if (!configResponse.ok()) {
        console.warn(`  ⚠️  Could not fetch security config (${configResponse.status()})`);
        return; // Endpoint might not exist, continue
      }

      const config = await configResponse.json();
      const aclEnabled = config.acl?.enabled === true;
      const rateLimitEnabled = config.rateLimit?.enabled === true;

      if (!aclEnabled && !rateLimitEnabled) {
        console.log('  ✅ Security modules confirmed disabled');
        return;
      }

      console.warn(`  ⚠️  Attempt ${attempt}: ACL=${aclEnabled} RateLimit=${rateLimitEnabled}`);

      if (attempt === 1) {
        // Retry emergency reset
        console.log('  🔄 Retrying emergency security reset...');
        await emergencySecurityReset(requestContext);
        await new Promise(resolve => setTimeout(resolve, 1000));
      } else {
        // Fail fast with actionable error
        throw new Error(
          `\n❌ SECURITY MODULES STILL ENABLED AFTER RESET\n` +
          `   ACL: ${aclEnabled}, Rate Limiting: ${rateLimitEnabled}\n` +
          `   This will cause test failures. Check:\n` +
          `   1. Emergency token is correct (CHARON_EMERGENCY_TOKEN)\n` +
          `   2. Emergency endpoint is working (/api/v1/emergency/security-reset)\n` +
          `   3. Settings service is applying changes correctly\n`
        );
      }
    } catch (error) {
      if (attempt === 2) {
        throw error;
      }
    }
  }
}

/**
 * Perform emergency security reset to disable ALL security modules.
 * This prevents deadlock if a previous test run left any security module enabled.
 *
 * USES THE CORRECT ENDPOINT: /api/v1/emergency/security-reset
 * This endpoint bypasses all security checks when a valid emergency token is provided.
 */
async function emergencySecurityReset(requestContext: APIRequestContext): Promise<void> {
  console.log('🔓 Performing emergency security reset...');

  const emergencyToken = process.env.CHARON_EMERGENCY_TOKEN || 'test-emergency-token-for-e2e-32chars';

  try {
    // Use the CORRECT endpoint: /api/v1/emergency/security-reset
    // This endpoint bypasses ACL, WAF, and all security checks
    const response = await requestContext.post('/api/v1/emergency/security-reset', {
      headers: {
        'X-Emergency-Token': emergencyToken,
      },
      timeout: 5000, // 5s timeout to prevent hanging
    });

    if (!response.ok()) {
      const body = await response.text();
      console.error(`  ❌ Emergency reset failed: ${response.status()} ${body}`);
      throw new Error(`Emergency reset returned ${response.status()}`);
    }

    const result = await response.json();
    console.log('  ✅ Emergency reset successful');
    console.log(`  ✅ Disabled modules: ${result.disabled_modules?.join(', ')}`);

    // Reduced wait time - fresh containers don't need long propagation
    console.log('  ⏳ Waiting for security reset to propagate...');
    await new Promise(resolve => setTimeout(resolve, 500));
  } catch (e) {
    console.error(`  ❌ Emergency reset error: ${e}`);
    throw e;
  }

  console.log('  ✅ Security reset complete');
}

export default globalSetup;
