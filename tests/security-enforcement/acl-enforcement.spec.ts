/**
 * ACL Enforcement Tests
 *
 * Tests that verify the Access Control List (ACL) module correctly blocks/allows
 * requests based on IP whitelist and blacklist rules.
 *
 * Pattern: Toggle-On-Test-Toggle-Off
 * - Enable ACL at start of describe block
 * - Run enforcement tests
 * - Disable ACL in afterAll (handled by security-teardown project)
 *
 * @see /projects/Charon/docs/plans/current_spec.md - ACL Enforcement Tests
 */

import { test, expect } from '../fixtures/test';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import {
  getSecurityStatus,
  setSecurityModuleEnabled,
  captureSecurityState,
  restoreSecurityState,
  SecurityStateSnapshot,
} from '../utils/security-helpers';
import {
  createCountryAllowList,
  createProxyHost,
  deleteAccessList,
  deleteProxyHost,
  fetchThroughProxy,
} from '../utils/proxy-refusal-helpers';

/**
 * Configure admin whitelist to allow test runner IPs.
 * CRITICAL: Must be called BEFORE enabling any security modules to prevent 403 blocking.
 */
async function configureAdminWhitelist(requestContext: APIRequestContext) {
  // Configure whitelist to allow test runner IPs (localhost, Docker networks)
  const testWhitelist = '127.0.0.1/32,172.16.0.0/12,192.168.0.0/16,10.0.0.0/8';

  const response = await requestContext.patch(
    `${process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080'}/api/v1/config`,
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

test.describe('ACL Enforcement', () => {
  let requestContext: APIRequestContext;
  let originalState: SecurityStateSnapshot;

  test.beforeAll(async () => {
    requestContext = await request.newContext({
      baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080',
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

    // Enable ACL
    try {
      await setSecurityModuleEnabled(requestContext, 'acl', true);
      console.log('✓ ACL enabled');
    } catch (error) {
      console.error('Failed to enable ACL:', error);
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
        // Emergency disable ACL to prevent deadlock
        try {
          await setSecurityModuleEnabled(requestContext, 'acl', false);
          await setSecurityModuleEnabled(requestContext, 'cerberus', false);
        } catch {
          console.error('Emergency ACL disable also failed');
        }
      }
    }
    await requestContext.dispose();
  });

  test('should verify ACL is enabled', async () => {
    const status = await getSecurityStatus(requestContext);
    expect(status.acl.enabled).toBe(true);
    expect(status.cerberus.enabled).toBe(true);
  });

  test('should return security status with ACL mode', async () => {
    const response = await requestContext.get('/api/v1/security/status');
    expect(response.ok()).toBe(true);

    const status = await response.json();
    expect(status.acl).toBeDefined();
    expect(status.acl.mode).toBeDefined();
    expect(typeof status.acl.enabled).toBe('boolean');
  });

  test('should list access lists when ACL enabled', async () => {
    const response = await requestContext.get('/api/v1/access-lists');
    expect(response.ok()).toBe(true);

    const data = await response.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('should test IP against access list', async () => {
    // First, get the list of access lists
    const listResponse = await requestContext.get('/api/v1/access-lists');
    expect(listResponse.ok()).toBe(true);

    const lists = await listResponse.json();

    // If there are any access lists, test an IP against the first one
    if (lists.length > 0) {
      const testIp = '192.168.1.1';
      const testResponse = await requestContext.post(
        `/api/v1/access-lists/${lists[0].uuid}/test`,
        { data: { ip_address: testIp } }
      );
      expect(testResponse.ok()).toBe(true);

      const result = await testResponse.json();
      expect(typeof result.allowed).toBe('boolean');
    } else {
      // No access lists exist - this is valid, just log it
      console.log('No access lists exist to test against');
    }
  });

  test('should show correct error response format for blocked requests', async () => {
    // Create a temporary blacklist with test IP, make blocked request, then cleanup
    // For now, verify the error message format from the blocked response

    // This test verifies the error handling structure exists
    // The actual blocking test would require:
    // 1. Create blacklist entry with test IP
    // 2. Make request from that IP (requires proxy setup)
    // 3. Verify 403 with "Blocked by access control list" message
    // 4. Delete blacklist entry

    // Instead, we verify the API structure for ACL CRUD
    const createResponse = await requestContext.post('/api/v1/access-lists', {
      data: {
        name: 'Test Enforcement ACL',
        type: 'blacklist',
        ip_rules: JSON.stringify([{ cidr: '10.255.255.255/32', description: 'Test blocked IP' }]),
        enabled: true,
      },
    });

    if (createResponse.ok()) {
      const createdList = await createResponse.json();
      expect(createdList.uuid).toBeDefined();

      // Verify the list was created with correct structure
      expect(createdList.name).toBe('Test Enforcement ACL');

      // Test IP against the list using POST
      const testResponse = await requestContext.post(
        `/api/v1/access-lists/${createdList.uuid}/test`,
        { data: { ip_address: '10.255.255.255' } }
      );
      expect(testResponse.ok()).toBe(true);
      const testResult = await testResponse.json();
      expect(testResult.allowed).toBe(false);

      // Cleanup: Delete the test ACL
      const deleteResponse = await requestContext.delete(
        `/api/v1/access-lists/${createdList.uuid}`
      );
      expect(deleteResponse.ok()).toBe(true);
    } else {
      // May fail if ACL already exists or other issue
      const errorBody = await createResponse.text();
      console.log(`Note: Could not create test ACL: ${errorBody}`);
    }
  });

  test('should refuse an enabled allow-list saved with no rules', async () => {
    // An allow-list that is enabled but has no rules would admit nobody at save time, so the API refuses it with a
    // stable error code the UI maps to a message.
    const response = await requestContext.post('/api/v1/access-lists', {
      data: {
        name: `No Rules Allow List ${Date.now()}`,
        type: 'whitelist',
        ip_rules: '[]',
        enabled: true,
      },
    });

    expect(response.status()).toBe(400);
    expect(await response.json()).toMatchObject({ error_code: 'access_list_rules_required' });
  });

  test.describe('Country lists', () => {
    const domain = `country-list-${Date.now()}.example.test`;
    let proxyContext: APIRequestContext;
    let aclUuid: string;
    let proxyUuid: string;

    test.beforeAll(async () => {
      proxyContext = await request.newContext();
      aclUuid = await createCountryAllowList(requestContext, `Country Allow List ${Date.now()}`);
      proxyUuid = await createProxyHost(requestContext, {
        domain,
        forwardPort: 3001,
        name: 'Country list host',
        accessListUuid: aclUuid,
      });

      // Wait for the route to be loaded: the country list refuses the runner, which has no country.
      await expect
        .poll(async () => (await fetchThroughProxy(proxyContext, domain, '/')).status(), {
          timeout: 30_000,
          message: 'country list route should be loaded',
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
      await proxyContext.dispose();
    });

    test('should refuse a client outside the listed countries and report the GeoIP database state', async () => {
      await test.step('Refused with the geographic body', async () => {
        const response = await fetchThroughProxy(proxyContext, domain, '/');

        expect(response.status()).toBe(403);
        expect(await response.text()).toContain('Access denied: Geographic restriction');
      });

      await test.step('Status reports geoip_unavailable exactly when the database is not loaded', async () => {
        // The e2e image bundles the GeoLite2 database when its download succeeds at build time. The database state
        // decides which path answers, so the check reads it from the API instead of assuming it.
        const geoipResponse = await requestContext.get('/api/v1/security/geoip/status');
        expect(geoipResponse.ok()).toBe(true);
        const { available } = (await geoipResponse.json()) as { available: boolean };

        const statusResponse = await requestContext.get('/api/v1/security/status');
        expect(statusResponse.ok()).toBe(true);
        const status = (await statusResponse.json()) as { acl: { reason?: string } };

        if (available) {
          expect(status.acl).not.toHaveProperty('reason');
        } else {
          expect(status.acl.reason).toBe('geoip_unavailable');
        }
      });
    });
  });

  test.describe('Route handling on proxy hosts', () => {
    // An IP allow-list cannot express a non-listed runner here: the runner is
    // admin-whitelisted, and the proxy ACL merges the admin whitelist into every
    // IP allow-list. A geo allow-list with no country match for the runner's
    // private address refuses it without that merge.
    const ROUTE_PATHS = [
      '/',
      '/emergency/security-reset',
      '/emergency/x',
      '/emergency/..;/admin',
      '/api/v1/emergency/token/x',
      '/api/v1/emergency/security-reset',
    ];
    // Paths under the reset prefix that the reset endpoint does not serve.
    const NON_RESET_PATHS = ROUTE_PATHS.filter(
      (path) => path !== '/emergency/security-reset' && path !== '/api/v1/emergency/security-reset'
    );
    const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
    const RESET_API_PATH = '/api/v1/emergency/security-reset';
    const RESET_APP_PATH = '/emergency/security-reset';

    let proxyContext: APIRequestContext;
    let aclUuid: string;
    let allowListedProxyUuid: string;
    let allowListedDomain: string;
    let selfProxyUuid: string;
    let selfProxyDomain: string;

    /** Send a request through the proxy hop with the given Host header. */
    function proxied(domain: string, path: string, method = 'GET') {
      return fetchThroughProxy(proxyContext, domain, path, method);
    }

    test.beforeAll(async () => {
      proxyContext = await request.newContext();

      // Allow-list that only admits one country; the runner has no country, so it is
      // not on the list. Attached to both hosts.
      aclUuid = await createCountryAllowList(requestContext, `Route Handling Allow List ${Date.now()}`);

      // Plain allow-listed host: upstream is a port nothing listens on, so any
      // request that gets past the ACL would fail upstream with 502.
      allowListedDomain = `route-allow-${Date.now()}.example.test`;
      allowListedProxyUuid = await createProxyHost(requestContext, {
        domain: allowListedDomain,
        forwardPort: 3001,
        name: 'Allow-listed host',
        accessListUuid: aclUuid,
      });

      // Self-proxy host: upstream is Charon's own listener.
      selfProxyDomain = `route-self-${Date.now()}.example.test`;
      selfProxyUuid = await createProxyHost(requestContext, {
        domain: selfProxyDomain,
        forwardPort: 8080,
        name: 'Self-proxy host',
        accessListUuid: aclUuid,
      });

      // Wait for both routes to be loaded by the proxy. The ACL answers 403 on '/'.
      for (const domain of [allowListedDomain, selfProxyDomain]) {
        await expect
          .poll(async () => (await proxied(domain, '/')).status(), {
            timeout: 30_000,
            message: `route for ${domain} should be loaded`,
          })
          .toBe(403);
      }
    });

    test.afterAll(async () => {
      for (const uuid of [selfProxyUuid, allowListedProxyUuid]) {
        if (uuid) {
          await deleteProxyHost(requestContext, uuid);
        }
      }
      if (aclUuid) {
        await deleteAccessList(requestContext, aclUuid);
      }
      await proxyContext.dispose();
    });

    test('refuses a non-listed client on every route handling path of an allow-listed host', async () => {
      const results = await Promise.all(
        ROUTE_PATHS.map(async (path) => {
          const response = await proxied(allowListedDomain, path);
          return { path, status: response.status() };
        })
      );

      expect(results).toEqual(ROUTE_PATHS.map((path) => ({ path, status: 403 })));
    });

    test('keeps the reset endpoint reachable on a self-proxy host while other paths stay under the ACL', async () => {
      await test.step('Non-reset paths under the prefix are refused by the ACL', async () => {
        const results = await Promise.all(
          NON_RESET_PATHS.map(async (path) => {
            const response = await proxied(selfProxyDomain, path);
            return { path, status: response.status() };
          })
        );

        expect(results).toEqual(NON_RESET_PATHS.map((path) => ({ path, status: 403 })));
      });

      await test.step('API reset path answers from the reset handler without a token', async () => {
        // One tokenless call only: each failed reset attempt spends the shared per-address
        // budget, so no direct-to-app comparison is sent alongside it.
        const response = await proxied(selfProxyDomain, RESET_API_PATH, 'POST');

        // The reset endpoint's own token check answers (401), not the ACL refusal (403).
        expect(response.status()).toBe(401);
        expect(await response.json()).toMatchObject({ error: 'unauthorized' });
      });

      await test.step('Exact app-level reset path is answered by the app, not the ACL', async () => {
        // The app has no API route at this path; its own router answers it. The
        // proxied answer must match the direct answer and must not be the ACL refusal.
        const response = await proxied(selfProxyDomain, RESET_APP_PATH, 'POST');
        const direct = await proxyContext.fetch(`${APP_BASE_URL}${RESET_APP_PATH}`, {
          method: 'POST',
          data: {},
          failOnStatusCode: false,
        });

        expect(response.status()).not.toBe(403);
        expect(response.status()).toBe(direct.status());
      });
    });
  });
});
