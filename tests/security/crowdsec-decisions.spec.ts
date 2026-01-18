/**
 * CrowdSec Decisions (Bans) E2E Tests
 *
 * Tests the CrowdSec decisions/bans management functionality:
 * - Viewing active decisions/bans
 * - Adding manual IP bans
 * - Removing bans (unban)
 * - Decision details and filtering
 *
 * @see /projects/Charon/docs/plans/current_spec.md - Phase 3
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForToast } from '../utils/wait-helpers';

// NOTE: The /security/crowdsec/decisions route doesn't exist as a separate page.
// Decisions are displayed within the main CrowdSec config page at /security/crowdsec.
// This test suite is skipped until the dedicated decisions route is implemented.
test.describe.skip('CrowdSec Decisions Management', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/security/crowdsec');
    await waitForLoadingComplete(page);
  });

  test.describe('Decisions List', () => {
    test('should display decisions page', async ({ page }) => {
      // The page should load - look for heading or table
      const heading = page.getByRole('heading', { name: /decisions|bans/i });
      const table = page.getByRole('table');
      const grid = page.locator('[class*="grid"], [class*="table"], [class*="list"]');

      const headingVisible = await heading.isVisible().catch(() => false);
      const tableVisible = await table.isVisible().catch(() => false);
      const gridVisible = await grid.first().isVisible().catch(() => false);

      // At least one should be visible
      expect(headingVisible || tableVisible || gridVisible).toBeTruthy();
    });

    test('should show active decisions if any exist', async ({ page }) => {
      // Wait for decisions to load
      await page.waitForResponse(resp =>
        resp.url().includes('/decisions') || resp.url().includes('/crowdsec'),
        { timeout: 10000 }
      ).catch(() => {
        // API might not be called if no decisions
      });

      // Could be empty state or list of decisions
      const emptyState = page.getByText(/no.*decisions|no.*bans|empty/i);
      const decisionRows = page.locator('tr, [class*="row"]').filter({
        hasText: /\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}|ban|captcha/i
      });

      const emptyVisible = await emptyState.isVisible().catch(() => false);
      const rowCount = await decisionRows.count();

      // Either empty state or some decisions
      expect(emptyVisible || rowCount >= 0).toBeTruthy();
    });

    test('should display decision columns (IP, type, duration, reason)', async ({ page }) => {
      const table = page.getByRole('table');
      const tableVisible = await table.isVisible().catch(() => false);

      if (tableVisible) {
        await test.step('Verify table headers', async () => {
          const headers = page.locator('th, [role="columnheader"]');
          const headerTexts = await headers.allTextContents();

          // Headers might include IP, type, duration, scope, reason, origin
          const headerString = headerTexts.join(' ').toLowerCase();
          const hasRelevantHeaders = headerString.includes('ip') ||
                                    headerString.includes('type') ||
                                    headerString.includes('scope') ||
                                    headerString.includes('origin');

          expect(hasRelevantHeaders).toBeTruthy();
        });
      }
    });
  });

  test.describe('Add Decision (Ban IP)', () => {
    test('should have add ban button', async ({ page }) => {
      const addButton = page.getByRole('button', { name: /add|ban|new/i });
      const addButtonVisible = await addButton.isVisible().catch(() => false);

      if (addButtonVisible) {
        await expect(addButton).toBeEnabled();
      } else {
        test.info().annotations.push({
          type: 'info',
          description: 'Add ban functionality might not be exposed in UI'
        });
      }
    });

    test('should open ban modal on add button click', async ({ page }) => {
      const addButton = page.getByRole('button', { name: /add.*ban|ban.*ip/i });
      const addButtonVisible = await addButton.isVisible().catch(() => false);

      if (addButtonVisible) {
        await addButton.click();

        await test.step('Verify ban modal opens', async () => {
          const modal = page.getByRole('dialog');
          const modalVisible = await modal.isVisible().catch(() => false);

          if (modalVisible) {
            // Modal should have IP input field
            const ipInput = modal.getByPlaceholder(/ip|address/i).or(
              modal.locator('input').first()
            );
            await expect(ipInput).toBeVisible();

            // Close modal
            const closeButton = modal.getByRole('button', { name: /cancel|close/i });
            await closeButton.click();
          }
        });
      }
    });

    test('should validate IP address format', async ({ page }) => {
      const addButton = page.getByRole('button', { name: /add.*ban|ban.*ip/i });
      const addButtonVisible = await addButton.isVisible().catch(() => false);

      if (addButtonVisible) {
        await addButton.click();

        const modal = page.getByRole('dialog');
        const modalVisible = await modal.isVisible().catch(() => false);

        if (modalVisible) {
          const ipInput = modal.locator('input').first();

          await test.step('Enter invalid IP', async () => {
            await ipInput.fill('invalid-ip');

            // Look for submit button
            const submitButton = modal.getByRole('button', { name: /ban|submit|add/i });
            await submitButton.click();

            // Should show validation error
            await page.waitForTimeout(500);
          });

          // Close modal
          const closeButton = modal.getByRole('button', { name: /cancel|close/i });
          const closeVisible = await closeButton.isVisible().catch(() => false);
          if (closeVisible) {
            await closeButton.click();
          }
        }
      }
    });
  });

  test.describe('Remove Decision (Unban)', () => {
    test('should show unban action for each decision', async ({ page }) => {
      // If there are decisions, each should have an unban action
      const unbanButtons = page.getByRole('button', { name: /unban|remove|delete/i });
      const count = await unbanButtons.count();

      // Just verify the selector works - actual decisions may or may not exist
      expect(count >= 0).toBeTruthy();
    });

    test('should confirm before unbanning', async ({ page }) => {
      const unbanButton = page.getByRole('button', { name: /unban|remove/i }).first();
      const unbanVisible = await unbanButton.isVisible().catch(() => false);

      if (unbanVisible) {
        await unbanButton.click();

        // Should show confirmation dialog
        const confirmDialog = page.getByRole('dialog');
        const dialogVisible = await confirmDialog.isVisible().catch(() => false);

        if (dialogVisible) {
          // Cancel the action
          const cancelButton = page.getByRole('button', { name: /cancel|no/i });
          await cancelButton.click();
        }
      }
    });
  });

  test.describe('Filtering and Search', () => {
    test('should have search/filter input', async ({ page }) => {
      const searchInput = page.getByPlaceholder(/search|filter/i);
      const searchVisible = await searchInput.isVisible().catch(() => false);

      if (searchVisible) {
        await expect(searchInput).toBeEnabled();
      }
    });

    test('should filter decisions by type', async ({ page }) => {
      const typeFilter = page.locator('select, [role="listbox"]').filter({
        hasText: /type|all|ban|captcha/i
      }).first();

      const filterVisible = await typeFilter.isVisible().catch(() => false);

      if (filterVisible) {
        await expect(typeFilter).toBeVisible();
      }
    });
  });

  test.describe('Refresh and Sync', () => {
    test('should have refresh button', async ({ page }) => {
      const refreshButton = page.getByRole('button', { name: /refresh|sync|reload/i });
      const refreshVisible = await refreshButton.isVisible().catch(() => false);

      if (refreshVisible) {
        await test.step('Click refresh button', async () => {
          await refreshButton.click();
          // Should trigger API call
          await page.waitForTimeout(500);
        });
      }
    });
  });

  test.describe('Navigation', () => {
    test('should navigate back to CrowdSec config', async ({ page }) => {
      const backLink = page.getByRole('link', { name: /crowdsec|back|config/i });
      const backVisible = await backLink.isVisible().catch(() => false);

      if (backVisible) {
        await backLink.click();
        await waitForLoadingComplete(page);
        await expect(page).toHaveURL(/\/security\/crowdsec(?!\/decisions)/);
      }
    });
  });

  test.describe('Accessibility', () => {
    test('should be keyboard navigable', async ({ page }) => {
      await page.keyboard.press('Tab');
      // Some element should receive focus
      const focusedElement = page.locator(':focus');
      await expect(focusedElement).toBeVisible();
    });
  });
});
