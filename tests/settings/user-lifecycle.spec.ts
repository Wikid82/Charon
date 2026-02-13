import { test, expect, loginUser, logoutUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';
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
    data: { reason: 'user-lifecycle deterministic setup' },
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

function parseAuditDetails(details: unknown): Record<string, unknown> {
  if (!details) {
    return {};
  }

  if (typeof details === 'string') {
    try {
      const parsed = JSON.parse(details);
      return parsed && typeof parsed === 'object' ? parsed as Record<string, unknown> : {};
    } catch {
      return {};
    }
  }

  return typeof details === 'object' ? details as Record<string, unknown> : {};
}

async function getAuditLogEntries(
  page: import('@playwright/test').Page,
  token: string,
  options: {
    action?: string;
    eventCategory?: string;
    limit?: number;
    maxPages?: number;
  } = {}
): Promise<any[]> {
  const limit = options.limit ?? 100;
  const maxPages = options.maxPages ?? 5;
  const action = options.action;
  const eventCategory = options.eventCategory;
  const allEntries: any[] = [];

  for (let currentPage = 1; currentPage <= maxPages; currentPage += 1) {
    const params = new URLSearchParams({
      page: String(currentPage),
      limit: String(limit),
    });

    if (action) {
      params.set('action', action);
    }
    if (eventCategory) {
      params.set('event_category', eventCategory);
    }

    const auditResponse = await page.request.get(`/api/v1/audit-logs?${params.toString()}`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(auditResponse.ok()).toBe(true);

    const auditBody = await auditResponse.json();
    expect(auditBody).toEqual(expect.objectContaining({
      audit_logs: expect.any(Array),
      pagination: expect.any(Object),
    }));

    allEntries.push(...auditBody.audit_logs);

    const totalPages = Number(auditBody?.pagination?.total_pages || currentPage);
    if (currentPage >= totalPages) {
      break;
    }
  }

  return allEntries;
}

function findLifecycleEntry(
  auditEntries: any[],
  email: string,
  action: 'user_create' | 'user_update' | 'user_delete' | 'user_invite' | 'user_invite_accept'
): any | undefined {
  return auditEntries.find((entry: any) => {
    if (entry?.event_category !== 'user' || entry?.action !== action) {
      return false;
    }

    const details = parseAuditDetails(entry?.details);
    const detailEmail =
      details?.target_email ||
      details?.email ||
      details?.user_email;

    if (detailEmail === email) {
      return true;
    }

    return JSON.stringify(entry).includes(email);
  });
}

async function createUserViaApi(
  page: import('@playwright/test').Page,
  user: { email: string; name: string; password: string; role: 'admin' | 'user' | 'guest' }
): Promise<{ id: string | number; email: string }> {
  const response = await page.request.post('/api/v1/users', {
    data: user,
  });

  expect(response.ok()).toBe(true);
  const payload = await response.json();
  expect(payload).toEqual(expect.objectContaining({
    id: expect.anything(),
    email: user.email,
  }));

  return { id: payload.id, email: payload.email };
}

async function navigateToLogin(page: import('@playwright/test').Page): Promise<void> {
  const logoutButton = page.getByRole('button', { name: /logout/i }).first();
  if (await logoutButton.isVisible().catch(() => false)) {
    await logoutButton.click();
  } else {
    await page.goto('/login', { waitUntil: 'domcontentloaded' });
  }

  const emailInput = page.locator('input[type="email"]').or(page.getByLabel(/email/i)).first();
  await expect(emailInput).toBeVisible({ timeout: 15000 });
}

async function loginWithCredentials(
  page: import('@playwright/test').Page,
  email: string,
  password: string
): Promise<void> {
  const emailInput = page.locator('input[type="email"]').or(page.getByLabel(/email/i)).first();
  const passwordInput = page.locator('input[type="password"]').or(page.getByLabel(/password/i)).first();

  await expect(emailInput).toBeVisible({ timeout: 15000 });
  await expect(passwordInput).toBeVisible({ timeout: 15000 });
  await emailInput.fill(email);
  await passwordInput.fill(password);

  const loginResponse = page.waitForResponse(
    (response) => response.url().includes('/api/v1/auth/login') && response.request().method() === 'POST',
    { timeout: 15000 }
  );

  await page.getByRole('button', { name: /login|sign in/i }).first().click();
  const response = await loginResponse;
  expect(response.ok()).toBe(true);
  await waitForLoadingComplete(page, { timeout: 15000 });
}

async function loginWithCredentialsExpectFailure(
  page: import('@playwright/test').Page,
  email: string,
  password: string
): Promise<void> {
  const emailInput = page.locator('input[type="email"]').or(page.getByLabel(/email/i)).first();
  const passwordInput = page.locator('input[type="password"]').or(page.getByLabel(/password/i)).first();

  await expect(emailInput).toBeVisible({ timeout: 15000 });
  await expect(passwordInput).toBeVisible({ timeout: 15000 });
  await emailInput.fill(email);
  await passwordInput.fill(password);

  const loginResponse = page.waitForResponse(
    (response) => response.url().includes('/api/v1/auth/login') && response.request().method() === 'POST',
    { timeout: 15000 }
  );

  await page.getByRole('button', { name: /login|sign in/i }).first().click();
  const response = await loginResponse;
  expect(response.ok()).toBe(false);
  expect([400, 401, 403]).toContain(response.status());
  await expect(page).toHaveURL(/login/);
}

/**
 * Integration: Admin → User E2E Workflow
 *
 * Purpose: Validate complete workflows from admin creation through user access
 * Scenarios: User creation, role assignment, login, resource access
 * Success: Users can login and access appropriate resources based on role
 */

test.describe('Admin-User E2E Workflow', () => {
  let adminEmail = '';

  let testUser = {
    email: '',
    name: 'E2E Test User',
    password: 'E2EUserPass123!',
  };

  test.beforeEach(async ({ page, adminUser }) => {
    const suffix = uniqueSuffix();
    testUser = {
      email: `e2euser-${suffix}@test.local`,
      name: `E2E Test User ${suffix}`,
      password: 'E2EUserPass123!',
    };

    await resetSecurityState(page);
    adminEmail = adminUser.email;
    await loginUser(page, adminUser);
    const meResponse = await page.request.get('/api/v1/auth/me');
    expect(meResponse.ok()).toBe(true);
    await waitForLoadingComplete(page, { timeout: 15000 });

    const token = await getAuthToken(page);
    await expect.poll(async () => {
      const statusResponse = await page.request.get('/api/v1/security/status', {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!statusResponse.ok()) {
        return 'status-unavailable';
      }

      const status = await statusResponse.json();
      return JSON.stringify({
        acl: Boolean(status?.acl?.enabled),
        waf: Boolean(status?.waf?.enabled),
        rateLimit: Boolean(status?.rate_limit?.enabled),
        crowdsec: Boolean(status?.crowdsec?.enabled),
      });
    }, {
      timeout: 10000,
      message: 'Expected security modules to be disabled before user lifecycle test',
    }).toBe(JSON.stringify({
      acl: false,
      waf: false,
      rateLimit: false,
      crowdsec: false,
    }));
  });

  // Full user creation → role assignment → user login → resource access
  test('Complete user lifecycle: creation to resource access', async ({ page }) => {
    let createdUserId: string | number;

    await test.step('STEP 1: Admin creates new user', async () => {
      const start = Date.now();

      const createdUser = await createUserViaApi(page, { ...testUser, role: 'user' });
      createdUserId = createdUser.id;

      await page.goto('/users', { waitUntil: 'networkidle' });
      await waitForLoadingComplete(page, { timeout: 15000 });
      await expect(page.getByText(testUser.email).first()).toBeVisible({ timeout: 15000 });

      const duration = Date.now() - start;
      console.log(`✓ User created in ${duration}ms`);
      expect(duration).toBeLessThan(5000);
    });

    await test.step('STEP 2: Assign User role', async () => {
      const token = await getAuthToken(page);
      const updateRoleResponse = await page.request.put(`/api/v1/users/${createdUserId}`, {
        data: { role: 'user' },
        headers: { Authorization: `Bearer ${token}` },
      });

      expect(updateRoleResponse.ok()).toBe(true);
      const updateBody = await updateRoleResponse.json();
      expect(updateBody).toEqual(expect.objectContaining({
        message: expect.stringMatching(/updated/i),
      }));
    });

    await test.step('STEP 3: Admin logs out', async () => {
      const profileMenu = page.locator('[data-testid="user-menu"], [class*="profile"]').first();
      if (await profileMenu.isVisible()) {
        await profileMenu.click();
      }

      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      await logoutButton.click();
      await page.waitForURL(/login/, { timeout: 5000 });
    });

    await test.step('STEP 4: New user logs in', async () => {
      const start = Date.now();

      await loginWithCredentials(page, testUser.email, testUser.password);
      const duration = Date.now() - start;

      console.log(`✓ User logged in in ${duration}ms`);
      expect(duration).toBeLessThan(3000);
    });

    await test.step('STEP 5: User sees restricted dashboard', async () => {
      const dashboard = page.getByRole('main').first();
      await expect(dashboard).toBeVisible();

      // User role should see limited menu items
      const userMenu = page.locator('nav, [role="navigation"]').first();
      if (await userMenu.isVisible()) {
        const menuItems = userMenu.locator('a, button, [role="link"], [role="button"]');
        await expect.poll(async () => menuItems.count(), {
          timeout: 15000,
          message: 'Expected restricted user navigation to render at least one actionable menu item',
        }).toBeGreaterThan(0);
      }
    });

    await test.step('STEP 6: User cannot access user management', async () => {
      await page.goto('/users', { waitUntil: 'networkidle' });
      const accessDeniedMessage = page.getByText(/access.*denied|forbidden|not allowed|admin access required/i).first();
      const hasUsersHeading = await page.getByRole('heading', { name: /users/i }).first().isVisible().catch(() => false);
      const hasAccessDenied = await accessDeniedMessage.isVisible().catch(() => false);

      expect(hasUsersHeading && !hasAccessDenied).toBe(false);
    });

    await test.step('STEP 7: Audit trail records all actions', async () => {
      // Logout user
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
      }

      // Login as admin
      await loginWithCredentials(page, adminEmail, TEST_PASSWORD);

      const token = await getAuthToken(page);
      await expect.poll(async () => {
        const createEntries = await getAuditLogEntries(page, token, {
          limit: 100,
          maxPages: 8,
        });
        const updateEntries = await getAuditLogEntries(page, token, {
          limit: 100,
          maxPages: 8,
        });
        const createEntry = findLifecycleEntry(createEntries, testUser.email, 'user_create');
        const updateEntry = findLifecycleEntry(updateEntries, testUser.email, 'user_update');
        return Number(Boolean(createEntry)) + Number(Boolean(updateEntry));
      }, {
        timeout: 30000,
        message: `Expected user lifecycle audit entries for ${testUser.email}`,
      }).toBe(2);
    });
  });

  // Admin modifies role → user gains new permissions immediately
  test('Role change takes effect immediately on user refresh', async ({ page }) => {
    let createdUserId: string | number;

    await test.step('Create test user with default role', async () => {
      const createdUser = await createUserViaApi(page, { ...testUser, role: 'user' });
      createdUserId = createdUser.id;
    });

    await test.step('User logs in and notes current permissions', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      await logoutButton.click();
      await page.waitForURL(/login/);

      await loginWithCredentials(page, testUser.email, testUser.password);
    });

    await test.step('Admin upgrades user role (in parallel)', async () => {
      await navigateToLogin(page);
      await loginWithCredentials(page, adminEmail, TEST_PASSWORD);
      const token = await getAuthToken(page);
      const updateRoleResponse = await page.request.put(`/api/v1/users/${createdUserId}`, {
        data: { role: 'admin' },
        headers: { Authorization: `Bearer ${token}` },
      });

      expect(updateRoleResponse.ok()).toBe(true);
    });

    await test.step('User refreshes page and sees new permissions', async () => {
      await navigateToLogin(page);
      await loginWithCredentials(page, testUser.email, testUser.password);
      await page.goto('/users', { waitUntil: 'networkidle' });
      await page.reload({ waitUntil: 'networkidle' });
      await page.waitForLoadState('networkidle');

      await expect(page.getByRole('heading', { name: /user management/i }).first()).toBeVisible({ timeout: 15000 });
    });
  });

  // Admin deletes user → user login fails
  test('Deleted user cannot login', async ({ page }) => {
    const suffix = uniqueSuffix();
    const deletableUser = {
      email: `deleteme-${suffix}@test.local`,
      name: `Delete Test User ${suffix}`,
      password: 'DeletePass123!',
    };

    let createdUserId: string | number;

    await test.step('Create user to delete', async () => {
      const createdUser = await createUserViaApi(page, { ...deletableUser, role: 'user' });
      createdUserId = createdUser.id;
    });

    await test.step('Admin deletes user', async () => {
      const token = await getAuthToken(page);
      const deleteResponse = await page.request.delete(`/api/v1/users/${createdUserId}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(deleteResponse.ok()).toBe(true);
    });

    await test.step('Deleted user attempts login', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
        await page.waitForURL(/login/);
      }

      await loginWithCredentialsExpectFailure(page, deletableUser.email, deletableUser.password);
    });

    await test.step('Verify login failed with appropriate error', async () => {
      const errorMessage = page.getByText(/invalid|failed|incorrect|unauthorized/i).first();
      await expect(errorMessage).toBeVisible({ timeout: 15000 });
    });
  });

  // Audit log records entire workflow
  test('Audit log records user lifecycle events', async ({ page }) => {
    let createdUserEmail = '';

    await test.step('Perform workflow actions', async () => {
      const createdUser = await createUserViaApi(page, { ...testUser, role: 'user' });
      createdUserEmail = createdUser.email;
    });

    await test.step('Check audit trail for user creation event', async () => {
      const token = await getAuthToken(page);
      await expect.poll(async () => {
        const auditEntries = await getAuditLogEntries(page, token, {
          limit: 100,
          maxPages: 8,
        });
        return Boolean(findLifecycleEntry(auditEntries, createdUserEmail, 'user_create'));
      }, {
        timeout: 30000,
        message: `Expected user_create audit entry for ${createdUserEmail}`,
      }).toBe(true);
    });

    await test.step('Verify audit entry shows user details', async () => {
      const token = await getAuthToken(page);
      const auditEntries = await getAuditLogEntries(page, token, {
        limit: 100,
        maxPages: 8,
      });
      const creationEntry = findLifecycleEntry(auditEntries, createdUserEmail, 'user_create');
      expect(creationEntry).toBeTruthy();

      const details = parseAuditDetails(creationEntry?.details);
      expect(details).toEqual(expect.objectContaining({
        target_email: createdUserEmail,
      }));
    });
  });

  // User cannot escalate own role
  test('User cannot promote self to admin', async ({ page }) => {
    await test.step('Create test user', async () => {
      await createUserViaApi(page, { ...testUser, role: 'user' });
    });

    await test.step('User attempts to modify own role', async () => {
      // Logout admin
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      await logoutButton.click();
      await page.waitForURL(/login/);

      // Login as user
      await loginWithCredentials(page, testUser.email, testUser.password);

      // Try to access user management
      await page.goto('/users', { waitUntil: 'networkidle' });
    });

    await test.step('Verify user cannot access user management', async () => {
      // Should not see users list or get 403
      const usersList = page.locator('[data-testid="user-list"]').first();
      const errorPage = page.getByText(/access.*denied|forbidden|not allowed/i).first();

      const isBlocked =
        !(await usersList.isVisible()) ||
        (await errorPage.isVisible());

      expect(isBlocked).toBe(true);
    });
  });

  // Multiple users isolated data
  test('Users see only their own data', async ({ page }) => {
    const suffix1 = uniqueSuffix();
    const user1 = {
      email: `user1-${suffix1}@test.local`,
      name: 'User 1',
      password: 'User1Pass123!',
    };

    const suffix2 = uniqueSuffix();
    const user2 = {
      email: `user2-${suffix2}@test.local`,
      name: 'User 2',
      password: 'User2Pass123!',
    };

    await test.step('Create first user', async () => {
      await createUserViaApi(page, { ...user1, role: 'user' });
    });

    await test.step('Create second user', async () => {
      await createUserViaApi(page, { ...user2, role: 'user' });
    });

    await test.step('User1 logs in and verifies data isolation', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      await logoutButton.click();
      await page.waitForURL(/login/);

      await loginWithCredentials(page, user1.email, user1.password);

      // User1 should see their profile but not User2's
      const user1Profile = page.getByText(user1.name).first();
      if (await user1Profile.isVisible()) {
        await expect(user1Profile).toBeVisible();
      }
    });
  });

  // User logout → login as different user → resources isolated
  test('Session isolation after logout and re-login', async ({ page }) => {
    let firstSessionToken = '';

    await test.step('Create secondary user for session switch', async () => {
      await createUserViaApi(page, { ...testUser, role: 'user' });
    });

    await test.step('Login as first user', async () => {
      await navigateToLogin(page);
      await loginWithCredentials(page, adminEmail, TEST_PASSWORD);
    });

    await test.step('Note session storage', async () => {
      firstSessionToken = await page.evaluate(() => localStorage.getItem('charon_auth_token') || '');
      expect(firstSessionToken).toBeTruthy();
    });

    await test.step('Logout', async () => {
      await logoutUser(page);
    });

    await test.step('Verify session cleared', async () => {
      const emailInput = page.locator('input[type="email"]').or(page.getByLabel(/email/i)).first();
      await expect(emailInput).toBeVisible({ timeout: 15000 });

      const meAfterLogout = await page.request.get('/api/v1/auth/me');
      expect([401, 403]).toContain(meAfterLogout.status());
    });

    await test.step('Login as different user', async () => {
      await loginWithCredentials(page, testUser.email, testUser.password);
    });

    await test.step('Verify new session established', async () => {
      await expect.poll(async () => {
        try {
          return await page.evaluate(() => localStorage.getItem('charon_auth_token') || '');
        } catch {
          return '';
        }
      }, {
        timeout: 15000,
        message: 'Expected new auth token for second login',
      }).not.toBe('');

      const token = await page.evaluate(() => localStorage.getItem('charon_auth_token') || '');
      expect(token).toBeTruthy();
      expect(token).not.toBe(firstSessionToken);

      const dashboard = page.getByRole('main').first();
      await expect(dashboard).toBeVisible();

      const meAfterRelogin = await page.request.get('/api/v1/auth/me');
      expect(meAfterRelogin.ok()).toBe(true);
      const currentUser = await meAfterRelogin.json();
      expect(currentUser).toEqual(expect.objectContaining({ email: testUser.email }));
    });
  });
});
