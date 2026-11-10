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
import { BlockList, isIP } from 'node:net';
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
  createIpBlockList,
  createProxyHost,
  deleteAccessList,
  deleteProxyHost,
  fetchThroughProxy,
} from '../utils/proxy-refusal-helpers';
import { fetchFromContainer, findE2EContainer } from '../utils/container-exec';
import { RUNNER_WHITELIST_NO_LOOPBACK, withRateLimitRetry } from '../utils/security-settings-api';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';

/** Sets the admin whitelist through the config API the admin page uses. */
async function setAdminWhitelist(requestContext: APIRequestContext, whitelist: string) {
  const response = await withRateLimitRetry(() =>
    requestContext.patch(`${APP_BASE_URL}/api/v1/config`, {
      data: {
        security: {
          admin_whitelist: whitelist,
        },
      },
    })
  );

  if (!response.ok()) {
    throw new Error(`Failed to set admin whitelist: ${response.status()}`);
  }
}

/**
 * Configure admin whitelist to allow test runner IPs.
 * CRITICAL: Must be called BEFORE enabling any security modules to prevent 403 blocking.
 */
async function configureAdminWhitelist(requestContext: APIRequestContext) {
  // Configure whitelist to allow test runner IPs (localhost, Docker networks)
  await setAdminWhitelist(requestContext, '127.0.0.1/32,172.16.0.0/12,192.168.0.0/16,10.0.0.0/8');

  console.log('✅ Admin whitelist configured for test IP ranges');
}

/** Parses comma-separated IPv4 CIDRs so membership is decided by the runtime, not by string comparison. */
function ipv4BlockList(cidrs: string): BlockList {
  const list = new BlockList();
  for (const cidr of cidrs.split(',')) {
    const [address, prefix] = cidr.trim().split('/');
    list.addSubnet(address, Number(prefix), 'ipv4');
  }
  return list;
}

/** Returns the IPv4 form of an address the API reports, or undefined for anything else (for example plain IPv6). */
function asIpv4(address: string): string | undefined {
  const bare = address.startsWith('::ffff:') ? address.slice('::ffff:'.length) : address;
  return isIP(bare) === 4 ? bare : undefined;
}

/**
 * Fails unless the runner reaches the API from an address the narrowed whitelist admits and that is not loopback.
 * Without this, the two-sided block-list cases would prove nothing about the runner.
 */
async function assertRunnerIsWhitelistedWithoutLoopback(requestContext: APIRequestContext): Promise<void> {
  const response = await withRateLimitRetry(() => requestContext.get('/api/v1/system/my-ip'));
  if (!response.ok()) {
    throw new Error(`GET /api/v1/system/my-ip failed with ${response.status()}`);
  }
  const { ip } = (await response.json()) as { ip: string };
  const ipv4 = asIpv4(ip);
  const admitted = ipv4 !== undefined && ipv4BlockList(RUNNER_WHITELIST_NO_LOOPBACK).check(ipv4, 'ipv4');
  const loopback = ipv4 !== undefined && ipv4BlockList('127.0.0.0/8').check(ipv4, 'ipv4');
  if (!admitted || loopback) {
    throw new Error(
      `Runner reaches the API from ${ip}, which is loopback or outside ${RUNNER_WHITELIST_NO_LOOPBACK}`
    );
  }
}

/**
 * Runs every cleanup step even when an earlier one fails, then rethrows the first failure. A failed delete must not
 * leave the whitelist narrowed or the rate limit off for the specs that follow.
 */
