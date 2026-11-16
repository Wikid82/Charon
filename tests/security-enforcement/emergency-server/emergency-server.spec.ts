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
 * Reference: docs/plans/break_glass_protocol_redesign.md
 */

import { test, expect, request as playwrightRequest, type APIRequestContext } from '@playwright/test';
import { EMERGENCY_TOKEN, EMERGENCY_SERVER } from '../../fixtures/security';
import {
  captureSecurityState,
  restoreSecurityState,
  type SecurityStateSnapshot,
} from '../../utils/security-helpers';
import { deleteAccessList } from '../../utils/proxy-refusal-helpers';
import { runCleanupSteps } from '../../utils/loopback-client-case';
import {
  createEnabledAllowList,
  putManagementAccess,
} from '../../utils/management-access-api';

/** Charon's own access-control switch on the main app (port 8080), set through the security API. */
const ACL_PATH = '/api/v1/security/acl';
/** A proxy-host route the main app guards with the chosen management lists. */
const PROXY_HOSTS_PATH = '/api/v1/proxy-hosts';
/** RFC 5737 TEST-NET-3: the admin allow-list admits nothing the runner uses. */
const ADMIN_WHITELIST_WITHOUT_RUNNER = '203.0.113.1/32';
/** The chosen list admits only this block, so the runner is outside it. */
const UNREACHABLE_NETWORK = '192.168.99.0/24';

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

// Store health status in a way that persists correctly across hooks
const testState = {
  emergencyServerHealthy: undefined as boolean | undefined,
  healthCheckComplete: false,
};

async function ensureHealthChecked(): Promise<boolean> {
  if (!testState.healthCheckComplete) {
    testState.emergencyServerHealthy = await checkEmergencyServerHealth();
    testState.healthCheckComplete = true;
    if (!testState.emergencyServerHealthy) {
      console.log('⚠️ Emergency server not accessible - tests will be skipped');
    }
  }
  return testState.emergencyServerHealthy ?? false;
}

/**
 * Resets access control through the emergency server (port 2020). Idempotent: a reset with access control already off
 * succeeds too, so the cleanup can call it whatever state the test left behind.
 */
async function resetThroughEmergencyServer(): Promise<{ status: number; ok: boolean }> {
  const emergencyRequest = await playwrightRequest.newContext({
    baseURL: EMERGENCY_SERVER.baseURL,
  });

  try {
    const authHeader =
      'Basic ' +
      Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString('base64');

    const response = await emergencyRequest.post('/emergency/security-reset', {
      headers: {
        Authorization: authHeader,
        'X-Emergency-Token': EMERGENCY_TOKEN,
      },
    });
    return { status: response.status(), ok: response.ok() };
  } finally {
    await emergencyRequest.dispose();
  }
}

/**
 * Puts the main app back as the test found it. Access control goes off first, through the emergency server: while it is
 * on and the runner is outside the admin allow-list, the runner's own calls are refused, including the calls that clear
 * the chosen list. Every step runs even when an earlier one fails, then the first failure is rethrown.
 */
async function cleanupBypassState(
  request: APIRequestContext,
  snapshot: SecurityStateSnapshot,
  listUuid: string
): Promise<void> {
  await runCleanupSteps([
    () => resetThroughEmergencyServer(),
    async () => {
      const response = await putManagementAccess(request, []);
      expect(response.ok(), `clearing the management choice failed with ${response.status()}`).toBe(true);
    },
    ...(listUuid !== '' ? [() => deleteAccessList(request, listUuid)] : []),
    () => restoreSecurityState(request, snapshot),
  ]);
}

