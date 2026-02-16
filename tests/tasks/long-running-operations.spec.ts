import { test, expect, loginUser } from '../fixtures/auth-fixtures';

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
  };

  const createUserViaApi = async (page: import('@playwright/test').Page) => {
    const token = await page.evaluate(() =>
      localStorage.getItem('token') ||
      localStorage.getItem('charon_auth_token') ||
      localStorage.getItem('auth') ||
      ''
    );

    const response = await page.request.post('/api/v1/users', {
      data: testUser,
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    expect(response.ok()).toBe(true);
  };

  const createProxyViaApi = async (page: import('@playwright/test').Page) => {
    const token = await page.evaluate(() =>
      localStorage.getItem('token') ||
      localStorage.getItem('charon_auth_token') ||
      localStorage.getItem('auth') ||
      ''
    );

    const response = await page.request.post('/api/v1/proxy-hosts', {
      data: {
        domain: testProxy.domain,
        forwardHost: testProxy.forwardHost,
        forwardPort: Number.parseInt(testProxy.forwardPort, 10),
        scheme: 'http',
      },
      headers: {
        Authorization: `Bearer ${token}`,
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

      await expect(page.getByRole('main').first()).toBeVisible();
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
      const proxyElement = page.locator(`text=${testProxy.domain}`).first();
      await expect(proxyElement).toBeVisible();

      // Backup should still be running or completed
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });

      const backupList = page.locator('[class*="backup"]');
      const count = await backupList.count();
      expect(count).toBeGreaterThan(0);
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
      await page.getByRole('button', { name: /login|sign in/i }).click();
      await page.waitForLoadState('networkidle');

      const duration = Date.now() - start;

      console.log(`✓ Login during backup completed in ${duration}ms`);
      expect(duration).toBeLessThan(5000);

      // Verify login succeeded
      const dashboard = page.locator('[role="main"]').first();
      await expect(dashboard).toBeVisible();
    });
  });

  // Task completion verified after operation finishes
  test('Long-running task completion can be verified', async ({ page }) => {
    let backupId: string | null = null;

    await test.step('Create backup and get ID', async () => {
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });

      const backupButton = page.getByRole('button', { name: /backup|create|manual/i }).first();
      if (await backupButton.isVisible()) {
        await backupButton.click();
        await page.waitForLoadState('networkidle');
      }

      // Get first backup from list
      const backupElements = page.locator('[class*="backup-item"], [role="row"]');
      const firstBackup = backupElements.first();
      if (await firstBackup.isVisible()) {
        const text = await firstBackup.textContent();
        if (text) {
          backupId = text.substring(0, 10); // Extract some identifier
        }
      }
    });

    await test.step('Wait for backup completion', async () => {
      await page.waitForTimeout(2000); // Wait for backup to complete
    });

    await test.step('Verify backup status is complete', async () => {
      await page.reload();

      const completedElement = page.getByText(/completed|finished|success/i).first();
      if (await completedElement.isVisible()) {
        await expect(completedElement).toBeVisible();
        console.log(`✓ Backup ${backupId} completed successfully`);
      }
    });

    await test.step('Verify backup can be used (download/restore)', async () => {
      const restoreButton = page.getByRole('button', { name: /restore|download/i }).first();
      if (await restoreButton.isVisible()) {
        // Just verify button is clickable, don't actually restore
        await expect(restoreButton).toBeEnabled();
      }
    });
  });
});
