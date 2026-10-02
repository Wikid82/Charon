/**
 * Automatic database maintenance (GH #1422) - E2E specs
 *
 * Covers docs/plans/current_spec.md Addendum A (D1-D8, A.4) for the passive
 * Tasks -> Database page at /tasks/database (admin only via RequireRole):
 *
 *   1. Never-empty page: the automatic-maintenance explainer, the status list
 *      (size, write-ahead log, free disk, reclaimable, mode), the last
 *      optimization block and exactly one "Nothing to do" on any page state.
 *   2. Optional Reclaim control: always visible, disabled with a reason
 *      (already incremental / under about 100 MB / env off / unavailable) or
 *      enabled per `can_request_optimize`, with the scheduled state and Undo.
 *   3. Passive notices by code: `restart_to_optimize`, `database_busy` and
 *      `disabled_by_env` (info) and `insufficient_disk` / `too_many_failures`
 *      (failure) all render as calm `role="status"` text on the Database page
 *      only: never `role="alert"`, never in the nav. The load-error Alert is
 *      the one alert and is not a notice.
 *   4. Navigation and gating: admin-only Tasks nav entry and tab, a Settings
 *      page without a Database card but with an admin-only link, and a
 *      non-admin deep link that redirects to "/" without any
 *      /system/database request.
 *   5. Maintenance view: a 503 `{maintenance:true}` from any API call shows the
 *      "Optimizing the database" view WITHOUT logging the user out or
 *      retry-storming, and returns to the app once the status route reports
 *      `active:false`.
 *   6. `/api/v1/maintenance/status` is static JSON in every phase.
 *
 * Sections 1-4 exercise the Database page (plan commits 9-11); sections 5 and 6
 * do not depend on it.
 *
 * Nothing here performs a real conversion: every endpoint is mocked with
 * page.route, in the style of tests/monitoring/uptime-monitoring-scale.spec.ts.
 * The one exception is the status-endpoint contract, which needs a live backend.
 *
 * Locators: the page content is the region named "Database" (aria-labelledby
 * the h3 `database-page-title`). Notices are `role="status"` elements, always
 * scoped by their text because toast regions share that role. The only
 * `role="alert"` on the page is the load-error Alert named "Database
 * information unavailable".
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

const NON_ADMIN_USER = {
  user_id: 2,
  role: 'user',
  name: 'E2E Maintenance User',
  email: 'e2e-maintenance-user@test.local',
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
async function stubAuthenticatedSession(
  page: Page,
  sessionUser: typeof SESSION_USER = SESSION_USER,
): Promise<void> {
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
      await fulfillJSON(route, sessionUser);
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

const DATABASE_PAGE_PATH = '/tasks/database';

async function gotoDatabasePage(page: Page): Promise<void> {
  await page.goto(DATABASE_PAGE_PATH);
  await expect(page.getByRole('heading', { name: /^database$/i, level: 3 })).toBeVisible();
}

function databaseRegion(page: Page) {
  return page.getByRole('region', { name: /^database$/i });
}

/**
 * A passive notice: `role="status"` scoped by its text. Never a bare
 * getByRole('status'), because toast regions share the role.
 */
function noticeWithText(page: Page, text: RegExp) {
  return page.getByRole('status').filter({ hasText: text });
}

/** Sidebar navigation links (the Settings page link lives in main, not here). */
function sidebarDatabaseLink(page: Page) {
  return page.getByRole('navigation').getByRole('link', { name: /(^|\s)database$/i });
}

/** The sidebar accordions start collapsed, so open Tasks before looking for its children. */
async function expandTasksNav(page: Page): Promise<void> {
  const tasksToggle = page.getByRole('navigation').getByRole('button', { name: /(^|\s)tasks$/i });
  await expect(tasksToggle).toBeVisible();
  if ((await tasksToggle.getAttribute('aria-expanded')) !== 'true') {
    await tasksToggle.click();
  }
  await expect(tasksToggle).toHaveAttribute('aria-expanded', 'true');
}

const reclaimButton = (page: Page) =>
  databaseRegion(page).getByRole('button', { name: /reclaim space on next restart/i });

