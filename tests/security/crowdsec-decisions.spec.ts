/**
 * CrowdSec Banned IPs (Decisions) E2E Tests
 *
 * Tests the "Banned IPs" card on /security/crowdsec against a stubbed CrowdSec API
 * (see tests/utils/crowdsec-stubs.ts), so every assertion runs against known data:
 * - Listing active bans, and the empty, disabled and error states
 * - Banning an IP (modal, validation, exact request payload, outcomes)
 * - Unbanning an IP (confirmation, request, outcomes)
 *
 * NOTE: CrowdSec "Decisions" are managed via the "Banned IPs" card on /security/crowdsec.
 * There is no separate decisions page and the card has no search, filter or refresh control.
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { getToastLocator } from '../utils/ui-helpers';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import {
  crowdsecFixtures,
  stubCrowdSecApi,
  type CrowdSecStubOptions,
} from '../utils/crowdsec-stubs';

const ERROR_TIMEOUT = 15_000;

const DECISIONS = [
  crowdsecFixtures.decision({
    id: 'decision-1',
    ip: '203.0.113.7',
    reason: 'e2e brute force',
    duration: '24h',
    source: 'crowdsec',
  }),
  crowdsecFixtures.decision({ id: 'decision-2', ip: '203.0.113.8', reason: '', duration: '4h', source: '' }),
];

/** Installs the stubs (CrowdSec in local mode), loads the CrowdSec page and waits for the Banned IPs card. */
async function openBannedIps(page: Page, options: CrowdSecStubOptions = {}) {
  const recorder = await stubCrowdSecApi(page, {
    crowdsecMode: 'local',
    decisions: DECISIONS,
    banApi: {},
    ...options,
  });
  await page.goto('/security/crowdsec');
  await waitForLoadingComplete(page);
  await expect(bannedIpsHeading(page)).toBeVisible();
  return recorder;
}

const bannedIpsHeading = (page: Page) => page.getByRole('heading', { name: 'Banned IPs', level: 3 });
const banDialog = (page: Page) => page.getByRole('dialog', { name: 'Ban IP Address' });
const unbanDialog = (page: Page) => page.getByRole('dialog', { name: 'Confirm Unban' });
/** Toasts prefix the message with an icon glyph, so they are matched on the message text within the live region. */
const successToast = (page: Page, message: string) => getToastLocator(page, message, { type: 'success' });
const errorToast = (page: Page, message: string) => getToastLocator(page, message, { type: 'error' });
const decisionRow = (page: Page, ip: string) => page.getByRole('row').filter({ hasText: ip });

