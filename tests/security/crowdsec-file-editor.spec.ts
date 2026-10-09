/**
 * CrowdSec Configuration File Editor E2E Tests
 *
 * Verifies the generic file editor on the CrowdSec configuration page:
 * - A successful save posts exactly one write for exactly the selected path
 * - Server rejections (400 disallowed type / invalid YAML, 413 too large)
 *   surface the server message and never report success
 *
 * The file list, read, write and backup endpoints are stubbed so the tests are
 * deterministic and never touch the real CrowdSec tree.
 *
 * Marked test.fixme until the feature commits land; enabled in the final
 * E2E commit of docs/plans/current_spec.md.
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const FILES_ROUTE = '**/api/v1/admin/crowdsec/files';
const FILE_ROUTE = '**/api/v1/admin/crowdsec/file**';
const BACKUPS_ROUTE = '**/api/v1/backups';
const SELECTED_PATH = 'config.yaml';
const OTHER_PATH = 'parsers/s01-parse/e2e.yaml';

interface WriteBody {
  path: string;
  content: string;
}

/** Stubs list/read/backup endpoints and records every POST to the write endpoint. */
async function stubEditor(page: Page, writeResponse: { status: number; json: Record<string, unknown> }): Promise<{ writes: WriteBody[] }> {
  const writes: WriteBody[] = [];
  await page.route(FILES_ROUTE, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      json: { files: [SELECTED_PATH, OTHER_PATH] },
    });
  });
  await page.route(BACKUPS_ROUTE, async (route) => {
    if (route.request().method() !== 'POST') {
      await route.fallback();
      return;
    }
    await route.fulfill({ status: 202, contentType: 'application/json', json: { id: 'e2e-backup', status: 'queued' } });
  });
  await page.route(FILE_ROUTE, async (route) => {
    const request = route.request();
    if (request.method() === 'POST') {
      writes.push(request.postDataJSON() as WriteBody);
      await route.fulfill({ status: writeResponse.status, contentType: 'application/json', json: writeResponse.json });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      json: { content: 'common:\n  log_level: info\n' },
    });
  });
  return { writes };
}

async function editSelectedFile(page: Page, newContent: string): Promise<void> {
  await test.step('Open the configuration page', async () => {
    await page.goto('/security/crowdsec');
    await waitForLoadingComplete(page);
  });
  await test.step('Select the file and edit its content', async () => {
    await page.getByRole('combobox', { name: /select a file/i }).selectOption(SELECTED_PATH);
    const editor = page.getByRole('textbox', { name: /file content|content/i });
    await expect(editor).toHaveValue(/log_level: info/);
    await editor.fill(newContent);
  });
  await test.step('Click Save', async () => {
    await page.getByRole('button', { name: /^save$/i }).click();
  });
}

test.describe('CrowdSec File Editor @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.fixme('should post exactly one write for exactly the selected path on save', async ({ page }) => {
    const stub = await stubEditor(page, {
      status: 200,
      json: { status: 'written', backup: '/app/data/crowdsec.filebackup.20260101-000000.000000' },
    });
    const newContent = 'common:\n  log_level: debug\n';

    await editSelectedFile(page, newContent);

    await test.step('Verify success toast', async () => {
      await expect(page.getByText(/file saved/i)).toBeVisible();
    });
    await test.step('Verify exactly one write for exactly the selected path', async () => {
      expect(stub.writes).toHaveLength(1);
      expect(stub.writes[0].path).toBe(SELECTED_PATH);
      expect(stub.writes[0].content).toBe(newContent);
    });
  });

  test.fixme('should show the server message when the write is rejected with 400 (invalid YAML)', async ({ page }) => {
    const message = 'file content is not valid YAML';
    const stub = await stubEditor(page, { status: 400, json: { error: message } });

    await editSelectedFile(page, 'common: [unclosed\n');

    await test.step('Verify the server message is surfaced and no success toast is shown', async () => {
      await expect(page.getByText(message).first()).toBeVisible();
      await expect(page.getByText(/file saved/i)).toHaveCount(0);
    });
    await test.step('Verify a single write attempt for the selected path', async () => {
      expect(stub.writes).toHaveLength(1);
      expect(stub.writes[0].path).toBe(SELECTED_PATH);
    });
  });

  test.fixme('should show the server message when the write is rejected with 400 (disallowed file type)', async ({ page }) => {
    const message = 'file type is not allowed';
    await stubEditor(page, { status: 400, json: { error: message } });

    await editSelectedFile(page, 'anything\n');

    await test.step('Verify the server message is surfaced and no success toast is shown', async () => {
      await expect(page.getByText(message).first()).toBeVisible();
      await expect(page.getByText(/file saved/i)).toHaveCount(0);
    });
  });

  test.fixme('should show the server message when the write is rejected with 413 (too large)', async ({ page }) => {
    const message = 'file content exceeds the maximum allowed size';
    await stubEditor(page, { status: 413, json: { error: message } });

    await editSelectedFile(page, 'common:\n  log_level: debug\n');

    await test.step('Verify the server message is surfaced and no success toast is shown', async () => {
      await expect(page.getByText(message).first()).toBeVisible();
      await expect(page.getByText(/file saved/i)).toHaveCount(0);
    });
  });
});
