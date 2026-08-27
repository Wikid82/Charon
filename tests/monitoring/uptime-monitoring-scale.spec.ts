/**
 * Uptime Monitoring at Scale — E2E specs (Commit 1)
 *
 * Failing-by-design coverage of the headline behaviors from the
 * "Uptime Monitoring at Scale" feature (docs/plans/current_spec.md §4 Phase 1,
 * §3.5 / §3.6, §7 Acceptance Criteria).
 *
 * NOTHING in this feature is implemented yet, so every test here is
 * `test.fixme(...)` — they collect and SKIP cleanly, they do not fail. The
 * hardening commit (C9) flips them to `test(...)` and reconciles any selector
 * drift against the real DOM.
 *
 * Style: `page.route` interception with mock JSON only (mirrors
 * `tests/monitoring/uptime-monitoring.spec.ts` and
 * `tests/a11y/uptime.a11y.spec.ts`). No live backend involvement.
 */

import type { Page, Route } from '@playwright/test';

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import {
  makeSummaryFixture,
  makeHealthFixture,
  type MonitorSummary,
} from '../fixtures/uptime';

// --- Route globs -----------------------------------------------------------
const SUMMARY_ROUTE = '**/api/v1/uptime/monitors/summary*';
const HISTORY_ROUTE = '**/api/v1/uptime/monitors/*/history*';
const MONITORS_ROUTE = '**/api/v1/uptime/monitors';
const HEALTH_ROUTE = '**/api/v1/uptime/health';
const SETTINGS_ROUTE = '**/api/v1/settings';
const FEATURE_FLAGS_ROUTE = '**/api/v1/feature-flags';

// --- Selectors -----------------------------------------------------------
// TODO(C8): confirm these once the summary-driven Uptime page and the admin
// "Uptime Monitoring" settings card exist. The values below are written
// against today's DOM (`tests/monitoring/uptime-monitoring.spec.ts`) plus the
// spec'd additions in §3.5.7 / §3.6.4.
const SELECTORS = {
  addMonitorButton: '[data-testid="add-monitor-button"]',
  monitorCard: '[data-testid="monitor-card"]',
  statusBadge: '[data-testid="status-badge"]',
  downBadge: '[data-testid="status-badge"][data-status="down"]',
  // The per-card heartbeat bar doubles as the sparkline in §3.5.7.
  sparkline: '[data-testid="heartbeat-bar"]',
  nameInput: 'input#create-monitor-name',
  urlInput: 'input#create-monitor-url',
  intervalInput: 'input#create-monitor-interval',
  // TODO(C8): confirm the helper-text element id / copy ("Minimum 30 seconds").
  intervalHelper: '#create-monitor-interval-helper',
  submitButton: 'button[type="submit"]',
};

// --- Helpers -----------------------------------------------------------

/** Tallies requests whose URL contains `needle`, for "exactly one" / "zero" assertions. */
function countRequests(page: Page, needle: string): { count: number; urls: string[] } {
  const tally = { count: 0, urls: [] as string[] };
  page.on('request', (req) => {
    if (req.url().includes(needle)) {
      tally.count += 1;
      tally.urls.push(req.url());
    }
  });
  return tally;
}

async function fulfillJSON(route: Route, body: unknown, status = 200): Promise<void> {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });
}

/** Maps a `MonitorSummary` back onto the legacy `GET /uptime/monitors` list shape. */
function summaryToLegacyList(summary: MonitorSummary[]) {
  return summary.map((m) => ({
    id: m.id,
    name: m.name,
    type: m.type,
    url: m.url,
    interval: m.interval,
    enabled: m.enabled,
    status: m.status,
    latency: m.latency,
    last_check: m.last_check,
    max_retries: 3,
    proxy_host_id: m.proxy_host_id ?? undefined,
    remote_server_id: m.remote_server_id ?? undefined,
  }));
}

/**
 * Wires the batch summary endpoint (post-C8 read path) plus the legacy list
 * endpoint (still hit by `UptimeWidget` and older code paths) to the same
 * fixture, and hard-fails any per-card history request.
 */
async function setupSummary(page: Page, summary: MonitorSummary[]): Promise<void> {
  await page.route(SUMMARY_ROUTE, (route) => fulfillJSON(route, summary));
  await page.route(MONITORS_ROUTE, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await fulfillJSON(route, summaryToLegacyList(summary));
  });
  // The whole point of the summary endpoint is that the list view issues no
  // history calls — make an accidental one loud.
  await page.route(HISTORY_ROUTE, (route) => fulfillJSON(route, [], 500));
}