/** Records every /system/database request so non-admin cases can assert none was made. */
function trackDatabaseRequests(page: Page): string[] {
  const seen: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/api/v1/system/database')) {
      seen.push(`${request.method()} ${request.url()}`);
    }
  });
  return seen;
}

// =========================================================================
// 1. Never-empty page
// =========================================================================
test.describe('Database page: never empty', () => {
  test('quiet incremental state shows explainer, status fields, one "Nothing to do" and a disabled optional button', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    const harness = await setupDatabaseApi(page, makeDatabaseInfo());

    await gotoDatabasePage(page);
    const region = databaseRegion(page);

    await test.step('the explainer lists the three automatic items', async () => {
      await expect(region).toContainText(/looks after its database by itself/i);
      await expect(region).toContainText(/checked at every start/i);
      await expect(region).toContainText(/converted once when worthwhile/i);
      await expect(region).toContainText(/handed back after each hourly cleanup/i);
    });

    await test.step('the status list reports size, WAL, free disk, reclaimable and mode', async () => {
      await expect(region).toContainText(/120(\.0)? MB/);
      await expect(region).toContainText(/write-ahead log/i);
      await expect(region).toContainText(/free disk space/i);
      await expect(region).toContainText(/could be reclaimed|space that could be reclaimed/i);
      await expect(region).toContainText(/automatic: freed space goes back to your disk on its own/i);
    });

    await test.step('last optimization says none was needed, and nothing to do appears exactly once', async () => {
      await expect(region).toContainText(/no optimization has been needed so far/i);
      await expect(region).not.toContainText(/not optimized yet/i);
      await expect(page.getByText(/nothing to do/i)).toHaveCount(1);
    });

    await test.step('the optional reclaim button is visible, disabled and explained', async () => {
      await expect(region).toContainText(/reclaim space now \(optional\)/i);
      await expect(region).toContainText(/charon already does this automatically/i);
      await expect(reclaimButton(page)).toBeVisible();
      await expect(reclaimButton(page)).toBeDisabled();
      await expect(region).toContainText(/database already returns unused space automatically/i);
      expect(harness.posts).toBe(0);
    });

    await test.step('no notice and no alert on a healthy database', async () => {
      await expect(page.getByRole('alert')).toHaveCount(0);
      await expect(noticeWithText(page, /next time it starts|could not run|has stopped/i)).toHaveCount(0);
    });
  });

  test('last_result null with can_request_optimize shows "Not optimized yet", an enabled button and no "Nothing to do"', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(page, makeLegacyDatabaseInfo());

    await gotoDatabasePage(page);
    const region = databaseRegion(page);

    await expect(region).toContainText(/not optimized yet/i);
    await expect(region).not.toContainText(/no optimization has been needed so far/i);
    await expect(page.getByText(/nothing to do/i)).toHaveCount(0);
    await expect(reclaimButton(page)).toBeEnabled();
    await expect(region).toContainText(/1\.7 GB could be reclaimed/i);
  });

  test('last_result null with optimization switched off but plenty reclaimable still shows "Not optimized yet"', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({ env_mode: 'off', can_request_optimize: false }),
    );

    await gotoDatabasePage(page);
    const region = databaseRegion(page);

    await expect(region).toContainText(/not optimized yet/i);
    await expect(region).not.toContainText(/no optimization has been needed so far/i);
    await expect(reclaimButton(page)).toBeDisabled();
    await expect(page.getByText(/nothing to do/i)).toHaveCount(0);
  });

  test('last_result null with compact_requested shows "Not optimized yet", the scheduled state and Undo', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({ compact_requested: true, can_request_optimize: false }),
    );

    await gotoDatabasePage(page);
    const region = databaseRegion(page);

    await expect(region).toContainText(/not optimized yet/i);
    await expect(page.getByText(/nothing to do/i)).toHaveCount(0);
    await expect(region).toContainText(/scheduled - the space is reclaimed the next time charon starts/i);
    await expect(region.getByRole('button', { name: /undo/i })).toBeVisible();
  });

  test('last optimization renders a completed conversion in plain words without raw codes', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeDatabaseInfo({
        last_result: {
          at: '2026-09-30T04:00:00Z',
          outcome: 'converted',
          reason: '',
          bytes_before: 2_277_880_440,
          bytes_after: 1_268_629_504,
        },
      }),
    );

    await gotoDatabasePage(page);
    const region = databaseRegion(page);

    await expect(region).toContainText(/optimized on/i);
    await expect(region).toContainText(/saved/i);
    await expect(region).not.toContainText(/invalid date/i);
    await expect(region).not.toContainText(/converted_pending_checkpoint|conversion_failed/);
    // The incremental reason is the only "nothing to do" even with a last result.
    await expect(page.getByText(/nothing to do/i)).toHaveCount(1);
  });

  test('an incremental database whose last result was skipped as already optimized still has exactly one "Nothing to do"', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeDatabaseInfo({
        last_result: {
          at: '2026-09-30T04:00:00Z',
          outcome: 'skipped',
          reason: 'already_optimized',
          bytes_before: 0,
          bytes_after: 0,
        },
      }),
    );

    await gotoDatabasePage(page);

    await expect(databaseRegion(page)).toContainText(/skipped on/i);
    await expect(page.getByText(/nothing to do/i)).toHaveCount(1);
    await expect(databaseRegion(page)).not.toContainText(/already_optimized/);
  });
});

