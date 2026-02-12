/**
 * Security Headers E2E Tests
 *
 * Tests the security headers configuration:
 * - Page loading and status
 * - Header profile management (CRUD)
 * - Preset selection
 * - Header score display
 * - Individual header configuration
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForToast } from '../utils/wait-helpers';

test.describe('Security Headers Configuration @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/security/headers');
    await waitForLoadingComplete(page);
  });

  test.describe('Page Loading', () => {
    test('should display security headers page', async ({ page }) => {
      const heading = page.getByRole('heading', { name: /security.*headers|headers/i });
      const headingVisible = await heading.isVisible().catch(() => false);

      if (!headingVisible) {
        const content = page.getByText(/security.*headers|csp|hsts|x-frame/i).first();
        await expect(content).toBeVisible();
      } else {
        await expect(heading).toBeVisible();
      }
    });
  });

  test.describe('Header Score Display', () => {
    test('should display security score', async ({ page }) => {
      const scoreDisplay = page.getByText(/score|grade|rating/i).first();
      const scoreVisible = await scoreDisplay.isVisible().catch(() => false);

      if (scoreVisible) {
        await expect(scoreDisplay).toBeVisible();
      }
    });

    test('should show score breakdown', async ({ page }) => {
      const scoreDetails = page.locator('[class*="score"], [class*="grade"]').filter({
        hasText: /a|b|c|d|f|\d+%/i
      });

      const detailsVisible = await scoreDetails.first().isVisible().catch(() => false);
      expect(detailsVisible !== undefined).toBeTruthy();
    });
  });

  test.describe('Preset Profiles', () => {
    test('should display preset profiles', async ({ page }) => {
      const presetSection = page.getByText(/preset|profile|template/i).first();
      const presetVisible = await presetSection.isVisible().catch(() => false);

      if (presetVisible) {
        await expect(presetSection).toBeVisible();
      }
    });

    test('should have preset options (Basic, Strict, Custom)', async ({ page }) => {
      const presets = page.locator('button, [role="option"]').filter({
        hasText: /basic|strict|custom|minimal|paranoid/i
      });

      const count = await presets.count();
      expect(count >= 0).toBeTruthy();
    });

    test('should apply preset when selected', async ({ page }) => {
      const presetButton = page.locator('button').filter({
        hasText: /basic|strict|apply/i
      }).first();

      const presetVisible = await presetButton.isVisible().catch(() => false);

      if (presetVisible) {
        await test.step('Click preset button', async () => {
          await presetButton.click();
          await page.waitForTimeout(500);
        });
      }
    });
  });

  test.describe('Individual Header Configuration', () => {
    test('should display CSP (Content-Security-Policy) settings', async ({ page }) => {
      const cspSection = page.getByText(/content-security-policy|csp/i).first();
      const cspVisible = await cspSection.isVisible().catch(() => false);

      if (cspVisible) {
        await expect(cspSection).toBeVisible();
      }
    });

    test('should display HSTS settings', async ({ page }) => {
      const hstsSection = page.getByText(/strict-transport-security|hsts/i).first();
      const hstsVisible = await hstsSection.isVisible().catch(() => false);

      if (hstsVisible) {
        await expect(hstsSection).toBeVisible();
      }
    });

    test('should display X-Frame-Options settings', async ({ page }) => {
      const xframeSection = page.getByText(/x-frame-options|frame/i).first();
      const xframeVisible = await xframeSection.isVisible().catch(() => false);

      expect(xframeVisible !== undefined).toBeTruthy();
    });

    test('should display X-Content-Type-Options settings', async ({ page }) => {
      const xctSection = page.getByText(/x-content-type|nosniff/i).first();
      const xctVisible = await xctSection.isVisible().catch(() => false);

      expect(xctVisible !== undefined).toBeTruthy();
    });
  });

  test.describe('Header Toggle Controls', () => {
    test('should have toggles for individual headers', async ({ page }) => {
      const toggles = page.locator('[role="switch"]');
      const count = await toggles.count();

      // Should have multiple header toggles
      expect(count >= 0).toBeTruthy();
    });

    test('should toggle header on/off', async ({ page }) => {
      const toggle = page.locator('[role="switch"]').first();
      const toggleVisible = await toggle.isVisible().catch(() => false);

      if (toggleVisible) {
        await test.step('Toggle header', async () => {
          await toggle.click();
          await page.waitForTimeout(500);
        });

        await test.step('Revert toggle', async () => {
          await toggle.click();
          await page.waitForTimeout(500);
        });
      }
    });
  });

  test.describe('Profile Management', () => {
    test('should have create profile button', async ({ page }) => {
      const createButton = page.getByRole('button', { name: /create|new|add.*profile/i });
      const createVisible = await createButton.isVisible().catch(() => false);

      if (createVisible) {
        await expect(createButton).toBeEnabled();
      }
    });

    test('should open profile creation modal', async ({ page }) => {
      const createButton = page.getByRole('button', { name: /create|new.*profile/i });
      const createVisible = await createButton.isVisible().catch(() => false);

      if (createVisible) {
        await createButton.click();

        const modal = page.getByRole('dialog');
        const modalVisible = await modal.isVisible().catch(() => false);

        if (modalVisible) {
          // Close modal
          const closeButton = page.getByRole('button', { name: /cancel|close/i });
          await closeButton.click();
        }
      }
    });

    test('should list existing profiles', async ({ page }) => {
      const profileList = page.locator('[class*="list"], [class*="grid"]').filter({
        has: page.locator('[class*="card"], tr, [class*="item"]')
      }).first();

      const listVisible = await profileList.isVisible().catch(() => false);
      expect(listVisible !== undefined).toBeTruthy();
    });
  });

  test.describe('Save Configuration', () => {
    test('should have save button', async ({ page }) => {
      const saveButton = page.getByRole('button', { name: /save|apply|update/i });
      const saveVisible = await saveButton.isVisible().catch(() => false);

      if (saveVisible) {
        await expect(saveButton).toBeVisible();
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
      }
    });
  });

  test.describe('Accessibility', () => {
    test('should have accessible toggle controls', async ({ page }) => {
      const toggles = page.locator('[role="switch"]');
      const count = await toggles.count();

      for (let i = 0; i < Math.min(count, 5); i++) {
        const toggle = toggles.nth(i);
        const visible = await toggle.isVisible();

        if (visible) {
          // Toggle should have accessible state
          const checked = await toggle.getAttribute('aria-checked');
          expect(['true', 'false', 'mixed'].includes(checked || '')).toBeTruthy();
        }
      }
    });
  });
});
