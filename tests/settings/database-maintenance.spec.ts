/**
 * Automatic database maintenance (GH #1422) - E2E specs
 *
 * Covers docs/plans/current_spec.md section 4 Phase 1, 3.6 and 3.7:
 *
 *   1. System Settings "Database" card: quiet when nothing needs doing.
 *   2. Notices by code and severity: `restart_to_optimize`, `database_busy`
 *      and `disabled_by_env` (info) render as a quiet line in the card;
 *      `insufficient_disk` and `too_many_failures` (warning) render as a
 *      page-top banner.
 *   3. "Reclaim space on next restart" button: shown, hidden or disabled with
 *      a reason per `can_request_optimize` / `auto_vacuum` / `env_mode`;
 *      POST then "Scheduled" then undo (DELETE); repeated POST is idempotent.
 *   4. Maintenance view: a 503 `{maintenance:true}` from any API call shows the
 *      "Optimizing the database" view WITHOUT logging the user out or
 *      retry-storming, and returns to the app once the status route reports
 *      `active:false`.
 *   5. `/api/v1/maintenance/status` is static JSON in every phase.
 *
 * All specs are `test.fixme` until commit 7 of the #1422 plan
 * ("feat: add database maintenance card and notice") ships the UI and the
 * backend endpoints; that commit removes the fixme markers. Nothing here
 * performs a real conversion: every endpoint is mocked with page.route, in the
 * style of tests/monitoring/uptime-monitoring-scale.spec.ts. The one
 * exception is the status-endpoint contract, which needs a live backend.
 *
 * Locator notes for commit 7: the card is located as a region/group named
 * "Database" and banners as role="alert"; adjust to the shipped markup if it
 * differs, keeping role-based locators.
 */

import { test, expect, type Page, type Route } from '@playwright/test';

// These specs stub their own session; the project-level storageState would
// leak a real token cookie into mocked requests.
test.use({ storageState: { cookies: [], origins: [] } });

// --- Routes ----------------------------------------------------------------
const DATABASE_ROUTE = '**/api/v1/system/database';
const OPTIMIZE_ROUTE = '**/api/v1/system/database/optimize-on-restart';
const STATUS_ROUTE = '**/api/v1/maintenance/status';
const AUTH_ME_ROUTE = '**/api/v1/auth/me';
const FEATURE_FLAGS_ROUTE = '**/api/v1/feature-flags';

const GB = 1_000_000_000;

// --- Types and fixtures ----------------------------------------------------
type NoticeCode =
  | 'restart_to_optimize'
  | 'insufficient_disk'
  | 'disabled_by_env'
  | 'too_many_failures'
  | 'database_busy';

interface DatabaseNotice {
  code: NoticeCode;
  severity: 'info' | 'warning';
  reclaimable_bytes: number;
  required_bytes?: number;
  available_bytes?: number;
}

interface DatabaseInfo {
  size_bytes: number;
  wal_bytes: number;
  reclaimable_bytes: number;
  auto_vacuum: 'none' | 'full' | 'incremental';
  env_mode: 'auto' | 'off';
  compact_requested: boolean;
  can_request_optimize: boolean;
  disk_free_bytes: number;
  last_result: {
    at: string;
    outcome: string;
    reason: string;
    bytes_before: number;
    bytes_after: number;
  } | null;
  notice: DatabaseNotice | null;
}

/** Healthy baseline: small, already optimized, nothing to say. */
function makeDatabaseInfo(overrides: Partial<DatabaseInfo> = {}): DatabaseInfo {
  return {
    size_bytes: 120_000_000,
    wal_bytes: 4_194_304,
    reclaimable_bytes: 0,
    auto_vacuum: 'incremental',
    env_mode: 'auto',
    compact_requested: false,
    can_request_optimize: false,
    disk_free_bytes: 52 * GB,
    last_result: null,
    notice: null,
    ...overrides,
  };
}

/** A legacy (auto_vacuum none) database with 1.7 GB reclaimable. */
function makeLegacyDatabaseInfo(overrides: Partial<DatabaseInfo> = {}): DatabaseInfo {
  return makeDatabaseInfo({
    size_bytes: 4.8 * GB,
    reclaimable_bytes: 1.7 * GB,
    auto_vacuum: 'none',
    can_request_optimize: true,
    ...overrides,
  });
}

const SESSION_USER = {
  user_id: 1,
  role: 'admin',
  name: 'E2E Maintenance Admin',
  email: 'e2e-maintenance-admin@test.local',
};

