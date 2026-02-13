import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { clickSwitch } from '../utils/ui-helpers';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const TEST_RUNNER_WHITELIST = '127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16';

type SecurityStatusResponse = {
  cerberus?: { enabled?: boolean };
  crowdsec?: { enabled?: boolean };
  acl?: { enabled?: boolean };
  waf?: { enabled?: boolean };
  rate_limit?: { enabled?: boolean };
};

async function emergencyReset(page: import('@playwright/test').Page): Promise<void> {
  const emergencyToken = process.env.CHARON_EMERGENCY_TOKEN;
  if (!emergencyToken) {
    return;
  }

  const username = process.env.CHARON_EMERGENCY_USERNAME || 'admin';
  const password = process.env.CHARON_EMERGENCY_PASSWORD || 'changeme';
  const basicAuth = `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`;

  const response = await page.request.post('http://localhost:2020/emergency/security-reset', {
    headers: {
      Authorization: basicAuth,
      'X-Emergency-Token': emergencyToken,
      'Content-Type': 'application/json',
    },
    data: { reason: 'security-dashboard deterministic precondition reset' },
  });

  expect(response.ok()).toBe(true);
}

async function patchWithRetry(
  page: import('@playwright/test').Page,
  url: string,
  data: Record<string, unknown>
): Promise<void> {
  const maxRetries = 5;

  for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
    const response = await page.request.patch(url, { data });
    if (response.ok()) {
      return;
    }

    if (response.status() !== 429 || attempt === maxRetries) {
      throw new Error(`PATCH ${url} failed: ${response.status()} ${await response.text()}`);
    }
  }
}

async function ensureSecurityDashboardPreconditions(
  page: import('@playwright/test').Page
): Promise<void> {
  await emergencyReset(page);

  await patchWithRetry(page, '/api/v1/settings', {
    key: 'feature.cerberus.enabled',
    value: 'true',
  });

  await patchWithRetry(page, '/api/v1/config', {
    security: { admin_whitelist: TEST_RUNNER_WHITELIST },
  });

  await expect.poll(async () => {
    const statusResponse = await page.request.get('/api/v1/security/status');
    if (!statusResponse.ok()) {
      return false;
    }

    const status = (await statusResponse.json()) as SecurityStatusResponse;
    return Boolean(status?.cerberus?.enabled);
  }, {
    timeout: 15000,
    message: 'Expected Cerberus to be enabled before security dashboard assertions',
  }).toBe(true);
}

async function readSecurityStatus(page: import('@playwright/test').Page): Promise<SecurityStatusResponse> {
  const response = await page.request.get('/api/v1/security/status');
  expect(response.ok()).toBe(true);
  return response.json();
}

