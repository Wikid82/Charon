/**
 * Global Setup - Runs once before all tests
 *
 * This setup ensures a clean test environment by:
 * 1. Cleaning up any orphaned test data from previous runs
 * 2. Verifying the application is accessible
 * 3. Performing base connectivity checks for test diagnostics
 */

import { request } from '@playwright/test';
import { TestDataManager } from './utils/TestDataManager';

/**
 * Get the base URL for the application
 */
function getBaseURL(): string {
  return process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
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
  const baseURL = getBaseURL();
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

async function globalSetup(): Promise<void> {
  console.log('\n🧹 Running global test setup...\n');

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

  console.log(
    `\n✅ Connectivity Summary: Caddy=${caddyHealthy ? '✓' : '✗'}\n`
  );

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
}

export default globalSetup;
