/**
 * Management access on the Access Lists page (PR 25, commit 1: pending specs).
 *
 * Access lists chosen under Management access protect Charon's own /api/v1 interface. They are chosen on the Access
 * Lists page, stored as a set of list UUIDs, and checked against the admin's own address before a save is accepted.
 * Every test here is `test.fixme` until the card lands (PR 25 commit 12); the bodies describe the intended behaviour.
 *
 * The proxy-layer trust list is set explicitly and restored, because the E2E compose file sets a broad
 * CHARON_TRUSTED_PROXIES value (see trusted-proxies.spec.ts). The management set is restored to its previous value
 * after each test, and the lists this file creates are deleted again.
 *
 * English labels below come from the en locale (accessLists.managementAccess.*) and must be kept in sync with it.
 */

import { request as playwrightRequest, type APIRequestContext } from '@playwright/test';
import { test, expect } from '../fixtures/test';
import { STORAGE_STATE } from '../constants';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
const ACCESS_LISTS_PATH = '/api/v1/access-lists';
const TRUSTED_PROXIES_PATH = '/api/v1/security/trusted-proxies';
const MANAGEMENT_ACCESS_PATH = '/api/v1/security/management-access';
const ACCESS_LISTS_PAGE = '/security/access-lists';

/** Card region and the list of selectable access lists. */
const CARD_NAME = 'Management access';
/** Shown while no list is chosen. */
const NOT_RESTRICTED_NAME = /Not restricted/;
/** Refusal shown when a save would lock the admin out (management_access_would_lock_out). */
const LOCK_OUT_TEXT = /lock(ed)? you out|would lock/i;
/** Refusal shown when a list is still chosen for Charon's own interface (InUse). */
const IN_USE_TEXT = /used by Charon's own interface/i;
const SAVE_NAME = 'Save';
const DELETE_NAME = 'Delete';

/** Documentation-range address the runner does not connect from, so a list built on it excludes the admin. */
const ADDRESS_OUTSIDE_RUNNER = '198.51.100.0/24';

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

/** Whitelist with one CIDR rule, in the request shape the access-lists API expects (ip_rules as a JSON string). */
function allowListBody(name: string, cidr: string): Record<string, unknown> {
  return {
    name,
    type: 'whitelist',
    ip_rules: JSON.stringify([{ cidr, description: 'Management access E2E' }]),
  };
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
  let previousProxies: { proxies: string[]; source: string };
  const createdIds: number[] = [];

  test.beforeEach(async () => {
    api = await playwrightRequest.newContext({ baseURL: APP_BASE_URL, storageState: STORAGE_STATE });
    previousManagementSet = (await readManagementAccess(api)).lists.map((list) => list.uuid);
    const proxiesResponse = await api.get(TRUSTED_PROXIES_PATH);
    expect(proxiesResponse.ok()).toBe(true);
    previousProxies = (await proxiesResponse.json()) as { proxies: string[]; source: string };
    await putManagementAccess(api, []);
    const proxyResponse = await api.put(TRUSTED_PROXIES_PATH, { data: { proxies: PROXY_LAYER_LIST } });
    expect(proxyResponse.ok()).toBe(true);
  });

  test.afterEach(async () => {
    // Clear the management set first so the lists can be deleted, then put back the stored state.
    await putManagementAccess(api, []);
    for (const id of createdIds.splice(0)) {
      await api.delete(`${ACCESS_LISTS_PATH}/${id}`);
    }
    await putManagementAccess(api, previousManagementSet);
    if (previousProxies.source === 'setting') {
      await api.put(TRUSTED_PROXIES_PATH, { data: { proxies: previousProxies.proxies } });
    }
    await api.dispose();
  });

  test.fixme('Admin chooses an access list for Charon\'s own interface and saves', async ({ page }) => {
    const list = await createAccessList(api, allowListBody(uniqueListName('Management access allow'), '10.0.0.0/8'));
    createdIds.push(list.id);

    await page.goto(ACCESS_LISTS_PAGE);
    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('Card starts as not restricted with no list chosen', async () => {
      await expect(card.getByText(NOT_RESTRICTED_NAME)).toBeVisible();
    });

    await test.step('Choose the list and save', async () => {
      await card.getByRole('checkbox', { name: list.name }).check();
      await card.getByRole('button', { name: SAVE_NAME }).click();
    });

    await test.step('The saved set contains the chosen list and the page reflects it', async () => {
      const body = await readManagementAccess(api);
      expect(body.lists.map((entry) => entry.uuid)).toContain(list.uuid);
      await page.reload();
      await expect(page.getByRole('region', { name: CARD_NAME }).getByRole('checkbox', { name: list.name })).toBeChecked();
    });
  });

  test.fixme('A choice that would refuse the admin is refused with the lock-out message', async ({ page }) => {
    // The admin's address is outside this allow-list, so enforcing it would refuse the admin's own requests.
    const list = await createAccessList(api, allowListBody(uniqueListName('Management access outside'), ADDRESS_OUTSIDE_RUNNER));
    createdIds.push(list.id);

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

  test.fixme('A list in use by Charon\'s own interface cannot be deleted', async ({ page }) => {
    const list = await createAccessList(api, allowListBody(uniqueListName('Management access allow'), '10.0.0.0/8'));
    createdIds.push(list.id);
    await putManagementAccess(api, [list.uuid]);

    await page.goto(ACCESS_LISTS_PAGE);

    await test.step('Deleting the chosen list is refused with the in-use reason', async () => {
      await page.getByRole('row', { name: list.name }).getByRole('button', { name: DELETE_NAME }).click();
      await page.getByRole('dialog').getByRole('button', { name: DELETE_NAME }).click();
      await expect(page.getByRole('alert')).toContainText(IN_USE_TEXT);
    });

    await test.step('The list still exists', async () => {
      const response = await api.get(`${ACCESS_LISTS_PATH}/${list.id}`);
      expect(response.ok()).toBe(true);
    });
  });
});
