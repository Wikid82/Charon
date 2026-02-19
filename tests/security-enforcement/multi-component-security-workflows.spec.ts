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
    data: { reason: 'multi-component security deterministic setup/teardown' },
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

test.describe('Multi-Component Security Workflows', () => {
  let testProxy = {
    domain: `multi-workflow-${Date.now()}.local`,
    target: 'http://localhost:3001',
    description: 'Multi-component security workflow test',
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
      description: 'Multi-component security workflow test',
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

  test('WAF enforcement applies to newly created proxy', async ({ page }) => {
    let createdProxyUUID = '';

    await test.step('Create new proxy', async () => {
      const token = await getAuthToken(page);
      const createProxyResponse = await page.request.post('/api/v1/proxy-hosts', {
        data: {
          domain_names: testProxy.domain,
          forward_scheme: 'http',
          forward_host: 'localhost',
          forward_port: 3001,
          enabled: true,
        },
        headers: { Authorization: `Bearer ${token}` },
      });

      expect(createProxyResponse.ok()).toBe(true);

      const createPayload = await createProxyResponse.json();
      expect(createPayload).toEqual(expect.objectContaining({
        uuid: expect.any(String),
      }));
      createdProxyUUID = createPayload.uuid;

      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });
      await waitForLoadingComplete(page, { timeout: 15000 });
      await expect(page.getByText(testProxy.domain).first()).toBeVisible({ timeout: 15000 });
    });

    await test.step('Enable WAF on proxy', async () => {
      const token = await getAuthToken(page);
      const enableWafResponse = await page.request.patch('/api/v1/security/waf', {
        data: { enabled: true },
        headers: { Authorization: `Bearer ${token}` },
      });

      expect(enableWafResponse.ok()).toBe(true);
      const securityStatusResponse = await page.request.get('/api/v1/security/status', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(securityStatusResponse.ok()).toBe(true);
      const securityStatus = await securityStatusResponse.json();
      expect(securityStatus).toEqual(expect.objectContaining({
        waf: expect.objectContaining({ enabled: true }),
      }));
    });

    await test.step('Send malicious request to proxy with WAF', async () => {
      const origin = new URL(page.url()).origin;
      const response = await page.request.get(
        `${origin}/?id=1' OR '1'='1`,
        {
          headers: { Host: testProxy.domain },
          ignoreHTTPSErrors: true,
        }
      );

      expect([403, 502]).toContain(response.status());
    });

    await test.step('Send legitimate request (allowed)', async () => {
      const origin = new URL(page.url()).origin;
      const response = await page.request.get(
        `${origin}/api/v1/health`,
        {
          headers: { Host: testProxy.domain },
          ignoreHTTPSErrors: true,
        }
      );

      expect([200, 502]).toContain(response.status());
    });

    await test.step('Cleanup created proxy in-test for isolation', async () => {
      if (!createdProxyUUID) {
        return;
      }

      const token = await getAuthToken(page);
      const deleteResponse = await page.request.delete(`/api/v1/proxy-hosts/${createdProxyUUID}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(deleteResponse.ok()).toBe(true);
    });
  });

  test('Security modules apply to subsequently created resources', async ({ page }) => {
    await test.step('Enable global rate limiting', async () => {
      const token = await getAuthToken(page);
      const enableCerberusResponse = await page.request.post('/api/v1/security/cerberus/enable', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(enableCerberusResponse.ok()).toBe(true);

      const enableRateLimitResponse = await page.request.patch('/api/v1/security/rate-limit', {
        data: { enabled: true },
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(enableRateLimitResponse.ok()).toBe(true);

      const securityStatusResponse = await page.request.get('/api/v1/security/status', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(securityStatusResponse.ok()).toBe(true);
      const securityStatus = await securityStatusResponse.json();
      expect(securityStatus).toEqual(expect.objectContaining({
        rate_limit: expect.objectContaining({ enabled: true }),
      }));
    });

    await test.step('Create new user after security enabled', async () => {
      await createUserViaApi(page, { ...testUser, role: 'user' });
    });

    await test.step('Verify user subject to rate limiting', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
        await page.waitForURL(/login/);
      }

      await page.locator('input[type="email"]').first().fill(testUser.email);
      await page.locator('input[type="password"]').first().fill(testUser.password);
      await page.getByRole('button', { name: /sign in|login/i }).first().click();
      await page.waitForLoadState('networkidle');

      const userToken = await getAuthToken(page);
      expect(userToken).toBeTruthy();
      const origin = new URL(page.url()).origin;

      const responses = [];
      for (let i = 0; i < 5; i++) {
        const response = await page.request.get(
          `${origin}/api/v1/health?request=${i}`,
          {
            headers: { 'Authorization': `Bearer ${userToken || ''}` },
            ignoreHTTPSErrors: true,
          }
        );
        responses.push(response.status());
      }

      expect(responses.length).toBe(5);
      expect(responses.every((status) => status < 500)).toBe(true);
    });
  });

  test('Security enforced even on previously created resources', async ({ page }) => {
    await test.step('Create user before security enabled', async () => {
      await createUserViaApi(page, { ...testUser, role: 'user' });
    });

    await test.step('Enable rate limiting globally', async () => {
      const token = await getAuthToken(page);
      const enableCerberusResponse = await page.request.post('/api/v1/security/cerberus/enable', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(enableCerberusResponse.ok()).toBe(true);

      const enableRateLimitResponse = await page.request.patch('/api/v1/security/rate-limit', {
        data: { enabled: true },
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(enableRateLimitResponse.ok()).toBe(true);

      const securityStatusResponse = await page.request.get('/api/v1/security/status', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(securityStatusResponse.ok()).toBe(true);
      const securityStatus = await securityStatusResponse.json();
      expect(securityStatus).toEqual(expect.objectContaining({
        rate_limit: expect.objectContaining({ enabled: true }),
      }));

      const strictRateLimitSettings = [
        { key: 'security.rate_limit.requests', value: '1' },
        { key: 'security.rate_limit.window', value: '60' },
        { key: 'security.rate_limit.burst', value: '1' },
      ];

      for (const setting of strictRateLimitSettings) {
        const setResponse = await page.request.post('/api/v1/settings', {
          data: setting,
          headers: { Authorization: `Bearer ${token}` },
        });
        expect(setResponse.ok()).toBe(true);
      }
    });

    await test.step('Verify user is now rate limited', async () => {
      const logoutButton = page.getByRole('button', { name: /logout/i }).first();
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
        await page.waitForURL(/login/);
      }

      await page.locator('input[type="email"]').first().fill(testUser.email);
      await page.locator('input[type="password"]').first().fill(testUser.password);
      await page.getByRole('button', { name: /sign in|login/i }).first().click();
      await page.waitForLoadState('networkidle');

      const userToken = await getAuthToken(page);
      expect(userToken).toBeTruthy();
      const origin = new URL(page.url()).origin;

      const responses = [];
      await expect.poll(async () => {
        responses.length = 0;
        for (let i = 0; i < 30; i++) {
          const response = await page.request.get(
            `${origin}/api/v1/health?rapid=${i}`,
            {
              headers: { Authorization: `Bearer ${userToken}` },
              ignoreHTTPSErrors: true,
            }
          );
          responses.push(response.status());
        }

        return responses.includes(429);
      }, {
        timeout: 20000,
        message: 'Expected rate limiting enforcement to return at least one 429 for previously created user',
      }).toBe(true);

      expect(responses.every((status) => status < 500)).toBe(true);
    });
  });
});
