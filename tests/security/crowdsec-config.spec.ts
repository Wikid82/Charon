/**
 * CrowdSec Configuration E2E Tests
 *
 * Tests the CrowdSec configuration page functionality including:
 * - Page loading and status display
 * - Preset management (view, apply, preview)
 * - Configuration file management
 * - Import/Export functionality
 * - Console enrollment (stubbed API states)
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import {
  CROWDSEC_ROUTES,
  crowdsecFixtures,
  stubCrowdSecApi,
  type CrowdSecStubOptions,
} from '../utils/crowdsec-stubs';

/** Installs the CrowdSec API stubs, then (re)loads the configuration page so they take effect. */
async function openWithStubs(page: Page, options: CrowdSecStubOptions) {
  const recorder = await stubCrowdSecApi(page, options);
  await page.goto('/security/crowdsec');
  await waitForLoadingComplete(page);
  return recorder;
}

test.describe('CrowdSec Configuration @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/security/crowdsec');
    await waitForLoadingComplete(page);
  });

  test.describe('Page Loading', () => {
    test('should display CrowdSec configuration page', async ({ page }) => {
      // The page should load without errors
      await expect(page.getByRole('heading', { name: /crowdsec/i }).first()).toBeVisible();
    });

    test('should navigate back to the security dashboard from the page note', async ({ page }) => {
      await page.getByRole('link', { name: 'Cerberus', exact: true }).click();
      await expect(page).toHaveURL(/\/security$/);
    });

    test('should display presets section', async ({ page }) => {
      await expect(page.getByRole('heading', { name: 'Presets', level: 3 })).toBeVisible();
      await expect(page.getByRole('button', { name: /apply preset/i })).toBeVisible();
    });
  });

  test.describe('Preset Management', () => {
    // Apply outcomes (success, error, fallback) are covered by crowdsec-preset-apply.spec.ts
    // and crowdsec-hub-preset-apply.spec.ts. These cases cover the list, search and preview.
    const CURATED_TITLE = 'Honeypot Friendly Defaults';
    const OTHER_TITLE = 'GeoIP Enrichment';

    test('should list the curated presets', async ({ page }) => {
      await expect(page.getByRole('button', { name: new RegExp(CURATED_TITLE, 'i') })).toBeVisible();
      await expect(page.getByRole('button', { name: new RegExp(OTHER_TITLE, 'i') })).toBeVisible();
    });

    test('should filter presets by the search query', async ({ page }) => {
      const search = page.getByRole('textbox', { name: /search presets/i });

      await test.step('Search for a matching preset', async () => {
        await search.fill('honeypot');
        await expect(page.getByRole('button', { name: new RegExp(CURATED_TITLE, 'i') })).toBeVisible();
        await expect(page.getByRole('button', { name: new RegExp(OTHER_TITLE, 'i') })).toHaveCount(0);
      });

      await test.step('Search for a non-matching term', async () => {
        await search.fill('no-such-preset-xyz');
        await expect(page.getByRole('button', { name: new RegExp(CURATED_TITLE, 'i') })).toHaveCount(0);
        await expect(page.getByText(/no presets/i).first()).toBeVisible();
      });
    });

    test('should show the preset details when a preset is selected', async ({ page }) => {
      const preset = page.getByRole('button', { name: new RegExp(CURATED_TITLE, 'i') });
      await preset.click();
      await expect(preset).toHaveAttribute('aria-pressed', 'true');
      await expect(page.getByTestId('preset-warning')).toBeVisible();
      await expect(page.getByTestId('preset-preview')).toBeVisible();
    });
  });

  test.describe('Configuration Files', () => {
    const FILES = ['acquis.yaml', 'config.yaml'];
    const CONTENTS: Record<string, string> = {
      'acquis.yaml': 'filenames:\n  - /var/log/caddy/access.log\n',
      'config.yaml': 'common:\n  log_level: info\n',
    };

    test.beforeEach(async ({ page }) => {
      await openWithStubs(page, { files: FILES, fileContents: CONTENTS });
    });

    test('should list the configuration files in the file selector', async ({ page }) => {
      const selector = page.getByRole('combobox', { name: 'Select a file...' });
      await expect(selector).toBeVisible();
      await expect(selector.getByRole('option')).toHaveText(['Select a file...', ...FILES]);
    });

    test('should show the file content when a file is selected', async ({ page }) => {
      const selector = page.getByRole('combobox', { name: 'Select a file...' });
      const content = page.locator('textarea');

      await test.step('No content is shown before a file is chosen', async () => {
        await expect(content).toHaveValue('');
      });

      await test.step('Select the first file', async () => {
        await selector.selectOption('acquis.yaml');
        await expect(content).toHaveValue(CONTENTS['acquis.yaml']);
      });

      await test.step('Select the second file', async () => {
        await selector.selectOption('config.yaml');
        await expect(content).toHaveValue(CONTENTS['config.yaml']);
      });
    });

    test('should clear the content when the editor is closed', async ({ page }) => {
      const selector = page.getByRole('combobox', { name: 'Select a file...' });
      const content = page.locator('textarea');
      await selector.selectOption('config.yaml');
      await expect(content).toHaveValue(CONTENTS['config.yaml']);

      await page.getByRole('button', { name: 'Close', exact: true }).click();
      await expect(content).toHaveValue('');
    });

    test('should expose the file editor with an accessible name', async ({ page }) => {
      test.fixme(true, '#1530 F2: files.content, packages.selectFile and presets.sortBy translation keys are missing, so aria-labels are raw keys');
      await expect(page.getByRole('textbox', { name: /file content/i })).toBeVisible();
    });
  });

  test.describe('Import/Export', () => {
    const ARCHIVE = Buffer.from('stub-archive-bytes');

    test('should export the configuration under the chosen file name', async ({ page }) => {
      const stubs = await openWithStubs(page, { exportBody: ARCHIVE });

      page.once('dialog', (dialog) => {
        expect(dialog.type()).toBe('prompt');
        void dialog.accept('my-backup');
      });
      const downloadPromise = page.waitForEvent('download');
      await page.getByRole('button', { name: 'Export', exact: true }).click();
      const download = await downloadPromise;

      expect(download.suggestedFilename()).toBe('my-backup.tar.gz');
      expect(stubs.exportRequests).toBe(1);
      await expect(page.getByText('CrowdSec configuration exported')).toBeVisible();
    });

    test('should not export when the file name prompt is cancelled', async ({ page }) => {
      const stubs = await openWithStubs(page, { exportBody: ARCHIVE });

      page.once('dialog', (dialog) => void dialog.dismiss());
      await page.getByRole('button', { name: 'Export', exact: true }).click();

      await expect(page.getByText('CrowdSec configuration exported')).toHaveCount(0);
      expect(stubs.exportRequests).toBe(0);
    });

    test('should show an error when the export fails', async ({ page }) => {
      await openWithStubs(page, {});
      await page.route(CROWDSEC_ROUTES.export, (route) =>
        route.fulfill({ status: 500, contentType: 'application/json', json: { error: 'export failed' } }),
      );

      page.once('dialog', (dialog) => void dialog.accept('my-backup'));
      await page.getByRole('button', { name: 'Export', exact: true }).click();

      await expect(page.getByText('Failed to export CrowdSec configuration')).toBeVisible();
      await expect(page.getByText('CrowdSec configuration exported')).toHaveCount(0);
    });

    test('should have import functionality', async ({ page }) => {
      await expect(page.getByTestId('import-file')).toBeVisible();
      await expect(page.getByRole('button', { name: /^import$/i })).toBeVisible();
    });
  });

  test.describe('Console Enrollment', () => {
    const consoleCard = (page: Page) => page.getByTestId('console-enrollment-card');

    test('should hide the console enrollment section when the feature is disabled', async ({ page }) => {
      await openWithStubs(page, { consoleEnrollmentEnabled: false, consoleStatus: crowdsecFixtures.notEnrolled() });

      await expect(page.getByRole('heading', { name: 'Presets', level: 3 })).toBeVisible();
      await expect(page.getByRole('heading', { name: 'Console Enrollment', level: 3 })).toHaveCount(0);
    });

    test('should show the enrollment form when the feature is enabled', async ({ page }) => {
      await openWithStubs(page, {
        consoleEnrollmentEnabled: true,
        consoleStatus: crowdsecFixtures.notEnrolled(),
        status: crowdsecFixtures.runningStatus(),
      });

      await expect(page.getByRole('heading', { name: 'Console Enrollment', level: 3 })).toBeVisible();
      await expect(consoleCard(page).getByLabel('Enrollment Token', { exact: true })).toBeVisible();
      await expect(consoleCard(page).getByLabel('Agent Name (optional)')).toBeVisible();
      await expect(consoleCard(page).getByRole('button', { name: 'Enroll', exact: true })).toBeVisible();
      await expect(page.getByText('Status: not enrolled')).toBeVisible();
      await expect(page.getByText('Not stored')).toBeVisible();
    });

    const STATES = [
      {
        name: 'pending acceptance',
        state: crowdsecFixtures.enrollment({ status: 'pending_acceptance' }),
        label: 'Status: pending acceptance',
        token: 'Stored (masked)',
      },
      {
        name: 'enrolled',
        state: crowdsecFixtures.enrollment({
          status: 'enrolled',
          last_heartbeat_at: '2026-01-02T03:04:05Z',
          enrolled_at: '2026-01-01T00:00:00Z',
        }),
        label: 'Status: enrolled',
        token: 'Stored (masked)',
      },
      {
        name: 'failed',
        state: crowdsecFixtures.enrollment({ status: 'failed', last_error: 'key rejected by console' }),
        label: 'Status: degraded',
        token: 'Stored (masked)',
      },
    ];

    for (const { name, state, label, token } of STATES) {
      test(`should display the ${name} enrollment state`, async ({ page }) => {
        await openWithStubs(page, {
          consoleEnrollmentEnabled: true,
          consoleStatus: state,
          status: crowdsecFixtures.runningStatus(),
        });

        await expect(page.getByText(label, { exact: true })).toBeVisible();
        await expect(page.getByTestId('console-token-state')).toHaveText(token);
        await expect(consoleCard(page).getByText('e2e-agent', { exact: true })).toBeVisible();
      });
    }

    test('should show the heartbeat time and enrolled actions for an enrolled agent', async ({ page }) => {
      await openWithStubs(page, {
        consoleEnrollmentEnabled: true,
        consoleStatus: crowdsecFixtures.enrollment({ last_heartbeat_at: '2026-01-02T03:04:05Z' }),
        status: crowdsecFixtures.runningStatus(),
      });

      await expect(page.getByText(/Last heartbeat: .*2026/)).toBeVisible();
      await expect(page.getByTestId('reenroll-section')).toBeVisible();
      await expect(page.getByRole('button', { name: 'Rotate Key' })).toBeEnabled();
    });

    test('should show the acceptance instructions while enrollment is pending acceptance', async ({ page }) => {
      await openWithStubs(page, {
        consoleEnrollmentEnabled: true,
        consoleStatus: crowdsecFixtures.enrollment({ status: 'pending_acceptance' }),
        status: crowdsecFixtures.runningStatus(),
      });

      await expect(page.getByTestId('pending-acceptance-info')).toContainText('Action Required');
    });

    test('should show the last error and a retry action for a failed enrollment', async ({ page }) => {
      await openWithStubs(page, {
        consoleEnrollmentEnabled: true,
        consoleStatus: crowdsecFixtures.enrollment({ status: 'failed', last_error: 'key rejected by console' }),
        status: crowdsecFixtures.runningStatus(),
      });

      await expect(page.getByTestId('console-status-error')).toContainText('key rejected by console');
      await expect(page.getByRole('button', { name: 'Retry Enrollment' })).toBeVisible();
    });

    test('should label the last error in readable text', async ({ page }) => {
      test.fixme(true, '#1530 F1: consoleEnrollment.lastError translation key is missing, so the last-error line renders a raw key');
      await openWithStubs(page, {
        consoleEnrollmentEnabled: true,
        consoleStatus: crowdsecFixtures.enrollment({ status: 'failed', last_error: 'key rejected by console' }),
        status: crowdsecFixtures.runningStatus(),
      });

      await expect(page.getByTestId('console-status-error')).toHaveText('Last error: key rejected by console');
    });

    test('should submit the enrollment and report success', async ({ page }) => {
      await openWithStubs(page, {
        consoleEnrollmentEnabled: true,
        consoleStatus: crowdsecFixtures.notEnrolled(),
        status: crowdsecFixtures.runningStatus(),
      });
      let payload: unknown;
      await page.route(CROWDSEC_ROUTES.consoleEnroll, async (route) => {
        payload = route.request().postDataJSON();
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          json: crowdsecFixtures.enrollment({ status: 'pending_acceptance', agent_name: 'e2e-agent', tenant: 'e2e-tenant' }),
        });
      });

      const card = consoleCard(page);
      await card.getByLabel('Enrollment Token', { exact: true }).fill('stub-enrollment-key');
      await card.getByLabel('Agent Name (optional)').fill('e2e-agent');
      await card.getByLabel('Tenant/Organization (optional)').fill('e2e-tenant');
      await card.getByRole('checkbox', { name: /I understand this will rotate/ }).check();
      await card.getByRole('button', { name: 'Enroll', exact: true }).click();

      await expect(page.getByText(/Enrollment request sent!/)).toBeVisible();
      expect(payload).toEqual({
        enrollment_key: 'stub-enrollment-key',
        tenant: 'e2e-tenant',
        agent_name: 'e2e-agent',
        force: false,
      });
    });

    test('should require the acknowledgement before enrolling', async ({ page }) => {
      await openWithStubs(page, {
        consoleEnrollmentEnabled: true,
        consoleStatus: crowdsecFixtures.notEnrolled(),
        status: crowdsecFixtures.runningStatus(),
      });
      let enrollRequests = 0;
      await page.route(CROWDSEC_ROUTES.consoleEnroll, (route) => {
        enrollRequests += 1;
        return route.fulfill({ status: 200, contentType: 'application/json', json: crowdsecFixtures.notEnrolled() });
      });

      const card = consoleCard(page);
      await card.getByLabel('Enrollment Token', { exact: true }).fill('stub-enrollment-key');
      await card.getByRole('button', { name: 'Enroll', exact: true }).click();

      await expect(page.getByText('You must acknowledge the console data-sharing notice')).toBeVisible();
      expect(enrollRequests).toBe(0);
    });

    test('should surface the backend error when enrollment is rejected', async ({ page }) => {
      await openWithStubs(page, {
        consoleEnrollmentEnabled: true,
        consoleStatus: crowdsecFixtures.notEnrolled(),
        status: crowdsecFixtures.runningStatus(),
      });
      await page.route(CROWDSEC_ROUTES.consoleEnroll, (route) =>
        route.fulfill({ status: 400, contentType: 'application/json', json: { error: 'invalid key' } }),
      );

      const card = consoleCard(page);
      await card.getByLabel('Enrollment Token', { exact: true }).fill('bad-key');
      await card.getByLabel('Tenant/Organization (optional)').fill('e2e-tenant');
      await card.getByRole('checkbox', { name: /I understand this will rotate/ }).check();
      await card.getByRole('button', { name: 'Enroll', exact: true }).click();

      await expect(page.getByTestId('console-enroll-error')).toContainText('invalid key');
      await expect(page.getByText(/Enrollment request sent!/)).toHaveCount(0);
    });
  });

  test.describe('Status Indicators', () => {
    // The LAPI banners appear only after the page's 3 second start-up grace period.
    const BANNER_TIMEOUT = 15_000;
    const ENROLL_FLAGS = { consoleEnrollmentEnabled: true, consoleStatus: crowdsecFixtures.notEnrolled() };

    test('should allow enrollment and show no warnings when CrowdSec and LAPI are ready', async ({ page }) => {
      await openWithStubs(page, { ...ENROLL_FLAGS, status: crowdsecFixtures.runningStatus() });

      const card = page.getByTestId('console-enrollment-card');
      await card.getByLabel('Enrollment Token', { exact: true }).fill('stub-enrollment-key');
      const enroll = card.getByRole('button', { name: 'Enroll', exact: true });
      await page.waitForResponse(CROWDSEC_ROUTES.status);

      await expect(enroll).toBeEnabled();
      await expect(page.getByTestId('lapi-warning')).toHaveCount(0);
      await expect(page.getByTestId('lapi-not-running-warning')).toHaveCount(0);
    });

    test('should warn and block enrollment while CrowdSec is running but LAPI is not ready', async ({ page }) => {
      await openWithStubs(page, {
        ...ENROLL_FLAGS,
        status: crowdsecFixtures.runningStatus({ lapi_ready: false }),
      });

      const card = page.getByTestId('console-enrollment-card');
      await card.getByLabel('Enrollment Token', { exact: true }).fill('stub-enrollment-key');

      await expect(page.getByTestId('lapi-warning')).toBeVisible({ timeout: BANNER_TIMEOUT });
      await expect(page.getByTestId('lapi-not-running-warning')).toHaveCount(0);
      await expect(card.getByRole('button', { name: 'Enroll', exact: true })).toBeDisabled();
      await expect(page.getByTestId('lapi-warning').getByRole('button', { name: 'Check Now' })).toBeEnabled();
    });

    test('should warn and block enrollment when CrowdSec is not running', async ({ page }) => {
      await openWithStubs(page, {
        ...ENROLL_FLAGS,
        status: crowdsecFixtures.runningStatus({ running: false, pid: 0, lapi_ready: false }),
      });

      const warning = page.getByTestId('lapi-not-running-warning');
      await expect(warning).toBeVisible({ timeout: BANNER_TIMEOUT });
      await expect(page.getByTestId('lapi-warning')).toHaveCount(0);
      await expect(warning.getByRole('button', { name: 'Check Now' })).toBeEnabled();
      await expect(page.getByTestId('console-enrollment-card').getByRole('button', { name: 'Enroll', exact: true })).toBeDisabled();
    });

    test('should show readable text in the LAPI warnings', async ({ page }) => {
      test.fixme(true, '#1530 F1: crowdsecConfig.lapiInitializing, notRunning, startCrowdsec, goToSecurity and related translation keys are missing, so the warnings render raw keys');
      await openWithStubs(page, {
        ...ENROLL_FLAGS,
        status: crowdsecFixtures.runningStatus({ running: false, pid: 0, lapi_ready: false }),
      });

      const warning = page.getByTestId('lapi-not-running-warning');
      await expect(warning).toBeVisible({ timeout: BANNER_TIMEOUT });
      await expect(warning).not.toContainText('crowdsecConfig.');
      await expect(warning.getByRole('button', { name: 'Start CrowdSec' })).toBeVisible();
    });
  });

  test.describe('Accessibility', () => {
    test('should have accessible form controls', async ({ page }) => {
      // Check that inputs have associated labels
      const inputs = page.locator('input:not([type="hidden"])');
      const count = await inputs.count();

      for (let i = 0; i < Math.min(count, 5); i++) {
        const input = inputs.nth(i);
        const visible = await input.isVisible();

        if (visible) {
          // Input should have some form of label (explicit, aria-label, or placeholder)
          const hasLabel = await input.getAttribute('aria-label') ||
                          await input.getAttribute('placeholder') ||
                          await input.getAttribute('id');
          expect(hasLabel).toBeTruthy();
        }
      }
    });
  });
});
