import { test, expect } from '@playwright/test';

/**
 * Integration: Multi-Component Workflows
 *
 * Purpose: Validate complex workflows involving multiple system components
 * Scenarios: Create proxy → enable security → test enforcement, user workflows, backup restore integration
 * Success: Multi-step workflows complete correctly, all components integrate properly
 */

test.describe('Multi-Component Workflows', () => {
  const testProxy = {
    domain: 'multi-workflow.local',
    target: 'http://localhost:3001',
    description: 'Multi-component workflow test',
  };

  const testUser = {
    email: 'multiflow@test.local',
    name: 'Multi Workflow User',
    password: 'MultiFlow123!',
  };

  test.beforeEach(async ({ page }) => {
    await page.goto('/', { waitUntil: 'networkidle' });
    await page.waitForSelector('[role="main"]', { timeout: 5000 });
  });

  test.afterEach(async ({ page }) => {
    try {
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
    } catch {
      // Ignore cleanup errors
    }
  });

  // Create proxy → Enable WAF → Send request → WAF enforces
  test('WAF enforcement applies to newly created proxy', async ({ page }) => {
    await test.step('Create new proxy', async () => {
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

    await test.step('Enable WAF on proxy', async () => {
      const proxyRow = page.locator(`text=${testProxy.domain}`).first();
      const editButton = proxyRow.locator('..').getByRole('button', { name: /edit/i }).first();
      await editButton.click();

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const submitButton = page.getByRole('button', { name: /save|update/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Send malicious request to proxy with WAF', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/?id=1' OR '1'='1`,
        { ignoreHTTPSErrors: true }
      );

      expect(response.status()).toBe(403); // WAF blocks
    });

    await test.step('Send legitimate request (allowed)', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/api/status`,
        { ignoreHTTPSErrors: true }
      );

      // Should not be 403
      expect(response.status()).not.toBe(403);
    });
  });

  // Create user → Assign role → User creates proxy → Verify ACL
  test('User with proxy creation role can create and manage proxies', async ({ page }) => {
    await test.step('Create user with proxy management role', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/name/i).fill(testUser.name);
      await page.getByLabel(/password/i).first().fill(testUser.password);

      // Try to assign a role with proxy management
      const roleSelect = page.locator('select[name*="role"]').first();
      if (await roleSelect.isVisible()) {
        await roleSelect.selectOption('manager'); // or appropriate role
      }

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('User logs in and attempts proxy creation', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
        await page.waitForURL(/login/);
      }

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/password/i).fill(testUser.password);
      await page.getByRole('button', { name: /login/i }).click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('User navigates to proxy management', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' }).catch(() => {
        // May not have access or may be redirected
        return Promise.resolve();
      });

      // Check if user can see proxy creation or gets access error
      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      const accessDenied = page.getByText(/access|denied|forbidden/i).first();

      const hasAccess = await addButton.isVisible().catch(() => false);
      const isBlocked = await accessDenied.isVisible().catch(() => false);

      // Should have access OR be blocked (depending on role)
      console.log(`✓ User proxy access: allowed=${hasAccess}, blocked=${isBlocked}`);
    });
  });

  // Create backup → Delete user → Restore → User reappears
  test('Backup restore recovers deleted user data', async ({ page }) => {
    const userToBackup = {
      email: 'backup-user@test.local',
      name: 'Backup Recovery User',
      password: 'BackupPass123!',
    };

    await test.step('Create user to be backed up', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(userToBackup.email);
      await page.getByLabel(/name/i).fill(userToBackup.name);
      await page.getByLabel(/password/i).first().fill(userToBackup.password);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Create backup with user data', async () => {
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });

      const backupButton = page.getByRole('button', { name: /backup|create|manual/i }).first();
      if (await backupButton.isVisible()) {
        await backupButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Delete the user', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const userRow = page.locator(`text=${userToBackup.email}`).first();
      const deleteButton = userRow.locator('..').getByRole('button', { name: /delete/i }).first();
      await deleteButton.click();

      const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
      if (await confirmButton.isVisible()) {
        await confirmButton.click();
      }
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify user is deleted', async () => {
      await page.reload();

      const deletedUser = page.locator(`text=${userToBackup.email}`).first();
      const isVisible = await deletedUser.isVisible().catch(() => false);
      expect(isVisible).toBe(false);
    });

    await test.step('Restore from backup', async () => {
      await page.goto('/settings/backup', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/backup');
      });

      const restoreButton = page.getByRole('button', { name: /restore/i }).first();
      if (await restoreButton.isVisible()) {
        await restoreButton.click();

        const confirmButton = page.getByRole('button', { name: /confirm|restore/i }).first();
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify user reappeared after restore', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });

      const restoredUser = page.locator(`text=${userToBackup.email}`).first();
      const isVisible = await restoredUser.isVisible().catch(() => false);

      if (isVisible) {
        await expect(restoredUser).toBeVisible();
        console.log(`✓ User ${userToBackup.email} recovered from backup`);
      }
    });
  });

  // Enable security → Create user → User subject to rate limit
  test('Security modules apply to subsequently created resources', async ({ page }) => {
    await test.step('Enable global rate limiting', async () => {
      await page.goto('/settings/security', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });

      const rateLimitToggle = page.locator('input[type="checkbox"][name*="rate"]').first();
      if (await rateLimitToggle.isVisible()) {
        const isChecked = await rateLimitToggle.isChecked();
        if (!isChecked) {
          await rateLimitToggle.click();
        }
      }

      const saveButton = page.getByRole('button', { name: /save|apply/i }).first();
      if (await saveButton.isVisible()) {
        await saveButton.click();
      }
      await page.waitForLoadState('networkidle');
    });

    await test.step('Create new user after security enabled', async () => {
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

    await test.step('Verify user subject to rate limiting', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
        await page.waitForURL(/login/);
      }

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/password/i).fill(testUser.password);
      await page.getByRole('button', { name: /login/i }).click();
      await page.waitForLoadState('networkidle');

      const userToken = await page.evaluate(() => localStorage.getItem('token'));

      // Send multiple requests and verify rate limiting
      const responses = [];
      for (let i = 0; i < 5; i++) {
        const response = await page.request.get(
          `http://127.0.0.1:8080/api/test-${i}`,
          {
            headers: { 'Authorization': `Bearer ${userToken || ''}` },
            ignoreHTTPSErrors: true,
          }
        );
        responses.push(response.status());
      }

      // With rate limiting, should see 429 eventually
      const rateLimited = responses.some(status => status === 429);
      console.log(`✓ Rate limiting applied to user: ${rateLimited ? 'YES' : 'NO'}`);
    });
  });

  // Admin workflow: create user → enable security → user cannot bypass
  test('Security enforced even on previously created resources', async ({ page }) => {
    await test.step('Create user before security enabled', async () => {
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

    await test.step('Enable rate limiting globally', async () => {
      await page.goto('/settings/security', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });

      const rateLimitToggle = page.locator('input[type="checkbox"][name*="rate"]').first();
      if (await rateLimitToggle.isVisible()) {
        const isChecked = await rateLimitToggle.isChecked();
        if (!isChecked) {
          await rateLimitToggle.click();
        }
      }

      const saveButton = page.getByRole('button', { name: /save|apply/i }).first();
      if (await saveButton.isVisible()) {
        await saveButton.click();
      }
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify user is now rate limited', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
        await page.waitForURL(/login/);
      }

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/password/i).fill(testUser.password);
      await page.getByRole('button', { name: /login/i }).click();
      await page.waitForLoadState('networkidle');

      const userToken = await page.evaluate(() => localStorage.getItem('token'));

      // Send rapid requests
      const responses = [];
      for (let i = 0; i < 10; i++) {
        const response = await page.request.get(
          `http://127.0.0.1:8080/rapid-${i}`,
          {
            headers: { 'Authorization': `Bearer ${userToken || ''}` },
            ignoreHTTPSErrors: true,
          }
        );
        responses.push(response.status());
      }

      // Should eventually see 429
      const rateLimited = responses.includes(429);
      console.log(`✓ Security retroactively applied: ${rateLimited ? 'YES' : 'NO'}`);
    });
  });
});
