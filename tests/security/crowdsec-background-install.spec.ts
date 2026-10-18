/**
 * CrowdSec background hub preset install E2E Tests (#1615, PR 6)
 *
 * Applying a hub preset can take minutes, so the UI sends `async: true`, receives a 202 with a
 * job id and shows a passive progress row in the presets card. These specs drive that flow with
 * route-mocked job endpoints (the E2E container has no cscli, hub or LAPI):
 * - progress row with fixed phase text, item count and elapsed time, polled until a terminal state
 * - resume after a page reload through `GET .../jobs/current`
 * - Cancel (DELETE) and the cancelled final state with its rollback note
 * - `crowdsec_busy` (409) text for Start and config import while a job is active; reads stay usable
 * - failed, interrupted and succeeded final states
 * - passive UI: no banner, no alert, no toast per poll
 *
 * All specs are `test.fixme` until the UI lands (PR 6 commit 7); commit 8 enables them.
 * Accessible names below are the intended copy; the i18n key each one maps to is noted beside it
 * and the exact English strings must be reconciled with the locale file when the fixme is lifted.
 */

import type { Page, Route } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { CROWDSEC_ROUTES, stubCrowdSecApi } from '../utils/crowdsec-stubs';

const ADMIN = String.raw`/api/v1/admin/crowdsec`;
const END = String.raw`(?:\?.*)?$`;

const PRESETS_ROUTE = '**/api/v1/admin/crowdsec/presets';
const PULL_ROUTE = '**/api/v1/admin/crowdsec/presets/pull';
const APPLY_ROUTE = new RegExp(`${ADMIN}/presets/apply${END}`);
const JOB_CURRENT_ROUTE = new RegExp(`${ADMIN}/presets/apply/jobs/current${END}`);
const JOB_BY_ID_ROUTE = new RegExp(`${ADMIN}/presets/apply/jobs/(?!current)[^/?]+${END}`);
const START_ROUTE = new RegExp(`${ADMIN}/start${END}`);
const IMPORT_ROUTE = new RegExp(`${ADMIN}/import${END}`);

const HUB_PRESET_SLUG = 'sigmahq/windows_proc_creation';
const HUB_PRESET_TITLE = 'Windows Process Creation';
const CACHE_KEY = 'e2e-background-install-cache-key';
const JOB_ID = '7b1f6c1e-0c55-4a53-9d3c-5a0d3b6f2e10';
const STARTED_AT = '2026-10-18T10:00:00Z';

type JobState = 'queued' | 'running' | 'verifying' | 'succeeded' | 'failed' | 'timed_out' | 'cancelled';
type JobPhase = 'preflight' | 'snapshot' | 'install' | 'verify' | 'reload' | 'rollback';

interface JobStatusOptions {
  state: JobState;
  phase?: JobPhase;
  itemsInstalled?: number;
  elapsedSeconds?: number;
  errorCode?: string | null;
  messageCode?: string | null;
  result?: Record<string, unknown> | null;
}

/** Builds a job status body shaped like `GET /admin/crowdsec/presets/apply/jobs/:id` (spec 8.2). */
function jobStatus(options: JobStatusOptions): Record<string, unknown> {
  const terminal = ['succeeded', 'failed', 'timed_out', 'cancelled'].includes(options.state);
  return {
    job_id: JOB_ID,
    slug: HUB_PRESET_SLUG,
    cache_key: CACHE_KEY,
    state: options.state,
    phase: options.phase ?? 'install',
    items_installed: options.itemsInstalled ?? 0,
    items_total: null,
    elapsed_seconds: options.elapsedSeconds ?? 0,
    started_at: STARTED_AT,
    finished_at: terminal ? '2026-10-18T10:06:00Z' : null,
    message_code: options.messageCode ?? null,
    message: '',
    result: options.result ?? null,
    error_code: options.errorCode ?? null,
  };
}

async function fulfillJson(route: Route, status: number, json: unknown): Promise<void> {
  await route.fulfill({ status, contentType: 'application/json', json });
}

