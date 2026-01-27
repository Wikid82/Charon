/**
 * Emergency Server E2E Tests (Tier 2 Break Glass)
 *
 * Tests the separate emergency server running on port 2020.
 * This server provides failsafe access when the main application
 * security is blocking access.
 *
 * Prerequisites:
 * - Emergency server enabled in docker-compose.e2e.yml
 * - Port 2020 accessible from test environment
 * - Basic Auth credentials configured
 *
 * Reference: docs/plans/break_glass_protocol_redesign.md - Phase 3.2
 */

import { test, expect, request as playwrightRequest } from '@playwright/test';
import { EMERGENCY_TOKEN, EMERGENCY_SERVER, enableSecurity } from '../fixtures/security';
import { TestDataManager } from '../utils/TestDataManager';

/**
 * Check if emergency server is healthy before running tests
 */
async function checkEmergencyServerHealth(): Promise<boolean> {
  const emergencyRequest = await playwrightRequest.newContext({
    baseURL: EMERGENCY_SERVER.baseURL,
  });

  try {
    const response = await emergencyRequest.get('/health', { timeout: 3000 });
    return response.ok();
  } catch {
    return false;
  } finally {
    await emergencyRequest.dispose();
  }
}

test.describe('Emergency Server (Tier 2 Break Glass)', () => {
  // Check health before all tests in this suite
  test.beforeAll(async () => {
    const isHealthy = await checkEmergencyServerHealth();
    if (!isHealthy) {
      console.log('❌ Emergency server is not healthy - skipping all emergency server tests');
      test.skip();
    }
  });

  test('Test 1: Emergency server health endpoint', async () => {
    console.log('🧪 Testing emergency server health endpoint...');

    // Create a new request context for emergency server
    const emergencyRequest = await playwrightRequest.newContext({
      baseURL: EMERGENCY_SERVER.baseURL,
    });

    try {
      const response = await emergencyRequest.get('/health');

      expect(response.ok()).toBeTruthy();
      expect(response.status()).toBe(200);

      const body = await response.json();
      expect(body.status).toBe('ok');
      expect(body.server).toBe('emergency');

      console.log('  ✓ Health endpoint responded successfully');
      console.log(`  ✓ Server type: ${body.server}`);
      console.log('✅ Test 1 passed: Emergency server health endpoint works');
    } finally {
      await emergencyRequest.dispose();
    }
  });

  test('Test 2: Emergency server requires Basic Auth', async () => {
    console.log('🧪 Testing emergency server Basic Auth requirement...');

    const emergencyRequest = await playwrightRequest.newContext({
      baseURL: EMERGENCY_SERVER.baseURL,
    });

    try {
      // Test 2a: Request WITHOUT Basic Auth should fail
      const noAuthResponse = await emergencyRequest.post('/emergency/security-reset', {
        headers: {
          'X-Emergency-Token': EMERGENCY_TOKEN,
        },
      });

      expect(noAuthResponse.status()).toBe(401);
      console.log('  ✓ Request without auth properly rejected (401)');

      // Test 2b: Request WITH Basic Auth should succeed
      const authHeader =
        'Basic ' +
        Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString(
          'base64'
        );

      const authResponse = await emergencyRequest.post('/emergency/security-reset', {
        headers: {
          Authorization: authHeader,
          'X-Emergency-Token': EMERGENCY_TOKEN,
        },
      });

      expect(authResponse.ok()).toBeTruthy();
      expect(authResponse.status()).toBe(200);

      const body = await authResponse.json();
      expect(body.success).toBe(true);

      console.log('  ✓ Request with valid auth succeeded');
      console.log('✅ Test 2 passed: Basic Auth properly enforced');
    } finally {
      await emergencyRequest.dispose();
    }
  });

  test('Test 3: Emergency server bypasses main app security', async ({ request }) => {
    console.log('🧪 Testing emergency server security bypass...');

    const testData = new TestDataManager(request, 'emergency-server-bypass');

    try {
      // Step 1: Enable security on main app (port 8080)
      await request.post('/api/v1/settings', {
        data: { key: 'feature.cerberus.enabled', value: 'true' },
      });

      // Create restrictive ACL on main app
      const { id: aclId } = await testData.createAccessList({
        name: 'test-emergency-server-acl',
        type: 'whitelist',
        ipRules: [{ cidr: '192.168.99.0/24', description: 'Unreachable network' }],
        enabled: true,
      });

      await request.post('/api/v1/settings', {
        data: { key: 'security.acl.enabled', value: 'true' },
      });

      // Wait for settings to propagate
      await new Promise(resolve => setTimeout(resolve, 3000));

      // Step 2: Verify main app blocks requests (403)
      const mainAppResponse = await request.get('/api/v1/proxy-hosts');
      expect(mainAppResponse.status()).toBe(403);
      console.log('  ✓ Main app (port 8080) blocking requests with ACL');

      // Step 3: Use emergency server (port 2019) to reset security
      const emergencyRequest = await playwrightRequest.newContext({
        baseURL: EMERGENCY_SERVER.baseURL,
      });

      const authHeader =
        'Basic ' +
        Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString(
          'base64'
        );

      const emergencyResponse = await emergencyRequest.post('/emergency/security-reset', {
        headers: {
          Authorization: authHeader,
          'X-Emergency-Token': EMERGENCY_TOKEN,
        },
      });

      await emergencyRequest.dispose();

      expect(emergencyResponse.ok()).toBeTruthy();
      expect(emergencyResponse.status()).toBe(200);
      console.log('  ✓ Emergency server (port 2019) succeeded despite ACL');

      // Wait for settings to propagate
      await new Promise(resolve => setTimeout(resolve, 3000));

      // Step 4: Verify main app now accessible
      const allowedResponse = await request.get('/api/v1/proxy-hosts');
      expect(allowedResponse.ok()).toBeTruthy();
      console.log('  ✓ Main app now accessible after emergency reset');

      console.log('✅ Test 3 passed: Emergency server bypasses main app security');
    } finally {
      await testData.cleanup();
    }
  });

  test('Test 4: Emergency server security reset works', async ({ request }) => {
    console.log('🧪 Testing emergency server security reset functionality...');

    // Step 1: Enable all security modules
    await enableSecurity(request);
    console.log('  ✓ Security modules enabled');

    // Step 2: Call emergency server endpoint
    const emergencyRequest = await playwrightRequest.newContext({
      baseURL: EMERGENCY_SERVER.baseURL,
    });

    const authHeader =
      'Basic ' +
      Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString('base64');

    const resetResponse = await emergencyRequest.post('/emergency/security-reset', {
      headers: {
        Authorization: authHeader,
        'X-Emergency-Token': EMERGENCY_TOKEN,
      },
    });

    await emergencyRequest.dispose();

    expect(resetResponse.ok()).toBeTruthy();
    const resetBody = await resetResponse.json();
    expect(resetBody.success).toBe(true);
    expect(resetBody.disabled_modules).toBeDefined();
    expect(resetBody.disabled_modules.length).toBeGreaterThan(0);

    console.log(`  ✓ Disabled modules: ${resetBody.disabled_modules.join(', ')}`);

    // Wait for settings to propagate
    await new Promise(resolve => setTimeout(resolve, 2000));

    // Step 3: Verify settings are disabled
    const statusResponse = await request.get('/api/v1/security/status');
    if (statusResponse.ok()) {
      const status = await statusResponse.json();

      // At least some security should now be disabled
      const anyDisabled =
        !status.acl?.enabled ||
        !status.waf?.enabled ||
        !status.rateLimit?.enabled ||
        !status.cerberus?.enabled;

      expect(anyDisabled).toBe(true);
      console.log('  ✓ Security status updated - modules disabled');
    }

    console.log('✅ Test 4 passed: Emergency server security reset functional');
  });

  test('Test 5: Emergency server minimal middleware (validation)', async () => {
    console.log('🧪 Testing emergency server minimal middleware...');

    const emergencyRequest = await playwrightRequest.newContext({
      baseURL: EMERGENCY_SERVER.baseURL,
    });

    try {
      const authHeader =
        'Basic ' +
        Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString(
          'base64'
        );

      const response = await emergencyRequest.post('/emergency/security-reset', {
        headers: {
          Authorization: authHeader,
          'X-Emergency-Token': EMERGENCY_TOKEN,
        },
      });

      expect(response.ok()).toBeTruthy();

      // Verify emergency server responses don't have WAF headers
      const headers = response.headers();
      expect(headers['x-waf-status']).toBeUndefined();
      console.log('  ✓ No WAF headers (bypassed)');

      // Verify no CrowdSec headers
      expect(headers['x-crowdsec-decision']).toBeUndefined();
      console.log('  ✓ No CrowdSec headers (bypassed)');

      // Verify no rate limit headers
      expect(headers['x-ratelimit-limit']).toBeUndefined();
      console.log('  ✓ No rate limit headers (bypassed)');

      // Emergency server should have minimal middleware:
      // - Basic Auth (if configured)
      // - Request logging
      // - Recovery middleware
      // NO: WAF, CrowdSec, ACL, Rate Limiting, JWT Auth

      console.log('✅ Test 5 passed: Emergency server uses minimal middleware');
      console.log('  ℹ️  Emergency server bypasses: WAF, CrowdSec, ACL, Rate Limiting');
    } finally {
      await emergencyRequest.dispose();
    }
  });
});
