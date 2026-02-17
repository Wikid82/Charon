import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { clickSwitch } from '../utils/ui-helpers';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const TEST_RUNNER_WHITELIST = '127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16';

type SecurityStatusResponse = {
  cerberus?: { enabled?: boolean };
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

  await patchWithRetry(page, '/api/v1/config', {
    security: { admin_whitelist: TEST_RUNNER_WHITELIST },
  });

  await patchWithRetry(page, '/api/v1/settings', {
    key: 'feature.cerberus.enabled',
    value: 'true',
  });

  await expect.poll(async () => {
    const response = await page.request.get('/api/v1/security/status');
    if (!response.ok()) {
      return false;
    }

    const status = (await response.json()) as SecurityStatusResponse;
    return Boolean(status.cerberus?.enabled);
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
      return Boolean(status.acl?.enabled);
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
      return Boolean(status.waf?.enabled);
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
      return Boolean(status.rate_limit?.enabled);
    }, {
      timeout: 15000,
      message: 'Expected rate limit state to change after toggle',
    }).toBe(!initialChecked);
  });

  test('navigates to security sub-pages from dashboard actions', async ({ page }) => {
    const crowdsecButton = page
      .getByTestId('toggle-crowdsec')
      .locator('xpath=ancestor::div[contains(@class, "flex")][1]')
      .getByRole('button', { name: /configure|manage.*lists/i })
      .first();
    await crowdsecButton.click({ force: true });
    await expect(page).toHaveURL(/\/security\/crowdsec/);

    await page.goto('/security');
    await waitForLoadingComplete(page);
    const aclButton = page
      .getByTestId('toggle-acl')
      .locator('xpath=ancestor::div[contains(@class, "flex")][1]')
      .getByRole('button', { name: /manage.*lists|configure/i })
      .first();
    await aclButton.click({ force: true });
    await expect(page).toHaveURL(/\/security\/access-lists|\/access-lists/);

    await page.goto('/security');
    await waitForLoadingComplete(page);
    const wafButton = page
      .getByTestId('toggle-waf')
      .locator('xpath=ancestor::div[contains(@class, "flex")][1]')
      .getByRole('button', { name: /configure/i })
      .first();
    await wafButton.click({ force: true });
    await expect(page).toHaveURL(/\/security\/waf/);

    await page.goto('/security');
    await waitForLoadingComplete(page);
    const rateLimitButton = page
      .getByTestId('toggle-rate-limit')
      .locator('xpath=ancestor::div[contains(@class, "flex")][1]')
      .getByRole('button', { name: /configure/i })
      .first();
    await rateLimitButton.click({ force: true });
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
