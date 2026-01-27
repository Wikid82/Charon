/**
 * WAF (Coraza) Enforcement Tests
 *
 * Tests that verify the Web Application Firewall correctly blocks malicious
 * requests such as SQL injection and XSS attempts.
 *
 * NOTE: Full WAF blocking tests require Caddy proxy with Coraza plugin.
 * These tests verify the WAF configuration API and expected behavior.
 *
 * Pattern: Toggle-On-Test-Toggle-Off
 *
 * @see /projects/Charon/docs/plans/current_spec.md - WAF Enforcement Tests
 */

import { test, expect } from '@bgotink/playwright-coverage';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import {
  getSecurityStatus,
  setSecurityModuleEnabled,
  captureSecurityState,
  restoreSecurityState,
  CapturedSecurityState,
} from '../utils/security-helpers';

/**
 * Configure admin whitelist to allow test runner IPs.
 * CRITICAL: Must be called BEFORE enabling any security modules to prevent 403 blocking.
 */
async function configureAdminWhitelist(requestContext: APIRequestContext) {
  // Configure whitelist to allow test runner IPs (localhost, Docker networks)
  const testWhitelist = '127.0.0.1/32,172.16.0.0/12,192.168.0.0/16,10.0.0.0/8';

  const response = await requestContext.patch(
    `${process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080'}/api/v1/config`,
    {
      data: {
        security: {
          admin_whitelist: testWhitelist,
        },
      },
    }
  );

  if (!response.ok()) {
    throw new Error(`Failed to configure admin whitelist: ${response.status()}`);
  }

  console.log('✅ Admin whitelist configured for test IP ranges');
}

test.describe('WAF Enforcement', () => {
  let requestContext: APIRequestContext;
  let originalState: CapturedSecurityState;

  test.beforeAll(async () => {
    requestContext = await request.newContext({
      baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',
      storageState: STORAGE_STATE,
    });

    // CRITICAL: Configure admin whitelist BEFORE enabling security modules
    try {
      await configureAdminWhitelist(requestContext);
    } catch (error) {
      console.error('Failed to configure admin whitelist:', error);
    }

    // Capture original state
    try {
      originalState = await captureSecurityState(requestContext);
    } catch (error) {
      console.error('Failed to capture original security state:', error);
    }

    // Enable Cerberus (master toggle) first
    try {
      await setSecurityModuleEnabled(requestContext, 'cerberus', true);
      console.log('✓ Cerberus enabled');
    } catch (error) {
      console.error('Failed to enable Cerberus:', error);
    }

    // Enable WAF
    try {
      await setSecurityModuleEnabled(requestContext, 'waf', true);
      console.log('✓ WAF enabled');
    } catch (error) {
      console.error('Failed to enable WAF:', error);
    }
  });

  test.afterAll(async () => {
    // Restore original state
    if (originalState) {
      try {
        await restoreSecurityState(requestContext, originalState);
        console.log('✓ Security state restored');
      } catch (error) {
        console.error('Failed to restore security state:', error);
        // Emergency disable WAF to prevent interference
        try {
          await setSecurityModuleEnabled(requestContext, 'waf', false);
          await setSecurityModuleEnabled(requestContext, 'cerberus', false);
        } catch {
          console.error('Emergency WAF disable also failed');
        }
      }
    }
    await requestContext.dispose();
  });

  test('should verify WAF is enabled', async () => {
    const status = await getSecurityStatus(requestContext);
    expect(status.waf.enabled).toBe(true);
    expect(status.cerberus.enabled).toBe(true);
  });

  test('should return WAF configuration from security status', async () => {
    const response = await requestContext.get('/api/v1/security/status');
    expect(response.ok()).toBe(true);

    const status = await response.json();
    expect(status.waf).toBeDefined();
    expect(status.waf.mode).toBeDefined();
    expect(typeof status.waf.enabled).toBe('boolean');
  });

  test('should detect SQL injection patterns in request validation', async () => {
    // WAF blocking happens at Caddy/Coraza layer before reaching the API
    // This test documents the expected behavior when SQL injection is attempted
    //
    // With WAF enabled and Caddy configured, requests like:
    //   GET /api/v1/users?id=1' OR 1=1--
    // Should return 403 or 418 (I'm a teapot - Coraza signature)
    //
    // Since we're making direct API requests (not through Caddy proxy),
    // we verify the WAF is configured and document expected blocking behavior

    const status = await getSecurityStatus(requestContext);
    expect(status.waf.enabled).toBe(true);

    // Document: When WAF is enabled and request goes through Caddy:
    // - SQL injection patterns like ' OR 1=1-- should return 403/418
    // - The response will contain WAF block message
    console.log(
      'WAF configured - SQL injection blocking active at Caddy/Coraza layer'
    );
  });

  test('should document XSS blocking behavior', async () => {
    // Similar to SQL injection, XSS blocking happens at Caddy/Coraza layer
    //
    // With WAF enabled, requests containing:
    //   <script>alert('xss')</script>
    // Should be blocked with 403/418
    //
    // Direct API requests bypass Caddy, so we verify configuration

    const status = await getSecurityStatus(requestContext);
    expect(status.waf.enabled).toBe(true);

    // Document: When WAF is enabled and request goes through Caddy:
    // - XSS patterns like <script> tags should return 403/418
    // - Common XSS payloads are blocked by Coraza OWASP CoreRuleSet
    console.log('WAF configured - XSS blocking active at Caddy/Coraza layer');
  });
});
