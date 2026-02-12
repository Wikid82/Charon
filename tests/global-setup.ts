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
import { dirname } from 'path';
import { TestDataManager } from './utils/TestDataManager';
import { STORAGE_STATE } from './constants';

// Singleton to prevent duplicate validation across workers
let tokenValidated = false;

/**
 * Validate emergency token is properly configured for E2E tests
 * This is a fail-fast check to prevent cascading test failures
 */
function validateEmergencyToken(): void {
  if (tokenValidated) {
    console.log('  ✅ Emergency token already validated (singleton)');
    return;
  }

  const token = process.env.CHARON_EMERGENCY_TOKEN;
  const errors: string[] = [];

  // Check 1: Token exists
  if (!token) {
    errors.push(
      '❌ CHARON_EMERGENCY_TOKEN is not set.\n' +
      '   Generate with: openssl rand -hex 32\n' +
      '   Add to .env file or set as environment variable'
    );
  } else {
    // Mask token for logging (show first 8 chars only)
    const maskedToken = token.slice(0, 8) + '...' + token.slice(-4);
    console.log(`  🔑 Token present: ${maskedToken}`);

    // Check 2: Token length (must be at least 64 chars)
    if (token.length < 64) {
      errors.push(
        `❌ CHARON_EMERGENCY_TOKEN is too short (${token.length} chars, minimum 64).\n` +
        '   Generate a new one with: openssl rand -hex 32'
      );
    } else {
      console.log(`  ✓ Token length: ${token.length} chars (valid)`);
    }

    // Check 3: Token is hex format (a-f0-9)
    const hexPattern = /^[a-f0-9]+$/i;
    if (!hexPattern.test(token)) {
      errors.push(
        '❌ CHARON_EMERGENCY_TOKEN must be hexadecimal (0-9, a-f).\n' +
        '   Generate with: openssl rand -hex 32'
      );
    } else {
      console.log('  ✓ Token format: Valid hexadecimal');
    }

    // Check 4: Token entropy (avoid placeholder values)
    const commonPlaceholders = [
      'test-emergency-token',
      'your_64_character',
      'replace_this',
      '0000000000000000',
      'ffffffffffffffff',
    ];
    const isPlaceholder = commonPlaceholders.some(ph => token.toLowerCase().includes(ph));
    if (isPlaceholder) {
      errors.push(
        '❌ CHARON_EMERGENCY_TOKEN appears to be a placeholder value.\n' +
        '   Generate a unique token with: openssl rand -hex 32'
      );
    } else {
      console.log('  ✓ Token appears to be unique (not a placeholder)');
    }
  }

  // Fail fast if validation errors found
  if (errors.length > 0) {
    console.error('\n🚨 Emergency Token Configuration Errors:\n');
    errors.forEach(error => console.error(error + '\n'));
    console.error('📖 See .env.example and docs/getting-started.md for setup instructions.\n');
    process.exit(1);
  }

  console.log('✅ Emergency token validation passed\n');
  tokenValidated = true;
}

/**
 * Get the base URL for the application
 * CRITICAL: Must use localhost (not 127.0.0.1) for cookie domain consistency
 */
function getBaseURL(): string {
  return process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
}

/**
 * Check if Caddy admin API is enabled and healthy (port 2019 - read-only config inspection)
 */
async function checkCaddyAdminHealth(): Promise<boolean> {
  const caddyAdminHost = process.env.CADDY_ADMIN_HOST || 'http://localhost:2019';
  const startTime = Date.now();
  console.log(`🔍 Checking Caddy admin API health at ${caddyAdminHost}...`);

  const caddyContext = await request.newContext({ baseURL: caddyAdminHost });
  try {
    const response = await caddyContext.get('/config', { timeout: 3000 });
    const elapsed = Date.now() - startTime;

    if (response.ok()) {
      console.log(`  ✅ Caddy admin API (port 2019) is healthy [${elapsed}ms]`);
      return true;
    } else {
      console.log(`  ⚠️  Caddy admin API returned: ${response.status()} [${elapsed}ms]`);
      return false;
    }
  } catch (e) {
    const elapsed = Date.now() - startTime;
    console.log(`  ⏭️  Caddy admin API unavailable (non-blocking) [${elapsed}ms]`);
    return false;
  } finally {
    await caddyContext.dispose();
  }
}