// =========================================================================
// 2. Passive notices by code
// =========================================================================
test.describe('Database page: passive notices', () => {
  test('restart_to_optimize (info) renders as calm status text on the page, not an alert', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        notice: { code: 'restart_to_optimize', severity: 'info', reclaimable_bytes: 1.7 * GB },
      }),
    );

    await gotoDatabasePage(page);

    const notice = noticeWithText(page, /reclaim about 1\.7 GB automatically the next time it starts/i);
    await expect(notice).toBeVisible();
    await expect(notice).toContainText(/nothing is needed from you/i);
    await expect(page.getByRole('alert')).toHaveCount(0);
    // Reworded so the phrase stays unique to the incremental reason.
    await expect(page.getByText(/nothing to do/i)).toHaveCount(0);
  });

  test('database_busy (info) renders as calm status text on the page', async ({ page }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        notice: { code: 'database_busy', severity: 'info', reclaimable_bytes: 1.7 * GB },
      }),
    );

    await gotoDatabasePage(page);

    const notice = noticeWithText(page, /postponed because the database was in use/i);
    await expect(notice).toBeVisible();
    await expect(notice).toContainText(/retried on the next start/i);
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  test('disabled_by_env (info) renders as calm status text naming the environment variable', async ({
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

    await gotoDatabasePage(page);

    await expect(noticeWithText(page, /CHARON_DB_COMPACT_ON_START/)).toBeVisible();
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  test('insufficient_disk renders as a calm in-page status notice, never an alert', async ({
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

    await gotoDatabasePage(page);

    const notice = noticeWithText(page, /automatic database cleanup because the disk is nearly full/i);
    await expect(notice).toBeVisible();
    await expect(notice).toContainText(/free some disk space/i);
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  test('too_many_failures renders as a calm in-page status notice, never an alert', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        notice: { code: 'too_many_failures', severity: 'warning', reclaimable_bytes: 1.7 * GB },
      }),
    );

    await gotoDatabasePage(page);

    const notice = noticeWithText(page, /stopped trying its automatic database cleanup/i);
    await expect(notice).toBeVisible();
    await expect(notice).toContainText(/several failed or interrupted attempts/i);
    await expect(notice).toContainText(/your proxies are not affected/i);
    await expect(notice).toContainText(/let the optimization finish; avoid restarting while it runs/i);
    await expect(notice).toContainText(/optional button below to schedule it, then restart charon/i);
    await expect(reclaimButton(page)).toBeEnabled();
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  test('a failure notice changes nothing in the nav on "/" and appears only on the Database page', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({
        notice: { code: 'too_many_failures', severity: 'warning', reclaimable_bytes: 1.7 * GB },
      }),
    );

    await page.goto('/');

    await test.step('the dashboard shows no notice, alert or banner for the database', async () => {
      await expect(page.getByRole('navigation')).toBeVisible();
      await expandTasksNav(page);
      await expect(noticeWithText(page, /automatic database cleanup/i)).toHaveCount(0);
      await expect(page.getByRole('alert', { name: /database/i })).toHaveCount(0);
    });

    await test.step('the Database nav link keeps exactly its normal name', async () => {
      await expect(sidebarDatabaseLink(page)).toHaveAccessibleName(/^(\S+\s)?database$/i);
    });

    await test.step('the notice renders only on the Database page', async () => {
      await gotoDatabasePage(page);
      await expect(noticeWithText(page, /stopped trying its automatic database cleanup/i)).toBeVisible();
    });
  });

  test('the load-error Alert has its own name and is not mistaken for a notice', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await page.route(DATABASE_ROUTE, (route) => fulfillJSON(route, { error: 'boom' }, 500));
    await page.route(FEATURE_FLAGS_ROUTE, (route) => fulfillJSON(route, {}));

    await page.goto(DATABASE_PAGE_PATH);

    await test.step('the alert is named and offers a retry', async () => {
      const alert = page.getByRole('alert', { name: 'Database information unavailable' });
      await expect(alert).toBeVisible();
      await expect(alert.getByRole('button', { name: /retry/i })).toBeVisible();
    });

    await test.step('no maintenance notice is rendered next to the error', async () => {
      await expect(
        noticeWithText(page, /automatic database cleanup|next time it starts|postponed|CHARON_DB_COMPACT_ON_START/i),
      ).toHaveCount(0);
    });
  });
});

