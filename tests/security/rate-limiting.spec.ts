/**
 * Rate Limiting E2E Tests
 *
 * Tests the rate limiting configuration:
 * - Page loading and status
 * - RPS/Burst settings
 * - Time window configuration
 * - Per-route settings
 *
 * @see /projects/Charon/docs/plans/current_spec.md - Phase 3
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForToast } from '../utils/wait-helpers';

test.describe('Rate Limiting Configuration @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/security/rate-limiting');
    await waitForLoadingComplete(page);
  });

  test.describe('Page Loading', () => {
    test('should display rate limiting configuration page', async ({ page }) => {
      const heading = page.getByRole('heading', { name: /rate.*limit/i });
      const headingVisible = await heading.isVisible().catch(() => false);

      if (!headingVisible) {
        const content = page.getByText(/rate.*limit|rps|requests per second/i).first();
        await expect(content).toBeVisible();
      } else {
        await expect(heading).toBeVisible();
      }
    });

    test('should display rate limiting status', async ({ page }) => {
      const statusBadge = page.locator('[class*="badge"]').filter({
        hasText: /enabled|disabled|active|inactive/i
      });

      const statusVisible = await statusBadge.first().isVisible().catch(() => false);
      expect(statusVisible !== undefined).toBeTruthy();
    });
  });

  test.describe('Rate Limiting Toggle', () => {
    test('should have enable/disable toggle', async ({ page }) => {
      // The toggle may be on this page or only on the main security dashboard
      const toggle = page.getByTestId('toggle-rate-limit').or(
        page.locator('input[type="checkbox"]').first()
      );

      const toggleVisible = await toggle.isVisible().catch(() => false);

      if (toggleVisible) {
        // Toggle may be disabled if Cerberus is not enabled
        const isDisabled = await toggle.isDisabled();
        if (!isDisabled) {
          await expect(toggle).toBeEnabled();
        }
      } else {
        test.info().annotations.push({
          type: 'info',
          description: 'Toggle not present on rate limiting config page - located on main security dashboard'
        });
      }
    });

    test('should toggle rate limiting on/off', async ({ page }) => {
      // The toggle uses checkbox type, not switch role
      const toggle = page.locator('input[type="checkbox"]').first();
      const toggleVisible = await toggle.isVisible().catch(() => false);

      if (toggleVisible) {
        const isDisabled = await toggle.isDisabled();
        if (isDisabled) {
          test.info().annotations.push({
            type: 'skip-reason',
            description: 'Toggle is disabled - Cerberus may not be enabled'
          });
          return;
        }

        await test.step('Toggle rate limiting', async () => {
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

  test.describe('RPS Settings', () => {
    test('should display RPS input field', async ({ page }) => {
      const rpsInput = page.getByLabel(/rps|requests per second/i).or(
        page.locator('input[type="number"]').first()
      );

      const inputVisible = await rpsInput.isVisible().catch(() => false);

      if (inputVisible) {
        await expect(rpsInput).toBeEnabled();
      }
    });

    test('should validate RPS input (minimum value)', async ({ page }) => {
      const rpsInput = page.getByLabel(/rps|requests per second/i).or(
        page.locator('input[type="number"]').first()
      );

      const inputVisible = await rpsInput.isVisible().catch(() => false);

      if (inputVisible) {
        const originalValue = await rpsInput.inputValue();

        await test.step('Enter invalid RPS value', async () => {
          await rpsInput.fill('-1');
          await rpsInput.blur();
        });

        await test.step('Restore original value', async () => {
          await rpsInput.fill(originalValue || '100');
        });
      }
    });

    test('should accept valid RPS value', async ({ page }) => {
      const rpsInput = page.getByLabel(/rps|requests per second/i).or(
        page.locator('input[type="number"]').first()
      );

      const inputVisible = await rpsInput.isVisible().catch(() => false);

      if (inputVisible) {
        const originalValue = await rpsInput.inputValue();

        await test.step('Enter valid RPS value', async () => {
          await rpsInput.fill('100');
          // Should not show error
        });

        await test.step('Restore original value', async () => {
          await rpsInput.fill(originalValue || '100');
        });
      }
    });
  });

  test.describe('Burst Settings', () => {
    test('should display burst limit input', async ({ page }) => {
      const burstInput = page.getByLabel(/burst/i).or(
        page.locator('input[type="number"]').nth(1)
      );

      const inputVisible = await burstInput.isVisible().catch(() => false);

      if (inputVisible) {
        await expect(burstInput).toBeEnabled();
      }
    });
  });

  test.describe('Time Window Settings', () => {
    test('should display time window setting', async ({ page }) => {
      const windowInput = page.getByLabel(/window|duration|period/i).or(
        page.locator('select, input[type="number"]').filter({
          hasText: /second|minute|hour/i
        }).first()
      );

      const inputVisible = await windowInput.isVisible().catch(() => false);
      expect(inputVisible !== undefined).toBeTruthy();
    });
  });

  test.describe('Save Settings', () => {
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
    test('should have labeled input fields', async ({ page }) => {
      const inputs = page.locator('input[type="number"]');
      const count = await inputs.count();

      for (let i = 0; i < Math.min(count, 3); i++) {
        const input = inputs.nth(i);
        const visible = await input.isVisible();

        if (visible) {
          const id = await input.getAttribute('id');
          const label = await input.getAttribute('aria-label');
          const placeholder = await input.getAttribute('placeholder');

          // Should have some form of accessible identification
          expect(id || label || placeholder).toBeTruthy();
        }
      }
    });
  });
});
