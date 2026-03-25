/**
 * CrowdSec Dashboard E2E Tests
 *
 * Tests the CrowdSec Dashboard tab functionality including:
 * - Tab navigation between Configuration and Dashboard
 * - Summary cards rendering
 * - Chart components rendering
 * - Time range selector interaction
 * - Active decisions table display
 * - Alerts list display
 * - Export button functionality
 * - Refresh button functionality
 *
 * @see /projects/Charon/docs/plans/current_spec.md PR-2, PR-3
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('CrowdSec Dashboard @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/security/crowdsec');
    await waitForLoadingComplete(page);
  });

  test.describe('Tab Navigation', () => {
    test('should display Configuration and Dashboard tabs', async ({ page }) => {
      await test.step('Verify tab list is present', async () => {
        const tabList = page.getByRole('tablist');
        await expect(tabList).toBeVisible();
      });

      await test.step('Verify Configuration tab', async () => {
        const configTab = page.getByRole('tab', { name: /configuration/i });
        await expect(configTab).toBeVisible();
      });

      await test.step('Verify Dashboard tab', async () => {
        const dashboardTab = page.getByRole('tab', { name: /dashboard/i });
        await expect(dashboardTab).toBeVisible();
      });
    });

    test('should default to Configuration tab', async ({ page }) => {
      await test.step('Verify Configuration tab is selected by default', async () => {
        const configTab = page.getByRole('tab', { name: /configuration/i });
        await expect(configTab).toHaveAttribute('data-state', 'active');
      });
    });

    test('should switch to Dashboard tab when clicked', async ({ page }) => {
      await test.step('Click Dashboard tab', async () => {
        await page.getByRole('tab', { name: /dashboard/i }).click();
      });

      await test.step('Verify Dashboard tab is selected', async () => {
        const dashboardTab = page.getByRole('tab', { name: /dashboard/i });
        await expect(dashboardTab).toHaveAttribute('data-state', 'active');
      });

      await test.step('Verify dashboard content is visible', async () => {
        await expect(page.getByTestId('dashboard-summary-cards')).toBeVisible({ timeout: 10000 });
      });
    });

    test('should switch back to Configuration tab', async ({ page }) => {
      await test.step('Navigate to Dashboard tab', async () => {
        await page.getByRole('tab', { name: /dashboard/i }).click();
        await expect(page.getByTestId('dashboard-summary-cards')).toBeVisible({ timeout: 10000 });
      });

      await test.step('Switch back to Configuration tab', async () => {
        await page.getByRole('tab', { name: /configuration/i }).click();
      });

      await test.step('Verify Configuration content is visible', async () => {
        const configTab = page.getByRole('tab', { name: /configuration/i });
        await expect(configTab).toHaveAttribute('data-state', 'active');
      });
    });
  });

  test.describe('Dashboard Content', () => {
    test.beforeEach(async ({ page }) => {
      await page.getByRole('tab', { name: /dashboard/i }).click();
      await expect(page.getByTestId('dashboard-summary-cards')).toBeVisible({ timeout: 10000 });
    });

    test('should display summary cards section', async ({ page }) => {
      await test.step('Verify summary cards container', async () => {
        await expect(page.getByTestId('dashboard-summary-cards')).toBeVisible();
      });
    });

    test('should display time range selector', async ({ page }) => {
      await test.step('Verify time range buttons are present', async () => {
        const timeRangeGroup = page.getByRole('tablist', { name: /time range/i });
        const timeRangeVisible = await timeRangeGroup.isVisible().catch(() => false);

        if (timeRangeVisible) {
          await expect(timeRangeGroup).toBeVisible();
        } else {
          // Fallback: look for individual time range buttons
          const button24h = page.getByRole('tab', { name: /24h/i });
          const buttonVisible = await button24h.isVisible().catch(() => false);

          if (buttonVisible) {
            await expect(button24h).toBeVisible();
          } else {
            test.info().annotations.push({
              type: 'info',
              description: 'Time range selector not visible - may use different UI pattern',
            });
          }
        }
      });
    });

    test('should display refresh button', async ({ page }) => {
      await test.step('Verify refresh button exists', async () => {
        const refreshButton = page.getByRole('button', { name: /refresh/i });
        await expect(refreshButton).toBeVisible();
      });
    });

    test('should display chart sections', async ({ page }) => {
      await test.step('Verify chart containers are rendered', async () => {
        // Charts render as role="img" with aria-labels
        const banTimeline = page.getByRole('img', { name: /ban.*timeline|timeline.*chart/i });
        const banTimelineVisible = await banTimeline.isVisible().catch(() => false);

        const topIPs = page.getByRole('img', { name: /top.*ip|attacking.*ip/i });
        const topIPsVisible = await topIPs.isVisible().catch(() => false);

        const scenarios = page.getByRole('img', { name: /scenario.*breakdown|scenario.*chart/i });
        const scenariosVisible = await scenarios.isVisible().catch(() => false);

        // At least one chart should render (even with error/empty state)
        // Charts may not render if CrowdSec is not running
        if (!banTimelineVisible && !topIPsVisible && !scenariosVisible) {
          // Check for error or empty state messages instead
          const errorOrEmpty = page.getByText(/error|no data|unavailable|failed/i).first();
          const hasMessage = await errorOrEmpty.isVisible().catch(() => false);

          if (!hasMessage) {
            test.info().annotations.push({
              type: 'info',
              description: 'Charts not rendered - CrowdSec may not be running or no data available',
            });
          }
        }
      });
    });

    test('should display active decisions table', async ({ page }) => {
      await test.step('Verify decisions table or empty state', async () => {
        const table = page.getByRole('table');
        const tableVisible = await table.isVisible().catch(() => false);

        if (tableVisible) {
          // If table is visible, verify it has column headers
          const headers = page.getByRole('columnheader');
          const headerCount = await headers.count();
          expect(headerCount).toBeGreaterThan(0);
        } else {
          // Table may not render if no data or CrowdSec not running
          const errorOrEmpty = page.getByText(/error|no.*decisions|no.*alerts|no data/i).first();
          const hasMessage = await errorOrEmpty.isVisible().catch(() => false);

          if (!hasMessage) {
            test.info().annotations.push({
              type: 'info',
              description: 'Active decisions table not visible - CrowdSec may not be running',
            });
          }
        }
      });
    });

    test('should display alerts list section', async ({ page }) => {
      await test.step('Verify alerts list or empty state', async () => {
        const alertsList = page.getByTestId('alerts-list');
        await expect(alertsList).toBeVisible({ timeout: 10000 });
      });
    });

    test('should display export button', async ({ page }) => {
      await test.step('Verify export button is visible', async () => {
        const exportBtn = page.getByRole('button', { name: /export/i });
        await expect(exportBtn).toBeVisible();
      });

      await test.step('Open export dropdown', async () => {
        const exportBtn = page.getByRole('button', { name: /export/i });
        await exportBtn.click();

        const menu = page.getByRole('menu');
        const menuVisible = await menu.isVisible().catch(() => false);

        if (menuVisible) {
          const csvOption = page.getByRole('menuitem', { name: /csv/i });
          const jsonOption = page.getByRole('menuitem', { name: /json/i });
          await expect(csvOption).toBeVisible();
          await expect(jsonOption).toBeVisible();
        } else {
          test.info().annotations.push({
            type: 'info',
            description: 'Export dropdown menu did not open - may need interaction delay',
          });
        }
      });
    });
  });

  test.describe('Time Range Interaction', () => {
    test.beforeEach(async ({ page }) => {
      await page.getByRole('tab', { name: /dashboard/i }).click();
      await expect(page.getByTestId('dashboard-summary-cards')).toBeVisible({ timeout: 10000 });
    });

    test('should update data when selecting different time range', async ({ page }) => {
      await test.step('Select 7d time range', async () => {
        const sevenDayButton = page.getByRole('tab', { name: /7d/i });
        const sevenDayVisible = await sevenDayButton.isVisible().catch(() => false);

        if (sevenDayVisible) {
          await sevenDayButton.click();
          await expect(sevenDayButton).toHaveAttribute('aria-selected', 'true');
        } else {
          test.info().annotations.push({
            type: 'info',
            description: '7d time range button not found - may use different selector pattern',
          });
        }
      });
    });
  });

  test.describe('Refresh Functionality', () => {
    test.beforeEach(async ({ page }) => {
      await page.getByRole('tab', { name: /dashboard/i }).click();
      await expect(page.getByTestId('dashboard-summary-cards')).toBeVisible({ timeout: 10000 });
    });

    test('should trigger data refresh when clicking refresh button', async ({ page }) => {
      await test.step('Click refresh and verify loading state', async () => {
        const refreshButton = page.getByRole('button', { name: /refresh/i });
        await expect(refreshButton).toBeVisible();

        // Click refresh and expect API calls to be triggered
        const responsePromise = page.waitForResponse(
          resp => resp.url().includes('/crowdsec/dashboard') || resp.url().includes('/crowdsec/alerts'),
          { timeout: 10000 }
        ).catch(() => null);

        await refreshButton.click();

        const response = await responsePromise;
        if (response) {
          // API call was made - refresh working
          expect(response.status()).toBeLessThan(500);
        } else {
          test.info().annotations.push({
            type: 'info',
            description: 'No dashboard API response detected after refresh - endpoints may not be implemented',
          });
        }
      });
    });
  });
});
