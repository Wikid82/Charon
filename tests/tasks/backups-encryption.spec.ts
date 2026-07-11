/**
 * Backups Page - Encryption E2E Tests (Issue #32)
 *
 * Verifies `BackupEncryptionCard.tsx` and the encryption surface of the
 * backup API described in docs/plans/current_spec.md §3.6 (Encryption
 * Design) and §3.8 (Frontend Design): age/scrypt passphrase encryption,
 * `.zip.age` archives, encrypted-lock badges, and the passphrase prompt in
 * `RestoreDialog.tsx`.
 *
 * Written as `test.fixme` in Commit 1, then flipped to live tests in
 * Commit 5 once Commit 3 (backend) and Commit 4 (frontend) landed.
 *
 * See docs/plans/current_spec.md §6 Commit 1 / Commit 3 / Commit 4 / Commit 5.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { setupBackupsList, BackupFile } from '../utils/phase5-helpers';
import { waitForToast, waitForLoadingComplete } from '../utils/wait-helpers';
import { clickSwitch } from '../utils/ui-helpers';

const mockBackups: BackupFile[] = [
  {
    filename: 'backup_2026-07-07_03-00-00.zip',
    size: 1048576,
    time: '2026-07-07T03:00:00Z',
    uuid: 'a1b2c3d4-0000-0000-0000-000000000001',
    type: 'scheduled',
    encrypted: false,
    format_version: 2,
    status: 'completed',
  },
  {
    filename: 'backup_2026-07-06_03-00-00.zip.age',
    size: 1148576,
    time: '2026-07-06T03:00:00Z',
    uuid: 'a1b2c3d4-0000-0000-0000-000000000002',
    type: 'scheduled',
    encrypted: true,
    format_version: 2,
    status: 'completed',
  },
];

test.describe('Backups Page - Encryption', () => {
  test.describe('Enabling encryption', () => {
    test(
      'should require a passphrase before encryption can be enabled',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, []);

        await page.route('**/api/v1/backups/settings', async (route) => {
          if (route.request().method() === 'GET') {
            await route.fulfill({
              status: 200,
              json: {
                schedule_enabled: true,
                schedule_cron: '0 3 * * *',
                retention_count: 7,
                remote_retention_count: 7,
                encryption_enabled: false,
                encryption_passphrase_set: false,
              },
            });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await clickSwitch(page.getByTestId('backup-encryption-toggle'));

        // Enabling without a passphrase should surface an inline requirement,
        // not silently enable encryption with no passphrase stored.
        await expect(page.getByTestId('backup-encryption-passphrase-input')).toBeVisible();
        await expect(page.getByRole('button', { name: /save/i })).toBeDisabled();
      }
    );

    test(
      'should never echo the passphrase back, only a "passphrase is set" indicator',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, []);

        await page.route('**/api/v1/backups/settings', async (route) => {
          if (route.request().method() === 'GET') {
            await route.fulfill({
              status: 200,
              json: {
                schedule_enabled: true,
                schedule_cron: '0 3 * * *',
                retention_count: 7,
                remote_retention_count: 7,
                encryption_enabled: true,
                encryption_passphrase_set: true,
              },
            });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await expect(page.getByTestId('backup-encryption-toggle')).toBeChecked();
        await expect(page.getByTestId('backup-encryption-passphrase-set-indicator')).toBeVisible();
        await expect(page.getByTestId('backup-encryption-passphrase-set-indicator')).toContainText(
          /passphrase is set/i
        );

        // The passphrase input must remain empty/write-only — never populated from the API.
        const passphraseInput = page.getByTestId('backup-encryption-passphrase-input');
        await expect(passphraseInput).toHaveValue('');
      }
    );

    test(
      'should display an explicit "cannot be recovered" warning',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, []);

        await page.route('**/api/v1/backups/settings', async (route) => {
          if (route.request().method() === 'GET') {
            await route.fulfill({
              status: 200,
              json: {
                schedule_enabled: true,
                schedule_cron: '0 3 * * *',
                retention_count: 7,
                remote_retention_count: 7,
                encryption_enabled: false,
                encryption_passphrase_set: false,
              },
            });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await clickSwitch(page.getByTestId('backup-encryption-toggle'));

        await expect(page.getByTestId('backup-encryption-warning')).toBeVisible();
        await expect(page.getByTestId('backup-encryption-warning')).toContainText(
          /cannot be recovered|lost passphrase/i
        );
      }
    );
  });

  test.describe('Encrypted backup creation', () => {
    test(
      'should produce a .zip.age file shown with an encrypted lock icon in the table',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, mockBackups);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const encryptedRow = page.getByTestId('backup-row').filter({
          hasText: 'backup_2026-07-06_03-00-00.zip.age',
        });
        await expect(encryptedRow).toBeVisible();
        await expect(encryptedRow.getByTestId('backup-encrypted-icon')).toBeVisible();

        const plainRow = page.getByTestId('backup-row').filter({
          hasText: 'backup_2026-07-07_03-00-00.zip',
        });
        await expect(plainRow.getByTestId('backup-encrypted-icon')).toHaveCount(0);
      }
    );

    test(
      'should create an encrypted backup on demand when a passphrase is supplied',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, []);

        let createBody: { encrypt?: boolean; passphrase?: string } | undefined;
        await page.route('**/api/v1/backups', async (route) => {
          if (route.request().method() === 'POST') {
            createBody = route.request().postDataJSON();
            await route.fulfill({
              status: 201,
              json: {
                filename: 'backup_2026-07-08_10-00-00.zip.age',
                uuid: 'a1b2c3d4-0000-0000-0000-000000000003',
                message: 'Backup created successfully',
              },
            });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        // The header "Create Backup" button and the empty-state CTA share the same
        // accessible name when the backup list is empty (both open the same create
        // dialog) — disambiguate with .first() to avoid a strict-mode violation,
        // matching the "Add Remote Target" fix in backups-remote-targets.spec.ts.
        await page.getByRole('button', { name: /create backup/i }).first().click();
        const dialog = page.getByRole('dialog');
        await expect(dialog).toBeVisible();
        await clickSwitch(dialog.getByTestId('backup-create-encrypt-toggle'));
        await dialog.getByTestId('backup-create-passphrase-input').fill('correct-horse-battery-staple');

        await Promise.all([
          page.waitForResponse(
            (r) => r.url().includes('/api/v1/backups') && r.request().method() === 'POST' && r.status() === 201
          ),
          dialog.getByRole('button', { name: /^create$/i }).click(),
        ]);

        expect(createBody?.encrypt).toBe(true);
        expect(createBody?.passphrase).toBe('correct-horse-battery-staple');
      }
    );
  });

  test.describe('Restoring an encrypted backup', () => {
    test(
      'should prompt for a passphrase when restoring a .zip.age backup',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, mockBackups);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const encryptedRow = page.getByTestId('backup-row').filter({
          hasText: 'backup_2026-07-06_03-00-00.zip.age',
        });
        await encryptedRow.getByTestId('backup-restore-btn').click();

        const dialog = page.getByRole('dialog');
        await expect(dialog).toBeVisible();
        await expect(dialog.getByTestId('backup-restore-passphrase-input')).toBeVisible();
      }
    );

    test(
      'should not prompt for a passphrase when restoring an unencrypted backup',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, mockBackups);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const plainRow = page.getByTestId('backup-row').filter({
          hasText: 'backup_2026-07-07_03-00-00.zip',
        });
        await plainRow.getByTestId('backup-restore-btn').click();

        const dialog = page.getByRole('dialog');
        await expect(dialog).toBeVisible();
        await expect(dialog.getByTestId('backup-restore-passphrase-input')).toHaveCount(0);
      }
    );

    test(
      'should show a clear error for a wrong passphrase without side effects',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupsList(page, mockBackups);

        const filename = 'backup_2026-07-06_03-00-00.zip.age';
        let restoreCalled = false;

        await page.route(`**/api/v1/backups/${filename}/restore`, async (route) => {
          restoreCalled = true;
          await route.fulfill({
            status: 400,
            json: {
              error: 'wrong passphrase',
              error_code: 'backup_passphrase_invalid',
            },
          });
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const encryptedRow = page.getByTestId('backup-row').filter({ hasText: filename });
        await encryptedRow.getByTestId('backup-restore-btn').click();

        const dialog = page.getByRole('dialog');
        await dialog.getByTestId('backup-restore-passphrase-input').fill('wrong-passphrase');
        await dialog.getByRole('button', { name: /restore/i }).click();

        await waitForToast(page, /wrong passphrase|invalid passphrase/i, { type: 'error' });

        expect(restoreCalled).toBe(true);
        // No live data should have been touched — dialog stays open so the user can retry,
        // and the backup list must be unaffected (no rows removed/altered).
        await expect(dialog).toBeVisible();
        await expect(page.getByTestId('backup-row')).toHaveCount(mockBackups.length);
      }
    );
  });
});
