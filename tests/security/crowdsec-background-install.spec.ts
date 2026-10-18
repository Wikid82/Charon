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
 * Time never passes in these specs: the elapsed text comes from the `elapsed_seconds` the mocked job
 * view reports. The poll delay is 2 s while a job is under 30 s old, so every sequence keeps its
 * `elapsed_seconds` small, and waits are auto-retrying assertions. The i18n key behind each copy
 * is noted beside it (crowdsecConfig.presets.installProgress.* and crowdsecConfig.busy).
 */

import type { Page, Route } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { crowdsecFixtures, stubCrowdSecApi } from '../utils/crowdsec-stubs';

const ADMIN = String.raw`/api/v1/admin/crowdsec`;
const END = String.raw`(?:\?.*)?$`;

const PRESETS_ROUTE = '**/api/v1/admin/crowdsec/presets';
const PULL_ROUTE = '**/api/v1/admin/crowdsec/presets/pull';
const APPLY_ROUTE = new RegExp(`${ADMIN}/presets/apply${END}`);
const JOB_CURRENT_ROUTE = new RegExp(`${ADMIN}/presets/jobs/current${END}`);
const JOB_BY_ID_ROUTE = new RegExp(`${ADMIN}/presets/jobs/(?!current)[^/?]+${END}`);
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
  warnings?: string[];
  cancelRequested?: boolean;
}

