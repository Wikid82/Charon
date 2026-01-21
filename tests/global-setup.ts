/**
 * Global Setup - Runs once before all tests
 *
 * This setup ensures a clean test environment by:
 * 1. Cleaning up any orphaned test data from previous runs
 * 2. Verifying the application is accessible
 */

import { request } from '@playwright/test';
import { TestDataManager } from './utils/TestDataManager';

/**
 * Get the base URL for the application
 */
function getBaseURL(): string {
  return process.env.PLAYWRIGHT_BASE_URL || 'http://100.98.12.109:8080';
}

async function globalSetup(): Promise<void> {
  console.log('\n🧹 Running global test setup...');

  const baseURL = getBaseURL();
  console.log(`📍 Base URL: ${baseURL}`);

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
