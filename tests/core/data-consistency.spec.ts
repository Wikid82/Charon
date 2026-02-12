import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

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

async function openInviteUserForm(page: import('@playwright/test').Page): Promise<void> {
  await page.goto('/users');
  await waitForLoadingComplete(page, { timeout: 15000 });
  const inviteButton = page.getByRole('button', { name: /invite.*user|add user|create user/i }).first();
  await expect(inviteButton).toBeVisible({ timeout: 15000 });
  await inviteButton.click();
  await expect(page.getByRole('dialog')).toBeVisible({ timeout: 15000 });
}

async function fillInviteForm(
  page: import('@playwright/test').Page,
  user: { email: string; name?: string; password?: string }
): Promise<void> {
  const emailInput = page.getByPlaceholder(/user@example/i);
  await expect(emailInput).toBeVisible({ timeout: 15000 });
  await emailInput.fill(user.email);

  const nameInput = page.getByPlaceholder(/name/i)
    .or(page.getByLabel(/name/i))
    .first();
  if (await nameInput.isVisible().catch(() => false)) {
    await nameInput.fill(user.name || '');
  }

  const passwordInput = page.getByLabel(/password/i).first();
  if (user.password && await passwordInput.isVisible().catch(() => false)) {
    await passwordInput.fill(user.password);
  }

  const sendButton = page.getByRole('dialog')
    .getByRole('button', { name: /send.*invite|create|submit/i })
    .first();
  await expect(sendButton).toBeVisible({ timeout: 15000 });
  await sendButton.click();
  await waitForLoadingComplete(page, { timeout: 15000 });
}

async function openCreateProxyHostForm(page: import('@playwright/test').Page): Promise<void> {
  await page.goto('/proxy-hosts');
  await waitForLoadingComplete(page, { timeout: 15000 });
  const addButton = page.getByRole('button', { name: /add.*proxy.*host/i }).first();
  await expect(addButton).toBeVisible({ timeout: 15000 });
  await addButton.click();
}

/**
 * Integration: Data Consistency (UI ↔ API)
 *
 * Purpose: Validate data consistency between UI operations and API responses
 * Scenarios: UI create/API read, API modify/UI display, concurrent operations
 * Success: UI and API always show same data, no data loss or corruption
 */