/** Stubs the preset list with one hub preset and a successful pull so Apply is reachable. */
async function stubHubPreset(page: Page): Promise<void> {
  await page.route(PRESETS_ROUTE, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await fulfillJson(route, 200, {
      presets: [
        {
          slug: HUB_PRESET_SLUG,
          title: HUB_PRESET_TITLE,
          summary: 'Large hub collection for E2E',
          source: 'hub',
          requires_hub: true,
          available: true,
          cached: false,
        },
      ],
    });
  });
  await page.route(PULL_ROUTE, async (route) => {
    await fulfillJson(route, 200, {
      status: 'pulled',
      slug: HUB_PRESET_SLUG,
      preview: 'name: windows_proc_creation\n',
      cache_key: CACHE_KEY,
      source: 'hub',
    });
  });
}

/** Answers `GET jobs/current` with 404 (no active or recent job) unless a test overrides it. */
async function stubNoCurrentJob(page: Page): Promise<void> {
  await page.route(JOB_CURRENT_ROUTE, async (route) => {
    await fulfillJson(route, 404, { error: 'no job', error_code: 'job_not_found' });
  });
}

/** Replays `responses` for successive `GET jobs/:id` polls, repeating the last one. */
async function stubJobPolls(page: Page, responses: Array<Record<string, unknown>>): Promise<{ polls: () => number }> {
  let polls = 0;
  await page.route(JOB_BY_ID_ROUTE, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    const body = responses[Math.min(polls, responses.length - 1)];
    polls += 1;
    await fulfillJson(route, 200, body);
  });
  return { polls: () => polls };
}

/** Stubs the async apply start with a 202 and records the request bodies. */
async function stubAsyncApply(page: Page): Promise<Array<Record<string, unknown>>> {
  const bodies: Array<Record<string, unknown>> = [];
  await page.route(APPLY_ROUTE, async (route) => {
    bodies.push(route.request().postDataJSON() as Record<string, unknown>);
    await fulfillJson(route, 202, {
      job_id: JOB_ID,
      state: 'queued',
      slug: HUB_PRESET_SLUG,
      cache_key: CACHE_KEY,
      started_at: STARTED_AT,
    });
  });
  return bodies;
}

async function loadPage(page: Page): Promise<void> {
  await page.goto('/security/crowdsec');
  await waitForLoadingComplete(page);
}

/** Selects the hub preset, pulls the preview and clicks Apply. */
async function startApply(page: Page): Promise<void> {
  await page.getByRole('button', { name: new RegExp(HUB_PRESET_TITLE, 'i') }).click();
  await expect(page.getByText('Preview fetched from hub')).toBeVisible();
  await page.getByRole('button', { name: /apply preset/i }).click();
}

/** The inline progress row; intended i18n: crowdsecConfig.presets.installProgress.* */
function progressRow(page: Page) {
  return page.getByRole('status', { name: /installing preset/i });
}