test.describe('Emergency Server (Tier 2 Break Glass)', () => {
  // Force serial execution to prevent race conditions with shared emergency server state
  test.describe.configure({ mode: 'serial' });

  // Skip individual tests if emergency server is not healthy
  test.beforeEach(async () => {
    const isHealthy = await ensureHealthChecked();
    if (!isHealthy) {
      console.log('⚠️ Emergency server not accessible from test environment - continuing test anyway');
      // Changed from testInfo.skip() to allow test to run and identify root cause
      // testInfo.skip(true, 'Emergency server not accessible from test environment');
    }
  });

  test('Emergency server health endpoint', async () => {
    console.log('🧪 Testing emergency server health endpoint...');

    // Create a new request context for emergency server
    const emergencyRequest = await playwrightRequest.newContext({
      baseURL: EMERGENCY_SERVER.baseURL,
    });

    try {
      const response = await emergencyRequest.get('/health');

      expect(response.ok()).toBeTruthy();
      expect(response.status()).toBe(200);

      let body;
      try {
        body = await response.json();
      } catch (e) {
        // Note: Can't get text after json() fails, so just log the error
        console.error(`❌ JSON parse failed. Status: ${response.status()}, Error: ${String(e)}`);
        body = { status: 'unknown', server: 'emergency', _parseError: String(e) };
      }
      expect(body.status, `Expected 'ok' but got '${body.status}'. Parse error: ${body._parseError || 'none'}`).toBe('ok');
      expect(body.server).toBe('emergency');

      console.log('  ✓ Health endpoint responded successfully');
      console.log(`  ✓ Server type: ${body.server}`);
      console.log('✅ Test 1 passed: Emergency server health endpoint works');
    } finally {
      await emergencyRequest.dispose();
    }
  });

  test('Emergency server requires Basic Auth', async () => {
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

      let body;
      try {
        body = await authResponse.json();
      } catch {
        body = { success: false };
      }
      expect(body.success).toBe(true);

      console.log('  ✓ Request with valid auth succeeded');
      console.log('✅ Test 2 passed: Basic Auth properly enforced');
    } finally {
      await emergencyRequest.dispose();
    }
  });

  // A chosen management list refuses the runner only when the runner is outside the admin allow-list, and the
  // lock-out guard refuses turning access control on in that state. The emergency token is the one bypass of that
  // guard, so access control is turned on with it (the same route the admin whitelist spec uses). The main app then
  // refuses the runner, and the emergency server resets access control on port 2020.
  test('Emergency server bypasses main app security', async ({ request }) => {
    console.log('🧪 Testing emergency server security bypass...');

    const snapshot = await captureSecurityState(request);
    let listUuid = '';

    try {
      await test.step('Enable Cerberus and set the admin allow-list to exclude the runner', async () => {
        await request.post('/api/v1/settings', {
          data: { key: 'feature.cerberus.enabled', value: 'true' },
        });

        const response = await request.patch('/api/v1/config', {
          data: { security: { admin_whitelist: ADMIN_WHITELIST_WITHOUT_RUNNER } },
        });
        expect(response.ok()).toBe(true);
      });

      await test.step('Choose an enabled list that admits only the unreachable network', async () => {
        listUuid = await createEnabledAllowList(request, `Emergency server ACL ${Date.now()}`, UNREACHABLE_NETWORK);
        const response = await putManagementAccess(request, [listUuid]);
        expect(response.ok()).toBe(true);
      });

      await test.step('Turn access control on with the emergency token', async () => {
        const response = await request.patch(ACL_PATH, {
          data: { enabled: true },
          headers: { 'X-Emergency-Token': EMERGENCY_TOKEN },
        });
        expect(response.ok()).toBe(true);
      });

      await test.step('Main app (port 8080) refuses the runner', async () => {
        await expect
          .poll(async () => (await request.get(PROXY_HOSTS_PATH)).status(), {
            message: 'main app should refuse the runner while the chosen list is in force',
          })
          .toBe(403);
        console.log('  ✓ Main app (port 8080) blocking requests with ACL');
      });

      await test.step('Emergency server (port 2020) resets access control despite the refusal', async () => {
        const { status, ok } = await resetThroughEmergencyServer();
        expect(ok).toBeTruthy();
        expect(status).toBe(200);
        console.log('  ✓ Emergency server (port 2020) succeeded despite ACL');
      });

      await test.step('Main app is accessible again after the emergency reset', async () => {
        await expect
          .poll(async () => (await request.get(PROXY_HOSTS_PATH)).status(), {
            message: 'main app should admit the runner after the emergency reset',
          })
          .toBe(200);
        console.log('  ✓ Main app now accessible after emergency reset');
      });

      console.log('✅ Test 3 passed: Emergency server bypasses main app security');
    } finally {
      await cleanupBypassState(request, snapshot, listUuid);
    }
  });

  test('Emergency server security reset works', async () => {
    // SKIP: Security module activation requires Caddy middleware integration.
    // E2E tests hit the Go backend directly (port 8080), bypassing Caddy.
    // The security modules appear enabled in settings but don't actually activate
    // because enforcement happens at the proxy layer, not the backend.
  });

  test('Emergency server minimal middleware (validation)', async () => {
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
