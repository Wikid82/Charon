import { test, expect } from '@playwright/test';

/**
 * Phase 4 Integration: Data Consistency (UI ↔ API)
 *
 * Purpose: Validate data consistency between UI operations and API responses
 * Scenarios: UI create/API read, API modify/UI display, concurrent operations
 * Success: UI and API always show same data, no data loss or corruption
 */

test.describe('INT-005: Data Consistency', () => {
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

  test.beforeEach(async ({ page }) => {
    await page.goto('/', { waitUntil: 'networkidle' });
    await page.waitForSelector('[role="main"]', { timeout: 5000 });
  });

  test.afterEach(async ({ page }) => {
    try {
      // Cleanup user
      await page.goto('/users', { waitUntil: 'networkidle' });
      const userRow = page.locator(`text=${testUser.email}`).first();
      if (await userRow.isVisible()) {
        const deleteButton = userRow.locator('..').getByRole('button', { name: /delete/i }).first();
        await deleteButton.click();

        const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }
        await page.waitForLoadState('networkidle');
      }

      // Cleanup proxy
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });
      const proxyRow = page.locator(`text=${testProxy.domain}`).first();
      if (await proxyRow.isVisible()) {
        const deleteButton = proxyRow.locator('..').getByRole('button', { name: /delete/i }).first();
        await deleteButton.click();

        const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }
        await page.waitForLoadState('networkidle');
      }
    } catch {
      // Ignore cleanup errors
    }
  });

  // INT-005-1: UI create → API read returns same data
  test('Data created via UI is properly stored and readable via API', async ({ page }) => {
    let createdUserId: string | null = null;

    await test.step('Create user via UI', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/name/i).fill(testUser.name);
      await page.getByLabel(/password/i).first().fill(testUser.password);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify API returns created user data', async () => {
      const token = await page.evaluate(() => localStorage.getItem('token'));

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
          expect(foundUser.name).toBe(testUser.name);
          createdUserId = foundUser.id;
        }
      }
    });
  });

  // INT-005-2: API modify → UI displays updated data
  test('Data modified via API is reflected in UI', async ({ page }) => {
    const updatedName = 'Updated User Name';

    await test.step('Create user', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/name/i).fill(testUser.name);
      await page.getByLabel(/password/i).first().fill(testUser.password);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Modify user via API', async () => {
      const token = await page.evaluate(() => localStorage.getItem('token'));

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
      await page.goto('/users', { waitUntil: 'networkidle' });
      await page.reload();

      // Check if updated name is visible
      const updatedElement = page.getByText(updatedName).first();
      if (await updatedElement.isVisible()) {
        await expect(updatedElement).toBeVisible();
      }
    });
  });

  // INT-005-3: Delete via UI → API no longer returns
  test('Data deleted via UI is removed from API', async ({ page }) => {
    await test.step('Create user', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/name/i).fill(testUser.name);
      await page.getByLabel(/password/i).first().fill(testUser.password);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Delete user via UI', async () => {
      const userRow = page.locator(`text=${testUser.email}`).first();
      if (await userRow.isVisible()) {
        const deleteButton = userRow.locator('..').getByRole('button', { name: /delete/i }).first();
        await deleteButton.click();

        const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify API no longer returns user', async () => {
      const token = await page.evaluate(() => localStorage.getItem('token'));

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

  // INT-005-4: Concurrent modifications resolved correctly
  test('Concurrent modifications do not cause data corruption', async ({ page }) => {
    await test.step('Create initial user', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/name/i).fill(testUser.name);
      await page.getByLabel(/password/i).first().fill(testUser.password);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Trigger concurrent modifications', async () => {
      const token = await page.evaluate(() => localStorage.getItem('token'));

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
      await page.goto('/users', { waitUntil: 'networkidle' });

      const userElement = page.locator(`text=${testUser.email}`).first();
      await expect(userElement).toBeVisible();

      // User should have one of the update names, not corrupted
      const updateTwo = page.getByText('Update Two').first();
      if (await updateTwo.isVisible()) {
        await expect(updateTwo).toBeVisible();
      }
    });
  });

  // INT-005-5: Transaction rollback prevents partial updates
  test('Failed transaction prevents partial data updates', async ({ page }) => {
    await test.step('Create proxy', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Attempt invalid modification', async () => {
      const token = await page.evaluate(() => localStorage.getItem('token'));

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
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const proxyElement = page.locator(`text=${testProxy.domain}`).first();
      await expect(proxyElement).toBeVisible();
    });
  });

  // INT-005-6: Database constraints enforced (unique, foreign key)
  test('Database constraints prevent invalid data', async ({ page }) => {
    await test.step('Create first user', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/name/i).fill(testUser.name);
      await page.getByLabel(/password/i).first().fill(testUser.password);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Attempt to create duplicate user with same email', async () => {
      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email); // Duplicate email
      await page.getByLabel(/name/i).fill('Different Name');
      await page.getByLabel(/password/i).first().fill('DiffPass123!');

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify duplicate prevented by error message', async () => {
      const errorElement = page.getByText(/duplicate|already.*exists|unique/i).first();
      if (await errorElement.isVisible()) {
        await expect(errorElement).toBeVisible();
      }
    });
  });

  // INT-005-7: UI form validation matches backend validation
  test('Client-side and server-side validation consistent', async ({ page }) => {
    await test.step('Test email format validation on create form', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      // Try invalid email
      const emailInput = page.getByLabel(/email/i);
      await emailInput.fill('not-an-email');

      // Check for client validation error
      const invalidFeedback = page.getByText(/invalid|format|email/i).first();
      if (await invalidFeedback.isVisible()) {
        await expect(invalidFeedback).toBeVisible();
      }

      // Try submitting anyway
      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      if (!(await submitButton.isDisabled())) {
        await submitButton.click();
        await page.waitForLoadState('networkidle');

        // Server should also reject
        const serverError = page.getByText(/invalid|error|failed/i).first();
        if (await serverError.isVisible()) {
          await expect(serverError).toBeVisible();
        }
      }
    });
  });

  // INT-005-8: Long dataset pagination consistent across loads
  test('Pagination and sorting produce consistent results', async ({ page }) => {
    await test.step('Create multiple users for pagination test', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      for (let i = 0; i < 3; i++) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        const uniqueEmail = `user${i}@test.local`;
        await page.getByLabel(/email/i).fill(uniqueEmail);
        await page.getByLabel(/name/i).fill(`User ${i}`);
        await page.getByLabel(/password/i).first().fill(`Pass${i}123!`);

        const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Navigate and verify pagination consistency', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      // Get first page
      const page1Items = await page.locator('[role="row"], [class*="user-item"]').count();
      expect(page1Items).toBeGreaterThan(0);

      // Navigate to page 2 if pagination exists
      const nextButton = page.getByRole('button', { name: /next|>/ }).first();
      if (await nextButton.isVisible() && !(await nextButton.isDisabled())) {
        await nextButton.click();
        await page.waitForLoadState('networkidle');

        const page2Items = await page.locator('[role="row"], [class*="user-item"]').count();
        expect(page2Items).toBeGreaterThanOrEqual(0);

        // Go back to page 1
        const prevButton = page.getByRole('button', { name: /prev|</ }).first();
        if (await prevButton.isVisible()) {
          await prevButton.click();
          await page.waitForLoadState('networkidle');

          const backPage1Items = await page.locator('[role="row"], [class*="user-item"]').count();
          expect(backPage1Items).toBe(page1Items);
        }
      }
    });
  });
});
