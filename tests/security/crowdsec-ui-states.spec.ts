/**
 * CrowdSec newer UI states E2E Tests
 *
 * The E2E container has no cscli, hub or LAPI, so every state here is driven by
 * route-mocked responses:
 * - Hub preset type picker after a 409 `preset_type_ambiguous` pull, and the retry with `type`
 * - `hub_unreachable` (503) on pull and apply: fixed message, no retry storm
 * - "Files omitted" note after a config export, and `export_config_unsafe` (422) on export
 * - Soft-error note on the banned IPs list (`error_code` from cscli)
 * - Active decisions table hiding the Country column when no row has a country
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { CROWDSEC_ROUTES, crowdsecFixtures, stubCrowdSecApi } from '../utils/crowdsec-stubs';

const PRESETS_ROUTE = '**/api/v1/admin/crowdsec/presets';
const PULL_ROUTE = '**/api/v1/admin/crowdsec/presets/pull';
const APPLY_ROUTE = '**/api/v1/admin/crowdsec/presets/apply';
const HUB_PRESET_SLUG = 'crowdsecurity/e2e-ambiguous';
const HUB_PRESET_TITLE = 'E2E Ambiguous Preset';
const CACHE_KEY = 'e2e-ui-states-cache-key';

async function loadPage(page: Page): Promise<void> {
  await page.goto('/security/crowdsec');
  await waitForLoadingComplete(page);
}

/** Stubs the preset list with one hub preset so the pull/apply endpoints can be driven. */
async function stubHubPresetList(page: Page): Promise<void> {
  await page.route(PRESETS_ROUTE, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      json: {
        presets: [
          {
            slug: HUB_PRESET_SLUG,
            title: HUB_PRESET_TITLE,
            summary: 'Deterministic hub preset for E2E',
            source: 'hub',
            requires_hub: true,
            available: true,
            cached: false,
          },
        ],
      },
    });
  });
}

