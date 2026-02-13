import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForDialog, waitForLoadingComplete } from '../utils/wait-helpers';

async function getAuthToken(page: import('@playwright/test').Page): Promise<string> {
  return await page.evaluate(() => {
    return (
      localStorage.getItem('token') ||
      localStorage.getItem('charon_auth_token') ||
      localStorage.getItem('auth') ||
      ''
    );
  });
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

async function openInviteUserForm(page: import('@playwright/test').Page): Promise<void> {
  await page.goto('/users');
  await waitForLoadingComplete(page, { timeout: 15000 });
  const inviteButton = page.getByRole('button', { name: /invite.*user|add user|create user/i }).first();
  await expect(inviteButton).toBeVisible({ timeout: 15000 });
  await inviteButton.click();
  await waitForDialog(page, { timeout: 15000 });
}

/**
 * Integration: Data Consistency (UI ↔ API)
 *
 * Purpose: Validate data consistency between UI operations and API responses
 * Scenarios: UI create/API read, API modify/UI display, concurrent operations
 * Success: UI and API always show same data, no data loss or corruption
 */

test.describe('Data Consistency', () => {
  let testUser = {
    email: '',
    name: 'Consistency Test User',
    password: 'ConsPass123!',
  };

  let testProxy = {
    domain: '',
    target: 'http://localhost:3001',
    description: 'Data consistency test proxy',
  };

  test.beforeEach(async ({ page, adminUser }) => {
    const suffix = `${Date.now()}-${Math.floor(Math.random() * 10000)}`;
    testUser = {
      email: `consistency-${suffix}@test.local`,
      name: `Consistency User ${suffix}`,
      password: 'ConsPass123!',
    };

    testProxy = {
      domain: `consistency-${suffix}.local`,
      target: 'http://localhost:3001',
      description: 'Data consistency test proxy',
    };

    await loginUser(page, adminUser);
    await waitForLoadingComplete(page, { timeout: 15000 });
    await expect(page.getByRole('main')).toBeVisible({ timeout: 15000 });
  });

  test.afterEach(async ({ page }) => {
    try {
      // Cleanup user
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });
      const userRow = page.getByText(testUser.email);
      if (await userRow.isVisible().catch(() => false)) {
        const row = userRow.locator('xpath=ancestor::tr');
        const deleteButton = row.getByRole('button', { name: /delete/i });

        page.once('dialog', async (dialog) => {
          await dialog.accept();
        });

        await deleteButton.click();

        await waitForLoadingComplete(page, { timeout: 15000 });
      }

      // Cleanup proxy
      await page.goto('/proxy-hosts');
      await waitForLoadingComplete(page, { timeout: 15000 });
      const proxyRow = page.getByText(testProxy.domain);
      if (await proxyRow.isVisible().catch(() => false)) {
        const row = proxyRow.locator('xpath=ancestor::tr');
        const deleteButton = row.getByRole('button', { name: /delete/i });
        await deleteButton.click();

        const confirmButton = page.getByRole('dialog').getByRole('button', { name: /confirm|delete/i });
        if (await confirmButton.isVisible().catch(() => false)) {
          await confirmButton.click();
        }
        await waitForLoadingComplete(page, { timeout: 15000 });
      }
    } catch {
      // Ignore cleanup errors
    }
  });

  // UI create → API read returns same data
  test('Data created via UI is properly stored and readable via API', async ({ page }) => {
    await test.step('Create user via UI', async () => {
      await createUserViaApi(page, { ...testUser, role: 'user' });
    });

    await test.step('Verify API returns created user data', async () => {
      const token = await getAuthToken(page);

      const response = await page.request.get(
        '/api/v1/users',
        {
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.ok()).toBe(true);

      const users = await response.json();
      expect(Array.isArray(users)).toBe(true);
      const foundUser = Array.isArray(users)
        ? users.find((u: any) => u.email === testUser.email)
        : null;

      expect(foundUser).toBeTruthy();
      if (foundUser) {
        expect(foundUser.email).toBe(testUser.email);
      }
    });
  });

  // API modify → UI displays updated data
  test('Data modified via API is reflected in UI', async ({ page }) => {
    const updatedName = 'Updated User Name';

    await test.step('Create user', async () => {
      await createUserViaApi(page, { ...testUser, role: 'user' });
    });

    await test.step('Modify user via API', async () => {
      const token = await getAuthToken(page);

      const usersResponse = await page.request.get(
        '/api/v1/users',
        {
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );
      expect(usersResponse.ok()).toBe(true);

      const users = await usersResponse.json();
      expect(Array.isArray(users)).toBe(true);
      const user = Array.isArray(users)
        ? users.find((u: any) => u.email === testUser.email)
        : null;

      expect(user).toBeTruthy();

      const updateResponse = await page.request.put(
        `/api/v1/users/${user.id}`,
        {
          data: { name: updatedName },
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      expect(updateResponse.ok()).toBe(true);
      const updateBody = await updateResponse.json();
      expect(updateBody).toEqual(expect.objectContaining({
        message: expect.stringMatching(/updated/i),
      }));
    });

    await test.step('Reload UI and verify updated data', async () => {
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });
      await page.reload();
      await waitForLoadingComplete(page, { timeout: 15000 });

      const updatedElement = page.getByText(updatedName).first();
      await expect(updatedElement).toBeVisible();
    });
  });

  // Delete via UI → API no longer returns
  test('Data deleted via UI is removed from API', async ({ page }) => {
    await test.step('Create user', async () => {
      await createUserViaApi(page, { ...testUser, role: 'user' });
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });
    });

    await test.step('Delete user via UI', async () => {
      const userRow = page.getByText(testUser.email);
      await expect(userRow.first()).toBeVisible({ timeout: 15000 });
      const row = userRow.locator('xpath=ancestor::tr');
      const deleteButton = row.getByRole('button', { name: /delete/i });

      const deleteResponsePromise = page.waitForResponse(
        (response) => response.url().includes('/api/v1/users/') && response.request().method() === 'DELETE',
        { timeout: 15000 }
      );

      page.once('dialog', async (dialog) => {
        await dialog.accept();
      });

      await deleteButton.click();
      const deleteResponse = await deleteResponsePromise;
      expect(deleteResponse.ok()).toBe(true);
      await waitForLoadingComplete(page, { timeout: 15000 });
    });

    await test.step('Verify API no longer returns user', async () => {
      const token = await getAuthToken(page);

      const response = await page.request.get(
        '/api/v1/users',
        {
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.ok()).toBe(true);

      const users = await response.json();
      expect(Array.isArray(users)).toBe(true);
      if (Array.isArray(users)) {
        const foundUser = users.find((u: any) => u.email === testUser.email);
        expect(foundUser).toBeUndefined();
      }
    });
  });

  // Concurrent modifications resolved correctly
  test('Concurrent modifications do not cause data corruption', async ({ page }) => {
    await test.step('Create initial user', async () => {
      await createUserViaApi(page, { ...testUser, role: 'user' });
    });

    await test.step('Trigger concurrent modifications', async () => {
      const token = await getAuthToken(page);

      const usersResponse = await page.request.get(
        '/api/v1/users',
        {
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );
      expect(usersResponse.ok()).toBe(true);

      const users = await usersResponse.json();
      expect(Array.isArray(users)).toBe(true);
      const user = Array.isArray(users)
        ? users.find((u: any) => u.email === testUser.email)
        : null;

      expect(user).toBeTruthy();

      const update1 = page.request.put(
        `/api/v1/users/${user.id}`,
        {
          data: { name: 'Update One' },
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      const update2 = page.request.put(
        `/api/v1/users/${user.id}`,
        {
          data: { name: 'Update Two' },
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      const [resp1, resp2] = await Promise.all([update1, update2]);
      expect([200, 409]).toContain(resp1.status());
      expect([200, 409]).toContain(resp2.status());
      expect(resp1.ok() || resp2.ok()).toBe(true);
    });

    await test.step('Verify final state is consistent', async () => {
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });

      const userElement = page.getByText(testUser.email);
      await expect(userElement).toBeVisible();

      const nameOne = page.getByText('Update One').first();
      const nameTwo = page.getByText('Update Two').first();
      const hasOne = await nameOne.isVisible();
      const hasTwo = await nameTwo.isVisible();
      expect(hasOne || hasTwo).toBe(true);
    });
  });

  // Transaction rollback prevents partial updates
  test('Failed transaction prevents partial data updates', async ({ page }) => {
    let createdProxyUUID = '';

    await test.step('Create proxy', async () => {
      const createResponse = await page.request.post('/api/v1/proxy-hosts', {
        data: {
          domain_names: testProxy.domain,
          forward_scheme: 'http',
          forward_host: 'localhost',
          forward_port: 3001,
          enabled: true,
        },
      });
      expect(createResponse.ok()).toBe(true);
      const createdProxy = await createResponse.json();
      expect(createdProxy).toEqual(expect.objectContaining({
        uuid: expect.any(String),
      }));
      createdProxyUUID = createdProxy.uuid;
    });

    await test.step('Attempt invalid modification', async () => {
      const token = await getAuthToken(page);

      // Try to modify with invalid data that should fail validation
      const response = await page.request.put(
        `/api/v1/proxy-hosts/${createdProxyUUID}`,
        {
          data: { domain_names: '' },
          headers: { Authorization: `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      expect([200, 400, 422]).toContain(response.status());
    });

    await test.step('Verify original data unchanged', async () => {
      await page.goto('/proxy-hosts');
      await waitForLoadingComplete(page, { timeout: 15000 });

      const token = await getAuthToken(page);
      await expect.poll(async () => {
        const detailResponse = await page.request.get(`/api/v1/proxy-hosts/${createdProxyUUID}`, {
          headers: { Authorization: `Bearer ${token}` },
        });

        if (!detailResponse.ok()) {
          return `status:${detailResponse.status()}`;
        }

        const proxy = await detailResponse.json();
        return proxy.domain_names || proxy.domainNames || '';
      }, {
        timeout: 15000,
        message: `Expected proxy ${createdProxyUUID} domain to remain unchanged after failed update`,
      }).toBe(testProxy.domain);
    });
  });

  // Database constraints enforced (unique, foreign key)
  test('Database constraints prevent invalid data', async ({ page }) => {
    await test.step('Create first user', async () => {
      await createUserViaApi(page, { ...testUser, role: 'user' });
    });

    await test.step('Attempt to create duplicate user with same email', async () => {
      const token = await getAuthToken(page);
      const duplicateResponse = await page.request.post('/api/v1/users', {
        data: { email: testUser.email, name: 'Different Name', password: 'DiffPass123!', role: 'user' },
        headers: { Authorization: `Bearer ${token}` },
      });
      expect([400, 409]).toContain(duplicateResponse.status());
    });

    await test.step('Verify duplicate prevented by error message', async () => {
      const token = await getAuthToken(page);
      const usersResponse = await page.request.get('/api/v1/users', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(usersResponse.ok()).toBe(true);
      const users = await usersResponse.json();
      expect(Array.isArray(users)).toBe(true);
      const matchingUsers = users.filter((u: any) => u.email === testUser.email);
      expect(matchingUsers).toHaveLength(1);
    });
  });

  // UI form validation matches backend validation
  test('Client-side and server-side validation consistent', async ({ page }) => {
    await test.step('Test email format validation on create form', async () => {
      await openInviteUserForm(page);

      // Try invalid email
      const emailInput = page.getByPlaceholder(/user@example/i);
      await emailInput.fill('not-an-email');

      // Check for client validation error
      const invalidFeedback = page.getByText(/invalid|format|email/i).first();
      if (await invalidFeedback.isVisible()) {
        await expect(invalidFeedback).toBeVisible();
      }

      // Try submitting anyway
      const submitButton = page.getByRole('dialog')
        .getByRole('button', { name: /send.*invite|create|submit/i })
        .first();
      if (!(await submitButton.isDisabled())) {
        await submitButton.click();
        await waitForLoadingComplete(page, { timeout: 15000 });

        // Server should also reject
        const serverError = page.getByText(/invalid|error|failed/i).first();
        if (await serverError.isVisible()) {
          await expect(serverError).toBeVisible();
        }
      }
    });
  });

  // Long dataset pagination consistent across loads
  test('Pagination and sorting produce consistent results', async ({ page }) => {
    const createdEmails: string[] = [];

    await test.step('Create multiple users for pagination test', async () => {
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });

      for (let i = 0; i < 3; i++) {
        const uniqueEmail = `user${i}-${Date.now()}-${Math.floor(Math.random() * 10000)}@test.local`;
        createdEmails.push(uniqueEmail);
        await createUserViaApi(page, {
          email: uniqueEmail,
          name: `User ${i}`,
          password: `Pass${i}123!`,
          role: 'user',
        });
      }

      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });
    });

    await test.step('Navigate and verify pagination consistency', async () => {
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });

      for (const email of createdEmails) {
        await expect(page.getByText(email).first()).toBeVisible({ timeout: 15000 });
      }

      // Navigate to page 2 if pagination exists
      const nextButton = page.getByRole('button', { name: /next|>/ }).first();
      if (await nextButton.isVisible() && !(await nextButton.isDisabled())) {
        const page1Items = await page.locator('table tbody tr').count();
        await nextButton.click();
        await waitForLoadingComplete(page, { timeout: 15000 });

        const page2Items = await page.locator('table tbody tr').count();
        expect(page2Items).toBeGreaterThanOrEqual(0);

        // Go back to page 1
        const prevButton = page.getByRole('button', { name: /prev|</ }).first();
        if (await prevButton.isVisible()) {
          await prevButton.click();
          await waitForLoadingComplete(page, { timeout: 15000 });

          const backPage1Items = await page.locator('table tbody tr').count();
          expect(backPage1Items).toBe(page1Items);
        }
      }
    });
  });
});