interface SettingsHarness {
  values: Record<string, string>;
  posts: number;
}

/**
 * Mock `GET/POST /api/v1/settings` with an in-memory store so a set → reload →
 * read round-trip works, and mock `GET /api/v1/feature-flags` so the
 * feature-gated Uptime settings card renders.
 */
async function setupSettings(
  page: Page,
  seed: Record<string, string> = {},
): Promise<SettingsHarness> {
  const harness: SettingsHarness = {
    values: {
      'uptime.default_interval_seconds': '60',
      'uptime.worker_pool_size': '30',
      'uptime.heartbeat_retention_days': '90',
      ...seed,
    },
    posts: 0,
  };

  await page.route(SETTINGS_ROUTE, async (route) => {
    const req = route.request();
    if (req.method() === 'POST') {
      harness.posts += 1;
      const body = (req.postDataJSON() ?? {}) as { key?: string; value?: string };
      if (body.key) harness.values[body.key] = String(body.value ?? '');
      await fulfillJSON(route, { success: true });
      return;
    }
    await fulfillJSON(route, harness.values);
  });

  await page.route(FEATURE_FLAGS_ROUTE, (route) =>
    fulfillJSON(route, { 'feature.uptime.enabled': true }),
  );

  return harness;
}

// =========================================================================
// Scenario 1 — Per-monitor interval floor (§4 Phase 1 #1, §7 #1)
// =========================================================================
test.describe('Uptime at scale: per-monitor interval floor', () => {
  test.fixme(
    'create form enforces the 30s floor and forwards a valid interval unchanged',
    async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      let createPayload: Record<string, unknown> | null = null;
      await page.route(MONITORS_ROUTE, async (route) => {
        if (route.request().method() === 'POST') {
          createPayload = route.request().postDataJSON() as Record<string, unknown>;
          await fulfillJSON(
            route,
            { id: 'created-1', ...createPayload, status: 'pending', latency: 0, enabled: true },
            201,
          );
          return;
        }
        await fulfillJSON(route, []);
      });
      await page.route(SUMMARY_ROUTE, (route) => fulfillJSON(route, []));

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      await test.step('open the create-monitor form', async () => {
        await page.click(SELECTORS.addMonitorButton);
      });

      await test.step('interval input advertises the 30s minimum with helper text', async () => {
        const interval = page.locator(SELECTORS.intervalInput);
        await expect(interval).toHaveAttribute('min', '30');
        // TODO(C8): confirm helper-text selector + copy ("Minimum 30 seconds").
        await expect(page.locator(SELECTORS.intervalHelper)).toBeVisible();
      });

      await test.step('a below-floor value is rejected / clamped client-side', async () => {
        await page.fill(SELECTORS.nameInput, 'Floor Test');
        await page.fill(SELECTORS.urlInput, 'https://floor.example.test/health');
        const interval = page.locator(SELECTORS.intervalInput);
        await interval.fill('10');
        await interval.blur();
        // Either the field clamps to 30, or submit is blocked — never sends 10.
        await expect(interval).not.toHaveValue('10');
      });

      await test.step('a valid interval reaches the POST payload unchanged', async () => {
        const interval = page.locator(SELECTORS.intervalInput);
        await interval.fill('45');

        const posted = page.waitForResponse(
          (r) =>
            r.url().includes('/api/v1/uptime/monitors') &&
            r.request().method() === 'POST',
        );
        await page.click(SELECTORS.submitButton);
        await posted;

        expect(createPayload).not.toBeNull();
        expect(createPayload?.interval).toBe(45);
      });
    },
  );
});

// =========================================================================
// Scenario 2 — Single-request dashboard load at 100 monitors
// (§4 Phase 1 #2, §7 #4)
// =========================================================================
test.describe('Uptime at scale: dashboard load with many monitors', () => {
  test.fixme(
    'renders 100 monitor cards from one summary request and zero history requests',
    async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      const summary = makeSummaryFixture(100);
      const summaryReqs = countRequests(page, '/uptime/monitors/summary');
      const historyReqs = countRequests(page, '/history');

      await setupSummary(page, summary);

      const start = Date.now();
      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      await test.step('all 100 cards render with a status badge + sparkline', async () => {
        await expect(page.locator(SELECTORS.monitorCard)).toHaveCount(100);
        await expect(page.locator(SELECTORS.statusBadge)).toHaveCount(100);
        await expect(page.locator(SELECTORS.sparkline)).toHaveCount(100);
      });

      await test.step('the page is interactive within a generous budget', async () => {
        const elapsedMs = Date.now() - start;
        expect(elapsedMs).toBeLessThan(15_000);
      });

      await test.step('exactly one summary request, zero per-card history requests', async () => {
        expect(summaryReqs.count, summaryReqs.urls.join('\n')).toBe(1);
        expect(historyReqs.count, historyReqs.urls.join('\n')).toBe(0);
      });
    },
  );
});