test.describe('Security Dashboard @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await ensureSecurityDashboardPreconditions(page);
    await page.goto('/security');
    await waitForLoadingComplete(page);
  });

  test('loads dashboard with all module toggles', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /security/i }).first()).toBeVisible();
    await expect(page.getByText(/cerberus.*dashboard/i)).toBeVisible();
    await expect(page.getByTestId('toggle-crowdsec')).toBeVisible();
    await expect(page.getByTestId('toggle-acl')).toBeVisible();
    await expect(page.getByTestId('toggle-waf')).toBeVisible();
    await expect(page.getByTestId('toggle-rate-limit')).toBeVisible();
  });

  test('toggles ACL and persists state', async ({ page }) => {
    const toggle = page.getByTestId('toggle-acl');
    await expect(toggle).toBeEnabled({ timeout: 10000 });
    const initialChecked = await toggle.isChecked();

    await clickSwitch(toggle);

    await expect.poll(async () => {
      const status = await readSecurityStatus(page);
      return Boolean(status?.acl?.enabled);
    }, {
      timeout: 15000,
      message: 'Expected ACL state to change after toggle',
    }).toBe(!initialChecked);
  });

  test('toggles WAF and persists state', async ({ page }) => {
    const toggle = page.getByTestId('toggle-waf');
    await expect(toggle).toBeEnabled({ timeout: 10000 });
    const initialChecked = await toggle.isChecked();

    await clickSwitch(toggle);

    await expect.poll(async () => {
      const status = await readSecurityStatus(page);
      return Boolean(status?.waf?.enabled);
    }, {
      timeout: 15000,
      message: 'Expected WAF state to change after toggle',
    }).toBe(!initialChecked);
  });

  test('toggles Rate Limiting and persists state', async ({ page }) => {
    const toggle = page.getByTestId('toggle-rate-limit');
    await expect(toggle).toBeEnabled({ timeout: 10000 });
    const initialChecked = await toggle.isChecked();

    await clickSwitch(toggle);

    await expect.poll(async () => {
      const status = await readSecurityStatus(page);
      return Boolean(status?.rate_limit?.enabled);
    }, {
      timeout: 15000,
      message: 'Expected rate limit state to change after toggle',
    }).toBe(!initialChecked);
  });

  test('navigates to security sub-pages from dashboard actions', async ({ page }) => {
    const configureButtons = page.getByRole('button', { name: /configure|manage.*lists/i });
    await expect(configureButtons).toHaveCount(4);

    await configureButtons.first().click({ force: true });
    await expect(page).toHaveURL(/\/security\/crowdsec/);

    await page.goto('/security');
    await waitForLoadingComplete(page);
    await page.getByRole('button', { name: /manage.*lists|configure/i }).nth(1).click({ force: true });
    await expect(page).toHaveURL(/\/security\/access-lists|\/access-lists/);

    await page.goto('/security');
    await waitForLoadingComplete(page);
    await page.getByRole('button', { name: /configure/i }).nth(1).click({ force: true });
    await expect(page).toHaveURL(/\/security\/waf/);

    await page.goto('/security');
    await waitForLoadingComplete(page);
    await page.getByRole('button', { name: /configure/i }).nth(2).click({ force: true });
    await expect(page).toHaveURL(/\/security\/rate-limiting/);
  });

  test('opens audit logs from dashboard header', async ({ page }) => {
    const auditLogsButton = page.getByRole('button', { name: /audit.*logs/i });
    await expect(auditLogsButton).toBeVisible();
    await auditLogsButton.click();
    await expect(page).toHaveURL(/\/security\/audit-logs/);
  });

  test('shows admin whitelist controls and emergency token button', async ({ page }) => {
    await expect(page.getByPlaceholder(/192\.168|cidr/i)).toBeVisible({ timeout: 10000 });
    const generateButton = page.getByRole('button', { name: /generate.*token/i });
    await expect(generateButton).toBeVisible();
    await expect(generateButton).toBeEnabled();
  });

  test('exposes keyboard-navigable checkbox toggles', async ({ page }) => {
    const toggles = [
      page.getByTestId('toggle-crowdsec'),
      page.getByTestId('toggle-acl'),
      page.getByTestId('toggle-waf'),
      page.getByTestId('toggle-rate-limit'),
    ];

    for (const toggle of toggles) {
      await expect(toggle).toBeVisible();
      await expect(toggle).toHaveAttribute('type', 'checkbox');
    }

    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
  });
});
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
import { STORAGE_STATE } from '../constants';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { clickSwitch } from '../utils/ui-helpers';
import {
  captureSecurityState,
  restoreSecurityState,
  CapturedSecurityState,
} from '../utils/security-helpers';

const TEST_RUNNER_WHITELIST = '127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16';

async function patchWithRetry(
  page: import('@playwright/test').Page,
  url: string,
  data: Record<string, unknown>
): Promise<void> {
  const maxRetries = 5;
  const retryDelayMs = 1000;

  for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
    const response = await page.request.patch(url, { data });
    if (response.ok()) {
      return;
    }

    if (response.status() !== 429 || attempt === maxRetries) {
      throw new Error(`PATCH ${url} failed: ${response.status()} ${await response.text()}`);
    }

    await page.waitForTimeout(retryDelayMs);
  }
}

