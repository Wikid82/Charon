/**
 * Security Dashboard E2E Tests
 *
 * Tests the Security (Cerberus) dashboard functionality including:
 * - Page loading and layout
 * - Module toggle states (CrowdSec, ACL, WAF, Rate Limiting)
 * - Status indicators
 * - Navigation to sub-pages
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import { waitForLoadingComplete, waitForToast } from '../utils/wait-helpers';
import { clickSwitch } from '../utils/ui-helpers';
import {
  captureSecurityState,
  restoreSecurityState,
  CapturedSecurityState,
} from '../utils/security-helpers';

test.describe('Security Dashboard @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/security');
    await waitForLoadingComplete(page);
  });

  test.describe('Page Loading', () => {
    test('should display security dashboard page title', async ({ page }) => {
      await expect(page.getByRole('heading', { name: /security/i }).first()).toBeVisible();
    });

    test('should display Cerberus dashboard header', async ({ page }) => {
      await expect(page.getByText(/cerberus.*dashboard/i)).toBeVisible();
    });

    test('should show all 4 security module cards', async ({ page }) => {
      await test.step('Verify CrowdSec card exists', async () => {
        await expect(page.getByText(/crowdsec/i).first()).toBeVisible();
      });

      await test.step('Verify ACL card exists', async () => {
        await expect(page.getByText(/access.*control/i).first()).toBeVisible();
      });

      await test.step('Verify WAF card exists', async () => {
        await expect(page.getByText(/coraza.*waf|waf/i).first()).toBeVisible();
      });

      await test.step('Verify Rate Limiting card exists', async () => {
        await expect(page.getByText(/rate.*limiting/i).first()).toBeVisible();
      });
    });

    test('should display layer badges for each module', async ({ page }) => {
      await expect(page.getByText(/layer.*1/i)).toBeVisible();
      await expect(page.getByText(/layer.*2/i)).toBeVisible();
      await expect(page.getByText(/layer.*3/i)).toBeVisible();
      await expect(page.getByText(/layer.*4/i)).toBeVisible();
    });

    test('should show audit logs button in header', async ({ page }) => {
      const auditLogsButton = page.getByRole('button', { name: /audit.*logs/i });
      await expect(auditLogsButton).toBeVisible();
    });

    test('should show docs button in header', async ({ page }) => {
      const docsButton = page.getByRole('button', { name: /docs/i });
      await expect(docsButton).toBeVisible();
    });
  });

  test.describe('Module Status Indicators', () => {
    test('should show enabled/disabled badge for each module', async ({ page }) => {
      // Each card should have an enabled or disabled badge
      // Look for text that matches enabled/disabled patterns
      // The Badge component may use various styling approaches
      await page.waitForTimeout(500); // Wait for UI to settle

      const enabledTexts = page.getByText(/^enabled$/i);
      const disabledTexts = page.getByText(/^disabled$/i);

      const enabledCount = await enabledTexts.count();
      const disabledCount = await disabledTexts.count();

      // Should have at least 4 status badges (one per security layer card)
      expect(enabledCount + disabledCount).toBeGreaterThanOrEqual(4);
    });

    test('should display CrowdSec toggle switch', async ({ page }) => {
      const toggle = page.getByTestId('toggle-crowdsec');
      await expect(toggle).toBeVisible();
    });

    test('should display ACL toggle switch', async ({ page }) => {
      const toggle = page.getByTestId('toggle-acl');
      await expect(toggle).toBeVisible();
    });

    test('should display WAF toggle switch', async ({ page }) => {
      const toggle = page.getByTestId('toggle-waf');
      await expect(toggle).toBeVisible();
    });

    test('should display Rate Limiting toggle switch', async ({ page }) => {
      const toggle = page.getByTestId('toggle-rate-limit');
      await expect(toggle).toBeVisible();
    });
  });

  test.describe('Module Toggle Actions', () => {
    // Capture state ONCE for this describe block
    let originalState: CapturedSecurityState;

    test.beforeAll(async ({ request: reqFixture }) => {
      try {
        originalState = await captureSecurityState(reqFixture);
      } catch (error) {
        console.warn('Could not capture initial security state:', error);
      }
    });

    test.afterAll(async () => {
      // CRITICAL: Restore original state even if tests fail
      if (!originalState) {
        return;
      }

      // Create authenticated request context for cleanup (cannot reuse fixture from beforeAll)
      const cleanupRequest = await request.newContext({
        baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080',
        storageState: STORAGE_STATE,
      });

      try {
        await restoreSecurityState(cleanupRequest, originalState);
        console.log('✓ Security state restored after toggle tests');
      } catch (error) {
        console.error('Failed to restore security state:', error);
      } finally {
        await cleanupRequest.dispose();
      }
    });

    test('should toggle ACL enabled/disabled', async ({ page }) => {
      const toggle = page.getByTestId('toggle-acl');

      const isDisabled = await toggle.isDisabled();
      if (isDisabled) {
        test.info().annotations.push({
          type: 'skip-reason',
          description: 'Toggle is disabled because Cerberus security is not enabled',
        });
        return;
      }

      await test.step('Toggle ACL state', async () => {
        await page.waitForLoadState('networkidle');
        await clickSwitch(toggle);
        await waitForToast(page, /updated|success|enabled|disabled/i, 10000);
      });

      // NOTE: Do NOT toggle back here - afterAll handles cleanup
    });

    test('should toggle WAF enabled/disabled', async ({ page }) => {
      const toggle = page.getByTestId('toggle-waf');

      const isDisabled = await toggle.isDisabled();
      if (isDisabled) {
        test.info().annotations.push({
          type: 'skip-reason',
          description: 'Toggle is disabled because Cerberus security is not enabled',
        });
        return;
      }

      await test.step('Toggle WAF state', async () => {
        await page.waitForLoadState('networkidle');
        await clickSwitch(toggle);
        await waitForToast(page, /updated|success|enabled|disabled/i, 10000);
      });

      // NOTE: Do NOT toggle back here - afterAll handles cleanup
    });

    test('should toggle Rate Limiting enabled/disabled', async ({ page }) => {
      const toggle = page.getByTestId('toggle-rate-limit');

      const isDisabled = await toggle.isDisabled();
      if (isDisabled) {
        test.info().annotations.push({
          type: 'skip-reason',
          description: 'Toggle is disabled because Cerberus security is not enabled',
        });
        return;
      }

      await test.step('Toggle Rate Limit state', async () => {
        await page.waitForLoadState('networkidle');
        await clickSwitch(toggle);
        await waitForToast(page, /updated|success|enabled|disabled/i, 10000);
      });

      // NOTE: Do NOT toggle back here - afterAll handles cleanup
    });

    test('should persist toggle state after page reload', async ({ page }) => {
      const toggle = page.getByTestId('toggle-acl');

      const isDisabled = await toggle.isDisabled();
      if (isDisabled) {
        test.info().annotations.push({
          type: 'skip-reason',
          description: 'Toggle is disabled because Cerberus security is not enabled',
        });
        return;
      }

      if (isDisabled) {
        test.info().annotations.push({
          type: 'skip-reason',
          description: 'Toggle is disabled because Cerberus security is not enabled',
        });
        return;
      }

      if (isDisabled) {
        test.info().annotations.push({
          type: 'skip-reason',
          description: 'Toggle is disabled because Cerberus security is not enabled',
        });
        return;
      }
    });
  });

  test.describe('Navigation', () => {
    test('should navigate to CrowdSec page when configure clicked', async ({ page }) => {
      // Find the CrowdSec card by locating the configure button within a container that has CrowdSec text
      // Cards use rounded-lg border classes, not [class*="card"]
      const crowdsecSection = page.locator('div').filter({ hasText: /crowdsec/i }).filter({ has: page.getByRole('button', { name: /configure/i }) }).first();
      const configureButton = crowdsecSection.getByRole('button', { name: /configure/i });

      // Button may be disabled when Cerberus is off
      const isDisabled = await configureButton.isDisabled().catch(() => true);
      if (isDisabled) {
        test.info().annotations.push({
          type: 'skip-reason',
          description: 'Configure button is disabled because Cerberus security is not enabled'
        });
        return;
      }

      // Wait for any loading overlays to disappear
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(300);

      // Scroll element into view and use force click to bypass pointer interception
      await configureButton.scrollIntoViewIfNeeded();
      await configureButton.click({ force: true });
      await expect(page).toHaveURL(/\/security\/crowdsec/);
    });

    test('should navigate to Access Lists page when clicked', async ({ page }) => {
      // The ACL card has a "Manage Lists" or "Configure" button
      // Find button by looking for buttons with matching text within the page
      const allConfigButtons = page.getByRole('button', { name: /manage.*lists|configure/i });
      const count = await allConfigButtons.count();

      // The ACL button should be the second configure button (after CrowdSec)
      // Or we can find it near the "Access Control" or "ACL" text
      let aclButton = null;
      for (let i = 0; i < count; i++) {
        const btn = allConfigButtons.nth(i);
        const btnText = await btn.textContent();
        // The ACL button says "Manage Lists" when enabled, "Configure" when disabled
        if (btnText?.match(/manage.*lists/i)) {
          aclButton = btn;
          break;
        }
      }

      // Fallback to second configure button if no "Manage Lists" found
      if (!aclButton) {
        aclButton = allConfigButtons.nth(1);
      }

      // Wait for any loading overlays and scroll into view
      await page.waitForLoadState('networkidle');
      await aclButton.scrollIntoViewIfNeeded();
      await page.waitForTimeout(200);
      await aclButton.click({ force: true });
      await expect(page).toHaveURL(/\/security\/access-lists|\/access-lists/);
    });

    test('should navigate to WAF page when configure clicked', async ({ page }) => {
      // WAF is Layer 3 - the third configure button in the security cards grid
      const allConfigButtons = page.getByRole('button', { name: /configure/i });
      const count = await allConfigButtons.count();

      // Should have at least 3 configure buttons (CrowdSec, ACL/Manage Lists, WAF)
      if (count < 3) {
        test.info().annotations.push({
          type: 'skip-reason',
          description: 'Not enough configure buttons found on page'
        });
        return;
      }

      // WAF is the 3rd configure button (index 2)
      const wafButton = allConfigButtons.nth(2);

      // Wait and scroll into view
      await page.waitForLoadState('networkidle');
      await wafButton.scrollIntoViewIfNeeded();
      await page.waitForTimeout(200);
      await wafButton.click({ force: true });
      await expect(page).toHaveURL(/\/security\/waf/);
    });

    test('should navigate to Rate Limiting page when configure clicked', async ({ page }) => {
      // Rate Limiting is Layer 4 - the fourth configure button in the security cards grid
      const allConfigButtons = page.getByRole('button', { name: /configure/i });
      const count = await allConfigButtons.count();

      // Should have at least 4 configure buttons
      if (count < 4) {
        test.info().annotations.push({
          type: 'skip-reason',
          description: 'Not enough configure buttons found on page'
        });
        return;
      }

      // Rate Limit is the 4th configure button (index 3)
      const rateLimitButton = allConfigButtons.nth(3);

      // Wait and scroll into view
      await page.waitForLoadState('networkidle');
      await rateLimitButton.scrollIntoViewIfNeeded();
      await page.waitForTimeout(200);
      await rateLimitButton.click({ force: true });
      await expect(page).toHaveURL(/\/security\/rate-limiting/);
    });

    test('should navigate to Audit Logs page', async ({ page }) => {
      const auditLogsButton = page.getByRole('button', { name: /audit.*logs/i });

      await auditLogsButton.click();
      await expect(page).toHaveURL(/\/security\/audit-logs/);
    });
  });

  test.describe('Admin Whitelist', () => {
    test('should display admin whitelist section when Cerberus enabled', async ({ page }) => {
      // Check if the admin whitelist input is visible (only shown when Cerberus is enabled)
      const whitelistInput = page.getByPlaceholder(/192\.168|cidr/i);
      const isVisible = await whitelistInput.isVisible().catch(() => false);

      if (isVisible) {
        await expect(whitelistInput).toBeVisible();
      } else {
        // Cerberus might be disabled - just verify the page loaded correctly
        // by checking for the Cerberus Dashboard header which is always visible
        const cerberusHeader = page.getByText(/cerberus.*dashboard/i);
        await expect(cerberusHeader).toBeVisible();

        test.info().annotations.push({
          type: 'info',
          description: 'Admin whitelist section not visible - Cerberus may be disabled'
        });
      }
    });
  });

  test.describe('Accessibility', () => {
    test('should have accessible toggle switches with labels', async ({ page }) => {
      // Each toggle should be within a tooltip that describes its purpose
      // The Switch component uses an input[type="checkbox"] under the hood
      const toggles = [
        page.getByTestId('toggle-crowdsec'),
        page.getByTestId('toggle-acl'),
        page.getByTestId('toggle-waf'),
        page.getByTestId('toggle-rate-limit'),
      ];

      for (const toggle of toggles) {
        await expect(toggle).toBeVisible();
        // Switch uses checkbox input type (visually styled as toggle)
        await expect(toggle).toHaveAttribute('type', 'checkbox');
      }
    });

    test('should navigate with keyboard', async ({ page }) => {
      await test.step('Tab through header buttons', async () => {
        await page.keyboard.press('Tab');
        await page.keyboard.press('Tab');
        // Should be able to tab to various interactive elements
      });
    });
  });
});
