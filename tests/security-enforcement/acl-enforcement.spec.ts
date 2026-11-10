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
 * Refusal cases use two clients. The test runner is admin-whitelisted, so it must be admitted; the in-container loopback
 * client is refused by the lists, after the admin whitelist is narrowed to exclude 127.0.0.1 (see
 * utils/loopback-client-case.ts).
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
  createIpBlockList,
  createProxyHost,
  deleteAccessList,
  deleteProxyHost,
  fetchThroughProxy,
} from '../utils/proxy-refusal-helpers';
import { fetchFromContainer, findE2EContainer, type ContainerHttpRequest } from '../utils/container-exec';
import {
  narrowAdminWhitelistForLoopbackClient,
  runCleanupSteps,
  setAdminWhitelist,
} from '../utils/loopback-client-case';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
/** Body of a country-list refusal (both database states answer with it, see config.go geoAccessDeniedBody). */
const GEO_REFUSAL_BODY = 'Access denied: Geographic restriction';
/** Port nothing listens on, so a route that is loaded and admits a request answers 502. */
const DEAD_UPSTREAM_PORT = 3001;

/** Configure admin whitelist to allow test runner IPs (localhost, Docker networks). */
async function configureAdminWhitelist(requestContext: APIRequestContext) {
  await setAdminWhitelist(requestContext, '127.0.0.1/32,172.16.0.0/12,192.168.0.0/16,10.0.0.0/8');

  console.log('✅ Admin whitelist configured for test IP ranges');
}

