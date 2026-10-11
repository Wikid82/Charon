/**
 * CrowdSec hub item restore E2E tests (#1545)
 *
 * Installing items cannot be tested end to end (hub reachability of the E2E container is
 * environment-dependent). These cases cover what is deterministic:
 * - Restore endpoint validation, answered by the real backend before anything runs
 * - The import-result panel, driven by `page.route` mocks of the import and restore endpoints
 *
 * The restore-endpoint cases run against the real backend. The import-result panel cases are
 * still `test.fixme` until commit 2.6 implements the panel.
 */

import { promises as fs } from 'fs';
import type { APIRequestContext, Page, Route, TestInfo } from '@playwright/test';
import { request as playwrightRequest } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { createTarGz } from '../utils/archive-helpers';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const ADMIN = '/api/v1/admin/crowdsec';
const RESTORE = `${ADMIN}/hub-items/restore`;
const MAX_ITEMS = 1000;
const INVALID_ITEM_MESSAGE = 'invalid hub item';
const NO_ITEMS_MESSAGE = 'no hub items given';
const INVALID_REQUEST_MESSAGE = 'invalid request';
const TOO_LARGE_MESSAGE = 'request too large';
const ALREADY_RUNNING_MESSAGE = 'a restore is already running';
const HUB_UNREACHABLE_MESSAGE = 'the CrowdSec hub could not be reached; nothing was installed';

/** Anchored matchers: a loose glob would also match sibling endpoints. */
const IMPORT_ROUTE = /\/api\/v1\/admin\/crowdsec\/import(?:\?.*)?$/;
const RESTORE_ROUTE = /\/api\/v1\/admin\/crowdsec\/hub-items\/restore(?:\?.*)?$/;

interface HubItem {
  type: string;
  name: string;
}

async function restore(request: APIRequestContext, data: unknown) {
  return request.post(RESTORE, { data });
}