/**
 * Wait for container to be ready before running global setup.
 * This prevents 401 errors when global-setup runs before containers finish starting.
 */
async function waitForContainer(maxRetries = 15, delayMs = 2000): Promise<void> {
  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
  console.log(`⏳ Waiting for container to be ready at ${baseURL}...`);

  for (let i = 0; i < maxRetries; i++) {
    try {
      const context = await request.newContext({ baseURL });
      const response = await context.get('/api/v1/health', { timeout: 3000 });
      await context.dispose();

      if (response.ok()) {
        console.log(`  ✅ Container ready after ${i + 1} attempt(s) [${(i + 1) * delayMs}ms]`);
        return;
      }
    } catch (error) {
      console.log(`  ⏳ Waiting for container... (${i + 1}/${maxRetries})`);
      if (i < maxRetries - 1) {
        await new Promise(resolve => setTimeout(resolve, delayMs));
      }
    }
  }
  throw new Error(`Container failed to start after ${maxRetries * delayMs}ms`);
}

/**
 * Check if emergency tier-2 server is enabled and healthy (port 2020 - break-glass with auth)
 */
async function checkEmergencyServerHealth(): Promise<boolean> {
  const emergencyHost = process.env.EMERGENCY_SERVER_HOST || 'http://localhost:2020';
  const startTime = Date.now();
  console.log(`🔍 Checking emergency tier-2 server health at ${emergencyHost}...`);

  const emergencyContext = await request.newContext({ baseURL: emergencyHost });
  try {
    const response = await emergencyContext.get('/health', { timeout: 3000 });
    const elapsed = Date.now() - startTime;

    if (response.ok()) {
      console.log(`  ✅ Emergency tier-2 server (port 2020) is healthy [${elapsed}ms]`);
      return true;
    } else {
      console.log(`  ⚠️  Emergency tier-2 server returned: ${response.status()} [${elapsed}ms]`);
      return false;
    }
  } catch (e) {
    const elapsed = Date.now() - startTime;
    console.log(`  ⏭️  Emergency tier-2 server unavailable (tests will skip tier-2 features) [${elapsed}ms]`);
    return false;
  } finally {
    await emergencyContext.dispose();
  }
}

