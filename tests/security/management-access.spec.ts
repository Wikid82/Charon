/**
 * Management access on the Access Lists page (PR 25, commit 13: end-to-end coverage).
 *
 * Access lists chosen under Management access protect Charon's own /api/v1 interface. They are chosen on the Access
 * Lists page, stored as a set of list UUIDs, and checked against the admin's own address before a save is accepted.
 *
 * The proxy-layer trust list is set explicitly and restored, because the E2E compose file sets a broad
 * CHARON_TRUSTED_PROXIES value (see trusted-proxies.spec.ts). The management set is restored to its previous value
 * after each test, and the lists this file creates are deleted again.
 *
 * English labels below come from the en locale (accessLists.managementAccess.* and errors.*) and must be kept in sync.
 */

import { request as playwrightRequest, type APIRequestContext } from '@playwright/test';
import { test, expect } from '../fixtures/test';
import { STORAGE_STATE } from '../constants';
import {
  captureSecurityState,
  restoreSecurityState,
  setSecurityModuleEnabled,
  type SecurityStateSnapshot,
} from '../utils/security-helpers';
import { setAdminWhitelist } from '../utils/loopback-client-case';
import { RUNNER_WHITELIST } from '../utils/security-settings-api';
import {
  readTrustedProxies,
  putTrustedProxies,
  restoreTrustedProxies,
  type TrustedProxiesBody,
} from '../utils/trusted-proxies-api';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
const ACCESS_LISTS_PATH = '/api/v1/access-lists';
const MANAGEMENT_ACCESS_PATH = '/api/v1/security/management-access';
const MY_IP_PATH = '/api/v1/system/my-ip';
const ACCESS_LISTS_PAGE = '/security/access-lists';

/** Card region and the list of selectable access lists. */
const CARD_NAME = 'Management access';
/** Shown while no list is chosen. */
const NOT_RESTRICTED_NAME = 'Not restricted';
/** Refusal shown when a save would lock the admin out (management_access_would_lock_out). */
const LOCK_OUT_TEXT = /lock(ed)? you out|would lock/i;
/** Refusal shown when a list is still chosen for Charon's own interface (access_list_in_use). */
const IN_USE_TEXT = /used by Charon's own interface/i;
const SAVE_NAME = 'Save';
const DELETE_NAME = 'Delete';
const DELETE_DIALOG_CONFIRM_NAME = 'Delete';
/** Refusal for a chosen list that no longer exists (management_list_unknown). */
const UNKNOWN_LIST_UUID = '00000000-0000-4000-8000-000000000000';

/** Documentation-range address the runner does not connect from, so a list built on it excludes the admin. */
const ADDRESS_OUTSIDE_RUNNER = '198.51.100.0/24';
/** Admin allow-list that does not admit the runner (RFC 5737 TEST-NET-3), so only a management list can admit it. */
const ADMIN_WHITELIST_WITHOUT_RUNNER = '203.0.113.0/24';

/** Proxy-layer list set explicitly, so the broad E2E environment list does not decide the outcome. */
const PROXY_LAYER_LIST = ['198.51.100.7'];

/** Unique list name so a leftover list from an interrupted run never collides with this one. */
function uniqueListName(prefix: string): string {
  return `${prefix} ${Date.now()}`;
}

interface ManagementAccessBody {
  lists: { uuid: string; name: string; type: string; enabled: boolean }[];
  enforced: boolean;
}

interface CreatedAccessList {
  id: number;
  uuid: string;
  name: string;
}

/**
 * Enabled whitelist with one CIDR rule, in the request shape the access-lists API expects (ip_rules as a JSON string).
 * It must be enabled: the lock-out guard and the enforcement only weigh enabled lists.
 */
function allowListBody(name: string, cidr: string): Record<string, unknown> {
  return {
    name,
    type: 'whitelist',
    enabled: true,
    ip_rules: JSON.stringify([{ cidr, description: 'Management access E2E' }]),
  };
}

/**
 * CIDR that admits exactly the address the API sees for this runner. A list built on it keeps the admin in, so the
 * lock-out guard accepts it.
 */
async function runnerAddressCidr(api: APIRequestContext): Promise<string> {
  const response = await api.get(MY_IP_PATH);
  expect(response.ok()).toBe(true);
  const { ip } = (await response.json()) as { ip: string };
  const bare = ip.startsWith('::ffff:') ? ip.slice('::ffff:'.length) : ip;
  return bare.includes(':') ? `${bare}/128` : `${bare}/32`;
}

async function readManagementAccess(api: APIRequestContext): Promise<ManagementAccessBody> {
  const response = await api.get(MANAGEMENT_ACCESS_PATH);
  expect(response.ok()).toBe(true);
  return (await response.json()) as ManagementAccessBody;
}

async function putManagementAccess(api: APIRequestContext, listUUIDs: string[]): Promise<void> {
  const response = await api.put(MANAGEMENT_ACCESS_PATH, { data: { list_uuids: listUUIDs } });
  expect(response.ok(), `PUT ${MANAGEMENT_ACCESS_PATH} failed with ${response.status()}`).toBe(true);
}

async function createAccessList(api: APIRequestContext, body: Record<string, unknown>): Promise<CreatedAccessList> {
  const response = await api.post(ACCESS_LISTS_PATH, { data: body });
  expect(response.ok(), `POST ${ACCESS_LISTS_PATH} failed with ${response.status()}`).toBe(true);
  return (await response.json()) as CreatedAccessList;
}