test.describe('CrowdSec Hub Item Restore @security', () => {
  test.describe('Restore Validation', () => {
    const INVALID_NAMES: Array<{ title: string; name: string }> = [
      { title: 'a flag-like name', name: '--help' },
      { title: 'a leading dash', name: '-x/y' },
      { title: 'a path traversal', name: '../x' },
      { title: 'an embedded path traversal', name: 'a/../b' },
      { title: 'shell metacharacters', name: 'a;b/c' },
      { title: 'a name with a space', name: 'a/b c' },
      { title: 'a trailing newline', name: 'crowdsecurity/sshd\n' },
      { title: 'a second slash', name: 'a/b/c' },
      { title: 'an empty name', name: '' },
      { title: 'an empty owner', name: '/sshd' },
      { title: 'a non-ASCII name', name: 'crowdsecurity/sshé' },
      { title: 'an over-long owner', name: `${'a'.repeat(65)}/sshd` },
      { title: 'an over-long item name', name: `crowdsecurity/${'a'.repeat(129)}` },
    ];
    for (const { title, name } of INVALID_NAMES) {
      test(`rejects ${title} with 400 and runs nothing`, async ({ request }) => {
        const response = await restore(request, { items: [{ type: 'collections', name }] });

        expect(response.status()).toBe(400);
        expect((await response.json()).error).toBe(INVALID_ITEM_MESSAGE);
      });
    }

    test('rejects an unknown item type with 400', async ({ request }) => {
      const response = await restore(request, { items: [{ type: 'bouncers', name: 'crowdsecurity/sshd' }] });

      expect(response.status()).toBe(400);
      expect((await response.json()).error).toBe(INVALID_ITEM_MESSAGE);
    });

    test('rejects one invalid item among valid ones and installs none of them', async ({ request }) => {
      const response = await restore(request, {
        items: [
          { type: 'collections', name: 'crowdsecurity/sshd' },
          { type: 'collections', name: '--help' },
        ],
      });

      expect(response.status()).toBe(400);
      expect((await response.json()).error).toBe(INVALID_ITEM_MESSAGE);
    });

    test('rejects more than 1,000 items with 400', async ({ request }) => {
      const items = Array.from({ length: MAX_ITEMS + 1 }, (_, i) => ({ type: 'collections', name: `owner/item-${i}` }));

      const response = await restore(request, { items });

      expect(response.status()).toBe(400);
      expect((await response.json()).error).toBe(INVALID_ITEM_MESSAGE);
    });

    test('rejects an empty item list with 400', async ({ request }) => {
      const response = await restore(request, { items: [] });

      expect(response.status()).toBe(400);
      expect((await response.json()).error).toBe(NO_ITEMS_MESSAGE);
    });

    test('rejects malformed JSON with 400', async ({ request }) => {
      const response = await request.post(RESTORE, {
        headers: { 'Content-Type': 'application/json' },
        data: '{"items": [',
      });

      expect(response.status()).toBe(400);
      expect((await response.json()).error).toBe(INVALID_REQUEST_MESSAGE);
    });

    test('rejects a body over 1 MiB with 413', async ({ request }) => {
      const response = await request.post(RESTORE, {
        headers: { 'Content-Type': 'application/json' },
        data: JSON.stringify({ items: [], padding: 'x'.repeat(1024 * 1024 + 1) }),
      });

      expect(response.status()).toBe(413);
      expect((await response.json()).error).toBe(TOO_LARGE_MESSAGE);
    });

    // Valid requests, the 200 success shape and the 409 "already running" answer are covered by the
    // Go handler tests with a fake executor, not here: whether the E2E container can reach the
    // CrowdSec hub depends on its environment, and a successful restore really installs items,
    // which would change the installed-item set the import manifest specs depend on.

    test('requires authentication', async ({ baseURL }) => {
      // An explicit empty storage state: a bare newContext() inherits the project's signed-in one.
      const anonymous = await playwrightRequest.newContext({
        baseURL,
        storageState: { cookies: [], origins: [] },
      });
      try {
        const response = await anonymous.post(RESTORE, {
          data: { items: [{ type: 'collections', name: 'crowdsecurity/sshd' }] },
        });

        expect(response.status()).toBe(401);
      } finally {
        await anonymous.dispose();
      }
    });
  });

  test.describe('Import Result Panel', () => {
    const MISSING: HubItem[] = [
      { type: 'collections', name: 'Azlaroc/sftpgo' },
      { type: 'parsers', name: 'crowdsecurity/sshd-logs' },
      { type: 'scenarios', name: 'crowdsecurity/ssh-bf' },
    ];

    test.beforeEach(async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await waitForLoadingComplete(page);
    });

    /** Mocks the import answer, then performs an import through the page. */
    async function importArchive(
      page: Page,
      testInfo: TestInfo,
      hubItems: { in_export: number; skipped_invalid: number; missing: HubItem[] } | null,
    ) {
      await page.route(IMPORT_ROUTE, (route) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          json: { status: 'imported', backup: 'backup-1', ...(hubItems ? { hub_items: hubItems } : {}) },
        }),
      );
      await page.goto('/security/crowdsec');
      await waitForLoadingComplete(page);

      const archive = testInfo.outputPath('import.tar.gz');
      await createTarGz({ 'config.yaml': 'api:\n  server:\n    listen_uri: 0.0.0.0:8080\n' }, archive);
      await page.getByTestId('import-file').setInputFiles({
        name: 'import.tar.gz',
        mimeType: 'application/gzip',
        buffer: await fs.readFile(archive),
      });
      await page.getByRole('button', { name: /^import$/i }).click();
    }

    /** Mocks the restore endpoint and records every request body. */
    async function mockRestore(page: Page, respond: (route: Route, call: number) => Promise<void>) {
      const bodies: Array<{ items: HubItem[] }> = [];
      await page.route(RESTORE_ROUTE, async (route) => {
        bodies.push(route.request().postDataJSON() as { items: HubItem[] });
        await respond(route, bodies.length);
      });
      return bodies;
    }

    test.fixme('commit 2.6: offers to install the missing items after an import that reports them', async ({ page }, testInfo) => {
      await importArchive(page, testInfo, { in_export: 5, skipped_invalid: 0, missing: MISSING });

      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toBeEnabled();
      for (const item of MISSING) {
        await expect(page.getByText(item.name, { exact: true })).toBeAttached();
      }
    });

    test.fixme('commit 2.6: shows no panel when the import reports no hub items', async ({ page }, testInfo) => {
      await importArchive(page, testInfo, null);

      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toHaveCount(0);
    });

    test.fixme('commit 2.6: shows no panel when nothing is missing', async ({ page }, testInfo) => {
      await importArchive(page, testInfo, { in_export: 2, skipped_invalid: 0, missing: [] });

      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toHaveCount(0);
    });

    test.fixme('commit 2.6: sends the missing items and reports a full success', async ({ page }, testInfo) => {
      const bodies = await mockRestore(page, (route) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          json: { installed: MISSING, already_installed: [], failed: [], remaining: 0, skipped_invalid: 0, reloaded: true },
        }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });

      await page.getByRole('button', { name: 'Install missing hub items' }).click();

      await expect.poll(() => bodies.length).toBe(1);
      expect(bodies[0]).toEqual({ items: MISSING });
      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toHaveCount(0);
      await expect(page.getByRole('button', { name: 'Install remaining' })).toHaveCount(0);
    });

    test.fixme('commit 2.6: disables the button while the install runs', async ({ page }, testInfo) => {
      let release: () => void = () => undefined;
      const gate = new Promise<void>((resolve) => {
        release = resolve;
      });
      await mockRestore(page, async (route) => {
        await gate;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          json: { installed: MISSING, already_installed: [], failed: [], remaining: 0, skipped_invalid: 0, reloaded: false },
        });
      });
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });

      await page.getByRole('button', { name: 'Install missing hub items' }).click();

      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toBeDisabled();
      release();
      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toHaveCount(0);
    });

    test.fixme('commit 2.6: resends only the not_attempted entries from "Install remaining"', async ({ page }, testInfo) => {
      const failed = [
        { ...MISSING[1], reason: 'install_failed' },
        { ...MISSING[2], reason: 'not_attempted' },
      ];
      const bodies = await mockRestore(page, (route, call) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          json:
            call === 1
              ? { installed: [MISSING[0]], already_installed: [], failed, remaining: 1, skipped_invalid: 0, reloaded: true }
              : { installed: [MISSING[2]], already_installed: [], failed: [], remaining: 0, skipped_invalid: 0, reloaded: true },
        }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });

      await page.getByRole('button', { name: 'Install missing hub items' }).click();
      await expect(page.getByRole('button', { name: 'Install remaining' })).toBeEnabled();
      await expect(page.getByText(MISSING[1].name, { exact: true })).toBeVisible();

      await page.getByRole('button', { name: 'Install remaining' }).click();

      await expect.poll(() => bodies.length).toBe(2);
      expect(bodies[0]).toEqual({ items: MISSING });
      expect(bodies[1]).toEqual({ items: [{ type: MISSING[2].type, name: MISSING[2].name }] });
      await expect(page.getByRole('button', { name: 'Install remaining' })).toHaveCount(0);
    });

    test.fixme('commit 2.6: shows a neutral note when some manifest entries could not be used', async ({ page }, testInfo) => {
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 2, missing: MISSING });

      await expect(page.getByText(/2 entries in the export could not be used/i)).toBeVisible();
    });

    test.fixme('commit 2.6: keeps the page usable and the button available when the hub is unreachable (502)', async ({ page }, testInfo) => {
      const bodies = await mockRestore(page, (route) =>
        route.fulfill({ status: 502, contentType: 'application/json', json: { error: HUB_UNREACHABLE_MESSAGE } }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });

      await page.getByRole('button', { name: 'Install missing hub items' }).click();

      await expect.poll(() => bodies.length).toBe(1);
      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toBeEnabled();
      await expect(page.getByRole('button', { name: 'Export', exact: true })).toBeEnabled();
    });

    test.fixme('commit 2.6: keeps the button available when a restore is already running (409)', async ({ page }, testInfo) => {
      const bodies = await mockRestore(page, (route) =>
        route.fulfill({ status: 409, contentType: 'application/json', json: { error: ALREADY_RUNNING_MESSAGE } }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });

      await page.getByRole('button', { name: 'Install missing hub items' }).click();

      await expect.poll(() => bodies.length).toBe(1);
      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toBeEnabled();
    });

    test.fixme('commit 2.6: clears the panel when another file is selected', async ({ page }, testInfo) => {
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });
      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toBeVisible();

      await page.getByTestId('import-file').setInputFiles({
        name: 'other.tar.gz',
        mimeType: 'application/gzip',
        buffer: Buffer.from('other'),
      });

      await expect(page.getByRole('button', { name: 'Install missing hub items' })).toHaveCount(0);
    });
  });
});
