/**
 * Security Teardown Setup
 *
 * This file runs AFTER all security-tests complete.
 * It disables all security modules to ensure browser tests run without blocking.
 *
 * Uses a two-strategy approach:
 *   1. Try normal API with authentication
 *   2. Fall back to emergency reset endpoint if API is blocked by ACL/security
 *
 * Uses continue-on-error pattern - individual module disable failures won't
 * prevent other modules from being disabled.
 *
 * @see /projects/Charon/docs/plans/current_spec.md - Security Module Testing Plan
 */

import { test as teardown } from '@bgotink/playwright-coverage';
import { request } from '@playwright/test';

teardown('disable-all-security-modules', async () => {
  console.log('\n🔒 Security Teardown: Disabling all security modules...');

  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
  const emergencyToken = process.env.CHARON_EMERGENCY_TOKEN;

  const modules = [
    { key: 'security.acl.enabled', value: 'false' },
    { key: 'security.waf.enabled', value: 'false' },
    { key: 'security.crowdsec.enabled', value: 'false' },
    { key: 'security.rate_limit.enabled', value: 'false' },
    { key: 'feature.cerberus.enabled', value: 'false' },
  ];

  // CRITICAL: Initialize errors array early to prevent "Cannot read properties of undefined"
  const errors: string[] = [];
  let apiBlocked = false;

  // Strategy 1: Try normal API with auth
  const requestContext = await request.newContext({
    baseURL,
    storageState: 'playwright/.auth/user.json',
  });

  for (const { key, value } of modules) {
    try {
      const response = await requestContext.post('/api/v1/settings', {
        data: { key, value },
      });
      if (response.status() === 403) {
        apiBlocked = true;
        console.warn(`  ⚠ API blocked (403) while disabling ${key}`);
        break;
      }
      console.log(`  ✓ Disabled via API: ${key}`);
    } catch (e) {
      const errorMsg = `Failed to disable ${key}: ${e}`;
      errors.push(errorMsg);
      console.warn(`  ⚠ ${errorMsg}`);
      apiBlocked = true;
      break;
    }
  }

  await requestContext.dispose();

  // Strategy 2: If API is blocked, use emergency reset endpoint
  if (apiBlocked && emergencyToken) {
    console.log('  ⚠ API blocked - using emergency reset endpoint...');

    // Mask token for logging (show first 8 chars only)
    const maskedToken = emergencyToken.slice(0, 8) + '...' + emergencyToken.slice(-4);
    console.log(`  🔑 Using emergency token: ${maskedToken}`);

    try {
      // Emergency server runs on port 2020 with basic auth
      const emergencyURL = baseURL.replace(':8080', ':2020');
      const emergencyContext = await request.newContext({
        baseURL: emergencyURL,
        httpCredentials: {
          username: process.env.CHARON_EMERGENCY_USERNAME || 'admin',
          password: process.env.CHARON_EMERGENCY_PASSWORD || 'changeme',
        },
      });

      const response = await emergencyContext.post(
        '/emergency/security-reset',
        {
          headers: {
            'X-Emergency-Token': emergencyToken,
            'Content-Type': 'application/json',
          },
          data: { reason: 'Playwright teardown - API was blocked' },
        }
      );

      if (response.ok()) {
        const body = await response.json();
        console.log(
          `  ✓ Emergency reset successful: ${body.disabled_modules?.join(', ') || 'all modules'}`
        );
        // Clear errors since emergency reset succeeded
        errors.length = 0;
      } else {
        const errorMsg = `Emergency reset failed with status ${response.status()}`;
        console.error(`  ✗ ${errorMsg}`);
        errors.push(errorMsg);
      }
      await emergencyContext.dispose();
    } catch (e) {
      const errorMsg = `Emergency reset network error: ${e instanceof Error ? e.message : String(e)}`;
      console.error(`  ✗ ${errorMsg}`);
      errors.push(errorMsg);
    }
  } else if (apiBlocked && !emergencyToken) {
    const errorMsg = 'API blocked but CHARON_EMERGENCY_TOKEN not set. Generate with: openssl rand -hex 32';
    console.error(`  ✗ ${errorMsg}`);
    errors.push(errorMsg);
  }

  // Stabilization delay - wait for Caddy config reload
  console.log('  ⏳ Waiting for Caddy config reload...');
  await new Promise((resolve) => setTimeout(resolve, 1000));

  if (errors.length > 0) {
    const errorMessage = `Security teardown FAILED - ACL/security modules still enabled!\nThis will cause cascading test failures.\n\nErrors:\n  ${errors.join('\n  ')}\n\nFix: Ensure CHARON_EMERGENCY_TOKEN is set in .env file (generate with: openssl rand -hex 32)`;
    console.error(`\n❌ ${errorMessage}`);
    throw new Error(errorMessage);
  }

  console.log('✅ Security teardown complete: All modules disabled\n');
});
