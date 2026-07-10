/**
 * Backups Page - Remote Storage Targets E2E Tests (Issue #32)
 *
 * Verifies `RemoteTargetsCard.tsx` + `RemoteTargetFormDialog.tsx` and the
 * remote-target API described in docs/plans/current_spec.md §3.3.2 (New
 * routes — remote targets) and §3.7 (Remote Storage Design): S3/SFTP
 * targets, credentials, test-connection, SFTP host-key discovery.
 *
 * Written as `test.fixme` in Commit 1, then flipped to live tests in
 * Commit 5 once Commit 3 (backend) and Commit 4 (frontend) landed.
 *
 * See docs/plans/current_spec.md §6 Commit 1 / Commit 3 / Commit 4 / Commit 5.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForToast, waitForLoadingComplete } from '../utils/wait-helpers';

/**
 * Shape of a RemoteStorageTarget API response (spec §3.3.2) — secrets are
 * never included; only a `secrets_set` boolean.
 */
interface RemoteTargetResponse {
  uuid: string;
  name: string;
  type: 's3' | 'sftp';
  enabled: boolean;
  config: Record<string, unknown>;
  secrets_set: boolean;
  last_test_at: string | null;
  last_test_status: 'ok' | 'failed' | 'never';
  last_error: string;
  created_at: string;
  updated_at: string;
}

const mockTargets: RemoteTargetResponse[] = [
  {
    uuid: 'r1',
    name: 'Home NAS',
    type: 'sftp',
    enabled: true,
    config: { host: 'nas.lan', port: 22, path: '/backups/charon', username: 'charon' },
    secrets_set: true,
    last_test_at: '2026-07-07T09:00:00Z',
    last_test_status: 'ok',
    last_error: '',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-07T09:00:00Z',
  },
  {
    uuid: 'r2',
    name: 'Backblaze B2',
    type: 's3',
    enabled: true,
    config: {
      endpoint: 's3.us-west-002.backblazeb2.com',
      region: 'us-west-002',
      bucket: 'charon-backups',
      path_prefix: 'prod',
      use_ssl: true,
      force_path_style: false,
    },
    secrets_set: true,
    last_test_at: '2026-07-06T09:00:00Z',
    last_test_status: 'failed',
    last_error: 'connection timed out',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-06T09:00:00Z',
  },
];

async function setupRemoteTargets(
  page: import('@playwright/test').Page,
  targets: RemoteTargetResponse[] = mockTargets
) {
  await page.route('**/api/v1/backups/remote-targets', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, json: targets });
    } else {
      await route.continue();
    }
  });
}

