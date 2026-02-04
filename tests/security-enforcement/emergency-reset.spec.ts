/**
 * Emergency Security Reset (Break-Glass) E2E Tests
 *
 * Tests the emergency reset endpoint that bypasses ACL and disables all security
 * modules. This is a break-glass mechanism for recovery when locked out.
 *
 * @see POST /api/v1/emergency/security-reset
 */

import { test, expect } from '@playwright/test';

test.describe('Emergency Security Reset (Break-Glass)', () => {
  const EMERGENCY_TOKEN = process.env.CHARON_EMERGENCY_TOKEN || 'test-emergency-token-for-e2e-32chars';

  test('should reset security when called with valid token', async ({ request }) => {
    const response = await request.post('/api/v1/emergency/security-reset', {
      headers: {
        'X-Emergency-Token': EMERGENCY_TOKEN,
        'Content-Type': 'application/json',
      },
      data: { reason: 'E2E test validation' },
    });

    expect(response.ok()).toBeTruthy();
    const body = await response.json();
    expect(body.success).toBe(true);

    // Verify individual security modules are disabled
    expect(body.disabled_modules).toContain('security.acl.enabled');
    expect(body.disabled_modules).toContain('security.waf.enabled');
    expect(body.disabled_modules).toContain('security.rate_limit.enabled');
    expect(body.disabled_modules).toContain('security.crowdsec.enabled');
    expect(body.disabled_modules).toContain('security.crowdsec.mode');

    // NOTE: feature.cerberus.enabled is NOT disabled by emergency reset
    // The Cerberus framework stays enabled to allow security module management
    // Only enforcement modules (ACL, WAF, Rate Limit, CrowdSec) are disabled
    expect(body.disabled_modules).not.toContain('feature.cerberus.enabled');
  });

  test('should reject request with invalid token', async ({ request }) => {
    const response = await request.post('/api/v1/emergency/security-reset', {
      headers: {
        'X-Emergency-Token': 'invalid-token-here',
        'Content-Type': 'application/json',
      },
    });

    expect(response.status()).toBe(401);
  });

  test('should reject request without token', async ({ request }) => {
    const response = await request.post('/api/v1/emergency/security-reset');
    expect(response.status()).toBe(401);
  });

  test('should allow recovery when ACL blocks everything', async ({ request }) => {
    // This test verifies the emergency reset works when normal API is blocked
    // Pre-condition: ACL must be enabled and blocking requests
    // The emergency endpoint should still work because it bypasses ACL

    // Attempt emergency reset - should succeed even if ACL is blocking
    const response = await request.post('/api/v1/emergency/security-reset', {
      headers: {
        'X-Emergency-Token': EMERGENCY_TOKEN,
        'Content-Type': 'application/json',
      },
      data: { reason: 'E2E test - ACL recovery validation' },
    });

    // Verify reset was successful
    expect(response.ok()).toBeTruthy();
    const body = await response.json();
    expect(body.success).toBe(true);
    expect(body.disabled_modules).toContain('security.acl.enabled');
  });

  // Rate limit test runs LAST to avoid blocking subsequent tests
  test('should rate limit after 5 attempts', async ({ request }) => {
    test.skip(
      true,
      'Rate limiting enforced via Cerberus middleware (port 80). Verified in integration tests (backend/integration/).'
    );

    // Rate limiting is covered in emergency-token.spec.ts (Test 2), which also
    // waits for the limiter window to reset to avoid affecting subsequent specs.
    for (let i = 0; i < 5; i++) {
      await request.post('/api/v1/emergency/security-reset', {
        headers: { 'X-Emergency-Token': 'wrong' },
      });
    }

    const response = await request.post('/api/v1/emergency/security-reset', {
      headers: { 'X-Emergency-Token': 'wrong' },
    });
    expect(response.status()).toBe(429);
  });
});
