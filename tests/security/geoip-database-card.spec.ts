/**
 * GeoIP database card on the Security page (spec 9.3 and A.6, C1).
 *
 * The admin card shows the database source and path, reloads the database, looks up an address, replaces the data-folder
 * copy from a browser upload, refuses a text file, and removes the data-folder copy again. The upload and removal routes
 * refuse cross-origin requests (403 `cross_origin_refused`) before the handler runs.
 *
 * Serial: each test builds on the database state the previous one leaves. The bundled copy is never hidden here; the
 * data-folder copy is created by the replace test and removed by the last one.
 *
 * Labels come from the en locale (`security.geoip` in frontend/src/locales/en). The lookup lives in GeoIPLookupForm and
 * its result paragraph carries `data-testid="geoip-lookup-result"` with no role, so the card's own `role="status"`
 * region stays the only status region in the card.
 */

import { request } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';
import { test, expect } from '../fixtures/test';
import { STORAGE_STATE } from '../constants';
import { findE2EContainer } from '../utils/container-exec';
import { restoreLeftoverGeoIPDatabases } from '../utils/geoip-container';
import { loadGeoIPUploadFixtures, type GeoIPUploadFixtures, type UploadFile } from '../utils/geoip-fixture';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
const STATUS_PATH = '/api/v1/security/geoip/status';
const DATABASE_PATH = '/api/v1/security/geoip/database';
const RELOAD_PATH = '/api/v1/security/geoip/reload';
const ALERTS_PATH = '/api/v1/notifications/alerts';
const ALERT_KEY = 'geoip_unavailable';
const BUNDLED_PATH_TEXT = '/app/geoip/GeoLite2-Country.mmdb';

/** Card region name (A.4 title, en). */
const CARD_NAME = 'Country database (GeoIP)';
const RELOAD_NAME = 'Reload';
const REPLACE_NAME = 'Replace database…';
const USE_INCLUDED_NAME = 'Use the included copy';
/** Visible text of the data-folder source (A.6, en). */
const DATA_SOURCE_TEXT = 'Your data folder';
/** Text of the refusal for a file that is not a GeoIP database (A.6, en). */
const NOT_A_DATABASE_TEXT = /not a GeoIP database/i;
/** Lookup form labels (security.geoip.lookup.label and .submit, en). */
const LOOKUP_FIELD_NAME = 'IP address';
const LOOKUP_SUBMIT_NAME = 'Look up';
/** Lookup result paragraph (GeoIPLookupForm). It has no role, so it is located by test id. */
const LOOKUP_RESULT_TEST_ID = 'geoip-lookup-result';

type GeoIPSource = 'bundled' | 'data' | 'custom' | 'none';

interface GeoIPStatusBody {
  source: GeoIPSource;
  data_copy_present: boolean;
}

async function readStatus(requestContext: APIRequestContext): Promise<GeoIPStatusBody> {
  const response = await requestContext.get(STATUS_PATH);
  expect(response.ok()).toBe(true);
  return (await response.json()) as GeoIPStatusBody;
}

