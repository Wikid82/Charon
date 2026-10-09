/**
 * CrowdSec Dashboard E2E Tests
 *
 * The E2E container has no CrowdSec engine, so every dashboard endpoint is stubbed
 * (see tests/utils/crowdsec-stubs.ts) and the assertions run against known data:
 * - Tab navigation between Configuration and Dashboard
 * - Summary cards, charts, active decisions table and alerts list
 * - Empty and error states of every panel
 * - Time range selection and refresh (asserted on the requests the UI sends)
 * - Decision export (CSV and JSON)
 *
 * @see /projects/Charon/docs/plans/current_spec.md PR-2, PR-3
 */

import type { Locator, Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import {
  crowdsecFixtures,
  stubCrowdSecApi,
  type CrowdSecStubOptions,
  type StubFailure,
} from '../utils/crowdsec-stubs';

const ALERT_TOTAL = 25;
const ALERT_PAGE_SIZE = 10;
const ERROR_TIMEOUT = 15_000;
const SERVER_ERROR: StubFailure = { failWith: 500 };

const TOP_IPS = [
  { ip: '203.0.113.1', count: 5, last_seen: '2020-01-01T00:00:00Z', country: 'DE' },
  { ip: '203.0.113.2', count: 9, last_seen: '2020-01-02T00:00:00Z', country: 'US' },
  { ip: '203.0.113.3', count: 1, last_seen: '2020-01-03T00:00:00Z', country: '' },
];

const SCENARIOS = [
  { name: 'crowdsecurity/http-probing', count: 60, percentage: 60 },
  { name: 'crowdsecurity/ssh-bf', count: 40, percentage: 40 },
];

const TIMELINE = [
  { timestamp: '2026-01-01T00:00:00Z', bans: 3, captchas: 1 },
  { timestamp: '2026-01-01T01:00:00Z', bans: 5, captchas: 0 },
];

/** Serves a page of generated alerts; alert N has IP 198.51.100.N and scenario crowdsecurity/scenario-N. */
function alertsPage(query: URLSearchParams) {
  const offset = Number(query.get('offset') ?? 0);
  const limit = Number(query.get('limit') ?? ALERT_PAGE_SIZE);
  const alerts = Array.from({ length: Math.max(0, Math.min(limit, ALERT_TOTAL - offset)) }, (_, i) => {
    const n = offset + i + 1;
    return crowdsecFixtures.alert({
      id: n,
      ip: `198.51.100.${n}`,
      scenario: `crowdsecurity/scenario-${n}`,
      events_count: n * 2,
    });
  });
  return { alerts, total: ALERT_TOTAL, source: 'lapi', cached: false };
}

/** Dashboard stub set with data in every panel; override individual panels per test. */
function dashboardOptions(
  overrides: NonNullable<CrowdSecStubOptions['dashboard']> = {},
): CrowdSecStubOptions {
  return {
    dashboard: {
      summary: crowdsecFixtures.dashboardSummary(),
      timeline: { buckets: TIMELINE, range: '24h', interval: '1h', cached: false },
      topIps: { ips: TOP_IPS, range: '24h', cached: false },
      scenarios: { scenarios: SCENARIOS, total: 100, range: '24h', cached: false },
      alerts: alertsPage,
      ...overrides,
    },
  };
}

/** Installs the stubs, loads the CrowdSec page and opens the Dashboard tab. */
async function openDashboard(page: Page, options: CrowdSecStubOptions = dashboardOptions()) {
  const recorder = await stubCrowdSecApi(page, options);
  await page.goto('/security/crowdsec');
  await waitForLoadingComplete(page);
  await page.getByRole('tab', { name: 'Dashboard', exact: true }).click();
  await expect(page.getByTestId('dashboard-summary-cards')).toBeVisible();
  return recorder;
}

/** The accessible name of this group is currently a raw i18n error string (F3), so it is located by role only. */
const timeRangeGroup = (page: Page) => page.getByRole('radiogroup');
const activeDecisionsTable = (page: Page) =>
  page.getByRole('table').filter({ has: page.getByRole('button', { name: 'Sort by Alerts' }) });
const alertsTable = (page: Page) =>
  page.getByRole('table').filter({ has: page.getByRole('columnheader', { name: 'Scenario' }) });

/** Text of the first cell of each body row, in display order. */
async function firstColumn(table: Locator): Promise<string[]> {
  return table.getByRole('row').filter({ has: table.page().getByRole('cell') }).evaluateAll((rows) =>
    rows.map((row) => row.querySelector('td')?.textContent?.trim() ?? ''),
  );
}

test.describe('CrowdSec Dashboard @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.describe('Tab Navigation', () => {
    test.beforeEach(async ({ page }) => {
      await stubCrowdSecApi(page, dashboardOptions());
      await page.goto('/security/crowdsec');
      await waitForLoadingComplete(page);
    });

    test('should display Configuration and Dashboard tabs', async ({ page }) => {
      await expect(page.getByRole('tablist')).toBeVisible();
      await expect(page.getByRole('tab', { name: 'Configuration', exact: true })).toBeVisible();
      await expect(page.getByRole('tab', { name: 'Dashboard', exact: true })).toBeVisible();
    });

    test('should default to Configuration tab', async ({ page }) => {
      await expect(page.getByRole('tab', { name: 'Configuration', exact: true })).toHaveAttribute('data-state', 'active');
      await expect(page.getByRole('tab', { name: 'Dashboard', exact: true })).toHaveAttribute('data-state', 'inactive');
      await expect(page.getByTestId('dashboard-summary-cards')).toHaveCount(0);
    });

    test('should switch to the Dashboard tab and back', async ({ page }) => {
      await test.step('Open the Dashboard tab', async () => {
        await page.getByRole('tab', { name: 'Dashboard', exact: true }).click();
        await expect(page.getByRole('tab', { name: 'Dashboard', exact: true })).toHaveAttribute('data-state', 'active');
        await expect(page.getByTestId('dashboard-summary-cards')).toBeVisible();
      });

      await test.step('Return to the Configuration tab', async () => {
        await page.getByRole('tab', { name: 'Configuration', exact: true }).click();
        await expect(page.getByRole('tab', { name: 'Configuration', exact: true })).toHaveAttribute('data-state', 'active');
        await expect(page.getByTestId('dashboard-summary-cards')).toHaveCount(0);
        await expect(page.getByRole('heading', { name: 'Presets', level: 3 })).toBeVisible();
      });
    });
  });

  test.describe('Summary Cards', () => {
    test('should show the summary figures', async ({ page }) => {
      await openDashboard(page);

      const cards = page.getByTestId('dashboard-summary-cards');
      await expect(cards.getByText('Total Decisions', { exact: true })).toBeVisible();
      await expect(cards.getByText('1,234', { exact: true })).toBeVisible();
      await expect(cards.getByText('+12.5%', { exact: true })).toBeVisible();
      await expect(cards.getByText('Active Decisions', { exact: true })).toBeVisible();
      await expect(cards.getByText('56', { exact: true })).toBeVisible();
      await expect(cards.getByText('Currently enforced', { exact: true })).toBeVisible();
      await expect(cards.getByText('Unique IPs', { exact: true })).toBeVisible();
      await expect(cards.getByText('78', { exact: true })).toBeVisible();
      await expect(cards.getByText('Top Scenario', { exact: true })).toBeVisible();
      await expect(cards.getByText('http-probing', { exact: true })).toBeVisible();
      await expect(cards.getByText('crowdsecurity/http-probing', { exact: true })).toBeVisible();
    });

    const TRENDS = [
      { name: 'a falling', trend: -3, label: '-3.0%' },
      { name: 'a flat', trend: 0, label: '0%' },
    ];
    for (const { name, trend, label } of TRENDS) {
      test(`should show ${name} decision trend`, async ({ page }) => {
        await openDashboard(
          page,
          dashboardOptions({ summary: crowdsecFixtures.dashboardSummary({ decisions_trend: trend }) }),
        );

        await expect(page.getByTestId('dashboard-summary-cards').getByText(label, { exact: true })).toBeVisible();
      });
    }

    test('should show N/A when the local API is unavailable', async ({ page }) => {
      await openDashboard(
        page,
        dashboardOptions({ summary: crowdsecFixtures.dashboardSummary({ active_decisions: -1 }) }),
      );

      const cards = page.getByTestId('dashboard-summary-cards');
      await expect(cards.getByText('N/A', { exact: true })).toBeVisible();
      await expect(cards.getByText('LAPI unavailable', { exact: true })).toBeVisible();
    });

    test('should show a placeholder when there is no top scenario', async ({ page }) => {
      await openDashboard(
        page,
        dashboardOptions({ summary: crowdsecFixtures.dashboardSummary({ top_scenario: '' }) }),
      );

      const cards = page.getByTestId('dashboard-summary-cards');
      await expect(cards.getByText('—', { exact: true })).toBeVisible();
      await expect(cards.getByText('No data', { exact: true })).toBeVisible();
    });

    test('should show an error when the summary cannot be loaded', async ({ page }) => {
      await openDashboard(page, dashboardOptions({ summary: SERVER_ERROR }));

      await expect(page.getByTestId('dashboard-summary-cards')).toHaveText('Failed to load summary data.', {
        timeout: ERROR_TIMEOUT,
      });
    });
  });

  test.describe('Charts', () => {
    test('should render the timeline, top IPs and scenario charts', async ({ page }) => {
      await openDashboard(page);

      await expect(page.getByRole('heading', { name: 'Decision Timeline' })).toBeVisible();
      await expect(page.getByRole('img', { name: 'Area chart showing bans and captchas over time' })).toBeVisible();
      await expect(page.getByRole('heading', { name: 'Top Attacking IPs' })).toBeVisible();
      await expect(
        page.getByRole('img', { name: 'Horizontal bar chart showing top attacking IPs by decision count' }),
      ).toBeVisible();
      await expect(page.getByRole('heading', { name: 'Scenario Breakdown' })).toBeVisible();
      await expect(
        page.getByRole('img', { name: 'Donut chart showing distribution of scenarios by decision count' }),
      ).toBeVisible();
    });

    test('should list each scenario with its count and share', async ({ page }) => {
      await openDashboard(page);

      const legend = page.getByRole('list', { name: 'Scenario legend' });
      await expect(legend.getByRole('listitem')).toHaveCount(2);
      await expect(legend.getByRole('listitem').nth(0)).toHaveText('http-probing6060.0%');
      await expect(legend.getByRole('listitem').nth(1)).toHaveText('ssh-bf4040.0%');
    });

    test('should show empty states when there is no data', async ({ page }) => {
      await openDashboard(
        page,
        dashboardOptions({
          timeline: { buckets: [], range: '24h', interval: '1h', cached: false },
          topIps: { ips: [], range: '24h', cached: false },
          scenarios: { scenarios: [], total: 0, range: '24h', cached: false },
        }),
      );

      await expect(page.getByText('No decision data for the selected period.', { exact: true })).toBeVisible();
      await expect(page.getByText('No attacking IPs in the selected period.', { exact: true })).toBeVisible();
      await expect(page.getByText('No scenario data for the selected period.', { exact: true })).toBeVisible();
      await expect(page.getByRole('img', { name: /chart showing/ })).toHaveCount(0);
    });

    test('should show an error in each chart panel when its data cannot be loaded', async ({ page }) => {
      await openDashboard(
        page,
        dashboardOptions({ timeline: SERVER_ERROR, topIps: SERVER_ERROR, scenarios: SERVER_ERROR }),
      );

      await expect(page.getByText('Failed to load timeline data.', { exact: true })).toBeVisible({ timeout: ERROR_TIMEOUT });
      await expect(page.getByText('Failed to load top IPs data.', { exact: true })).toBeVisible({ timeout: ERROR_TIMEOUT });
      await expect(page.getByText('Failed to load scenario data.', { exact: true })).toBeVisible({ timeout: ERROR_TIMEOUT });
    });
  });

  test.describe('Active Decisions Table', () => {
    test('should list the decisions sorted by alert count, highest first', async ({ page }) => {
      await openDashboard(page);

      const table = activeDecisionsTable(page);
      await expect(page.getByRole('heading', { name: 'Active Decisions' })).toBeVisible();
      await expect(table.getByRole('columnheader')).toHaveText(['IP', 'Alerts', 'Last Seen', 'Country']);
      await expect.poll(() => firstColumn(table)).toEqual(['203.0.113.2', '203.0.113.1', '203.0.113.3']);
      await expect(table.getByRole('columnheader', { name: 'Sort by Alerts' })).toHaveAttribute('aria-sort', 'descending');
    });

    test('should show the country of each decision, or a dash when unknown', async ({ page }) => {
      await openDashboard(page);

      const rows = activeDecisionsTable(page).getByRole('row');
      await expect(rows.filter({ hasText: '203.0.113.1' }).getByRole('cell').nth(3)).toHaveText('DE');
      await expect(rows.filter({ hasText: '203.0.113.2' }).getByRole('cell').nth(3)).toHaveText('US');
      await expect(rows.filter({ hasText: '203.0.113.3' }).getByRole('cell').nth(3)).toHaveText('—');
    });

    test('should expose the exact last seen time on each row', async ({ page }) => {
      await openDashboard(page);

      const row = activeDecisionsTable(page).getByRole('row').filter({ hasText: '203.0.113.1' });
      await expect(row.locator('time')).toHaveAttribute('datetime', '2020-01-01T00:00:00Z');
    });

    test('should re-sort when a column header is activated', async ({ page }) => {
      await openDashboard(page);
      const table = activeDecisionsTable(page);

      await test.step('Sort by IP, highest first', async () => {
        await table.getByRole('button', { name: 'Sort by IP' }).click();
        await expect(table.getByRole('columnheader', { name: 'Sort by IP' })).toHaveAttribute('aria-sort', 'descending');
        await expect(table.getByRole('columnheader', { name: 'Sort by Alerts' })).toHaveAttribute('aria-sort', 'none');
        await expect.poll(() => firstColumn(table)).toEqual(['203.0.113.3', '203.0.113.2', '203.0.113.1']);
      });

      await test.step('Activate the same header again to reverse the order', async () => {
        await table.getByRole('button', { name: 'Sort by IP' }).click();
        await expect(table.getByRole('columnheader', { name: 'Sort by IP' })).toHaveAttribute('aria-sort', 'ascending');
        await expect.poll(() => firstColumn(table)).toEqual(['203.0.113.1', '203.0.113.2', '203.0.113.3']);
      });
    });

    test('should show an empty state when there are no decisions', async ({ page }) => {
      await openDashboard(page, dashboardOptions({ topIps: { ips: [], range: '24h', cached: false } }));

      await expect(page.getByText('No active decisions.', { exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Sort by IP' })).toHaveCount(0);
    });

    test('should show an error when the decisions cannot be loaded', async ({ page }) => {
      await openDashboard(page, dashboardOptions({ topIps: SERVER_ERROR }));

      await expect(page.getByText('Failed to load decisions.', { exact: true })).toBeVisible({ timeout: ERROR_TIMEOUT });
    });
  });

  test.describe('Alerts List', () => {
    test('should list the first page of alerts', async ({ page }) => {
      const recorder = await openDashboard(page);

      const list = page.getByTestId('alerts-list');
      await expect(list.getByRole('heading', { name: 'Recent Alerts' })).toBeVisible();
      await expect(list.getByText('25 total', { exact: true })).toBeVisible();

      const table = alertsTable(page);
      await expect(table.getByRole('columnheader')).toHaveText(['IP', 'Scenario', 'Time', 'Events']);
      await expect(table.getByRole('row')).toHaveCount(ALERT_PAGE_SIZE + 1);
      const first = table.getByRole('row').nth(1);
      await expect(first.getByRole('cell').nth(0)).toHaveText('198.51.100.1');
      await expect(first.getByRole('cell').nth(1)).toHaveText('scenario-1');
      await expect(first.getByRole('cell').nth(3)).toHaveText('2');
      await expect(first.locator('time')).toHaveAttribute('datetime', '2026-01-01T00:05:00Z');
      expect(recorder.alertQueries[0]).toEqual({ range: '24h', limit: '10', offset: '0' });
    });

    test('should page through the alerts', async ({ page }) => {
      const recorder = await openDashboard(page);
      const pagination = page.getByRole('navigation', { name: 'Alerts pagination' });
      const previous = pagination.getByRole('button', { name: 'Previous page' });
      const next = pagination.getByRole('button', { name: 'Next page' });
      const table = alertsTable(page);

      await test.step('First page', async () => {
        await expect(pagination.getByText('Page 1 of 3', { exact: true })).toBeVisible();
        await expect(previous).toBeDisabled();
        await expect(next).toBeEnabled();
      });

      await test.step('Second page', async () => {
        await next.click();
        await expect(pagination.getByText('Page 2 of 3', { exact: true })).toBeVisible();
        await expect(table.getByRole('row').nth(1).getByRole('cell').nth(0)).toHaveText('198.51.100.11');
        await expect(previous).toBeEnabled();
        expect(recorder.alertQueries.at(-1)).toEqual({ range: '24h', limit: '10', offset: '10' });
      });

      await test.step('Last page holds the remaining alerts', async () => {
        await next.click();
        await expect(pagination.getByText('Page 3 of 3', { exact: true })).toBeVisible();
        await expect(table.getByRole('row')).toHaveCount(5 + 1);
        await expect(next).toBeDisabled();
        expect(recorder.alertQueries.at(-1)).toEqual({ range: '24h', limit: '10', offset: '20' });
      });

      await test.step('Back to the previous page', async () => {
        await previous.click();
        await expect(pagination.getByText('Page 2 of 3', { exact: true })).toBeVisible();
        await expect(table.getByRole('row').nth(1).getByRole('cell').nth(0)).toHaveText('198.51.100.11');
      });
    });

    test('should not show pagination when everything fits on one page', async ({ page }) => {
      await openDashboard(
        page,
        dashboardOptions({
          alerts: () => ({ alerts: [crowdsecFixtures.alert()], total: 1, source: 'lapi', cached: false }),
        }),
      );

      await expect(page.getByTestId('alerts-list').getByText('1 total', { exact: true })).toBeVisible();
      await expect(alertsTable(page).getByRole('row')).toHaveCount(2);
      await expect(page.getByRole('navigation', { name: 'Alerts pagination' })).toHaveCount(0);
    });

    test('should show an empty state when there are no alerts', async ({ page }) => {
      await openDashboard(
        page,
        dashboardOptions({ alerts: () => ({ alerts: [], total: 0, source: 'lapi', cached: false }) }),
      );

      await expect(page.getByTestId('alerts-list')).toHaveText('No alerts for the selected period.');
    });

    test('should show an error when the alerts cannot be loaded', async ({ page }) => {
      await openDashboard(page, dashboardOptions({ alerts: SERVER_ERROR }));

      await expect(page.getByTestId('alerts-list')).toHaveText('Failed to load alerts.', { timeout: ERROR_TIMEOUT });
    });
  });

  test.describe('Time Range Interaction', () => {
    test('should expose the time range as a radio group', async ({ page }) => {
      await openDashboard(page);

      await expect(timeRangeGroup(page)).toMatchAriaSnapshot(`
        - radiogroup:
          - radio "1H"
          - radio "6H"
          - radio "24H" [checked]
          - radio "7D"
          - radio "30D"
      `);
    });

    test('should name the time range group in readable text', async ({ page }) => {
      test.fixme(true, '#1530 F3: security.crowdsec.dashboard.timeRange is an object in the translations, so the radiogroup name is an i18next error string instead of "Time range"');
      await openDashboard(page);

      await expect(page.getByRole('radiogroup', { name: 'Time range', exact: true })).toBeVisible();
    });

    test('should request data for the selected time range', async ({ page }) => {
      const recorder = await openDashboard(page);
      const group = timeRangeGroup(page);

      await test.step('Defaults to 24 hours', async () => {
        await expect(group.getByRole('radio', { name: '24H' })).toBeChecked();
        expect(recorder.dashboardQueries.summary[0]).toEqual({ range: '24h' });
        expect(recorder.dashboardQueries['top-ips'][0]).toEqual({ range: '24h', limit: '10' });
      });

      await test.step('Select 7 days', async () => {
        await group.getByRole('radio', { name: '7D' }).click();
        await expect(group.getByRole('radio', { name: '7D' })).toBeChecked();
        await expect(group.getByRole('radio', { name: '24H' })).not.toBeChecked();
        await expect.poll(() => recorder.dashboardQueries.summary.at(-1)).toEqual({ range: '7d' });
        await expect.poll(() => recorder.dashboardQueries.timeline.at(-1)).toEqual({ range: '7d' });
        await expect.poll(() => recorder.dashboardQueries.scenarios.at(-1)).toEqual({ range: '7d' });
        await expect.poll(() => recorder.dashboardQueries['top-ips'].at(-1)).toEqual({ range: '7d', limit: '10' });
        await expect.poll(() => recorder.alertQueries.at(-1)).toEqual({ range: '7d', limit: '10', offset: '0' });
      });
    });

    test('should move between time ranges with the arrow keys', async ({ page }) => {
      await openDashboard(page);
      const group = timeRangeGroup(page);

      await group.getByRole('radio', { name: '24H' }).focus();
      await page.keyboard.press('ArrowRight');
      await expect(group.getByRole('radio', { name: '7D' })).toBeChecked();
      await expect(group.getByRole('radio', { name: '7D' })).toBeFocused();

      await page.keyboard.press('End');
      await expect(group.getByRole('radio', { name: '30D' })).toBeChecked();

      await page.keyboard.press('Home');
      await expect(group.getByRole('radio', { name: '1H' })).toBeChecked();

      await page.keyboard.press('ArrowLeft');
      await expect(group.getByRole('radio', { name: '30D' })).toBeChecked();
    });

    test('should return to the first alerts page when the time range changes', async ({ page }) => {
      const recorder = await openDashboard(page);
      const pagination = page.getByRole('navigation', { name: 'Alerts pagination' });

      await pagination.getByRole('button', { name: 'Next page' }).click();
      await expect(pagination.getByText('Page 2 of 3', { exact: true })).toBeVisible();

      await timeRangeGroup(page).getByRole('radio', { name: '30D' }).click();
      await expect(pagination.getByText('Page 1 of 3', { exact: true })).toBeVisible();
      expect(recorder.alertQueries.at(-1)).toEqual({ range: '30d', limit: '10', offset: '0' });
    });
  });

  test.describe('Refresh Functionality', () => {
    test('should reload every panel when clicking refresh', async ({ page }) => {
      const recorder = await openDashboard(page);
      await expect.poll(() => recorder.alertQueries.length).toBe(1);

      await page.getByRole('button', { name: 'Refresh', exact: true }).click();

      await expect.poll(() => recorder.dashboardQueries.summary.length).toBe(2);
      await expect.poll(() => recorder.dashboardQueries.timeline.length).toBe(2);
      await expect.poll(() => recorder.dashboardQueries['top-ips'].length).toBe(2);
      await expect.poll(() => recorder.dashboardQueries.scenarios.length).toBe(2);
      await expect.poll(() => recorder.alertQueries.length).toBe(2);
      await expect(page.getByRole('button', { name: 'Refresh', exact: true })).toBeEnabled();
    });
  });

  test.describe('Export', () => {
    const today = () => new Date().toISOString().slice(0, 10);

    test('should offer CSV and JSON export formats', async ({ page }) => {
      await openDashboard(page);
      const exportButton = page.getByRole('button', { name: 'Export decisions' });

      await expect(exportButton).toHaveAttribute('aria-expanded', 'false');
      await exportButton.click();
      await expect(exportButton).toHaveAttribute('aria-expanded', 'true');
      await expect(page.getByRole('menu', { name: 'Export format' })).toMatchAriaSnapshot(`
        - menu "Export format":
          - menuitem "Export as CSV"
          - menuitem "Export as JSON"
      `);
    });

    const FORMATS = [
      { label: 'Export as CSV', format: 'csv', contentType: 'text/csv', body: 'ip,reason\n203.0.113.2,probing\n' },
      { label: 'Export as JSON', format: 'json', contentType: 'application/json', body: '[{"ip":"203.0.113.2"}]' },
    ];
    for (const { label, format, contentType, body } of FORMATS) {
      test(`should download the decisions as ${format.toUpperCase()}`, async ({ page }) => {
        const recorder = await openDashboard(page, {
          ...dashboardOptions(),
          decisionsExport: { body, contentType },
        });

        await page.getByRole('button', { name: 'Export decisions' }).click();
        const downloadPromise = page.waitForEvent('download');
        await page.getByRole('menuitem', { name: label }).click();
        const download = await downloadPromise;

        expect(download.suggestedFilename()).toBe(`crowdsec-decisions-${today()}.${format}`);
        expect(recorder.decisionsExportQueries).toEqual([{ format, range: '24h', source: 'all' }]);
        await expect(page.getByRole('menu')).toHaveCount(0);
        await expect(page.getByRole('alert')).toHaveCount(0);
      });
    }

    test('should export the selected time range', async ({ page }) => {
      const recorder = await openDashboard(page, {
        ...dashboardOptions(),
        decisionsExport: { body: 'ip\n', contentType: 'text/csv' },
      });

      await timeRangeGroup(page).getByRole('radio', { name: '7D' }).click();
      await page.getByRole('button', { name: 'Export decisions' }).click();
      const downloadPromise = page.waitForEvent('download');
      await page.getByRole('menuitem', { name: 'Export as CSV' }).click();
      await downloadPromise;

      expect(recorder.decisionsExportQueries).toEqual([{ format: 'csv', range: '7d', source: 'all' }]);
    });

    test('should close the menu with Escape', async ({ page }) => {
      await openDashboard(page);
      const exportButton = page.getByRole('button', { name: 'Export decisions' });

      await exportButton.click();
      await expect(page.getByRole('menu')).toBeVisible();
      await page.keyboard.press('Escape');
      await expect(page.getByRole('menu')).toHaveCount(0);
      await expect(exportButton).toBeFocused();
    });

    test('should show an error when the export fails', async ({ page }) => {
      await openDashboard(page, { ...dashboardOptions(), decisionsExport: SERVER_ERROR });

      await page.getByRole('button', { name: 'Export decisions' }).click();
      await page.getByRole('menuitem', { name: 'Export as CSV' }).click();

      await expect(page.getByRole('alert')).toHaveText('Export failed. Please try again.');
    });

    test('should show an error when the export is empty', async ({ page }) => {
      await openDashboard(page, {
        ...dashboardOptions(),
        decisionsExport: { body: '', contentType: 'text/csv' },
      });

      await page.getByRole('button', { name: 'Export decisions' }).click();
      await page.getByRole('menuitem', { name: 'Export as CSV' }).click();

      await expect(page.getByRole('alert')).toHaveText('Export failed. Please try again.');
    });
  });
});