async function ensureSecurityDashboardPreconditions(
  page: import('@playwright/test').Page
): Promise<void> {
  const emergencyToken = process.env.CHARON_EMERGENCY_TOKEN;
  if (emergencyToken) {
    const username = process.env.CHARON_EMERGENCY_USERNAME || 'admin';
    const password = process.env.CHARON_EMERGENCY_PASSWORD || 'changeme';
    const basicAuth = `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`;
    await page.request.post('http://localhost:2020/emergency/security-reset', {
      headers: {
        Authorization: basicAuth,
        'X-Emergency-Token': emergencyToken,
        'Content-Type': 'application/json',
      },
      data: { reason: 'security-dashboard deterministic precondition reset' },
    });
  }

  await patchWithRetry(page, '/api/v1/config', {
    security: { admin_whitelist: TEST_RUNNER_WHITELIST },
  });

  await patchWithRetry(page, '/api/v1/settings', {
    key: 'feature.cerberus.enabled',
    value: 'true',
  });

  await expect.poll(async () => {
    const statusResponse = await page.request.get('/api/v1/security/status');
    if (!statusResponse.ok()) {
      return false;
    }
    const status = await statusResponse.json();
    return Boolean(status?.cerberus?.enabled);
  }, {
    timeout: 15000,
    message: 'Expected Cerberus to be enabled before running security dashboard assertions',
  }).toBe(true);
}

