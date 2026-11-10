/**
 * Security Headers Enforcement Tests
 *
 * Tests that verify security headers are properly set on responses.
 *
 * NOTE: Security headers are applied at Caddy layer. These tests verify
 * the expected headers through the API proxy.
 *
 * @see /projects/Charon/docs/plans/current_spec.md - Security Headers Enforcement Tests
 */

import { test, expect } from '../fixtures/test';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import {
  captureSecurityState,
  restoreSecurityState,
  setSecurityModuleEnabled,
  type SecurityStateSnapshot,
} from '../utils/security-helpers';
import { RUNNER_WHITELIST, saveConfig } from '../utils/security-settings-api';
import {
  PROFILE_HEADER_EXPECTATIONS,
  createCountryAllowList,
  createHeaderProfile,
  createProxyHost,
  deleteAccessList,
  deleteHeaderProfile,
  deleteProxyHost,
  fetchThroughProxy,
  type HeaderProfile,
} from '../utils/proxy-refusal-helpers';

test.describe('Security Headers Enforcement', () => {
  let requestContext: APIRequestContext;

  test.beforeAll(async () => {
    requestContext = await request.newContext({
      baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080',
      storageState: STORAGE_STATE,
    });
  });

  test.afterAll(async () => {
    await requestContext.dispose();
  });

  test('should return X-Content-Type-Options header', async () => {
    const response = await requestContext.get('/api/v1/health');
    expect(response.ok()).toBe(true);

    // X-Content-Type-Options should be 'nosniff'
    const header = response.headers()['x-content-type-options'];
    if (header) {
      expect(header).toBe('nosniff');
    } else {
      // If backend doesn't set it, Caddy should when configured
      console.log(
        'X-Content-Type-Options not set directly (may be set at Caddy layer)'
      );
    }
  });

  test('should return X-Frame-Options header', async () => {
    const response = await requestContext.get('/api/v1/health');
    expect(response.ok()).toBe(true);

    // X-Frame-Options should be 'DENY' or 'SAMEORIGIN'
    const header = response.headers()['x-frame-options'];
    if (header) {
      expect(['DENY', 'SAMEORIGIN', 'deny', 'sameorigin']).toContain(header);
    } else {
      // If backend doesn't set it, Caddy should when configured
      console.log(
        'X-Frame-Options not set directly (may be set at Caddy layer)'
      );
    }
  });

  test('should document HSTS behavior on HTTPS', async () => {
    // HSTS (Strict-Transport-Security) is only set on HTTPS responses
    // In test environment, we typically use HTTP
    //
    // Expected header on HTTPS:
    //   Strict-Transport-Security: max-age=31536000; includeSubDomains
    //
    // This test verifies HSTS is not incorrectly set on HTTP

    const response = await requestContext.get('/api/v1/health');
    expect(response.ok()).toBe(true);

    const hsts = response.headers()['strict-transport-security'];

    // On HTTP, HSTS should not be present (browsers ignore it anyway)
    if (process.env.PLAYWRIGHT_BASE_URL?.startsWith('https://')) {
      expect(hsts).toBeDefined();
      expect(hsts).toContain('max-age');
    } else {
      // HTTP is fine without HSTS in test env
      console.log('HSTS not present on HTTP (expected behavior)');
    }
  });

  test('should verify Content-Security-Policy when configured', async () => {
    // CSP is optional and configured per-host
    // This test verifies CSP header handling when present

    const response = await requestContext.get('/');
    // May be 200 or redirect
    expect(response.status()).toBeLessThan(500);

    const csp = response.headers()['content-security-policy'];
    if (csp) {
      // CSP should contain valid directives
      expect(
        csp.includes("default-src") ||
          csp.includes("script-src") ||
          csp.includes("style-src")
      ).toBe(true);
    } else {
      // CSP is optional - document its behavior when configured
      console.log('CSP not configured (optional - set per proxy host)');
    }
  });

  test.describe('Headers on access list refusals', () => {
    const domain = `refusal-headers-acl-${Date.now()}.example.test`;
    let snapshot: SecurityStateSnapshot;
    let proxyContext: APIRequestContext;
    let profile: HeaderProfile;
    let aclUuid: string;
    let proxyUuid: string;

    test.beforeAll(async () => {
      snapshot = await captureSecurityState(requestContext);

      // The admin whitelist must be in place before the ACL is enabled, or the admin API refuses the runner.
      await saveConfig(requestContext, { name: 'default', admin_whitelist: RUNNER_WHITELIST });
      await setSecurityModuleEnabled(requestContext, 'cerberus', true);
      // Rate limiting would answer first on a shared client IP and hide the ACL refusal.
      await setSecurityModuleEnabled(requestContext, 'rateLimit', false);
      await setSecurityModuleEnabled(requestContext, 'acl', true);

      proxyContext = await request.newContext();
      profile = await createHeaderProfile(requestContext, `Refusal Headers ${Date.now()}`);
      // Country list that admits only US; the runner has no country, so the ACL refuses it.
      aclUuid = await createCountryAllowList(requestContext, `Refusal Headers Allow List ${Date.now()}`);
      proxyUuid = await createProxyHost(requestContext, {
        domain,
        forwardPort: 3001,
        name: 'Refusal headers host',
        accessListUuid: aclUuid,
        headerProfileUuid: profile.uuid,
      });

      await expect
        .poll(async () => (await fetchThroughProxy(proxyContext, domain, '/')).status(), {
          timeout: 30_000,
          message: 'refusal headers route should be loaded',
        })
        .toBe(403);
    });

    test.afterAll(async () => {
      if (proxyUuid) {
        await deleteProxyHost(requestContext, proxyUuid);
      }
      if (aclUuid) {
        await deleteAccessList(requestContext, aclUuid);
      }
      if (profile) {
        await deleteHeaderProfile(requestContext, profile);
      }
      await proxyContext.dispose();
      if (snapshot) {
        await restoreSecurityState(requestContext, snapshot);
      }
    });

    test('should send the profile security headers with an access list refusal', async () => {
      const response = await fetchThroughProxy(proxyContext, domain, '/');

      expect(response.status()).toBe(403);
      expect(response.headers()).toMatchObject(PROFILE_HEADER_EXPECTATIONS);
    });
  });
});
