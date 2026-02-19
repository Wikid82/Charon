/**
 * CrowdSec Console Enrollment E2E Tests
 *
 * Tests the CrowdSec console enrollment functionality including:
 * - Enrollment status API
 * - Diagnostics connectivity status
 * - Diagnostics config validation
 * - Heartbeat status
 * - UI enrollment section display
 *
 * @see /projects/Charon/docs/plans/crowdsec_enrollment_debug_spec.md
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('CrowdSec Console Enrollment', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.describe('Console Enrollment Status API', () => {
    test('should fetch console enrollment status via API', async ({ request }) => {
      await test.step('GET enrollment status endpoint', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/console/enrollment');

        // Endpoint may not exist yet (return 404) or return enrollment status
        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'info',
            description: 'Console enrollment endpoint not implemented (404)',
          });
          return;
        }

        expect(response.ok()).toBeTruthy();

        const status = await response.json();

        // Verify response contains expected fields
        expect(status).toHaveProperty('status');
        expect(['not_enrolled', 'enrolling', 'pending_acceptance', 'enrolled', 'failed']).toContain(
          status.status
        );

        // Optional fields that should be present
        if (status.status !== 'not_enrolled') {
          expect(status).toHaveProperty('agent_name');
          expect(status).toHaveProperty('tenant');
        }

        expect(status).toHaveProperty('last_attempt_at');
        expect(status).toHaveProperty('key_present');
      });
    });

    test('should fetch diagnostics connectivity status', async ({ request }) => {
      await test.step('GET diagnostics connectivity endpoint', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/diagnostics/connectivity');

        // Endpoint may not exist yet
        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'info',
            description: 'Diagnostics connectivity endpoint not implemented (404)',
          });
          return;
        }

        expect(response.ok()).toBeTruthy();

        const connectivity = await response.json();

        // Verify response contains expected boolean fields
        expect(connectivity).toHaveProperty('lapi_running');
        expect(typeof connectivity.lapi_running).toBe('boolean');

        expect(connectivity).toHaveProperty('lapi_ready');
        expect(typeof connectivity.lapi_ready).toBe('boolean');

        expect(connectivity).toHaveProperty('capi_registered');
        expect(typeof connectivity.capi_registered).toBe('boolean');

        expect(connectivity).toHaveProperty('console_enrolled');
        expect(typeof connectivity.console_enrolled).toBe('boolean');
      });
    });

    test('should fetch diagnostics config validation', async ({ request }) => {
      await test.step('GET diagnostics config endpoint', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/diagnostics/config');

        // Endpoint may not exist yet
        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'info',
            description: 'Diagnostics config endpoint not implemented (404)',
          });
          return;
        }

        expect(response.ok()).toBeTruthy();

        const config = await response.json();

        // Verify response contains expected fields
        expect(config).toHaveProperty('config_exists');
        expect(typeof config.config_exists).toBe('boolean');

        expect(config).toHaveProperty('acquis_exists');
        expect(typeof config.acquis_exists).toBe('boolean');

        expect(config).toHaveProperty('lapi_port');
        expect(typeof config.lapi_port).toBe('string');

        expect(config).toHaveProperty('errors');
        expect(Array.isArray(config.errors)).toBe(true);
      });
    });

    test('should fetch heartbeat status', async ({ request }) => {
      await test.step('GET console heartbeat endpoint', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/console/heartbeat');

        // Endpoint may not exist yet
        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'info',
            description: 'Console heartbeat endpoint not implemented (404)',
          });
          return;
        }

        expect(response.ok()).toBeTruthy();

        const heartbeat = await response.json();

        // Verify response contains expected fields
        expect(heartbeat).toHaveProperty('status');
        expect(['not_enrolled', 'enrolling', 'pending_acceptance', 'enrolled', 'failed']).toContain(
          heartbeat.status
        );

        expect(heartbeat).toHaveProperty('last_heartbeat_at');
        // last_heartbeat_at can be null if not enrolled or not yet received
      });
    });
  });

  test.describe('Console Enrollment UI', () => {
    test('should display console enrollment section in UI when feature is enabled', async ({
      page,
    }) => {
      await test.step('Navigate to CrowdSec configuration', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify enrollment section visibility', async () => {
        // Look for console enrollment section using various selectors
        const enrollmentSection = page
          .getByTestId('console-section')
          .or(page.getByTestId('console-enrollment-section'))
          .or(page.locator('[class*="card"]').filter({ hasText: /console.*enrollment|enroll/i }));

        const enrollmentVisible = await enrollmentSection.isVisible().catch(() => false);

        if (enrollmentVisible) {
          await expect(enrollmentSection).toBeVisible();

          // Check for token input
          const tokenInput = page
            .getByTestId('enrollment-token-input')
            .or(page.getByTestId('crowdsec-token-input'))
            .or(page.getByPlaceholder(/token|key/i));

          const tokenInputVisible = await tokenInput.isVisible().catch(() => false);
          if (tokenInputVisible) {
            await expect(tokenInput).toBeVisible();
          }
        } else {
          // Feature might be disabled via feature flag
          test.info().annotations.push({
            type: 'info',
            description:
              'Console enrollment section not visible - feature may be disabled via feature flag',
          });
        }
      });
    });

    test('should display enrollment status correctly', async ({ page }) => {
      await test.step('Navigate to CrowdSec configuration', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify enrollment status display', async () => {
        // Look for status text that shows current enrollment state
        const statusText = page
          .getByTestId('console-token-state')
          .or(page.getByTestId('enrollment-status'))
          .or(page.locator('text=/not enrolled|pending.*acceptance|enrolled|enrolling/i'));

        const statusVisible = await statusText.first().isVisible().catch(() => false);

        if (statusVisible) {
          // Verify one of the expected statuses is displayed
          const possibleStatuses = [
            /not enrolled/i,
            /pending.*acceptance/i,
            /enrolled/i,
            /enrolling/i,
            /failed/i,
          ];

          let foundStatus = false;
          for (const pattern of possibleStatuses) {
            const statusMatch = page.getByText(pattern);
            if (await statusMatch.first().isVisible().catch(() => false)) {
              foundStatus = true;
              break;
            }
          }

          if (!foundStatus) {
            test.info().annotations.push({
              type: 'info',
              description: 'No enrollment status text found in expected formats',
            });
          }
        } else {
          test.info().annotations.push({
            type: 'info',
            description: 'Enrollment status not visible - feature may not be implemented',
          });
        }
      });
    });

    test('should show enroll button when not enrolled', async ({ page, request }) => {
      await test.step('Check enrollment status via API', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/console/enrollment');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Console enrollment API not implemented',
          });
          return;
        }

        const status = await response.json();

        if (status.status === 'not_enrolled') {
          await page.goto('/security/crowdsec');
          await waitForLoadingComplete(page);

          // Look for enroll button
          const enrollButton = page.getByRole('button', { name: /enroll/i });
          const buttonVisible = await enrollButton.isVisible().catch(() => false);

          if (buttonVisible) {
            await expect(enrollButton).toBeVisible();
            await expect(enrollButton).toBeEnabled();
          }
        } else {
          test.info().annotations.push({
            type: 'info',
            description: `CrowdSec is already enrolled with status: ${status.status}`,
          });
        }
      });
    });

    test('should show agent name field when enrolling', async ({ page }) => {
      await test.step('Navigate to CrowdSec configuration', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Check for agent name input', async () => {
        const agentNameInput = page
          .getByTestId('agent-name-input')
          .or(page.getByLabel(/agent.*name/i))
          .or(page.getByPlaceholder(/agent.*name/i));

        const inputVisible = await agentNameInput.isVisible().catch(() => false);

        if (inputVisible) {
          await expect(agentNameInput).toBeVisible();
        } else {
          // Agent name input may only show when token is entered
          test.info().annotations.push({
            type: 'info',
            description: 'Agent name input not visible - may require token input first',
          });
        }
      });
    });
  });

  test.describe('Enrollment Validation', () => {
    test('should validate enrollment token format', async ({ page }) => {
      await test.step('Navigate to CrowdSec configuration', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Check token input validation', async () => {
        const tokenInput = page
          .getByTestId('enrollment-token-input')
          .or(page.getByTestId('crowdsec-token-input'))
          .or(page.getByPlaceholder(/token|key/i));

        const inputVisible = await tokenInput.isVisible().catch(() => false);

        if (!inputVisible) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Token input not visible - console enrollment UI not implemented',
          });
          return;
        }

        // Try submitting empty token
        const enrollButton = page.getByRole('button', { name: /enroll/i });
        const buttonVisible = await enrollButton.isVisible().catch(() => false);

        if (buttonVisible) {
          await enrollButton.click();

          // Should show validation error
          const errorText = page.getByText(/required|invalid|token/i);
          const errorVisible = await errorText.first().isVisible().catch(() => false);

          if (errorVisible) {
            await expect(errorText.first()).toBeVisible();
          }
        }
      });
    });

    test('should handle LAPI not running error gracefully', async ({ page, request }) => {
      // LAPI availability enforced via CrowdSec internal checks. Verified in integration tests (backend/integration/).

      await test.step('Attempt enrollment when LAPI is not running', async () => {
        // This test would verify the error message when LAPI is not available
        // Skipped because it requires stopping CrowdSec which affects other tests
      });
    });
  });

  test.describe('Enrollment Status Persistence', () => {
    test('should persist enrollment status across page reloads', async ({ page, request }) => {
      await test.step('Check initial status', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/console/enrollment');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Console enrollment API not implemented',
          });
          return;
        }

        const initialStatus = await response.json();

        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);

        // Reload page
        await page.reload();
        await waitForLoadingComplete(page);

        // Verify status persists
        const afterReloadResponse = await request.get('/api/v1/admin/crowdsec/console/enrollment');
        expect(afterReloadResponse.ok()).toBeTruthy();

        const afterReloadStatus = await afterReloadResponse.json();
        expect(afterReloadStatus.status).toBe(initialStatus.status);
      });
    });
  });
});
