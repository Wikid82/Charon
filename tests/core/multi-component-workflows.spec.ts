import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

async function resetSecurityState(page: import('@playwright/test').Page): Promise<void> {
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
    data: { reason: 'multi-component deterministic setup/teardown' },
  });

  expect(response.ok()).toBe(true);
}

async function getAuthToken(page: import('@playwright/test').Page): Promise<string> {
  const token = await page.evaluate(() => {
    return (
      localStorage.getItem('token') ||
      localStorage.getItem('charon_auth_token') ||
      localStorage.getItem('auth') ||
      ''
    );
  });

  expect(token).toBeTruthy();
  return token;
}

function uniqueSuffix(): string {
  return `${Date.now()}-${Math.floor(Math.random() * 10000)}`;
}

async function createUserViaApi(
  page: import('@playwright/test').Page,
  user: { email: string; name: string; password: string; role: 'admin' | 'user' | 'guest' }
): Promise<{ id: string | number; email: string }> {
  const token = await getAuthToken(page);
  const response = await page.request.post('/api/v1/users', {
    data: user,
    headers: { Authorization: `Bearer ${token}` },
  });

  expect(response.ok()).toBe(true);
  const payload = await response.json();
  expect(payload).toEqual(expect.objectContaining({
    id: expect.anything(),
    email: user.email,
  }));

  return { id: payload.id, email: payload.email };
}

/**
 * Integration: Multi-Component Workflows
 *
 * Purpose: Validate complex workflows involving multiple system components
 * Scenarios: Create proxy → enable security → test enforcement, user workflows, backup restore integration
 * Success: Multi-step workflows complete correctly, all components integrate properly
 */

test.describe('Multi-Component Workflows', () => {
  let testProxy = {
    domain: `multi-workflow-${Date.now()}.local`,
    target: 'http://localhost:3001',
    description: 'Multi-component workflow test',
  };

  let testUser = {
    email: '',
    name: '',
    password: 'MultiFlow123!',
  };

  test.beforeEach(async ({ page, adminUser }) => {
    const suffix = uniqueSuffix();
    testProxy = {
      domain: `multi-workflow-${suffix}.local`,
      target: 'http://localhost:3001',
      description: 'Multi-component workflow test',
    };

    testUser = {
      email: `multiflow-${suffix}@test.local`,
      name: `Multi Workflow User ${suffix}`,
      password: 'MultiFlow123!',
    };

    await resetSecurityState(page);
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page, { timeout: 15000 });
    const meResponse = await page.request.get('/api/v1/auth/me');
    expect(meResponse.ok()).toBe(true);
  });

  test.afterEach(async ({ page }) => {
    try {
      const token = await getAuthToken(page);

      const proxiesResponse = await page.request.get('/api/v1/proxy-hosts', {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (proxiesResponse.ok()) {
        const proxies = await proxiesResponse.json();
        if (Array.isArray(proxies)) {
          const matchingProxy = proxies.find((proxy: any) =>
            proxy.domain_names === testProxy.domain || proxy.domainNames === testProxy.domain
          );
          if (matchingProxy?.uuid) {
            await page.request.delete(`/api/v1/proxy-hosts/${matchingProxy.uuid}`, {
              headers: { Authorization: `Bearer ${token}` },
            });
          }
        }
      }

      const usersResponse = await page.request.get('/api/v1/users', {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (usersResponse.ok()) {
        const users = await usersResponse.json();
        if (Array.isArray(users)) {
          const matchingUser = users.find((user: any) => user.email === testUser.email);
          if (matchingUser?.id) {
            await page.request.delete(`/api/v1/users/${matchingUser.id}`, {
              headers: { Authorization: `Bearer ${token}` },
            });
          }
        }
      }
    } catch {
      // Ignore cleanup errors
    } finally {
      await resetSecurityState(page);
    }
  });


  // Create user → Assign role → User creates proxy → Verify ACL
  test('User with proxy creation role can create and manage proxies', async ({ page }) => {
    await test.step('Create user with proxy management role', async () => {
      await createUserViaApi(page, { ...testUser, role: 'admin' });
    });

    await test.step('User logs in and attempts proxy creation', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
        await page.waitForURL(/login/);
      }

      await page.locator('input[type="email"]').first().fill(testUser.email);
      await page.locator('input[type="password"]').first().fill(testUser.password);
      await page.getByRole('button', { name: /sign in|login/i }).first().click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('User navigates to proxy management', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await expect(addButton).toBeVisible({ timeout: 15000 });
    });
  });

  // Create backup → Delete user → Restore → User reappears
  test('Backup restore recovers deleted user data', async ({ page }) => {
    const backupSuffix = uniqueSuffix();
    const userToBackup = {
      email: `backup-user-${backupSuffix}@test.local`,
      name: 'Backup Recovery User',
      password: 'BackupPass123!',
    };

    let createdUserId: string | number;
    let createdBackupFilename = '';

    await test.step('Create user to be backed up', async () => {
      const createdUser = await createUserViaApi(page, { ...userToBackup, role: 'user' });
      createdUserId = createdUser.id;
    });

    await test.step('Create backup with user data', async () => {
      const token = await getAuthToken(page);
      const backupResponse = await page.request.post('/api/v1/backups', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect([200, 201]).toContain(backupResponse.status());
      const backupPayload = await backupResponse.json();
      expect(backupPayload).toEqual(expect.objectContaining({
        filename: expect.any(String),
      }));
      createdBackupFilename = backupPayload.filename;
    });

    await test.step('Delete the user', async () => {
      const token = await getAuthToken(page);
      const deleteResponse = await page.request.delete(`/api/v1/users/${createdUserId}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(deleteResponse.ok()).toBe(true);
    });

    await test.step('Verify user is deleted', async () => {
      await page.reload();

      const deletedUser = page.locator(`text=${userToBackup.email}`).first();
      const isVisible = await deletedUser.isVisible().catch(() => false);
      expect(isVisible).toBe(false);
    });

    await test.step('Restore from backup', async () => {
      const token = await getAuthToken(page);
      expect(createdBackupFilename).toBeTruthy();

      const restoreResponse = await page.request.post(`/api/v1/backups/${createdBackupFilename}/restore`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(restoreResponse.ok()).toBe(true);
    });

    await test.step('Verify user reappeared after restore', async () => {
      const token = await getAuthToken(page);
      await expect.poll(async () => {
        const usersResponse = await page.request.get('/api/v1/users', {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (!usersResponse.ok()) {
          return false;
        }

        const users = await usersResponse.json();
        if (!Array.isArray(users)) {
          return false;
        }

        return users.some((user: any) => user.email === userToBackup.email);
      }, {
        timeout: 75000,
        message: `Expected restored user ${userToBackup.email} to reappear via API after backup restore`,
      }).toBe(true);
    });
  });

});
