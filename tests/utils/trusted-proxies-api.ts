/**
 * API helpers for the proxy-layer trust list (Security > Trusted proxies).
 *
 * The E2E compose file sets a broad CHARON_TRUSTED_PROXIES value, so specs must set an explicit list before it asserts
 * anything and put the previous state back afterwards. A stored list is restored with PUT; an environment or default
 * source has no stored value to write back, so it is restored with DELETE, which clears the stored list.
 */

import { expect, type APIRequestContext } from '@playwright/test';

export const TRUSTED_PROXIES_PATH = '/api/v1/security/trusted-proxies';

export type TrustSource = 'setting' | 'environment' | 'default';

export interface TrustedProxiesBody {
  proxies: string[];
  source: TrustSource;
  dropped_entries: string[];
  always_trusted: string[];
  api_layer: string[];
  warnings: string[];
}

/**
 * Sorted CIDR form of a list. The API stores a single address as a host range (`198.51.100.7` is read back as
 * `198.51.100.7/32`), so assertions compare canonical entries rather than the text that was sent.
 */
export function canonicalProxies(entries: string[]): string[] {
  return entries.map((entry) => (entry.includes('/') ? entry : `${entry}/${entry.includes(':') ? 128 : 32}`)).sort();
}

export async function readTrustedProxies(api: APIRequestContext): Promise<TrustedProxiesBody> {
  const response = await api.get(TRUSTED_PROXIES_PATH);
  expect(response.ok(), `GET ${TRUSTED_PROXIES_PATH} failed with ${response.status()}`).toBe(true);
  return (await response.json()) as TrustedProxiesBody;
}

export async function putTrustedProxies(api: APIRequestContext, proxies: string[]): Promise<void> {
  const response = await api.put(TRUSTED_PROXIES_PATH, { data: { proxies } });
  expect(response.ok(), `PUT ${TRUSTED_PROXIES_PATH} failed with ${response.status()}`).toBe(true);
}

/** Clears the stored list, so the environment list or the built-in default applies again. */
export async function clearTrustedProxies(api: APIRequestContext): Promise<void> {
  const response = await api.delete(TRUSTED_PROXIES_PATH);
  expect(response.ok(), `DELETE ${TRUSTED_PROXIES_PATH} failed with ${response.status()}`).toBe(true);
}

/**
 * Puts back the state captured by readTrustedProxies: the stored list when one was saved, otherwise a cleared list so
 * the environment or default source applies again.
 */
export async function restoreTrustedProxies(api: APIRequestContext, previous: TrustedProxiesBody): Promise<void> {
  if (previous.source === 'setting') {
    await putTrustedProxies(api, previous.proxies);
    return;
  }
  await clearTrustedProxies(api);
}
