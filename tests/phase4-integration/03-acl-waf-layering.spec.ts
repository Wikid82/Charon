import { test, expect } from '@playwright/test';

/**
 * Phase 4 Integration: ACL & WAF Layering (Defense in Depth)
 *
 * Purpose: Validate ACL and WAF work as defense-in-depth layers
 * Scenarios: Both modules apply, WAF independent of role, ACL independent of payload
 * Success: Malicious requests blocked regardless of role, unauthorized users blocked regardless of payload
 */

test.describe('INT-003: ACL & WAF Layering', () => {
  const testProxy = {
    domain: 'acl-waf-test.local',
    target: 'http://localhost:3001',
    description: 'Test proxy for ACL and WAF layering',
  };

  const testUser = {
    email: 'aclusertest@test.local',
    name: 'ACL User Test',
    password: 'ACLUserPass123!',
  };

  test.beforeEach(async ({ page }) => {
    await page.goto('/', { waitUntil: 'networkidle' });
    await page.waitForSelector('[role="main"]', { timeout: 5000 });
  });

  test.afterEach(async ({ page }) => {
    try {
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
    } catch {
      // Ignore cleanup errors
    }
  });

  // INT-003-1: Non-admin user cannot bypass WAF even with proxy access
  test('Regular user cannot bypass WAF on authorized proxy', async ({ page }) => {
    await test.step('Admin creates test user with limited permissions', async () => {
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

    await test.step('Admin creates proxy with WAF enabled', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('User logs in', async () => {
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

    await test.step('User sends malicious request to proxy', async () => {
      const response = await page.request.get(
        `http://127.0.0.1:8080/?id=1' OR '1'='1`,
        {
          headers: { 'Authorization': await page.evaluate(() => localStorage.getItem('token') || '') },
          ignoreHTTPSErrors: true,
        }
      );

      // WAF blocks regardless of user privilege
      expect(response.status()).toBe(403);
    });
  });

  // INT-003-2: WAF enforces regardless of user role
  test('WAF blocks malicious requests from all user roles', async ({ page }) => {
    await test.step('Create proxy with WAF', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Admin sends malicious request', async () => {
      const adminToken = await page.evaluate(() => localStorage.getItem('token'));

      const response = await page.request.post(
        `http://127.0.0.1:8080/api/test`,
        {
          data: { payload: `<script>alert('xss')</script>` },
          headers: { 'Authorization': adminToken || '' },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.status()).toBe(403);
    });

    await test.step('Non-admin also blocked by WAF', async () => {
      // Create and login non-admin
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/name/i).fill(testUser.name);
      await page.getByLabel(/password/i).first().fill(testUser.password);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');

      // Logout and login as user
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      await logoutButton.click();
      await page.waitForURL(/login/);

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/password/i).fill(testUser.password);
      await page.getByRole('button', { name: /login/i }).click();
      await page.waitForLoadState('networkidle');

      const userToken = await page.evaluate(() => localStorage.getItem('token'));

      const response = await page.request.post(
        `http://127.0.0.1:8080/api/test`,
        {
          data: { payload: `'; DROP TABLE users;--` },
          headers: { 'Authorization': userToken || '' },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.status()).toBe(403);
    });
  });

  // INT-003-3: Admin and user both subject to WAF and ACL
  test('Both admin and user roles subject to WAF protection', async ({ page }) => {
    await test.step('Setup: Create proxy and user', async () => {
      // Create user
      await page.goto('/users', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/name/i).fill(testUser.name);
      await page.getByLabel(/password/i).first().fill(testUser.password);

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');

      // Create proxy with WAF
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const createButton = page.getByRole('button', { name: /add|create/i }).first();
      await createButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const proxySubmit = page.getByRole('button', { name: /create|submit/i }).first();
      await proxySubmit.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify admin blocked by WAF', async () => {
      const adminToken = await page.evaluate(() => localStorage.getItem('token'));

      const response = await page.request.get(
        `http://127.0.0.1:8080/?cmd=env`,
        {
          headers: { 'Authorization': adminToken || '' },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.status()).toBe(403);
    });

    await test.step('Verify user also blocked by WAF', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      await logoutButton.click();
      await page.waitForURL(/login/);

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/password/i).fill(testUser.password);
      await page.getByRole('button', { name: /login/i }).click();
      await page.waitForLoadState('networkidle');

      const userToken = await page.evaluate(() => localStorage.getItem('token'));

      const response = await page.request.get(
        `http://127.0.0.1:8080/?cmd=whoami`,
        {
          headers: { 'Authorization': userToken || '' },
          ignoreHTTPSErrors: true,
        }
      );

      expect(response.status()).toBe(403);
    });
  });

  // INT-003-4: ACL adds layer beyond WAF (defense in depth)
  test('ACL restricts access beyond WAF protection', async ({ page }) => {
    await test.step('Create restricted user', async () => {
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

    await test.step('Create proxy with WAF but restrict access via ACL', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/domain/i).fill(testProxy.domain);
      await page.getByLabel(/target|forward/i).fill(testProxy.target);
      await page.getByLabel(/description/i).fill(testProxy.description);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      // Setup ACL to restrict access
      const aclInput = page.locator('input[name*="acl"], textarea[name*="acl"]').first();
      if (await aclInput.isVisible()) {
        await aclInput.fill('admin_only');
      }

      const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('User with ACL restriction gets blocked', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      await logoutButton.click();
      await page.waitForURL(/login/);

      await page.getByLabel(/email/i).fill(testUser.email);
      await page.getByLabel(/password/i).fill(testUser.password);
      await page.getByRole('button', { name: /login/i }).click();
      await page.waitForLoadState('networkidle');

      const userToken = await page.evaluate(() => localStorage.getItem('token'));

      const response = await page.request.get(
        `http://127.0.0.1:8080/public`,
        {
          headers: { 'Authorization': userToken || '' },
          ignoreHTTPSErrors: true,
        }
      );

      // Should get 401/403 from ACL before reaching WAF check
      expect([401, 403]).toContain(response.status());
    });
  });
});
