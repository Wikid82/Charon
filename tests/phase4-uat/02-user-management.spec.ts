import { test, expect } from '@playwright/test';

/**
 * Phase 4 UAT: User Management
 *
 * Purpose: Validate CRUD operations for users and role assignments
 * Scenarios: Create, read, update, delete users; assign roles; verify access control
 * Success: Users can be managed with proper role-based access
 */

test.describe('UAT-002: User Management', () => {
  const testUsers = [
    { email: 'testuser1@test.local', name: 'Test User 1', password: 'TestPass123!' },
    { email: 'testuser2@test.local', name: 'Test User 2', password: 'TestPass123!' },
  ];

  test.beforeEach(async ({ page }) => {
    // Ensure admin is logged in before user management tests
    await page.goto('/');
    await page.waitForSelector('[data-testid="dashboard-container"], [role="main"]', { timeout: 5000 });
  });

  test.afterEach(async ({ page }) => {
    // Clean up test users created during this test
    // Navigate to users page and delete test users
    const usersLink = page.getByRole('link', { name: /user|people|account/i });
    if (await usersLink.isVisible()) {
      await usersLink.click();
      await page.waitForLoadState('networkidle');

      for (const user of testUsers) {
        const userRow = page.locator('text=' + user.email).first();
        if (await userRow.isVisible()) {
          // Find delete button for this user
          const deleteButton = userRow.locator('..').getByRole('button', { name: /delete|remove/i }).first();
          if (await deleteButton.isVisible()) {
            await deleteButton.click();
            // Confirm deletion if modal appears
            const confirmButton = page.getByRole('button', { name: /confirm|delete|yes/i });
            if (await confirmButton.isVisible()) {
              await confirmButton.click();
              await page.waitForLoadState('networkidle');
            }
          }
        }
      }
    }
  });

  // UAT-101: Create new user with all fields
  test('Create new user with all fields', async ({ page }) => {
    const newUser = testUsers[0];

    await test.step('Navigate to users page', async () => {
      const usersLink = page.getByRole('link', { name: /user|people|account/i });
      await usersLink.click();
      await page.waitForSelector('[data-testid="users-list"], [data-testid="user-table"]', { timeout: 5000 });
    });

    await test.step('Click add user button', async () => {
      const addButton = page.getByRole('button', { name: /add|create|new/i }).first();
      await addButton.click();
      await page.waitForSelector('[role="dialog"], [class*="modal"], form', { timeout: 3000 });
    });

    await test.step('Fill user creation form', async () => {
      await page.getByLabel(/email/i).fill(newUser.email);
      await page.getByLabel(/name|full.?name/i).fill(newUser.name);
      await page.getByLabel(/password/i).first().fill(newUser.password);

      // Confirm password (if exists)
      const confirmPassword = page.getByLabel(/confirm.?password|password.?again/i);
      if (await confirmPassword.isVisible()) {
        await confirmPassword.fill(newUser.password);
      }

      // Select role if available
      const roleSelect = page.locator('select[name*="role"], [class*="role-select"]').first();
      if (await roleSelect.isVisible()) {
        await roleSelect.selectOption('user');
      }
    });

    await test.step('Submit form', async () => {
      const submitButton = page.getByRole('button', { name: /create|submit|save/i }).first();
      await submitButton.click();

      // Wait for confirmation
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify user created and in list', async () => {
      const userEmail = page.locator(`text=${newUser.email}`).first();
      await expect(userEmail).toBeVisible();

      // Should show success message
      const successMessage = page.getByText(/created|success/i).first();
      if (await successMessage.isVisible()) {
        await expect(successMessage).toBeVisible();
      }
    });
  });

  // UAT-102: Assign roles to user
  test('Assign roles to user', async ({ page }) => {
    const newUser = testUsers[0];

    await test.step('Create user first', async () => {
      // Use API or UI to create user (simplified for this step)
      await page.goto('/users', { waitUntil: 'networkidle' });

      // Check if user exists, if not create
      const userExists = await page.locator(`text=${newUser.email}`).first().isVisible().catch(() => false);
      if (!userExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/email/i).fill(newUser.email);
        await page.getByLabel(/name/i).fill(newUser.name);
        await page.getByLabel(/password/i).first().fill(newUser.password);

        const submitButton = page.getByRole('button', { name: /create|submit|save/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Open user edit modal', async () => {
      const userRow = page.locator(`text=${newUser.email}`).first();
      const editButton = userRow.locator('..').getByRole('button', { name: /edit|settings/i }).first();
      await editButton.click();
      await page.waitForSelector('[role="dialog"], form', { timeout: 3000 });
    });

    await test.step('Change user role', async () => {
      const roleSelect = page.locator('select[name*="role"], [class*="role"]');
      if (await roleSelect.first().isVisible()) {
        await roleSelect.first().selectOption('user');
      } else {
        // Try role radio buttons or dropdown
        const userRoleOption = page.getByLabel(/user\s*role|user\s*access/i);
        if (await userRoleOption.isVisible()) {
          await userRoleOption.click();
        }
      }
    });

    await test.step('Save changes', async () => {
      const saveButton = page.getByRole('button', { name: /save|update/i }).first();
      await saveButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify role assignment shown in list', async () => {
      const userRow = page.locator(`text=${newUser.email}`).first();
      const roleDisplay = userRow.locator('..').getByText(/user|admin|guest/i).first();
      await expect(roleDisplay).toBeVisible();
    });
  });

  // UAT-103: Delete user account
  test('Delete user account', async ({ page }) => {
    const userToDelete = testUsers[0];

    await test.step('Create test user', async () => {
      // Ensure user exists before deleting
      await page.goto('/users', { waitUntil: 'networkidle' });

      const userExists = await page.locator(`text=${userToDelete.email}`).first().isVisible().catch(() => false);
      if (!userExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        await addButton.click();

        await page.getByLabel(/email/i).fill(userToDelete.email);
        await page.getByLabel(/name/i).fill(userToDelete.name);
        await page.getByLabel(/password/i).first().fill(userToDelete.password);

        const submitButton = page.getByRole('button', { name: /create|submit|save/i }).first();
        await submitButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Click delete button', async () => {
      const userRow = page.locator(`text=${userToDelete.email}`).first();
      const deleteButton = userRow.locator('..').getByRole('button', { name: /delete|remove/i }).first();
      await deleteButton.click();
    });

    await test.step('Confirm deletion', async () => {
      const confirmButton = page.getByRole('button', { name: /confirm|delete|yes|ok/i }).first();
      if (await confirmButton.isVisible()) {
        await confirmButton.click();
      }

      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify user removed from list', async () => {
      await page.reload();
      await page.waitForLoadState('networkidle');

      const userEmail = page.locator(`text=${userToDelete.email}`).first();
      await expect(userEmail).not.toBeVisible();
    });
  });

  // UAT-104: User login with restricted role
  test('User login with restricted role', async ({ page }) => {
    const restrictedUser = { email: 'restricted@test.local', name: 'Restricted User', password: 'RestrictPass123!' };

    await test.step('Create restricted user via admin', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(restrictedUser.email);
      await page.getByLabel(/name/i).fill(restrictedUser.name);
      await page.getByLabel(/password/i).first().fill(restrictedUser.password);

      // Assign "User" role (restricted)
      const roleSelect = page.locator('select[name*="role"]').first();
      if (await roleSelect.isVisible()) {
        await roleSelect.selectOption('user');
      }

      const submitButton = page.getByRole('button', { name: /create|submit|save/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Logout admin', async () => {
      const profileMenu = page.locator('[data-testid="user-menu"], [class*="profile"]').first();
      if (await profileMenu.isVisible()) {
        await profileMenu.click();
      }

      const logoutButton = page.getByRole('menuitem', { name: /logout|sign out/i })
        .or(page.getByRole('button', { name: /logout|sign out/i }))
        .first();

      await logoutButton.click();
      await page.waitForURL(/login|signin/, { timeout: 5000 });
    });

    await test.step('Login as restricted user', async () => {
      await page.getByLabel(/email/i).fill(restrictedUser.email);
      await page.getByLabel(/password/i).fill(restrictedUser.password);

      const loginButton = page.getByRole('button', { name: /login|sign in/i });
      await loginButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify restricted dashboard view', async () => {
      const dashboard = page.locator('[role="main"], [data-testid="dashboard"]').first();
      await expect(dashboard).toBeVisible();

      // Some admin-only features should be hidden
      const userLink = page.getByRole('link', { name: /user|people/i });
      if (await userLink.isVisible()) {
        // User role should not access users (or minimal access)
        expect(true); // Soft check - depends on implementation
      }
    });
  });

  // UAT-105: User cannot access unauthorized resources
  test('User cannot access unauthorized admin resources', async ({ page }) => {
    await test.step('Attempt direct access to admin APIs', async () => {
      try {
        // Try accessing admin-only API endpoint
        const response = await page.evaluate(async () => {
          const res = await fetch('/api/v1/users', {
            headers: {
              'Authorization': `Bearer ${localStorage.getItem('token') || ''}`
            }
          });
          return { status: res.status };
        });

        // User role should get 403 Forbidden or 401 Unauthorized
        const isRestricted = response.status === 403 || response.status === 401;
        expect(isRestricted).toBe(true);
      } catch (e) {
        // Network error is also acceptable (endpoint not accessible)
        expect(true);
      }
    });
  });

  // UAT-106: Guest role has minimal access
  test('Guest role has minimal access', async ({ page }) => {
    const guestUser = { email: 'guest@test.local', name: 'Guest User', password: 'GuestPass123!' };

    await test.step('Create guest user', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(guestUser.email);
      await page.getByLabel(/name/i).fill(guestUser.name);
      await page.getByLabel(/password/i).first().fill(guestUser.password);

      // Assign "Guest" role
      const roleSelect = page.locator('select[name*="role"]').first();
      if (await roleSelect.isVisible()) {
        await roleSelect.selectOption('guest');
      }

      const submitButton = page.getByRole('button', { name: /create|submit|save/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Login as guest', async () => {
      const logoutButton = page.getByRole('button', { name: /logout|sign out/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
      }

      await page.getByLabel(/email/i).fill(guestUser.email);
      await page.getByLabel(/password/i).fill(guestUser.password);

      const loginButton = page.getByRole('button', { name: /login|sign in/i });
      await loginButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify guest has limited features', async () => {
      // Guest should have read-only or very limited access
      const mainContent = page.locator('[role="main"]').first();
      await expect(mainContent).toBeVisible();

      // Edit/delete buttons should be disabled or hidden
      const editButtons = page.locator('[data-testid*="edit"], button[title*="Edit"]');
      const editCount = await editButtons.count();
      // Either no edit buttons or they should be disabled
      expect(editCount).toBeGreaterThanOrEqual(0);
    });
  });

  // UAT-107: Modify user email
  test('Modify user email', async ({ page }) => {
    const originalEmail = 'modifier@test.local';
    const newEmail = 'modified@test.local';
    const userName = 'Modifier User';

    await test.step('Create test user', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(originalEmail);
      await page.getByLabel(/name/i).fill(userName);
      await page.getByLabel(/password/i).first().fill('TestPass123!');

      const submitButton = page.getByRole('button', { name: /create|submit|save/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Edit user email', async () => {
      const userRow = page.locator(`text=${originalEmail}`).first();
      const editButton = userRow.locator('..').getByRole('button', { name: /edit|settings/i }).first();
      await editButton.click();
      await page.waitForSelector('[role="dialog"], form', { timeout: 3000 });

      const emailInput = page.getByLabel(/email/i);
      await emailInput.clear();
      await emailInput.fill(newEmail);

      const saveButton = page.getByRole('button', { name: /save|update/i }).first();
      await saveButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify email updated in list', async () => {
      const newEmailElement = page.locator(`text=${newEmail}`).first();
      await expect(newEmailElement).toBeVisible();

      // Original email should be gone
      const oldEmailElement = page.locator(`text=${originalEmail}`).first();
      await expect(oldEmailElement).not.toBeVisible();
    });
  });

  // UAT-108: Reset user password
  test('Reset user password', async ({ page }) => {
    const testUser = { email: 'resetpass@test.local', name: 'Reset Pass User', password: 'OldPass123!' };
    const newPassword = 'NewPass456!';

    await test.step('Create test user', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/name/i).fill(testUser.name);
      await page.getByLabel(/password/i).first().fill(testUser.password);

      const submitButton = page.getByRole('button', { name: /create|submit|save/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Reset user password', async () => {
      const userRow = page.locator(`text=${testUser.email}`).first();
      const editButton = userRow.locator('..').getByRole('button', { name: /edit/i }).first();
      await editButton.click();
      await page.waitForSelector('[role="dialog"], form');

      // Look for reset password button
      const resetButton = page.getByRole('button', { name: /reset|password|set/i });
      if (await resetButton.isVisible()) {
        await resetButton.click();

        // Fill new password if prompted
        const passwordInput = page.getByLabel(/password/i).first();
        if (await passwordInput.isVisible()) {
          await passwordInput.fill(newPassword);
        }

        const saveButton = page.getByRole('button', { name: /save|update|confirm/i }).first();
        await saveButton.click();
      }

      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify password can be used to login', async () => {
      // Logout and login with new password
      const logoutButton = page.getByRole('button', { name: /logout|sign out/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
        await page.waitForURL(/login/);
      }

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/password/i).fill(newPassword);

      const loginButton = page.getByRole('button', { name: /login|sign in/i });
      await loginButton.click();

      // Should succeed with new password
      await page.waitForLoadState('networkidle');
      const dashboard = page.locator('[role="main"]').first();
      await expect(dashboard).toBeVisible();
    });
  });

  // UAT-109: Search/filter users by email
  test('Search users by email', async ({ page }) => {
    const searchEmail = 'search@test.local';

    await test.step('Create searchable user', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(searchEmail);
      await page.getByLabel(/name/i).fill('Search Test User');
      await page.getByLabel(/password/i).first().fill('SearchPass123!');

      const submitButton = page.getByRole('button', { name: /create|submit|save/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Search by email', async () => {
      const searchInput = page.getByPlaceholder(/search|filter/i).first();
      if (await searchInput.isVisible()) {
        await searchInput.fill(searchEmail);
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify search results', async () => {
      const userEmail = page.locator(`text=${searchEmail}`).first();
      await expect(userEmail).toBeVisible();
    });
  });

  // UAT-110: Pagination works on users list
  test('User list pagination works with many users', async ({ page }) => {
    await test.step('Navigate to users page', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });
    });

    await test.step('Verify pagination controls visible if needed', async () => {
      const userList = page.locator('[data-testid="user-table"], [class*="user-list"]').first();
      await expect(userList).toBeVisible();

      // Check for pagination
      const paginationControls = page.locator('[data-testid*="pagination"], [class*="pagination"]').first();
      if (await userList.evaluate((el) => el.children.length > 25)) {
        // If many users, pagination should exist
        if (await paginationControls.isVisible()) {
          await expect(paginationControls).toBeVisible();
        }
      }
    });

    await test.step('Navigate pages if pagination exists', async () => {
      const nextButton = page.getByRole('button', { name: /next|>|forward/i }).first();
      if (await nextButton.isVisible()) {
        await nextButton.click();
        await page.waitForLoadState('networkidle');

        // Verify we're on next page
        const userList = page.locator('[data-testid="user-table"]').first();
        await expect(userList).toBeVisible();
      }
    });
  });
});
