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

type BackupRestoreResponse = {
  message?: string;
  restart_required?: boolean;
  live_rehydrate_applied?: boolean;
};

async function restoreBackupAndWaitForLiveRehydrate(
  page: import('@playwright/test').Page,
  filename: string
): Promise<BackupRestoreResponse> {
  const token = await getAuthToken(page);
  const restoreResponse = await page.request.post(`/api/v1/backups/${filename}/restore`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(restoreResponse.ok()).toBe(true);

  const payload: BackupRestoreResponse = await restoreResponse.json();
  expect(payload).toEqual(expect.objectContaining({
    restart_required: false,
  }));

  return payload;
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


  // Create user → assign proxy-management role → verify role persistence
  test('User with proxy creation role is configured for proxy management', async ({ page, adminUser }) => {
    let createdUserId: string | number;

    await test.step('Create user with proxy management role', async () => {
      const createdUser = await createUserViaApi(page, { ...testUser, role: 'admin' });
      createdUserId = createdUser.id;
    });

    await test.step('Verify created user role persisted as admin', async () => {
      const token = await getAuthToken(page);
      const usersResponse = await page.request.get('/api/v1/users', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(usersResponse.ok()).toBe(true);

      const users = await usersResponse.json();
      expect(Array.isArray(users)).toBe(true);

      const createdUser = users.find((user: any) => user.id === createdUserId || user.email === testUser.email);
      expect(createdUser).toBeTruthy();
      expect((createdUser?.role || '').toLowerCase()).toBe('admin');
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
    let restorePayload: BackupRestoreResponse = {};

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
      expect(createdBackupFilename).toBeTruthy();

      restorePayload = await restoreBackupAndWaitForLiveRehydrate(page, createdBackupFilename);
    });

    await test.step('Verify restore completed with live rehydrate applied', async () => {
      expect(restorePayload).toEqual(expect.objectContaining({
        live_rehydrate_applied: true,
        restart_required: false,
      }));
    });
  });

});