// --- Helpers ---------------------------------------------------------------

async function fulfillJSON(route: Route, body: unknown, status = 200): Promise<void> {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });
}

/**
 * Seeds the token AuthContext reads on boot, answers /auth/me with an admin
 * and installs a catch-all API fallback so an unmocked call can never 401 into
 * the axios session-expiry redirect. Call FIRST: Playwright matches the
 * last-registered route first, so scenario routes registered afterwards win.
 */
async function stubAuthenticatedSession(page: Page): Promise<void> {
  await page.addInitScript(() => {
    try {
      window.localStorage.setItem('charon_auth_token', 'e2e-mock-token');
    } catch {
      /* storage unavailable - nothing to seed */
    }
  });

  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url();
    if (url.includes('/auth/me')) {
      await fulfillJSON(route, SESSION_USER);
      return;
    }
    if (url.includes('/feature-flags')) {
      await fulfillJSON(route, {});
      return;
    }
    // `[]` is the safe universal default for collection endpoints.
    await fulfillJSON(route, []);
  });
}

interface DatabaseHarness {
  info: DatabaseInfo;
  posts: number;
  deletes: number;
}

/**
 * Mocks GET /system/database from a mutable store plus the POST/DELETE flag
 * endpoints. POST is idempotent (200 {requested:true} every time) and flips
 * `compact_requested`; DELETE clears it, mirroring plan section 3.7.
 */
async function setupDatabaseApi(page: Page, info: DatabaseInfo): Promise<DatabaseHarness> {
  const harness: DatabaseHarness = { info: { ...info }, posts: 0, deletes: 0 };

  await page.route(DATABASE_ROUTE, (route) => fulfillJSON(route, harness.info));
  await page.route(OPTIMIZE_ROUTE, async (route) => {
    const method = route.request().method();
    if (method === 'POST') {
      harness.posts += 1;
      harness.info = { ...harness.info, compact_requested: true };
      await fulfillJSON(route, { requested: true });
      return;
    }
    if (method === 'DELETE') {
      harness.deletes += 1;
      harness.info = { ...harness.info, compact_requested: false };
      await fulfillJSON(route, { requested: false });
      return;
    }
    await route.fallback();
  });
  await page.route(FEATURE_FLAGS_ROUTE, (route) => fulfillJSON(route, {}));

  return harness;
}

async function gotoSystemSettings(page: Page): Promise<void> {
  await page.goto('/settings/system');
  await expect(page.getByRole('heading', { name: /^database$/i })).toBeVisible();
}

function databaseCard(page: Page) {
  return page.getByRole('region', { name: /^database$/i });
}

const reclaimButton = (page: Page) =>
  databaseCard(page).getByRole('button', { name: /reclaim space on next restart/i });

// =========================================================================
// 1. Quiet when nothing to do
// =========================================================================
test.describe('Database maintenance: quiet state', () => {
  // Enabled by commit 7 of the #1422 plan.
  test.fixme('card shows the size and renders no notice, banner or button when notice is null', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(page, makeDatabaseInfo());

    await gotoSystemSettings(page);

    await test.step('the card reports the current database size', async () => {
      await expect(databaseCard(page)).toContainText(/120(\.0)? MB/);
    });

    await test.step('no notice line, no warning banner, no reclaim button', async () => {
      await expect(databaseCard(page)).not.toContainText(/next time it starts/i);
      await expect(databaseCard(page)).not.toContainText(/could be reclaimed/i);
      await expect(page.getByRole('alert')).toHaveCount(0);
      await expect(reclaimButton(page)).toHaveCount(0);
    });
  });
});

