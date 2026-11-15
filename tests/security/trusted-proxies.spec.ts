/**
 * Trusted proxies card on the Security page (PR 25, commit 13: end-to-end coverage).
 *
 * The proxy layer trusts only the addresses listed under Security > Trusted proxies (loopback is always trusted).
 * The E2E compose file sets a broad CHARON_TRUSTED_PROXIES value, so every test PUTs an explicit proxy-layer list first
 * and afterEach puts back the state captured before the test (see tests/utils/trusted-proxies-api.ts). The outcome
 * therefore never depends on the environment list.
 *
 * English labels below come from the en locale (security.trustedProxies.*) and must be kept in sync with it.
 */

import { request as playwrightRequest, type APIRequestContext } from '@playwright/test';
import { test, expect, loginUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { STORAGE_STATE } from '../constants';
import { suppressChangelogModal } from '../utils/api-helpers';
import {
  TRUSTED_PROXIES_PATH,
  canonicalProxies,
  readTrustedProxies,
  putTrustedProxies,
  restoreTrustedProxies,
  type TrustedProxiesBody,
} from '../utils/trusted-proxies-api';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';

/** Card region and the textarea (one entry per line) share this accessible name. */
const CARD_NAME = 'Trusted proxies';
const FIELD_NAME = 'Trusted proxies';
const SAVE_NAME = 'Save';
const RESET_NAME = 'Reset to default';
/** Status line after a successful save, and after a reset. */
const SAVED_TEXT = 'Trusted proxies saved.';
const RESET_TEXT = /Saved list cleared/;
/** Always-trusted loopback entries shown read-only on the card. */
const LOOPBACK_ENTRIES = ['127.0.0.1/32', '::1/128'];
/** Read-only note about the API-layer list, which only CHARON_TRUSTED_PROXIES sets. */
const API_LAYER_NOTE = /CHARON_TRUSTED_PROXIES/;
/** Sentence shown while the stored list is in force (sourceSetting). */
const SOURCE_SETTING_TEXT = /Using the list saved here/;
/** Sentence shown while the environment or the built-in default is in force (sourceEnvironment, sourceDefault). */
const SOURCE_FALLBACK_TEXT = /Using (CHARON_TRUSTED_PROXIES from the environment|the default)/;
/** Passive hint for non-admins (permissions.adminOnlyError). */
const ADMIN_ONLY_NOTE = /Only administrators can change this setting/;
/** Documentation-range addresses (RFC 5737), so no test ever trusts the runner's own network. */
const PROXY_ENTRY = '198.51.100.7';
const OTHER_PROXY_ENTRY = '198.51.100.8';
const INVALID_ENTRY = 'not-an-address';
/** Wider than the /8 limit for IPv4, so the API refuses it as too broad. */
const TOO_BROAD_ENTRY = '10.0.0.0/4';

test.describe('Trusted proxies card', () => {
  test.describe.configure({ mode: 'serial' });

  let api: APIRequestContext;
  let previous: TrustedProxiesBody | undefined;

  test.beforeEach(async () => {
    api = await playwrightRequest.newContext({ baseURL: APP_BASE_URL, storageState: STORAGE_STATE });
    previous = await readTrustedProxies(api);
    // Explicit proxy-layer list: the E2E environment's broad CHARON_TRUSTED_PROXIES must not decide the outcome.
    await putTrustedProxies(api, [PROXY_ENTRY]);
  });

  test.afterEach(async () => {
    try {
      if (previous) {
        await restoreTrustedProxies(api, previous);
      }
    } finally {
      await api.dispose();
    }
  });

  test('Admin sees the trusted proxies card with the loopback and API-layer entries', async ({ page }) => {
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
      // Partial template: the card also holds the explanatory text, the list and the docs link, which are not pinned.
      await expect(card).toMatchAriaSnapshot(`
        - region "${CARD_NAME}":
          - textbox "${FIELD_NAME}"
          - button "${SAVE_NAME}"
      `);
    });
  });

  test('Invalid entry shows an error under the field and nothing is saved', async ({ page }) => {
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
      expect(canonicalProxies(body.proxies)).toEqual(canonicalProxies([PROXY_ENTRY]));
    });
  });

  test('Saved list persists after reload and is shown in the field', async ({ page }) => {
    await page.goto('/security');

    const card = page.getByRole('region', { name: CARD_NAME });
    const field = card.getByRole('textbox', { name: FIELD_NAME });

    await test.step('Save a second proxy entry', async () => {
      await field.fill(`${PROXY_ENTRY}\n${OTHER_PROXY_ENTRY}`);
      await card.getByRole('button', { name: SAVE_NAME }).click();
      await expect(card.getByRole('status')).toHaveText(SAVED_TEXT);
    });

    await test.step('Reload shows both entries and the API reports the setting as the source', async () => {
      await page.reload();
      const reloadedCard = page.getByRole('region', { name: CARD_NAME });
      await expect(reloadedCard.getByRole('textbox', { name: FIELD_NAME })).toHaveValue(
        new RegExp(`${PROXY_ENTRY}[\\s\\S]*${OTHER_PROXY_ENTRY}`)
      );

      const body = await readTrustedProxies(api);
      expect(body.source).toBe('setting');
      expect(canonicalProxies(body.proxies)).toEqual(canonicalProxies([PROXY_ENTRY, OTHER_PROXY_ENTRY]));
    });
  });

  test('Source is named passively and reset returns to the environment or default', async ({ page }) => {
    await page.goto('/security');

    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('A stored list is reported as the saved source', async () => {
      await expect(card).toContainText(SOURCE_SETTING_TEXT);
      await expect(card.getByRole('button', { name: RESET_NAME })).toBeVisible();
    });

    await test.step('Reset clears the stored list and the card names the fallback source', async () => {
      await card.getByRole('button', { name: RESET_NAME }).click();
      await expect(card.getByRole('status')).toContainText(RESET_TEXT);
      await expect(card).toContainText(SOURCE_FALLBACK_TEXT);
      const body = await readTrustedProxies(api);
      expect(body.source).not.toBe('setting');
    });

    await test.step('Every dropped environment entry is listed by the card', async () => {
      // Dropped entries only exist when CHARON_TRUSTED_PROXIES is in use; the set is empty otherwise.
      const body = await readTrustedProxies(api);
      for (const entry of body.dropped_entries) {
        await expect(card).toContainText(entry);
      }
    });
  });

  test('Standard user does not see the editable list and sees the admin-only note', async ({ page, regularUser }) => {
    // The regular user is new and would otherwise see the blocking "What's New" dialog over the page.
    await page.goto('/');
    await suppressChangelogModal(page, regularUser.email, TEST_PASSWORD);
    await loginUser(page, regularUser);
    await page.goto('/security');

    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('No editable field or Save for a non-admin', async () => {
      await expect(card.getByRole('textbox', { name: FIELD_NAME })).toHaveCount(0);
      await expect(card.getByRole('button', { name: SAVE_NAME })).toHaveCount(0);
    });

    await test.step('The card explains that only administrators can change the list', async () => {
      await expect(card).toContainText(ADMIN_ONLY_NOTE);
    });
  });
});

