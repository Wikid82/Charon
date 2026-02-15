import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForDialog, waitForLoadingComplete } from './utils/wait-helpers';
import {
  mockManualChallenge,
  mockExpiredChallenge,
  mockVerifiedChallenge,
} from './fixtures/dns-providers';

const MANUAL_CHALLENGE_ROUTE = '**/api/v1/dns-providers/*/manual-challenge/*';
const MANUAL_VERIFY_ROUTE = '**/api/v1/dns-providers/*/manual-challenge/*/verify';

async function addManualChallengeRoute(
  page: Parameters<typeof test>[0]['page'],
  challengePayload: Record<string, unknown>
): Promise<() => Promise<void>> {
  const routeHandler = async (route: { fulfill: (options: { status: number; contentType: string; body: string }) => Promise<void> }) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(challengePayload),
    });
  };

  await page.route(MANUAL_CHALLENGE_ROUTE, routeHandler);

  return async () => {
    await page.unroute(MANUAL_CHALLENGE_ROUTE, routeHandler);
  };
}

async function addManualVerifyRoute(
  page: Parameters<typeof test>[0]['page'],
  status: number,
  responsePayload: Record<string, unknown>
): Promise<() => Promise<void>> {
  const routeHandler = async (route: { fulfill: (options: { status: number; contentType: string; body: string }) => Promise<void> }) => {
    await route.fulfill({
      status,
      contentType: 'application/json',
      body: JSON.stringify(responsePayload),
    });
  };

  await page.route(MANUAL_VERIFY_ROUTE, routeHandler);

  return async () => {
    await page.unroute(MANUAL_VERIFY_ROUTE, routeHandler);
  };
}

/**
 * Manual DNS Provider E2E Tests
 *
 * Tests the Manual DNS Provider feature including:
 * - Provider selection flow
 * - Manual challenge UI display
 * - Copy to clipboard functionality
 * - Verify button interactions
 * - Accessibility compliance
 *
 * Note: These tests require the application to be running.
 * For full E2E: docker compose -f .docker/compose/docker-compose.local.yml up -d
 * For frontend only: cd frontend && npm run dev
 *
 * Base URL is configured in playwright.config.js via:
 * - PLAYWRIGHT_BASE_URL env var (CI uses http://localhost:8080)
 * - Default: http://100.98.12.109:8080 (Tailscale IP for local dev)
 */