test.describe('CrowdSec Banned IPs Management', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.describe('Banned IPs Card', () => {
    test('should display the banned IPs card on the CrowdSec config page', async ({ page }) => {
      await openBannedIps(page);

      await expect(page).toHaveURL('/security/crowdsec');
      await expect(page.getByRole('button', { name: 'Ban IP', exact: true })).toBeEnabled();
    });

    test('should disable banning and explain why when CrowdSec is disabled', async ({ page }) => {
      await openBannedIps(page, { crowdsecMode: 'disabled' });

      await expect(page.getByRole('button', { name: 'Ban IP', exact: true })).toBeDisabled();
      await expect(page.getByText('Enable CrowdSec to manage banned IPs', { exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Unban' })).toHaveCount(0);
    });
  });

  test.describe('Active Decisions', () => {
    test('should list each active ban with its details', async ({ page }) => {
      await openBannedIps(page);

      await expect(page.getByRole('columnheader', { name: /^(IP|Reason|Duration|Banned At|Source|Actions)$/ })).toHaveText([
        'IP',
        'Reason',
        'Duration',
        'Banned At',
        'Source',
        'Actions',
      ]);
      const first = decisionRow(page, '203.0.113.7').getByRole('cell');
      await expect(first.nth(0)).toHaveText('203.0.113.7');
      await expect(first.nth(1)).toHaveText('e2e brute force');
      await expect(first.nth(2)).toHaveText('24h');
      await expect(first.nth(3)).toHaveText(new Date('2026-01-01T00:00:00Z').toLocaleString());
      await expect(first.nth(4)).toHaveText('crowdsec');
      await expect(first.nth(5).getByRole('button', { name: 'Unban' })).toBeVisible();
    });

    test('should show placeholders for a ban without a reason or source', async ({ page }) => {
      await openBannedIps(page);

      const cells = decisionRow(page, '203.0.113.8').getByRole('cell');
      await expect(cells.nth(1)).toHaveText('-');
      await expect(cells.nth(2)).toHaveText('4h');
      await expect(cells.nth(4)).toHaveText('manual');
    });

    test('should offer an unban action for every ban', async ({ page }) => {
      await openBannedIps(page);

      await expect(page.getByRole('button', { name: 'Unban', exact: true })).toHaveCount(DECISIONS.length);
    });

    test('should show an empty state when nothing is banned', async ({ page }) => {
      await openBannedIps(page, { decisions: [] });

      await expect(page.getByText('No banned IPs', { exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Unban' })).toHaveCount(0);
    });

    test('should show an error when the bans cannot be loaded', async ({ page }) => {
      await openBannedIps(page, { decisions: { failWith: 500 } });

      await expect(page.getByText('Failed to load banned IPs', { exact: true })).toBeVisible({ timeout: ERROR_TIMEOUT });
    });

    test('should make each ban row keyboard focusable', async ({ page }) => {
      await openBannedIps(page);
      const row = decisionRow(page, '203.0.113.7');

      await row.focus();
      await expect(row).toBeFocused();
      await page.keyboard.press('Tab');
      await expect(row.getByRole('button', { name: 'Unban' })).toBeFocused();
    });
  });

  test.describe('Ban IP', () => {
    test('should open the ban dialog with a focused IP field and sensible defaults', async ({ page }) => {
      await openBannedIps(page);

      await page.getByRole('button', { name: 'Ban IP', exact: true }).click();

      const dialog = banDialog(page);
      await expect(dialog).toBeVisible();
      await expect(dialog.getByLabel('IP Address')).toBeFocused();
      await expect(dialog.getByLabel('IP Address')).toHaveValue('');
      await expect(dialog.getByLabel('Duration')).toHaveValue('24h');
      await expect(dialog.getByLabel('Duration').getByRole('option')).toHaveText([
        '1 hour',
        '4 hours',
        '24 hours',
        '7 days',
        '30 days',
        'Permanent',
      ]);
      await expect(dialog.getByLabel('Reason')).toHaveValue('');
      await expect(dialog.getByRole('button', { name: 'Ban IP', exact: true })).toBeDisabled();
    });

    test('should send the entered IP, duration and reason and report success', async ({ page }) => {
      const stubs = await openBannedIps(page);

      await page.getByRole('button', { name: 'Ban IP', exact: true }).click();
      const dialog = banDialog(page);
      await dialog.getByLabel('IP Address').fill('198.51.100.9');
      await dialog.getByLabel('Duration').selectOption({ label: '7 days' });
      await dialog.getByLabel('Reason').fill('e2e manual ban');
      await dialog.getByRole('button', { name: 'Ban IP', exact: true }).click();

      await expect(successToast(page, 'IP 198.51.100.9 has been banned')).toBeVisible();
      await expect(dialog).toHaveCount(0);
      expect(stubs.bans).toEqual([{ ip: '198.51.100.9', duration: '7d', reason: 'e2e manual ban' }]);
    });

    test('should reload the list after banning an IP', async ({ page }) => {
      const stubs = await openBannedIps(page);
      await expect.poll(() => stubs.decisionsRequests).toBe(1);

      await page.getByRole('button', { name: 'Ban IP', exact: true }).click();
      await banDialog(page).getByLabel('IP Address').fill('198.51.100.9');
      await banDialog(page).getByRole('button', { name: 'Ban IP', exact: true }).click();

      await expect(successToast(page, 'IP 198.51.100.9 has been banned')).toBeVisible();
      await expect.poll(() => stubs.decisionsRequests).toBe(2);
    });

    test('should submit with the keyboard from the IP field', async ({ page }) => {
      const stubs = await openBannedIps(page);

      await page.getByRole('button', { name: 'Ban IP', exact: true }).click();
      await banDialog(page).getByLabel('IP Address').fill('198.51.100.10');
      await page.keyboard.press('Enter');

      await expect(successToast(page, 'IP 198.51.100.10 has been banned')).toBeVisible();
      expect(stubs.bans).toEqual([{ ip: '198.51.100.10', duration: '24h', reason: '' }]);
    });

    test('should keep the ban button disabled while the IP is blank', async ({ page }) => {
      const stubs = await openBannedIps(page);

      await page.getByRole('button', { name: 'Ban IP', exact: true }).click();
      const dialog = banDialog(page);
      const submit = dialog.getByRole('button', { name: 'Ban IP', exact: true });

      await dialog.getByLabel('IP Address').fill('   ');
      await expect(submit).toBeDisabled();
      await dialog.getByLabel('IP Address').fill('198.51.100.9');
      await expect(submit).toBeEnabled();
      await dialog.getByLabel('IP Address').fill('');
      await expect(submit).toBeDisabled();
      expect(stubs.bans).toEqual([]);
    });

    test('should show the server error and keep the dialog open when the ban is rejected', async ({ page }) => {
      const stubs = await openBannedIps(page, { banApi: { banFailure: { failWith: 400 } } });

      await page.getByRole('button', { name: 'Ban IP', exact: true }).click();
      const dialog = banDialog(page);
      await dialog.getByLabel('IP Address').fill('not-an-ip');
      await dialog.getByRole('button', { name: 'Ban IP', exact: true }).click();

      await expect(errorToast(page, 'ban rejected')).toBeVisible();
      await expect(getToastLocator(page, /has been banned/, { type: 'success' })).toHaveCount(0);
      await expect(dialog).toBeVisible();
      await expect(dialog.getByLabel('IP Address')).toHaveValue('not-an-ip');
      expect(stubs.bans).toEqual([{ ip: 'not-an-ip', duration: '24h', reason: '' }]);
    });

    test('should close the dialog without banning when cancelled', async ({ page }) => {
      const stubs = await openBannedIps(page);

      await page.getByRole('button', { name: 'Ban IP', exact: true }).click();
      await banDialog(page).getByLabel('IP Address').fill('198.51.100.9');
      await banDialog(page).getByRole('button', { name: 'Cancel' }).click();

      await expect(banDialog(page)).toHaveCount(0);
      expect(stubs.bans).toEqual([]);
    });

    test('should close the dialog with Escape without banning', async ({ page }) => {
      const stubs = await openBannedIps(page);

      await page.getByRole('button', { name: 'Ban IP', exact: true }).click();
      await banDialog(page).getByLabel('IP Address').fill('198.51.100.9');
      await page.keyboard.press('Escape');
      await expect(banDialog(page)).toHaveCount(0);
      expect(stubs.bans).toEqual([]);
    });
  });

  test.describe('Unban IP', () => {
    test('should ask for confirmation naming the IP before unbanning', async ({ page }) => {
      const stubs = await openBannedIps(page);

      await decisionRow(page, '203.0.113.7').getByRole('button', { name: 'Unban' }).click();

      const dialog = unbanDialog(page);
      await expect(dialog).toBeVisible();
      await expect(dialog.getByText('Are you sure you want to unban 203.0.113.7?')).toBeVisible();
      await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused();
      expect(stubs.unbans).toEqual([]);
    });

    test('should leave the ban in place when the confirmation is cancelled', async ({ page }) => {
      const stubs = await openBannedIps(page);

      await decisionRow(page, '203.0.113.7').getByRole('button', { name: 'Unban' }).click();
      await unbanDialog(page).getByRole('button', { name: 'Cancel' }).click();

      await expect(unbanDialog(page)).toHaveCount(0);
      await expect(decisionRow(page, '203.0.113.7')).toBeVisible();
      expect(stubs.unbans).toEqual([]);
    });

    test('should unban the IP once confirmed and reload the list', async ({ page }) => {
      const stubs = await openBannedIps(page);
      await expect.poll(() => stubs.decisionsRequests).toBe(1);

      await decisionRow(page, '203.0.113.7').getByRole('button', { name: 'Unban' }).click();
      await unbanDialog(page).getByRole('button', { name: 'Unban', exact: true }).click();

      await expect(successToast(page, 'IP 203.0.113.7 has been unbanned')).toBeVisible();
      await expect(unbanDialog(page)).toHaveCount(0);
      expect(stubs.unbans).toEqual(['203.0.113.7']);
      await expect.poll(() => stubs.decisionsRequests).toBe(2);
    });

    test('should show the server error and keep the confirmation open when the unban fails', async ({ page }) => {
      const stubs = await openBannedIps(page, { banApi: { unbanFailure: { failWith: 500 } } });

      await decisionRow(page, '203.0.113.7').getByRole('button', { name: 'Unban' }).click();
      await unbanDialog(page).getByRole('button', { name: 'Unban', exact: true }).click();

      await expect(errorToast(page, 'unban rejected')).toBeVisible();
      await expect(getToastLocator(page, /has been unbanned/, { type: 'success' })).toHaveCount(0);
      await expect(unbanDialog(page)).toBeVisible();
      expect(stubs.unbans).toEqual(['203.0.113.7']);
    });

    test('should close the confirmation with Escape', async ({ page }) => {
      const stubs = await openBannedIps(page);

      await decisionRow(page, '203.0.113.7').getByRole('button', { name: 'Unban' }).click();
      await expect(unbanDialog(page)).toBeVisible();
      await page.keyboard.press('Escape');

      await expect(unbanDialog(page)).toHaveCount(0);
      expect(stubs.unbans).toEqual([]);
    });
  });
});
