/**
 * Security Test Helpers - Safe ACL/WAF/Rate Limit toggle for E2E tests
 *
 * These helpers provide safe mechanisms to temporarily enable security features
 * during tests, with guaranteed cleanup even on test failure.
 *
 * Problem: If ACL is left enabled after a test failure, it blocks all API requests
 * causing subsequent tests to fail with 403 Forbidden (deadlock).
 *
 * Solution: Use Playwright's test.afterAll() with captured original state to
 * guarantee restoration regardless of test outcome.
 *
 * @example
 * ```typescript
 * import { withSecurityEnabled, getSecurityStatus } from './utils/security-helpers';
 *
 * test.describe('ACL Tests', () => {
 *   let cleanup: () => Promise<void>;
 *
 *   test.beforeAll(async ({ request }) => {
 *     cleanup = await withSecurityEnabled(request, { acl: true });
 *   });
 *
 *   test.afterAll(async () => {
 *     await cleanup();
 *   });
 *
 *   test('should enforce ACL', async ({ page }) => {
 *     // ACL is now enabled, test enforcement
 *   });
 * });
 * ```
 */

import { APIRequestContext } from '@playwright/test';

/**
 * Security module status from GET /api/v1/security/status
 */
export interface SecurityStatus {
  cerberus: { enabled: boolean };
  crowdsec: { mode: string; api_url: string; enabled: boolean };
  waf: { mode: string; enabled: boolean };
  rate_limit: { mode: string; enabled: boolean };
  acl: { mode: string; enabled: boolean };
}

/**
 * Options for enabling specific security modules
 */
export interface SecurityModuleOptions {
  /** Enable ACL enforcement */
  acl?: boolean;
  /** Enable WAF protection */
  waf?: boolean;
  /** Enable rate limiting */
  rateLimit?: boolean;
  /** Enable CrowdSec */
  crowdsec?: boolean;
  /** Enable master Cerberus toggle (required for other modules) */
  cerberus?: boolean;
}

/**
 * Captured state for restoration
 */
export interface CapturedSecurityState {
  acl: boolean;
  waf: boolean;
  rateLimit: boolean;
  crowdsec: boolean;
  cerberus: boolean;
}

/**
 * Mapping of module names to their settings keys
 */
const SECURITY_SETTINGS_KEYS: Record<keyof SecurityModuleOptions, string> = {
  acl: 'security.acl.enabled',
  waf: 'security.waf.enabled',
  rateLimit: 'security.rate_limit.enabled',
  crowdsec: 'security.crowdsec.enabled',
  cerberus: 'feature.cerberus.enabled',
};

/**
 * Get current security status from the API
 * @param request - Playwright APIRequestContext (authenticated)
 * @returns Current security status
 */
export async function getSecurityStatus(
  request: APIRequestContext
): Promise<SecurityStatus> {
  const maxRetries = 5;
  const retryDelayMs = 1000;

  for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
    const response = await request.get('/api/v1/security/status');

    if (response.ok()) {
      return response.json();
    }

    if (response.status() !== 429 || attempt === maxRetries) {
      throw new Error(
        `Failed to get security status: ${response.status()} ${await response.text()}`
      );
    }

    await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
  }

  throw new Error('Failed to get security status after retries');
}

/**
 * Set a specific security module's enabled state
 * @param request - Playwright APIRequestContext (authenticated)
 * @param module - Which module to toggle
 * @param enabled - Whether to enable or disable
 */
export async function setSecurityModuleEnabled(
  request: APIRequestContext,
  module: keyof SecurityModuleOptions,
  enabled: boolean
): Promise<void> {
  const key = SECURITY_SETTINGS_KEYS[module];
  const value = enabled ? 'true' : 'false';

  const maxRetries = 5;
  const retryDelayMs = 1000;

  for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
    const response = await request.post('/api/v1/settings', {
      data: { key, value },
    });

    if (response.ok()) {
      break;
    }

    if (response.status() !== 429 || attempt === maxRetries) {
      throw new Error(
        `Failed to set ${module} to ${enabled}: ${response.status()} ${await response.text()}`
      );
    }

    await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
  }

  // Wait a brief moment for Caddy config reload
  await new Promise((resolve) => setTimeout(resolve, 500));
}

/**
 * Capture current security state for later restoration
 * @param request - Playwright APIRequestContext (authenticated)
 * @returns Captured state object
 */