test.describe('Trusted proxies API contract', () => {
  test.describe.configure({ mode: 'serial' });

  let api: APIRequestContext;
  let previous: TrustedProxiesBody | undefined;

  test.beforeEach(async () => {
    api = await playwrightRequest.newContext({ baseURL: APP_BASE_URL, storageState: STORAGE_STATE });
    previous = await readTrustedProxies(api);
    await putTrustedProxies(api, [PROXY_ENTRY]);
  });

  test.afterEach(async () => {
    try {
      if (previous) {
        await restoreTrustedProxies(api, previous);
      }
    } finally {
      await api.dispose();
    }
  });

  test('An unparsable entry is refused with its code and the entry named', async () => {
    const response = await api.put(TRUSTED_PROXIES_PATH, { data: { proxies: [INVALID_ENTRY] } });
    expect(response.status()).toBe(400);
    expect(await response.json()).toMatchObject({ error_code: 'trusted_proxy_invalid', entry: INVALID_ENTRY });
  });

  test('A range wider than the limit is refused as too broad', async () => {
    const response = await api.put(TRUSTED_PROXIES_PATH, { data: { proxies: [TOO_BROAD_ENTRY] } });
    expect(response.status()).toBe(400);
    expect(await response.json()).toMatchObject({ error_code: 'trusted_proxy_too_broad', entry: TOO_BROAD_ENTRY });
  });

  test('An explicit list is stored as the setting and a refused save leaves it unchanged', async () => {
    await test.step('Store two entries', async () => {
      await putTrustedProxies(api, [PROXY_ENTRY, OTHER_PROXY_ENTRY]);
      const body = await readTrustedProxies(api);
      expect(body.source).toBe('setting');
      expect(canonicalProxies(body.proxies)).toEqual(canonicalProxies([PROXY_ENTRY, OTHER_PROXY_ENTRY]));
    });

    await test.step('A refused save keeps the stored entries', async () => {
      const response = await api.put(TRUSTED_PROXIES_PATH, { data: { proxies: [INVALID_ENTRY] } });
      expect(response.status()).toBe(400);
      const body = await readTrustedProxies(api);
      expect(canonicalProxies(body.proxies)).toEqual(canonicalProxies([PROXY_ENTRY, OTHER_PROXY_ENTRY]));
    });
  });

  test('Clearing the stored list moves the source off the setting', async () => {
    const response = await api.delete(TRUSTED_PROXIES_PATH);
    expect(response.ok()).toBe(true);
    const body = await readTrustedProxies(api);
    expect(body.source).not.toBe('setting');
  });
});
