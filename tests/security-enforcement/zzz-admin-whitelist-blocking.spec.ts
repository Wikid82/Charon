import { EMERGENCY_SERVER } from '../fixtures/security';
/**
 * Admin Whitelist IP Blocking Enforcement Tests
 *
 * Ordering: this spec is pinned in scripts/ci/security-shard-manifest.txt to the same shard as the
 * break-glass chain. It runs after emergency-reset and the non-zzz specs, and before the zzz-security-ui
 * specs and zzzz-break-glass-recovery in that shard. Its afterAll emergency reset unblocks the test IP
 * for those later specs.
 *
 * Tests validate the admin allow-list and the lists chosen for Charon's own interface (management access):
 * - turning access control on is refused when a chosen list would lock out a caller the admin allow-list does not admit;
 * - a caller the admin allow-list admits may turn it on;
 * - the emergency token bypasses that lock-out guard.
 * Access control on its own does not refuse Charon's own interface: a list must be chosen for that.
 *
 * Recovery: Uses emergency reset in afterAll to unblock test IP, after clearing the chosen lists.
 */

import { test, expect, request, APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import { RUNNER_WHITELIST } from '../utils/security-settings-api';
import { deleteAccessList } from '../utils/proxy-refusal-helpers';
import {
  createEnabledAllowList,
  MANAGEMENT_LOCKOUT_CODE,
  putManagementAccess,
} from '../utils/management-access-api';

test.describe.serial('Admin Whitelist IP Blocking (RUN LAST)', () => {
  const EMERGENCY_TOKEN = process.env.CHARON_EMERGENCY_TOKEN ?? '';
  const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
  const ACL_PATH = '/api/v1/security/acl';
  /** RFC 5737 TEST-NET-1 (documentation only). The admin allow-list admits 192.0.2.1 and nothing that the runner uses. */
  const FAKE_ADMIN_ADDRESS = '192.0.2.1/32';
  /** The chosen list admits only the documentation block, so the runner is outside it and refused by it. */
  const LIST_OUTSIDE_RUNNER = '192.0.2.0/24';
  let apiContext: APIRequestContext;
  /** Lists this spec created, deleted in afterAll once the choice for Charon's own interface is cleared. */
  const createdListUuids: string[] = [];

  test.beforeAll(async () => {
    if (!EMERGENCY_TOKEN) {
      throw new Error(
        'CHARON_EMERGENCY_TOKEN required for admin whitelist tests\n' +
        'Generate with: openssl rand -hex 32'
      );
    }

    apiContext = await request.newContext({
      baseURL: BASE_URL,
      storageState: STORAGE_STATE,
    });
  });

  test.afterAll(async ({ request }) => {
    console.log('🔧 Emergency reset - cleaning up admin whitelist test');

    // Access control must be off before the choice is cleared: while it is on and the runner is outside the admin
    // allow-list, a chosen list refuses the runner's own requests, including the call that clears the choice.
    // The reset does not clear the choice itself, so the choice is cleared next.
    try {
      const response = await request.post('http://localhost:2020/emergency/security-reset', {
        headers: {
          'Authorization': 'Basic ' + Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString('base64'),
          'X-Emergency-Token': EMERGENCY_TOKEN,
          'Content-Type': 'application/json',
        },
        data: { reason: 'E2E test cleanup - admin whitelist blocking test' },
      });

      if (response.ok()) {
        console.log('✅ Emergency reset completed - test IP unblocked');
      } else {
        console.error(`❌ Emergency reset failed: ${response.status()}`);
      }
    } catch (error) {
      console.error('Emergency reset error:', error);
    }

    try {
      const cleared = await putManagementAccess(apiContext, []);
      expect(cleared.ok(), `clearing the management choice failed with ${cleared.status()}`).toBe(true);
      for (const uuid of createdListUuids.splice(0)) {
        await deleteAccessList(apiContext, uuid);
      }
      // Later specs in this shard expect the runner to be admitted by the admin allow-list.
      await apiContext.patch('/api/v1/config', {
        data: { security: { admin_whitelist: RUNNER_WHITELIST } },
      });
    } catch (error) {
      console.error('Management access cleanup error:', error);
    }

    if (apiContext) {
      await apiContext.dispose();
    }
  });

  test('should block a caller outside the admin allow-list when a chosen list would lock it out', async () => {
    await test.step('Configure admin whitelist with non-matching IP', async () => {
      const response = await apiContext.patch(ACL_PATH, {
        data: {
          enabled: false, // Ensure disabled first
        },
      });
      expect(response.ok()).toBeTruthy();

      // Set the admin whitelist
      const configResponse = await apiContext.patch('/api/v1/config', {
        data: {
          security: {
            admin_whitelist: FAKE_ADMIN_ADDRESS,
          },
        },
      });
      expect(configResponse.ok()).toBeTruthy();
    });

    await test.step('Choose an enabled list that excludes the runner for Charon\'s own interface', async () => {
      const uuid = await createEnabledAllowList(apiContext, `Admin whitelist outside runner ${Date.now()}`, LIST_OUTSIDE_RUNNER);
      createdListUuids.push(uuid);
      // Access control is off, so the choice itself is not judged yet.
      const response = await putManagementAccess(apiContext, [uuid]);
      expect(response.ok()).toBe(true);
    });

    await test.step('Enable ACL - expect 403 because the chosen list would refuse the runner', async () => {
      const response = await apiContext.patch(ACL_PATH, {
        data: { enabled: true },
      });

      // The runner is outside the admin allow-list and outside the chosen list, so the lock-out guard refuses.
      expect(response.status()).toBe(403);

      const body = await response.json().catch(() => ({}));
      expect(body.error_code).toBe(MANAGEMENT_LOCKOUT_CODE);
    });

    await test.step('The refused enable leaves access control off', async () => {
      const response = await apiContext.get('/api/v1/security/status');
      expect(response.ok()).toBeTruthy();

      const body = await response.json();
      expect(body.acl?.enabled).toBe(false);
    });
  });

  test('should allow whitelisted IP to enable Cerberus', async () => {
    // Use localhost/Docker network IP that will match test runner
    // In Docker compose, Playwright runs from host connecting to localhost:8080
    const testWhitelist = '127.0.0.1/32,172.16.0.0/12,192.168.0.0/16,10.0.0.0/8';

    await test.step('Configure admin whitelist with test IP ranges', async () => {
      const response = await apiContext.patch('/api/v1/config', {
        data: {
          security: {
            admin_whitelist: testWhitelist,
          },
        },
      });
      expect(response.ok()).toBeTruthy();
    });

    await test.step('Enable ACL with whitelisted IP while the chosen list still excludes the runner', async () => {
      // The admin allow-list exempts the runner from the chosen list, so the same enable that was refused above is allowed.
      const response = await apiContext.patch(ACL_PATH, {
        data: { enabled: true },
      });
      expect(response.ok()).toBeTruthy();

      const body = await response.json();
      expect(body.enabled).toBe(true);
    });

    await test.step('Verify ACL is enforcing', async () => {
      const response = await apiContext.get('/api/v1/security/status');
      expect(response.ok()).toBeTruthy();

      const body = await response.json();
      expect(body.acl?.enabled).toBe(true);
    });
  });

  test('should allow emergency token to bypass the lock-out guard', async ({ request }) => {
    await test.step('Emergency reset turns access control off, then the admin whitelist excludes the runner', async () => {
      // The chosen list from the first test is still stored; access control is off, so it does not refuse anything yet.
      await request.post('http://localhost:2020/emergency/security-reset', {
        headers: {
          'Authorization': 'Basic ' + Buffer.from(`${EMERGENCY_SERVER.username}:${EMERGENCY_SERVER.password}`).toString('base64'),
          'X-Emergency-Token': EMERGENCY_TOKEN,
        },
        data: { reason: 'Test setup - reset for emergency token test' },
      });

      const response = await apiContext.patch('/api/v1/config', {
        data: {
          security: {
            admin_whitelist: FAKE_ADMIN_ADDRESS,
          },
        },
      });
      expect(response.ok()).toBeTruthy();
    });

    await test.step('Without the token the lock-out guard still refuses the enable', async () => {
      const response = await apiContext.patch(ACL_PATH, {
        data: { enabled: true },
      });
      expect(response.status()).toBe(403);
      expect(await response.json()).toMatchObject({ error_code: MANAGEMENT_LOCKOUT_CODE });
    });

    await test.step('Enable ACL using emergency token despite the lock-out', async () => {
      const response = await apiContext.patch(ACL_PATH, {
        data: { enabled: true },
        headers: {
          'X-Emergency-Token': EMERGENCY_TOKEN,
        },
      });

      // Should succeed with valid emergency token even though the runner is refused by the chosen list.
      expect(response.ok()).toBeTruthy();
    });
  });
});