test.describe('Management access on the Access Lists page', () => {
  test.describe.configure({ mode: 'serial' });

  let api: APIRequestContext;
  let previousManagementSet: string[];
  let previousProxies: TrustedProxiesBody | undefined;
  const createdUuids: string[] = [];
  let securityApi: APIRequestContext;
  let securitySnapshot: SecurityStateSnapshot | undefined;

  test.beforeAll(async () => {
    securityApi = await playwrightRequest.newContext({ baseURL: APP_BASE_URL, storageState: STORAGE_STATE });
    securitySnapshot = await captureSecurityState(securityApi);
    // The lock-out guard judges a management choice only while access control is on, so the run turns it on. The
    // whitelist first, so the runner may enable it.
    await setAdminWhitelist(securityApi, RUNNER_WHITELIST);
    await setSecurityModuleEnabled(securityApi, 'cerberus', true);
    await setSecurityModuleEnabled(securityApi, 'acl', true);
  });

  test.beforeEach(async () => {
    api = await playwrightRequest.newContext({ baseURL: APP_BASE_URL, storageState: STORAGE_STATE });
    previousManagementSet = (await readManagementAccess(api)).lists.map((list) => list.uuid);
    previousProxies = await readTrustedProxies(api);
    await putManagementAccess(api, []);
    await putTrustedProxies(api, PROXY_LAYER_LIST);
  });

  test.afterEach(async () => {
    try {
      await setAdminWhitelist(securityApi, RUNNER_WHITELIST);
      // Clear the management set first so the lists can be deleted, then put back the stored state.
      await putManagementAccess(api, []);
      for (const uuid of createdUuids.splice(0)) {
        const response = await api.delete(`${ACCESS_LISTS_PATH}/${uuid}`);
        expect(response.ok(), `DELETE ${ACCESS_LISTS_PATH}/${uuid} failed with ${response.status()}`).toBe(true);
      }
      await putManagementAccess(api, previousManagementSet);
      if (previousProxies) {
        await restoreTrustedProxies(api, previousProxies);
      }
    } finally {
      await api.dispose();
    }
  });

  test.afterAll(async () => {
    try {
      if (securitySnapshot) {
        await restoreSecurityState(securityApi, securitySnapshot);
      }
    } finally {
      await securityApi.dispose();
    }
  });

  test('Admin chooses an access list for Charon\'s own interface and saves', async ({ page }) => {
    const list = await createAccessList(
      api,
      allowListBody(uniqueListName('Management access allow'), await runnerAddressCidr(api))
    );
    createdUuids.push(list.uuid);

    await page.goto(ACCESS_LISTS_PAGE);
    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('Card starts as not restricted with no list chosen', async () => {
      await expect(card.getByText(NOT_RESTRICTED_NAME, { exact: true })).toBeVisible();
    });

    await test.step('Choose the list and save', async () => {
      await card.getByRole('checkbox', { name: list.name }).check();
      await card.getByRole('button', { name: SAVE_NAME }).click();
      await expect(card.getByRole('status')).toContainText('Management access saved.');
    });

    await test.step('The saved set contains the chosen list and the page reflects it after reload', async () => {
      const body = await readManagementAccess(api);
      expect(body.lists.map((entry) => entry.uuid)).toContain(list.uuid);
      await page.reload();
      await expect(
        page.getByRole('region', { name: CARD_NAME }).getByRole('checkbox', { name: list.name })
      ).toBeChecked();
    });
  });

  test('A choice that would refuse the admin is refused with the lock-out message', async ({ page }) => {
    // An admin whitelisted address is exempt from the lock-out check, so the admin allow-list must leave the runner out.
    // afterEach puts the runner whitelist back.
    await setAdminWhitelist(securityApi, ADMIN_WHITELIST_WITHOUT_RUNNER);
    // The admin's address is outside this allow-list, so enforcing it would refuse the admin's own requests.
    const list = await createAccessList(
      api,
      allowListBody(uniqueListName('Management access outside'), ADDRESS_OUTSIDE_RUNNER)
    );
    createdUuids.push(list.uuid);

    await page.goto(ACCESS_LISTS_PAGE);
    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('Choose the list and save', async () => {
      await card.getByRole('checkbox', { name: list.name }).check();
      await card.getByRole('button', { name: SAVE_NAME }).click();
    });

    await test.step('The save is refused with the lock-out message and nothing is stored', async () => {
      await expect(card.getByRole('alert')).toContainText(LOCK_OUT_TEXT);
      const body = await readManagementAccess(api);
      expect(body.lists.map((entry) => entry.uuid)).not.toContain(list.uuid);
    });
  });

  test('A list in use by Charon\'s own interface cannot be deleted', async ({ page }) => {
    const list = await createAccessList(
      api,
      allowListBody(uniqueListName('Management access in use'), await runnerAddressCidr(api))
    );
    createdUuids.push(list.uuid);
    await putManagementAccess(api, [list.uuid]);

    await page.goto(ACCESS_LISTS_PAGE);

    await test.step('Deleting the chosen list is refused with the in-use reason', async () => {
      await page.getByRole('row', { name: list.name }).getByRole('button', { name: DELETE_NAME }).click();
      const dialog = page.getByRole('dialog');
      await dialog.getByRole('button', { name: DELETE_DIALOG_CONFIRM_NAME }).click();
      await expect(dialog.getByRole('alert')).toContainText(IN_USE_TEXT);
    });

    await test.step('The list still exists', async () => {
      const response = await api.get(`${ACCESS_LISTS_PATH}/${list.uuid}`);
      expect(response.ok()).toBe(true);
    });
  });

  test('Choosing a list that no longer exists is refused with its code', async () => {
    const response = await api.put(MANAGEMENT_ACCESS_PATH, { data: { list_uuids: [UNKNOWN_LIST_UUID] } });
    expect(response.status()).toBe(400);
    expect(await response.json()).toMatchObject({
      error_code: 'management_list_unknown',
      uuid: UNKNOWN_LIST_UUID,
    });
  });
});
