/**
 * Backups Page - Upload & Restore E2E Tests (Issue #32)
 *
 * Verifies `UploadBackupButton.tsx` + `POST /api/v1/backups/upload` (spec
 * §3.3.2) and the v0/v1/v2 format detection described in
 * docs/plans/current_spec.md §3.2 (Backup Archive Format v2 — backward
 * compatibility table), plus the extracted `RestoreDialog.tsx`.
 *
 * Written as `test.fixme` in Commit 1, then flipped to live tests in
 * Commit 5 once Commit 3 (backend) and Commit 4 (frontend) landed.
 *
 * See docs/plans/current_spec.md §6 Commit 1 / Commit 3 / Commit 4 / Commit 5.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { setupBackupsList, BackupFile } from '../utils/phase5-helpers';
import { waitForToast, waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('Backups Page - Upload & Restore', () => {
  test.describe('File picker', () => {
    test('should accept .zip, .age, and .db files', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupBackupsList(page, []);

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      const fileInput = page.getByTestId('backup-upload-input');
      await expect(fileInput).toHaveAttribute('accept', '.zip,.age,.db');
    });
  });

  test.describe('Upload validation feedback', () => {
    test(
      'should show format_version and no warnings for a valid v2 archive',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, []);

        await page.route('**/api/v1/backups/upload', async (route) => {
          await route.fulfill({
            status: 201,
            json: {
              filename: 'uploaded_2026-07-08_10-12-00.zip',
              uuid: 'u1',
              legacy_format: false,
              message: 'Backup uploaded and validated',
            },
          });
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const fileInput = page.getByTestId('backup-upload-input');
        await fileInput.setInputFiles({
          name: 'my-backup.zip',
          mimeType: 'application/zip',
          buffer: Buffer.from('mock v2 archive content'),
        });

        await expect(page.getByTestId('backup-upload-feedback')).toContainText(/format version 2/i);
        await expect(page.getByTestId('backup-upload-feedback')).not.toContainText(/legacy/i);
      }
    );

    test(
      'should flag a legacy_format v1 archive with a warning banner',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, []);

        await page.route('**/api/v1/backups/upload', async (route) => {
          await route.fulfill({
            status: 201,
            json: {
              filename: 'uploaded_2026-07-08_10-13-00.zip',
              uuid: 'u2',
              legacy_format: true,
              message: 'Backup uploaded and validated',
            },
          });
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const fileInput = page.getByTestId('backup-upload-input');
        await fileInput.setInputFiles({
          name: 'old-backup.zip',
          mimeType: 'application/zip',
          buffer: Buffer.from('mock v1 archive, no manifest.json'),
        });

        await expect(page.getByTestId('backup-upload-feedback')).toContainText(/legacy format/i);
      }
    );

    test(
      'should surface an encryption_key_required warning when the uploaded archive needs CHARON_ENCRYPTION_KEY',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, []);

        await page.route('**/api/v1/backups/upload', async (route) => {
          await route.fulfill({
            status: 201,
            json: {
              filename: 'uploaded_2026-07-08_10-14-00.zip',
              uuid: 'u3',
              legacy_format: false,
              encryption_key_required: true,
              message: 'Backup uploaded and validated',
            },
          });
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const fileInput = page.getByTestId('backup-upload-input');
        await fileInput.setInputFiles({
          name: 'has-secrets.zip',
          mimeType: 'application/zip',
          buffer: Buffer.from('mock archive containing encrypted DB rows'),
        });

        await expect(page.getByTestId('backup-upload-feedback')).toContainText(
          /encryption key|CHARON_ENCRYPTION_KEY/i
        );
      }
    );

    test(
      'should reject an upload rejected by the server with a clear error toast',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, []);

        await page.route('**/api/v1/backups/upload', async (route) => {
          await route.fulfill({
            status: 400,
            json: { error: 'not a recognized backup format', error_code: 'backup_validation_failed' },
          });
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const fileInput = page.getByTestId('backup-upload-input');
        await fileInput.setInputFiles({
          name: 'garbage.zip',
          mimeType: 'application/zip',
          buffer: Buffer.from('not a zip at all'),
        });

        await waitForToast(page, /not a recognized backup format|invalid/i, { type: 'error' });
      }
    );
  });

  test.describe('Uploaded backup appears in list', () => {
    test(
      'should show the uploaded backup with type "uploaded" in the backup table',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);

        const existing: BackupFile[] = [
          {
            filename: 'backup_2026-07-07_03-00-00.zip',
            size: 1048576,
            time: '2026-07-07T03:00:00Z',
            type: 'scheduled',
          },
        ];
        await setupBackupsList(page, existing);

        const uploaded: BackupFile = {
          filename: 'uploaded_2026-07-08_10-12-00.zip',
          size: 2048,
          time: '2026-07-08T10:12:00Z',
          uuid: 'u1',
          type: 'uploaded',
          encrypted: false,
          format_version: 2,
          status: 'completed',
        };

        await page.route('**/api/v1/backups/upload', async (route) => {
          await route.fulfill({
            status: 201,
            json: { filename: uploaded.filename, uuid: uploaded.uuid, legacy_format: false, message: 'ok' },
          });
        });

        // After upload, the list refetch should include the new uploaded row.
        let listRequestCount = 0;
        await page.route('**/api/v1/backups', async (route) => {
          if (route.request().method() === 'GET') {
            listRequestCount += 1;
            const backups = listRequestCount > 1 ? [uploaded, ...existing] : existing;
            await route.fulfill({ status: 200, json: backups });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const fileInput = page.getByTestId('backup-upload-input');
        await fileInput.setInputFiles({
          name: 'my-upload.zip',
          mimeType: 'application/zip',
          buffer: Buffer.from('mock archive content'),
        });

        const uploadedRow = page.getByTestId('backup-row').filter({ hasText: uploaded.filename });
        await expect(uploadedRow).toBeVisible({ timeout: 5000 });
        await expect(uploadedRow.getByTestId('backup-type-badge')).toContainText(/uploaded/i);
      }
    );
  });

  test.describe('Restore flow after upload', () => {
    test(
      'should open the passphrase-aware RestoreDialog and show validate results before confirming restore',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);

        const uploadedEncrypted: BackupFile = {
          filename: 'uploaded_2026-07-08_10-15-00.zip.age',
          size: 4096,
          time: '2026-07-08T10:15:00Z',
          uuid: 'u4',
          type: 'uploaded',
          encrypted: true,
          format_version: 2,
          status: 'completed',
        };
        await setupBackupsList(page, [uploadedEncrypted]);

        await page.route(`**/api/v1/backups/${uploadedEncrypted.filename}/validate`, async (route) => {
          await route.fulfill({
            status: 200,
            json: {
              valid: true,
              format_version: 2,
              legacy_format: false,
              app_version: '1.42.0',
              created_at: '2026-07-08T10:15:00Z',
              database_integrity: 'ok',
              encryption_key_required: false,
              warnings: [],
            },
          });
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const row = page.getByTestId('backup-row').filter({ hasText: uploadedEncrypted.filename });
        await row.getByTestId('backup-restore-btn').click();

        const dialog = page.getByRole('dialog');
        await expect(dialog).toBeVisible();

        // Filename ends in .age -> passphrase input must be shown.
        await expect(dialog.getByTestId('backup-restore-passphrase-input')).toBeVisible();

        await dialog.getByTestId('backup-restore-passphrase-input').fill('correct-horse-battery-staple');
        await dialog.getByRole('button', { name: /validate/i }).click();

        await expect(dialog.getByTestId('backup-restore-validate-results')).toContainText(/database integrity: ok/i);

        await page.route(`**/api/v1/backups/${uploadedEncrypted.filename}/restore`, async (route) => {
          await route.fulfill({
            status: 200,
            json: {
              message: 'Backup restored successfully',
              restart_required: false,
              database_swap_pending: false,
              live_rehydrate_applied: true,
              caddy_reloaded: true,
              pre_restore_backup: 'backup_2026-07-08_11-00-00.zip',
              legacy_format: false,
            },
          });
        });

        await dialog.getByRole('button', { name: /^restore$/i }).click();
        await waitForToast(page, /restored successfully/i, { type: 'success' });
      }
    );
  });
});