// =========================================================================
// 2. Notices by code and severity
// =========================================================================
test.describe('Database maintenance: notices', () => {
  // Enabled by commit 7 of the #1422 plan.
  test.fixme('restart_to_optimize (info) renders as a quiet line in the card, not a banner', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        notice: { code: 'restart_to_optimize', severity: 'info', reclaimable_bytes: 1.7 * GB },
      }),
    );

    await gotoSystemSettings(page);

    await test.step('the card carries the plain-language line', async () => {
      await expect(databaseCard(page)).toContainText(
        /shrink the database by about 1\.7 GB the next time it starts/i,
      );
      await expect(databaseCard(page)).toContainText(/nothing to do/i);
    });

    await test.step('it is not promoted to a page-top banner', async () => {
      await expect(page.getByRole('alert')).toHaveCount(0);
    });
  });

  // Enabled by commit 7 of the #1422 plan.
  test.fixme('database_busy (info) renders as a quiet line in the card, not a banner', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        notice: { code: 'database_busy', severity: 'info', reclaimable_bytes: 1.7 * GB },
      }),
    );

    await gotoSystemSettings(page);

    await expect(databaseCard(page)).toContainText(/postponed because the database was in use/i);
    await expect(databaseCard(page)).toContainText(/retried on the next start/i);
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  // Enabled by commit 7 of the #1422 plan.
  test.fixme('disabled_by_env (info) renders as a quiet line in the card, not a banner', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        env_mode: 'off',
        compact_requested: true,
        can_request_optimize: false,
        notice: { code: 'disabled_by_env', severity: 'info', reclaimable_bytes: 1.7 * GB },
      }),
    );

    await gotoSystemSettings(page);

    await expect(databaseCard(page)).toContainText(/CHARON_DB_COMPACT_ON_START/);
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  // Enabled by commit 7 of the #1422 plan.
  test.fixme('insufficient_disk (warning) renders as a page-top banner with plain instructions', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        disk_free_bytes: 2 * GB,
        notice: {
          code: 'insufficient_disk',
          severity: 'warning',
          reclaimable_bytes: 1.7 * GB,
          required_bytes: 9.6 * GB,
          available_bytes: 2 * GB,
        },
      }),
    );

    await gotoSystemSettings(page);

    const banner = page.getByRole('alert');
    await expect(banner).toBeVisible();
    await expect(banner).toContainText(/free up about .*GB/i);
    await expect(banner).toContainText(/restart/i);

    await test.step('the banner sits above the Database card', async () => {
      const bannerBox = await banner.boundingBox();
      const cardBox = await databaseCard(page).boundingBox();
      expect(bannerBox).not.toBeNull();
      expect(cardBox).not.toBeNull();
      expect(bannerBox!.y).toBeLessThan(cardBox!.y);
    });
  });

  // Enabled by commit 7 of the #1422 plan.
  test.fixme('too_many_failures (warning) renders as a page-top banner', async ({ page }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        notice: { code: 'too_many_failures', severity: 'warning', reclaimable_bytes: 1.7 * GB },
      }),
    );

    await gotoSystemSettings(page);

    const banner = page.getByRole('alert');
    await expect(banner).toBeVisible();
    await expect(banner).toContainText(/optimi[sz]/i);
  });
});

// =========================================================================
// 3. "Reclaim space on next restart" button
// =========================================================================
test.describe('Database maintenance: reclaim button', () => {
  // Enabled by commit 7 of the #1422 plan.
  test.fixme('button schedules the optimization, shows "Scheduled" and undo clears it', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    const harness = await setupDatabaseApi(page, makeLegacyDatabaseInfo());

    await gotoSystemSettings(page);

    await test.step('the button is offered when can_request_optimize is true', async () => {
      await expect(databaseCard(page)).toContainText(/1\.7 GB could be reclaimed/i);
      await expect(reclaimButton(page)).toBeEnabled();
    });

    await test.step('clicking POSTs the flag and shows the scheduled message', async () => {
      const posted = page.waitForRequest(
        (r) => r.url().includes('/system/database/optimize-on-restart') && r.method() === 'POST',
      );
      await reclaimButton(page).click();
      await posted;
      await expect(databaseCard(page)).toContainText(
        /scheduled - the space is reclaimed the next time charon starts/i,
      );
      expect(harness.posts).toBe(1);
    });

    await test.step('undo DELETEs the flag and restores the button', async () => {
      const deleted = page.waitForRequest(
        (r) => r.url().includes('/system/database/optimize-on-restart') && r.method() === 'DELETE',
      );
      await databaseCard(page).getByRole('button', { name: /undo/i }).click();
      await deleted;
      await expect(reclaimButton(page)).toBeVisible();
      expect(harness.deletes).toBe(1);
    });
  });

  // Enabled by commit 7 of the #1422 plan.
  test.fixme('button is hidden when auto_vacuum is already incremental', async ({ page }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeDatabaseInfo({ auto_vacuum: 'incremental', reclaimable_bytes: 500_000_000 }),
    );

    await gotoSystemSettings(page);

    await expect(reclaimButton(page)).toHaveCount(0);
  });

  // Enabled by commit 7 of the #1422 plan.
  test.fixme('button is hidden when reclaimable space is below the 100 MB floor', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        reclaimable_bytes: 50_000_000,
        can_request_optimize: false,
      }),
    );

    await gotoSystemSettings(page);

    await expect(reclaimButton(page)).toHaveCount(0);
  });

  // Enabled by commit 7 of the #1422 plan.
  test.fixme('button is shown disabled with the reason when the env override is off', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    const harness = await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({ env_mode: 'off', can_request_optimize: false }),
    );

    await gotoSystemSettings(page);

    await expect(reclaimButton(page)).toBeVisible();
    await expect(reclaimButton(page)).toBeDisabled();
    await expect(databaseCard(page)).toContainText(/disabled by CHARON_DB_COMPACT_ON_START/i);
    expect(harness.posts).toBe(0);
  });

  // Enabled by commit 7 of the #1422 plan.
  test.fixme('a repeated POST is idempotent: 200 {requested:true} every time, never 409', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(page, makeLegacyDatabaseInfo({ compact_requested: true }));

    await gotoSystemSettings(page);

    // The mock mirrors the contract (plan 3.7); the UI must treat a repeat as
    // success, so exercise it through the page's own authenticated client.
    const statuses = await page.evaluate(async () => {
      const token = window.localStorage.getItem('charon_auth_token') ?? '';
      const results: number[] = [];
      for (let i = 0; i < 3; i += 1) {
        const res = await fetch('/api/v1/system/database/optimize-on-restart', {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
        });
        results.push(res.status);
      }
      return results;
    });

    expect(statuses).toEqual([200, 200, 200]);
    await expect(databaseCard(page)).toContainText(/scheduled/i);
  });
});