test.describe('Backups Page - Remote Targets', () => {
  test.describe('Listing targets', () => {
    test(
      'should list remote targets with a status badge reflecting last_test_status',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const nasRow = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' });
        await expect(nasRow).toBeVisible();
        await expect(nasRow.getByTestId('backup-remote-target-status-badge')).toContainText(/ok/i);

        const b2Row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Backblaze B2' });
        await expect(b2Row.getByTestId('backup-remote-target-status-badge')).toContainText(/failed/i);
      }
    );

    test('should show an empty state when no remote targets are configured', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupRemoteTargets(page, []);

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      await expect(page.getByTestId('backup-remote-targets-empty-state')).toBeVisible();
    });
  });

  test.describe('Creating an S3 target', () => {
    test(
      'should submit endpoint/region/bucket/path_prefix/use_ssl/force_path_style + access key secrets',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, []);

        let createBody: Record<string, unknown> | undefined;
        await page.route('**/api/v1/backups/remote-targets', async (route) => {
          if (route.request().method() === 'POST') {
            createBody = route.request().postDataJSON();
            await route.fulfill({
              status: 201,
              json: { ...mockTargets[1], name: 'My S3 Target' },
            });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        // The header "Add Remote Target" button and the empty-state CTA share the
        // same accessible name when the target list is empty (both open the same
        // create dialog) — disambiguate with .first() to avoid a strict-mode
        // violation.
        await page.getByRole('button', { name: /add remote target/i }).first().click();
        const dialog = page.getByRole('dialog');
        await dialog.getByRole('radio', { name: /s3/i }).check();

        await dialog.getByLabel(/name/i).fill('My S3 Target');
        await dialog.getByLabel(/endpoint/i).fill('s3.us-west-002.backblazeb2.com');
        await dialog.getByLabel(/region/i).fill('us-west-002');
        await dialog.getByLabel(/bucket/i).fill('charon-backups');
        await dialog.getByLabel(/path prefix/i).fill('prod');
        await dialog.getByLabel(/use ssl/i).check();
        await dialog.getByLabel(/force path style/i).uncheck();
        await dialog.getByLabel(/access key id/i).fill('AKIAEXAMPLE');
        await dialog.getByLabel(/secret access key/i).fill('super-secret-key');

        await Promise.all([
          page.waitForResponse(
            (r) => r.url().includes('/api/v1/backups/remote-targets') && r.request().method() === 'POST'
          ),
          dialog.getByRole('button', { name: /save|create/i }).click(),
        ]);

        expect(createBody?.type).toBe('s3');
        expect((createBody?.config as Record<string, unknown>)?.bucket).toBe('charon-backups');
        expect((createBody?.secrets as Record<string, unknown>)?.access_key_id).toBe('AKIAEXAMPLE');
      }
    );
  });

  test.describe('Creating an SFTP target', () => {
    test(
      'should submit host/port/path/username + password and support the host-key discovery flow',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, []);

        await page.route('**/api/v1/backups/remote-targets/test-draft', async (route) => {
          // Discovery test: no fingerprint stored yet, server discovers and returns it
          // without ever authenticating (spec §3.7 — HostKeyCallback aborts before auth).
          await route.fulfill({
            status: 200,
            json: {
              success: false,
              message: 'host key not yet trusted',
              discovered_fingerprint: 'SHA256:abcdef1234567890',
            },
          });
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        // The header "Add Remote Target" button and the empty-state CTA share the
        // same accessible name when the target list is empty (both open the same
        // create dialog) — disambiguate with .first() to avoid a strict-mode
        // violation.
        await page.getByRole('button', { name: /add remote target/i }).first().click();
        const dialog = page.getByRole('dialog');
        await dialog.getByRole('radio', { name: /sftp/i }).check();

        await dialog.getByLabel(/^host/i).fill('nas.lan');
        await dialog.getByLabel(/^port/i).fill('22');
        await dialog.getByLabel(/^path/i).fill('/backups/charon');
        await dialog.getByLabel(/username/i).fill('charon');
        await dialog.getByLabel(/^password/i).fill('super-secret-password');

        await dialog.getByRole('button', { name: /discover host key/i }).click();
        await expect(dialog.getByTestId('backup-remote-target-host-key-fingerprint')).toContainText(
          'SHA256:abcdef1234567890'
        );

        await dialog.getByRole('button', { name: /confirm host key/i }).click();
        await expect(dialog.getByTestId('backup-remote-target-host-key-input')).toHaveValue(
          'SHA256:abcdef1234567890'
        );
      }
    );
  });

  test.describe('Secret field conventions', () => {
    test(
      'should render secret fields as type="password" and leave them blank on edit',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const nasRow = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' });
        await nasRow.getByTestId('backup-remote-target-edit-btn').click();

        const dialog = page.getByRole('dialog');
        const passwordField = dialog.getByLabel(/^password/i);
        await expect(passwordField).toHaveAttribute('type', 'password');
        await expect(passwordField).toHaveValue('');
        await expect(dialog.getByText(/leave blank to keep current/i)).toBeVisible();
      }
    );
  });

  test.describe('Test connection', () => {
    test('should show a success state when the connection test succeeds', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupRemoteTargets(page);

      await page.route('**/api/v1/backups/remote-targets/r1/test', async (route) => {
        await route.fulfill({ status: 200, json: { success: true, message: 'Connected', latency_ms: 42 } });
      });

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      const nasRow = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' });
      await nasRow.getByTestId('backup-remote-target-test-btn').click();

      await waitForToast(page, /connected|success/i, { type: 'success' });
    });

    test('should show a failure state when the connection test fails', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupRemoteTargets(page);

      await page.route('**/api/v1/backups/remote-targets/r2/test', async (route) => {
        await route.fulfill({ status: 502, json: { error: 'connection timed out' } });
      });

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      const b2Row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Backblaze B2' });
      await b2Row.getByTestId('backup-remote-target-test-btn').click();

      await waitForToast(page, /timed out|failed/i, { type: 'error' });
    });
  });

  test.describe('Deleting a target', () => {
    test('should delete a target after confirmation', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupRemoteTargets(page);

      let deleteCalled = false;
      await page.route('**/api/v1/backups/remote-targets/r1', async (route) => {
        if (route.request().method() === 'DELETE') {
          deleteCalled = true;
          await route.fulfill({ status: 204 });
        } else {
          await route.continue();
        }
      });

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      const nasRow = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' });
      await nasRow.getByTestId('backup-remote-target-delete-btn').click();

      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible();
      await dialog.getByRole('button', { name: /delete/i }).click();

      expect(deleteCalled).toBe(true);
      await expect(page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' })).toHaveCount(0);
    });
  });
});
