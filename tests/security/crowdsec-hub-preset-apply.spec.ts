/**
 * CrowdSec Hub Preset Apply E2E Tests
 *
 * Verifies that applying a non-curated (hub) CrowdSec preset reports the real
 * outcome and never falls back to a client-side file write:
 * - Hub installs run as background jobs (202 + polled job views); success is shown only for a succeeded job
 * - Errors (500/503/504) are surfaced and never shown as success
 * - HTTP 501 shows an error instead of writing the preview locally
 *
 * The preset list, pull and apply endpoints are stubbed so the tests are
 * deterministic and do not require the CrowdSec hub or cscli.
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const PRESETS_ROUTE = '**/api/v1/admin/crowdsec/presets';
const PULL_ROUTE = '**/api/v1/admin/crowdsec/presets/pull';
const APPLY_ROUTE = '**/api/v1/admin/crowdsec/presets/apply';
const JOB_CURRENT_ROUTE = /\/api\/v1\/admin\/crowdsec\/presets\/jobs\/current(?:\?.*)?$/;
const JOB_BY_ID_ROUTE = /\/api\/v1\/admin\/crowdsec\/presets\/jobs\/(?!current)[^/?]+(?:\?.*)?$/;
const JOB_ID = '7b1f6c1e-0c55-4a53-9d3c-5a0d3b6f2e10';
const FILE_WRITE_ROUTE = '**/api/v1/admin/crowdsec/file';
const HUB_PRESET_SLUG = 'crowdsecurity/e2e-hub-preset';
const HUB_PRESET_TITLE = 'E2E Hub Preset';
const CACHE_KEY = 'e2e-hub-cache-key';

/** Records POSTs to the generic file-write endpoint (the local fallback path). */
function trackFileWrites(page: Page): { count: () => number } {
  let writes = 0;
  void page.route(FILE_WRITE_ROUTE, async (route) => {
    if (route.request().method() === 'POST') {
      writes += 1;
    }
    await route.continue();
  });
  return { count: () => writes };
}

/** Stubs the preset list and pull endpoints so a single hub preset is available. */
async function stubHubPreset(page: Page): Promise<void> {
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
            cached: true,
            cache_key: CACHE_KEY,
          },
        ],
      },
    });
  });
  await page.route(PULL_ROUTE, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      json: {
        status: 'pulled',
        slug: HUB_PRESET_SLUG,
        preview: 'name: e2e-hub-preset\n',
        cache_key: CACHE_KEY,
        source: 'hub',
      },
    });
  });
}

type JobState = 'queued' | 'running' | 'verifying' | 'succeeded' | 'failed';

/** Builds a job status body shaped like `GET /admin/crowdsec/presets/jobs/:id`. */
function jobStatus(options: { state: JobState; phase: string; itemsInstalled?: number; elapsedSeconds?: number }): Record<string, unknown> {
  const terminal = options.state === 'succeeded' || options.state === 'failed';
  return {
    job_id: JOB_ID,
    slug: HUB_PRESET_SLUG,
    item_type: 'collections',
    cache_key: CACHE_KEY,
    state: options.state,
    phase: options.phase,
    error_code: '',
    error: '',
    warnings: [],
    items_installed: options.itemsInstalled ?? 0,
    items_total: null,
    cancel_requested: false,
    elapsed_seconds: options.elapsedSeconds ?? 0,
    created_at: '2026-10-18T10:00:00Z',
    started_at: '2026-10-18T10:00:00Z',
    finished_at: terminal ? '2026-10-18T10:01:00Z' : null,
  };
}

/** Stubs the async apply start with a 202 and records the request bodies. */
async function stubAsyncApply(page: Page): Promise<Array<Record<string, unknown>>> {
  const bodies: Array<Record<string, unknown>> = [];
  await page.route(APPLY_ROUTE, async (route) => {
    bodies.push(route.request().postDataJSON() as Record<string, unknown>);
    await route.fulfill({ status: 202, contentType: 'application/json', json: jobStatus({ state: 'queued', phase: 'preflight' }) });
  });
  return bodies;
}

/** Answers `GET jobs/current` with no active or recent job. */
async function stubNoCurrentJob(page: Page): Promise<void> {
  await page.route(JOB_CURRENT_ROUTE, async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', json: { job: null } });
  });
}

/** Replays `responses` for successive `GET jobs/:id` polls, repeating the last one. */
async function stubJobPolls(page: Page, responses: Array<Record<string, unknown>>): Promise<void> {
  let polls = 0;
  await page.route(JOB_BY_ID_ROUTE, async (route) => {
    const body = responses[Math.min(polls, responses.length - 1)];
    polls += 1;
    await route.fulfill({ status: 200, contentType: 'application/json', json: body });
  });
}

async function openPageAndApplyHubPreset(page: Page): Promise<void> {
  await test.step('Reload so the stubbed preset list is used', async () => {
    await page.goto('/security/crowdsec');
    await waitForLoadingComplete(page);
  });
  await test.step('Select the hub preset', async () => {
    await page.getByRole('button', { name: new RegExp(HUB_PRESET_TITLE, 'i') }).click();
  });
  await test.step('Click Apply Preset', async () => {
    await page.getByRole('button', { name: /apply preset/i }).click();
  });
}

