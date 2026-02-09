import { test, expect, request as playwrightRequest } from '@playwright/test';
import { EMERGENCY_TOKEN, EMERGENCY_SERVER } from '../../fixtures/security';

/**
 * Break Glass - Tier 2 (Emergency Server) Validation Tests
 *
 * These tests verify the emergency server (port 2020) works independently of the main application,
 * proving defense in depth for the break glass protocol.
 *
 * Architecture:
 * - Tier 1: Main app endpoint (/api/v1/emergency/security-reset) - goes through Caddy/CrowdSec
 * - Tier 2: Emergency server (:2020/emergency/*) - bypasses all security layers (sidecar door)
 *
 * Why this matters: If Tier 1 is blocked by ACL/WAF/CrowdSec, Tier 2 provides an independent recovery path.
 */

// Store health status in a way that persists correctly across hooks
const testState = {
  emergencyServerHealthy: undefined as boolean | undefined,
  healthCheckComplete: false,
};

async function checkEmergencyServerHealth(): Promise<boolean> {
  const BASIC_AUTH = 'Basic ' + Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString('base64');
  const emergencyRequest = await playwrightRequest.newContext({
    baseURL: EMERGENCY_SERVER.baseURL,
  });

  try {
    const response = await emergencyRequest.get('/health', {
      headers: { 'Authorization': BASIC_AUTH },
      timeout: 3000,
    });
    return response.ok();
  } catch {
    return false;
  } finally {
    await emergencyRequest.dispose();
  }
}

async function ensureHealthChecked(): Promise<boolean> {
  if (!testState.healthCheckComplete) {
    console.log('🔍 Checking tier-2 server health before tests...');
    testState.emergencyServerHealthy = await checkEmergencyServerHealth();
    testState.healthCheckComplete = true;
    if (!testState.emergencyServerHealthy) {
      console.log('⚠️ Tier-2 server is unavailable - tests will be skipped');
    } else {
      console.log('✅ Tier-2 server is healthy');
    }
  }
  return testState.emergencyServerHealthy ?? false;
}

