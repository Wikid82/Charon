/**
 * Management access API helpers: the access lists chosen for Charon's own /api/v1 interface.
 *
 * Only enabled lists count for Charon's own interface, and a list created through the API is disabled unless the body
 * says otherwise, so the helpers here always create lists enabled. With access control on, a choice that would refuse
 * the caller is refused unless the caller is admitted by the admin allow-list, so cleanup that clears the choice must
 * run while access control is off or while the caller is exempt.
 */

import { expect, type APIRequestContext, type APIResponse } from '@playwright/test';

export const ACCESS_LISTS_PATH = '/api/v1/access-lists';
export const MANAGEMENT_ACCESS_PATH = '/api/v1/security/management-access';

/** Refusal code for a choice or a change that would lock the caller out of Charon's own interface. */
export const MANAGEMENT_LOCKOUT_CODE = 'management_access_would_lock_out';

/**
 * Creates an enabled allow-list (whitelist) on one CIDR and returns its uuid. The list admits only the CIDR, so a
 * caller outside it is refused once the list is chosen for Charon's own interface.
 */
export async function createEnabledAllowList(api: APIRequestContext, name: string, cidr: string): Promise<string> {
  const response = await api.post(ACCESS_LISTS_PATH, {
    data: {
      name,
      type: 'whitelist',
      enabled: true,
      ip_rules: JSON.stringify([{ cidr, description: name }]),
    },
  });
  expect(response.ok(), `POST ${ACCESS_LISTS_PATH} failed with ${response.status()}`).toBe(true);
  return ((await response.json()) as { uuid: string }).uuid;
}

/** Returns the uuids of the lists currently chosen for Charon's own interface. */
export async function readManagementAccessUuids(api: APIRequestContext): Promise<string[]> {
  const response = await api.get(MANAGEMENT_ACCESS_PATH);
  expect(response.ok(), `GET ${MANAGEMENT_ACCESS_PATH} failed with ${response.status()}`).toBe(true);
  const body = (await response.json()) as { lists: { uuid: string }[] };
  return body.lists.map((list) => list.uuid);
}

/** Replaces the chosen lists. The response is returned so a caller can assert a refusal as well as a save. */
export function putManagementAccess(api: APIRequestContext, listUuids: string[]): Promise<APIResponse> {
  return api.put(MANAGEMENT_ACCESS_PATH, { data: { list_uuids: listUuids } });
}