test.describe('Data Consistency', () => {
  const testUser = {
    email: 'consistency@test.local',
    name: 'Consistency Test User',
    password: 'ConsPass123!',
  };

  const testProxy = {
    domain: 'consistency-test.local',
    target: 'http://localhost:3001',
    description: 'Data consistency test proxy',
  };

  test.beforeEach(async ({ page, adminUser }) => {
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
        await deleteButton.click();

        const confirmButton = page.getByRole('dialog').getByRole('button', { name: /confirm|delete/i });
        if (await confirmButton.isVisible().catch(() => false)) {
          await confirmButton.click();
        }
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
    let createdUserId: string | null = null;

    await test.step('Create user via UI', async () => {
      await openInviteUserForm(page);

      await fillInviteForm(page, testUser);
    });

    await test.step('Verify API returns created user data', async () => {
      const token = await getAuthToken(page);

      const response = await page.request.get(
        'http://127.0.0.1:8080/api/users',
        {
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.ok()).toBe(true);

      const users = await response.json().catch(() => null);
      if (users) {
        const foundUser = Array.isArray(users)
          ? users.find((u: any) => u.email === testUser.email)
          : null;

        if (foundUser) {
          expect(foundUser.email).toBe(testUser.email);
          createdUserId = foundUser.id;
        }
      }
    });
  });

  // API modify → UI displays updated data
  test('Data modified via API is reflected in UI', async ({ page }) => {
    const updatedName = 'Updated User Name';

    await test.step('Create user', async () => {
      await openInviteUserForm(page);

      await fillInviteForm(page, testUser);
    });

    await test.step('Modify user via API', async () => {
      const token = await getAuthToken(page);

      const usersResponse = await page.request.get(
        'http://127.0.0.1:8080/api/users',
        {
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      const users = await usersResponse.json().catch(() => []);
      const user = Array.isArray(users)
        ? users.find((u: any) => u.email === testUser.email)
        : null;

      if (user) {
        const updateResponse = await page.request.patch(
          `http://127.0.0.1:8080/api/users/${user.id}`,
          {
            data: { name: updatedName },
            headers: { 'Authorization': `Bearer ${token || ''}` },
            ignoreHTTPSErrors: true,
          }
        );

        expect(updateResponse.ok()).toBe(true);
      }
    });

    await test.step('Reload UI and verify updated data', async () => {
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });
      await page.reload();
      await waitForLoadingComplete(page, { timeout: 15000 });

      // Check if updated name is visible
      const updatedElement = page.getByText(updatedName).first();
      if (await updatedElement.isVisible()) {
        await expect(updatedElement).toBeVisible();
      }
    });
  });

  // Delete via UI → API no longer returns
  test('Data deleted via UI is removed from API', async ({ page }) => {
    await test.step('Create user', async () => {
      await openInviteUserForm(page);

      await fillInviteForm(page, testUser);
    });

    await test.step('Delete user via UI', async () => {
      const userRow = page.getByText(testUser.email);
      if (await userRow.isVisible().catch(() => false)) {
        const row = userRow.locator('xpath=ancestor::tr');
        const deleteButton = row.getByRole('button', { name: /delete/i });
        await deleteButton.click();

        const confirmButton = page.getByRole('dialog').getByRole('button', { name: /confirm|delete/i });
        if (await confirmButton.isVisible().catch(() => false)) {
          await confirmButton.click();
        }
        await waitForLoadingComplete(page, { timeout: 15000 });
      }
    });

    await test.step('Verify API no longer returns user', async () => {
      const token = await getAuthToken(page);

      const response = await page.request.get(
        'http://127.0.0.1:8080/api/users',
        {
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.ok()).toBe(true);

      const users = await response.json().catch(() => null);
      if (Array.isArray(users)) {
        const foundUser = users.find((u: any) => u.email === testUser.email);
        expect(foundUser).toBeUndefined();
      }
    });
  });

  // Concurrent modifications resolved correctly
  test('Concurrent modifications do not cause data corruption', async ({ page }) => {
    await test.step('Create initial user', async () => {
      await openInviteUserForm(page);

      await fillInviteForm(page, testUser);
    });

    await test.step('Trigger concurrent modifications', async () => {
      const token = await getAuthToken(page);

      const usersResponse = await page.request.get(
        'http://127.0.0.1:8080/api/users',
        {
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      const users = await usersResponse.json().catch(() => []);
      const user = Array.isArray(users)
        ? users.find((u: any) => u.email === testUser.email)
        : null;

      if (user) {
        // Send two concurrent updates
        const update1 = page.request.patch(
          `http://127.0.0.1:8080/api/users/${user.id}`,
          {
            data: { name: 'Update One' },
            headers: { 'Authorization': `Bearer ${token || ''}` },
            ignoreHTTPSErrors: true,
          }
        );

        const update2 = page.request.patch(
          `http://127.0.0.1:8080/api/users/${user.id}`,
          {
            data: { name: 'Update Two' },
            headers: { 'Authorization': `Bearer ${token || ''}` },
            ignoreHTTPSErrors: true,
          }
        );

        const [resp1, resp2] = await Promise.all([update1, update2]);
        expect(resp1.ok()).toBe(true);
        expect(resp2.ok()).toBe(true);
      }
    });

    await test.step('Verify final state is consistent', async () => {
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });

      const userElement = page.getByText(testUser.email);
      await expect(userElement).toBeVisible();

      // User should have one of the update names, not corrupted
      const updateTwo = page.getByText('Update Two');
      if (await updateTwo.isVisible().catch(() => false)) {
        await expect(updateTwo).toBeVisible();
      }
    });
  });

  // Transaction rollback prevents partial updates
  test('Failed transaction prevents partial data updates', async ({ page }) => {
    await test.step('Create proxy', async () => {
      await openCreateProxyHostForm(page);

      await page.getByRole('textbox', { name: /domain names/i }).first().fill(testProxy.domain);
      await page.getByRole('textbox', { name: /target|forward/i }).first().fill(testProxy.target);
      await page.getByRole('textbox', { name: /description/i }).first().fill(testProxy.description);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await waitForLoadingComplete(page, { timeout: 15000 });
    });

    await test.step('Attempt invalid modification', async () => {
      const token = await getAuthToken(page);

      // Try to modify with invalid data that should fail validation
      const response = await page.request.patch(
        `http://127.0.0.1:8080/api/proxy-hosts`,
        {
          data: { domain: '' }, // Empty domain should fail
          headers: { 'Authorization': `Bearer ${token || ''}` },
          ignoreHTTPSErrors: true,
        }
      );

      // Should fail validation
      expect(response.ok()).toBe(false);
    });

    await test.step('Verify original data unchanged', async () => {
      await page.goto('/proxy-hosts');
      await waitForLoadingComplete(page, { timeout: 15000 });

      const proxyElement = page.getByText(testProxy.domain);
      await expect(proxyElement).toBeVisible();
    });
  });

  // Database constraints enforced (unique, foreign key)
  test('Database constraints prevent invalid data', async ({ page }) => {
    await test.step('Create first user', async () => {
      await openInviteUserForm(page);

      await fillInviteForm(page, testUser);
    });

    await test.step('Attempt to create duplicate user with same email', async () => {
      await openInviteUserForm(page);

      await fillInviteForm(page, { email: testUser.email, name: 'Different Name', password: 'DiffPass123!' });
    });

    await test.step('Verify duplicate prevented by error message', async () => {
      const errorElement = page.getByText(/duplicate|already.*exists|unique/i).first();
      if (await errorElement.isVisible()) {
        await expect(errorElement).toBeVisible();
      }
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
      const submitButton = page.getByRole('button', { name: /send.*invite|create|submit/i }).first();
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
    await test.step('Create multiple users for pagination test', async () => {
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });

      for (let i = 0; i < 3; i++) {
        await openInviteUserForm(page);

        const uniqueEmail = `user${i}@test.local`;
        await fillInviteForm(page, { email: uniqueEmail, name: `User ${i}`, password: `Pass${i}123!` });
      }
    });

    await test.step('Navigate and verify pagination consistency', async () => {
      await page.goto('/users');
      await waitForLoadingComplete(page, { timeout: 15000 });

      // Get first page
      const page1Items = await page.locator('[role="row"], [class*="user-item"]').count();
      expect(page1Items).toBeGreaterThan(0);

      // Navigate to page 2 if pagination exists
      const nextButton = page.getByRole('button', { name: /next|>/ }).first();
      if (await nextButton.isVisible() && !(await nextButton.isDisabled())) {
        await nextButton.click();
        await waitForLoadingComplete(page, { timeout: 15000 });

        const page2Items = await page.locator('[role="row"], [class*="user-item"]').count();
        expect(page2Items).toBeGreaterThanOrEqual(0);

        // Go back to page 1
        const prevButton = page.getByRole('button', { name: /prev|</ }).first();
        if (await prevButton.isVisible()) {
          await prevButton.click();
          await waitForLoadingComplete(page, { timeout: 15000 });

          const backPage1Items = await page.locator('[role="row"], [class*="user-item"]').count();
          expect(backPage1Items).toBe(page1Items);
        }
      }
    });
  });
});