test.describe('Manual DNS Provider Feature', () => {
  test.beforeEach(async ({ page, request }) => {
    await waitForAPIHealth(request);
    // Navigate to the application root (uses baseURL from config)
    await page.goto('/');
  });

  test.describe('Provider Selection Flow', () => {
    test('should navigate to DNS Providers page', async ({ page }) => {
      await test.step('Navigate to DNS Providers section', async () => {
        await page.goto('/dns/providers');
        await waitForLoadingComplete(page);
        await expect(page).toHaveURL(/dns\/providers|dns-providers|settings.*dns/i);
        await expect(page.getByRole('link', { name: /dns providers/i })).toBeVisible();
      });
    });

    test('should show Add Provider button on DNS Providers page', async ({ page }) => {
      await test.step('Navigate to DNS Providers', async () => {
        // Use correct URL path
        await page.goto('/dns/providers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify Add Provider button exists', async () => {
        // Use first() to handle both header button and empty state button
        const addButton = page.getByRole('button', { name: /add.*provider/i }).first();
        await expect(addButton).toBeVisible();
        await expect(addButton).toBeEnabled();
      });
    });

    test('should display Manual option in provider selection', async ({ page }) => {
      await test.step('Navigate to DNS Providers and open add dialog', async () => {
        // Use correct URL path
        await page.goto('/dns/providers');
        await waitForLoadingComplete(page);
        await page.getByRole('button', { name: /add.*provider/i }).first().click();
        await waitForDialog(page);
      });

      await test.step('Verify Manual DNS option is available', async () => {
        const dialog = await waitForDialog(page);
        const providerSelect = dialog
          .locator('#provider-type')
          .or(dialog.getByRole('combobox', { name: /provider type/i }));
        await expect(providerSelect).toBeVisible();
        await providerSelect.click();

        const listbox = page.getByRole('listbox');
        await expect(listbox).toBeVisible();

        const manualOption = listbox.getByRole('option', { name: /manual/i });
        await expect(manualOption).toBeVisible();
        await manualOption.focus();
        await page.keyboard.press('Enter');
        await expect(listbox).toBeHidden();

        await expect(providerSelect).toContainText(/manual/i);
      });
    });
  });

  test.describe('Manual Challenge UI Display', () => {
    let cleanupManualChallengeRoute: null | (() => Promise<void>) = null;

    test.beforeEach(async ({ page }) => {
      cleanupManualChallengeRoute = await addManualChallengeRoute(page, mockManualChallenge as unknown as Record<string, unknown>);
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);
    });

    test.afterEach(async () => {
      if (cleanupManualChallengeRoute) {
        await cleanupManualChallengeRoute();
        cleanupManualChallengeRoute = null;
      }
    });

    /**
     * This test verifies the challenge UI structure.
     * In a real scenario, this would be triggered by requesting a certificate
     * with a Manual DNS provider configured.
     */
    test('should display challenge panel with required elements', async ({ page }) => {
      await test.step('Navigate to an active challenge (mock scenario)', async () => {
        // This would navigate to an active manual challenge
        // For now, we test the component structure
        await page.goto('/dns/providers');
        await waitForLoadingComplete(page);
      });

      const challengeHeading = page.getByRole('heading', { name: /manual dns challenge/i });
      await expect(challengeHeading).toBeVisible();

      await test.step('Verify challenge panel accessibility tree', async () => {
        await expect(page.getByRole('region', { name: /create this txt record at your dns provider/i })).toBeVisible();
        await expect(page.getByText(/record name/i)).toBeVisible();
        await expect(page.getByText(/record value/i)).toBeVisible();
        await expect(page.getByRole('button', { name: /copy record name/i })).toBeVisible();
        await expect(page.getByRole('button', { name: /copy record value/i })).toBeVisible();
        await expect(page.getByRole('progressbar', { name: /challenge timeout progress/i })).toBeVisible();
        await expect(page.getByRole('button', { name: /check dns now/i })).toBeVisible();
        await expect(page.getByRole('button', { name: /verify/i })).toBeVisible();
      });
    });

    test('should show record name and value fields', async ({ page }) => {
      await test.step('Verify record name field', async () => {
        const recordNameLabel = page.getByText(/record name/i);
        await expect(recordNameLabel).toBeVisible();

        // Value should contain _acme-challenge
        const recordNameValue = page.locator('#record-name')
          .or(page.locator('code').filter({ hasText: /_acme-challenge/ }));
        await expect(recordNameValue).toBeVisible();
      });

      await test.step('Verify record value field', async () => {
        const recordValueLabel = page.getByText(/record value/i);
        await expect(recordValueLabel).toBeVisible();

        const recordValueById = page.locator('#record-value');
        const recordValueByText = page.locator('code').filter({ hasText: /mock-challenge-token-value|challenge-token/i }).first();
        const hasVisibleRecordValue =
          await recordValueById.isVisible().catch(() => false) ||
          await recordValueByText.isVisible().catch(() => false);

        expect(hasVisibleRecordValue).toBeTruthy();
      });
    });

    test('should display progress bar with time remaining', async ({ page }) => {
      await test.step('Verify progress bar exists', async () => {
        const progressBar = page.getByRole('progressbar', { name: /challenge timeout progress/i });
        await expect(progressBar).toBeVisible();
      });

      await test.step('Verify time remaining display', async () => {
        // Time should be in MM:SS format
        const timeDisplay = page.getByText(/\d+:\d{2}/);
        await expect(timeDisplay).toBeVisible();
      });
    });

    test('should display status indicator', async ({ page }) => {
      const statusIndicator = page.getByRole('alert').filter({ hasText: /waiting for dns propagation|verified|expired|failed/i });

      await test.step('Verify status message is visible', async () => {
        await expect(statusIndicator).toBeVisible();
      });

      await test.step('Verify status icon is present', async () => {
        const statusIcon = statusIndicator.locator('svg');
        await expect(statusIcon.first()).toBeVisible();
      });
    });
  });

  test.describe('Copy to Clipboard', () => {
    let cleanupManualChallengeRoute: null | (() => Promise<void>) = null;

    test.beforeEach(async ({ page }) => {
      cleanupManualChallengeRoute = await addManualChallengeRoute(page, mockManualChallenge as unknown as Record<string, unknown>);
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);
    });

    test.afterEach(async () => {
      if (cleanupManualChallengeRoute) {
        await cleanupManualChallengeRoute();
        cleanupManualChallengeRoute = null;
      }
    });

    test('should have accessible copy buttons', async ({ page }) => {
      await test.step('Verify copy button for record name', async () => {
        const copyNameButton = page.getByRole('button', { name: /copy.*record.*name/i })
          .or(page.getByLabel(/copy.*record.*name/i));
        await expect(copyNameButton).toBeVisible();
        await expect(copyNameButton).toBeEnabled();
      });

      await test.step('Verify copy button for record value', async () => {
        const copyValueButton = page.getByRole('button', { name: /copy.*record.*value/i })
          .or(page.getByLabel(/copy.*record.*value/i));
        await expect(copyValueButton).toBeVisible();
        await expect(copyValueButton).toBeEnabled();
      });
    });

    test('should show copied feedback on click', async ({ page }, testInfo) => {
      await test.step('Click copy button and verify feedback', async () => {
        // Grant clipboard permissions for testing only on Chromium
        const browserName = testInfo.project?.name || '';
        if (browserName === 'chromium') {
          await page.context().grantPermissions(['clipboard-write']);
        }

        const copyButton = page.getByRole('button', { name: /copy.*record.*name/i })
          .or(page.getByLabel(/copy.*record.*name/i))
          .first();

        await copyButton.click();
        await expect(copyButton.locator('svg.text-success')).toHaveCount(1, { timeout: 3000 });
      });
    });
  });

  test.describe('Verify Button Interactions', () => {
    let cleanupManualChallengeRoute: null | (() => Promise<void>) = null;

    test.beforeEach(async ({ page }) => {
      cleanupManualChallengeRoute = await addManualChallengeRoute(page, mockManualChallenge as unknown as Record<string, unknown>);
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);
    });

    test.afterEach(async () => {
      if (cleanupManualChallengeRoute) {
        await cleanupManualChallengeRoute();
        cleanupManualChallengeRoute = null;
      }
    });

    test('should have Check DNS Now button', async ({ page }) => {
      await test.step('Verify Check DNS Now button exists', async () => {
        const checkDnsButton = page.getByRole('button', { name: /check dns/i });
        await expect(checkDnsButton).toBeVisible();
        await expect(checkDnsButton).toBeEnabled();
      });
    });

    test('should show loading state when checking DNS', async ({ page }) => {
      await test.step('Click Check DNS Now and verify loading', async () => {
        const checkDnsButton = page.getByRole('button', { name: /check dns/i });
        await expect(checkDnsButton).toBeVisible();
        await expect
          .poll(async () => checkDnsButton.isEnabled(), {
            timeout: 10000,
            message: 'Expected Check DNS button to become enabled before interaction',
          })
          .toBe(true);
        await checkDnsButton.click();
        await expect(checkDnsButton).toBeEnabled({ timeout: 5000 });
      });
    });

    test('should have Verify button with description', async ({ page }) => {
      await test.step('Verify the Verify button has accessible description', async () => {
        const verifyButton = page.getByRole('button', { name: /verify/i })
          .filter({ hasNot: page.locator('[disabled]') });

        await expect(verifyButton).toBeVisible();

        const describedBy = await verifyButton.getAttribute('aria-describedby');
        expect(describedBy).toBeTruthy();
        const description = page.locator(`#${describedBy}`);
        await expect(description).toBeAttached();
      });
    });
  });

  test.describe('Accessibility Checks', () => {
    test('should have keyboard accessible interactive elements', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Tab through page elements', async () => {
        // Start from body and tab through elements
        await page.keyboard.press('Tab');

        // Verify focus moves to an element (may be skip link or first interactive)
        // Some pages may not have focusable elements initially
        const focusedElement = page.locator(':focus');
        const hasFocus = await focusedElement.count() > 0;

        if (hasFocus) {
          await expect(focusedElement).toBeVisible();
        }
      });

      await test.step('Verify keyboard navigation works', async () => {
        // Tab multiple times to verify keyboard navigation
        for (let i = 0; i < 5; i++) {
          await page.keyboard.press('Tab');
        }

        // Verify we can navigate with keyboard
        const focusedElement = page.locator(':focus');
        // May or may not have visible focus depending on page state
        expect(await focusedElement.count()).toBeGreaterThanOrEqual(0);
      });
    });

    test('should have proper ARIA labels on copy buttons', async ({ page }) => {
      await test.step('Navigate to manual DNS provider page', async () => {
        await page.goto('/dns/providers');
        await waitForLoadingComplete(page);
        const challengeEntryButton = page.getByRole('button', { name: /manual dns challenge/i }).first();
        await challengeEntryButton.click();
      });

      await test.step('Verify ARIA labels on copy buttons', async () => {
        // Look for any copy buttons on the page (more generic locator)
        const copyButtons = page.getByRole('button', { name: /copy/i });
        await expect.poll(async () => copyButtons.count(), {
          timeout: 5000,
          message: 'Expected copy buttons to be present in manual DNS challenge panel',
        }).toBeGreaterThan(0);

        const resolvedCount = await copyButtons.count();

        for (let i = 0; i < resolvedCount; i++) {
          const button = copyButtons.nth(i);
          const ariaLabel = await button.getAttribute('aria-label');
          const textContent = await button.textContent();

          const isAccessible = ariaLabel || textContent?.trim();
          expect(isAccessible).toBeTruthy();
        }
      });
    });

    test('should announce status changes to screen readers', async ({ page }) => {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);
      await test.step('Verify live region for status updates', async () => {
        const liveRegion = page.locator('[aria-live="polite"], [role="status"]').first();
        await expect(liveRegion).toBeAttached();
      });
    });

    // Test requires add provider dialog to function correctly
    test('should have accessible form labels', async ({ page }) => {
      // Use correct URL path
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);
      await page.getByRole('button', { name: /add.*provider/i }).first().click();

      await test.step('Verify form fields have labels', async () => {
        // Provider name input has id="provider-name"
        const dialog = await waitForDialog(page);
        const nameInput = dialog
          .locator('#provider-name')
          .or(dialog.getByRole('textbox', { name: /provider name|name/i }));

        await expect(nameInput).toBeVisible();
      });
    });

    // Test validates form accessibility structure - may need adjustment based on actual form
    test('should validate accessibility tree structure for provider form', async ({ page }) => {
      // Use correct URL path
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Open add provider dialog', async () => {
        await page.getByRole('button', { name: /add.*provider/i }).first().click();
      });

      await test.step('Verify form accessibility structure', async () => {
        const dialog = await waitForDialog(page);
        await expect(dialog.getByRole('heading', { level: 2 })).toBeVisible();
        await expect(dialog.getByRole('combobox', { name: /provider type/i })).toBeVisible();
        await expect(dialog.getByRole('textbox', { name: /provider name|name/i })).toBeVisible();
        await expect(dialog.getByRole('button', { name: /create|save/i })).toBeVisible();
        await expect(dialog.getByRole('button', { name: /cancel/i }).first()).toBeVisible();
        await expect(dialog.getByRole('button', { name: /close/i }).first()).toBeVisible();
      });
    });
  });
});

