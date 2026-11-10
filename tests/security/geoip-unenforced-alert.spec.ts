/**
 * GeoIP missing-database alert (spec 9.3, C1).
 *
 * With the GeoIP database hidden, a country list on an enabled host is not enforced. The app must say so: the reload
 * endpoint answers 500 without a path, the loopback client is no longer refused, the notification bell shows one
 * red error alert, and the access list status reports `geoip_unavailable`. Restoring the database clears the alert.
 *
 * Every test is `test.fixme` until C5 (the unenforced behaviour and the alert) lands; C5 enables them.
 *
 * The runner is admin-whitelisted, so the refusal side uses the in-container loopback client with a narrowed admin
 * whitelist (see utils/loopback-client-case.ts). Each assertion checks both the loopback client and the runner.
 */

import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { test, expect } from '../fixtures/test';
import { STORAGE_STATE } from '../constants';
import {
  captureSecurityState,
  restoreSecurityState,
  setSecurityModuleEnabled,
  type SecurityStateSnapshot,
} from '../utils/security-helpers';
import { createCountryAllowList, createProxyHost, deleteAccessList, deleteProxyHost } from '../utils/proxy-refusal-helpers';
import { fetchFromContainer, findE2EContainer } from '../utils/container-exec';
import { narrowAdminWhitelistForLoopbackClient, runCleanupSteps } from '../utils/loopback-client-case';
import { hideGeoIPDatabases, restoreGeoIPDatabases, restoreLeftoverGeoIPDatabases } from '../utils/geoip-container';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
/** Port nothing listens on, so a route that is loaded and admits a request answers 502. */
const DEAD_UPSTREAM_PORT = 3001;
const ALERT_KEY = 'geoip_unavailable';
/** Visible text of the alert title (notificationCenter.alerts.geoip_unavailable.title, en). */
const ALERT_TITLE = 'Country access lists are not being enforced';
/** Section that holds the bell's alerts (notificationCenter.alerts.label, en). Assumed to be a labelled region. */
const ALERTS_SECTION_NAME = 'Needs attention';
/** Bell accessible name with one unread item (notificationCenter.bellLabel, en). */
const BELL_NAME_ONE_NEW = 'Notifications, 1 new';
/** Docs and issue link labels (notificationCenter.alerts.geoip_unavailable.*, en). */
const DOCS_LINK_NAME = 'How to provide the database';
const ISSUE_LINK_NAME = 'Open an issue';
const DISMISS_NAME = 'Dismiss';
const MARK_ALL_READ_NAME = 'Mark all read';
/** Not-enforced note on the Security page (security.enforcement.acl.geoip_unavailable, C5, en). */
const SECURITY_PAGE_NOTE = 'Country lists are not enforced: the GeoIP database is missing';
/** Bell refreshes its alerts every 30 seconds, so the alert can take up to one poll to appear. */
const ALERT_POLL_TIMEOUT_MS = 35_000;

interface AlertBody {
  key: string;
  severity: string;
  affected_lists?: string[];
  affected_lists_total?: number;
  database_available?: boolean;
}

function fromLoopback(container: string, domain: string) {
  return fetchFromContainer(container, { host: domain, path: '/' });
}

async function reloadGeoIPDatabase(requestContext: APIRequestContext) {
  return requestContext.post('/api/v1/security/geoip/reload');
}

async function getAlerts(requestContext: APIRequestContext): Promise<AlertBody[]> {
  const response = await requestContext.get('/api/v1/notifications/alerts');
  expect(response.ok()).toBe(true);
  return ((await response.json()) as { alerts: AlertBody[] }).alerts;
}

