/**
 * Trusted proxies card on the Security page (PR 25, commit 1: pending specs).
 *
 * The proxy layer trusts only the addresses listed under Security > Trusted proxies (loopback is always trusted).
 * Every test here is `test.fixme` until the card lands (PR 25 commit 11); the bodies describe the intended behaviour.
 *
 * The E2E compose file sets a broad CHARON_TRUSTED_PROXIES value for the API layer, and that value is clamped for the
 * proxy layer. Each test therefore PUTs an explicit proxy-layer list first and restores the previous value in
 * afterEach, so the outcome never depends on the environment list.
 *
 * English labels below come from the en locale (security.trustedProxies.*) and must be kept in sync with it.
 */

import { request as playwrightRequest, type APIRequestContext } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { STORAGE_STATE } from '../constants';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
const TRUSTED_PROXIES_PATH = '/api/v1/security/trusted-proxies';

/** Card region and the textarea (one entry per line) share this accessible name. */
const CARD_NAME = 'Trusted proxies';
const FIELD_NAME = 'Trusted proxies';
const SAVE_NAME = 'Save';
/** Always-trusted loopback entries shown read-only on the card. */
const LOOPBACK_ENTRIES = ['127.0.0.1/32', '::1/128'];
/** Read-only note about the API-layer list, which only CHARON_TRUSTED_PROXIES sets. */
const API_LAYER_NOTE = /CHARON_TRUSTED_PROXIES/;
/** Documentation-range addresses (RFC 5737), so no test ever trusts the runner's own network. */
const PROXY_ENTRY = '198.51.100.7';
const OTHER_PROXY_ENTRY = '198.51.100.8';
const INVALID_ENTRY = 'not-an-address';
/** Passive hint for non-admins (permissions.hiddenForRole). */
const HIDDEN_FOR_ROLE_NOTE = /administrators/i;

type TrustSource = 'setting' | 'environment' | 'default';

interface TrustedProxiesBody {
  proxies: string[];
  source: TrustSource;
  dropped_entries: string[];
  always_trusted: string[];
  api_layer: string[];
  warnings: string[];
}

async function readTrustedProxies(api: APIRequestContext): Promise<TrustedProxiesBody> {
  const response = await api.get(TRUSTED_PROXIES_PATH);
  expect(response.ok()).toBe(true);
  return (await response.json()) as TrustedProxiesBody;
}

async function putTrustedProxies(api: APIRequestContext, proxies: string[]): Promise<void> {
  const response = await api.put(TRUSTED_PROXIES_PATH, { data: { proxies } });
  expect(response.ok(), `PUT ${TRUSTED_PROXIES_PATH} failed with ${response.status()}`).toBe(true);
}

test.describe('Trusted proxies card', () => {
  test.describe.configure({ mode: 'serial' });

  let api: APIRequestContext;
  let previous: TrustedProxiesBody;

  test.beforeEach(async () => {
    api = await playwrightRequest.newContext({ baseURL: APP_BASE_URL, storageState: STORAGE_STATE });
    previous = await readTrustedProxies(api);
    // Explicit proxy-layer list: the E2E environment's broad CHARON_TRUSTED_PROXIES must not decide the outcome.
    await putTrustedProxies(api, [PROXY_ENTRY]);
  });

  test.afterEach(async () => {
    // Restore the value that was stored before the test. A non-stored source (environment or default) has no
    // reset in the API contract; see the open point in the commit 1 report before un-fixme-ing these tests.
    if (previous.source === 'setting') {
      await putTrustedProxies(api, previous.proxies);
    }
    await api.dispose();
  });

  test.fixme('Admin sees the trusted proxies card with the loopback and API-layer entries', async ({ page }) => {
    await page.goto('/security');

    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('Card shows the editable list and Save', async () => {
      await expect(card.getByRole('textbox', { name: FIELD_NAME })).toBeVisible();
      await expect(card.getByRole('button', { name: SAVE_NAME })).toBeVisible();
    });

    await test.step('Always-trusted loopback entries and the API-layer note are shown read-only', async () => {
      for (const entry of LOOPBACK_ENTRIES) {
        await expect(card).toContainText(entry);
      }
      await expect(card).toContainText(API_LAYER_NOTE);
    });

    await test.step('Card structure matches the expected accessibility tree', async () => {
      await expect(card).toMatchAriaSnapshot(`
        - region "${CARD_NAME}":
          - textbox "${FIELD_NAME}"
          - button "${SAVE_NAME}"
      `);
    });
  });

  test.fixme('Invalid entry shows an error under the field and nothing is saved', async ({ page }) => {
    await page.goto('/security');

    const card = page.getByRole('region', { name: CARD_NAME });
    const field = card.getByRole('textbox', { name: FIELD_NAME });

    await test.step('Enter an address that cannot be parsed and save', async () => {
      await field.fill(INVALID_ENTRY);
      await card.getByRole('button', { name: SAVE_NAME }).click();
    });

    await test.step('The error names the entry and the stored list is unchanged', async () => {
      await expect(card.getByRole('alert')).toContainText(INVALID_ENTRY);
      const body = await readTrustedProxies(api);
      expect(body.proxies).toEqual([PROXY_ENTRY]);
    });
  });

  test.fixme('Saved list persists after reload and is shown in the field', async ({ page }) => {
    await page.goto('/security');

    const card = page.getByRole('region', { name: CARD_NAME });
    const field = card.getByRole('textbox', { name: FIELD_NAME });

    await test.step('Save a second proxy entry', async () => {
      await field.fill(`${PROXY_ENTRY}\n${OTHER_PROXY_ENTRY}`);
      await card.getByRole('button', { name: SAVE_NAME }).click();
      await expect(card.getByRole('status')).not.toBeEmpty();
    });

    await test.step('Reload shows both entries and the API reports the setting as the source', async () => {
      await page.reload();
      const reloadedCard = page.getByRole('region', { name: CARD_NAME });
      await expect(reloadedCard.getByRole('textbox', { name: FIELD_NAME })).toHaveValue(
        new RegExp(`${PROXY_ENTRY}[\\s\\S]*${OTHER_PROXY_ENTRY}`)
      );

      const body = await readTrustedProxies(api);
      expect(body.source).toBe('setting');
      expect(body.proxies).toEqual([PROXY_ENTRY, OTHER_PROXY_ENTRY].sort());
    });
  });

  test.fixme('Source and dropped environment entries are shown passively', async ({ page }) => {
    await page.goto('/security');

    const card = page.getByRole('region', { name: CARD_NAME });
    const body = await readTrustedProxies(api);

    await test.step('The card names where the stored list comes from', async () => {
      await expect(card).toContainText(body.source === 'setting' ? /saved/i : /environment|default/i);
    });

    await test.step('Every dropped environment entry is listed by the card', async () => {
      // Dropped entries only exist when CHARON_TRUSTED_PROXIES is in use; with a stored list this set is empty.
      for (const entry of body.dropped_entries) {
        await expect(card).toContainText(entry);
      }
    });
  });

  test.fixme('Standard user does not see the editable list and sees the admin-only note', async ({ page, regularUser }) => {
    await loginUser(page, regularUser);
    await page.goto('/security');

    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('No editable field or Save for a non-admin', async () => {
      await expect(card.getByRole('textbox', { name: FIELD_NAME })).toHaveCount(0);
      await expect(card.getByRole('button', { name: SAVE_NAME })).toHaveCount(0);
    });

    await test.step('The card explains that only administrators can change the list', async () => {
      await expect(card).toContainText(HIDDEN_FOR_ROLE_NOTE);
    });
  });
});
