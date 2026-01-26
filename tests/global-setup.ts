/**
 * Global Setup - Runs once before all tests
 *
 * This setup ensures a clean test environment by:
 * 1. Cleaning up any orphaned test data from previous runs
 * 2. Verifying the application is accessible
 * 3. Performing emergency ACL reset to prevent deadlock from previous failed runs
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

async function globalSetup(): Promise<void> {
  console.log('\n🧹 Running global test setup...');

  const baseURL = getBaseURL();
  console.log(`📍 Base URL: ${baseURL}`);

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
    } catch (error) {
      console.warn('⚠️ Authenticated security reset failed:', error);
    }
    await authenticatedContext.dispose();
  } else {
    console.log('⏭️  Skipping authenticated security reset (no auth state file)');
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