test.describe('Security Dashboard @security', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await ensureSecurityDashboardPreconditions(page);
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
      const toggles = [
        page.getByTestId('toggle-crowdsec'),
        page.getByTestId('toggle-acl'),
        page.getByTestId('toggle-waf'),
        page.getByTestId('toggle-rate-limit'),
      ];

      for (const toggle of toggles) {
        await expect(toggle).toBeVisible();
        expect(typeof (await toggle.isChecked())).toBe('boolean');
      }
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
        baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',
        storageState: STORAGE_STATE,
      });

      try {
        await restoreSecurityState(cleanupRequest, originalState);
        console.log('✓ Security state restored after toggle tests');
      } catch (error) {
        console.error('Failed to restore security state:', error);
        const emergencyToken = process.env.CHARON_EMERGENCY_TOKEN;
        if (emergencyToken) {
          const username = process.env.CHARON_EMERGENCY_USERNAME || 'admin';
          const password = process.env.CHARON_EMERGENCY_PASSWORD || 'changeme';
          const basicAuth = `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`;
          await cleanupRequest.post('http://localhost:2020/emergency/security-reset', {
            headers: {
              Authorization: basicAuth,
              'X-Emergency-Token': emergencyToken,
              'Content-Type': 'application/json',
            },
            data: { reason: 'security-dashboard cleanup fallback' },
          });
        }
      } finally {
        await cleanupRequest.dispose();
      }
    });

    test('should toggle ACL enabled/disabled', async ({ page }) => {
      const toggle = page.getByTestId('toggle-acl');
      await expect(toggle).toBeEnabled({ timeout: 10000 });
      const initialChecked = await toggle.isChecked();

      await test.step('Toggle ACL state', async () => {
        await page.waitForLoadState('networkidle');
        await clickSwitch(toggle);
        await expect.poll(async () => {
          const statusResponse = await page.request.get('/api/v1/security/status');
          if (!statusResponse.ok()) {
            return initialChecked;
          }
          const status = await statusResponse.json();
          return Boolean(status?.acl?.enabled);
        }, {
          timeout: 15000,
          message: 'Expected ACL state to change after toggle',
        }).toBe(!initialChecked);
      });

      // NOTE: Do NOT toggle back here - afterAll handles cleanup
    });

    test('should toggle WAF enabled/disabled', async ({ page }) => {
      const toggle = page.getByTestId('toggle-waf');
      await expect(toggle).toBeEnabled({ timeout: 10000 });
      const initialChecked = await toggle.isChecked();

      await test.step('Toggle WAF state', async () => {
        await page.waitForLoadState('networkidle');
        await clickSwitch(toggle);
        await expect.poll(async () => {
          const statusResponse = await page.request.get('/api/v1/security/status');
          if (!statusResponse.ok()) {
            return initialChecked;
          }
          const status = await statusResponse.json();
          return Boolean(status?.waf?.enabled);
        }, {
          timeout: 15000,
          message: 'Expected WAF state to change after toggle',
        }).toBe(!initialChecked);
      });

      // NOTE: Do NOT toggle back here - afterAll handles cleanup
    });

    test('should toggle Rate Limiting enabled/disabled', async ({ page }) => {
      const toggle = page.getByTestId('toggle-rate-limit');
      await expect(toggle).toBeEnabled({ timeout: 10000 });
      const initialChecked = await toggle.isChecked();

      await test.step('Toggle Rate Limit state', async () => {
        await page.waitForLoadState('networkidle');
        await clickSwitch(toggle);
        await expect.poll(async () => {
          const statusResponse = await page.request.get('/api/v1/security/status');
          if (!statusResponse.ok()) {
            return initialChecked;
          }
          const status = await statusResponse.json();
          return Boolean(status?.rate_limit?.enabled);
        }, {
          timeout: 15000,
          message: 'Expected rate limit state to change after toggle',
        }).toBe(!initialChecked);
      });

      // NOTE: Do NOT toggle back here - afterAll handles cleanup
    });

    test('should persist toggle state after page reload', async ({ page }) => {
      const toggle = page.getByTestId('toggle-acl');
      await expect(toggle).toBeEnabled({ timeout: 10000 });
      const initialChecked = await toggle.isChecked();

      await clickSwitch(toggle);
      await waitForToast(page, /updated|success|enabled|disabled/i, 10000);
      await page.reload({ waitUntil: 'networkidle' });

      await expect.poll(async () => {
        const statusResponse = await page.request.get('/api/v1/security/status');
        if (!statusResponse.ok()) {
          return initialChecked;
        }
        const status = await statusResponse.json();
        return Boolean(status?.acl?.enabled);
      }, {
        timeout: 15000,
        message: 'Expected ACL enabled state to persist after page reload',
      }).toBe(!initialChecked);
    });
  });

  test.describe('Navigation', () => {
    test('should navigate to CrowdSec page when configure clicked', async ({ page }) => {
      // Find the CrowdSec card by locating the configure button within a container that has CrowdSec text
      const configureButtons = page.getByRole('button', { name: /configure|manage.*lists/i });
      await expect(configureButtons).toHaveCount(4);
      const configureButton = configureButtons.first();
      await expect(configureButton).toBeEnabled({ timeout: 10000 });

      // Wait for any loading overlays to disappear
      await page.waitForLoadState('networkidle');

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
      await aclButton.click({ force: true });
      await expect(page).toHaveURL(/\/security\/access-lists|\/access-lists/);
    });

    test('should navigate to WAF page when configure clicked', async ({ page }) => {
      const wafCard = page.getByTestId('toggle-waf').locator('xpath=ancestor::div[contains(@class, "flex")][1]');
      const wafButton = wafCard.getByRole('button', { name: /configure/i });
      await expect(wafButton).toBeVisible({ timeout: 10000 });

      // Wait and scroll into view
      await page.waitForLoadState('networkidle');
      await wafButton.scrollIntoViewIfNeeded();
      await wafButton.click({ force: true });
      await expect(page).toHaveURL(/\/security\/waf/);
    });

    test('should navigate to Rate Limiting page when configure clicked', async ({ page }) => {
      const rateLimitCard = page.getByTestId('toggle-rate-limit').locator('xpath=ancestor::div[contains(@class, "flex")][1]');
      const rateLimitButton = rateLimitCard.getByRole('button', { name: /configure/i });
      await expect(rateLimitButton).toBeVisible({ timeout: 10000 });

      // Wait and scroll into view
      await page.waitForLoadState('networkidle');
      await rateLimitButton.scrollIntoViewIfNeeded();
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
      await expect(whitelistInput).toBeVisible({ timeout: 10000 });
    });

    test('Emergency token can be generated', async ({ page, request }, testInfo) => {
      const securityStatePre = await captureSecurityState(request);
      testInfo.annotations.push({
        type: 'security-state-pre',
        description: JSON.stringify(securityStatePre),
      });

      try {
        await test.step('Verify generate token button exists in security dashboard', async () => {
          const generateButton = page.getByRole('button', { name: /generate.*token/i });
          await expect(generateButton).toBeVisible();
          await expect(generateButton).toBeEnabled();
        });

        await test.step('Generate emergency token from security dashboard UI', async () => {
          await page.getByRole('button', { name: /generate.*token/i }).click();
        });
      } finally {
        const securityStatePost = await captureSecurityState(request);
        testInfo.annotations.push({
          type: 'security-state-post',
          description: JSON.stringify(securityStatePost),
        });

        expect(securityStatePost.cerberus).toBe(securityStatePre.cerberus);
        expect(securityStatePost.acl).toBe(securityStatePre.acl);
        expect(securityStatePost.waf).toBe(securityStatePre.waf);
        expect(securityStatePost.rateLimit).toBe(securityStatePre.rateLimit);
        expect(securityStatePost.crowdsec).toBe(securityStatePre.crowdsec);
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