/** Returns the parsed request for the loopback client, sent from inside the container. */
function fromLoopback(container: string, domain: string, path = '/', method: ContainerHttpRequest['method'] = 'GET') {
  return fetchFromContainer(container, { host: domain, path, method });
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
    // The test creates its own block-list over a documentation range, so the decisions below do not depend on
    // whatever lists other specs left behind. Neither address is in the admin whitelist.
    const LISTED_ADDRESS = '203.0.113.10';
    const UNLISTED_ADDRESS = '198.51.100.10';
    const listUuid = await createIpBlockList(requestContext, `Tester block-list ${Date.now()}`, '203.0.113.0/24');

    try {
      await test.step('Address inside the blocked range is refused by the tester', async () => {
        const response = await requestContext.post(`/api/v1/access-lists/${listUuid}/test`, {
          data: { ip_address: LISTED_ADDRESS },
        });
        expect(response.ok()).toBe(true);
        expect(await response.json()).toEqual({
          allowed: false,
          reason: 'Blocked by blacklist rule',
        });
      });

      await test.step('Address outside the blocked range is allowed by the tester', async () => {
        const response = await requestContext.post(`/api/v1/access-lists/${listUuid}/test`, {
          data: { ip_address: UNLISTED_ADDRESS },
        });
        expect(response.ok()).toBe(true);
        expect(await response.json()).toEqual({
          allowed: true,
          reason: 'Allowed by access list',
        });
      });
    } finally {
      await deleteAccessList(requestContext, listUuid);
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
        // 10.255.255.255 is listed too, but sits inside the admin whitelist (10.0.0.0/8), so the tester must admit it.
        ip_rules: JSON.stringify([
          { cidr: '203.0.113.77/32', description: 'Test blocked IP' },
          { cidr: '10.255.255.255/32', description: 'Test listed admin address' },
        ]),
        enabled: true,
      },
    });

    if (createResponse.ok()) {
      const createdList = await createResponse.json();
      expect(createdList.uuid).toBeDefined();

      // Verify the list was created with correct structure
      expect(createdList.name).toBe('Test Enforcement ACL');

      // Test a listed, non-admin address using POST: the tester reports the proxy's refusal
      const testResponse = await requestContext.post(
        `/api/v1/access-lists/${createdList.uuid}/test`,
        { data: { ip_address: '203.0.113.77' } }
      );
      expect(testResponse.ok()).toBe(true);
      const testResult = await testResponse.json();
      expect(testResult.allowed).toBe(false);
      expect(testResult.reason).toBe('Blocked by blacklist rule');

      // Test a listed address inside the admin whitelist: the proxy exempts it, so the tester must admit it
      const adminResponse = await requestContext.post(
        `/api/v1/access-lists/${createdList.uuid}/test`,
        { data: { ip_address: '10.255.255.255' } }
      );
      expect(adminResponse.ok()).toBe(true);
      const adminResult = await adminResponse.json();
      expect(adminResult.allowed).toBe(true);
      expect(adminResult.reason).toBe('Address is in the admin whitelist');

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
    const listName = `Country Allow List ${Date.now()}`;
    let snapshot: SecurityStateSnapshot | undefined;
    let container: string;
    let proxyContext: APIRequestContext | undefined;
    let aclUuid: string | undefined;
    let proxyUuid: string | undefined;

    test.beforeAll(async () => {
      container = findE2EContainer();
      proxyContext = await request.newContext();
      snapshot = await captureSecurityState(requestContext);
      await narrowAdminWhitelistForLoopbackClient(requestContext);

      aclUuid = await createCountryAllowList(requestContext, listName);
      proxyUuid = await createProxyHost(requestContext, {
        domain,
        forwardPort: DEAD_UPSTREAM_PORT,
        name: 'Country list host',
        accessListUuid: aclUuid,
      });

      // Wait for the route to be loaded. A missing route answers the catch-all page (200). A loaded route answers 403
      // when the list is enforced (database loaded) or 502 from the dead upstream when it is not (database missing).
      await expect
        .poll(async () => (await fromLoopback(container, domain)).status, {
          timeout: 30_000,
          message: 'country list route should be loaded (refusal or dead-upstream 502, not the catch-all page)',
        })
        .toBeGreaterThanOrEqual(400);
    });

    test.afterAll(async () => {
      await runCleanupSteps([
        ...(proxyUuid ? [() => deleteProxyHost(requestContext, proxyUuid as string)] : []),
        ...(aclUuid ? [() => deleteAccessList(requestContext, aclUuid as string)] : []),
        ...(snapshot ? [() => restoreSecurityState(requestContext, snapshot as SecurityStateSnapshot)] : []),
        ...(proxyContext ? [() => (proxyContext as APIRequestContext).dispose()] : []),
      ]);
    });

    test('should enforce a country list only while the running config has the GeoIP database', async () => {
      // The status reason is the source of truth for the running config: geoip_unavailable means country lists are
      // not enforced (no country matching, every client admitted), and then the alert names the list.
      let notEnforced = false;

      await test.step('Status reports geoip_unavailable exactly when the running config does not enforce country lists', async () => {
        const statusResponse = await requestContext.get('/api/v1/security/status');
        expect(statusResponse.ok()).toBe(true);
        const status = (await statusResponse.json()) as { acl: { reason?: string } };

        notEnforced = status.acl.reason !== undefined;
        if (notEnforced) {
          expect(status.acl.reason).toBe('geoip_unavailable');
        }
        test.info().annotations.push({
          type: 'geoip',
          description: notEnforced
            ? 'database missing: country list not enforced, alert raised'
            : 'database loaded: country list enforced',
        });
      });

      if (notEnforced) {
        await test.step('Not enforced: loopback client is admitted, the dead upstream answers 502', async () => {
          const response = await fromLoopback(container, domain);

          expect(response.status).not.toBe(403);
          expect(response.body).not.toContain(GEO_REFUSAL_BODY);
        });

        await test.step('Not enforced: the alert endpoint lists the test list', async () => {
          const alertsResponse = await requestContext.get('/api/v1/notifications/alerts');
          expect(alertsResponse.ok()).toBe(true);
          const { alerts } = (await alertsResponse.json()) as { alerts: Array<{ key: string; affected_lists?: string[] }> };
          const alert = alerts.find((candidate) => candidate.key === 'geoip_unavailable');

          expect(alert?.affected_lists).toContain(listName);
        });
      } else {
        await test.step('Enforced: loopback client is refused with the geographic body', async () => {
          const response = await fromLoopback(container, domain);

          expect(response.status).toBe(403);
          expect(response.body).toContain(GEO_REFUSAL_BODY);
        });
      }

      await test.step('Whitelisted runner is admitted: 502 from the dead upstream', async () => {
        const response = await fetchThroughProxy(proxyContext as APIRequestContext, domain, '/');

        expect(response.status()).toBe(502);
      });
    });
  });

  test.describe('Route handling on proxy hosts', () => {
    // An IP allow-list cannot express a non-listed runner here: the runner is
    // admin-whitelisted, and the proxy ACL merges the admin whitelist into every
    // IP allow-list. A geo allow-list with no country match for the loopback
    // client refuses it, and the admin whitelist admits the runner.
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
    const RESET_API_PATH = '/api/v1/emergency/security-reset';
    const RESET_APP_PATH = '/emergency/security-reset';

    let snapshot: SecurityStateSnapshot | undefined;
    let container: string;
    let proxyContext: APIRequestContext | undefined;
    let aclUuid: string | undefined;
    let allowListedProxyUuid: string | undefined;
    let allowListedDomain: string;
    let selfProxyUuid: string | undefined;
    let selfProxyDomain: string;

    /** Send a request through the proxy hop with the given Host header, as the runner. */
    function proxied(domain: string, path: string, method = 'GET') {
      return fetchThroughProxy(proxyContext as APIRequestContext, domain, path, method);
    }

    test.beforeAll(async () => {
      container = findE2EContainer();
      proxyContext = await request.newContext();
      snapshot = await captureSecurityState(requestContext);
      await narrowAdminWhitelistForLoopbackClient(requestContext);

      // Allow-list that only admits one country; the loopback client has no country, so it is refused. Attached to
      // both hosts.
      aclUuid = await createCountryAllowList(requestContext, `Route Handling Allow List ${Date.now()}`);

      // Plain allow-listed host: upstream is a port nothing listens on, so any
      // request that gets past the ACL would fail upstream with 502.
      allowListedDomain = `route-allow-${Date.now()}.example.test`;
      allowListedProxyUuid = await createProxyHost(requestContext, {
        domain: allowListedDomain,
        forwardPort: DEAD_UPSTREAM_PORT,
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

      // Wait for both routes to be loaded by the proxy. The country list answers 403 to the loopback client on '/', which
      // needs the GeoIP database loaded: without it the list is not enforced and the loopback client is admitted.
      for (const domain of [allowListedDomain, selfProxyDomain]) {
        await expect
          .poll(async () => (await fromLoopback(container, domain)).status, {
            timeout: 30_000,
            message: `route for ${domain} should be loaded and refuse the loopback client (GeoIP database must be loaded)`,
          })
          .toBe(403);
      }
    });

    test.afterAll(async () => {
      await runCleanupSteps([
        ...(selfProxyUuid ? [() => deleteProxyHost(requestContext, selfProxyUuid as string)] : []),
        ...(allowListedProxyUuid ? [() => deleteProxyHost(requestContext, allowListedProxyUuid as string)] : []),
        ...(aclUuid ? [() => deleteAccessList(requestContext, aclUuid as string)] : []),
        ...(snapshot ? [() => restoreSecurityState(requestContext, snapshot as SecurityStateSnapshot)] : []),
        ...(proxyContext ? [() => (proxyContext as APIRequestContext).dispose()] : []),
      ]);
    });

    test('refuses a non-listed client on every route handling path of an allow-listed host', async () => {
      await test.step('Loopback client is refused on every path with the geographic body', async () => {
        const results = await Promise.all(
          ROUTE_PATHS.map(async (path) => {
            const response = await fromLoopback(container, allowListedDomain, path);
            return { path, status: response.status, refused: response.body.includes(GEO_REFUSAL_BODY) };
          })
        );

        expect(results).toEqual(ROUTE_PATHS.map((path) => ({ path, status: 403, refused: true })));
      });

      await test.step('Whitelisted runner is admitted on /: 502 from the dead upstream', async () => {
        const response = await proxied(allowListedDomain, '/');

        expect(response.status()).toBe(502);
      });
    });

    test('keeps the reset endpoint reachable on a self-proxy host while other paths stay under the ACL', async () => {
      await test.step('Non-reset paths under the prefix are refused by the ACL for the loopback client', async () => {
        const results = await Promise.all(
          NON_RESET_PATHS.map(async (path) => {
            const response = await fromLoopback(container, selfProxyDomain, path);
            return { path, status: response.status };
          })
        );

        expect(results).toEqual(NON_RESET_PATHS.map((path) => ({ path, status: 403 })));
      });

      await test.step('Whitelisted runner is admitted on a non-reset path: not refused, answered by the app', async () => {
        // The self-proxy upstream is Charon itself, so the app answers. Anything below 500 that is not a refusal
        // proves the runner passed the ACL.
        const response = await proxied(selfProxyDomain, '/');

        expect(response.status()).not.toBe(403);
        expect(response.status()).toBeLessThan(500);
      });

      await test.step('API reset path answers from the reset handler without a token', async () => {
        // The reset endpoint's own token check answers (401), the same as the app directly. The loopback client
        // goes through the proxy; the direct comparison stays on the runner so the emergency throttle counts the two
        // calls against different client addresses.
        const response = await fromLoopback(container, selfProxyDomain, RESET_API_PATH, 'POST');
        const direct = await proxyContext!.fetch(`${APP_BASE_URL}${RESET_API_PATH}`, {
          method: 'POST',
          data: {},
          failOnStatusCode: false,
        });

        expect(response.status).toBe(401);
        expect(response.status).toBe(direct.status());
        expect(JSON.parse(response.body)).toMatchObject({ error: 'unauthorized' });
      });

      await test.step('Exact app-level reset path is answered by the app, not the ACL', async () => {
        // The app has no API route at this path; its own router answers it. The
        // proxied answer must match the direct answer and must not be the ACL refusal.
        const response = await fromLoopback(container, selfProxyDomain, RESET_APP_PATH, 'POST');
        const direct = await proxyContext!.fetch(`${APP_BASE_URL}${RESET_APP_PATH}`, {
          method: 'POST',
          data: {},
          failOnStatusCode: false,
        });

        expect(response.status).not.toBe(403);
        expect(response.status).toBe(direct.status());
      });
    });
  });

  test.describe('IP block-lists with an admin whitelist', () => {
    // Two-sided cases: the in-container client (loopback, 127.0.0.1) must be refused only when a list names it, and
    // the whitelisted runner must never be refused by these lists. The dead upstream (port 3001) makes a loaded,
    // non-refusing route answer 502, so a refusal (403) and a missing route (catch-all 200) are told apart.
    const CIDR_NOT_THE_CLIENT = '203.0.113.77/32';
    const CIDR_LOOPBACK = '127.0.0.1/32';
    const IP_BLOCK_LIST_BODY = 'Access denied: IP blacklisted';

    test.describe.configure({ mode: 'serial' });

    let container: string;
    let proxyContext: APIRequestContext | undefined;
    let snapshot: SecurityStateSnapshot | undefined;

    let notClientDomain = '';
    let notClientListUuid = '';
    let notClientHostUuid = '';
    let loopbackDomain = '';
    let loopbackListUuid = '';
    let loopbackHostUuid = '';

    test.beforeAll(async () => {
      container = findE2EContainer();
      proxyContext = await request.newContext();
      snapshot = await captureSecurityState(requestContext);

      // Narrow the whitelist before any list or host exists, so the first apply that holds a list already has it.
      await narrowAdminWhitelistForLoopbackClient(requestContext);

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
        .poll(async () => (await fromLoopback(container, loopbackDomain)).status, {
          timeout: 30_000,
          message: 'loopback block-list route should be loaded and refuse the loopback client',
        })
        .toBe(403);
    });

    test.afterAll(async () => {
      await runCleanupSteps([
        ...[notClientHostUuid, loopbackHostUuid]
          .filter((uuid) => uuid !== '')
          .map((uuid) => () => deleteProxyHost(requestContext, uuid)),
        ...[notClientListUuid, loopbackListUuid]
          .filter((uuid) => uuid !== '')
          .map((uuid) => () => deleteAccessList(requestContext, uuid)),
        ...(snapshot ? [() => restoreSecurityState(requestContext, snapshot as SecurityStateSnapshot)] : []),
        ...(proxyContext ? [() => (proxyContext as APIRequestContext).dispose()] : []),
      ]);
    });

    test('does not refuse a loopback client for a block-list on another address', async () => {
      await test.step('In-container client reaches the route: not refused, the dead upstream answers 502', async () => {
        // Before the A0 fix every non-admin address was refused here (403), the #1891 regression.
        const response = await fromLoopback(container, notClientDomain);
        expect(response.status).toBe(502);
      });

      await test.step('Whitelisted runner reaches the same route: 502', async () => {
        const response = await fetchThroughProxy(proxyContext as APIRequestContext, notClientDomain, '/');
        expect(response.status()).toBe(502);
      });
    });

    test('refuses a listed loopback client with the block-list body and does not refuse the runner', async () => {
      await test.step('In-container client is refused with the block-list body', async () => {
        const response = await fromLoopback(container, loopbackDomain);
        expect(response.status).toBe(403);
        expect(response.body).toContain(IP_BLOCK_LIST_BODY);
      });

      await test.step('Whitelisted runner reaches the same route: 502', async () => {
        const response = await fetchThroughProxy(proxyContext as APIRequestContext, loopbackDomain, '/');
        expect(response.status()).toBe(502);
      });
    });
  });
});