test.describe('CrowdSec newer UI states @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.describe('Hub preset pull', () => {
    test.beforeEach(async ({ page }) => {
      await stubHubPresetList(page);
    });

    test('should offer a type picker on an ambiguous pull and retry with the chosen type', async ({ page }) => {
      const pullBodies: Array<Record<string, unknown>> = [];
      await page.route(PULL_ROUTE, async (route) => {
        const body = route.request().postDataJSON() as Record<string, unknown>;
        pullBodies.push(body);
        if (!body.type) {
          await route.fulfill({
            status: 409,
            contentType: 'application/json',
            json: {
              error: 'name exists in several sections',
              error_code: 'preset_type_ambiguous',
              types: ['collections', 'parsers'],
            },
          });
          return;
        }
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          json: {
            status: 'pulled',
            slug: HUB_PRESET_SLUG,
            type: body.type,
            preview: 'name: e2e-ambiguous\n',
            cache_key: CACHE_KEY,
            source: 'hub',
          },
        });
      });

      await loadPage(page);
      await page.getByRole('button', { name: new RegExp(HUB_PRESET_TITLE, 'i') }).click();

      await test.step('Verify the ambiguity message and the type picker are shown', async () => {
        await expect(page.getByText('This name exists in several hub sections. Choose which item to use.')).toBeVisible();
        const picker = page.getByRole('combobox', { name: 'Item type' });
        await expect(picker).toBeVisible();
        await expect(picker.getByRole('option')).toHaveText(['Choose a type', 'collections', 'parsers']);
      });

      await test.step('Pick a type and verify the pull is retried with it', async () => {
        await page.getByRole('combobox', { name: 'Item type' }).selectOption('parsers');
        await expect(page.getByText('Preview fetched from hub')).toBeVisible();
        expect(pullBodies).toEqual([
          { slug: HUB_PRESET_SLUG },
          { slug: HUB_PRESET_SLUG, type: 'parsers' },
        ]);
        await expect(page.getByRole('combobox', { name: 'Item type' })).toBeHidden();
      });
    });

    test('should show the hub unreachable message once when a pull returns hub_unreachable', async ({ page }) => {
      let pullRequests = 0;
      await page.route(PULL_ROUTE, async (route) => {
        pullRequests += 1;
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          json: { error: 'hub unreachable', error_code: 'hub_unreachable' },
        });
      });

      await loadPage(page);
      await page.getByRole('button', { name: new RegExp(HUB_PRESET_TITLE, 'i') }).click();

      await test.step('Verify the fixed message and the manual retry button', async () => {
        await expect(page.getByText('CrowdSec hub unavailable. Retry or load cached copy.')).toBeVisible();
        await expect(page.getByText('CrowdSec Hub unreachable. Hub presets may not work.')).toBeVisible();
        await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible();
      });

      await test.step('Verify the UI did not retry on its own', async () => {
        await expect.poll(() => pullRequests, { timeout: 3000 }).toBe(1);
        // Let any would-be automatic retry fire before asserting the count stayed at one.
        await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeEnabled();
        expect(pullRequests).toBe(1);
      });
    });

    test('should show the fixed message once when applying returns hub_unreachable', async ({ page }) => {
      let applyRequests = 0;
      await page.route(PULL_ROUTE, async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          json: {
            status: 'pulled',
            slug: HUB_PRESET_SLUG,
            preview: 'name: e2e-ambiguous\n',
            cache_key: CACHE_KEY,
            source: 'hub',
          },
        });
      });
      await page.route(APPLY_ROUTE, async (route) => {
        applyRequests += 1;
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          json: { error: 'hub unreachable', error_code: 'hub_unreachable' },
        });
      });

      await loadPage(page);
      await page.getByRole('button', { name: new RegExp(HUB_PRESET_TITLE, 'i') }).click();
      await expect(page.getByText('Preview fetched from hub')).toBeVisible();
      await page.getByRole('button', { name: /apply preset/i }).click();

      await expect(
        page.getByText('CrowdSec could not reach the hub. Check the network connection and try again.'),
      ).toBeVisible();
      await expect(page.getByText(/preset applied/i)).toHaveCount(0);
      await expect(page.getByRole('button', { name: /apply preset/i })).toBeEnabled();
      expect(applyRequests).toBe(1);
    });
  });

  test.describe('Configuration export', () => {
    test('should list the files left out of an export', async ({ page }) => {
      await stubCrowdSecApi(page, {});
      await page.route(CROWDSEC_ROUTES.export, async (route) => {
        if (route.request().method() !== 'GET') return route.fallback();
        await route.fulfill({
          status: 200,
          contentType: 'application/gzip',
          headers: { 'X-Charon-Omitted-Files': JSON.stringify(['notifications/http_default.yaml', 'local_api_credentials.yaml']) },
          body: Buffer.from('stub-archive-bytes'),
        });
      });
      await loadPage(page);

      page.once('dialog', (dialog) => void dialog.accept('omitted-backup'));
      const downloadPromise = page.waitForEvent('download');
      await page.getByRole('button', { name: 'Export', exact: true }).click();
      await downloadPromise;

      await expect(
        page.getByText(
          'Left out of this export (2): notifications/http_default.yaml, local_api_credentials.yaml. Supply these files again after restoring on another host.',
        ),
      ).toBeVisible();
    });

    test('should summarise the files beyond the first five', async ({ page }) => {
      await stubCrowdSecApi(page, {});
      const names = ['a.yaml', 'b.yaml', 'c.yaml', 'd.yaml', 'e.yaml'];
      await page.route(CROWDSEC_ROUTES.export, async (route) => {
        if (route.request().method() !== 'GET') return route.fallback();
        await route.fulfill({
          status: 200,
          contentType: 'application/gzip',
          headers: { 'X-Charon-Omitted-Files': JSON.stringify([...names, '+3 more']) },
          body: Buffer.from('stub-archive-bytes'),
        });
      });
      await loadPage(page);

      page.once('dialog', (dialog) => void dialog.accept('omitted-backup'));
      const downloadPromise = page.waitForEvent('download');
      await page.getByRole('button', { name: 'Export', exact: true }).click();
      await downloadPromise;

      await expect(page.getByText(/Left out of this export \(8\): a\.yaml, b\.yaml, c\.yaml, d\.yaml, e\.yaml\./)).toBeVisible();
      await expect(page.getByText(/\(and 3 more\)/)).toBeVisible();
    });

    test('should show the unsafe configuration error when export returns export_config_unsafe', async ({ page }) => {
      await stubCrowdSecApi(page, {});
      await page.route(CROWDSEC_ROUTES.export, (route) => {
        if (route.request().method() !== 'GET') return route.fallback();
        return route.fulfill({
          status: 422,
          contentType: 'application/json',
          json: { error: 'config unsafe', error_code: 'export_config_unsafe' },
        });
      });
      await loadPage(page);

      page.once('dialog', (dialog) => void dialog.accept('unsafe-backup'));
      await page.getByRole('button', { name: 'Export', exact: true }).click();

      await expect(page.getByText(/cannot be hidden reliably.*nothing was exported/)).toBeVisible();
      await expect(page.getByText('CrowdSec configuration exported')).toHaveCount(0);
      await expect(page.getByText(/Left out of this export/)).toHaveCount(0);
    });
  });

  test.describe('Banned IPs list soft error', () => {
    const SOFT_ERRORS = [
      ['cscli_timeout', 'The decision service took too long to answer, so this list may be incomplete. It will refresh on its own.'],
      ['cscli_output_too_large', 'The decision service returned more data than can be shown, so this list may be incomplete.'],
      ['cscli_unavailable', 'The decision service is not available right now, so this list may be incomplete.'],
    ] as const;

    for (const [code, message] of SOFT_ERRORS) {
      test(`should show a passive note for ${code} alongside the partial list`, async ({ page }) => {
        await stubCrowdSecApi(page, { crowdsecMode: 'local' });
        await page.route(CROWDSEC_ROUTES.decisions, (route) => {
          if (route.request().method() !== 'GET') return route.fallback();
          return route.fulfill({
            status: 200,
            contentType: 'application/json',
            json: { decisions: [crowdsecFixtures.decision()], error: `English fallback for ${code}`, error_code: code },
          });
        });
        await loadPage(page);

        const note = page.getByRole('note').filter({ hasText: message });
        await expect(note).toBeVisible();
        await expect(note).toHaveAttribute('data-testid', 'crowdsec-list-error-note');
        await expect(page.getByText('203.0.113.7')).toBeVisible();
      });
    }

    test('should show no note when the list carries no error', async ({ page }) => {
      await stubCrowdSecApi(page, { crowdsecMode: 'local', decisions: [crowdsecFixtures.decision()] });
      await loadPage(page);

      await expect(page.getByText('203.0.113.7')).toBeVisible();
      await expect(page.getByRole('note')).toHaveCount(0);
    });
  });

  test.describe('Active decisions table', () => {
    test('should hide the empty Country column when no row has a country', async ({ page }) => {
      await stubCrowdSecApi(page, {
        dashboard: {
          summary: crowdsecFixtures.dashboardSummary(),
          timeline: { buckets: [], range: '24h', interval: '1h', cached: false },
          topIps: {
            ips: [
              { ip: '203.0.113.1', count: 5, last_seen: '2020-01-01T00:00:00Z', country: '' },
              { ip: '203.0.113.2', count: 9, last_seen: '2020-01-02T00:00:00Z' },
            ],
            range: '24h',
            cached: false,
          },
          scenarios: { scenarios: [], total: 0, range: '24h', cached: false },
          alerts: () => ({ alerts: [], total: 0, source: 'lapi', cached: false }),
        },
      });
      await loadPage(page);
      await page.getByRole('tab', { name: 'Dashboard', exact: true }).click();

      const table = page
        .getByRole('table')
        .filter({ has: page.getByRole('button', { name: 'Sort by Alerts' }) });
      await expect(table.getByRole('columnheader')).toHaveText(['IP', 'Alerts', 'Last Seen']);
      await expect(table.getByRole('columnheader', { name: 'Country' })).toHaveCount(0);
      await expect(table.getByRole('cell', { name: '203.0.113.2' })).toBeVisible();
      await expect(table.getByRole('cell', { name: '—' })).toHaveCount(0);
    });
  });
});