// =========================================================================
// 3. Optional Reclaim control
// =========================================================================
test.describe('Database page: reclaim control', () => {
  test('button schedules the optimization, shows "Scheduled" and undo clears it', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    const harness = await setupDatabaseApi(page, makeLegacyDatabaseInfo());

    await gotoDatabasePage(page);

    await test.step('the optional button is enabled when can_request_optimize is true', async () => {
      await expect(databaseRegion(page)).toContainText(/reclaim space now \(optional\)/i);
      await expect(databaseRegion(page)).toContainText(/1\.7 GB could be reclaimed/i);
      await expect(reclaimButton(page)).toBeEnabled();
    });

    await test.step('clicking POSTs the flag and shows the scheduled message', async () => {
      const posted = page.waitForRequest(
        (r) => r.url().includes('/system/database/optimize-on-restart') && r.method() === 'POST',
      );
      await reclaimButton(page).click();
      await posted;
      await expect(databaseRegion(page)).toContainText(
        /scheduled - the space is reclaimed the next time charon starts/i,
      );
      expect(harness.posts).toBe(1);
    });

    await test.step('undo DELETEs the flag and restores the button', async () => {
      const deleted = page.waitForRequest(
        (r) => r.url().includes('/system/database/optimize-on-restart') && r.method() === 'DELETE',
      );
      await databaseRegion(page).getByRole('button', { name: /undo/i }).click();
      await deleted;
      await expect(reclaimButton(page)).toBeVisible();
      expect(harness.deletes).toBe(1);
    });
  });

  test('button is disabled with the reason when auto_vacuum is already incremental', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeDatabaseInfo({ auto_vacuum: 'incremental', reclaimable_bytes: 500_000_000 }),
    );

    await gotoDatabasePage(page);

    await expect(reclaimButton(page)).toBeDisabled();
    await expect(databaseRegion(page)).toContainText(/already returns unused space automatically/i);
  });

  test('button is disabled with the reason when reclaimable space is under about 100 MB', async ({
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

    await gotoDatabasePage(page);

    await expect(reclaimButton(page)).toBeDisabled();
    await expect(databaseRegion(page)).toContainText(/not worth reclaiming/i);
    await expect(databaseRegion(page)).toContainText(/about 100 MB/i);
  });

  test('button is disabled with the reason when the env override is off', async ({ page }) => {
    await stubAuthenticatedSession(page);
    const harness = await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({ env_mode: 'off', can_request_optimize: false }),
    );

    await gotoDatabasePage(page);

    await expect(reclaimButton(page)).toBeVisible();
    await expect(reclaimButton(page)).toBeDisabled();
    await expect(databaseRegion(page)).toContainText(/disabled by CHARON_DB_COMPACT_ON_START/i);
    expect(harness.posts).toBe(0);
  });

  test('button is disabled with "Not available right now" for any other not-allowed state', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(
      page,
      makeLegacyDatabaseInfo({ reclaimable_bytes: 500_000_000, can_request_optimize: false }),
    );

    await gotoDatabasePage(page);

    await expect(reclaimButton(page)).toBeDisabled();
    await expect(databaseRegion(page)).toContainText(/not available right now/i);
  });

  test('a repeated POST is idempotent: 200 {requested:true} every time, never 409', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(page, makeLegacyDatabaseInfo({ compact_requested: true }));

    await gotoDatabasePage(page);

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
    await expect(databaseRegion(page)).toContainText(/scheduled/i);
  });
});