// =========================================================================
// Scenario 3 — Retention setting round-trip + health surface
// (§4 Phase 1 #3, §7 #3a / #6)
// =========================================================================
test.describe('Uptime at scale: admin Uptime settings card', () => {
  test.fixme(
    'heartbeat_retention_days round-trips and the ingester health signal is shown',
    async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      const settings = await setupSettings(page, { 'uptime.heartbeat_retention_days': '90' });
      await page.route(HEALTH_ROUTE, (route) =>
        fulfillJSON(route, makeHealthFixture({ heartbeats_dropped: 0 })),
      );

      await page.goto('/settings/system');
      await waitForLoadingComplete(page);

      // TODO(C8): confirm the field label / test id for the retention input.
      const retention = page.getByLabel(/heartbeat retention/i);

      await test.step('set retention to 30 and save', async () => {
        await retention.fill('30');
        const saved = page.waitForResponse(
          (r) =>
            r.url().includes('/api/v1/settings') && r.request().method() === 'POST',
        );
        // TODO(C8): confirm the save affordance (per-field mutation on blur vs a Save button).
        await page.getByRole('button', { name: /save/i }).click();
        await saved;
      });

      await test.step('value persists across a reload', async () => {
        await page.reload();
        await waitForLoadingComplete(page);
        await expect(page.getByLabel(/heartbeat retention/i)).toHaveValue('30');
        expect(settings.values['uptime.heartbeat_retention_days']).toBe('30');
      });

      await test.step('the card surfaces the dropped-heartbeats health signal', async () => {
        // TODO(C8): confirm where heartbeats_dropped is rendered (settings card
        // vs the Uptime page health indicator) and its exact copy.
        await expect(page.getByText(/heartbeats?\s+dropped/i)).toBeVisible();
      });
    },
  );

  test.fixme(
    'rejects out-of-bounds worker_pool_size / default_interval_seconds client-side',
    async ({ page, adminUser }) => {
      await loginUser(page, adminUser);

      const settings = await setupSettings(page);

      await page.goto('/settings/system');
      await waitForLoadingComplete(page);

      // TODO(C8): confirm labels + inline-error copy for the two bounded fields.
      await test.step('worker pool size above 200 is rejected', async () => {
        const workerPool = page.getByLabel(/worker pool size/i);
        await workerPool.fill('999');
        await workerPool.blur();
        await expect(page.getByText(/between 1 and 200/i)).toBeVisible();
      });

      await test.step('default interval below 30 is rejected', async () => {
        const defaultInterval = page.getByLabel(/default check interval/i);
        await defaultInterval.fill('5');
        await defaultInterval.blur();
        await expect(page.getByText(/at least 30 seconds/i)).toBeVisible();
      });

      await test.step('save is disabled and nothing is written while invalid', async () => {
        await expect(page.getByRole('button', { name: /save/i })).toBeDisabled();
        expect(settings.posts).toBe(0);
      });
    },
  );
});

// =========================================================================
// Scenario 4 — Summary payload drives card state (§4 Phase 1 #4, §7 #4)
// =========================================================================
test.describe('Uptime at scale: summary drives card state', () => {
  test.fixme(
    'a down monitor in the summary renders a DOWN card with no history request',
    async ({ page, authenticatedUser }) => {
      await loginUser(page, authenticatedUser);

      const summary = makeSummaryFixture(3, { downIndices: [1] });
      const downMonitor = summary[1];
      const lastBeat = downMonitor.recent_beats[downMonitor.recent_beats.length - 1];

      // Guard the fixture itself: down monitor, trailing down beat.
      expect(downMonitor.status).toBe('down');
      expect(lastBeat?.status).toBe('down');

      const historyReqs = countRequests(page, '/history');
      await setupSummary(page, summary);

      await page.goto('/uptime');
      await waitForLoadingComplete(page);

      const downCard = page
        .locator(SELECTORS.monitorCard)
        .filter({ hasText: downMonitor.name });

      await expect(downCard.locator(SELECTORS.downBadge)).toBeVisible();
      await expect(downCard.locator(SELECTORS.downBadge)).toContainText(/down/i);
      expect(historyReqs.count, historyReqs.urls.join('\n')).toBe(0);
    },
  );
});