/** Builds a job status body shaped like `GET /admin/crowdsec/presets/jobs/:id` . */
function jobStatus(options: JobStatusOptions): Record<string, unknown> {
  const terminal = ['succeeded', 'failed', 'timed_out', 'cancelled'].includes(options.state);
  return {
    job_id: JOB_ID,
    slug: HUB_PRESET_SLUG,
    item_type: 'collections',
    cache_key: CACHE_KEY,
    state: options.state,
    phase: options.phase ?? 'install',
    error_code: options.errorCode ?? '',
    error: '',
    warnings: options.warnings ?? [],
    items_installed: options.itemsInstalled ?? 0,
    items_total: null,
    cancel_requested: options.cancelRequested ?? false,
    elapsed_seconds: options.elapsedSeconds ?? 0,
    created_at: STARTED_AT,
    started_at: STARTED_AT,
    finished_at: terminal ? '2026-10-18T10:06:00Z' : null,
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
          cached: true,
          cache_key: CACHE_KEY,
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

/** Answers `GET jobs/current` with `{ job: null }` (no active or recent job) unless a test overrides it. */
async function stubNoCurrentJob(page: Page): Promise<void> {
  await page.route(JOB_CURRENT_ROUTE, async (route) => {
    await fulfillJson(route, 200, { job: null });
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
    await fulfillJson(route, 202, jobStatus({ state: 'queued', phase: 'preflight' }));
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
  const apply = page.getByRole('button', { name: /apply preset/i });
  await expect(apply).toBeEnabled();
  await apply.click();
}

/** The inline progress row; i18n: crowdsecConfig.presets.installProgress.label */
function progressRow(page: Page) {
  return page.getByRole('status', { name: /installing preset/i });
}

/**
 * The running-state container (phase text, Cancel button). The live region keeps the same
 * accessible name after the job ends, so "running UI gone" is asserted on this container.
 */
function runningPanel(page: Page) {
  return page.getByTestId('preset-install-progress');
}

/** The terminal-state container holding the result text; same persistent live region inside. */
function resultPanel(page: Page) {
  return page.getByTestId('preset-install-result');
}

test.describe('CrowdSec background hub preset install @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await stubHubPreset(page);
  });

  test('should start the install asynchronously and show a passive progress row', async ({ page }) => {
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
      // phase: installProgress.phase.install
      await expect(row).toContainText(/installing/i);
      // item count: installProgress.itemsSoFar
      await expect(row).toContainText(/12 items so far/i);
      // elapsed time: installProgress.elapsed
      await expect(row).toContainText(/3 min 20 s/i);
      await expect(page.getByRole('button', { name: /cancel/i })).toBeVisible();
    });

    await test.step('Verify the progress UI stays passive', async () => {
      await expect(page.getByRole('alert')).toHaveCount(0);
      await expect(page.getByRole('progressbar')).toHaveCount(0);
    });
  });

  test('should show the running phases in order as polls advance', async ({ page }) => {
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
    await expect(row).toContainText(/saving a snapshot/i);
    await expect(row).toContainText(/installing/i);
    await expect(row).toContainText(/verifying/i);
    await expect(row).toContainText(/reloading crowdsec/i);
  });

  test('should poll until a terminal state, then stop polling and show the result once', async ({ page }) => {
    // Installed before any navigation so the elapsed polling timers can be fast-forwarded.
    await page.clock.install();
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    const jobPolls = await stubJobPolls(page, [
      jobStatus({ state: 'running', phase: 'install', itemsInstalled: 5, elapsedSeconds: 4 }),
      jobStatus({ state: 'succeeded', phase: 'reload', itemsInstalled: 41, elapsedSeconds: 360 }),
    ]);

    await loadPage(page);
    await startApply(page);

    await test.step('Verify the succeeded state replaces the progress row', async () => {
      // installProgress.succeeded + installProgress.itemsInstalled
      await expect(page.getByText(/preset applied\. 41 items installed/i)).toBeVisible();
      await expect(resultPanel(page).getByRole('status', { name: /installing preset/i })).toContainText(/preset applied\. 41 items installed/i);
      await expect(runningPanel(page)).toHaveCount(0);
      await expect(page.getByRole('button', { name: /apply preset/i })).toBeEnabled();
    });

    await test.step('Verify no further polls are issued after the terminal state', async () => {
      const pollsAtTerminal = jobPolls.polls();
      // Ten minutes of fake time would trigger many slow-cadence polls if the loop were still running.
      await page.clock.fastForward('10:00');
      expect(jobPolls.polls()).toBe(pollsAtTerminal);
      await expect(page.getByText(/preset applied\. 41 items installed/i)).toBeVisible();
    });
  });

  test('should resume the progress row after a page reload from the current job', async ({ page }) => {
    await stubAsyncApply(page);
    let reloaded = false;
    await page.route(JOB_CURRENT_ROUTE, async (route) => {
      // Before the reload there is no job; after it the job is active.
      await fulfillJson(
        route,
        200,
        !reloaded ? { job: null } : {
          job: jobStatus({ state: 'running', phase: 'install', itemsInstalled: 18, elapsedSeconds: 95 }),
        },
      );
    });
    await stubJobPolls(page, [
      jobStatus({ state: 'running', phase: 'install', itemsInstalled: 18, elapsedSeconds: 95 }),
    ]);

    await loadPage(page);
    await expect(progressRow(page)).toHaveCount(0);

    await test.step('Reload and verify the active job is resumed without restarting it', async () => {
      reloaded = true;
      await page.reload();
      await waitForLoadingComplete(page);
      const row = progressRow(page);
      await expect(row).toBeVisible();
      await expect(row).toContainText(/18 items so far/i);
      await expect(row).toContainText(/1 min 35 s/i);
      await expect(page.getByRole('button', { name: /cancel/i })).toBeVisible();
    });
  });

  test('should show a recently finished job once on load', async ({ page }) => {
    await page.route(JOB_CURRENT_ROUTE, async (route) => {
      await fulfillJson(route, 200, {
        job: jobStatus({ state: 'succeeded', phase: 'reload', itemsInstalled: 41, elapsedSeconds: 360 }),
      });
    });

    await loadPage(page);

    await expect(page.getByText(/preset applied\. 41 items installed/i)).toBeVisible();
    await expect(resultPanel(page)).toContainText(/preset applied\. 41 items installed/i);
    await expect(runningPanel(page)).toHaveCount(0);
    await expect(page.getByRole('button', { name: /cancel/i })).toHaveCount(0);
  });

  test('should cancel a running install and show the cancelled state with a rollback note', async ({ page }) => {
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    let cancelled = false;
    let cancelRequests = 0;
    await page.route(JOB_BY_ID_ROUTE, async (route) => {
      const method = route.request().method();
      if (method === 'DELETE') {
        cancelRequests += 1;
        cancelled = true;
        await fulfillJson(route, 202, jobStatus({ state: 'running', phase: 'install', itemsInstalled: 9, elapsedSeconds: 10, cancelRequested: true }));
        return;
      }
      await fulfillJson(
        route,
        200,
        cancelled
          ? jobStatus({ state: 'cancelled', phase: 'rollback', itemsInstalled: 0, elapsedSeconds: 60, errorCode: 'cancelled' })
          : jobStatus({ state: 'running', phase: 'install', itemsInstalled: 9, elapsedSeconds: 10 }),
      );
    });

    await loadPage(page);
    await startApply(page);
    await expect(progressRow(page)).toContainText(/installing/i);

    await test.step('Click Cancel and verify one DELETE is sent', async () => {
      await page.getByRole('button', { name: /^cancel$/i }).click();
      await expect.poll(() => cancelRequests).toBe(1);
    });

    await test.step('Verify the cancelled state explains the rollback', async () => {
      // crowdsecConfig.presets.installProgress.cancelled
      await expect(page.getByText(/install cancelled/i)).toBeVisible();
      await expect(page.getByText(/previous configuration was restored/i)).toBeVisible();
      await expect(resultPanel(page)).toContainText(/install cancelled/i);
      await expect(runningPanel(page)).toHaveCount(0);
      await expect(page.getByRole('button', { name: /^cancel$/i })).toHaveCount(0);
      await expect(page.getByRole('button', { name: /apply preset/i })).toBeEnabled();
    });
  });

  test('should show the failure text and rollback note when the install fails', async ({ page }) => {
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
      }),
    ]);

    await loadPage(page);
    await startApply(page);

    // installProgress.failed + the preset_local_modified outcome text
    await expect(page.getByText(/install failed\./i)).toBeVisible();
    await expect(page.getByText(/previous configuration was restored/i)).toBeVisible();
    await expect(page.getByText(/preset applied/i)).toHaveCount(0);
    await expect(resultPanel(page)).toContainText(/install failed\./i);
    await expect(runningPanel(page)).toHaveCount(0);
    await expect(page.getByRole('button', { name: /apply preset/i })).toBeEnabled();
  });

  test('should show the interrupted state when the server restarted mid-install', async ({ page }) => {
    await stubAsyncApply(page);
    await page.route(JOB_CURRENT_ROUTE, async (route) => {
      await fulfillJson(route, 200, {
        job: jobStatus({
          state: 'failed',
          phase: 'rollback',
          elapsedSeconds: 120,
          errorCode: 'interrupted',
        }),
      });
    });

    await loadPage(page);

    // crowdsecConfig.presets.installProgress.interrupted
    await expect(page.getByText(/install was interrupted by a restart/i)).toBeVisible();
    await expect(page.getByText(/previous configuration was restored/i)).toBeVisible();
    await expect(resultPanel(page)).toContainText(/install was interrupted by a restart/i);
    await expect(runningPanel(page)).toHaveCount(0);
  });

  test('should show the timed out state with a rollback note', async ({ page }) => {
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    await stubJobPolls(page, [
      jobStatus({
        state: 'timed_out',
        phase: 'rollback',
        elapsedSeconds: 1800,
        errorCode: 'preset_apply_timeout',
      }),
    ]);

    await loadPage(page);
    await startApply(page);

    // crowdsecConfig.presets.installProgress.timedOut
    await expect(page.getByText(/install took too long/i)).toBeVisible();
    await expect(page.getByText(/previous configuration was restored/i)).toBeVisible();
  });

  test('should show "waiting for the server" and recover when status polls fail', async ({ page }) => {
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

  test('should answer a second apply with the busy text and show the active job', async ({ page }) => {
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
    // The refusal names the running job, so its progress row is shown.
    await expect(progressRow(page)).toContainText(/4 items so far/i);
  });

  test.describe('while a job is active', () => {
    let recorder: Awaited<ReturnType<typeof stubCrowdSecApi>>;

    test.beforeEach(async ({ page }) => {
      recorder = await stubCrowdSecApi(page, {
        consoleEnrollmentEnabled: true,
        consoleStatus: crowdsecFixtures.notEnrolled(),
        status: crowdsecFixtures.runningStatus({ running: false, pid: 0, lapi_ready: false }),
        exportBody: Buffer.from('e2e-archive'),
      });
      await page.route(JOB_CURRENT_ROUTE, async (route) => {
        await fulfillJson(route, 200, {
          job: jobStatus({ state: 'running', phase: 'install', itemsInstalled: 11, elapsedSeconds: 70 }),
        });
      });
      await stubJobPolls(page, [jobStatus({ state: 'running', phase: 'install', itemsInstalled: 11, elapsedSeconds: 70 })]);
    });

    test('should show the busy text when starting CrowdSec during a job', async ({ page }) => {
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

      // The not-running banner appears only after the page's 3 second start-up grace period.
      await page.getByRole('button', { name: 'Start CrowdSec' }).click({ timeout: 15_000 });

      await test.step('Verify the translated busy message and no extra polling or banner', async () => {
        await expect(page.getByText(/crowdsec is busy installing a preset/i)).toBeVisible();
        expect(startRequests).toBe(1);
        // The progress row remains; the busy message is inline text, not a persistent banner.
        await expect(progressRow(page)).toBeVisible();
      });
    });

    test('should show the busy text when importing a configuration during a job', async ({ page }) => {
      await page.route(IMPORT_ROUTE, async (route) => {
        await fulfillJson(route, 409, {
          error: 'CrowdSec is busy',
          error_code: 'crowdsec_busy',
          job_id: JOB_ID,
        });
      });

      await loadPage(page);
      await expect(progressRow(page)).toBeVisible();

      await page.getByLabel('Select CrowdSec package file').setInputFiles({
        name: 'crowdsec-config.tar.gz',
        mimeType: 'application/gzip',
        buffer: Buffer.from('e2e-archive'),
      });
      await page.getByRole('button', { name: /^import$/i }).click();

      await expect(page.getByText(/crowdsec is busy installing a preset/i)).toBeVisible();
    });

    test('should keep read-only views usable during a job', async ({ page }) => {
      await loadPage(page);
      await expect(progressRow(page)).toBeVisible();

      await test.step('Verify the files list and status still render', async () => {
        await expect(page.getByRole('heading', { name: /crowdsec/i }).first()).toBeVisible();
        await expect(page.getByRole('alert')).toHaveCount(0);
      });

      await test.step('Verify export is not blocked by the busy flag', async () => {
        page.once('dialog', (dialog) => void dialog.accept('crowdsec-config.tar.gz'));
        const download = page.waitForEvent('download');
        await page.getByRole('button', { name: /^export$/i }).click();
        await download;
        expect(recorder.exportRequests).toBe(1);
      });
    });
  });
});