test.describe('CrowdSec Hub Preset Apply @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await stubHubPreset(page);
  });

  test('should run the hub apply as a background job and report success when the job succeeds', async ({ page }) => {
    const fileWrites = trackFileWrites(page);
    const applyBodies = await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    await stubJobPolls(page, [
      jobStatus({ state: 'running', phase: 'install', itemsInstalled: 3, elapsedSeconds: 4 }),
      jobStatus({ state: 'succeeded', phase: 'reload', itemsInstalled: 5, elapsedSeconds: 8 }),
    ]);

    await openPageAndApplyHubPreset(page);

    await test.step('Verify the apply request opted in to a background job for the previewed entry', async () => {
      await expect.poll(() => applyBodies.length).toBe(1);
      expect(applyBodies[0]).toMatchObject({ slug: HUB_PRESET_SLUG, cache_key: CACHE_KEY, async: true });
    });
    await test.step('Verify the success result is shown once the job succeeds', async () => {
      // crowdsecConfig.presets.installProgress.succeeded + itemsInstalled
      await expect(page.getByTestId('preset-install-result')).toContainText(/preset applied\. 5 items installed/i);
    });
    await test.step('Verify no local file write happened', async () => {
      expect(fileWrites.count()).toBe(0);
    });
  });

  test('should surface a failed job and not report success', async ({ page }) => {
    const fileWrites = trackFileWrites(page);
    await stubAsyncApply(page);
    await stubNoCurrentJob(page);
    await stubJobPolls(page, [
      jobStatus({ state: 'failed', phase: 'rollback', itemsInstalled: 0, elapsedSeconds: 12 }),
    ]);

    await openPageAndApplyHubPreset(page);

    await test.step('Verify the failure and rollback note are shown and no success message appears', async () => {
      // installProgress.failed + failedGeneric (no known error_code) + rollbackNote
      const result = page.getByTestId('preset-install-result');
      await expect(result).toContainText(/install failed\. the preset could not be installed/i);
      await expect(result).toContainText(/previous configuration was restored/i);
      await expect(page.getByText(/preset applied/i)).toHaveCount(0);
    });
    await test.step('Verify no local fallback write', async () => {
      expect(fileWrites.count()).toBe(0);
    });
  });

  for (const status of [500, 503, 504]) {
    test(`should show an error and no success toast when hub apply returns ${status}`, async ({ page }) => {
      const fileWrites = trackFileWrites(page);
      const message = 'hub preset could not be applied';
      await page.route(APPLY_ROUTE, async (route) => {
        await route.fulfill({
          status,
          contentType: 'application/json',
          json: { error: message },
        });
      });

      await openPageAndApplyHubPreset(page);

      await test.step('Verify an error is surfaced', async () => {
        await expect(page.getByText(message).first()).toBeVisible();
      });
      await test.step('Verify no success toast is shown', async () => {
        await expect(page.getByText(/preset applied/i)).toHaveCount(0);
      });
      await test.step('Verify zero local fallback writes', async () => {
        expect(fileWrites.count()).toBe(0);
      });
    });
  }

  test('should show an error and perform no write when hub apply returns 501', async ({ page }) => {
    const fileWrites = trackFileWrites(page);
    await page.route(APPLY_ROUTE, async (route) => {
      await route.fulfill({
        status: 501,
        contentType: 'application/json',
        json: { error: 'preset apply not implemented' },
      });
    });

    await openPageAndApplyHubPreset(page);

    await test.step('Verify error shown and no success or local-apply toast', async () => {
      await expect(page.getByText(/preset apply not implemented/i).first()).toBeVisible();
      await expect(page.getByText(/preset applied/i)).toHaveCount(0);
      await expect(page.getByText(/applying locally/i)).toHaveCount(0);
    });
    await test.step('Verify zero local fallback writes', async () => {
      expect(fileWrites.count()).toBe(0);
    });
  });

  test('should send the pulled cache key when applying a hub preset', async ({ page }) => {
    let applyBody: Record<string, unknown> | undefined;
    await page.route(APPLY_ROUTE, async (route) => {
      applyBody = route.request().postDataJSON() as Record<string, unknown>;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        json: { status: 'applied', used_cscli: true, reload_hint: false, cache_key: CACHE_KEY },
      });
    });

    await openPageAndApplyHubPreset(page);

    await test.step('Verify the request names the previewed entry', async () => {
      await expect.poll(() => applyBody).toMatchObject({ slug: HUB_PRESET_SLUG, cache_key: CACHE_KEY });
    });
  });

  test('should load the cached preview from a path that keeps the owner and name segments', async ({ page }) => {
    const cacheRequests: string[] = [];
    await page.route(PULL_ROUTE, async (route) => {
      await route.fulfill({ status: 503, contentType: 'application/json', json: { error: 'hub unavailable' } });
    });
    await page.route('**/api/v1/admin/crowdsec/presets/cache/**', async (route) => {
      cacheRequests.push(new URL(route.request().url()).pathname);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        json: { preview: 'name: e2e-hub-preset\n', cache_key: CACHE_KEY, etag: 'etag', retrieved_at: new Date().toISOString() },
      });
    });
    await page.goto('/security/crowdsec');
    await waitForLoadingComplete(page);
    await page.getByRole('button', { name: new RegExp(HUB_PRESET_TITLE, 'i') }).click();

    await page.getByRole('button', { name: /use cached/i }).click();

    await expect(page.getByText(/loaded cached preview/i)).toBeVisible();
    expect(cacheRequests).toEqual([`/api/v1/admin/crowdsec/presets/cache/${HUB_PRESET_SLUG}`]);
  });

  test('should ask for a new pull and show no success toast when the cached entry is outdated', async ({ page }) => {
    await page.route(APPLY_ROUTE, async (route) => {
      await route.fulfill({
        status: 409,
        contentType: 'application/json',
        json: { error: 'preset cache is outdated', error_code: 'preset_cache_stale' },
      });
    });

    await openPageAndApplyHubPreset(page);

    await test.step('Verify the pull-again message is shown and nothing is reported as applied', async () => {
      await expect(page.getByText(/pull the preset again/i).first()).toBeVisible();
      await expect(page.getByText(/preset applied/i)).toHaveCount(0);
    });
  });
});
