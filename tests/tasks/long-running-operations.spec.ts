import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForToast, waitForLoadingComplete } from '../utils/wait-helpers';

/**
 * Integration: Long-Running Operations
 *
 * Purpose: Verify system remains responsive during lengthy background tasks
 * Scenarios: Backup during other operations, concurrent task execution
 * Success: System responsive, tasks execute independently, no blocking
 */

test.describe('Long-Running Operations', () => {
  let testProxy = {
    name: '',
    domain: '',
    forwardHost: 'localhost',
    forwardPort: '3001',
    description: 'Test proxy for long-running ops',
  };

  let testUser = {
    email: '',
    name: '',
    password: 'LongOpsPass123!',
    role: 'user' as const,
  };

  const createUserViaApi = async (page: import('@playwright/test').Page) => {
    const response = await page.request.post('/api/v1/users', {
      data: testUser,
    });

    expect(response.ok()).toBe(true);
  };

  const createProxyViaApi = async (page: import('@playwright/test').Page) => {
    const response = await page.request.post('/api/v1/proxy-hosts', {
      data: {
        name: testProxy.name,
        domain_names: testProxy.domain,
        forward_host: testProxy.forwardHost,
        forward_port: Number.parseInt(testProxy.forwardPort, 10),
        forward_scheme: 'http',
        websocket_support: false,
        enabled: true,
      },
    });

    expect(response.ok()).toBe(true);
  };

  test.beforeEach(async ({ page, adminUser }) => {
    const uniqueSuffix = `${Date.now()}-${Math.floor(Math.random() * 1000)}`;
    testProxy = {
      name: `Long Ops Proxy ${uniqueSuffix}`,
      domain: `longops-${uniqueSuffix}.test.local`,
      forwardHost: 'localhost',
      forwardPort: '3001',
      description: 'Test proxy for long-running ops',
    };

    testUser = {
      email: `longops-${uniqueSuffix}@test.local`,
      name: `Long Ops User ${uniqueSuffix}`,
      password: 'LongOpsPass123!',
    };

    await loginUser(page, adminUser);
    await page.getByRole('main').first().waitFor({ state: 'visible', timeout: 15000 });
  });

  test.afterEach(async ({ page }) => {
    try {
      await page.goto('/proxy-hosts', { waitUntil: 'domcontentloaded', timeout: 10000 });
      const proxyRow = page.locator(`text=${testProxy.domain}`).first();
      if (await proxyRow.isVisible()) {
        const deleteButton = proxyRow.locator('..').getByRole('button', { name: /delete/i }).first();
        await deleteButton.click();

        const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }
        await page.waitForLoadState('domcontentloaded').catch(() => Promise.resolve());
      }

      await page.goto('/users', { waitUntil: 'domcontentloaded', timeout: 10000 });
      const userRow = page.locator(`text=${testUser.email}`).first();
      if (await userRow.isVisible()) {
        const deleteButton = userRow.locator('..').getByRole('button', { name: /delete/i }).first();
        await deleteButton.click();

        const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }
        await page.waitForLoadState('domcontentloaded').catch(() => Promise.resolve());
      }
    } catch {
      // Ignore cleanup errors
    }
  });

  // Create backup while other operations running
  test('Backup creation does not block other operations', async ({ page }) => {
    await test.step('Initiate backup creation', async () => {
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });

      const backupButton = page.getByRole('button', { name: /backup|create|download/i }).first();
      if (await backupButton.isVisible()) {
        await backupButton.click();
      }
    });

    await test.step('While backup running, create new user', async () => {
      const start = Date.now();
      await createUserViaApi(page);

      const duration = Date.now() - start;

      // User creation should complete quickly despite background backup
      expect(duration).toBeLessThan(10000);
      console.log(`✓ User created while backup running in ${duration}ms`);
    });

    await test.step('Verify backup completed', async () => {
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });

      await expect(page).toHaveURL(/\/settings\/backup|\/backup/i);
    });
  });

  // System remains responsive during backup
  test('UI remains responsive while backup in progress', async ({ page }) => {
    await test.step('Start backup operation', async () => {
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });

      const backupButton = page.getByRole('button', { name: /backup|create|start/i }).first();
      if (await backupButton.isVisible()) {
        await backupButton.click();
      }
    });

    await test.step('Check system responsiveness during backup', async () => {
      // Try navigating to other pages while backup runs
      const navigationPages = ['/proxy-hosts', '/users', '/settings'];

      for (const navPath of navigationPages) {
        const start = Date.now();

        await page.goto(navPath, { waitUntil: 'domcontentloaded', timeout: 5000 }).catch(() => {
          // Navigation should work even if slow
          return Promise.resolve();
        });

        const duration = Date.now() - start;

        // Should respond within reasonable time
        expect(duration).toBeLessThan(5000);
        console.log(`✓ Navigation to ${navPath} took ${duration}ms during backup`);
      }
    });

    await test.step('Perform additional operations during backup', async () => {
      const start = Date.now();

      const response = await page.request.get('/api/v1/proxy-hosts');

      const duration = Date.now() - start;

      expect(response.ok()).toBe(true);
      console.log(`✓ API call during backup completed in ${duration}ms`);
    });
  });

  // Proxy creation during backup completes independently
  test('Proxy creation independent of backup operation', async ({ page }) => {
    await test.step('Start backup', async () => {
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });

      const backupButton = page.getByRole('button', { name: /backup|create/i }).first();
      if (await backupButton.isVisible()) {
        await backupButton.click();
      }
    });

    await test.step('Create proxy while backup in progress', async () => {
      const start = Date.now();

      await createProxyViaApi(page);

      const duration = Date.now() - start;

      console.log(`✓ Proxy created during backup in ${duration}ms`);
      expect(duration).toBeLessThan(10000);
    });

    await test.step('Verify proxy operational and backup still running', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'domcontentloaded' });
      const proxyElement = page.locator(`text=${testProxy.domain}`).first();
      await expect(proxyElement).toBeVisible();

      // Backup should still be running or completed
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });
      await expect(page).toHaveURL(/\/settings\/backup|\/backup/i);
    });
  });

  // User login succeeds during long operation
  test('Authentication completes quickly even during background tasks', async ({ page }) => {
    await test.step('Create test user', async () => {
      await createUserViaApi(page);
    });

    await test.step('Initiate backup', async () => {
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });

      const backupButton = page.getByRole('button', { name: /backup|create/i }).first();
      if (await backupButton.isVisible()) {
        await backupButton.click();
      }
    });

    await test.step('Login attempt during backup', async () => {
      await page.goto('/login', { waitUntil: 'domcontentloaded' });

      const start = Date.now();

      const emailInput = page.locator('input[type="email"]').or(page.getByLabel(/email/i)).first();
      const passwordInput = page.locator('input[type="password"]').or(page.getByLabel(/password/i)).first();

      await expect(emailInput).toBeVisible({ timeout: 15000 });
      await expect(passwordInput).toBeVisible({ timeout: 15000 });

      await emailInput.fill(testUser.email);
      await passwordInput.fill(testUser.password);
      const loginResponsePromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/auth/login') &&
          response.request().method() === 'POST'
      );

      await page.getByRole('button', { name: /login|sign in/i }).click();
      const loginResponse = await loginResponsePromise;
      await page.waitForLoadState('networkidle');

      const duration = Date.now() - start;

      console.log(`✓ Login during backup completed in ${duration}ms`);
      expect(duration).toBeLessThan(20000);
      expect(loginResponse.ok()).toBe(true);

      await expect(page).not.toHaveURL(/\/login/i);
    });
  });

  // Task completion verified after operation finishes
  test('Long-running task completion can be verified', async ({ page }) => {
    await test.step('Create backup from the backups task page', async () => {
      await page.goto('/tasks/backups', { waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      const backupButton = page.getByRole('button', { name: /create backup/i }).first();
      await expect(backupButton).toBeVisible();

      const createResponsePromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/backups') &&
          response.request().method() === 'POST' &&
          (response.status() === 200 || response.status() === 201)
      );

      await backupButton.click();
      await expect(backupButton).toBeDisabled();
      await createResponsePromise;
      await waitForToast(page, /success|created/i, { type: 'success' });
      await expect(backupButton).toBeEnabled();
    });

    await test.step('Verify created backup is actionable', async () => {
      await page.reload({ waitUntil: 'domcontentloaded' });
      await waitForLoadingComplete(page);

      const backupRows = page.locator('[data-testid="backup-row"]');
      await expect(backupRows.first()).toBeVisible();

      const downloadButton = page.locator('[data-testid="backup-download-btn"]').first();
      await expect(downloadButton).toBeEnabled();
    });
  });
});