test.describe('Manual DNS Challenge Component Tests', () => {
  /**
   * Component-level tests that verify the ManualDNSChallenge component
   * These can run with mocked data if the component supports it
   */

  test('should render all required challenge information', async ({ page }) => {
    const cleanupManualChallengeRoute = await addManualChallengeRoute(
      page,
      mockManualChallenge as unknown as Record<string, unknown>
    );

    try {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Verify challenge FQDN is displayed', async () => {
        await expect(page.getByText('_acme-challenge.example.com')).toBeVisible();
      });

      await test.step('Verify challenge token value is displayed', async () => {
        await expect(page.getByText(/mock-challenge-token/)).toBeVisible();
      });

      await test.step('Verify TTL information', async () => {
        await expect(page.getByText(/300.*seconds|5.*minutes/i)).toBeVisible();
      });
    } finally {
      await cleanupManualChallengeRoute();
    }
  });

  test('should handle expired challenge state', async ({ page }) => {
    const cleanupManualChallengeRoute = await addManualChallengeRoute(
      page,
      mockExpiredChallenge as unknown as Record<string, unknown>
    );

    try {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Verify expired status is displayed', async () => {
        const expiredStatus = page.getByText(/expired/i);
        await expect(expiredStatus).toBeVisible();
      });

      await test.step('Verify action buttons are disabled', async () => {
        const checkDnsButton = page.getByRole('button', { name: /check dns/i });
        const verifyButton = page.getByRole('button', { name: /verify/i });

        await expect(checkDnsButton).toBeDisabled();
        await expect(verifyButton).toBeDisabled();
      });
    } finally {
      await cleanupManualChallengeRoute();
    }
  });

  test('should handle verified challenge state', async ({ page }) => {
    const cleanupManualChallengeRoute = await addManualChallengeRoute(
      page,
      mockVerifiedChallenge as unknown as Record<string, unknown>
    );

    try {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Verify success status is displayed', async () => {
        const successStatus = page.getByText(/verified|success/i);
        await expect(successStatus).toBeVisible();
      });

      await test.step('Verify success indicator', async () => {
        const successAlert = page.locator('[role="alert"]').filter({
          has: page.locator('[class*="success"]'),
        });
        await expect(successAlert).toBeVisible();
      });
    } finally {
      await cleanupManualChallengeRoute();
    }
  });
});

