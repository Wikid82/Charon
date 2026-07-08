/**
 * Backups Page - Schedule Settings E2E Tests (Issue #32 — target behavior, not yet implemented)
 *
 * Encodes the target behavior for `BackupScheduleCard.tsx` + the
 * `GET/PUT /api/v1/backups/settings` API described in
 * docs/plans/current_spec.md §3.3.1 (API Contracts) and §3.8 (Frontend Design).
 *
 * All tests in this file are `test.fixme` because the schedule settings UI/API
 * do not exist yet — they land in Commit 4 (frontend). This spec is written now
 * (Commit 1) so the target behavior is explicit before implementation, per the
 * CLAUDE.md Commit Slicing convention ("E2E specs for new behavior as
 * test.fixme until implemented"). Flip to live tests in Commit 5.
 *
 * See docs/plans/current_spec.md §6 Commit 1 / Commit 4 / Commit 5.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForToast, waitForLoadingComplete } from '../utils/wait-helpers';

/**
 * Shape of the GET/PUT /api/v1/backups/settings payload (spec §3.3.1).
 */
interface BackupSettings {
  schedule_enabled: boolean;
  schedule_cron: string;
  retention_count: number;
  remote_retention_count: number;
  encryption_enabled: boolean;
  encryption_passphrase_set: boolean;
}

const defaultSettings: BackupSettings = {
  schedule_enabled: true,
  schedule_cron: '0 3 * * *',
  retention_count: 7,
  remote_retention_count: 7,
  encryption_enabled: false,
  encryption_passphrase_set: false,
};

/**
 * Mocks GET /api/v1/backups/settings and captures PUT payloads for assertions.
 */
async function setupBackupSettings(
  page: import('@playwright/test').Page,
  overrides: Partial<BackupSettings> = {}
) {
  const settings: BackupSettings = { ...defaultSettings, ...overrides };
  const putCalls: Partial<BackupSettings>[] = [];

  await page.route('**/api/v1/backups/settings', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, json: settings });
    } else if (route.request().method() === 'PUT') {
      const body = route.request().postDataJSON() as Partial<BackupSettings>;
      putCalls.push(body);
      Object.assign(settings, body);
      await route.fulfill({ status: 200, json: settings });
    } else {
      await route.continue();
    }
  });

  return { settings, putCalls };
}