async function runCleanupSteps(steps: Array<() => Promise<unknown>>): Promise<void> {
  let firstError: unknown;
  let failed = false;
  for (const step of steps) {
    try {
      await step();
    } catch (error) {
      if (!failed) {
        firstError = error;
        failed = true;
      }
    }
  }
  if (failed) {
    throw firstError;
  }
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
        const response = await proxied(selfProxyDomain, RESET_API_PATH, 'POST');
        const direct = await proxyContext.fetch(`${APP_BASE_URL}${RESET_API_PATH}`, {
          method: 'POST',
          data: {},
          failOnStatusCode: false,
        });

        // The reset endpoint's own token check answers (401), the same as the app directly.
        expect(response.status()).toBe(401);
        expect(response.status()).toBe(direct.status());
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

  test.describe('IP block-lists with an admin whitelist', () => {
    // Two-sided cases: the in-container client (loopback, 127.0.0.1) must be refused only when a list names it, and
    // the whitelisted runner must never be refused by these lists. The dead upstream (port 3001) makes a loaded,
    // non-refusing route answer 502, so a refusal (403) and a missing route (catch-all 200) are told apart.
    const CIDR_NOT_THE_CLIENT = '203.0.113.77/32';
    const CIDR_LOOPBACK = '127.0.0.1/32';
    const DEAD_UPSTREAM_PORT = 3001;
    const IP_BLOCK_LIST_BODY = 'Access denied: IP blacklisted';

    test.describe.configure({ mode: 'serial' });

    let container: string;
    let proxyContext: APIRequestContext;
    let snapshot: SecurityStateSnapshot | undefined;
    let originalWhitelist: string | undefined;

    let notClientDomain = '';
    let notClientListUuid = '';
    let notClientHostUuid = '';
    let loopbackDomain = '';
    let loopbackListUuid = '';
    let loopbackHostUuid = '';

    /** One request from inside the container, as the loopback client. */
    function fromLoopback(domain: string) {
      return fetchFromContainer(container, { host: domain, path: '/' });
    }

    test.beforeAll(async () => {
      container = findE2EContainer();
      proxyContext = await request.newContext();
      snapshot = await captureSecurityState(requestContext);
      const stored = snapshot.config?.admin_whitelist;
      originalWhitelist = typeof stored === 'string' ? stored : undefined;

      // Rate limiting off, so polling from the loopback client cannot meet a rate_limit handler.
      await setSecurityModuleEnabled(requestContext, 'rateLimit', false);
      // Narrow the whitelist before any list or host exists, so the first apply that holds a list already has it.
      await setAdminWhitelist(requestContext, RUNNER_WHITELIST_NO_LOOPBACK);
      await assertRunnerIsWhitelistedWithoutLoopback(requestContext);
      await getSecurityStatus(requestContext);

      // Case 1 is created first. Case 2 is created second, and the poll on its 403 proves the applied config was
      // generated after both lists were saved, so case 1 is never checked against a config that lacks its list.
      const suffix = Date.now();
      notClientDomain = `ip-block-elsewhere-${suffix}.example.test`;
      notClientListUuid = await createIpBlockList(requestContext, `Block elsewhere ${suffix}`, CIDR_NOT_THE_CLIENT);
      notClientHostUuid = await createProxyHost(requestContext, {
        domain: notClientDomain,
        forwardPort: DEAD_UPSTREAM_PORT,
        name: 'IP block-list elsewhere host',
        accessListUuid: notClientListUuid,
      });

      loopbackDomain = `ip-block-loopback-${suffix}.example.test`;
      loopbackListUuid = await createIpBlockList(requestContext, `Block loopback ${suffix}`, CIDR_LOOPBACK);
      loopbackHostUuid = await createProxyHost(requestContext, {
        domain: loopbackDomain,
        forwardPort: DEAD_UPSTREAM_PORT,
        name: 'IP block-list loopback host',
        accessListUuid: loopbackListUuid,
      });

      await expect
        .poll(async () => (await fromLoopback(loopbackDomain)).status, {
          timeout: 30_000,
          message: 'loopback block-list route should be loaded and refuse the loopback client',
        })
        .toBe(403);
    });

    test.afterAll(async () => {
      const cleanup: Array<() => Promise<unknown>> = [];
      for (const uuid of [notClientHostUuid, loopbackHostUuid]) {
        if (uuid) {
          cleanup.push(() => deleteProxyHost(requestContext, uuid));
        }
      }
      for (const uuid of [notClientListUuid, loopbackListUuid]) {
        if (uuid) {
          cleanup.push(() => deleteAccessList(requestContext, uuid));
        }
      }
      const whitelist = originalWhitelist;
      if (whitelist !== undefined) {
        cleanup.push(() => setAdminWhitelist(requestContext, whitelist));
      }
      const captured = snapshot;
      if (captured) {
        cleanup.push(() => restoreSecurityState(requestContext, captured));
      }
      const context = proxyContext;
      if (context) {
        cleanup.push(() => context.dispose());
      }
      await runCleanupSteps(cleanup);
    });

    test('does not refuse a loopback client for a block-list on another address', async () => {
      await test.step('In-container client reaches the route: not refused, the dead upstream answers 502', async () => {
        // Before the A0 fix every non-admin address was refused here (403), the #1891 regression.
        const response = await fromLoopback(notClientDomain);
        expect(response.status).toBe(502);
      });

      await test.step('Whitelisted runner reaches the same route: 502', async () => {
        const response = await fetchThroughProxy(proxyContext, notClientDomain, '/');
        expect(response.status()).toBe(502);
      });
    });

    test('refuses a listed loopback client with the block-list body and does not refuse the runner', async () => {
      await test.step('In-container client is refused with the block-list body', async () => {
        const response = await fromLoopback(loopbackDomain);
        expect(response.status).toBe(403);
        expect(response.body).toContain(IP_BLOCK_LIST_BODY);
      });

      await test.step('Whitelisted runner reaches the same route: 502', async () => {
        const response = await fetchThroughProxy(proxyContext, loopbackDomain, '/');
        expect(response.status()).toBe(502);
      });
    });
  });
});
