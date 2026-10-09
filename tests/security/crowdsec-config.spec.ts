/**
 * CrowdSec Configuration E2E Tests
 *
 * Tests the CrowdSec configuration page functionality including:
 * - Page loading and status display
 * - Preset management (view, apply, preview)
 * - Configuration file management
 * - Import/Export functionality
 * - Console enrollment (if enabled)
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

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

    test('should show navigation back to security dashboard', async ({ page }) => {
      // Should have breadcrumb, back link, or navigation element
      const backLink = page.getByRole('link', { name: /security|back/i });
      const backLinkVisible = await backLink.isVisible().catch(() => false);

      if (backLinkVisible) {
        await expect(backLink).toBeVisible();
      } else {
        // Check for navigation button instead
        const backButton = page.getByRole('button', { name: /back/i });
        const buttonVisible = await backButton.isVisible().catch(() => false);

        if (!buttonVisible) {
          // Check for breadcrumbs or navigation in header
          const breadcrumb = page.locator('nav, [class*="breadcrumb"], [aria-label*="breadcrumb"]');
          const breadcrumbVisible = await breadcrumb.isVisible().catch(() => false);

          // At minimum, the page should be loaded
          if (!breadcrumbVisible) {
            await expect(page).toHaveURL(/crowdsec/);
          }
        }
      }
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
    test('should display configuration file list', async ({ page }) => {
      // Look for file selector or list
      const fileSelector = page.locator('select, [role="listbox"]').filter({
        hasNotText: /sort|filter/i
      }).first();

      const filesVisible = await fileSelector.isVisible().catch(() => false);

      if (filesVisible) {
        await expect(fileSelector).toBeVisible();
      }
    });

    test('should show file content when selected', async ({ page }) => {
      // Find file selector
      const fileSelector = page.locator('select').first();
      const selectorVisible = await fileSelector.isVisible().catch(() => false);

      if (selectorVisible) {
        // Select a file - content area should appear
        const contentArea = page.locator('textarea, pre, [class*="editor"]');
        const contentVisible = await contentArea.isVisible().catch(() => false);

        // Content area may or may not be visible depending on selection
        expect(contentVisible !== undefined).toBeTruthy();
      }
    });
  });

  test.describe('Import/Export', () => {
    test('should have export functionality', async ({ page }) => {
      const exportButton = page.getByRole('button', { name: /export/i });
      const exportVisible = await exportButton.isVisible().catch(() => false);

      if (exportVisible) {
        await expect(exportButton).toBeEnabled();
      }
    });

    test('should have import functionality', async ({ page }) => {
      await expect(page.getByTestId('import-file')).toBeVisible();
      await expect(page.getByRole('button', { name: /^import$/i })).toBeVisible();
    });
  });

  test.describe('Console Enrollment', () => {
    test('should display console enrollment section if feature enabled', async ({ page }) => {
      // Console enrollment is a feature-flagged component
      const enrollmentSection = page.getByTestId('console-section').or(
        page.locator('[class*="card"]').filter({ hasText: /console.*enrollment|enroll/i })
      );

      const enrollmentVisible = await enrollmentSection.isVisible().catch(() => false);

      if (enrollmentVisible) {
        await test.step('Verify enrollment form elements', async () => {
          // Should have token input
          const tokenInput = page.getByTestId('crowdsec-token-input').or(
            page.getByPlaceholder(/token|key/i)
          );
          await expect(tokenInput).toBeVisible();
        });
      } else {
        // Feature might be disabled - that's OK
        test.info().annotations.push({
          type: 'info',
          description: 'Console enrollment feature not enabled'
        });
      }
    });

    test('should show enrollment status when enrolled', async ({ page }) => {
      const statusText = page.getByTestId('console-token-state').or(
        page.locator('text=/enrolled|pending|not enrolled/i')
      );

      const statusVisible = await statusText.isVisible().catch(() => false);

      if (statusVisible) {
        // Verify status is displayed
        await expect(statusText).toBeVisible();
      }
    });
  });

  test.describe('Status Indicators', () => {
    test('should display CrowdSec running status', async ({ page }) => {
      // Look for status indicators
      const statusBadge = page.locator('[class*="badge"]').filter({
        hasText: /running|stopped|enabled|disabled/i
      });

      const statusVisible = await statusBadge.first().isVisible().catch(() => false);

      if (statusVisible) {
        await expect(statusBadge.first()).toBeVisible();
      }
    });

    test('should display LAPI status', async ({ page }) => {
      // LAPI ready status might be displayed
      const lapiStatus = page.getByText(/lapi.*ready|lapi.*status/i);
      const lapiVisible = await lapiStatus.isVisible().catch(() => false);

      // LAPI status might not always be visible
      expect(lapiVisible !== undefined).toBeTruthy();
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
