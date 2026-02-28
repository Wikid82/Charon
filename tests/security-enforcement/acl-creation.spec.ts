import { test, expect } from '@playwright/test';

const TEST_EMAIL = process.env.E2E_TEST_EMAIL || 'e2e-test@example.com';
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD || 'TestPassword123!';

async function authenticate(request: import('@playwright/test').APIRequestContext): Promise<string> {
  const loginResponse = await request.post('/api/v1/auth/login', {
    data: {
      email: TEST_EMAIL,
      password: TEST_PASSWORD,
    },
  });

  expect(loginResponse.ok()).toBeTruthy();
  const loginBody = await loginResponse.json();
  expect(loginBody.token).toBeTruthy();
  return loginBody.token as string;
}

test.describe('ACL Creation Baseline', () => {
  test('should create ACL and security header profile for dropdown coverage', async ({ request }) => {
    const token = await authenticate(request);
    const unique = Date.now();
    const aclName = `ACL Baseline ${unique}`;
    const profileName = `Headers Baseline ${unique}`;

    await test.step('Create ACL baseline entry', async () => {
      const aclResponse = await request.post('/api/v1/access-lists', {
        headers: {
          Authorization: `Bearer ${token}`,
        },
        data: {
          name: aclName,
          type: 'whitelist',
          enabled: true,
          ip_rules: JSON.stringify([
            {
              cidr: '127.0.0.1/32',
              description: 'Local test runner',
            },
          ]),
        },
      });

      expect(aclResponse.ok()).toBeTruthy();
    });

    await test.step('Create security headers profile baseline entry', async () => {
      const profileResponse = await request.post('/api/v1/security/headers/profiles', {
        headers: {
          Authorization: `Bearer ${token}`,
        },
        data: {
          name: profileName,
        },
      });

      expect(profileResponse.status()).toBe(201);
    });

    await test.step('Verify baseline entries are queryable', async () => {
      const aclListResponse = await request.get('/api/v1/access-lists', {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      expect(aclListResponse.ok()).toBeTruthy();
      const aclList = await aclListResponse.json();
      expect(Array.isArray(aclList)).toBeTruthy();
      expect(aclList.some((item: { name?: string }) => item.name === aclName)).toBeTruthy();

      const profileListResponse = await request.get('/api/v1/security/headers/profiles', {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      expect(profileListResponse.ok()).toBeTruthy();
      const profilePayload = await profileListResponse.json();
      const profiles = Array.isArray(profilePayload?.profiles) ? profilePayload.profiles : [];
      expect(profiles.some((item: { name?: string }) => item.name === profileName)).toBeTruthy();
    });
  });
});