test.describe('GeoIP database card', () => {
  test.describe.configure({ mode: 'serial' });

  let requestContext: APIRequestContext;
  let fixtures: GeoIPUploadFixtures;
  const baseHostname = new URL(APP_BASE_URL).hostname;

  test.beforeAll(async () => {
    // Loading the fixtures copies the bundled database, so no earlier interrupted run may still have it hidden.
    restoreLeftoverGeoIPDatabases();
    fixtures = loadGeoIPUploadFixtures(findE2EContainer());
    requestContext = await request.newContext({ baseURL: APP_BASE_URL, storageState: STORAGE_STATE });
  });

  test.afterAll(async () => {
    // Remove the data-folder copy if a failed run left it, then reload so the running config matches the bundled copy.
    // A 404 means there was no data-folder copy, which is the expected state.
    const removal = await requestContext.delete(DATABASE_PATH);
    if (!removal.ok() && removal.status() !== 404) {
      throw new Error(`removing the data-folder GeoIP copy failed: ${removal.status()}`);
    }
    const reload = await requestContext.post(RELOAD_PATH);
    if (!reload.ok()) {
      throw new Error(`geoip reload after cleanup failed: ${reload.status()}`);
    }
    await requestContext.dispose();
  });

  test('Admin sees the card with the bundled source and path, and no removal action', async ({ page }) => {
    await page.goto('/security');
    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('Card shows the path, Reload and Replace', async () => {
      await expect(card).toContainText(BUNDLED_PATH_TEXT);
      await expect(card.getByRole('button', { name: RELOAD_NAME })).toBeVisible();
      await expect(card.getByRole('button', { name: REPLACE_NAME })).toBeVisible();
    });

    await test.step('The bundled source has no removal action', async () => {
      await expect(card.getByRole('button', { name: USE_INCLUDED_NAME })).toHaveCount(0);
      expect((await readStatus(requestContext)).source).toBe('bundled');
    });
  });

  test('Reload reports its result inline in a polite status region', async ({ page }) => {
    await page.goto('/security');
    const card = page.getByRole('region', { name: CARD_NAME });

    await card.getByRole('button', { name: RELOAD_NAME }).click();

    const result = card.getByRole('status');
    await expect(result).toHaveAttribute('aria-live', 'polite');
    await expect(result).not.toBeEmpty();
  });

  test('Lookup of 127.0.0.1 reports that no country is found', async ({ page }) => {
    await page.goto('/security');

    await page.getByRole('textbox', { name: LOOKUP_FIELD_NAME }).fill('127.0.0.1');
    await page.getByRole('button', { name: LOOKUP_SUBMIT_NAME }).click();

    // security.geoip.lookup.notFound (en): "No country found for {{ip}}."
    await expect(page.getByTestId(LOOKUP_RESULT_TEST_ID)).toHaveText('No country found for 127.0.0.1.');
  });

  test('Replacing with a generated database uses the data folder and offers the included copy', async ({ page }) => {
    await page.goto('/security');
    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('Browser upload from the card (same origin) is accepted', async () => {
      await uploadThroughCard(page, fixtures.countryDatabase);

      await expect(card).toContainText(DATA_SOURCE_TEXT);
      await expect(card.getByRole('status')).not.toBeEmpty();
    });

    await test.step('Status reports the data-folder source with its copy present', async () => {
      const status = await readStatus(requestContext);
      expect(status.source).toBe('data');
      expect(status.data_copy_present).toBe(true);
    });

    await test.step('The included-copy action is offered', async () => {
      await expect(card.getByRole('button', { name: USE_INCLUDED_NAME })).toBeVisible();
    });
  });

  test('Replacing with a text file is refused and the data-folder copy stays', async ({ page }) => {
    await page.goto('/security');
    const card = page.getByRole('region', { name: CARD_NAME });

    await uploadThroughCard(page, fixtures.textFile);

    await expect(card.getByRole('status')).toContainText(NOT_A_DATABASE_TEXT);
    expect((await readStatus(requestContext)).source).toBe('data');
  });

  test('Cross-origin upload and removal are refused with 403 before the handler runs, and the data-folder copy stays', async () => {
    await test.step('POST from a sibling origin with Sec-Fetch-Site same-site is refused', async () => {
      const response = await requestContext.post(DATABASE_PATH, {
        headers: { 'Sec-Fetch-Site': 'same-site', Origin: `https://other.${baseHostname}` },
        multipart: { file: fixtures.countryDatabase },
        failOnStatusCode: false,
      });
      expect(response.status()).toBe(403);
      expect(await response.json()).toMatchObject({ error_code: 'cross_origin_refused' });
    });

    await test.step('DELETE from an unrelated origin is refused', async () => {
      const response = await requestContext.delete(DATABASE_PATH, {
        headers: { Origin: 'https://evil.example' },
        failOnStatusCode: false,
      });
      expect(response.status()).toBe(403);
      expect(await response.json()).toMatchObject({ error_code: 'cross_origin_refused' });
    });

    await test.step('The refused removal left the data-folder copy in place', async () => {
      const status = await readStatus(requestContext);
      expect(status.source).toBe('data');
      expect(status.data_copy_present).toBe(true);
    });
  });

  test('Use the included copy asks for confirmation, removes the data-folder copy, and raises no alert', async ({ page }) => {
    await page.goto('/security');
    const card = page.getByRole('region', { name: CARD_NAME });

    await test.step('Confirmation dialog appears before anything is removed', async () => {
      await card.getByRole('button', { name: USE_INCLUDED_NAME }).click();
      await expect(page.getByRole('dialog')).toBeVisible();
    });

    await test.step('Confirming removes the data-folder copy and returns to the bundled source', async () => {
      await page.getByRole('dialog').getByRole('button', { name: USE_INCLUDED_NAME }).click();

      await expect(card.getByRole('button', { name: USE_INCLUDED_NAME })).toHaveCount(0);
      const status = await readStatus(requestContext);
      expect(status.source).toBe('bundled');
      expect(status.data_copy_present).toBe(false);
    });

    await test.step('No GeoIP alert is raised while the bundled copy is in use', async () => {
      const response = await requestContext.get(ALERTS_PATH);
      expect(response.ok()).toBe(true);
      const { alerts } = (await response.json()) as { alerts: Array<{ key: string }> };
      expect(alerts.map((alert) => alert.key)).not.toContain(ALERT_KEY);
    });
  });
});

/**
 * Uploads a file through the card's Replace button, the way a browser user does: the button opens the file chooser
 * and the chosen file is sent by the page itself (same origin).
 */
async function uploadThroughCard(page: Page, file: UploadFile): Promise<void> {
  const card = page.getByRole('region', { name: CARD_NAME });
  const [chooser] = await Promise.all([
    page.waitForEvent('filechooser'),
    card.getByRole('button', { name: REPLACE_NAME }).click(),
  ]);
  await chooser.setFiles(file);
}
