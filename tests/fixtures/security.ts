/**
 * Security Test Fixtures
 *
 * Provides helper functions for enabling/disabling security modules
 * and testing emergency access during E2E tests.
 */

import { APIRequestContext } from '@playwright/test';

/**
 * Emergency token for E2E tests - must match docker-compose.e2e.yml
 */
export const EMERGENCY_TOKEN =
  process.env.CHARON_EMERGENCY_TOKEN || 'test-emergency-token-for-e2e-32chars';

/**
 * Emergency server configuration for E2E tests
 * Port 2020 is used to avoid conflict with Caddy admin API (port 2019)
 */
export const EMERGENCY_SERVER = {
  baseURL: 'http://localhost:2020',
  username: 'admin',
  password: 'changeme',
};

/**
 * Enable all security modules for testing.
 * This simulates a production environment with full security enabled.
 *
 * @param request - Playwright APIRequestContext
 */
export async function enableSecurity(request: APIRequestContext): Promise<void> {
  console.log('🔒 Enabling all security modules...');

  const modules = [
    { key: 'security.acl.enabled', value: 'true' },
    { key: 'security.waf.enabled', value: 'true' },
    { key: 'security.rate_limit.enabled', value: 'true' },
    { key: 'feature.cerberus.enabled', value: 'true' },
  ];

  for (const { key, value } of modules) {
    await request.post('/api/v1/settings', {
      data: { key, value },
    });
    console.log(`  ✓ Enabled: ${key}`);
  }

  // Wait for settings to propagate
  console.log('  ⏳ Waiting for security settings to propagate...');
  await new Promise(resolve => setTimeout(resolve, 2000));

  console.log('  ✅ Security enabled');
}

/**
 * Disable all security modules using the emergency token.
 * This is the proper way to recover from security lockouts.
 *
 * @param request - Playwright APIRequestContext
 * @throws Error if emergency reset fails
 */
export async function disableSecurity(request: APIRequestContext): Promise<void> {
  console.log('🔓 Disabling security using emergency token...');

  const response = await request.post('/api/v1/emergency/security-reset', {
    headers: {
      'X-Emergency-Token': EMERGENCY_TOKEN,
    },
  });

  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Emergency reset failed: ${response.status()} ${body}`);
  }

  const result = await response.json();
  console.log(`  ✅ Disabled modules: ${result.disabled_modules?.join(', ')}`);

  // Wait for settings to propagate
  console.log('  ⏳ Waiting for security reset to propagate...');
  await new Promise(resolve => setTimeout(resolve, 2000));

  console.log('  ✅ Security disabled');
}

/**
 * Test if emergency token access is functional.
 * This is useful for verifying the emergency bypass system is working.
 *
 * @param request - Playwright APIRequestContext
 * @returns true if emergency token works, false otherwise
 */
export async function testEmergencyAccess(request: APIRequestContext): Promise<boolean> {
  try {
    const response = await request.post('/api/v1/emergency/security-reset', {
      headers: {
        'X-Emergency-Token': EMERGENCY_TOKEN,
      },
    });

    return response.ok();
  } catch (e) {
    console.error(`Emergency access test failed: ${e}`);
    return false;
  }
}

/**
 * Test emergency server access (Tier 2 break glass).
 * This tests the separate emergency server on port 2020.
 *
 * @param request - Playwright APIRequestContext
 * @returns true if emergency server is accessible, false otherwise
 */
export async function testEmergencyServerAccess(
  request: APIRequestContext
): Promise<boolean> {
  try {
    // Create Basic Auth header
    const authHeader =
      'Basic ' +
      Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString('base64');

    const response = await request.post(`${EMERGENCY_SERVER.baseURL}/emergency/security-reset`, {
      headers: {
        Authorization: authHeader,
        'X-Emergency-Token': EMERGENCY_TOKEN,
      },
    });

    return response.ok();
  } catch (e) {
    console.error(`Emergency server access test failed: ${e}`);
    return false;
  }
}

/**
 * Wait for security settings to propagate through the system.
 * Some security changes take time to apply due to caching and module loading.
 *
 * @param durationMs - Duration to wait in milliseconds (default: 2000)
 */
export async function waitForSecurityPropagation(durationMs: number = 2000): Promise<void> {
  console.log(`  ⏳ Waiting ${durationMs}ms for security changes to propagate...`);
  await new Promise(resolve => setTimeout(resolve, durationMs));
}
