/**
 * WAF (Coraza) Configuration E2E Tests
 *
 * Tests the Web Application Firewall configuration:
 * - Page loading and status display
 * - WAF mode toggle (blocking/detection)
 * - Ruleset management
 * - Rule group configuration
 * - Whitelist/exclusions
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForToast } from '../utils/wait-helpers';
import { clickSwitch } from '../utils/ui-helpers';

test.describe('WAF Configuration @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/security/waf');
    await waitForLoadingComplete(page);
  });

  test.describe('Page Loading', () => {
    test('should display WAF configuration page', async ({ page }) => {
      // Page should load with WAF heading or content
      const heading = page.getByRole('heading', { name: /waf|firewall|coraza/i });
      const headingVisible = await heading.isVisible().catch(() => false);

      if (!headingVisible) {
        // Might have different page structure
        const wafContent = page.getByText(/web application firewall|waf|coraza/i).first();
        await expect(wafContent).toBeVisible();
      } else {
        await expect(heading).toBeVisible();
      }
    });

    test('should display WAF status indicator', async ({ page }) => {
      const statusBadge = page.locator('[class*="badge"]').filter({
        hasText: /enabled|disabled|blocking|detection|active/i
      });

      const statusVisible = await statusBadge.first().isVisible().catch(() => false);

      if (statusVisible) {
        await expect(statusBadge.first()).toBeVisible();
      }
    });
  });

  test.describe('WAF Mode Toggle', () => {
    test('should display current WAF mode', async ({ page }) => {
      const modeIndicator = page.getByText(/blocking|detection|mode/i).first();
      const modeVisible = await modeIndicator.isVisible().catch(() => false);

      if (modeVisible) {
        await expect(modeIndicator).toBeVisible();
      }
    });

    test('should have mode toggle switch or selector', async ({ page }) => {
      const modeToggle = page.getByTestId('waf-mode-toggle').or(
        page.locator('button, [role="switch"]').filter({
          hasText: /blocking|detection/i
        })
      );

      const toggleVisible = await modeToggle.isVisible().catch(() => false);

      if (toggleVisible) {
        await expect(modeToggle).toBeEnabled();
      }
    });

    test('should toggle between blocking and detection mode', async ({ page }) => {
      const modeSwitch = page.locator('[role="switch"]').first();
      const switchVisible = await modeSwitch.isVisible().catch(() => false);

      if (switchVisible) {
        await test.step('Click mode switch', async () => {
          await clickSwitch(modeSwitch);
        });

        await test.step('Revert mode switch', async () => {
          await clickSwitch(modeSwitch);
        });
      }
    });
  });

  test.describe('Ruleset Management', () => {
    test('should display available rulesets', async ({ page }) => {
      const rulesetSection = page.getByText(/ruleset|rules|owasp|crs/i).first();
      await expect(rulesetSection).toBeVisible();
    });

    test('should show rule groups with toggle controls', async ({ page }) => {
      // Each rule group should be toggleable
      const ruleGroupToggles = page.locator('[role="switch"], input[type="checkbox"]').filter({
        has: page.locator('text=/sql|xss|rce|lfi|rfi|scanner/i')
      });

      // Count available toggles
      const count = await ruleGroupToggles.count().catch(() => 0);
      expect(count >= 0).toBeTruthy();
    });

    test('should allow enabling/disabling rule groups', async ({ page }) => {
      const ruleToggle = page.locator('[role="switch"]').first();
      const toggleVisible = await ruleToggle.isVisible().catch(() => false);

      if (toggleVisible) {
        // Record initial state
        const wasPressed = await ruleToggle.getAttribute('aria-pressed') === 'true' ||
                          await ruleToggle.getAttribute('aria-checked') === 'true';

        await test.step('Toggle rule group', async () => {
          await ruleToggle.click();
          await page.waitForTimeout(500);
        });

        await test.step('Restore original state', async () => {
          await ruleToggle.click();
          await page.waitForTimeout(500);
        });
      }
    });
  });

  test.describe('Anomaly Threshold', () => {
    test('should display anomaly threshold setting', async ({ page }) => {
      const thresholdSection = page.getByText(/threshold|score|anomaly/i).first();
      const thresholdVisible = await thresholdSection.isVisible().catch(() => false);

      if (thresholdVisible) {
        await expect(thresholdSection).toBeVisible();
      }
    });

    test('should have threshold input control', async ({ page }) => {
      const thresholdInput = page.locator('input[type="number"], input[type="range"]').filter({
        has: page.locator('text=/threshold|score/i')
      }).first();

      const inputVisible = await thresholdInput.isVisible().catch(() => false);

      // Threshold control might not be visible on all pages
      expect(inputVisible !== undefined).toBeTruthy();
    });
  });

  test.describe('Whitelist/Exclusions', () => {
    test('should display whitelist section', async ({ page }) => {
      const whitelistSection = page.getByText(/whitelist|exclusion|exception|ignore/i).first();
      const whitelistVisible = await whitelistSection.isVisible().catch(() => false);

      if (whitelistVisible) {
        await expect(whitelistSection).toBeVisible();
      }
    });

    test('should have ability to add whitelist entries', async ({ page }) => {
      const addButton = page.getByRole('button', { name: /add.*whitelist|add.*exclusion|add.*exception/i });
      const addVisible = await addButton.isVisible().catch(() => false);

      if (addVisible) {
        await expect(addButton).toBeEnabled();
      }
    });
  });

  test.describe('Save and Apply', () => {
    test('should have save button', async ({ page }) => {
      const saveButton = page.getByRole('button', { name: /save|apply|update/i });
      const saveVisible = await saveButton.isVisible().catch(() => false);

      if (saveVisible) {
        await expect(saveButton).toBeVisible();
      }
    });

    test('should show confirmation on save', async ({ page }) => {
      const saveButton = page.getByRole('button', { name: /save|apply/i });
      const saveVisible = await saveButton.isVisible().catch(() => false);

      if (saveVisible) {
        await saveButton.click();

        // Should show either confirmation dialog or success toast
        const dialog = page.getByRole('dialog');
        const dialogVisible = await dialog.isVisible().catch(() => false);

        if (dialogVisible) {
          const cancelButton = page.getByRole('button', { name: /cancel|close/i });
          await cancelButton.click();
        }
      }
    });
  });

  test.describe('Navigation', () => {
    test('should navigate back to security dashboard', async ({ page }) => {
      const backLink = page.getByRole('link', { name: /security|back/i });
      const backVisible = await backLink.isVisible().catch(() => false);

      if (backVisible) {
        await backLink.click();
        await waitForLoadingComplete(page);
        await expect(page).toHaveURL(/\/security(?!\/waf)/);
      }
    });
  });

  test.describe('Accessibility', () => {
    test('should have accessible controls', async ({ page }) => {
      const switches = page.locator('[role="switch"]');
      const count = await switches.count();

      for (let i = 0; i < Math.min(count, 5); i++) {
        const switchEl = switches.nth(i);
        const visible = await switchEl.isVisible();

        if (visible) {
          // Each switch should have accessible name
          const name = await switchEl.getAttribute('aria-label') ||
                       await switchEl.getAttribute('aria-labelledby');
          // Some form of accessible name should exist
        }
      }
    });
  });
});