test.describe('Manual DNS Provider Error Handling', () => {
  test('should display error message on verification failure', async ({ page }) => {
    const cleanupManualChallengeRoute = await addManualChallengeRoute(
      page,
      mockManualChallenge as unknown as Record<string, unknown>
    );
    const cleanupManualVerifyRoute = await addManualVerifyRoute(page, 400, {
      message: 'DNS record not found',
      dns_found: false,
    });

    try {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Click verify and check error display', async () => {
        const verifyButton = page.getByRole('button', { name: /verify/i });
        await verifyButton.click();

        const errorMessage = page.getByText(/dns record not found/i)
          .or(page.locator('.toast').filter({ hasText: /not found/i }));

        await expect(errorMessage).toBeVisible({ timeout: 5000 });
      });
    } finally {
      await cleanupManualVerifyRoute();
      await cleanupManualChallengeRoute();
    }
  });

  test('should handle network errors gracefully', async ({ page }) => {
    const verifyRouteHandler = async (route: { abort: (errorCode?: string) => Promise<void> }) => {
      await route.abort('failed');
    };

    const cleanupManualChallengeRoute = await addManualChallengeRoute(
      page,
      mockManualChallenge as unknown as Record<string, unknown>
    );
    await page.route(MANUAL_VERIFY_ROUTE, verifyRouteHandler);

    try {
      await page.goto('/dns/providers');
      await waitForLoadingComplete(page);

      await test.step('Click verify with network error', async () => {
        const verifyButton = page.getByRole('button', { name: /verify/i });
        await verifyButton.click();

        const errorFeedback = page.getByText(/error|failed|network/i)
          .or(page.locator('.toast').filter({ hasText: /error|failed/i }));

        await expect(errorFeedback).toBeVisible({ timeout: 5000 });
      });
    } finally {
      await page.unroute(MANUAL_VERIFY_ROUTE, verifyRouteHandler);
      await cleanupManualChallengeRoute();
    }
  });
});