test.describe('Break Glass - Tier 2 (Emergency Server)', () => {
  const EMERGENCY_BASE_URL = EMERGENCY_SERVER.baseURL;
  const BASIC_AUTH = 'Basic ' + Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString('base64');

  // Skip individual tests if emergency server is not healthy
  test.beforeEach(async ({}, testInfo) => {
    const isHealthy = await ensureHealthChecked();
    if (!isHealthy) {
      console.log('⚠️ Emergency server not accessible from test environment - continuing test anyway');
      // Changed from testInfo.skip() to allow test to run and identify root cause
      // testInfo.skip(true, 'Emergency server not accessible from test environment');
    }
  });

  test('should access emergency server health endpoint without ACL blocking', async ({ request }) => {
    // This tests the "sidecar door" - completely bypasses main app security

    const response = await request.get(`${EMERGENCY_BASE_URL}/health`, {
      headers: {
        'Authorization': BASIC_AUTH,
      },
    });

    expect(response.ok()).toBeTruthy();
    let body;
    try {
      body = await response.json();
    } catch (e) {
      // Note: Can't get text after json() fails because body is consumed
      console.error(`❌ JSON parse failed: ${String(e)}`);
      body = { _parseError: String(e) };
    }
    expect(body.status, `Expected 'ok' but got '${body.status}'. Parse error: ${body._parseError || 'none'}`).toBe('ok');
    expect(body.server).toBe('emergency');
  });

  test('should reset security via emergency server (bypasses Caddy layer)', async ({ request }) => {
    // Use Tier 2 endpoint - proves we can bypass if Tier 1 is blocked

    const response = await request.post(`${EMERGENCY_BASE_URL}/emergency/security-reset`, {
      headers: {
        'X-Emergency-Token': EMERGENCY_TOKEN,
        'Authorization': BASIC_AUTH,
      },
    });

    expect(response.ok()).toBeTruthy();
    let result;
    try {
      result = await response.json();
    } catch {
      result = { success: false, disabled_modules: [] };
    }
    expect(result.success).toBe(true);
    expect(result.disabled_modules).toContain('security.acl.enabled');
    expect(result.disabled_modules).toContain('security.waf.enabled');
    expect(result.disabled_modules).toContain('security.rate_limit.enabled');
  });

  test('should validate defense in depth - both tiers work independently', async ({ request }) => {
    // First, ensure security is enabled by resetting via Tier 2
    const resetResponse = await request.post(`${EMERGENCY_BASE_URL}/emergency/security-reset`, {
      headers: {
        'X-Emergency-Token': EMERGENCY_TOKEN,
        'Authorization': BASIC_AUTH,
      },
    });

    expect(resetResponse.ok()).toBeTruthy();

    // Wait for propagation
    await new Promise(resolve => setTimeout(resolve, 2000));

    // Verify Tier 2 still accessible even after reset
    const healthCheck = await request.get(`${EMERGENCY_BASE_URL}/health`, {
      headers: {
        'Authorization': BASIC_AUTH,
      },
    });

    expect(healthCheck.ok()).toBeTruthy();
    let health;
    try {
      health = await healthCheck.json();
    } catch (e) {
      // Note: Can't get text after json() fails because body is consumed
      console.error(`❌ JSON parse failed: ${String(e)}`);
      health = { status: 'unknown', _parseError: String(e) };
    }
    expect(health.status, `Expected 'ok' but got '${health.status}'. Parse error: ${health._parseError || 'none'}`).toBe('ok');
  });

  test('should enforce Basic Auth on emergency server', async ({ request }) => {
    // /health is intentionally unauthenticated for monitoring probes
    // Protected endpoints like /emergency/security-reset require Basic Auth

    const response = await request.post(`${EMERGENCY_BASE_URL}/emergency/security-reset`, {
      headers: {
        'X-Emergency-Token': EMERGENCY_TOKEN,
        // Deliberately omitting Authorization header to test auth enforcement
      },
      failOnStatusCode: false,
    });

    // Should get 401 without Basic Auth credentials
    expect(response.status()).toBe(401);
  });

  test('should reject invalid emergency token on Tier 2', async ({ request }) => {
    // Even Tier 2 validates the emergency token

    const response = await request.post(`${EMERGENCY_BASE_URL}/emergency/security-reset`, {
      headers: {
        'X-Emergency-Token': 'invalid-token-12345678901234567890',
        'Authorization': BASIC_AUTH,
      },
      failOnStatusCode: false,
    });

    expect(response.status()).toBe(401);
    const result = await response.json();
    expect(result.error).toBe('unauthorized');
  });

  test('should rate limit emergency server requests (lenient in test mode)', async ({ request }) => {
    // Test that rate limiting works but is lenient (50 attempts vs 5 in production)

    // Make multiple requests rapidly
    const requests = Array.from({ length: 10 }, () =>
      request.post(`${EMERGENCY_BASE_URL}/emergency/security-reset`, {
        headers: {
          'X-Emergency-Token': EMERGENCY_TOKEN,
          'Authorization': BASIC_AUTH,
        },
      })
    );

    const responses = await Promise.all(requests);

    // All should succeed in test environment (50 attempts allowed)
    for (const response of responses) {
      expect(response.ok()).toBeTruthy();
    }
  });

  test('should provide independent access even when main app is blocking', async ({ request }) => {
    // Scenario: Main app (:8080) might be blocked by ACL/WAF
    // Emergency server (:2019) should still work

    // Test emergency server is accessible
    const emergencyHealth = await request.get(`${EMERGENCY_BASE_URL}/health`, {
      headers: {
        'Authorization': BASIC_AUTH,
      },
    });

    expect(emergencyHealth.ok()).toBeTruthy();

    // Test main app is also accessible (in E2E environment both work)
    const mainHealth = await request.get('http://localhost:8080/api/v1/health');
    expect(mainHealth.ok()).toBeTruthy();

    // Key point: Emergency server provides alternative path if main is blocked
    const mainHealthData = await mainHealth.json();
    const emergencyHealthData = await emergencyHealth.json();

    expect(mainHealthData.status).toBe('ok');
    expect(emergencyHealthData.server).toBe('emergency');
  });
});
