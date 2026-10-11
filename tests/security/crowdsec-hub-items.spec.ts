/**
 * CrowdSec hub item restore E2E tests (#1545)
 *
 * Installing items cannot be tested end to end (hub reachability of the E2E container is
 * environment-dependent). These cases cover what is deterministic:
 * - Restore endpoint validation, answered by the real backend before anything runs
 * - The import-result panel, driven by `page.route` mocks of the import and restore endpoints
 *
 * The restore-endpoint cases run against the real backend. The import-result panel cases mock
 * both endpoints, so they never reach the real hub or install anything.
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

    const INSTALL_BUTTON = { name: 'Install missing hub items' };
    const ALL_INSTALLED = {
      installed: MISSING,
      already_installed: [],
      failed: [],
      remaining: 0,
      skipped_invalid: 0,
      reloaded: true,
    };
    const HUB_UNREACHABLE_TEXT =
      'The CrowdSec hub could not be reached. Nothing was installed. Check the internet connection and try again.';
    const ALREADY_RUNNING_TEXT = 'A restore is already running. Try again when it has finished.';
    const GENERIC_ERROR_TEXT = 'The hub items could not be installed. Try again.';

    test('offers to install the missing items after an import that reports them', async ({ page }, testInfo) => {
      await importArchive(page, testInfo, { in_export: 5, skipped_invalid: 0, missing: MISSING });

      const panel = page.getByTestId('hub-items-panel');
      await expect(panel).toBeVisible();
      await expect(panel.getByRole('heading', { name: 'Install missing hub items' })).toBeVisible();
      await expect(panel.getByRole('button', INSTALL_BUTTON)).toBeEnabled();
      for (const item of MISSING) {
        await expect(panel.getByText(item.name, { exact: true })).toBeVisible();
      }
      // Accessibility: the panel is a labelled region, idle (not busy), and the button is keyboard focusable.
      await expect(panel).toHaveAttribute('aria-busy', 'false');
      await expect(panel).toHaveAccessibleName('Install missing hub items');
      await expect(panel.getByRole('status')).toBeAttached();
      await panel.getByRole('button', INSTALL_BUTTON).focus();
      await expect(panel.getByRole('button', INSTALL_BUTTON)).toBeFocused();
    });

    test('shows no panel when the import reports no hub items', async ({ page }, testInfo) => {
      await importArchive(page, testInfo, null);

      await expect(page.getByTestId('import-file')).toBeAttached();
      await expect(page.getByTestId('hub-items-panel')).toHaveCount(0);
      await expect(page.getByRole('button', INSTALL_BUTTON)).toHaveCount(0);
    });

    test('shows no panel when nothing is missing', async ({ page }, testInfo) => {
      await importArchive(page, testInfo, { in_export: 2, skipped_invalid: 0, missing: [] });

      await expect(page.getByTestId('hub-items-panel')).toHaveCount(0);
      await expect(page.getByRole('button', INSTALL_BUTTON)).toHaveCount(0);
    });

    test('sends the missing items and reports a full success', async ({ page }, testInfo) => {
      const bodies = await mockRestore(page, (route) =>
        route.fulfill({ status: 200, contentType: 'application/json', json: ALL_INSTALLED }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });
      const panel = page.getByTestId('hub-items-panel');

      await panel.getByRole('button', INSTALL_BUTTON).click();

      await expect(panel.getByRole('status')).toHaveText('3 installed, 0 already installed, 0 failed.');
      expect(bodies).toHaveLength(1);
      expect(bodies[0]).toEqual({ items: MISSING });
      await expect(panel.getByRole('heading', { name: 'Installed', exact: true })).toBeVisible();
      await expect(panel.getByRole('heading', { name: 'Already installed', exact: true })).toHaveCount(0);
      await expect(panel.getByRole('heading', { name: 'Not installed', exact: true })).toHaveCount(0);
      for (const item of MISSING) {
        await expect(panel.getByText(item.name, { exact: true })).toHaveCount(1);
      }
      await expect(panel.getByRole('button', INSTALL_BUTTON)).toHaveCount(0);
      await expect(panel.getByRole('button', { name: 'Install remaining' })).toHaveCount(0);
      await expect(panel.getByText('items were not attempted before the time limit.', { exact: false })).toHaveCount(0);
    });

    test('disables the button and announces progress while the install runs', async ({ page }, testInfo) => {
      let release: () => void = () => undefined;
      const gate = new Promise<void>((resolve) => {
        release = resolve;
      });
      const bodies = await mockRestore(page, async (route) => {
        await gate;
        await route.fulfill({ status: 200, contentType: 'application/json', json: ALL_INSTALLED });
      });
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });
      const panel = page.getByTestId('hub-items-panel');

      await panel.getByRole('button', INSTALL_BUTTON).click();

      await expect(panel.getByRole('button', INSTALL_BUTTON)).toBeDisabled();
      await expect(panel).toHaveAttribute('aria-busy', 'true');
      await expect(panel.getByRole('status')).toHaveText('Installing hub items. This can take several minutes.');
      for (const item of MISSING) {
        await expect(panel.getByText(item.name, { exact: true })).toBeVisible();
      }
      release();
      await expect(panel.getByRole('status')).toHaveText('3 installed, 0 already installed, 0 failed.');
      await expect(panel).toHaveAttribute('aria-busy', 'false');
      await expect(panel.getByRole('button', INSTALL_BUTTON)).toHaveCount(0);
      expect(bodies).toHaveLength(1);
    });

    test('reports installed, already installed and failed items with a reason for each', async ({ page }, testInfo) => {
      const failed = [
        { ...MISSING[0], reason: 'not_found' },
        { ...MISSING[1], reason: 'install_failed' },
        { ...MISSING[2], reason: 'verify_failed' },
        { type: 'postoverflows', name: 'crowdsecurity/rdns', reason: 'not_attempted' },
      ];
      await mockRestore(page, (route) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          json: {
            installed: [{ type: 'parsers', name: 'crowdsecurity/nginx-logs' }],
            already_installed: [{ type: 'scenarios', name: 'crowdsecurity/http-probing' }],
            failed,
            remaining: 1,
            skipped_invalid: 0,
            reloaded: true,
          },
        }),
      );
      await importArchive(page, testInfo, { in_export: 7, skipped_invalid: 0, missing: MISSING });
      const panel = page.getByTestId('hub-items-panel');

      await panel.getByRole('button', INSTALL_BUTTON).click();

      await expect(panel.getByRole('status')).toContainText('1 installed, 1 already installed, 4 failed.');
      await expect(panel.getByRole('status')).toContainText('1 items were not attempted before the time limit.');
      await expect(panel.getByRole('heading', { name: 'Installed', exact: true })).toBeVisible();
      await expect(panel.getByRole('heading', { name: 'Already installed', exact: true })).toBeVisible();
      await expect(panel.getByRole('heading', { name: 'Not installed', exact: true })).toBeVisible();
      await expect(panel.getByText('crowdsecurity/nginx-logs', { exact: true })).toBeVisible();
      await expect(panel.getByText('crowdsecurity/http-probing', { exact: true })).toBeVisible();

      const expectedRows: Array<{ item: HubItem; reason: string }> = [
        { item: failed[0], reason: 'Not found in the hub' },
        { item: failed[1], reason: 'Installation failed' },
        { item: failed[2], reason: 'Installed but failed verification' },
        { item: failed[3], reason: 'Not attempted yet' },
      ];
      for (const { item, reason } of expectedRows) {
        const row = panel.getByRole('listitem').filter({ has: page.getByText(item.name, { exact: true }) });
        await expect(row).toHaveCount(1);
        await expect(row.getByText(item.type, { exact: true })).toBeVisible();
        await expect(row.getByText(reason, { exact: true })).toBeVisible();
      }
      await expect(panel.getByRole('button', INSTALL_BUTTON)).toHaveCount(0);
    });

    test('resends only the not_attempted entries from "Install remaining"', async ({ page }, testInfo) => {
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
      const panel = page.getByTestId('hub-items-panel');

      await panel.getByRole('button', INSTALL_BUTTON).click();
      await expect(panel.getByRole('status')).toContainText('1 installed, 0 already installed, 2 failed.');
      await expect(panel.getByRole('status')).toContainText('1 items were not attempted before the time limit.');
      await expect(panel.getByRole('button', { name: 'Install remaining' })).toBeEnabled();
      await expect(panel.getByText(MISSING[1].name, { exact: true })).toBeVisible();

      await panel.getByRole('button', { name: 'Install remaining' }).click();

      await expect(panel.getByRole('status')).toContainText('1 installed, 0 already installed, 0 failed.');
      expect(bodies).toHaveLength(2);
      expect(bodies[0]).toEqual({ items: MISSING });
      expect(bodies[1]).toEqual({ items: [{ type: MISSING[2].type, name: MISSING[2].name }] });
      await expect(panel.getByRole('button', { name: 'Install remaining' })).toHaveCount(0);
    });

    test('keeps the earlier result and the "Install remaining" button when the follow-up fails', async ({ page }, testInfo) => {
      const failed = [{ ...MISSING[2], reason: 'not_attempted' }];
      const bodies = await mockRestore(page, (route, call) =>
        call === 1
          ? route.fulfill({
              status: 200,
              contentType: 'application/json',
              json: { installed: [MISSING[0], MISSING[1]], already_installed: [], failed, remaining: 1, skipped_invalid: 0, reloaded: true },
            })
          : route.fulfill({ status: 502, contentType: 'application/json', json: { error: HUB_UNREACHABLE_MESSAGE } }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });
      const panel = page.getByTestId('hub-items-panel');
      await panel.getByRole('button', INSTALL_BUTTON).click();
      await expect(panel.getByRole('status')).toContainText('2 installed, 0 already installed, 1 failed.');

      await panel.getByRole('button', { name: 'Install remaining' }).click();

      await expect(panel.getByRole('status')).toContainText(HUB_UNREACHABLE_TEXT);
      expect(bodies).toHaveLength(2);
      await expect(panel.getByRole('status')).toContainText('2 installed, 0 already installed, 1 failed.');
      await expect(panel.getByText(MISSING[2].name, { exact: true })).toBeVisible();
      await expect(panel.getByRole('button', { name: 'Install remaining' })).toBeEnabled();
    });

    test('shows a plain note when some manifest entries could not be used', async ({ page }, testInfo) => {
      await mockRestore(page, (route) =>
        route.fulfill({ status: 200, contentType: 'application/json', json: { ...ALL_INSTALLED, skipped_invalid: 2 } }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 2, missing: MISSING });
      const panel = page.getByTestId('hub-items-panel');

      await expect(panel.getByText('2 entries in the export could not be used.', { exact: true })).toBeVisible();

      await panel.getByRole('button', INSTALL_BUTTON).click();

      await expect(panel.getByRole('status')).toContainText('3 installed, 0 already installed, 0 failed.');
      await expect(panel.getByText('2 entries in the export could not be used.', { exact: true })).toBeVisible();
    });

    test('keeps the page usable and the button available when the hub is unreachable (502)', async ({ page }, testInfo) => {
      const bodies = await mockRestore(page, (route) =>
        route.fulfill({ status: 502, contentType: 'application/json', json: { error: HUB_UNREACHABLE_MESSAGE } }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });
      const panel = page.getByTestId('hub-items-panel');

      await panel.getByRole('button', INSTALL_BUTTON).click();

      await expect(panel.getByRole('status')).toHaveText(HUB_UNREACHABLE_TEXT);
      expect(bodies).toHaveLength(1);
      await expect(panel.getByRole('button', INSTALL_BUTTON)).toBeEnabled();
      for (const item of MISSING) {
        await expect(panel.getByText(item.name, { exact: true })).toBeVisible();
      }
      await expect(page.getByRole('button', { name: 'Export', exact: true })).toBeEnabled();
    });

    test('keeps the button available when a restore is already running (409)', async ({ page }, testInfo) => {
      const bodies = await mockRestore(page, (route) =>
        route.fulfill({ status: 409, contentType: 'application/json', json: { error: ALREADY_RUNNING_MESSAGE } }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });
      const panel = page.getByTestId('hub-items-panel');

      await panel.getByRole('button', INSTALL_BUTTON).click();

      await expect(panel.getByRole('status')).toHaveText(ALREADY_RUNNING_TEXT);
      expect(bodies).toHaveLength(1);
      await expect(panel.getByRole('button', INSTALL_BUTTON)).toBeEnabled();
      for (const item of MISSING) {
        await expect(panel.getByText(item.name, { exact: true })).toBeVisible();
      }
    });

    test('shows a generic error and keeps the button available on any other failure', async ({ page }, testInfo) => {
      const bodies = await mockRestore(page, (route) =>
        route.fulfill({ status: 500, contentType: 'application/json', json: { error: 'internal error' } }),
      );
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });
      const panel = page.getByTestId('hub-items-panel');

      await panel.getByRole('button', INSTALL_BUTTON).click();

      await expect(panel.getByRole('status')).toHaveText(GENERIC_ERROR_TEXT);
      expect(bodies).toHaveLength(1);
      await expect(panel.getByRole('button', INSTALL_BUTTON)).toBeEnabled();
      for (const item of MISSING) {
        await expect(panel.getByText(item.name, { exact: true })).toBeVisible();
      }
    });

    test('clears the panel when another file is selected', async ({ page }, testInfo) => {
      await importArchive(page, testInfo, { in_export: 3, skipped_invalid: 0, missing: MISSING });
      await expect(page.getByTestId('hub-items-panel')).toBeVisible();

      await page.getByTestId('import-file').setInputFiles({
        name: 'other.tar.gz',
        mimeType: 'application/gzip',
        buffer: Buffer.from('other'),
      });

      await expect(page.getByTestId('hub-items-panel')).toHaveCount(0);
      await expect(page.getByRole('button', INSTALL_BUTTON)).toHaveCount(0);
    });
  });
});