test.describe('CrowdSec background hub preset install @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await stubHubPreset(page);
  });

  test.fixme('should start the install asynchronously and show a passive progress row', async ({ page }) => {
    const applyBodies = await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    await stubJobPolls(page, [
      jobStatus({ state: 'running', phase: 'snapshot', itemsInstalled: 0, elapsedSeconds: 2 }),
      jobStatus({ state: 'running', phase: 'install', itemsInstalled: 12, elapsedSeconds: 200 }),
    ]);

    await loadPage(page);
    await startApply(page);

    await test.step('Verify the apply request opted in to a background job', async () => {
      await expect.poll(() => applyBodies.length).toBe(1);
      expect(applyBodies[0]).toMatchObject({ slug: HUB_PRESET_SLUG, cache_key: CACHE_KEY, async: true });
    });

    await test.step('Verify the progress row shows the phase, item count and elapsed time', async () => {
      const row = progressRow(page);
      await expect(row).toBeVisible();
      // phases: crowdsecConfig.presets.installProgress.phase.{preflight,snapshot,install,verify,reload,rollback}
      await expect(row).toContainText(/installing/i);
      // item count (ICU plural): crowdsecConfig.presets.installProgress.items
      await expect(row).toContainText(/12 items/i);
      // elapsed time: crowdsecConfig.presets.installProgress.elapsed
      await expect(row).toContainText(/3 min 20 s/i);
      await expect(page.getByRole('button', { name: /cancel/i })).toBeVisible();
    });

    await test.step('Verify the progress UI stays passive', async () => {
      await expect(page.getByRole('alert')).toHaveCount(0);
      await expect(page.getByRole('progressbar')).toHaveCount(0);
    });
  });

  test.fixme('should show the running phases in order as polls advance', async ({ page }) => {
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    await stubJobPolls(page, [
      jobStatus({ state: 'running', phase: 'preflight' }),
      jobStatus({ state: 'running', phase: 'snapshot' }),
      jobStatus({ state: 'running', phase: 'install', itemsInstalled: 3 }),
      jobStatus({ state: 'verifying', phase: 'verify', itemsInstalled: 40 }),
      jobStatus({ state: 'verifying', phase: 'reload', itemsInstalled: 40 }),
    ]);

    await loadPage(page);
    await startApply(page);

    const row = progressRow(page);
    await expect(row).toContainText(/preparing/i);
    await expect(row).toContainText(/snapshot/i);
    await expect(row).toContainText(/installing/i);
    await expect(row).toContainText(/verifying/i);
    await expect(row).toContainText(/reloading/i);
  });

  test.fixme('should poll until a terminal state, then stop polling and show the result once', async ({ page }) => {
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    const jobPolls = await stubJobPolls(page, [
      jobStatus({ state: 'running', phase: 'install', itemsInstalled: 5, elapsedSeconds: 4 }),
      jobStatus({
        state: 'succeeded',
        phase: 'reload',
        itemsInstalled: 41,
        elapsedSeconds: 360,
        result: {
          status: 'applied',
          backup: '/data/backups/pre-apply',
          reload_hint: true,
          used_cscli: true,
          cache_key: CACHE_KEY,
          slug: HUB_PRESET_SLUG,
          warnings: [],
        },
      }),
    ]);

    await loadPage(page);
    await startApply(page);

    await test.step('Verify the succeeded state replaces the progress row', async () => {
      // crowdsecConfig.presets.installProgress.succeeded
      await expect(page.getByText(/preset applied/i)).toBeVisible();
      await expect(progressRow(page)).toBeHidden();
      await expect(page.getByRole('button', { name: /apply preset/i })).toBeEnabled();
    });

    await test.step('Verify no further polls are issued after the terminal state', async () => {
      const pollsAtTerminal = jobPolls.polls();
      // A terminal job must not be polled again; poll count settles and stays put.
      await expect.poll(() => jobPolls.polls(), { timeout: 3000 }).toBe(pollsAtTerminal);
    });
  });

  test.fixme('should resume the progress row after a page reload from the current job', async ({ page }) => {
    await stubAsyncApply(page);
    let currentRequests = 0;
    await page.route(JOB_CURRENT_ROUTE, async (route) => {
      currentRequests += 1;
      // Before Apply there is no job; once started (and after the reload) the job is active.
      await fulfillJson(
        route,
        currentRequests === 1 ? 404 : 200,
        currentRequests === 1 ? { error: 'no job', error_code: 'job_not_found' } : jobStatus({
          state: 'running',
          phase: 'install',
          itemsInstalled: 18,
          elapsedSeconds: 95,
        }),
      );
    });
    await stubJobPolls(page, [
      jobStatus({ state: 'running', phase: 'install', itemsInstalled: 18, elapsedSeconds: 95 }),
    ]);

    await loadPage(page);
    await expect(progressRow(page)).toBeHidden();

    await test.step('Reload and verify the active job is resumed without restarting it', async () => {
      await page.reload();
      await waitForLoadingComplete(page);
      const row = progressRow(page);
      await expect(row).toBeVisible();
      await expect(row).toContainText(/18 items/i);
      await expect(row).toContainText(/1 min 35 s/i);
      await expect(page.getByRole('button', { name: /cancel/i })).toBeVisible();
    });
  });

  test.fixme('should show a recently finished job once on load', async ({ page }) => {
    await page.route(JOB_CURRENT_ROUTE, async (route) => {
      await fulfillJson(
        route,
        200,
        jobStatus({
          state: 'succeeded',
          phase: 'reload',
          itemsInstalled: 41,
          elapsedSeconds: 360,
          result: {
            status: 'applied',
            backup: '/data/backups/pre-apply',
            reload_hint: true,
            used_cscli: true,
            cache_key: CACHE_KEY,
            slug: HUB_PRESET_SLUG,
            warnings: [],
          },
        }),
      );
    });

    await loadPage(page);

    await expect(page.getByText(/preset applied/i)).toBeVisible();
    await expect(progressRow(page)).toBeHidden();
    await expect(page.getByRole('button', { name: /cancel/i })).toHaveCount(0);
  });

  test.fixme('should cancel a running install and show the cancelled state with a rollback note', async ({ page }) => {
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    let cancelled = false;
    let cancelRequests = 0;
    await page.route(JOB_BY_ID_ROUTE, async (route) => {
      const method = route.request().method();
      if (method === 'DELETE') {
        cancelRequests += 1;
        cancelled = true;
        await fulfillJson(route, 202, { job_id: JOB_ID, state: 'running' });
        return;
      }
      await fulfillJson(
        route,
        200,
        cancelled
          ? jobStatus({ state: 'cancelled', phase: 'rollback', itemsInstalled: 0, elapsedSeconds: 60, messageCode: 'cancelled' })
          : jobStatus({ state: 'running', phase: 'install', itemsInstalled: 9, elapsedSeconds: 30 }),
      );
    });

    await loadPage(page);
    await startApply(page);
    await expect(progressRow(page)).toContainText(/installing/i);

    await test.step('Click Cancel and verify one DELETE is sent', async () => {
      await page.getByRole('button', { name: /cancel/i }).click();
      await expect.poll(() => cancelRequests).toBe(1);
    });

    await test.step('Verify the cancelled state explains the rollback', async () => {
      // crowdsecConfig.presets.installProgress.cancelled
      await expect(page.getByText(/install cancelled/i)).toBeVisible();
      await expect(page.getByText(/previous configuration was restored/i)).toBeVisible();
      await expect(progressRow(page)).toBeHidden();
      await expect(page.getByRole('button', { name: /apply preset/i })).toBeEnabled();
    });
  });

  test.fixme('should show the failure text and rollback note when the install fails', async ({ page }) => {
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    await stubJobPolls(page, [
      jobStatus({ state: 'running', phase: 'install', itemsInstalled: 2, elapsedSeconds: 5 }),
      jobStatus({
        state: 'failed',
        phase: 'rollback',
        itemsInstalled: 0,
        elapsedSeconds: 40,
        errorCode: 'preset_local_modified',
        messageCode: 'preset_local_modified',
      }),
    ]);

    await loadPage(page);
    await startApply(page);

    // crowdsecConfig.presets.installProgress.failed + the existing preset_local_modified error text
    await expect(page.getByText(/install failed/i)).toBeVisible();
    await expect(page.getByText(/previous configuration was restored/i)).toBeVisible();
    await expect(page.getByText(/preset applied/i)).toHaveCount(0);
    await expect(progressRow(page)).toBeHidden();
    await expect(page.getByRole('button', { name: /apply preset/i })).toBeEnabled();
  });

  test.fixme('should show the interrupted state when the server restarted mid-install', async ({ page }) => {
    await stubAsyncApply(page);
    await page.route(JOB_CURRENT_ROUTE, async (route) => {
      await fulfillJson(
        route,
        200,
        jobStatus({
          state: 'failed',
          phase: 'rollback',
          elapsedSeconds: 120,
          errorCode: 'interrupted',
          messageCode: 'interrupted',
        }),
      );
    });

    await loadPage(page);

    // crowdsecConfig.presets.installProgress.interrupted
    await expect(page.getByText(/install was interrupted/i)).toBeVisible();
    await expect(page.getByText(/previous configuration was restored/i)).toBeVisible();
    await expect(progressRow(page)).toBeHidden();
  });

  test.fixme('should show the timed out state with a rollback note', async ({ page }) => {
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    await stubJobPolls(page, [
      jobStatus({
        state: 'timed_out',
        phase: 'rollback',
        elapsedSeconds: 1800,
        errorCode: 'preset_apply_timeout',
        messageCode: 'preset_apply_timeout',
      }),
    ]);

    await loadPage(page);
    await startApply(page);

    // crowdsecConfig.presets.installProgress.timedOut
    await expect(page.getByText(/install took too long/i)).toBeVisible();
    await expect(page.getByText(/previous configuration was restored/i)).toBeVisible();
  });

  test.fixme('should show "waiting for the server" and recover when status polls fail', async ({ page }) => {
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    let polls = 0;
    await page.route(JOB_BY_ID_ROUTE, async (route) => {
      polls += 1;
      if (polls === 1) {
        await route.abort('connectionrefused');
        return;
      }
      await fulfillJson(route, 200, jobStatus({ state: 'running', phase: 'install', itemsInstalled: 7, elapsedSeconds: 8 }));
    });

    await loadPage(page);
    await startApply(page);

    // crowdsecConfig.presets.installProgress.waitingForServer
    await expect(progressRow(page)).toContainText(/waiting for the server/i);
    await expect(progressRow(page)).toContainText(/7 items/i);
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  test.fixme('should answer a second apply with the busy text and show the active job', async ({ page }) => {
    await stubNoCurrentJob(page);
    await page.route(APPLY_ROUTE, async (route) => {
      await fulfillJson(route, 409, {
        error: 'another CrowdSec install is running',
        error_code: 'crowdsec_busy',
        job_id: JOB_ID,
      });
    });
    await stubJobPolls(page, [jobStatus({ state: 'running', phase: 'install', itemsInstalled: 4, elapsedSeconds: 20 })]);

    await loadPage(page);
    await startApply(page);

    // translateApiError: errors.crowdsec_busy
    await expect(page.getByText(/crowdsec is busy installing a preset/i)).toBeVisible();
    await expect(page.getByText(/preset applied/i)).toHaveCount(0);
  });

  test.describe('while a job is active', () => {
    test.beforeEach(async ({ page }) => {
      await stubCrowdSecApi(page, {});
      await page.route(JOB_CURRENT_ROUTE, async (route) => {
        await fulfillJson(
          route,
          200,
          jobStatus({ state: 'running', phase: 'install', itemsInstalled: 11, elapsedSeconds: 70 }),
        );
      });
      await stubJobPolls(page, [jobStatus({ state: 'running', phase: 'install', itemsInstalled: 11, elapsedSeconds: 70 })]);
    });

    test.fixme('should show the busy text when starting CrowdSec during a job', async ({ page }) => {
      let startRequests = 0;
      await page.route(START_ROUTE, async (route) => {
        startRequests += 1;
        await fulfillJson(route, 409, {
          error: 'CrowdSec is busy',
          error_code: 'crowdsec_busy',
          job_id: JOB_ID,
        });
      });

      await loadPage(page);
      await expect(progressRow(page)).toBeVisible();

      await page.getByRole('button', { name: /^start$/i }).click();

      await test.step('Verify the translated busy message and no extra polling or banner', async () => {
        await expect(page.getByText(/crowdsec is busy installing a preset/i)).toBeVisible();
        expect(startRequests).toBe(1);
        // The progress row remains; the busy message is inline text, not a persistent banner.
        await expect(progressRow(page)).toBeVisible();
      });
    });

    test.fixme('should show the busy text when importing a configuration during a job', async ({ page }) => {
      await page.route(IMPORT_ROUTE, async (route) => {
        await fulfillJson(route, 409, {
          error: 'CrowdSec is busy',
          error_code: 'crowdsec_busy',
          job_id: JOB_ID,
        });
      });

      await loadPage(page);
      await expect(progressRow(page)).toBeVisible();

      // Hidden file input behind the "Import" control; role-based fallback is the labelled input.
      await page.getByLabel(/import/i).setInputFiles({
        name: 'crowdsec-config.tar.gz',
        mimeType: 'application/gzip',
        buffer: Buffer.from('e2e-archive'),
      });
      await page.getByRole('button', { name: /^import$/i }).click();

      await expect(page.getByText(/crowdsec is busy installing a preset/i)).toBeVisible();
    });

    test.fixme('should keep read-only views usable during a job', async ({ page }) => {
      let exportRequests = 0;
      await page.route(CROWDSEC_ROUTES.export, async (route) => {
        exportRequests += 1;
        await route.fulfill({
          status: 200,
          contentType: 'application/gzip',
          headers: { 'content-disposition': 'attachment; filename="crowdsec-config.tar.gz"' },
          body: Buffer.from('e2e-archive'),
        });
      });

      await loadPage(page);
      await expect(progressRow(page)).toBeVisible();

      await test.step('Verify the files list and status still render', async () => {
        await expect(page.getByRole('heading', { name: /crowdsec/i }).first()).toBeVisible();
        await expect(page.getByRole('alert')).toHaveCount(0);
      });

      await test.step('Verify export is not blocked by the busy flag', async () => {
        const download = page.waitForEvent('download');
        await page.getByRole('button', { name: /export/i }).click();
        await download;
        expect(exportRequests).toBe(1);
      });
    });
  });
});