// =========================================================================
// 4. Navigation, Settings page and role gating
// =========================================================================
test.describe('Database page: navigation and gating', () => {
  test('admin sees Database in the Tasks nav and as a Tasks tab, as plain links', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(page, makeDatabaseInfo());

    // Start at the dashboard: the generic `[]` API fallback is not a valid Backups
    // payload, so this test never renders the Backups page.
    await page.goto('/');

    await test.step('the sidebar lists Database under Tasks as a plain link', async () => {
      await expandTasksNav(page);
      await expect(sidebarDatabaseLink(page)).toBeVisible();
      await expect(sidebarDatabaseLink(page)).toHaveAccessibleName(/^(\S+\s)?database$/i);
      await sidebarDatabaseLink(page).click();
      await expect(page).toHaveURL(/\/tasks\/database$/);
      await expect(page.getByRole('heading', { name: /^database$/i, level: 3 })).toBeVisible();
    });

    await test.step('the Tasks tab bar carries the same plain link', async () => {
      await expect(page.getByRole('main').getByRole('link', { name: /^database$/i })).toBeVisible();
    });
  });

  test('System Settings has no Database card and links admins to Tasks -> Database', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page);
    await setupDatabaseApi(page, makeLegacyDatabaseInfo({
      notice: { code: 'restart_to_optimize', severity: 'info', reclaimable_bytes: 1.7 * GB },
    }));

    await page.goto('/settings/system');
    await expect(page.getByRole('heading', { name: /system settings/i }).first()).toBeVisible();

    await expect(page.getByRole('region', { name: /^database$/i })).toHaveCount(0);
    await expect(noticeWithText(page, /next time it starts/i)).toHaveCount(0);

    const link = page.getByRole('main').getByRole('link', { name: /database size and cleanup/i });
    await expect(link).toBeVisible();
    await link.click();
    await expect(page).toHaveURL(/\/tasks\/database$/);
  });

  test('a non-admin has no Database nav item or Settings link and makes no /system/database request', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page, NON_ADMIN_USER);
    await setupDatabaseApi(page, makeDatabaseInfo());
    const requests = trackDatabaseRequests(page);

    await page.goto('/settings/system');
    await expect(page.getByRole('heading', { name: /system settings/i }).first()).toBeVisible();

    // Open Tasks first: a collapsed accordion would make the absence check vacuous.
    await expandTasksNav(page);
    await expect(sidebarDatabaseLink(page)).toHaveCount(0);
    await expect(page.getByRole('link', { name: /database size and cleanup/i })).toHaveCount(0);
    expect(requests).toEqual([]);
  });

  test('a non-admin deep link to /tasks/database redirects to "/" without a /system/database request', async ({
    page,
  }) => {
    await stubAuthenticatedSession(page, NON_ADMIN_USER);
    await setupDatabaseApi(page, makeDatabaseInfo());
    const requests = trackDatabaseRequests(page);

    await page.goto(DATABASE_PAGE_PATH);

    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByRole('heading', { name: /^database$/i, level: 3 })).toHaveCount(0);
    expect(requests).toEqual([]);
  });
});

// =========================================================================
// 5. Maintenance view on 503 {maintenance:true}
// =========================================================================
test.describe('Database maintenance: maintenance view', () => {
  test('a maintenance 503 shows the Optimizing view without logout or retry storm, then returns to the app', async ({
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
// 6. Status endpoint: static JSON in every phase
// =========================================================================
test.describe('Database maintenance: status endpoint contract', () => {
  test('GET /api/v1/maintenance/status returns static JSON, never the SPA index', async ({
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