export async function captureSecurityState(
  request: APIRequestContext
): Promise<CapturedSecurityState> {
  const status = await getSecurityStatus(request);

  return {
    acl: status.acl.enabled,
    waf: status.waf.enabled,
    rateLimit: status.rate_limit.enabled,
    crowdsec: status.crowdsec.enabled,
    cerberus: status.cerberus.enabled,
  };
}

/**
 * Restore security state to previously captured values
 * @param request - Playwright APIRequestContext (authenticated)
 * @param state - Previously captured state
 */
export async function restoreSecurityState(
  request: APIRequestContext,
  state: CapturedSecurityState
): Promise<void> {
  const currentStatus = await getSecurityStatus(request);

  // Restore in reverse dependency order (features before master toggle)
  const modules: (keyof SecurityModuleOptions)[] = ['acl', 'waf', 'rateLimit', 'crowdsec', 'cerberus'];

  for (const module of modules) {
    const currentValue = module === 'rateLimit'
      ? currentStatus.rate_limit.enabled
      : module === 'crowdsec'
      ? currentStatus.crowdsec.enabled
      : currentStatus[module].enabled;

    if (currentValue !== state[module]) {
      await setSecurityModuleEnabled(request, module, state[module]);
    }
  }
}

/**
 * Enable security modules temporarily with guaranteed cleanup.
 *
 * Returns a cleanup function that MUST be called in test.afterAll().
 * The cleanup function restores the original state even if tests fail.
 *
 * @param request - Playwright APIRequestContext (authenticated)
 * @param options - Which modules to enable
 * @returns Cleanup function to restore original state
 *
 * @example
 * ```typescript
 * test.describe('ACL Tests', () => {
 *   let cleanup: () => Promise<void>;
 *
 *   test.beforeAll(async ({ request }) => {
 *     cleanup = await withSecurityEnabled(request, { acl: true, cerberus: true });
 *   });
 *
 *   test.afterAll(async () => {
 *     await cleanup();
 *   });
 * });
 * ```
 */
export async function withSecurityEnabled(
  request: APIRequestContext,
  options: SecurityModuleOptions
): Promise<() => Promise<void>> {
  // Capture original state BEFORE making any changes
  const originalState = await captureSecurityState(request);

  // Enable Cerberus first (master toggle) if any security module is requested
  const needsCerberus = options.acl || options.waf || options.rateLimit || options.crowdsec;
  if ((needsCerberus || options.cerberus) && !originalState.cerberus) {
    await setSecurityModuleEnabled(request, 'cerberus', true);
  }

  // Enable requested modules
  if (options.acl) {
    await setSecurityModuleEnabled(request, 'acl', true);
  }
  if (options.waf) {
    await setSecurityModuleEnabled(request, 'waf', true);
  }
  if (options.rateLimit) {
    await setSecurityModuleEnabled(request, 'rateLimit', true);
  }
  if (options.crowdsec) {
    await setSecurityModuleEnabled(request, 'crowdsec', true);
  }

  // Return cleanup function that restores original state
  return async () => {
    try {
      await restoreSecurityState(request, originalState);
    } catch (error) {
      // Log error but don't throw - cleanup should not fail tests
      console.error('Failed to restore security state:', error);
      // Try emergency disable of ACL to prevent deadlock
      try {
        await setSecurityModuleEnabled(request, 'acl', false);
      } catch {
        console.error('Emergency ACL disable also failed - manual intervention may be required');
      }
    }
  };
}

/**
 * Disable all security modules (emergency reset).
 * Use this in global-setup.ts or when tests need a clean slate.
 *
 * @param request - Playwright APIRequestContext (authenticated)
 */
export async function disableAllSecurityModules(
  request: APIRequestContext
): Promise<void> {
  const modules: (keyof SecurityModuleOptions)[] = ['acl', 'waf', 'rateLimit', 'crowdsec'];

  for (const module of modules) {
    try {
      await setSecurityModuleEnabled(request, module, false);
    } catch (error) {
      console.warn(`Failed to disable ${module}:`, error);
    }
  }
}

/**
 * Check if ACL is currently blocking requests.
 * Useful for debugging test failures.
 *
 * @param request - Playwright APIRequestContext
 * @returns True if ACL is enabled and blocking
 */
export async function isAclBlocking(request: APIRequestContext): Promise<boolean> {
  try {
    const status = await getSecurityStatus(request);
    return status.acl.enabled && status.cerberus.enabled;
  } catch {
    // If we can't get status, ACL might be blocking
    return true;
  }
}