// =========================================================================
// 4. Maintenance view on 503 {maintenance:true}
// =========================================================================
test.describe('Database maintenance: maintenance view', () => {
  // Enabled by commit 7 of the #1422 plan.
  test.fixme('a maintenance 503 shows the Optimizing view without logout or retry storm, then returns to the app', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);

    let maintenanceActive = true;
    let authMeCalls = 0;
    let statusCalls = 0;

    await page.route(AUTH_ME_ROUTE, async (route) => {
      authMeCalls += 1;
      if (maintenanceActive) {
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          headers: { 'Retry-After': '15' },
          body: JSON.stringify({
            error: 'Database optimization in progress',
            maintenance: true,
            retry_after_seconds: 15,
          }),
        });
        return;
      }
      await fulfillJSON(route, SESSION_USER);
    });
    await page.route(STATUS_ROUTE, async (route) => {
      statusCalls += 1;
      await fulfillJSON(route, {
        active: maintenanceActive,
        phase: maintenanceActive ? 'converting' : 'done',
        elapsed_seconds: 42,
      });
    });
    await setupDatabaseApi(page, makeDatabaseInfo());

    await page.goto('/');

    await test.step('the in-app maintenance view replaces the app', async () => {
      await expect(page.getByText(/optimizing the database/i)).toBeVisible();
      await expect(page.getByText(/your proxies are still running/i)).toBeVisible();
    });

    await test.step('the 503 is not a logout', async () => {
      await expect(page).not.toHaveURL(/\/login/);
      const token = await page.evaluate(() => window.localStorage.getItem('charon_auth_token'));
      expect(token).toBe('e2e-mock-token');
    });

    await test.step('the session check is not retry-stormed while the view is shown', async () => {
      await expect.poll(() => statusCalls).toBeGreaterThan(0);
      expect(authMeCalls).toBeLessThanOrEqual(3);
    });

    await test.step('the app returns once the status route reports active:false', async () => {
      maintenanceActive = false;
      await expect(page.getByText(/optimizing the database/i)).toBeHidden({ timeout: 15_000 });
      await expect(page).not.toHaveURL(/\/login/);
    });
  });
});

// =========================================================================
// 5. Status endpoint: static JSON in every phase
// =========================================================================
test.describe('Database maintenance: status endpoint contract', () => {
  // Enabled by commit 7 of the #1422 plan. Needs the real backend (the gate
  // answers before the router), so it uses the project's baseURL request
  // context rather than a mock.
  test.fixme('GET /api/v1/maintenance/status returns static JSON, never the SPA index', async ({
    request,
  }) => {
    const response = await request.get('/api/v1/maintenance/status');

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('application/json');
    expect(response.headers()['cache-control']).toContain('no-store');

    const body = await response.json();
    expect(body).toEqual({
      active: expect.any(Boolean),
      phase: expect.stringMatching(/^(idle|planned|checking|converting|done|skipped|failed)$/),
      elapsed_seconds: expect.any(Number),
    });
    // Idle backend (E2E compose defaults CHARON_DB_COMPACT_ON_START=off).
    expect(body.active).toBe(false);
  });
});