test.describe('GeoIP unenforced alert', () => {
  test.describe.configure({ mode: 'serial' });

  const domain = `geoip-unenforced-${Date.now()}.example.test`;
  const listName = `GeoIP alert list ${Date.now()}`;
  let container: string;
  let requestContext: APIRequestContext;
  let proxyContext: APIRequestContext;
  let snapshot: SecurityStateSnapshot | undefined;
  let aclUuid: string | undefined;
  let proxyUuid: string | undefined;

  test.beforeAll(async () => {
    container = findE2EContainer();
    // A leftover from an interrupted run would leave the first test with no database to hide.
    restoreLeftoverGeoIPDatabases();

    requestContext = await request.newContext({ baseURL: APP_BASE_URL, storageState: STORAGE_STATE });
    proxyContext = await request.newContext();
    snapshot = await captureSecurityState(requestContext);

    await setSecurityModuleEnabled(requestContext, 'cerberus', true);
    await setSecurityModuleEnabled(requestContext, 'acl', true);
    // The narrowed whitelist must be in place before any host or list is created, so the first applied config holds it.
    await narrowAdminWhitelistForLoopbackClient(requestContext);

    // A country list for Germany: the loopback client has no country, so it is refused while the database is loaded.
    aclUuid = await createCountryAllowList(requestContext, listName, 'DE');
    proxyUuid = await createProxyHost(requestContext, {
      domain,
      forwardPort: DEAD_UPSTREAM_PORT,
      name: 'GeoIP alert host',
      accessListUuid: aclUuid,
    });

    // Only a refusal ends the poll: a route that is not loaded answers the catch-all page with 200.
    await expect
      .poll(async () => (await fromLoopback(container, domain)).status, {
        timeout: 30_000,
        message: 'country list route should be loaded and refuse the loopback client',
      })
      .toBe(403);
  });

  test.afterAll(async () => {
    await runCleanupSteps([
      async () => {
        restoreGeoIPDatabases();
      },
      async () => {
        const response = await reloadGeoIPDatabase(requestContext);
        if (!response.ok()) {
          throw new Error(`geoip reload after cleanup failed: ${response.status()}`);
        }
      },
      ...(proxyUuid ? [() => deleteProxyHost(requestContext, proxyUuid as string)] : []),
      ...(aclUuid ? [() => deleteAccessList(requestContext, aclUuid as string)] : []),
      ...(snapshot ? [() => restoreSecurityState(requestContext, snapshot as SecurityStateSnapshot)] : []),
      () => proxyContext.dispose(),
      () => requestContext.dispose(),
    ]);
  });

  test.fixme('Hidden database: reload answers 500 without a path, the list is not enforced, and the bell shows the red alert', async ({ page }) => {
    await test.step('Hide the database and reload: 500 geoip_reload_failed with no path in the body', async () => {
      expect(hideGeoIPDatabases()).toBe(true);

      const response = await reloadGeoIPDatabase(requestContext);
      expect(response.status()).toBe(500);
      const body = await response.text();
      expect(body).toContain('geoip_reload_failed');
      expect(body).not.toContain('/app/');
    });

    await test.step('Config is applied without country matching: the loopback client is no longer refused', async () => {
      // The config is applied even though the reload failed (C5), so no other trigger is needed.
      await expect
        .poll(async () => (await fromLoopback(container, domain)).status, {
          timeout: 30_000,
          message: 'country list should stop refusing once the database is missing',
        })
        .not.toBe(403);
    });

    await test.step('The alert is in the API with the list name, and the access list status reports geoip_unavailable', async () => {
      const alerts = await getAlerts(requestContext);
      const alert = alerts.find((candidate) => candidate.key === ALERT_KEY);
      expect(alert).toBeDefined();
      expect(alert).toMatchObject({
        key: ALERT_KEY,
        severity: 'error',
        affected_lists: [listName],
        affected_lists_total: 1,
        database_available: false,
      });

      const statusResponse = await requestContext.get('/api/v1/security/status');
      expect(statusResponse.ok()).toBe(true);
      const status = (await statusResponse.json()) as { acl: { reason?: string } };
      expect(status.acl.reason).toBe(ALERT_KEY);
    });

    await test.step('The bell is red, counts the alert, and lists it with its links and no dismiss control', async () => {
      await page.goto('/security');

      const bell = page.getByRole('button', { name: BELL_NAME_ONE_NEW });
      await expect(bell).toHaveClass(/red/, { timeout: ALERT_POLL_TIMEOUT_MS });

      await bell.click();
      const alerts = page.getByRole('region', { name: ALERTS_SECTION_NAME });
      await expect(alerts).toContainText(ALERT_TITLE);
      await expect(alerts).toContainText(listName);

      await expect(alerts.getByRole('link', { name: DOCS_LINK_NAME })).toHaveAttribute(
        'href',
        /\/troubleshooting\/geoip-database/
      );
      await expect(alerts.getByRole('link', { name: ISSUE_LINK_NAME })).toHaveAttribute(
        'href',
        /github\.com\/.+\/issues/
      );
      await expect(alerts.getByRole('button', { name: DISMISS_NAME })).toHaveCount(0);
    });

    await test.step('Mark all read does not clear the alert', async () => {
      await page.getByRole('button', { name: MARK_ALL_READ_NAME }).click();

      await expect(page.getByRole('region', { name: ALERTS_SECTION_NAME })).toContainText(ALERT_TITLE);
      await expect(page.getByRole('button', { name: BELL_NAME_ONE_NEW })).toBeVisible();
    });

    await test.step('The Security page shows the not-enforced note on the access list card', async () => {
      await expect(page.getByRole('main')).toContainText(SECURITY_PAGE_NOTE);
    });
  });

  test.fixme('Restored database: reload succeeds, the alert disappears, and the loopback client is refused again', async ({ page }) => {
    await test.step('Restore the database and reload: 200 with the status', async () => {
      expect(restoreGeoIPDatabases()).toBe(true);

      const response = await reloadGeoIPDatabase(requestContext);
      expect(response.status()).toBe(200);
      expect(await response.json()).toHaveProperty('status');
    });

    await test.step('The loopback client is refused again', async () => {
      await expect
        .poll(async () => (await fromLoopback(container, domain)).status, {
          timeout: 30_000,
          message: 'country list should refuse the loopback client once the database is back',
        })
        .toBe(403);
    });

    await test.step('The alert is gone from the API', async () => {
      await expect
        .poll(async () => (await getAlerts(requestContext)).map((alert) => alert.key), {
          timeout: ALERT_POLL_TIMEOUT_MS,
        })
        .not.toContain(ALERT_KEY);
    });

    await test.step('The bell has no alert after a page reload', async () => {
      await page.goto('/security');
      await expect(page.getByRole('button', { name: BELL_NAME_ONE_NEW })).toHaveCount(0, {
        timeout: ALERT_POLL_TIMEOUT_MS,
      });
    });
  });
});
