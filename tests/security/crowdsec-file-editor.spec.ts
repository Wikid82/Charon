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
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const FILES_ROUTE = '**/api/v1/admin/crowdsec/files';
// Anchored so it does not also match the sibling /crowdsec/files list endpoint.
const FILE_ROUTE = /\/api\/v1\/admin\/crowdsec\/file(\?.*)?$/;
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

  test('should post exactly one write for exactly the selected path on save', async ({ page }) => {
    const stub = await stubEditor(page, {
      status: 200,
      json: { status: 'written', backup: 'crowdsec.filebackup.20260101-000000.000000' },
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

  test('should show the server message when the write is rejected with 400 (invalid YAML)', async ({ page }) => {
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

  test('should show the server message when the write is rejected with 400 (disallowed file type)', async ({ page }) => {
    const message = 'file type is not allowed';
    await stubEditor(page, { status: 400, json: { error: message } });

    await editSelectedFile(page, 'anything\n');

    await test.step('Verify the server message is surfaced and no success toast is shown', async () => {
      await expect(page.getByText(message).first()).toBeVisible();
      await expect(page.getByText(/file saved/i)).toHaveCount(0);
    });
  });

  test('should show the server message when the write is rejected with 413 (too large)', async ({ page }) => {
    const message = 'file content exceeds the maximum allowed size';
    await stubEditor(page, { status: 413, json: { error: message } });

    await editSelectedFile(page, 'common:\n  log_level: debug\n');

    await test.step('Verify the server message is surfaced and no success toast is shown', async () => {
      await expect(page.getByText(message).first()).toBeVisible();
      await expect(page.getByText(/file saved/i)).toHaveCount(0);
    });
  });

  test.describe('credentials files and response paths (live API)', () => {
    const CREDENTIAL_FILES = ['local_api_credentials.yaml', 'online_api_credentials.yaml'];
    const ABSOLUTE_PATH = /(^|["'\s(])\/(app|data|etc|tmp|var|home)\//;

    test('should omit both credentials files from the file list', async ({ page }) => {
      const response = await page.request.get('/api/v1/admin/crowdsec/files');
      expect(response.ok()).toBe(true);
      const body = (await response.json()) as { files: string[] };
      for (const name of CREDENTIAL_FILES) {
        expect(body.files.map((f) => f.toLowerCase())).not.toContain(name);
      }
    });

    for (const name of CREDENTIAL_FILES) {
      test(`should reject reading ${name} with invalid path`, async ({ page }) => {
        const response = await page.request.get(`/api/v1/admin/crowdsec/file?path=${encodeURIComponent(name)}`);
        expect(response.status()).toBe(400);
        expect(await response.json()).toMatchObject({ error: 'invalid path' });
      });

      test(`should reject writing ${name} with file cannot be edited`, async ({ page }) => {
        const response = await page.request.post('/api/v1/admin/crowdsec/file', {
          data: { path: name, content: 'url: http://127.0.0.1:8080\n' },
        });
        expect(response.status()).toBe(400);
        expect(await response.json()).toMatchObject({ error: 'file cannot be edited' });
      });
    }

    test('should not leak an absolute path in rejection bodies', async ({ page }) => {
      const read = await page.request.get(`/api/v1/admin/crowdsec/file?path=${encodeURIComponent(CREDENTIAL_FILES[0])}`);
      const write = await page.request.post('/api/v1/admin/crowdsec/file', {
        data: { path: CREDENTIAL_FILES[1], content: 'x: y\n' },
      });
      expect(await read.text()).not.toMatch(ABSOLUTE_PATH);
      expect(await write.text()).not.toMatch(ABSOLUTE_PATH);
    });

    test('should return only a backup name, never an absolute path, on a successful write', async ({ page }) => {
      // Pick a real, editable config.yaml from the live listing rather than assuming its location,
      // and write it back unchanged so repeated runs leave the config untouched (no backup-count assumptions).
      const list = await page.request.get('/api/v1/admin/crowdsec/files');
      expect(list.ok()).toBe(true);
      const { files } = (await list.json()) as { files: string[] };
      const target = files.find((f) => f === SELECTED_PATH || f.endsWith(`/${SELECTED_PATH}`));
      expect(target, 'live file list should include config.yaml').toBeTruthy();

      const read = await page.request.get(`/api/v1/admin/crowdsec/file?path=${encodeURIComponent(target!)}`);
      expect(read.ok()).toBe(true);
      const { content } = (await read.json()) as { content: string };

      const write = await page.request.post('/api/v1/admin/crowdsec/file', {
        data: { path: target, content },
      });
      expect(write.ok()).toBe(true);
      const body = (await write.json()) as { backup?: string };
      expect(body.backup).toBeTruthy();
      expect(body.backup).not.toContain('/');
      expect(JSON.stringify(body)).not.toMatch(ABSOLUTE_PATH);
    });
  });
});