test.describe('Backups Page - Schedule Settings', () => {
  test.describe('Viewing current schedule', () => {
    test.fixme(
      'should display the current schedule enabled toggle, cron string, and retention counts',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupSettings(page, {
          schedule_enabled: true,
          schedule_cron: '0 3 * * *',
          retention_count: 7,
          remote_retention_count: 7,
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const scheduleCard = page.getByTestId('backup-schedule-card');
        await expect(scheduleCard).toBeVisible();

        await expect(page.getByTestId('backup-schedule-enabled-toggle')).toBeChecked();
        await expect(page.getByTestId('backup-schedule-cron-input')).toHaveValue('0 3 * * *');
        await expect(page.getByTestId('backup-schedule-retention-input')).toHaveValue('7');
        await expect(page.getByTestId('backup-schedule-remote-retention-input')).toHaveValue('7');
      }
    );

    test.fixme(
      'should reflect a disabled schedule by unchecking the enabled toggle',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupSettings(page, { schedule_enabled: false });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await expect(page.getByTestId('backup-schedule-enabled-toggle')).not.toBeChecked();
      }
    );
  });

  test.describe('Frequency presets', () => {
    test.fixme(
      'should switch to the Daily preset and populate a daily cron expression',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupSettings(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await page.getByRole('radio', { name: /daily/i }).check();
        await page.getByLabel(/time/i).fill('03:00');

        // Daily preset should populate a standard daily cron under the hood.
        await expect(page.getByTestId('backup-schedule-cron-input')).toHaveValue('0 3 * * *');
      }
    );

    test.fixme(
      'should switch to the Weekly preset and populate a weekly cron expression with day + time',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupSettings(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await page.getByRole('radio', { name: /weekly/i }).check();
        await page.getByLabel(/day of week/i).selectOption({ label: 'Sunday' });
        await page.getByLabel(/time/i).fill('03:00');

        await expect(page.getByTestId('backup-schedule-cron-input')).toHaveValue('0 3 * * 0');
      }
    );

    test.fixme(
      'should reveal a custom cron escape hatch and accept a raw cron expression',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupSettings(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await page.getByRole('radio', { name: /custom/i }).check();
        const cronInput = page.getByTestId('backup-schedule-cron-input');
        await expect(cronInput).toBeEditable();

        await cronInput.fill('0 */6 * * *');
        await expect(cronInput).toHaveValue('0 */6 * * *');
      }
    );
  });

  test.describe('Validation feedback', () => {
    test.fixme(
      'should show inline validation feedback for an invalid cron string before saving',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupSettings(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await page.getByRole('radio', { name: /custom/i }).check();
        const cronInput = page.getByTestId('backup-schedule-cron-input');
        await cronInput.fill('not a cron expression');
        await cronInput.blur();

        await expect(page.getByTestId('backup-schedule-cron-error')).toBeVisible();
        await expect(page.getByTestId('backup-schedule-cron-error')).toContainText(/invalid/i);

        // Save should be blocked while validation fails client-side.
        await expect(page.getByRole('button', { name: /save schedule/i })).toBeDisabled();
      }
    );

    test.fixme(
      'should surface a server-side backup_invalid_cron error if validation is bypassed',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        const { settings } = await setupBackupSettings(page);
        void settings;

        await page.route('**/api/v1/backups/settings', async (route) => {
          if (route.request().method() === 'PUT') {
            await route.fulfill({
              status: 400,
              json: { error: 'invalid cron expression', error_code: 'backup_invalid_cron' },
            });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await page.getByRole('button', { name: /save schedule/i }).click();
        await waitForToast(page, /invalid cron/i, { type: 'error' });
      }
    );

    test.fixme(
      'should reject a retention count below 1 with backup_invalid_retention_count',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupBackupSettings(page);

        await page.route('**/api/v1/backups/settings', async (route) => {
          if (route.request().method() === 'PUT') {
            await route.fulfill({
              status: 400,
              json: {
                error: 'retention count must be at least 1',
                error_code: 'backup_invalid_retention_count',
              },
            });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await page.getByTestId('backup-schedule-retention-input').fill('0');
        await page.getByRole('button', { name: /save schedule/i }).click();

        await waitForToast(page, /retention/i, { type: 'error' });
      }
    );
  });

  test.describe('Saving triggers a reschedule', () => {
    test.fixme(
      'should PUT the updated schedule and show a success toast confirming no restart is needed',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        const { putCalls } = await setupBackupSettings(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await page.getByRole('radio', { name: /custom/i }).check();
        await page.getByTestId('backup-schedule-cron-input').fill('0 4 * * *');

        await Promise.all([
          page.waitForResponse(
            (r) => r.url().includes('/api/v1/backups/settings') && r.request().method() === 'PUT'
          ),
          page.getByRole('button', { name: /save schedule/i }).click(),
        ]);

        expect(putCalls.length).toBeGreaterThan(0);
        expect(putCalls[0]?.schedule_cron).toBe('0 4 * * *');

        // Backend reschedules the cron entry in-process (BackupService.Reschedule) —
        // the UI must not imply a container restart is required for this change.
        await waitForToast(page, /schedule updated|saved/i, { type: 'success' });
        await expect(page.getByText(/restart required/i)).not.toBeVisible();
      }
    );

    test.fixme(
      'should update retention_count and remote_retention_count independently',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        const { putCalls } = await setupBackupSettings(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await page.getByTestId('backup-schedule-retention-input').fill('14');
        await page.getByTestId('backup-schedule-remote-retention-input').fill('30');

        await Promise.all([
          page.waitForResponse(
            (r) => r.url().includes('/api/v1/backups/settings') && r.request().method() === 'PUT'
          ),
          page.getByRole('button', { name: /save schedule/i }).click(),
        ]);

        expect(putCalls[0]?.retention_count).toBe(14);
        expect(putCalls[0]?.remote_retention_count).toBe(30);
      }
    );
  });
});