async function globalSetup(): Promise<void> {
  console.log('\n🧹 Running global test setup...\n');
  const setupStartTime = Date.now();

  // CRITICAL: Validate emergency token before proceeding
  console.log('🔐 Validating emergency token configuration...');
  validateEmergencyToken();

  const baseURL = getBaseURL();
  console.log(`📍 Base URL: ${baseURL}`);

  // CRITICAL: Wait for container to be ready before proceeding
  // This prevents 401 errors when containers are still starting up
  await waitForContainer();

  // Log URL analysis for IPv4 vs IPv6 debugging
  try {
    const parsedURL = new URL(baseURL);
    const isIPv6 = parsedURL.hostname.includes(':') || parsedURL.hostname.startsWith('[');
    const isLocalhost = parsedURL.hostname === 'localhost';
    const port = parsedURL.port || (parsedURL.protocol === 'https:' ? '443' : '80');

    console.log(`   └─ Hostname: ${parsedURL.hostname}`);
    console.log(`   ├─ Port: ${port}`);
    console.log(`   ├─ Protocol: ${parsedURL.protocol}`);
    console.log(`   ├─ IPv6: ${isIPv6 ? 'Yes' : 'No'}`);
    console.log(`   └─ Localhost: ${isLocalhost ? 'Yes' : 'No'}\n`);
  } catch (e) {
    console.log('   ⚠️  Could not parse base URL\n');
  }

  // Health-check Caddy admin and emergency tier-2 servers (non-blocking)
  console.log('📊 Port Connectivity Checks:');
  const caddyHealthy = await checkCaddyAdminHealth();
  const emergencyHealthy = await checkEmergencyServerHealth();

  console.log(
    `\n✅ Connectivity Summary: Caddy=${caddyHealthy ? '✓' : '✗'} Emergency=${emergencyHealthy ? '✓' : '✗'}\n`
  );


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
    const authDir = dirname(STORAGE_STATE);
    console.log(`⏭️  Skipping authenticated security reset (no auth state file at ${STORAGE_STATE})`);
    console.log(`   └─ Auth dir exists: ${existsSync(authDir) ? 'Yes' : 'No'} (${authDir})`);
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
 * USES THE CORRECT ENDPOINT: /emergency/security-reset (on port 2020)
 * This endpoint bypasses all security checks when a valid emergency token is provided.
 */
async function emergencySecurityReset(requestContext: APIRequestContext): Promise<void> {
  const startTime = Date.now();
  console.log('🔓 Performing emergency security reset...');

  const emergencyToken = process.env.CHARON_EMERGENCY_TOKEN;
  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

  if (!emergencyToken) {
    console.warn('  ⚠️  CHARON_EMERGENCY_TOKEN not set, skipping emergency reset');
    return;
  }

  // Debug logging to troubleshoot 401 errors
  const maskedToken = emergencyToken.slice(0, 8) + '...' + emergencyToken.slice(-4);
  console.log(`  🔑 Token configured: ${maskedToken} (${emergencyToken.length} chars)`);

  try {
    // Create new context for emergency server on port 2020 with basic auth
    const emergencyURL = baseURL.replace(':8080', ':2020');
    console.log(`  📍 Emergency URL: ${emergencyURL}/emergency/security-reset`);

    const emergencyContext = await request.newContext({
      baseURL: emergencyURL,
      httpCredentials: {
        username: process.env.CHARON_EMERGENCY_USERNAME || 'admin',
        password: process.env.CHARON_EMERGENCY_PASSWORD || 'changeme',
      },
    });

    // Use the CORRECT endpoint: /emergency/security-reset
    // This endpoint bypasses ACL, WAF, and all security checks
    const response = await emergencyContext.post('/emergency/security-reset', {
      headers: {
        'X-Emergency-Token': emergencyToken,
        'Content-Type': 'application/json',
      },
      data: { reason: 'Global setup - reset all modules for clean test state' },
      timeout: 5000, // 5s timeout to prevent hanging
    });

    const elapsed = Date.now() - startTime;
    console.log(`  📊 Emergency reset status: ${response.status()} [${elapsed}ms]`);

    if (!response.ok()) {
      const body = await response.text();
      console.error(`  ❌ Emergency reset failed: ${response.status()}`);
      console.error(`  📄 Response body: ${body}`);
      throw new Error(`Emergency reset returned ${response.status()}: ${body}`);
    }

    const result = await response.json();
    console.log(`  ✅ Emergency reset successful [${elapsed}ms]`);
    if (result.disabled_modules && Array.isArray(result.disabled_modules)) {
      console.log(`  ✓ Disabled modules: ${result.disabled_modules.join(', ')}`);
    }

    await emergencyContext.dispose();

    // Reduced wait time - fresh containers don't need long propagation
    console.log('  ⏳ Waiting for security reset to propagate...');
    await new Promise(resolve => setTimeout(resolve, 500));
  } catch (e) {
    const elapsed = Date.now() - startTime;
    console.error(`  ❌ Emergency reset error: ${e instanceof Error ? e.message : String(e)} [${elapsed}ms]`);
    throw e;
  }

  const totalTime = Date.now() - startTime;
  console.log(`  ✅ Security reset complete [${totalTime}ms]`);
}

export default globalSetup;
